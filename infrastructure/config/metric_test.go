package config_test

import (
	"fmt"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/taosdata/taoskeeper/infrastructure/config"
)

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
