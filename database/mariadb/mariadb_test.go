package mariadb

import (
	"testing"
	"time"

	"github.com/devops-works/slowql/query"
	"github.com/devops-works/slowql/server"
)

// parseTime is a helper function that allow us to cast a string into a time.Time
// value in the tests
func parseTime(t string) time.Time {
	time, _ := time.Parse("060102 15:04:05", t)
	return time
}
func TestDatabase_parseMariaDBHeader(t *testing.T) {
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
				line: "# Time: 210323 11:31:57",
			},
			refQuery: query.Query{
				Time: parseTime("210323 11:31:57"),
			},
		},
		{
			name: "user, host",
			args: args{
				line: "# User@Host: hugo[hugo] @  [172.18.0.3]",
			},
			refQuery: query.Query{
				User: "hugo",
				Host: "172.18.0.3",
			},
		},
		{
			name: "id, schema, QC hit",
			args: args{
				line: "# Thread_id: 12794  Schema:   QC_hit: No",
			},
			refQuery: query.Query{
				ID:     12794,
				Schema: "",
				QCHit:  false,
			},
		},
		{
			name: "query time, lock time, rows sent, rows examined",
			args: args{
				line: "# Query_time: 0.000035  Lock_time: 0.000000  Rows_sent: 0  Rows_examined: 0",
			},
			refQuery: query.Query{
				QueryTime:    0.000035,
				LockTime:     0.000000,
				RowsSent:     0,
				RowsExamined: 0,
			},
		},
		{
			name: "rows affected, bytes sent",
			args: args{
				line: "# Rows_affected: 0  Bytes_sent: 11",
			},
			refQuery: query.Query{
				RowsAffected: 0,
				BytesSent:    11,
			},
		},
	}
	for _, tt := range tests {
		db := New(nil)
		t.Run(tt.name, func(t *testing.T) {
			q := query.Query{}
			db.parseMariaDBHeader(tt.args.line, &q)
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
			lines: []string{
				"/opt/bitnami/mariadb/sbin/mysqld, Version: 10.5.9-MariaDB (Source distribution). started with:",
				"Tcp port: 3306  Unix socket: /opt/bitnami/mariadb/tmp/mysql.sock",
				"Time		    Id Command	Argument",
			},
			refSrv: server.Server{
				Binary:             "/opt/bitnami/mariadb/sbin/mysqld",
				Port:               3306,
				Socket:             "/opt/bitnami/mariadb/tmp/mysql.sock",
				Version:            "10.5.9-MariaDB",
				VersionShort:       "10.5.9",
				VersionDescription: "Source distribution",
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

func TestDatabase_ParseBlocs(t *testing.T) {
	tests := []struct {
		name     string
		bloc     []string
		refQuery query.Query
	}{
		{
			name: "testing",
			bloc: []string{
				"# Time: 210323 11:31:57",
				"# User@Host: hugo[hugo] @  [172.18.0.3]",
				"# Thread_id: 12794  Schema:   QC_hit: No",
				"# Query_time: 0.000035  Lock_time: 0.000000  Rows_sent: 0  Rows_examined: 0",
				"# Rows_affected: 0  Bytes_sent: 11",
				"SELECT col1 AS c1",
				"FROM table1 AS t1;",
			},
			refQuery: query.Query{
				Time:         parseTime("210323 11:31:57"),
				User:         "hugo",
				Host:         "172.18.0.3",
				ID:           12794,
				Schema:       "",
				QueryTime:    0.000035,
				LockTime:     0.000000,
				RowsSent:     0,
				RowsExamined: 0,
				RowsAffected: 0,
				BytesSent:    11,
				Query:        "SELECT col1 AS c1 FROM table1 AS t1;",
				QCHit:        false,
			},
		}, {
			name: "testing",
			bloc: []string{
				"# Time: 210323 11:31:57",
				"# User@Host: hugo[hugo] @  [172.18.0.3]",
				"# Thread_id: 12794  Schema:   QC_hit: No",
				"# Query_time: 0.000035  Lock_time: 0.000000  Rows_sent: 0  Rows_examined: 0",
				"# Rows_affected: 0  Bytes_sent: 11",
				"SET timestamp=1616499117;",
				"SET NAMES utf8mb4;",
			},
			refQuery: query.Query{
				Time:         parseTime("210323 11:31:57"),
				User:         "hugo",
				Host:         "172.18.0.3",
				ID:           12794,
				Schema:       "",
				QueryTime:    0.000035,
				LockTime:     0.000000,
				RowsSent:     0,
				RowsExamined: 0,
				RowsAffected: 0,
				BytesSent:    11,
				Query:        "SET timestamp=1616499117;SET NAMES utf8mb4;",
				QCHit:        false,
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
