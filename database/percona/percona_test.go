package percona

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
				line: "# Time: 2022-06-22T14:25:08.796525Z",
			},
			refQuery: query.Query{
				Time: parseTime("2022-06-22T14:25:08.796525Z"),
			},
		},
		{
			name: "user, host, id",
			args: args{
				line: "# User@Host: user @  [127.0.0.1]  Id: 498200077",
			},
			refQuery: query.Query{
				User: "user",
				Host: "127.0.0.1",
				ID:   498200077,
			},
		},
		{
			name: "query time, lock time, rows sent, rows examined, rows affected",
			args: args{
				line: "# Query_time: 5.390275  Lock_time: 0.000388  Rows_sent: 464  Rows_examined: 2057052  Rows_affected: 0",
			},
			refQuery: query.Query{
				QueryTime:    5.390275,
				LockTime:     0.000388,
				RowsSent:     464,
				RowsExamined: 2057052,
				RowsAffected: 0,
			},
		},
		{
			name: "schema, last errno, killed",
			args: args{
				line: "# Schema: schema_name  Last_errno: 1  Killed: 2",
			},
			refQuery: query.Query{
				Schema:    "schema_name",
				LastErrNo: 1,
				Killed:    2,
			},
		},
		{
			name: "bytes sent",
			args: args{
				line: "# Bytes_sent: 17781  Tmp_tables: 3  Tmp_disk_tables: 1  Tmp_table_sizes: 1060944",
			},
			refQuery: query.Query{
				BytesSent: 17781,
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
			lines: []string{"/usr/sbin/mysqld, Version: 5.7.29-32-log (Percona Server (GPL), Release 32, Revision 56bce88). started with:",
				"Tcp port: 3306  Unix socket: /tmp/mysql.sock",
				"Time                 Id Command    Argument"},
			refSrv: server.Server{
				Binary:             "/usr/sbin/mysqld",
				Port:               3306,
				Socket:             "/tmp/mysql.sock",
				Version:            "5.7.29-32-log",
				VersionShort:       "5.7.29",
				VersionDescription: "Percona Server (GPL), Release 32, Revision 56bce88",
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
				"# Time: 2022-06-22T14:25:08.796525Z",
				"# User@Host: user @  [127.0.0.1]  Id: 498200077",
				"# Schema: schema_name  Last_errno: 1  Killed: 2",
				"# Query_time: 5.390275  Lock_time: 0.000388  Rows_sent: 464  Rows_examined: 2057052  Rows_affected: 0",
				"# Bytes_sent: 17781  Tmp_tables: 3  Tmp_disk_tables: 1  Tmp_table_sizes: 1060944",
				"# InnoDB_trx_id: 0",
				"# QC_Hit: No  Full_scan: Yes  Full_join: No  Tmp_table: Yes  Tmp_table_on_disk: Yes",
				"# Filesort: Yes  Filesort_on_disk: No  Merge_passes: 0",
				"#   InnoDB_IO_r_ops: 0  InnoDB_:IO_r_bytes: 0  InnoDB_IO_r_wait: 0.000000",
				"#   InnoDB_rec_lock_wait: 0.000000  InnoDB_queue_wait: 0.000000",
				"#   InnoDB_pages_distinct: 8191",
				"SELECT * FROM some_table;",
			},
			refQuery: query.Query{
				Time:         parseTime("2022-06-22T14:25:08.796525Z"),
				User:         "user",
				Host:         "127.0.0.1",
				ID:           498200077,
				Schema:       "schema_name",
				LastErrNo:    1,
				Killed:       2,
				QueryTime:    5.390275,
				LockTime:     0.000388,
				RowsSent:     464,
				RowsExamined: 2057052,
				RowsAffected: 0,
				BytesSent:    17781,
				Query:        "SELECT * FROM some_table;",
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

func TestDatabase_ParseEmptyBlocs(t *testing.T) {
	tests := []struct {
		name string
		bloc []string
	}{
		{
			name: "testing",
			bloc: []string{
				"# Time: 210323 11:31:57",
				"# User@Host: hugo[hugo] @  [172.18.0.3]",
				"# Thread_id: 12794  Schema:   QC_hit: No",
				"# Query_time: 0.000035  Lock_time: 0.000000  Rows_sent: 0  Rows_examined: 0",
				"# Rows_affected: 0  Bytes_sent: 11",
				"",
				"SET NAMES utf8mb4;",
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
			<-db.WaitingList
		})
	}
}