package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/taosdata/go-utils/web"
	"github.com/taosdata/taoskeeper/api"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
	"github.com/taosdata/taoskeeper/monitor"
	"github.com/taosdata/taoskeeper/process"
)

func main() {
	conf := config.InitConfig()
	router := web.CreateRouter(conf.Debug, &conf.Cors, false)
	reporter := api.NewReporter(conf)
	reporter.Init(router)
	monitor.StartMonitor("", conf, reporter)
	processor := process.NewProcessor(conf)
	api.NewAdapterImporter(conf)
	node := api.NewNodeExporter(processor)
	node.Init(router)

	server := &http.Server{
		Addr:    ":" + strconv.Itoa(conf.Port),
		Handler: router,
	}
	logger := log.GetLogger("main")

	shutdown := make(chan struct{})
	go func() {
		defer close(shutdown)
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(timeoutCtx); err != nil {
			logger.Error("taoskeeper shutdown error ", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(fmt.Errorf("taoskeeper start up fail! %v", err))
	}

	<-shutdown
	logger.Warn("stop server")
}
