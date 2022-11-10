package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/taosdata/taoskeeper/db"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/process"
)

var router *gin.Engine

func TestMain(m *testing.M) {
	var dbName = "exporter_test"
	conf := config.InitConfig()
	conn, err := db.NewConnector(conf.TDengine.Username, conf.TDengine.Password, conf.TDengine.Host, conf.TDengine.Port)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	ctx := context.Background()
	if _, err = conn.Exec(ctx, fmt.Sprintf("create database if not exists %s", dbName)); err != nil {
		logger.Errorf("execute sql: %s, error: %s", fmt.Sprintf("create database %s", dbName), err)
	}
	gin.SetMode(gin.ReleaseMode)
	router = gin.New()
	reporter := NewReporter(conf)
	reporter.Init(router)
	processor := process.NewProcessor(conf)
	node := NewNodeExporter(processor)
	node.Init(router)
	m.Run()
	if _, err = conn.Exec(ctx, fmt.Sprintf("drop database if exists %s", dbName)); err != nil {
		logger.Errorf("execute sql: %s, error: %s", fmt.Sprintf("drop database %s", dbName), err)
	}
}

func TestGetMetrics(t *testing.T) {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

var report = Report{
	Ts:        "2022-03-03T20:23:00.929+0800",
	DnodeID:   1,
	DnodeEp:   "localhost:7100",
	ClusterID: "6980428120398645172",
	Protocol:  1,
	ClusterInfo: ClusterInfo{
		FirstEp:          "localhost:7100",
		FirstEpDnodeID:   1,
		Version:          "3.0.0.0",
		MasterUptime:     2.3090276954462752e-05,
		MonitorInterval:  1,
		VgroupsTotal:     2,
		VgroupsAlive:     2,
		VnodesTotal:      2,
		VnodesAlive:      2,
		ConnectionsTotal: 1,
		Dnodes: []Dnode{
			{
				DnodeID: 1,
				DnodeEp: "localhost:7100",
				Status:  "ready",
			},
		},
		Mnodes: []Mnode{
			{
				MnodeID: 1,
				MnodeEp: "localhost:7100",
				Role:    "master",
			},
		},
	},
	VgroupInfos: []VgroupInfo{
		{
			VgroupID:     1,
			DatabaseName: "test",
			TablesNum:    1,
			Status:       "ready",
			Vnodes: []Vnode{
				{
					DnodeID:   1,
					VnodeRole: "LEADER",
				},
				{
					DnodeID:   2,
					VnodeRole: "FOLLOWER",
				},
			},
		},
	},
	GrantInfo: GrantInfo{
		ExpireTime:      2147483647,
		TimeseriesUsed:  800,
		TimeseriesTotal: 2147483647,
	},
	DnodeInfo: DnodeInfo{
		Uptime:                0.000291412026854232,
		CPUEngine:             0.0828500414250207,
		CPUSystem:             0.4971002485501243,
		CPUCores:              12,
		MemEngine:             9268,
		MemSystem:             54279816,
		MemTotal:              65654816,
		DiskEngine:            0,
		DiskUsed:              39889702912,
		DiskTotal:             210304475136,
		NetIn:                 4727.45292368682,
		NetOut:                2194.251734390486,
		IoRead:                3789.8909811694753,
		IoWrite:               12311.19920713578,
		IoReadDisk:            0,
		IoWriteDisk:           12178.394449950447,
		ReqSelect:             2,
		ReqSelectRate:         0,
		ReqInsert:             6,
		ReqInsertSuccess:      4,
		ReqInsertRate:         0,
		ReqInsertBatch:        10,
		ReqInsertBatchSuccess: 8,
		ReqInsertBatchRate:    0,
		Errors:                2,
		VnodesNum:             2,
		Masters:               2,
		HasMnode:              1,
		HasQnode:              1,
		HasSnode:              1,
		HasBnode:              1,
	},
	DiskInfos: DiskInfo{
		Datadir: []DataDir{
			{
				Name:  "/root/TDengine/sim/dnode1/data",
				Level: 0,
				Avail: decimal.NewFromInt(171049893888),
				Used:  decimal.NewFromInt(39254581248),
				Total: decimal.NewFromInt(210304475136),
			},
			{
				Name:  "/root/TDengine/sim/dnode2/data",
				Level: 1,
				Avail: decimal.NewFromInt(171049893888),
				Used:  decimal.NewFromInt(39254581248),
				Total: decimal.NewFromInt(210304475136),
			},
		},
		Logdir: LogDir{
			Name:  "/root/TDengine/sim/dnode1/log",
			Avail: decimal.NewFromInt(171049771008),
			Used:  decimal.NewFromInt(39254704128),
			Total: decimal.NewFromInt(210304475136),
		},
		Tempdir: TempDir{
			Name:  "/tmp",
			Avail: decimal.NewFromInt(171049771008),
			Used:  decimal.NewFromInt(39254704128),
			Total: decimal.NewFromInt(210304475136),
		},
	},
	LogInfos: LogInfo{
		Logs: []Log{
			{
				Ts:      "2022-03-04T17:11:06.353+0800",
				Level:   "info",
				Content: "mnode open successfully",
			}, {
				Ts:      "2022-03-04T17:11:06.353+0800",
				Level:   "info",
				Content: "dnode-transpaort is initialized",
			}, {
				Ts:      "2022-03-04T17:11:06.353+0800",
				Level:   "info",
				Content: "dnode object is created, data:0x55ae5ac8a4a0",
			},
		},
		Summary: []Summary{
			{
				Level: "error",
				Total: 0,
			}, {
				Level: "info",
				Total: 114,
			}, {
				Level: "debug",
				Total: 117,
			}, {
				Level: "trace",
				Total: 126,
			},
		},
	},
}

func TestPutMetrics(t *testing.T) {
	conf := config.InitConfig()
	w := httptest.NewRecorder()
	b, _ := json.Marshal(report)
	body := strings.NewReader(string(b))
	req, _ := http.NewRequest(http.MethodPost, "/report", body)
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	conn, err := db.NewConnectorWithDb(conf.TDengine.Username, conf.TDengine.Password, conf.TDengine.Host,
		conf.TDengine.Port, conf.Metrics.Database)
	if err != nil {
		logger.WithError(err).Errorf("connect to database error")
		return
	}
	ctx := context.Background()
	data, err := conn.Query(ctx, "select info from log_summary")
	if err != nil {
		logger.Errorf("execute sql: %s, error: %s", "select * from log_summary", err)
	}
	for _, info := range data.Data {
		assert.Equal(t, int32(114), info[0])
	}
}
