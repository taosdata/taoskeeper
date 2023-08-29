package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/taosdata/taoskeeper/db"
	"github.com/taosdata/taoskeeper/infrastructure/config"
)

func TestAudit(t *testing.T) {
	c := &config.Config{
		Port: 6043,
		TDengine: config.TDengineRestful{
			Host:     "127.0.0.1",
			Port:     6041,
			Username: "root",
			Password: "taosdata",
		},
		Audit: config.AuditConfig{
			Database: config.Database{
				Name: "audit",
			},
		},
	}
	a, err := NewAudit(c)
	assert.NoError(t, err)
	err = a.Init(router)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"timestamp": 1692840000000, "cluster_id": "cluster_id", "user": "user", "operation": "operation", "target_1": "target_1", "target_2": "target_2", "details": "detail"}`)
	req, _ := http.NewRequest(http.MethodPost, "/audit", body)
	router.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	conn, err := db.NewConnectorWithDb(c.TDengine.Username, c.TDengine.Password, c.TDengine.Host, c.TDengine.Port, c.Audit.Database.Name)
	assert.NoError(t, err)
	data, err := conn.Query(context.Background(), "select ts, user_name from audit.operations")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(data.Data))
	assert.Equal(t, "user", data.Data[0][1])
}
