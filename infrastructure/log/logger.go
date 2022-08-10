package log

import (
	"github.com/sirupsen/logrus"
	"github.com/taosdata/go-utils/log"
)

var Logger = log.NewLogger("taosKeeper")

func Init(level string) {
	log.SetLevel(level)
}

func GetLogger(model string) *logrus.Entry {
	return Logger.WithFields(logrus.Fields{"model": model})
}
