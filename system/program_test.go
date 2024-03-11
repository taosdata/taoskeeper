package system

import (
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	server := Init()
	s := Start(server)
	time.Sleep(5 * time.Second)
	s.Stop()
}
