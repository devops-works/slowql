package mysql

import (
	"testing"
	"time"

	"github.com/devops-works/slowql/query"
	"github.com/devops-works/slowql/server"
)

// parseTime is a helper function that allow us to cast a string into a time.Time
// value in the tests
func parseTime(t string) time.Time {
	time, _ := time.Parse(time.RFC3339, t)
	return time
}
func TestDatabase_parseMySQLHeader(t *testing.T) {
	type args struct {
		line string
	}
	tests := []struct {
		name     string
		args     args
		refQuery query.Query
	}{
		{
			name: "time",
			args: args{
				line: "# Time: 2021-03-23T14:38:32.489447Z",
			},
			refQuery: query.Query{
				Time: parseTime("2021-03-23T14:38:32.489447Z"),
			},
		},
		{
			name: "user, host, id",
			args: args{
				line: "# User@Host: root[root] @  [172.18.0.1]  Id:     9",
			},
			refQuery: query.Query{
				User: "root",
				Host: "172.18.0.1",
				ID:   9,
			},
		},
		{
			name: "query time, lock time, rows sent, rows examined, rows affected",
			args: args{
				line: "# Query_time: 0.000328  Lock_time: 0.000013  Rows_sent: 1  Rows_examined: 1  Rows_affected: 0",
			},
			refQuery: query.Query{
				QueryTime:    0.000328,
				LockTime:     0.000013,
				RowsSent:     1,
				RowsExamined: 1,
				RowsAffected: 0,
			},
		},
		{
			name: "schema, last errno, killed",
			args: args{
				line: "# Schema: client-prod  Last_errno: 1  Killed: 2",
			},
			refQuery: query.Query{
				Schema:    "client-prod",
				LastErrNo: 1,
				Killed:    2,
			},
		},
		{
			name: "bytes sent",
			args: args{
				line: "# Bytes_sent: 1337",
			},
			refQuery: query.Query{
				BytesSent: 1337,
			},
		},
	}
	for _, tt := range tests {
		db := New(nil)
		t.Run(tt.name, func(t *testing.T) {
			q := query.Query{}
			db.parseMySQLHeader(tt.args.line, &q)
			if q != tt.refQuery {
				t.Errorf("got = %v, want %v", q, tt.refQuery)
			}
		})
	}
}

func TestDatabase_ParseServerMeta(t *testing.T) {
	tests := []struct {
		name   string
		lines  []string
		refSrv server.Server
	}{
		{
			name: "parsable",
			lines: []string{"/usr/sbin/mysqld, Version: 8.0.23 (MySQL Community Server - GPL). started with:",
				"Tcp port: 3306  Unix socket: /var/run/mysqld/mysqld.sock",
				"Time                 Id Command    Argument"},
			refSrv: server.Server{
				Binary:             "/usr/sbin/mysqld",
				Port:               3306,
				Socket:             "/var/run/mysqld/mysqld.sock",
				Version:            "8.0.23",
				VersionShort:       "8.0.2",
				VersionDescription: "MySQL Community Server - GPL",
			},
		},
		{
			name: "unparsable",
			lines: []string{"Version: 8.0.23 (MySQL Community Server - GPL). started with:",
				"Tcp port: 3306",
				"Time                 Id Command    Argument"},
			refSrv: server.Server{
				Binary:             "unable to parse line",
				Port:               0,
				Socket:             "unable to parse line",
				Version:            "unable to parse line",
				VersionShort:       "unable to parse line",
				VersionDescription: "unable to parse line",
			},
		},
	}
	for _, tt := range tests {
		lines := make(chan []string, 2)
		t.Run(tt.name, func(t *testing.T) {
			db := New(nil)
			lines <- tt.lines
			db.ParseServerMeta(lines)
			if db.srv != tt.refSrv {
				t.Errorf("got = %v, want = %v", db.srv, tt.refSrv)
			}
		})
	}
}

func TestDatabase_ParseBlocks(t *testing.T) {
	tests := []struct {
		name     string
		bloc     []string
		refQuery query.Query
	}{
		{
			name: "testing",
			bloc: []string{
				"# Time: 2020-07-07T12:28:02.804900Z",
				"# User@Host: api[api] @  [192.168.0.101]  Id: 5603761",
				"# Schema: client-prod  Last_errno: 0  Killed: 0",
				"# Query_time: 0.000089  Lock_time: 0.000000  Rows_sent: 0  Rows_examined: 0  Rows_affected: 0",
				"# Bytes_sent: 1183",
				"SET timestamp=1594124882;",
			},
			refQuery: query.Query{
				Time:         parseTime("2020-07-07T12:28:02.804900Z"),
				User:         "api",
				Host:         "192.168.0.101",
				ID:           5603761,
				Schema:       "client-prod",
				LastErrNo:    0,
				Killed:       0,
				QueryTime:    0.000089,
				LockTime:     0.000000,
				RowsSent:     0,
				RowsExamined: 0,
				RowsAffected: 0,
				BytesSent:    1183,
				Query:        "SET timestamp=1594124882;",
			},
		},
	}
	for _, tt := range tests {
		rawBlocs := make(chan []string, 10)
		qc := make(chan query.Query)
		db := New(qc)
		t.Run(tt.name, func(t *testing.T) {
			rawBlocs <- tt.bloc
			go db.ParseBlocks(rawBlocs)
			q := <-db.WaitingList
			if q != tt.refQuery {
				t.Errorf("got = %v, want = %v", q, tt.refQuery)
			}
		})
	}
}

func BenchmarkParseBlocks(b *testing.B) {
	blocks := []string{`SELECT col1 AS c1`, `FROM table1 AS t1;`}
	db := New(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.parseQuery(blocks)
	}
}