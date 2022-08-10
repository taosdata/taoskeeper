package monitor

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/taosdata/taoskeeper/api"
	"github.com/taosdata/taoskeeper/db"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
	"os"
	"sync/atomic"
	"time"
)

var logger = log.GetLogger("monitor")

var (
	cpuPercent  float64
	memPercent  float64
	totalReport int32
)

var identity string

func StartMonitor() {
	conn, err := db.NewConnector()
	if err != nil {
		logger.WithError(err).Errorf("connect to database error")
		panic(err)
	}
	var ctx = context.Background()
	_, err = conn.Exec(ctx, fmt.Sprintf("create database if not exists %s", config.Metrics.Database))
	if err != nil {
		logger.WithError(err).Errorf("create database %s error", config.Metrics.Database)
		panic(err)
	}
	if err := conn.Close(); err != nil {
		logger.WithError(err).Errorf("close connection error")
	}
	if len(config.Conf.Metrics.Cluster) != 0 {
		identity = config.Conf.Metrics.Cluster
	} else {
		hostname, err := os.Hostname()
		if err != nil {
			logger.WithError(err).Panic("can not get hostname")
		}
		if len(hostname) > 40 {
			hostname = hostname[:40]
		}
		identity = fmt.Sprintf("%s:%d", hostname, config.Conf.Port)
	}
	systemStatus := make(chan SysStatus)
	go func() {
		for {
			select {
			case status := <-systemStatus:
				if status.CpuError == nil {
					cpuPercent = status.CpuPercent
				}
				if status.MemError == nil {
					memPercent = status.MemPercent
				}
				report := api.TotalRep
				totalReport = report
				atomic.CompareAndSwapInt32(&api.TotalRep, report, 0)
				kn := md5.Sum([]byte(identity))
				sql := fmt.Sprintf("insert into `keeper_monitor_%s` using keeper_monitor tags ('%s') values ( now, "+
					" %f, %f, %d)", hex.EncodeToString(kn[:]), identity, cpuPercent, memPercent, totalReport)
				conn, err := db.NewConnectorWithDb()
				if err != nil {
					logger.WithError(err).Errorf("connect to database error")
					return
				}

				ctx := context.Background()
				if _, err = conn.Exec(ctx, sql); err != nil {
					logger.Errorf("execute sql: %s, error: %s", sql, err)
				}

				if err := conn.Close(); err != nil {
					logger.WithError(err).Errorf("close connection error")
				}
			}
		}
	}()
	SysMonitor.Register(systemStatus)
	interval, err := time.ParseDuration(config.Conf.RotationInterval)
	if err != nil {
		panic(err)
	}
	Start(interval, config.Conf.Env.InCGroup)
}
