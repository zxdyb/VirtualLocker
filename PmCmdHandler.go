package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "strconv"
)

const (
    PM_SUCCESS_CODE = 110000
    PM_SUCCESS_MSG = "成功"

    PM_FAILED_CODE = 990000
    PM_FAILED_MSG = "失败"

)

type PmCommonReq struct {
    Type string         `json:"type"`
    Appid string        `json:"appid"`
    Sign string         `json:"sign"`
}

type PmCommonRsp struct {
    Status int          `json:"status"`
    Msg string          `json:"msg"`
    ResultCode string   `json:"resultcode"`
}

//http://120.24.54.186:8788/lockWeb?type=addParkVMLock&mac=FB:E9:E1:62:1B:42&appid=20190514121721&sign=19606c191285bc91063ab4955e25eb85
type PmAddLockCmdReq struct {
    PmCommonReq
    Mac string          `json:"mac"`
}

//{"status":110000,"msg":"成功","resultcode":"","vmlock":"191219905329"}
type PmAddLockCmdRsp struct {
    PmCommonRsp
    Vmlock string       `json:"vmlock"`
}


//http://120.24.54.186:8788/lockWeb?vmlock=191014651106&type=delParkVMLock&appid=20190514121721&sign=583c93199edf35caf3a17f62402c06c4
type PmDelLockCmdReq struct {
    PmCommonReq
    Vmlock string       `json:"vmlock"`
}

//{"status":110000,"msg":"成功","resultcode":""}
type PmDelLockCmdRsp struct {
    PmCommonRsp
}


//http://120.24.54.186:8788/lockClient?vmlock=191118962015&type=vmLockSwitch&status=0&appid=20190514121721&sign=33263363766c57fa6b831cd211973aa8
type PmActionCmdReq struct {
    PmCommonReq
    Vmlock string       `json:"vmlock"`
    Status string       `json:"status"` //0 Up 1 Down
}

//{"status":110000,"msg":"操作成功","resultcode":"","msgid":"19D2"}
type PmActionCmdRsp struct {
    PmCommonRsp
    Msgid string        `json:"msgid"`
}


//http://120.24.54.186:8788/lockClient?vmlock=191219905329&type=switchVMDetection&status=0&appid=20190514121721&sign=bc31eea925c8771789b479e5394f03c8
type PmSwitchMagCmdReq struct {
    PmCommonReq
    Vmlock string       `json:"vmlock"`
    Status string       `json:"status"` //0 关闭， 1 打开
}

//{"status":110000,"msg":"操作成功","resultcode":"","msgid":"19D2"}
type PmSwitchMagCmdRsp struct {
    PmCommonRsp
}


//http://120.24.54.186:8788/lockWeb?vmlock=191015383134&type=getAllParkVMLock&pagesize=20&pageindex=1&appid=20190514121721&sign=e5ef0766465d67e8f07784fb4c89dac3
type PmGetCmdReq struct {
    PmCommonReq
    Vmlock string       `json:"vmlock"`
    Pagesize string     `json:"pagesize"`
    Pageindex string    `json:"pageindex"`
}

//{"status":110000,"msg":"成功","count":1,"data":[{"vmlock":"191015383134","status":"0","detectorstatus":"0","lockstatustime":"2019-12-11 15:46:25","detectorstatustime":"2019-12-11 15:42:13","lasttime":"2019-12-11 15:46:31","detectionauto":0,"comstatus":"0","electricity":"5.30"}]}
type PmGetCmdRsp struct {
    PmCommonRsp
    Count int           `json:"count"`
    Data []PMLock       `json:"data"`
}


//http://xxxx:xx/lockWeb?type=regUploadURL&url=172.16.14.247:80
type PmRegUploadURLRsp struct {
    PmCommonRsp
}


type PmCmdHandler struct {
    lckImpl *LockImpl
}

func (cmd *PmCmdHandler) AddLockCmd(w http.ResponseWriter, r *http.Request) {
    tlog.Debugf("Add lock info url is %s", r.RequestURI)
    var err error
    defer func() {
        if err != nil {
            rsp := &PmAddLockCmdRsp{
                PmCommonRsp: PmCommonRsp{
                    Status:     PM_FAILED_CODE,
                    Msg:        PM_FAILED_MSG,
                    ResultCode: "",
                },
                Vmlock:      "",
            }

            resultstr, err := json.Marshal(rsp)
            if err != nil {
                tlog.Errorf("Add lock cmd failed json error is %s", err.Error())
                return
            }

            body := resultstr

            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            w.Write(body)
        }
    }()

    values, err := url.ParseQuery(r.URL.RawQuery)
    if err != nil {
        tlog.Errorf("parse url query parameters failed: %s", err.Error())
        return
    }

    pmbd := &PMLock{}
    var randMac string
    var sn int
    mac := values["mac"][0]
    if len(mac) > 0 {
        if lckOjbAlready := cmd.lckImpl.GetLockObjByMac(mac); lckOjbAlready != nil {
            sn = lckOjbAlready.lck.GetLockSn()
            randMac = lckOjbAlready.lck.GetLockMac()
        } else {
            sn, err = cmd.lckImpl.AddLockObj(PM_LOCK, pmbd.SetMac(mac))
            if err != nil {
                tlog.Errorf("Add lock obj failed %s", err.Error())
                return
            }
        }
    } else {
        randMac = NewRandomMac()
        sn, err = cmd.lckImpl.AddLockObj(PM_LOCK, pmbd.SetMac(randMac))
        if err != nil {
            tlog.Errorf("Add lock obj failed %s", err.Error())
            return
        }
    }

    rsp := &PmAddLockCmdRsp{
        PmCommonRsp: PmCommonRsp{
            Status:     PM_SUCCESS_CODE,
            Msg:        PM_SUCCESS_MSG,
            ResultCode: randMac, //"",
        },
        Vmlock:      strconv.Itoa(sn),
    }

    resultstr, err := json.Marshal(rsp)
    if err != nil {
        tlog.Errorf("Add lock json failed %s", err.Error())
        return
    }

    body := resultstr

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write(body)

    tlog.Debugf("Add lock success and rsp body is %s", body)
}

func (cmd *PmCmdHandler) DelLockCmd(w http.ResponseWriter, r *http.Request) {
    tlog.Debugf("Del lock info url is %s", r.RequestURI)
    var err error
    defer func() {
        if err != nil {
            rsp := &PmDelLockCmdRsp{
                PmCommonRsp: PmCommonRsp{
                    Status:     PM_FAILED_CODE,
                    Msg:        PM_FAILED_MSG,
                    ResultCode: "",
                },
            }

            resultstr, err := json.Marshal(rsp)
            if err != nil {
                tlog.Errorf("Del lock cmd failed json error is %s", err.Error())
                return
            }

            body := resultstr

            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            w.Write(body)
        }
    }()

    values, err := url.ParseQuery(r.URL.RawQuery)
    if err != nil {
        tlog.Errorf("parse url query parameters failed: %s", err.Error())
        return
    }

    vmlock := values["vmlock"][0]
    sn, err := strconv.Atoi(vmlock)
    if err != nil {
        tlog.Errorf("Vmlock failed %s", err.Error())
        return
    }

    err = cmd.lckImpl.DelLockObj(sn, PM_LOCK)
    if err != nil {
        tlog.Errorf("Del lock obj failed %s", err.Error())
        return
    }

    rsp := &PmDelLockCmdRsp{
        PmCommonRsp: PmCommonRsp{
            Status:     PM_SUCCESS_CODE,
            Msg:        PM_SUCCESS_MSG,
            ResultCode: "",
        },
    }

    resultstr, err := json.Marshal(rsp)
    if err != nil {
        tlog.Errorf("Del lock json failed %s", err.Error())
        return
    }

    body := resultstr

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write(body)

    tlog.Debugf("Del lock success and rsp body is %s", body)
}

func (cmd PmCmdHandler) ActionCmdDispatch(w http.ResponseWriter, r *http.Request) {
    values, err := url.ParseQuery(r.URL.RawQuery)
    if err != nil {
        tlog.Errorf("parse url query parameters failed: %s", err.Error())
        return
    }

    action := values["type"][0]
    if action == "vmLockSwitch" {
        cmd.ActionLockCmd(w, r)
    } else if action == "switchVMDetection" {
        cmd.SwitchMagLockCmd(w, r)
    } else {
        tlog.Errorf("Unknown action cmd %s", action)

        rsp := &PmActionCmdRsp{
            PmCommonRsp: PmCommonRsp{
                Status:     PM_FAILED_CODE,
                Msg:        PM_FAILED_MSG,
                ResultCode: "",
            },
            Msgid:      "",
        }

        resultstr, err := json.Marshal(rsp)
        if err != nil {
            tlog.Errorf("Lock action cmd failed json error is %s", err.Error())
            return
        }

        body := resultstr

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write(body)
    }
}

func (cmd *PmCmdHandler) ActionLockCmd(w http.ResponseWriter, r *http.Request) {
    tlog.Debugf("Lock action cmd url is %s", r.RequestURI)
    var err error
    defer func() {
        if err != nil {
            rsp := &PmActionCmdRsp{
                PmCommonRsp: PmCommonRsp{
                    Status:     PM_FAILED_CODE,
                    Msg:        PM_FAILED_MSG,
                    ResultCode: "",
                },
                Msgid:      "",
            }

            resultstr, err := json.Marshal(rsp)
            if err != nil {
                tlog.Errorf("Lock action cmd failed json error is %s", err.Error())
                return
            }

            body := resultstr

            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            w.Write(body)
        }
    }()

    values, err := url.ParseQuery(r.URL.RawQuery)
    if err != nil {
        tlog.Errorf("parse url query parameters failed: %s", err.Error())
        return
    }

    vmlock := values["vmlock"][0]
    sn, err := strconv.Atoi(vmlock)
    if err != nil {
        tlog.Errorf("Vmlock failed %s", err.Error())
        return
    }

    status := values["status"][0]
    action, err := strconv.Atoi(status)
    if err != nil {
        tlog.Errorf("Action failed %s", err.Error())
        return
    }

    err = cmd.lckImpl.LockCmdAction(sn, action, PM_LOCK)
    if err != nil {
        tlog.Errorf("Lock action cmd failed %s", err.Error())
        return
    }

    rsp := &PmActionCmdRsp{
        PmCommonRsp: PmCommonRsp{
            Status:     PM_SUCCESS_CODE,
            Msg:        PM_SUCCESS_MSG,
            ResultCode: "",
        },
        Msgid:      NewMsgId(),

    }

    resultstr, err := json.Marshal(rsp)
    if err != nil {
        tlog.Errorf("Lock action cmd json failed %s", err.Error())
        return
    }

    body := resultstr

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write(body)

    tlog.Debugf("Lock action cmd success and rsp body is %s", body)
}


func (cmd *PmCmdHandler) SwitchMagLockCmd(w http.ResponseWriter, r *http.Request) {
    tlog.Debugf("Switch mag lock cmd url is %s", r.RequestURI)
    var err error
    defer func() {
        if err != nil {
            rsp := &PmSwitchMagCmdRsp{
                PmCommonRsp: PmCommonRsp{
                    Status:     PM_FAILED_CODE,
                    Msg:        PM_FAILED_MSG,
                    ResultCode: "",
                },
            }

            resultstr, err := json.Marshal(rsp)
            if err != nil {
                tlog.Errorf("Switch mag lock cmd failed json error is %s", err.Error())
                return
            }

            body := resultstr

            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            w.Write(body)
        }
    }()

    values, err := url.ParseQuery(r.URL.RawQuery)
    if err != nil {
        tlog.Errorf("parse url query parameters failed: %s", err.Error())
        return
    }

    vmlock := values["vmlock"][0]
    sn, err := strconv.Atoi(vmlock)
    if err != nil {
        tlog.Errorf("Vmlock failed %s", err.Error())
        return
    }

    status := values["status"][0]
    action, err := strconv.Atoi(status)
    if err != nil {
        tlog.Errorf("Action failed %s", err.Error())
        return
    }

    err = cmd.lckImpl.MagCmdAction(sn, action, PM_LOCK)
    if err != nil {
        tlog.Errorf("Switch mag lock cmd failed %s", err.Error())
        return
    }

    rsp := &PmSwitchMagCmdRsp{
        PmCommonRsp: PmCommonRsp{
            Status:     PM_SUCCESS_CODE,
            Msg:        PM_SUCCESS_MSG,
            ResultCode: "",
        },
    }

    resultstr, err := json.Marshal(rsp)
    if err != nil {
        tlog.Errorf("Switch mag lock cmd json failed %s", err.Error())
        return
    }

    body := resultstr

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write(body)

    tlog.Debugf("Switch mag lock cmd success and rsp body is %s", body)
}

func (cmd *PmCmdHandler) GetLockCmd(w http.ResponseWriter, r *http.Request) {
    tlog.Debugf("Get lock info url is %s", r.RequestURI)
    var err error
    defer func() {
        if err != nil {
            rsp := &PmGetCmdRsp{
                PmCommonRsp: PmCommonRsp{
                    Status:     PM_FAILED_CODE,
                    Msg:        PM_FAILED_MSG,
                    ResultCode: "",
                },
                Count:  0,
                Data:   nil,
            }

            resultstr, err := json.Marshal(rsp)
            if err != nil {
                tlog.Errorf("Get lock cmd failed json error is %s", err.Error())
                return
            }

            body := resultstr

            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            w.Write(body)
        }
    }()

    values, err := url.ParseQuery(r.URL.RawQuery)
    if err != nil {
        tlog.Errorf("parse url query parameters failed: %s", err.Error())
        return
    }

    vmlock := values["vmlock"][0]
    sn, err := strconv.Atoi(vmlock)
    if err != nil {
        tlog.Errorf("Vmlock failed %s", err.Error())
        return
    }

    info, err := cmd.lckImpl.GetLockInfo(sn, PM_LOCK)
    if err != nil {
        tlog.Errorf("Get lock obj failed %s", err.Error())
        return
    }

    pm := &PMLock{}
    err = json.Unmarshal([]byte(info), pm)
    if err != nil{
        tlog.Errorf("Get lock json failed %s", err.Error())
        return
    }

    rsp := &PmGetCmdRsp{
        PmCommonRsp: PmCommonRsp{
            Status:     PM_SUCCESS_CODE,
            Msg:        PM_SUCCESS_MSG,
            ResultCode: "",
        },
        Count:  1,
        Data:   nil,
    }

    rsp.Data = append(rsp.Data, *pm)

    resultstr, err := json.Marshal(rsp)
    if err != nil {
        tlog.Errorf("Get lock json failed %s", err.Error())
        return
    }

    body := resultstr

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write(body)

    tlog.Debugf("Get lock success and rsp body is %s", body)
}

func (cmd *PmCmdHandler) RegUploadUrlCmd(w http.ResponseWriter, r *http.Request) {
    tlog.Debugf("Register upload url is %s", r.RequestURI)
    var err error
    defer func() {
        if err != nil {
            rsp := &PmRegUploadURLRsp{
                PmCommonRsp: PmCommonRsp{
                    Status:     PM_FAILED_CODE,
                    Msg:        PM_FAILED_MSG,
                    ResultCode: "",
                },
            }

            resultstr, err := json.Marshal(rsp)
            if err != nil {
                tlog.Errorf("Register upload url failed %s", err.Error())
                return
            }

            body := resultstr

            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            w.Write(body)
        }
    }()

    values, err := url.ParseQuery(r.URL.RawQuery)
    if err != nil {
        tlog.Errorf("parse url query parameters failed: %s", err.Error())
        return
    }

    urladdress := "http://" + values["url"][0]

    cmd.lckImpl.SetUploadUrl(PM_LOCK_TYPE, urladdress)
    tlog.Debugf("Pm upload url is %s", urladdress)

    rsp := &PmRegUploadURLRsp{
        PmCommonRsp: PmCommonRsp{
            Status:     PM_SUCCESS_CODE,
            Msg:        PM_SUCCESS_MSG,
            ResultCode: "",
        },
    }

    resultstr, err := json.Marshal(rsp)
    if err != nil {
        tlog.Errorf("Register upload url json failed %s", err.Error())
        return
    }

    body := resultstr

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write(body)

    tlog.Debugf("PM Register upload url success and rsp body is %s", body)
}

func (cmd *PmCmdHandler) SetStInfoCmd(w http.ResponseWriter, r *http.Request) {
    tlog.Debugf("Register upload url is %s", r.RequestURI)
    var err error
    defer func() {
        if err != nil {
            rsp := &PmRegUploadURLRsp{
                PmCommonRsp: PmCommonRsp{
                    Status:     PM_FAILED_CODE,
                    Msg:        PM_FAILED_MSG,
                    ResultCode: "",
                },
            }

            resultstr, err := json.Marshal(rsp)
            if err != nil {
                tlog.Errorf("Register upload url failed %s", err.Error())
                return
            }

            body := resultstr

            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            w.Write(body)
        }
    }()

    values, err := url.ParseQuery(r.URL.RawQuery)
    if err != nil {
        tlog.Errorf("parse url query parameters failed: %s", err.Error())
        return
    }

    vmlock := values["vmlock"][0]
    sn, err := strconv.Atoi(vmlock)
    if err != nil {
        tlog.Errorf("Vmlock failed %s", err.Error())
        return
    }

    stMag := values["StatusMag"][0]

    switch stMag {
    case PM_MAG_EMPTY:
        fallthrough
    case PM_MAG_PARK:
        fallthrough
    case PM_MAG_FAULT:
        break
    default:
        tlog.Errorf("Unknown mag status %s", stMag)
        err = fmt.Errorf("Unknown mag status %s", stMag)
        return
    }

    pmbd := &PMLock{}
    err = cmd.lckImpl.SetStInfo(sn, PM_LOCK, pmbd.SetStatusMag(stMag), pmbd.SetDetectorStatus(stMag))
    if err != nil {
        tlog.Errorf("Pm status mag set failed %s", err.Error())
        return
    }

    tlog.Debugf("Pm status mag is %s", stMag)

    rsp := &PmRegUploadURLRsp{
        PmCommonRsp: PmCommonRsp{
            Status:     PM_SUCCESS_CODE,
            Msg:        PM_SUCCESS_MSG,
            ResultCode: "",
        },
    }

    resultstr, err := json.Marshal(rsp)
    if err != nil {
        tlog.Errorf("Pm status mag json failed %s", err.Error())
        return
    }

    body := resultstr

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write(body)

    tlog.Debugf("Pm status mag success and rsp body is %s", body)
}


