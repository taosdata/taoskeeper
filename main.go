package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/taosdata/taoskeeper/monitor"

	"github.com/taosdata/go-utils/web"
	"github.com/taosdata/taoskeeper/api"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/process"
	"golang.org/x/sync/errgroup"
)

func main() {
	config.Init()
	router := web.CreateRouter(config.Conf.Debug, &config.Conf.Cors, false)
	monitor.StartMonitor("")
	report := api.Report{}
	report.Init(router)
	processor := process.NewProcessor()
	api.NewAdapterImporter()
	node := api.NewNodeExporter(processor)
	node.Init(router)
	conf := config.Conf
	server := &http.Server{
		Addr:    ":" + strconv.Itoa(conf.Port),
		Handler: router,
	}
	var g errgroup.Group
	g.Go(server.ListenAndServe)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	<-quit
	fmt.Println("stop server")
}
