package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/taosdata/driver-go/v3/taosRestful"
)

type Connector struct {
	db *sql.DB
}

type Data struct {
	Head []string        `json:"head"`
	Data [][]interface{} `json:"data"`
}

func NewConnector(username, password, host string, port int, usessl bool) (*Connector, error) {
	var protocol string
	if usessl {
		protocol = "https"
	} else {
		protocol = "http"
	}
	db, err := sql.Open("taosRestful", fmt.Sprintf("%s:%s@%s(%s:%d)/?skipVerify=true", username, password, protocol, host, port))
	if err != nil {
		return nil, err
	}
	return &Connector{db: db}, nil
}

func NewConnectorWithDb(username, password, host string, port int, dbname string, usessl bool) (*Connector, error) {
	var protocol string
	if usessl {
		protocol = "https"
	} else {
		protocol = "http"
	}
	db, err := sql.Open("taosRestful", fmt.Sprintf("%s:%s@%s(%s:%d)/%s?skipVerify=true", username, password, protocol, host, port, dbname))
	if err != nil {
		return nil, err
	}
	return &Connector{db: db}, nil
}

func (c *Connector) Exec(ctx context.Context, sql string) (int64, error) {
	res, err := c.db.ExecContext(ctx, sql)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (c *Connector) Query(ctx context.Context, sql string) (*Data, error) {
	rows, err := c.db.QueryContext(ctx, sql)
	if err != nil {
		return nil, err
	}
	data := &Data{}
	data.Head, err = rows.Columns()
	columnCount := len(data.Head)
	if err != nil {
		return nil, err
	}
	scanData := make([]interface{}, columnCount)
	for rows.Next() {
		tmp := make([]interface{}, columnCount)
		for i := 0; i < columnCount; i++ {
			scanData[i] = &tmp[i]
		}
		err = rows.Scan(scanData...)
		if err != nil {
			rows.Close()
			return nil, err
		}
		data.Data = append(data.Data, tmp)
	}
	return data, nil
}

func (c *Connector) Close() error {
	return c.db.Close()
}
