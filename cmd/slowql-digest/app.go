package main

import (
	"errors"
	"sync"
	"time"

	"github.com/devops-works/slowql"
	"github.com/devops-works/slowql/query"
	"github.com/sirupsen/logrus"
)

func newApp(loglevel, kind string) (*app, error) {
	var a app

	// init res map
	a.res = make(map[string]statistics)

	// create application logger
	a.logger = logrus.New()
	switch loglevel {
	case "trace":
		a.logger.SetLevel(logrus.TraceLevel)
	case "debug":
		a.logger.SetLevel(logrus.DebugLevel)
	case "info":
		a.logger.SetLevel(logrus.InfoLevel)
	case "warn":
		a.logger.SetLevel(logrus.WarnLevel)
	case "error", "err":
		a.logger.SetLevel(logrus.ErrorLevel)
	case "fatal":
		a.logger.SetLevel(logrus.FatalLevel)
	default:
		return nil, errors.New("log level not recognised: " + loglevel)
	}

	// convert kind from string to slowql.Kind
	switch kind {
	case "mysql":
		a.kind = slowql.MySQL
	case "mariadb":
		a.kind = slowql.MariaDB
	case "pxc":
		a.kind = slowql.PXC
	default:
		return nil, errors.New("kind not recognised: " + kind)
	}

	return &a, nil
}

func (a *app) digest(q query.Query, wg *sync.WaitGroup) error {
	defer wg.Done()
	var s statistics
	s.fingerprint = fingerprint(q.Query)
	s.hash = hash(s.fingerprint)

	a.mu.Lock()
	if cur, ok := a.res[s.hash]; ok {
		// there is already results
		cur.calls++
		cur.cumBytesSent += q.BytesSent
		cur.cumKilled += q.Killed
		cur.cumLockTime += time.Duration(q.LockTime)
		cur.cumRowsExamined += q.RowsExamined
		cur.cumRowsSent += q.RowsSent
		s.cumQueryTime += time.Duration(q.QueryTime)

		// update the entry in the map
		a.res[s.hash] = cur
	} else {
		// it is the first time this hash appears
		s.calls++
		s.cumBytesSent = q.BytesSent
		s.cumKilled = q.Killed
		s.cumLockTime = time.Duration(q.LockTime)
		s.cumRowsExamined = q.RowsExamined
		s.cumRowsSent = q.RowsSent
		s.cumQueryTime = time.Duration(q.QueryTime)

		// getting those values is done only once: same hash == same fingerprint & schema
		s.schema = q.Schema

		// add the entry to the map
		a.res[s.hash] = s
	}
	a.mu.Unlock()

	return nil
}
