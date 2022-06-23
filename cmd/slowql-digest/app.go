package main

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/devops-works/slowql"
	"github.com/devops-works/slowql/query"
	"github.com/sirupsen/logrus"
)

type app struct {
	mu             sync.Mutex
	logger         *logrus.Logger
	kind           slowql.Kind
	fd             io.Reader
	p              slowql.Parser
	res            map[string]statistics
	digestDuration time.Duration
	queriesNumber  int
}

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
	case "percona":
		a.kind = slowql.PerconaDB
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
	defer a.mu.Unlock()

	if cur, ok := a.res[s.Hash]; ok {
		// there is already results
		cur.Calls++
		cur.CumBytesSent += q.BytesSent
		cur.CumKilled += q.Killed
		cur.CumLockTime += q.LockTime
		cur.CumRowsExamined += q.RowsExamined
		cur.CumRowsSent += q.RowsSent
		cur.CumQueryTime += q.QueryTime
		cur.QueryTimes = append(cur.QueryTimes, q.QueryTime)

		// update max time
		if q.QueryTime > cur.MaxTime {
			cur.MaxTime = q.QueryTime
		}

		// update min time
		if q.QueryTime < cur.MinTime {
			cur.MinTime = q.QueryTime
		}

		// update the entry in the map
		a.res[s.Hash] = cur
	} else {
		// it is the first time this hash appears
		s.Calls++
		s.CumBytesSent = q.BytesSent
		s.CumKilled = q.Killed
		s.CumLockTime = q.LockTime
		s.CumRowsExamined = q.RowsExamined
		s.CumRowsSent = q.RowsSent
		s.CumQueryTime = q.QueryTime
		s.MinTime = q.QueryTime
		s.MaxTime = q.QueryTime
		s.MeanTime = q.QueryTime
		s.QueryTimes = append(s.QueryTimes, q.QueryTime)

		// getting those values is done only once: same hash == same fingerprint & schema
		s.Schema = q.Schema

		// add the entry to the map
		a.res[s.Hash] = s
	}

	return nil
}
