package monitor

import (
	"github.com/stretchr/testify/assert"
	"github.com/taosdata/go-utils/web"
	"github.com/taosdata/taoskeeper/api"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
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

	log.ConfigLog()
	router := web.CreateRouter(conf.Debug, &conf.Cors, false)

	reporter := api.NewReporter(conf)
	reporter.Init(router)
	conf.RotationInterval = "1s"
	StartMonitor("", conf, reporter)
	time.Sleep(2 * time.Second)
	for k, _ := range SysMonitor.outputs {
		SysMonitor.Deregister(k)
	}
}

func TestParseUint(t *testing.T) {
	num, err := parseUint("-1", 10, 8)
	assert.Equal(t, nil, err)
	assert.Equal(t, uint64(0), num)
	num, err = parseUint("0", 10, 8)
	assert.Equal(t, nil, err)
	assert.Equal(t, uint64(0), num)
	num, err = parseUint("257", 10, 8)
	assert.Equal(t, "strconv.ParseUint: parsing \"257\": value out of range", err.Error())
	assert.Equal(t, uint64(0), num)
}
