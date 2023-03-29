package api

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/taosdata/go-utils/json"
	"github.com/taosdata/taoskeeper/db"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
)

var logger = log.GetLogger("report")

var createList = []string{
	CreateClusterInfoSql,
	CreateDnodeSql,
	CreateMnodeSql,
	CreateDnodeInfoSql,
	CreateDataDirSql,
	CreateLogDirSql,
	CreateTempDirSql,
	CreateVgroupsInfoSql,
	CreateVnodeRoleSql,
	CreateLogSql,
	CreateSummarySql,
	CreateGrantInfoSql,
	CreateKeeperSql,
}

type Reporter struct {
	username string
	password string
	host     string
	port     int
	dbname   string
	totalRep atomic.Value
}

func NewReporter(conf *config.Config) *Reporter {
	r := &Reporter{
		username: conf.TDengine.Username,
		password: conf.TDengine.Password,
		host:     conf.TDengine.Host,
		port:     conf.TDengine.Port,
		dbname:   conf.Metrics.Database,
	}
	r.totalRep.Store(0)
	return r
}

func (r *Reporter) Init(c gin.IRouter) {
	c.POST("report", r.handlerFunc())
	r.createDatabase()
	r.creatTables()
	// todo: it can delete in the future.
	r.detectGrantInfoFieldType()
	r.detectClusterInfoFieldType()
	r.detectVgroupsInfoType()
}

func (r *Reporter) getConn() *db.Connector {
	conn, err := db.NewConnector(r.username, r.password, r.host, r.port)
	if err != nil {
		logger.WithError(err).Error("connect to database error")
		panic(err)
	}
	return conn
}

func (r *Reporter) detectGrantInfoFieldType() {
	// `expire_time` `timeseries_used` `timeseries_total` in table `grant_info` changed to bigint from TS-3003.
	ctx := context.Background()
	conn := r.getConn()
	defer r.closeConn(conn)

	r.detectFieldType(ctx, conn, "grants_info", "expire_time", "bigint")
	r.detectFieldType(ctx, conn, "grants_info", "timeseries_used", "bigint")
	r.detectFieldType(ctx, conn, "grants_info", "timeseries_total", "bigint")
}

func (r *Reporter) detectClusterInfoFieldType() {
	// `tbs_total` in table `cluster_info` changed to bigint from TS-3003.
	ctx := context.Background()
	conn := r.getConn()
	defer r.closeConn(conn)

	r.detectFieldType(ctx, conn, "cluster_info", "tbs_total", "bigint")
}

func (r *Reporter) detectVgroupsInfoType() {
	// `tables_num` in table `vgroups_info` changed to bigint from TS-3003.
	ctx := context.Background()
	conn := r.getConn()
	defer r.closeConn(conn)

	r.detectFieldType(ctx, conn, "vgroups_info", "tables_num", "bigint")
}

func (r *Reporter) detectFieldType(ctx context.Context, conn *db.Connector, table, field, fieldType string) {
	res, err := conn.Query(ctx, fmt.Sprintf("select col_type from information_schema.ins_columns where table_name='%s' and db_name='%s' and col_name='%s'", table, r.dbname, field))
	if err != nil {
		logger.WithError(err).Errorf("get %s field type error", r.dbname)
		panic(err)
	}

	if len(res.Data) == 0 {
		return
	}

	if len(res.Data) != 1 && len(res.Data[0]) != 1 {
		logger.Errorf("get field type for %s error. response is %+v", table, res)
		panic(fmt.Sprintf("get field type for %s error. response is %+v", table, res))
	}

	colType := res.Data[0][0].(string)
	if colType == "INT" {
		logger.Warningf("## %s.%s.%s type is %s, will change to %s", r.dbname, table, field, colType, fieldType)
		// drop column `tables_num`
		if _, err = conn.Exec(ctx, fmt.Sprintf("alter table %s.%s drop column %s", r.dbname, table, field)); err != nil {
			logger.WithError(err).Errorf("drop column %s error", field)
			panic(err)
		}

		// add column `tables_num`
		if _, err = conn.Exec(ctx, fmt.Sprintf("alter table %s.%s add column %s %s", r.dbname, table, field, fieldType)); err != nil {
			logger.WithError(err).Errorf("add column %s error", field)
			panic(err)
		}
	}
}

func (r *Reporter) createDatabase() {
	ctx := context.Background()
	conn := r.getConn()
	defer r.closeConn(conn)

	if _, err := conn.Exec(ctx, fmt.Sprintf("create database if not exists %s", r.dbname)); err != nil {
		logger.WithError(err).Errorf("create database %s error %v", r.dbname, err)
		panic(err)
	}
}

func (r *Reporter) creatTables() {
	ctx := context.Background()
	conn, err := db.NewConnectorWithDb(r.username, r.password, r.host, r.port, r.dbname)
	if err != nil {
		logger.WithError(err).Errorf("connect to database error")
		return
	}
	defer r.closeConn(conn)

	for _, createSql := range createList {
		logger.Infof("execute sql: %s", createSql)
		if _, err = conn.Exec(ctx, createSql); err != nil {
			logger.Errorf("execute sql: %s, error: %s", createSql, err)
		}
	}
}

func (r *Reporter) closeConn(conn *db.Connector) {
	if err := conn.Close(); err != nil {
		logger.WithError(err).Errorf("close connection error")
	}
}

func (r *Reporter) handlerFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		r.recordTotalRep()
		// data parse
		data, err := c.GetRawData()
		if err != nil {
			logger.WithError(err).Errorf("receiving taosd data error")
			return
		}
		var report Report
		logger.Tracef("report data: %s", string(data))
		if e := json.Unmarshal(data, &report); e != nil {
			logger.WithError(e).Errorf("error occurred while unmarshal request data: %s ", data)
			return
		}
		var sqls []string
		if report.ClusterInfo != nil {
			sqls = append(sqls, insertClusterInfoSql(*report.ClusterInfo, report.ClusterID, report.Protocol, report.Ts)...)
		}
		sqls = append(sqls, insertDnodeSql(report.DnodeInfo, report.DnodeID, report.DnodeEp, report.ClusterID, report.Ts))
		if report.GrantInfo != nil {
			sqls = append(sqls, insertGrantSql(*report.GrantInfo, report.DnodeID, report.DnodeEp, report.ClusterID, report.Ts))
		}
		sqls = append(sqls, insertDataDirSql(report.DiskInfos, report.DnodeID, report.DnodeEp, report.ClusterID, report.Ts)...)
		for _, group := range report.VgroupInfos {
			sqls = append(sqls, insertVgroupSql(group, report.DnodeID, report.DnodeEp, report.ClusterID, report.Ts)...)
		}
		sqls = append(sqls, insertLogSql(report.LogInfos, report.DnodeID, report.DnodeEp, report.ClusterID, report.Ts)...)

		conn, err := db.NewConnectorWithDb(r.username, r.password, r.host, r.port, r.dbname)
		if err != nil {
			logger.WithError(err).Errorf("connect to database error")
			return
		}
		defer r.closeConn(conn)
		ctx := context.Background()

		for _, sql := range sqls {
			logger.Tracef("execute sql %s", sql)
			if _, err := conn.Exec(ctx, sql); err != nil {
				logger.WithError(err).Errorf("execute sql : %s", sql)
			}
		}
	}
}

func (r *Reporter) recordTotalRep() {
	old := r.totalRep.Load().(int)
	for i := 0; i < 3; i++ {
		r.totalRep.CompareAndSwap(old, old+1)
	}
}

func (r *Reporter) GetTotalRep() *atomic.Value {
	return &r.totalRep
}

func insertClusterInfoSql(info ClusterInfo, ClusterID string, protocol int, ts string) []string {
	var sqls []string
	var dtotal, dalive, mtotal, malive int
	for _, dnode := range info.Dnodes {
		sqls = append(sqls, fmt.Sprintf("insert into d_info_%s using d_info tags (%d, '%s', '%s') values ('%s', '%s')",
			ClusterID+strconv.Itoa(dnode.DnodeID), dnode.DnodeID, dnode.DnodeEp, ClusterID, ts, dnode.Status))
		dtotal++
		if "ready" == dnode.Status {
			dalive++
		}
	}

	for _, mnode := range info.Mnodes {
		sqls = append(sqls, fmt.Sprintf("insert into m_info_%s using m_info tags (%d, '%s', '%s') values ('%s', '%s')",
			ClusterID+strconv.Itoa(mnode.MnodeID), mnode.MnodeID, mnode.MnodeEp, ClusterID, ts, mnode.Role))
		mtotal++
		//LEADER FOLLOWER CANDIDATE ERROR
		if "ERROR" != mnode.Role {
			malive++
		}
	}

	sqls = append(sqls, fmt.Sprintf(
		"insert into cluster_info_%s using cluster_info tags('%s') (ts, first_ep, first_ep_dnode_id, version, "+
			"master_uptime, monitor_interval, dbs_total, tbs_total, stbs_total, dnodes_total, dnodes_alive, mnodes_total, "+
			"mnodes_alive, vgroups_total, vgroups_alive, vnodes_total, vnodes_alive, connections_total, protocol) "+
			"values ('%s', '%s', %d, '%s', %f, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d)",
		ClusterID, ClusterID, ts, info.FirstEp, info.FirstEpDnodeID, info.Version, info.MasterUptime, info.MonitorInterval, info.DbsTotal, info.TbsTotal, info.StbsTotal,
		dtotal, dalive, mtotal, malive, info.VgroupsTotal, info.VgroupsAlive, info.VnodesTotal, info.VnodesAlive, info.ConnectionsTotal, protocol))
	return sqls
}

func insertDnodeSql(info DnodeInfo, DnodeID int, DnodeEp string, ClusterID string, ts string) string {
	return fmt.Sprintf("insert into dnode_info_%s using dnodes_info tags (%d, '%s', '%s') values ('%s', %f, %f, %f, %f, %d, %d, %d, %d, %d, %d, %f, %f, %f, %f, %f, %f, %d, %f, %d, %d, %f, %d, %d, %f, %d, %d, %d, %d, %d, %d, %d)",
		ClusterID+strconv.Itoa(DnodeID), DnodeID, DnodeEp, ClusterID,
		ts, info.Uptime, info.CPUEngine, info.CPUSystem, info.CPUCores, info.MemEngine, info.MemSystem, info.MemTotal,
		info.DiskEngine, info.DiskUsed, info.DiskTotal, info.NetIn, info.NetOut, info.IoRead, info.IoWrite,
		info.IoReadDisk, info.IoWriteDisk, info.ReqSelect, info.ReqSelectRate, info.ReqInsert, info.ReqInsertSuccess,
		info.ReqInsertRate, info.ReqInsertBatch, info.ReqInsertBatchSuccess, info.ReqInsertBatchRate, info.Errors,
		info.VnodesNum, info.Masters, info.HasMnode, info.HasQnode, info.HasSnode, info.HasBnode)
}

func insertDataDirSql(disk DiskInfo, DnodeID int, DnodeEp string, ClusterID string, ts string) []string {
	var sqls []string
	for _, data := range disk.Datadir {
		sqls = append(sqls,
			fmt.Sprintf("insert into data_dir_%s using data_dir tags (%d, '%s', '%s') values ('%s', '%s', %d, %d, %d, %d)",
				ClusterID+strconv.Itoa(DnodeID), DnodeID, DnodeEp, ClusterID,
				ts, data.Name, data.Level, data.Avail.IntPart(), data.Used.IntPart(), data.Total.IntPart()),
		)
	}
	sqls = append(sqls,
		fmt.Sprintf("insert into log_dir_%s using log_dir tags (%d, '%s', '%s') values ('%s', '%s', %d, %d, %d)",
			ClusterID+strconv.Itoa(DnodeID), DnodeID, DnodeEp, ClusterID,
			ts, disk.Logdir.Name, disk.Logdir.Avail.IntPart(), disk.Logdir.Used.IntPart(), disk.Logdir.Total.IntPart()),
		fmt.Sprintf("insert into temp_dir_%s using temp_dir tags (%d, '%s', '%s') values ('%s', '%s', %d, %d, %d)",
			ClusterID+strconv.Itoa(DnodeID), DnodeID, DnodeEp, ClusterID,
			ts, disk.Tempdir.Name, disk.Tempdir.Avail.IntPart(), disk.Tempdir.Used.IntPart(), disk.Tempdir.Total.IntPart()),
	)
	return sqls
}

func insertVgroupSql(g VgroupInfo, DnodeID int, DnodeEp string, ClusterID string, ts string) []string {
	var sqls []string
	sqls = append(sqls, fmt.Sprintf("insert into vgroups_info_%s using vgroups_info tags (%d, '%s', '%s') "+
		"(ts, vgroup_id, database_name, tables_num, status, ) values ( '%s','%d', '%s', %d, '%s')",
		ClusterID+strconv.Itoa(DnodeID), DnodeID, DnodeEp, ClusterID,
		ts, g.VgroupID, g.DatabaseName, g.TablesNum, g.Status))
	for _, v := range g.Vnodes {
		sqls = append(sqls, fmt.Sprintf("insert into vnodes_role_%s using vnodes_role tags (%d, '%s', '%s') values ('%s', '%s')",
			ClusterID+strconv.Itoa(DnodeID), DnodeID, DnodeEp, ClusterID, ts, v.VnodeRole))
	}
	return sqls
}

func insertLogSql(log LogInfo, DnodeID int, DnodeEp string, ClusterID string, ts string) []string {
	var sqls []string
	for _, l := range log.Logs {
		sqls = append(sqls, fmt.Sprintf("insert into logs_%s using logs tags (%d, '%s', '%s') values ('%s', '%s', '%s')",
			ClusterID+strconv.Itoa(DnodeID), DnodeID, DnodeEp, ClusterID, l.Ts, l.Level, l.Content))
	}
	var e, info, debug, trace int
	for _, s := range log.Summary {
		switch s.Level {
		case "error":
			e = s.Total
		case "info":
			info = s.Total
		case "debug":
			debug = s.Total
		case "trace":
			trace = s.Total
		}
	}
	sqls = append(sqls, fmt.Sprintf("insert into log_summary_%s using log_summary tags (%d, '%s', '%s') values ('%s', %d, %d, %d, %d)",
		ClusterID+strconv.Itoa(DnodeID), DnodeID, DnodeEp, ClusterID, ts, e, info, debug, trace))
	return sqls
}

func insertGrantSql(g GrantInfo, DnodeID int, DnodeEp string, ClusterID string, ts string) string {
	return fmt.Sprintf("insert into grants_info_%s using grants_info tags (%d, '%s', '%s') (ts, expire_time, "+
		"timeseries_used, timeseries_total) values ('%s', %d, %d, %d)", ClusterID+strconv.Itoa(DnodeID), DnodeID,
		DnodeEp, ClusterID, ts, g.ExpireTime, g.TimeseriesUsed, g.TimeseriesTotal)
}
