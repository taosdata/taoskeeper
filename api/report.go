package api

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/taosdata/go-utils/json"
	"github.com/taosdata/taoskeeper/db"
	"github.com/taosdata/taoskeeper/infrastructure/log"
	"strconv"
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

func (r *Report) Init(c gin.IRouter) {
	c.POST("report", handler)
	creatTables()
}

func creatTables() {
	conn, err := db.NewConnectorWithDb()
	if err != nil {
		logger.WithError(err).Errorf("connect to database error")
		return
	}
	defer closeConn(conn)

	ctx := context.Background()
	for i := 0; i < len(createList); i++ {
		logger.Infof("execute sql: %s", createList[i])
		if _, err = conn.Exec(ctx, createList[i]); err != nil {
			logger.Errorf("execute sql: %s, error: %s", createList[i], err)
		}
	}
}

func closeConn(conn *db.Connector) {
	if err := conn.Close(); err != nil {
		logger.WithError(err).Errorf("close connection error")
	}
}

var TotalRep int32

func handler(c *gin.Context) {
	TotalRep++
	// data parse
	data, err := c.GetRawData()
	if err != nil {
		logger.WithError(err).Errorf("receiving taosd data error")
		return
	}
	r := Report{}
	if e := json.Unmarshal(data, &r); e != nil {
		logger.WithError(e).Errorf("error occurred while unmarshal request data: %s ", data)
		return
	}
	var sqls []string
	sqls = append(sqls, insertClusterInfoSql(r.ClusterInfo, r.ClusterID, r.Protocol, r.Ts)...)
	sqls = append(sqls, insertDnodeSql(r.DnodeInfo, r.DnodeID, r.DnodeEp, r.ClusterID, r.Ts),
		insertGrantSql(r.GrantInfo, r.DnodeID, r.DnodeEp, r.ClusterID, r.Ts))
	sqls = append(sqls, insertDataDirSql(r.DiskInfos, r.DnodeID, r.DnodeEp, r.ClusterID, r.Ts)...)
	for _, group := range r.VgroupInfos {
		sqls = append(sqls, insertVgroupSql(group, r.DnodeID, r.DnodeEp, r.ClusterID, r.Ts)...)
	}
	sqls = append(sqls, insertLogSql(r.LogInfos, r.DnodeID, r.DnodeEp, r.ClusterID, r.Ts)...)

	conn, err := db.NewConnectorWithDb()
	if err != nil {
		logger.WithError(err).Errorf("connect to database error")
		return
	}
	defer closeConn(conn)
	ctx := context.Background()

	for _, sql := range sqls {
		logger.Tracef("execute sql %s", sql)
		if _, err := conn.Exec(ctx, sql); err != nil {
			logger.WithError(err).Errorf("execute sql : %s", sql)
		}
	}
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
		if "unsynced" != mnode.Role {
			malive++
		}
	}

	sqls = append(sqls, fmt.Sprintf("insert into cluster_info_%s using cluster_info tags('%s') values ('%s', '%s', %d, '%s', %f, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d, %d)",
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
				ts, data.Name, data.Level, data.Avail, data.Used, data.Total),
		)
	}
	sqls = append(sqls,
		fmt.Sprintf("insert into log_dir_%s using log_dir tags (%d, '%s', '%s') values ('%s', '%s', %d, %d, %d)",
			ClusterID+strconv.Itoa(DnodeID), DnodeID, DnodeEp, ClusterID,
			ts, disk.Logdir.Name, disk.Logdir.Avail, disk.Logdir.Used, disk.Logdir.Total),
		fmt.Sprintf("insert into temp_dir_%s using temp_dir tags (%d, '%s', '%s') values ('%s', '%s', %d, %d, %d)",
			ClusterID+strconv.Itoa(DnodeID), DnodeID, DnodeEp, ClusterID,
			ts, disk.Tempdir.Name, disk.Tempdir.Avail, disk.Tempdir.Used, disk.Tempdir.Total),
	)
	return sqls
}

func insertVgroupSql(g VgroupInfo, DnodeID int, DnodeEp string, ClusterID string, ts string) []string {
	var sqls []string
	sqls = append(sqls, fmt.Sprintf("insert into vgroups_info_%s using vgroups_info tags (%d, '%s', '%s') values ( '%s','%d', '%s', %d, '%s')",
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
	return fmt.Sprintf("insert into grants_info_%s using grants_info tags (%d, '%s', '%s') values ('%s', %d, %d, %d)",
		ClusterID+strconv.Itoa(DnodeID), DnodeID, DnodeEp, ClusterID,
		ts, g.ExpireTime, g.TimeseriesUsed, g.TimeseriesTotal)
}
