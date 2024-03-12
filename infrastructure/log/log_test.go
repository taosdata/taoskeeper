package log

import (
	"github.com/taosdata/taoskeeper/infrastructure/log"
	"testing"
)

var logger = log.GetLogger("log")

func TestConfigLog(t *testing.T) {
	log.IsDebug()
}
