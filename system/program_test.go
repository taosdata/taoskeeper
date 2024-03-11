package system

import (
	"context"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	server := Init()
	Start(server)
	time.Sleep(5 * time.Second)
	server.Shutdown(context.Background())
}
