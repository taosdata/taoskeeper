package api

import (
	"context"
	"fmt"
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

	longDetails := strings.Repeat("0123456789", 5000)

	cases := []struct {
		name   string
		ts     int64
		detail string
		data   string
		expect string
	}{
		{
			name:   "1",
			ts:     1699839716440000000,
			data:   `{"timestamp": "1699839716440000000", "cluster_id": "cluster_id", "user": "user", "operation": "operation", "db":"dbnamea", "resource":"resourcenamea", "client_add": "localhost:30000", "details": "detail"}`,
			expect: "detail",
		},
		{
			name:   "2",
			ts:     1699839716441000000,
			data:   `{"timestamp": "1699839716441000000", "cluster_id": "cluster_id", "user": "user", "operation": "operation", "db":"dbnamea", "resource":"resourcenamea", "client_add": "localhost:30000", "details": "` + longDetails + `"}`,
			expect: longDetails[:50000],
		},
		{
			name:   "3",
			ts:     1699839716442000000,
			data:   "{\"timestamp\": \"1699839716442000000\", \"cluster_id\": \"cluster_id\", \"user\": \"user\", \"operation\": \"operation\", \"db\":\"dbnameb\", \"resource\":\"resourcenameb\", \"client_add\": \"localhost:30000\", \"details\": \"create database `meter` buffer 32 cachemodel 'none' duration 50d keep 3650d single_stable 0 wal_retention_period 3600 precision 'ms'\"}",
			expect: "create database `meter` buffer 32 cachemodel 'none' duration 50d keep 3650d single_stable 0 wal_retention_period 3600 precision 'ms'",
		},
	}

	conn, err := db.NewConnectorWithDb(c.TDengine.Username, c.TDengine.Password, c.TDengine.Host, c.TDengine.Port, c.Audit.Database.Name)
	assert.NoError(t, err)
	defer func() {
		_, _ = conn.Query(context.Background(), "drop stable if exists audit.operations")
	}()

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			body := strings.NewReader(c.data)
			req, _ := http.NewRequest(http.MethodPost, "/audit", body)
			router.ServeHTTP(w, req)
			assert.Equal(t, 200, w.Code)

			data, err := conn.Query(context.Background(), fmt.Sprintf("select ts, details from audit.operations where ts=%d", c.ts))
			assert.NoError(t, err)
			assert.Equal(t, 1, len(data.Data))
			assert.Equal(t, c.expect, data.Data[0][1])
		})
	}
}

func TestAuditBatch(t *testing.T) {
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

	input := `{"records": [{"timestamp": "1699839716440000000", "cluster_id": "cluster_id_batch", "user": "user", "operation": "operation", "db":"dbnamea", "resource":"resourcenamea", "client_add": "localhost:30000", "details": "detail"},` +
		`{"timestamp": "1699839716441000000", "cluster_id": "cluster_id_batch", "user": "user", "operation": "operation", "db":"dbnamea", "resource":"resourcenamea", "client_add": "localhost:30000", "details": "detail"}]}`

	conn, err := db.NewConnectorWithDb(c.TDengine.Username, c.TDengine.Password, c.TDengine.Host, c.TDengine.Port, c.Audit.Database.Name)
	assert.NoError(t, err)
	defer func() {
		_, _ = conn.Query(context.Background(), "drop stable if exists audit.operations")
	}()

	t.Run("testbatch", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := strings.NewReader(input)
		req, _ := http.NewRequest(http.MethodPost, "/audit-batch", body)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)

		data, err := conn.Query(context.Background(), "select ts, details from audit.operations where cluster_id='cluster_id_batch'")
		assert.NoError(t, err)
		assert.Equal(t, 2, len(data.Data))
	})

}
