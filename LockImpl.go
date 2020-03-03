package main

import (
    . "PCPP/lockermodel"
    "bytes"
    "container/list"
    "crypto/md5"
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    mrand "math/rand"
    "net/http"
    "strconv"
    "strings"
    "sync"
    "sync/atomic"
    "time"
)

const (
    KN_BORD_LOCK_TYPE = 26 //平板锁类型
    KN_ARM_LOCK_TYPE     = 17 //摇臂锁类型
    KN_ARMLORA_LOCK_TYPE = 18 //lora摇臂锁类型
    KN_RADAR_LOCK_TYPE   = 33 //车位雷达类型

    PM_LOCK_TYPE = 99

    KN_LOCK = "KN"
    PM_LOCK = "PM"

    WORK_SET_NUM_LIMIT = 3000
)

type Worker struct {
    done chan int
    id int
    keyList *list.List
    keyMutex sync.RWMutex
    urlfn func(int) (string, bool)
    lckfn func(string) *LockObj
}

func (wk *Worker) ProcessInfinte() {
    tmdu := time.Second*(time.Duration(1))
    timer := time.NewTicker(tmdu)
    tick := timer.C //time.Tick(ticktime)
    defer func() {
        timer.Stop()
    }()

    atomic.AddUint32(&sInfo.TotalSendThread, 1)
    tlog.Infof("Worker %d process infinite begin and total send thread num is %d", wk.id, sInfo.TotalSendThread)

    for {
        select {
        case <-tick:
            if wk.process() == 0 {
                tlog.Infof("Worker %d process infinite finished and exit", wk.id)
                go func() {
                    wk.done <- wk.id
                }()

                atomic.AddUint32(&sInfo.TotalSendThread, ^uint32(0))
                return
            }
        }
    }
}

func (wk *Worker) process() int {
    wk.keyMutex.RLock()
    defer wk.keyMutex.RUnlock()

    tlog.Debugf("%d worker process begin", wk.id)

    var next *list.Element
    for value := wk.keyList.Front(); value != nil; value = next {
        next = value.Next()

        lckObj := wk.lckfn(value.Value.(string))
        if lckObj == nil {
            tlog.Errorf("Not found lock obj %s", value.Value.(string))
            wk.keyList.Remove(value)
            continue
        }

        lckObj.currentInterval++
        if lckObj.currentInterval <= lckObj.timerInterval {
            tlog.Debugf("%d %s lock obj current interval is %d", wk.id, value.Value.(string), lckObj.currentInterval)
            continue
        }

        lckObj.currentInterval = 0

        url, ok := wk.urlfn(lckObj.lck.GetLockType())
        if !ok {
            tlog.Debugf("Empty url that lock use to upload %d", lckObj.lck.GetLockSn())
            continue
        }

        //go func(lckObj *LockObj, url string) {
            lckObj.rwmutex.RLock()
            err := lckObj.lck.UpReport(url)
            if err != nil {
                atomic.AddUint32(&lckObj.uploadFailedNums, 1)
                atomic.AddUint64(&sInfo.FailedNumsOfUpload, 1)
                tlog.Errorf("Upload load error is %s", err.Error())
            } else {
                atomic.AddUint32(&lckObj.uploadSuccessNums, 1)
                atomic.AddUint64(&sInfo.SuccessNumsOfUpload, 1)
                tlog.Debugf("Upload load success.")
            }
            lckObj.rwmutex.RUnlock()
        //}(lckObj, url)

    }

    return wk.keyList.Len()
}

type SummaryInfo struct {
    TotalNumsOfUpload uint64   `json:"total_nums_of_upload"`
    SuccessNumsOfUpload uint64 `json:"success_nums_of_upload"`
    FailedNumsOfUpload uint64  `json:"failed_nums_of_upload"`
    RatioOfUpload float32      `json:"ratio_of_upload"`
    TotalLockNums int          `json:"total_lock_nums"`
    TotalSendThread uint32     `json:"total_send_thread"`
}

var sInfo SummaryInfo

type LockObj struct {
    done chan struct{}
    lck LockStatus
    rwmutex sync.RWMutex

    uploadSuccessNums uint32
    uploadFailedNums uint32

    timerInterval int
    currentInterval int
}

type LockImpl struct {
    rwmutex       sync.RWMutex
    uploadUrlInfo map[int]string

    lckObjMutex   sync.RWMutex
    lckObjMap map[string]*LockObj
    done      chan struct{}

    workListMutx sync.RWMutex
    workList *list.List
    workIdBase int
    workDone chan int
}

func (cmd *LockImpl) GetUploadUrl(devType int) (string, bool) {
    cmd.rwmutex.RLock()
    defer cmd.rwmutex.RUnlock()
    url, ok := cmd.uploadUrlInfo[devType]
    return url, ok
}

func (cmd *LockImpl) SetUploadUrl(devType int, url string) {
    cmd.rwmutex.Lock()
    defer cmd.rwmutex.Unlock()
    cmd.uploadUrlInfo[devType] = url
}

func (cmd *LockImpl) GetLockObj(sn int, locktype string) *LockObj {
    cmd.lckObjMutex.RLock()
    defer cmd.lckObjMutex.RUnlock()
    if obj, ok := cmd.lckObjMap[locktype + strconv.Itoa(sn)]; ok {
        return obj
    } else {
        return nil
    }
}

func (cmd *LockImpl) GetLockObjByMac(mac string) *LockObj {
    cmd.lckObjMutex.RLock()
    defer cmd.lckObjMutex.RUnlock()

    for _, value := range cmd.lckObjMap {
        if value.lck.GetLockMac() == mac {
            return value
        }
    }

    return nil
}

func (cmd *LockImpl) AddLockObj(locktype string, options ...func(interface{})) (int, error) {
    cmd.lckObjMutex.Lock()
    defer cmd.lckObjMutex.Unlock()

    sn := CreateLockSn()

    key := locktype + strconv.Itoa(sn)
    if _, ok := cmd.lckObjMap[key]; ok {
        return -1, fmt.Errorf("Already exist lock %d", sn)
    }

    var lockobj *LockObj
    var err error
    if lockobj, err = cmd.AddLockObjInner(sn, locktype, true, options...); err != nil {
        tlog.Errorf("Add lock obj failed %s", err.Error())
        return -1, err
    }

    lockobj.timerInterval = GetRandInt(cfgInfo.ConcurrentTimeScope)

    //go cmd.createUploadThread(lockobj, cmd.done, lockobj.done)

    go cmd.createWorkLoad(key)

    return sn, nil
}

func (cmd *LockImpl) createWorkLoad(key string) {
    tlog.Debugf("Create work load key is %s", key)

    cmd.workListMutx.Lock()
    defer cmd.workListMutx.Unlock()

    tlog.Debugf("Create work load get mutex and key is %s", key)

    var newFlag bool = true
    for e := cmd.workList.Front(); e != nil; e = e.Next() {
        e.Value.(*Worker).keyMutex.Lock()
        defer e.Value.(*Worker).keyMutex.Unlock()

        tlog.Debugf("Begin check worker %d", e.Value.(*Worker).id)

        if e.Value.(*Worker).keyList.Len() < cfgInfo.WorkSetLimit {
            tlog.Debugf("Use already exist worker %d", e.Value.(*Worker).id)

            e.Value.(*Worker).keyList.PushBack(key)
            newFlag = false
            break
        }
    }

    if newFlag {
        cmd.workIdBase++
        tlog.Infof("%s lock obj use new worker %d", key, cmd.workIdBase)
        wk := cmd.NewWorker(cmd.workIdBase)
        wk.keyList.PushBack(key)
        cmd.workList.PushFront(wk)
        go wk.ProcessInfinte()
    }
}

func (cmd *LockImpl) DelLockObj(sn int, locktype string) error {
    cmd.lckObjMutex.Lock()
    defer cmd.lckObjMutex.Unlock()
    //var obj *LockObj
    var ok bool
    key := locktype + strconv.Itoa(sn)
    if _, ok = cmd.lckObjMap[key]; !ok {
        return fmt.Errorf("Already not exist lock %d", sn)
    }

    lm := LockerModel{DevID: key}
    if err := lm.DeleteSync(0); err != nil {
        tlog.Errorf("Delete lock obj to db failed %s", err.Error())
        return err
    }

    delete(cmd.lckObjMap, key)

    ////
    //go func(obj *LockObj) {
    //    obj.done <- struct{}{}
    //}(obj)

    return nil
}

func (cmd *LockImpl) AddLockObjInner(sn int, locktype string, dbflag bool, options ...func(interface{})) (*LockObj, error) {
    lckobj := &LockObj{}
    if PM_LOCK == locktype {
        nowTimeStr := time.Now().Format("2006-01-02 15:04:05")
        pmbd := &PMLock{}
        lckobj.lck = pmbd
        lckobj.lck.SetStInfo(
            pmbd.SetSn(sn),
            pmbd.SetStatus("0"),
            pmbd.SetLockStateTime(nowTimeStr),
            pmbd.SetDetectorStatus("0"),
            pmbd.SetDetectorStatusTime(nowTimeStr),
            pmbd.SetLastTime(nowTimeStr),
            pmbd.SetDetectionAuto(0),
            pmbd.SetComStatus("1"),
            pmbd.SetElectricity("4.72"),
            pmbd.SetMac(NewRandomMac()),
            pmbd.SetStatusType(PM_LOCKSTATUS_TYPE),
            pmbd.SetStatusTime(strconv.Itoa(int(time.Now().Unix()))),
            pmbd.SetStatusLock(PM_LOCK_DOWN),
            pmbd.SetStatusMag(PM_MAG_EMPTY),
            pmbd.SetStatusMagFlag(PM_MAG_DISABLE),

        )
    } else {
        knbd := &KnBoardLock{}
        lckobj.lck = knbd
        lckobj.lck.SetStInfo(
            knbd.SetSn(sn),
            knbd.SetCar(0),
            knbd.SetPos(DOWN_POS),
            knbd.SetDevType(KN_BORD_LOCK_TYPE),
        )
    }

    key := locktype + strconv.Itoa(sn)

    lckobj.lck.SetStInfo(options...)

    lckobj.done = make(chan struct{})

    if !dbflag {
        cmd.lckObjMap[key] = lckobj
        return lckobj, nil
    }

    //存储到数据库中去
    //首先创建数据
    timestamp := time.Now().UnixNano() / 1e6

    lm := LockerModel{DevID: key}
    if PM_LOCK == locktype {
        lm.BaseInfo.Vid = lckobj.lck.GetLockMac()
    } else {
        lm.BaseInfo.Vid = strconv.Itoa(sn)
    }
    lm.BaseInfo.Sid = strconv.Itoa(sn)
    lm.BaseInfo.Model = locktype
    lm.BaseInfo.Ts = timestamp
    if err := lm.CreateSync(0); err != nil {
        tlog.Errorf("Add lock to db failed %s", err.Error())
        return nil, err
    }

    //所有数据都存储到extend字段去
    if err := cmd.SaveLockToDB(key, lckobj.lck.GetStInfo()); err != nil {
        tlog.Errorf("Save lock failed %s", err.Error())
        return nil, err
    }

    cmd.lckObjMap[key] = lckobj //确保数据库中添加成功之后再添加到内存中去

    return lckobj, nil
}

func (cmd *LockImpl) SaveLockToDB(key string, content string) error {
    lm2 := LockerModel{DevID:key}
    lm2.BaseInfo.ExtendInfo = content
    if err := lm2.SaveBaseInfoSync(0); err != nil {
        tlog.Errorf("Save lock to db failed %s", err.Error())
        return err
    }

    tlog.Debugf("Save lock to db success and info is %s", content)
    return nil
}

func (cmd *LockImpl) LoadAllLockFromDB() error {
    lm := LockerModel{}
    mlist, err := lm.LoadSyncAll(2)
    if err != nil {
        tlog.Errorf("Load from db failed %s", err.Error())
        return err
    }

    tlog.Infof("Load from db and locker number is %d", len(mlist))

    cmd.lckObjMutex.Lock()
    defer cmd.lckObjMutex.Unlock()
    cmd.lckObjMap = make(map[string]*LockObj) //重新赋值一个新的容器，相当于清除原来的容器中的对象

    for _, value := range mlist {
        sn, err := strconv.Atoi(value.BaseInfo.Sid)
        if err != nil {
            tlog.Errorf("Get sn failed %s", err.Error())
            return err
        }

        if PM_LOCK == value.BaseInfo.Model {
            pmbd := &PMLock{}
            err = json.Unmarshal([]byte(value.BaseInfo.ExtendInfo), pmbd)
            if err != nil{
                tlog.Errorf("Get lock json failed %s", err.Error())
                return err
            }

            _, err = cmd.AddLockObjInner(sn, value.BaseInfo.Model, false,
                pmbd.SetStatus(pmbd.Status),
                pmbd.SetLockStateTime(pmbd.LockStateTime),
                pmbd.SetDetectorStatus(pmbd.DetectorStatus),
                pmbd.SetDetectorStatusTime(pmbd.DetectorStatusTime),
                pmbd.SetLastTime(pmbd.LastTime),
                pmbd.SetDetectionAuto(pmbd.DetectionAuto),
                pmbd.SetComStatus(pmbd.ComStatus),
                pmbd.SetElectricity(pmbd.Electricity),
                pmbd.SetMac(pmbd.Mac),
                pmbd.SetStatusType(pmbd.StatusType),
                pmbd.SetStatusTime(pmbd.StatusTime),
                pmbd.SetStatusLock(pmbd.StatusLock),
                pmbd.SetStatusMag(pmbd.StatusMag),
                pmbd.SetStatusMagFlag(pmbd.StatusMagFlag),
                )
            if err != nil {
                tlog.Errorf("Add lock inner failed %s", err.Error())
                return err
            }


        } else {
            knbd := &KnBoardLock{}
            err = json.Unmarshal([]byte(value.BaseInfo.ExtendInfo), knbd)
            if err != nil{
                tlog.Errorf("Get lock json failed %s", err.Error())
                return err
            }

            _, err = cmd.AddLockObjInner(sn, value.BaseInfo.Model, false,
                knbd.SetCar(knbd.Car_sensor),
                knbd.SetPos(knbd.Position_sensor_status),
                knbd.SetDevType(knbd.Device_type),
                )
            if err != nil {
                tlog.Errorf("Add lock inner failed %s", err.Error())
                return err
            }
        }
    }

    return nil
}

func (cmd *LockImpl) Init(locknum int, pmlocknum int) error {
    cmd.workList = list.New()
    cmd.workDone = make(chan int)

    cmd.uploadUrlInfo = make(map[int]string)

    cmd.done = make(chan struct{})
    cmd.lckObjMap = make(map[string]*LockObj)

    for i := 0; i < locknum; i++ {
        if _, err := cmd.AddLockObjInner(CreateLockSn(), KN_LOCK, true); err != nil {
            tlog.Errorf("Add lock failed %s", err.Error())
            return err
        }
    }

    for i := 0; i < pmlocknum; i++ {
        if _, err := cmd.AddLockObjInner(CreateLockSn(), PM_LOCK, true); err != nil {
            tlog.Errorf("Add lock failed %s", err.Error())
            return err
        }
    }

    if err := cmd.LoadAllLockFromDB(); err != nil {
        tlog.Errorf("Load from db failed %s", err.Error())
        return err
    }

    cmd.UploadReport()
    return nil
}

func (cmd *LockImpl) createUploadThread(lck *LockObj, done chan struct{}, objDone chan struct{}) {
    tmvalue := GetRandInt(cfgInfo.ConcurrentTimeScope)
    tmdu := time.Second*(time.Duration(tmvalue))

    tlog.Debugf("%d Lock timeout %s", lck.lck.GetLockSn(), tmdu.String())

    timer := time.NewTicker(tmdu)
    tick := timer.C //time.Tick(ticktime)
    defer func() {
        timer.Stop()
    }()

    for {
        select {
        case <-tick:
            url, ok := cmd.GetUploadUrl(lck.lck.GetLockType())
            if !ok {
                tlog.Debugf("Empty url that lock use to upload %d", lck.lck.GetLockSn())
                continue
            }

            tlog.Debugf("Begin upload lock status to %s", url)

            lck.rwmutex.RLock()
            err := lck.lck.UpReport(url)
            if err != nil {
                atomic.AddUint32(&lck.uploadFailedNums, 1)
                atomic.AddUint64(&sInfo.FailedNumsOfUpload, 1)
                tlog.Errorf("Upload load error is %s", err.Error())
            } else {
                atomic.AddUint32(&lck.uploadSuccessNums, 1)
                atomic.AddUint64(&sInfo.SuccessNumsOfUpload, 1)
                tlog.Debugf("Upload load success.")
            }
            lck.rwmutex.RUnlock()

        case <-objDone:
            tlog.Debugf("%d timer exit", lck.lck.GetLockSn())
            return
        case <-done:
            tlog.Debugf("%d timer exit", lck.lck.GetLockSn())
            return
        }
    }
}

func (cmd *LockImpl) NewWorker(id int) *Worker {
    //cmd.workIdBase++

    return &Worker{
        done: cmd.workDone,
        id: id, //cmd.workIdBase,
        keyList:  list.New(),
        keyMutex: sync.RWMutex{},
        urlfn:    cmd.GetUploadUrl,
        lckfn: func(key string) *LockObj {
            cmd.lckObjMutex.RLock()
            defer cmd.lckObjMutex.RUnlock()
            if obj, ok := cmd.lckObjMap[key]; ok {
                return obj
            } else {
                return nil
            }
        },
    }
}

func (cmd *LockImpl) UploadReport() {
    cmd.lckObjMutex.RLock()
    defer cmd.lckObjMutex.RUnlock()

    var newFlag bool = true
    var count int
    for key, value := range cmd.lckObjMap {
        //time.Sleep(time.Millisecond*20)
        //go cmd.createUploadThread(value, cmd.done, value.done)

        value.timerInterval = GetRandInt(cfgInfo.ConcurrentTimeScope)

        var wk *Worker
        count++
        if newFlag || count > cfgInfo.WorkSetLimit {
            if newFlag {
                newFlag = false
            }
            if count > cfgInfo.WorkSetLimit {
                count = 0
            }

            cmd.workIdBase++
            wk = cmd.NewWorker(cmd.workIdBase)

            cmd.workList.PushFront(wk)
        } else {
            wk = cmd.workList.Front().Value.(*Worker)

        }

        wk.keyList.PushBack(key)
    }

    for e := cmd.workList.Front(); e != nil; e = e.Next() {
        //fmt.Printf("%v ", e.Value)

        go e.Value.(*Worker).ProcessInfinte()

    }

    go func() {
        for {
            select {
            case id := <- cmd.workDone:
                tlog.Infof("Begin worker check")
                cmd.workListMutx.Lock()

                var next *list.Element
                for value := cmd.workList.Front(); value != nil; value = next {
                    //fmt.Printf("%v ", e.Value)
                    next = value.Next()

                    if value.Value.(*Worker).id == id {
                        cmd.workList.Remove(value)
                        tlog.Infof("Work %d exit", id)
                        break
                    }
                }
                cmd.workListMutx.Unlock()

                tlog.Infof("Finish worker check")
            }
        }
    }()

}

func (cmd *LockImpl) GetLockInfo(sn int, locktype string) (string, error) {
    lckobj := cmd.GetLockObj(sn, locktype)
    if nil == lckobj {
        tlog.Errorf("Get lock obj is null %d", sn)
        return "", fmt.Errorf("Get lock obj is null")
    }

    lckobj.rwmutex.RLock()
    defer lckobj.rwmutex.RUnlock()

    value := lckobj.lck.GetStInfo()

    return  value, nil
}

func (cmd *LockImpl) SetLockInfo(sn int, locktype string, options ...func(interface{})) error {
    lckobj := cmd.GetLockObj(sn, locktype)
    if nil == lckobj {
        tlog.Errorf("Get lock obj is null %d", sn)
        return fmt.Errorf("Get lock obj is null")
    }

    lckobj.rwmutex.Lock()
    defer lckobj.rwmutex.Unlock()

    lckobj.lck.SetStInfo(options...)

    //所有数据都存储到extend字段去
    key := locktype + strconv.Itoa(sn)
    if err := cmd.SaveLockToDB(key, lckobj.lck.GetStInfo()); err != nil {
        tlog.Errorf("Save lock failed %s", err.Error())
        return err
    }

    return nil
}


func (cmd *LockImpl) LockCmdAction(sn int, action int, locktype string) error {
    lckobj := cmd.GetLockObj(sn, locktype)
    if nil == lckobj {
        tlog.Errorf("Get lock obj is null %d", sn)
        return fmt.Errorf("Get lock obj is null and device is %d", sn)
    }

    lckobj.rwmutex.Lock()
    defer lckobj.rwmutex.Unlock()
    if err := lckobj.lck.LockCmdAction(action); err != nil {
        tlog.Errorf("lock cmd failed %s", err.Error())
        return err
    }

    //所有数据都存储到extend字段去
    key := locktype + strconv.Itoa(sn)
    if err := cmd.SaveLockToDB(key, lckobj.lck.GetStInfo()); err != nil {
        tlog.Errorf("Save lock failed %s", err.Error())
        return err
    }

    return nil
}

func (cmd *LockImpl) MagCmdAction(sn int, action int, locktype string) error {
    lckobj := cmd.GetLockObj(sn, locktype)
    if nil == lckobj {
        tlog.Errorf("Get lock obj is null %d", sn)
        return fmt.Errorf("Get lock obj is null and device is %d", sn)
    }

    lckobj.rwmutex.Lock()
    defer lckobj.rwmutex.Unlock()
    if err := lckobj.lck.MagCmdAction(action); err != nil {
        tlog.Errorf("lock cmd failed %s", err.Error())
        return err
    }

    //所有数据都存储到extend字段去
    key := locktype + strconv.Itoa(sn)
    if err := cmd.SaveLockToDB(key, lckobj.lck.GetStInfo()); err != nil {
        tlog.Errorf("Save lock failed %s", err.Error())
        return err
    }

    return nil
}

func (cmd *LockImpl) SetStInfo(sn int, locktype string, options ...func(interface{})) error {
    lckobj := cmd.GetLockObj(sn, locktype)
    if nil == lckobj {
        tlog.Errorf("Get lock obj is null %d", sn)
        return fmt.Errorf("Get lock obj is null and device is %d", sn)
    }

    lckobj.rwmutex.Lock()
    defer lckobj.rwmutex.Unlock()

    lckobj.lck.SetStInfo(options...)

    //所有数据都存储到extend字段去
    key := locktype + strconv.Itoa(sn)
    if err := cmd.SaveLockToDB(key, lckobj.lck.GetStInfo()); err != nil {
        tlog.Errorf("Save lock failed %s", err.Error())
        return err
    }

    return nil
}

func (cmd *LockImpl) GetSummaryInfoCmd(w http.ResponseWriter, r *http.Request) {
    tlog.Debugf("Get summary info url is %s", r.RequestURI)
    var err error
    defer func() {
        if err != nil {
            body := []byte(err.Error())

            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            w.Write(body)
        }
    }()

    ////
    //values, err := url.ParseQuery(r.URL.RawQuery)
    //if err != nil {
    //    tlog.Errorf("parse url query parameters failed: %s", err.Error())
    //    return
    //}
    //
    //vmlock := values["vmlock"][0]
    //sn, err := strconv.Atoi(vmlock)
    //if err != nil {
    //    tlog.Errorf("Vmlock failed %s", err.Error())
    //    return
    //}

    info := cmd.GetSummaryInfo()

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(info))

    tlog.Debugf("Get summary info success and rsp body is %s", info)
}

func (cmd *LockImpl) GetSummaryInfo() string {
    cmd.lckObjMutex.RLock()
    defer cmd.lckObjMutex.RUnlock()

    summaryInfo := &SummaryInfo{}
    summaryInfo.TotalSendThread = atomic.LoadUint32(&sInfo.TotalSendThread)
    summaryInfo.TotalLockNums = len(cmd.lckObjMap)

    ////
    //for _, value := range cmd.lckObjMap {
    //    successNums := atomic.LoadUint32(&value.uploadSuccessNums)
    //    failedNums := atomic.LoadUint32(&value.uploadFailedNums)
    //    summaryInfo.SuccessNumsOfUpload += uint64(successNums)
    //    summaryInfo.FailedNumsOfUpload += uint64(failedNums)
    //}

    summaryInfo.SuccessNumsOfUpload = atomic.LoadUint64(&sInfo.SuccessNumsOfUpload)
    summaryInfo.FailedNumsOfUpload = atomic.LoadUint64(&sInfo.FailedNumsOfUpload)

    var successNum uint64
    if summaryInfo.SuccessNumsOfUpload == 0 {
        successNum = 1
    } else {
        successNum = summaryInfo.SuccessNumsOfUpload
    }

    ratioNum := float32(successNum) / float32(summaryInfo.FailedNumsOfUpload + successNum)

    summaryInfo.RatioOfUpload = ratioNum * 100
    summaryInfo.TotalNumsOfUpload = summaryInfo.SuccessNumsOfUpload + summaryInfo.FailedNumsOfUpload

    resultstr, err := json.Marshal(summaryInfo)
    if err != nil {
        tlog.Errorf("Get summary info failed json error is %s", err.Error())
        return err.Error()
    }

    return string(resultstr)
}


func GetMd5String(s string) string {
    h := md5.New()
    h.Write([]byte(s))
    return hex.EncodeToString(h.Sum(nil))
}


func GetMd5String2(s []byte) string {
    h := md5.New()
    h.Write(s)
    return hex.EncodeToString(h.Sum(nil))
}

func GetGuid() string {
    b := make([]byte, 48)

    if _, err := io.ReadFull(rand.Reader, b); err != nil {
        return ""
    }

    return GetMd5String2(b) //GetMd5String(base64.URLEncoding.EncodeToString(b))
}

func GetRandInt(uplimit int) int {
    mrand.Seed(time.Now().UnixNano())
    return mrand.Intn(uplimit) + 1
}

func HttpPostJson(jsonStr, urlStr string) (string, error) {
    parambytes := []byte(jsonStr)
    reader := bytes.NewReader(parambytes)
    req, err := http.NewRequest("POST", urlStr, reader)
    if err != nil{

        return "", err
    }

    client := &http.Client{}
    resp, err := client.Do(req)

    if err != nil {

        return "", err
    }

    defer resp.Body.Close()

    statuscode := resp.StatusCode
    //hea := resp.Header
    body, err := ioutil.ReadAll(resp.Body)
    if (err != nil){

        return "", err
    }


    if 200 == statuscode {
        return  string(body), nil
    } else {
        return "", fmt.Errorf("Post failed retcode is %d", statuscode)
    }
}

func HttpGet(params map[string]string, urlStr string) (string, error) {
    var keys []string
    for k := range params {
        keys = append(keys, k)
    }

    var paramStr bytes.Buffer
    for _, k := range keys {
        if paramStr.Len() > 0 {
            paramStr.WriteString("&")
        }
        paramStr.WriteString(k + "=")
        paramStr.WriteString(params[k])
    }

    url := fmt.Sprintf("%s?%s", urlStr, paramStr.String())

    c := http.Client{Timeout: time.Duration(cfgInfo.UploadTimeout) * time.Second}
    resp, err := c.Get(url)

    if resp != nil {
        defer resp.Body.Close()
    }

    if err != nil {
        tlog.Errorf("Pm upload failed %s", err.Error())
        return "", err
    }

    body, _ := ioutil.ReadAll(resp.Body)

    tlog.Debugf("Pm upload result is %s", string(body))
    return string(body), nil
}

type Mac [6]byte
func (m Mac) String() string {
    return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",m[0],m[1],m[2],m[3],m[4],m[5])
}

func NewRandomMac() string{
    var m [6]byte
    mrand.Seed(time.Now().UnixNano())
    for i:=0;i<6;i++ {
        mac_byte := mrand.Intn(256)
        m[i] = byte(mac_byte)

        //mrand.Seed(int64(mac_byte))
    }
    return strings.ToUpper(Mac(m).String())
}

func NewMsgId() string {
    mac := NewRandomMac()
    idlist := strings.Split(mac, ":")

    return idlist[0] + idlist[1]
}

//var lockSn int32 = 0

func CreateLockSn() int {
    //return int(atomic.AddInt32(&lockSn,1))
    mrand.Seed(time.Now().UnixNano())
    return mrand.Intn(190823330060)
}

