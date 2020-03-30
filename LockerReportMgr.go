package main

import (
    "encoding/json"
    "strconv"
    "time"
)

const (
    CMD_LOCK_UP   = 0
    CMD_LOCK_DOWN = 1
    CMD_MAG_ON    = 1
    CMD_MAG_OFF   = 0
)
type LockStatus interface {
    GetStInfo() string
    SetStInfo(options ...func(interface{}))
    GetLockType() int
    GetLockSn() int
    GetLockMac() string
    UpReport(URL string, flag string) error
    LockCmdAction(action int) error
    MagCmdAction(action int) error
    Clone() LockStatus
}

type optionFunc func(interface{})

type KnBoardLock struct {
    Sn               int            `json:"sn"`               //平板锁 SN（产品编号）
    Error            int            `json:"error"`            //异常码（0：正常；2：线圈故障）
    Coil_frequency   int            `json:"coil_frequency"`   //线圈频率
    Position_sensor_status int      `json:"position_sensor_status"`         //挡板位置值
    Locker_operating int            `json:"locker_operating"` //平板锁状态（0：未动作；1：动作中）
    Car_sensor       int            `json:"car_sensor"`       //车位状态检测（0：可停放；1：已占用）
    Action_counter   int            `json:"action_counter"`   //平板锁动作次数
    Updated_at       string         `json:"updated_at"`       //最后的更新时间
    Sign             string         `json:"sign"`             //签名字符
    Device_type      int            `json:"device_type"`      //设备类型（18：车位锁；26：平板锁； 34：车位雷达）
}

func (kblk *KnBoardLock) LockCmdAction(action int) error {
    pos := DOWN_POS
    if action == CMD_LOCK_UP {
        pos = UP_POS
    }

    kblk.SetStInfo(
        kblk.SetPos(pos),
        kblk.SetError(0),
    )

    tlog.Debugf("Lock cmd action is %d", action)
    return nil
}

func (kblk *KnBoardLock)MagCmdAction(action int) error {
    tlog.Debugf("Kn lock not impliment mag action cmd")
    return nil
}

func (kblk *KnBoardLock) UpReport(URL string, flag string) error {
    value := kblk.GetStInfo()

    tlog.Debugf("Upload content: %s %s", value, URL)
    ret, err := HttpPostJson(value, URL)
    if err != nil {
        tlog.Errorf("Up report failed %s", err.Error())
        return err
    }
    tlog.Debugf("Upload result success %s", ret)
    return nil
}

func (kblk *KnBoardLock) Clone() LockStatus {

    newInstance := &KnBoardLock{
        Sn:                     kblk.Sn,
        Error:                  kblk.Error,
        Coil_frequency:         kblk.Coil_frequency,
        Position_sensor_status: kblk.Position_sensor_status,
        Locker_operating:       kblk.Locker_operating,
        Car_sensor:             kblk.Car_sensor,
        Action_counter:         kblk.Action_counter,
        Updated_at:             kblk.Updated_at,
        Sign:                   kblk.Sign,
        Device_type:            kblk.Device_type,
    }

    return newInstance
}

func (kblk *KnBoardLock) GetLockType() int {
    return kblk.Device_type
}

func (kblk *KnBoardLock) GetLockSn() int {
    return kblk.Sn
}

func (kblk *KnBoardLock) GetLockMac() string {
    return ""
}

func (kblk *KnBoardLock) GetStInfo() string {
    resultstr, err := json.Marshal(kblk)
    if err != nil {
        tlog.Errorf("Get info failed %s", err.Error())
        return ""
    }

    return string(resultstr)
}

func (kblk *KnBoardLock) SetStInfo(options ...func(interface{})) {
    for _, option := range options {
        option(kblk)
    }
}

func (kblk *KnBoardLock) SetSn(sn int) optionFunc {
    return func(obj interface{}) {
        knbd := obj.(*KnBoardLock)
        knbd.Sn = sn
    }
}
func (kblk *KnBoardLock) SetError(er int) optionFunc {
    return func(obj interface{}) {
        knbd := obj.(*KnBoardLock)
        knbd.Error = er
    }
}
func (kblk *KnBoardLock) SetCoil(coil int) optionFunc {
    return func(obj interface{}) {
        knbd := obj.(*KnBoardLock)
        knbd.Coil_frequency = coil
    }
}
func (kblk *KnBoardLock) SetPos(pos int) optionFunc {
    return func(obj interface{}) {
        knbd := obj.(*KnBoardLock)
        knbd.Position_sensor_status = pos
    }
}
func (kblk *KnBoardLock) SetLckOp(lckop int) optionFunc {
    return func(obj interface{}) {
        knbd := obj.(*KnBoardLock)
        knbd.Locker_operating = lckop
    }
}
func (kblk *KnBoardLock) SetCar(car int) optionFunc {
    return func(obj interface{}) {
        knbd := obj.(*KnBoardLock)
        knbd.Car_sensor = car
    }
}
func (kblk *KnBoardLock) SetActCn(actcn int) optionFunc {
    return func(obj interface{}) {
        knbd := obj.(*KnBoardLock)
        knbd.Action_counter = actcn
    }
}
func (kblk *KnBoardLock) SetUpTime(uptm string) optionFunc {
    return func(obj interface{}) {
        knbd := obj.(*KnBoardLock)
        knbd.Updated_at = uptm
    }
}
func (kblk *KnBoardLock) SetSign(sg string) optionFunc {
    return func(obj interface{}) {
        knbd := obj.(*KnBoardLock)
        knbd.Sign = sg
    }
}
func (kblk *KnBoardLock) SetDevType(devtp int) optionFunc {
    return func(obj interface{}) {
        knbd := obj.(*KnBoardLock)
        knbd.Device_type = devtp
    }
}


type KnArmLock struct {
    Sn int                      `json:"sn"`//车位锁 SN（产品编号）
    Voltage float64             `json:"voltage"` //电池电压（单位：V）
    //Frequency float64           `json:"frequency"` //环形检测线圈的频率
    Locked int                  `json:"locked"` //车位锁状态（0：上升；1：下降）
    Car_detected int            `json:"car_detected"` //车位状态检测（0：可停放；1：已占用）
    Shell_opened int            `json:"shell_opened"` //外壳状态（0：正常；1：打开）
    Module_ready int            `json:"module_ready"` //NB-IoT 模块（0：未响应；1：正常）
    Network_ready int           `json:"network_ready"` //NB-IoT 模块（0：未连接；1：正常）
    Low_battery int             `json:"low_battery"` //电池状态（0：正常；1：电压过低）
    Coil_fault int              `json:"coil_fault"` //检测线圈状态（0：正常；1：故障）
    Bar_position_error int      `json:"bar_position_error"` //挡臂位置状态（0：正常；1：故障）
    Motor_fail int              `json:"motor_fail"` //电机状态（0：正常；1：故障）
    Bt_fail int                 `json:"bt_fail"` //蓝牙状态（0：正常；1：故障）
    Sim_card_fail int           `json:"sim_card_fail"` //NB-IoT 卡状态（0：正常；1：故障）
    Signal_fail int             `json:"signal_fail"` //NB-IoT 信号状态（0：正常；1：故障）
    Overdue int                 `json:"overdue"` //NB-IoT 卡欠费（0：正常；1：故障）
    Updated_at string           `json:"updated_at"` //最后的更新时间
    Sign string                 `json:"sign"`//签名字符
    Device_type int             `json:"device_type"` //设备类型（18：车位锁；26：平板锁；34：车位雷达）
}



type KnRaDar struct {
    Sn int                      `json:"sn"`//车位雷达 SN（产品编号）
    Voltage float64             `json:"voltage"` //电池电压（单位：V）
    Car_detected int            `json:"car_detected"` //车位状态检测（0：可停放；1：已占用）
    Module_ready int            `json:"module_ready"` //NB-IoT 模块（0：未响应；1：正常）
    Network_ready int           `json:"network_ready"` //NB-IoT 模块（0：未连接；1：正常）
    Low_battery int             `json:"low_battery"` //电池状态（0：正常；1：电压过低）
    Coil_fault int              `json:"coil_fault"` //检测线圈状态（0：正常；1：故障）
    Sim_card_fail int           `json:"sim_card_fail"` //NB-IoT 卡状态（0：正常；1：故障）
    Signal_fail int             `json:"signal_fail"` //NB-IoT 信号状态（0：正常；1：故障）
    Overdue int                 `json:"overdue"` //NB-IoT 卡欠费（0：正常；1：故障）
    Updated_at string           `json:"updated_at"` //最后的更新时间
    Sign string                 `json:"sign"`//签名字符
    Device_type int             `json:"device_type"` //设备类型（18：车位锁；26：平板锁；34：车位雷达）
}

const (
    PM_LOCKSTATUS_TYPE = "1"
    PM_MAGSTATUS_TYPE = "2"

    PM_LOCK_UP = "0"
    PM_LOCK_DOWN = "1"
    PM_LOCK_FAULT = "2"
    PM_LOCK_DAMAGE = "8"
    PM_LOCK_LOWPOWER = "3"

    PM_MAG_EMPTY = "0"
    PM_MAG_PARK = "1"
    PM_MAG_FAULT = "5"

    PM_MAG_DISABLE = "0"
    PM_MAG_ENABLE = "1"
)

type PMLock struct {
    Vmlock             string `json:"vmlock"`               //锁id
    Status             string `json:"status"`               //0 up, 1 down
    LockStateTime      string `json:"lockstatetime"`        //2019-11-20 15:18:30
    DetectorStatus     string `json:"detectorstatus"`       //0 empty 1 park
    DetectorStatusTime string `json:"detectorstatustime"`   //2019-10-15 20:08:42
    LastTime           string `json:"lasttime"`             //2019-10-15 20:08:42
    DetectionAuto      int    `json:"detectionauto"`        //0 disable， 1 enable
    ComStatus          string `json:"comstatus"`            //0 timeout , 1 normal
    Electricity        string `json:"electricity"`          //4.70
    Mac                string `json:"mac"`                  //mac address
    StatusType         string `json:"status_type"`          //1 lockstatus, 2 magstatus
    StatusTime         string `json:"status_time"`          //1570759562

    StatusLock         string `json:"status_lock"`          //0 up, 1 down, 2 fault, damage 8, lowpower 3
    StatusMag          string `json:"status_mag"`           //0 empty, 1 park, 5 fault
    StatusMagFlag      string `json:"status_mag_flag"`      //0 disable， 1 enable
}

func (pml *PMLock) LockCmdAction(action int) error {
    st := PM_LOCK_UP //0 up , 1 down
    if strconv.Itoa(action) == PM_LOCK_DOWN {
        st = PM_LOCK_DOWN
    }

    pml.StatusLock = st
    pml.Status = st
    return nil
}

func (pml *PMLock) MagCmdAction(action int) error {
    st := PM_MAG_ENABLE //1 enable, 0 disable
    if strconv.Itoa(action) == PM_MAG_DISABLE {
        st = PM_MAG_DISABLE
    }

    iSt, _ := strconv.Atoi(st)

    pml.StatusMagFlag = st
    pml.DetectionAuto = iSt
    return nil
}

///pmlocker?blue_mac=E5:EB:4F:BC:8C:2B&vmlock=190927827134&status_type=1&status=1&status_time=1570759562
func (pml *PMLock) UpReport(URL string, flag string) error {
    var st string
    if pml.StatusType == PM_LOCKSTATUS_TYPE && pml.StatusMagFlag == PM_MAG_ENABLE {
        pml.StatusType = PM_MAGSTATUS_TYPE
        st = pml.StatusMag
        tlog.Debugf("Upload info is lock mag and value is %s", st)
    } else {
        pml.StatusType = PM_LOCKSTATUS_TYPE
        st = pml.StatusLock
        tlog.Debugf("Upload info is lock status and value is %s", st)
    }

    pml.StatusTime = strconv.Itoa(int(time.Now().Unix())) //更新时间戳

    url := URL + "/pmlocker"
    params := make(map[string]string)
    params["blue_mac"] = pml.Mac
    params["vmlock"] = pml.Vmlock
    params["status_type"] = pml.StatusType
    params["status"] = st //pml.Status
    params["status_time"] = pml.StatusTime

    ret, err := HttpGet(params, url)
    if err != nil {
        tlog.Errorf("Pm upload failed %s", err.Error())
        return err
    }

    tlog.Debugf("Pm Upload result: %s and upload info is %+v", ret, params)
    return nil
}

func (pml *PMLock) Clone() LockStatus {
    newInstance := &PMLock{
        Vmlock:             pml.Vmlock,
        Status:             pml.Status,
        LockStateTime:      pml.LockStateTime,
        DetectorStatus:     pml.DetectorStatus,
        DetectorStatusTime: pml.DetectorStatusTime,
        LastTime:           pml.LastTime,
        DetectionAuto:      pml.DetectionAuto,
        ComStatus:          pml.ComStatus,
        Electricity:        pml.Electricity,
        Mac:                pml.Mac,
        StatusType:         pml.StatusType,
        StatusTime:         pml.StatusTime,
        StatusLock:         pml.StatusLock,
        StatusMag:          pml.StatusMag,
        StatusMagFlag:      pml.StatusMagFlag,
    }

    return newInstance
}

func (pml *PMLock)  GetStInfo() string {
    resultstr, err := json.Marshal(pml)
    if err != nil {
        tlog.Errorf("Get info failed %s", err.Error())
        return ""
    }

    return string(resultstr)
}

func (pml *PMLock) SetStInfo(options ...func(interface{})) {
    for _, option := range options {
        option(pml)
    }
}

func (pml *PMLock) GetLockType() int {
    return PM_LOCK_TYPE //pm锁不区分类型
}

func (pml *PMLock) GetLockSn() int {
    sn, err := strconv.Atoi(pml.Vmlock)
    if err != nil {
        tlog.Errorf("Get pm lock sn failed : %d %s", pml.Vmlock, err.Error())
        return 0
    }
    return sn
}

func (pml *PMLock) GetLockMac() string {
    return pml.Mac
}

func (pml *PMLock) SetSn(sn int) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.Vmlock = strconv.Itoa(sn)
    }
}

func (pml *PMLock) SetStatus(status string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.Status = status
    }
}

func (pml *PMLock) SetLockStateTime(time string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.LockStateTime = time
    }
}

func (pml *PMLock) SetDetectorStatus(status string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.DetectorStatus = status
    }
}

func (pml *PMLock) SetDetectorStatusTime(time string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.DetectorStatusTime = time
    }
}

func (pml *PMLock) SetLastTime(time string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.LastTime = time
    }
}

func (pml *PMLock) SetDetectionAuto(flag int) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.DetectionAuto = flag
    }
}

func (pml *PMLock) SetComStatus(status string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.ComStatus = status
    }
}

func (pml *PMLock) SetElectricity(elt string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.Electricity = elt
    }
}

func (pml *PMLock) SetMac(mac string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.Mac = mac
    }
}

func (pml *PMLock) SetStatusType(st string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.StatusType = st
    }
}

func (pml *PMLock) SetStatusTime(st string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.StatusTime = st
    }
}

func (pml *PMLock) SetStatusLock(st string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.StatusLock = st
    }
}

func (pml *PMLock) SetStatusMag(st string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.StatusMag = st
    }
}

func (pml *PMLock) SetStatusMagFlag(st string) optionFunc {
    return func(obj interface{}) {
        pmbd := obj.(*PMLock)
        pmbd.StatusMagFlag = st
    }
}


