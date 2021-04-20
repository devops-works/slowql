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
	s.Fingerprint = fingerprint(q.Query)
	s.Hash = hash(s.Fingerprint)

	a.mu.Lock()
	if cur, ok := a.res[s.Hash]; ok {
		// there is already results
		cur.Calls++
		cur.CumBytesSent += q.BytesSent
		cur.CumKilled += q.Killed
		cur.CumLockTime += time.Duration(q.LockTime)
		cur.CumRowsExamined += q.RowsExamined
		cur.CumRowsSent += q.RowsSent
		s.CumQueryTime += time.Duration(q.QueryTime)

		// update the entry in the map
		a.res[s.Hash] = cur
	} else {
		// it is the first time this hash appears
		s.Calls++
		s.CumBytesSent = q.BytesSent
		s.CumKilled = q.Killed
		s.CumLockTime = time.Duration(q.LockTime)
		s.CumRowsExamined = q.RowsExamined
		s.CumRowsSent = q.RowsSent
		s.CumQueryTime = time.Duration(q.QueryTime)

		// getting those values is done only once: same hash == same fingerprint & schema
		s.Schema = q.Schema

		// add the entry to the map
		a.res[s.Hash] = s
	}
	a.mu.Unlock()

	return nil
}
