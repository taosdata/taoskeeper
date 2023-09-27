package api

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/taosdata/taoskeeper/db"
	"github.com/taosdata/taoskeeper/infrastructure/config"
	"github.com/taosdata/taoskeeper/infrastructure/log"
)

var auditLogger = log.GetLogger("audit")

type Audit struct {
	username  string
	password  string
	host      string
	port      int
	conn      *db.Connector
	db        string
	dbOptions map[string]interface{}
}

type AuditInfo struct {
	Timestamp int64  `json:"timestamp"`
	ClusterID string `json:"cluster_id"`
	User      string `json:"user"`
	Operation string `json:"operation"`
	Target1   string `json:"target_1"`
	Target2   string `json:"target_2"`
	Details   string `json:"details"`
}

func NewAudit(c *config.Config) (*Audit, error) {
	a := Audit{
		username:  c.TDengine.Username,
		password:  c.TDengine.Password,
		host:      c.TDengine.Host,
		port:      c.TDengine.Port,
		db:        c.Audit.Database.Name,
		dbOptions: c.Audit.Database.Options,
	}
	if a.db == "" {
		a.db = "audit"
	}
	return &a, nil
}

func (a *Audit) Init(c gin.IRouter) error {
	if err := a.createDatabase(); err != nil {
		return fmt.Errorf("create database error: %s", err)
	}
	if err := a.initConnect(); err != nil {
		return fmt.Errorf("init db connect error: %s", err)
	}
	if err := a.createSTables(); err != nil {
		return fmt.Errorf("create stable error: %s", err)
	}
	c.POST("/audit", a.handleFunc())
	return nil
}

func (a *Audit) handleFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.conn == nil {
			auditLogger.Error("no connection")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "no connection"})
			return
		}

		data, err := c.GetRawData()
		if err != nil {
			auditLogger.WithError(err).Errorf("## get audit data error")
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("get audit data error. %s", err)})
			return
		}
		auditLogger.Trace("## receive audit data", "data", string(data))

		var audit AuditInfo
		if err := json.Unmarshal(data, &audit); err != nil {
			auditLogger.WithError(err).Errorf("## parse audit data %s error", string(data))
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("parse audit data error: %s", err)})
			return
		}

		sql := parseSql(audit)
		if _, err = a.conn.Exec(context.Background(), sql); err != nil {
			auditLogger.WithError(err).Error("##save audit data error", "sql", sql)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("save audit data error: %s", err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{})
	}
}

func parseSql(audit AuditInfo) string {
	return fmt.Sprintf(
		"insert into %s using operations tags ('%s', '%s', '%s') values (%d, '%s', '%s', '%s')",
		getTableName(audit), audit.Target1, audit.Target2, audit.ClusterID, audit.Timestamp, audit.User, audit.Operation, audit.Details)
}

func getTableName(audit AuditInfo) string {
	// table name is md5(user + operation + target_1 + target_2 + cluster_id)
	sum := md5.Sum([]byte(fmt.Sprintf("%s%s%s%s%s", audit.User, audit.Operation, audit.Target1, audit.Target2,
		audit.ClusterID)))
	return fmt.Sprintf("t_%s", hex.EncodeToString(sum[:]))
}

func (a *Audit) initConnect() error {
	conn, err := db.NewConnectorWithDb(a.username, a.password, a.host, a.port, a.db)
	if err != nil {
		auditLogger.Error("## init db connect error", "error", err)
		return err
	}
	a.conn = conn
	return nil
}

func (a *Audit) createDatabase() error {
	conn, err := db.NewConnector(a.username, a.password, a.host, a.port)
	if err != nil {
		return fmt.Errorf("connect to database error: %s", err)
	}
	defer func() { _ = conn.Close() }()
	sql := a.createDBSql()
	auditLogger.Info("## create database", "sql", sql)
	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		auditLogger.Error("## create database error", "error", err)
		return err
	}

	return err
}

var noConnectionError = errors.New("no connection")

func (a *Audit) createDBSql() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("create database if not exists %s ", a.db))

	for k, v := range a.dbOptions {
		buf.WriteString(k)
		switch v := v.(type) {
		case string:
			buf.WriteString(fmt.Sprintf(" '%s'", v))
		default:
			buf.WriteString(fmt.Sprintf(" %v", v))
		}
		buf.WriteString(" ")
	}

	return buf.String()
}

var createTableSql = "create stable if not exists operations " +
	"(ts timestamp, user_name varchar(25), operation varchar(20), details varchar(65472)) " +
	"tags (target_1 varchar(100), target_2 varchar(300), cluster_id varchar(32))"

func (a *Audit) createSTables() error {
	if a.conn == nil {
		return noConnectionError
	}
	_, err := a.conn.Exec(context.Background(), createTableSql)
	if err != nil {
		auditLogger.Error("## create stable error ", "error", err)
	}
	return err
}
