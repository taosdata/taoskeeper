package monitor

import (
	"github.com/taosdata/go-utils/web"
	"github.com/taosdata/taoskeeper/api"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
	"testing"
)

func TestStart(t *testing.T) {
	conf := config.InitConfig()
	if conf == nil {
		panic("config error")
	}
	conf.Debug = true
	conf.Env.InCGroup = true

	log.ConfigLog()
	router := web.CreateRouter(conf.Debug, &conf.Cors, false)

	reporter := api.NewReporter(conf)
	reporter.Init(router)
	StartMonitor("", conf, reporter)
	for k, _ := range SysMonitor.outputs {
		SysMonitor.Deregister(k)
	}

}
