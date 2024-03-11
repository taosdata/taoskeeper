package system

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/taosdata/go-utils/web"
	"github.com/taosdata/taoskeeper/api"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
	"github.com/taosdata/taoskeeper/monitor"
	"github.com/taosdata/taoskeeper/process"
	"github.com/taosdata/taoskeeper/version"

	"github.com/kardianos/service"
)

var logger = log.GetLogger("program")

func Init() *http.Server {
	conf := config.InitConfig()
	log.ConfigLog()
	router := web.CreateRouter(conf.Debug, &conf.Cors, false)

	reporter := api.NewReporter(conf)
	reporter.Init(router)
	monitor.StartMonitor("", conf, reporter)
	go func() {
		// wait for monitor to all metric received
		time.Sleep(time.Second * 35)

		processor := process.NewProcessor(conf)
		node := api.NewNodeExporter(processor)
		node.Init(router)
	}()

	api.NewAdapterImporter(conf)

	checkHealth := api.NewCheckHealth(version.Version)
	checkHealth.Init(router)
	audit, err := api.NewAudit(conf)
	if err != nil {
		panic(err)
	}
	if err = audit.Init(router); err != nil {
		panic(err)
	}
	adapter := api.NewAdapter(conf)
	if err = adapter.Init(router); err != nil {
		panic(err)
	}

	gen_metric := api.NewGeneralMetric(conf)
	if err = gen_metric.Init(router); err != nil {
		panic(err)
	}

	server := &http.Server{
		Addr:    ":" + strconv.Itoa(conf.Port),
		Handler: router,
	}
	return server
}

func Start(server *http.Server) {
	prg := newProgram(server)
	svcConfig := &service.Config{
		Name:        "taoskeeper",
		DisplayName: "taoskeeper",
		Description: "taosKeeper is a tool for TDengine that exports monitoring metrics",
	}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		logger.Fatal(err)
	}
	err = s.Run()
	if err != nil {
		logger.Fatal(err)
	}
}

type program struct {
	server *http.Server
}

func newProgram(server *http.Server) *program {
	return &program{server: server}
}

func (p *program) Start(s service.Service) error {
	if service.Interactive() {
		logger.Info("Running in terminal.")
	} else {
		logger.Info("Running under service manager.")
	}

	server := p.server
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(fmt.Errorf("taoskeeper start up fail! %v", err))
		}
	}()
	return nil
}

func (p *program) Stop(s service.Service) error {
	logger.Println("Shutdown WebServer ...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.server.Shutdown(ctx); err != nil {
		logger.Println("WebServer Shutdown error:", err)
	}

	logger.Println("Server exiting")
	return nil
}
