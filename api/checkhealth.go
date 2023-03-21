package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/taosdata/taoskeeper/version"
)

func NewCheckHealth() *CheckHealth {
	return &CheckHealth{}
}

type CheckHealth struct {
}

func (*CheckHealth) Init(c gin.IRouter) {
	c.GET("check_health", func(context *gin.Context) {
		context.JSON(http.StatusOK, map[string]string{"version": version.Version})
	})
}
