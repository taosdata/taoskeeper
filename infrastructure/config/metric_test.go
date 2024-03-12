package config_test

import (
	"fmt"
	"github.com/taosdata/taoskeeper/infrastructure/log"
	"io"
	"os"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/taosdata/taoskeeper/infrastructure/config"
)

var logger = log.GetLogger("infrastructure")

func TestConfig(t *testing.T) {
	data := `
# Start with debug middleware for gin
debug = true
# Listen port, default is 6043
port = 9000
# log level
loglevel = "error"
# go pool size
gopoolsize = 5000
# interval for TDengine metrics
RotationInterval = "10s"
[tdengine]
address = "http://localhost:6041"
authtype = "Basic"
username = "root"
password = "taosdata"
`
	var c config.Config
	_, err := toml.Decode(data, &c)
	if err != nil {
		t.Error(err)
		return
	}
	assert.EqualValues(t, c, c)
	fmt.Print(c)
}
func TestBakConfig(t *testing.T) {
	copyConfigFile()
	config.Name = "aaa"

	conf := config.InitConfig()
	fmt.Print(log.IsDebug(), log.GetLogNow(true), log.GetLogDuration(true, time.Now()))
	logger.Debug(conf)
	config.Name = "taoskeeper"

}

func copyConfigFile() {
	sourceFile := "/etc/taos/taoskeeper.toml"
	destinationFile := "/etc/taos/keeper.toml"

	source, err := os.Open(sourceFile) //open the source file
	if err != nil {
		panic(err)
	}
	defer source.Close()

	destination, err := os.Create(destinationFile) //create the destination file
	if err != nil {
		panic(err)
	}
	defer destination.Close()
	_, err = io.Copy(destination, source) //copy the contents of source to destination file
	if err != nil {
		panic(err)
	}
}
