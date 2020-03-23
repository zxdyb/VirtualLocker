package main

import (
    . "PCPP/lockercommon"
    . "PCPP/lockermodel"
    "encoding/json"
    "github.com/gorilla/mux"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "strconv"
    "time"
)

type ConfigInfo struct {
    DbUser          string `json:"db_user"`
    DbPwd           string `json:"db_pwd"`
    DbAddress       string `json:"db_address"`
    DbPort          string `json:"db_port"`
    DbInstance      string `json:"db_instance"`
    SqlFilePath     string `json:"sql_file_path"`

    ConcurrentTimeScopeUp int `json:"concurrent_time_scope_up"`
    ConcurrentTimeScopeLow int `json:"concurrent_time_scope_low"`
    LogLevel        int    `json:"log_level"`
    UploadTimeout   int    `json:"upload_timeout"`
    WorkSetLimit    int    `json:"work_set_limit"`
    ListenPort      int    `json:"listen_port"`
}

func (cfgInfo *ConfigInfo) GetConfigFromJsonFile(filename string) {
    cfgInfo.DbUser = "tars"
    cfgInfo.DbPwd = "tars2015"
    cfgInfo.DbAddress = "172.16.14.247"
    cfgInfo.DbPort = "3306"
    cfgInfo.DbInstance = "PCPP_LockVirtual"
    cfgInfo.SqlFilePath = "./src/PCPP"
    cfgInfo.ConcurrentTimeScopeUp = 30
    cfgInfo.ConcurrentTimeScopeLow = 5
    cfgInfo.LogLevel = 0
    cfgInfo.UploadTimeout = 5
    cfgInfo.WorkSetLimit = WORK_SET_NUM_LIMIT
    cfgInfo.ListenPort = 9210

    cfgFile, err := os.Open(filename)
    if err != nil {
        tlog.Errorf("Open VirLocker.json failed %s", err.Error())
        return
    }

    content, err := ioutil.ReadAll(cfgFile)
    if err != nil {
        tlog.Errorf("Read VirLocker.json failed %s", err.Error())
        return
    }

    err = json.Unmarshal(content, cfgInfo)
    if err != nil {
        tlog.Errorf("Json failed %s", err.Error())
        return
    }
}

var tlog *LogWrapper

var cfgInfo *ConfigInfo

func main() {

    lgparam := LogWrapperParam{
        FileName:   "./Virlocker.log",
        MaxSize:    128,
        MaxBackups: 10,
        MaxAge:     7,
        Compress:   true,
        IsJson:     false,
        ServerName: "Virlocker",
        LogLevel:   0,
    }

    LockerLogger = GetLogWrapper(lgparam)
    tlog = LockerLogger

    cfgInfo = &ConfigInfo{}
    cfgInfo.GetConfigFromJsonFile("VirLocker.json")

    dbuser := cfgInfo.DbUser //"tars"
    dbpwd := cfgInfo.DbPwd //"tars2015"
    dbaddress := cfgInfo.DbAddress //"172.16.14.247"
    dbport := cfgInfo.DbPort //"3306"
    dbinstance := cfgInfo.DbInstance //"PCPP_LockVirtual"
    sqlfilepath := cfgInfo.SqlFilePath //"./src/PCPP" // "/home/sql/locker"
    listenAddress := "0.0.0.0:" + strconv.Itoa(cfgInfo.ListenPort)

    tlog.SetLoggerLevel(cfgInfo.LogLevel)

    tlog.Infof("Run config info is %+v", *cfgInfo)

    ret := LockerModelUseMysqlInit(dbuser, dbpwd, dbaddress, dbport, dbinstance, sqlfilepath, LockerLogger)
    if !ret {
        tlog.Errorf("Locker model init failed.")
        return
    }
    tlog.Debugf("Locker model init success.")

    lck := &LockImpl{}
    if err := lck.Init(0, 0); err != nil {
        tlog.Errorf("Lock init failed %s", err.Error())
        return
    }
    
    knLckHttpHandler := &KnCmdHandler{
        lckImpl: lck,
    }
    
    mux := mux.NewRouter()//http.NewServeMux()
    //mux.Handle("/open/lora/v1/commands", &KnCmdHandler{})

    //开能锁相关命令
    mux.HandleFunc("/open/lora/v1/commands", knLckHttpHandler.ActionLockCmd)        //设备动作命令下发，例如升锁/降锁
    mux.HandleFunc("/open/lora/v1/notifications", knLckHttpHandler.RegUploadUrlCmd) //设备上报地址注册
    mux.HandleFunc("/open/rs485/v1/lockers/{[0-9]+}", knLckHttpHandler.GetLockCmd)  //设备查询命令下发

    pmLckHttpHandler := &PmCmdHandler{
        lckImpl:lck,
    }

    //PM锁相关命令
    mux.Path("/lockWeb").Queries("vmlock", "{vmlock}", "type", "{type}", "pagesize", "{pagesize}",
        "pageindex", "{pageindex}", "appid",
        "{appid}", "sign", "{sign}").HandlerFunc(pmLckHttpHandler.GetLockCmd)

    mux.Path("/lockWeb").Queries("type", "{type}", "mac", "{mac}", "appid",
        "{appid}", "sign", "{sign}").HandlerFunc(pmLckHttpHandler.AddLockCmd)

    mux.Path("/lockWeb").Queries("vmlock", "{vmlock}", "type", "{type}", "appid",
        "{appid}", "sign", "{sign}").HandlerFunc(pmLckHttpHandler.DelLockCmd)

    mux.Path("/lockClient").Queries("vmlock", "{vmlock}", "type", "{type}", "status", "{status}", "appid",
        "{appid}", "sign", "{sign}").HandlerFunc(pmLckHttpHandler.ActionCmdDispatch)


    //PM锁没有注册设备上报地址的命令，这里是为了方便，自行实现的这个命令
    mux.Path("/lockWeb").Queries("type", "{type}", "url", "{url}").HandlerFunc(pmLckHttpHandler.RegUploadUrlCmd)
    //设置地磁状态是否已经停车
    mux.Path("/lockWeb").Queries("type", "{type}", "StatusMag", "{StatusMag}",
        "vmlock", "{vmlock}").HandlerFunc(pmLckHttpHandler.SetStInfoCmd)

    //获取内部概要信息命令
    mux.HandleFunc("/getSummaryInfo", lck.GetSummaryInfoCmd)

    server := &http.Server{
        Addr:         listenAddress, //"0.0.0.0:9210",
        WriteTimeout: time.Second * 3,            //设置3秒的写超时
        ReadTimeout:  time.Second * 3,
        Handler:      mux,
    }

    tlog.Infof("Starting locker httpserver")

    log.Fatal(server.ListenAndServe())
}

/*type myHandler struct{}

func (*myHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("this is version 3"))
}*/

/*func sayBye(w http.ResponseWriter, r *http.Request) {
    // 睡眠4秒  上面配置了3秒写超时，所以访问 “/bye“路由会出现没有响应的现象
    //time.Sleep(4 * time.Second)
    w.Write([]byte("bye bye ,this is v3 httpServer"))
}*/
