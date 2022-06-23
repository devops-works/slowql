package main

import (
	"fmt"
	"os"
	"time"

	"github.com/devops-works/slowql"
	"github.com/devops-works/slowql/query"
	"github.com/devops-works/slowql/server"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s file.log\n", os.Args[0])
		os.Exit(1)
	}

	fd, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}

	p := slowql.NewParser(slowql.PerconaDB, fd)
	srv := p.GetServerMeta()
	showServer(srv)

	var count int
	start := time.Now()
	for {
		q := p.GetNext()
		if q == (query.Query{}) {
			break
		}

		// showQuery(q)

		count++
	}
	elapsed := time.Since(start)
	fmt.Printf("\nparsed %d queries in %s\n", count, elapsed)
}

func showQuery(q query.Query) {
	fmt.Printf("Time: %s\nUser: %s\nHost: %s\nID: %d\nSchema: %s\nLast_errno: %d\nKilled: %d\nQuery_time: %f\nLock_time: %f\nRows_sent: %d\nRows_examined: %d\nRows_affected: %d\nBytes_sent: %d\nQuery: %s\n",
		q.Time,
		q.User,
		q.Host,
		q.ID,
		q.Schema,
		q.LastErrNo,
		q.Killed,
		q.QueryTime,
		q.LockTime,
		q.RowsSent,
		q.RowsExamined,
		q.RowsAffected,
		q.BytesSent,
		q.Query,
	)
}

func showServer(srv server.Server) {
	fmt.Printf("Binary: %s\nVersion short: %s\nVersion: %s\nVersion description: %s\nSocket: %s\nPort: %d\n",
		srv.Binary,
		srv.VersionShort,
		srv.Version,
		srv.VersionDescription,
		srv.Socket,
		srv.Port,
	)
}
