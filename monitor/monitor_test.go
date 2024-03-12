package monitor

import (
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	conf := config.InitConfig()
	if conf == nil {
		panic("config error")
	}
	conf.Debug = true
	conf.Env.InCGroup = true
	interval, err := time.ParseDuration(conf.RotationInterval)
	if err != nil {
		panic(err)
	}
	Start(interval, conf.Env.InCGroup)
}
