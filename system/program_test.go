package system

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStart(t *testing.T) {
	server := Init()
	assert.NotNil(t, server)
}
