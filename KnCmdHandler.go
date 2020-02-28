package main

import (
    "encoding/json"
    "io/ioutil"
    "net/http"
    "strconv"
    "strings"
)

const (
    LOCK_CMD = "lock"
    UNLOCK_CMD = "unlock"
    UP_POS = 45532
    DOWN_POS = 100
)

type KnGetLockRsp struct {
    Code int                `json:"code"`
    Message string          `json:"message"`
    Objects []KnBoardLock   `json:"objects"`

}

type KnSetUploadUrlCmdReq struct {
    DevType int             `json:"device_type"`
    UploadUrl string        `json:"status_changed_callback_url"`
}
type KnSetUploadUrlCmdRsp struct {
    Code int                `json:"code"`
    Message string          `json:"message"`
}

type KnCmdReq struct {
    Sn int                  `json:"sn"`
    Command int             `json:"command"`
    Parameter KnCmdReqParam `json:"parameter"`
}
type KnCmdReqParam struct {
    Name string             `json:"name"`
    Command string          `json:"command"`
}

type KnCmdRsp struct {
    Code int                `json:"code"`
    Message string          `json:"message"`
    RequestId string        `json:"request_id"`
    Objects []KnCmdObj      `json:"objects"`
}
type KnCmdObj struct {
    Sn int                  `json:"sn"`
    Command int             `json:"command"`
    Parameter KnCmdReqParam `json:"parameter"`

    CommandStatus string    `json:"command_status"`
    Reason string           `json:"reason"`
    Guid string             `json:"guid"`
}

type KnCmdHandler struct {
    lckImpl *LockImpl

}

func (cmd *KnCmdHandler) GetLockCmd(w http.ResponseWriter, r *http.Request) {
    tlog.Debugf("Get lock info %s", r.RequestURI)
    var err error
    defer func() {
        if err != nil {
            rsp := &KnGetLockRsp{
                Code:    -1,
                Message: err.Error(),
                Objects: nil,
            }
            //rsp.Objects = append(rsp.Objects, *knb)

            resultstr, err := json.Marshal(rsp)
            if err != nil {
                tlog.Errorf("Get lock rsp json failed %s", err.Error())
                return
            }

            body := resultstr

            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            w.Write(body)
        }
    }()

    valuelist := strings.Split(r.RequestURI, "/")
    snStr := valuelist[len(valuelist) - 1]
    tlog.Debugf("Sn is %s", snStr)

    sn, err := strconv.Atoi(snStr)
    if err != nil {
        tlog.Errorf("Get lock failed %s", err.Error())
        return
    }

    value, err := cmd.lckImpl.GetLockInfo(sn, KN_LOCK)
    if err != nil {
        tlog.Errorf("Get lock failed %s", err.Error())
        return
    }

    knb := &KnBoardLock{}
    err = json.Unmarshal([]byte(value), knb)
    if err != nil{
        tlog.Errorf("Get lock json failed %s", err.Error())
        return
    }

    rsp := &KnGetLockRsp{
        Code:    0,
        Message: "OK",
        Objects: nil,
    }
    rsp.Objects = append(rsp.Objects, *knb)

    resultstr, err := json.Marshal(rsp)
    if err != nil {
        tlog.Errorf("Get lock json failed %s", err.Error())
        return
    }

    body := resultstr

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write(body)

}

func (cmd *KnCmdHandler) RegUploadUrlCmd(w http.ResponseWriter, r *http.Request) {
    var err error
    defer func() {
        if err != nil {
            rsp := &KnSetUploadUrlCmdRsp{
                Code:    -1,
                Message: err.Error(),
            }
            //rsp.Objects = append(rsp.Objects, *knb)

            resultstr, err := json.Marshal(rsp)
            if err != nil {
                tlog.Errorf("Register upload url cmd failed %s", err.Error())
                return
            }

            body := resultstr

            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            w.Write(body)
        }
    }()

    jsBody, err := ioutil.ReadAll(r.Body) //确保优先读取，再后面写入应答消息
    if err != nil{
        tlog.Errorf("Register upload url cmd failed %s", err.Error())
        return
    }

    req := &KnSetUploadUrlCmdReq{}
    err = json.Unmarshal(jsBody, req)
    if err != nil{
        tlog.Errorf("Register upload url cmd json failed %s", err.Error())
        return
    }

    cmd.lckImpl.SetUploadUrl(req.DevType, req.UploadUrl)
    tlog.Debugf("Upload url is %s and type is %d", req.UploadUrl, req.DevType)

    rsp := &KnSetUploadUrlCmdRsp{
        Code:    0,
        Message: "OK",
    }
    resultstr, err := json.Marshal(rsp)
    if err != nil {
        tlog.Errorf("Register upload url cmd json failed %s", err.Error())
        return
    }

    body := resultstr

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write(body)

}

func (cmd *KnCmdHandler) ActionLockCmd(w http.ResponseWriter, r *http.Request) {
    var err error
    defer func() {
        if err != nil {
            rsp := &KnCmdRsp{
                Code:    -1,
                Message: err.Error(),
                RequestId: "",
                Objects: nil,
            }
            //rsp.Objects = append(rsp.Objects, *knb)

            resultstr, err := json.Marshal(rsp)
            if err != nil {
                tlog.Errorf("Lock cmd failed %s", err.Error())
                return
            }

            body := resultstr

            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            w.Write(body)
        }
    }()

    jsBody, err := ioutil.ReadAll(r.Body) //确保优先读取，再后面写入应答消息
    if err != nil{
        tlog.Errorf("Lock cmd failed %s", err.Error())
        return
    }

    req := &KnCmdReq{}
    err = json.Unmarshal(jsBody, req)
    if err != nil{
        tlog.Errorf("Lock cmd failed %s", err.Error())
        return
    }

    action := CMD_LOCK_UP
    if "unlock" == req.Parameter.Command {
        action = CMD_LOCK_DOWN
    }

    err = cmd.lckImpl.LockCmdAction(req.Sn, action, KN_LOCK)
    if err != nil {
        tlog.Errorf("Lock cmd failed %s", err.Error())
        return
    }

    rsp := KnCmdRsp{
        Code:      0,
        Message:   "OK",
        RequestId: "",
        Objects:   []KnCmdObj{
            {
                Sn:            req.Sn,
                Command:       req.Command,
                Parameter:     req.Parameter,
                CommandStatus: "pending",
                Reason:        "",
                Guid:          GetGuid(),
            },
        },
    }

    resultstr, err := json.Marshal(rsp)
    if err != nil {
        tlog.Errorf("Lock cmd failed %s", err.Error())
        return
    }
    body := resultstr
    //body := []byte("{\"code\": 0, \"message\":\"ok\"}")

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    //w.Header().Add("Context-Type", "application/json")
    //w.Header().Add("Context-Length", strconv.Itoa(len(body)))
    //w.Header().Add("xxxx", "yyyyy")

    w.Write(body)

}



