package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/cloudflare/cfssl/log"
	"github.com/devops-works/slowql"
	"github.com/devops-works/slowql/query"
	. "github.com/logrusorgru/aurora"
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

type options struct {
	logfile  string
	loglevel string
	kind     string
	top      int
	order    string
	dec      bool
	nocache  bool
}

type statistics struct {
	Hash            string
	Fingerprint     string
	Schema          string
	Calls           int
	CumErrored      int
	CumKilled       int
	CumQueryTime    time.Duration
	CumLockTime     time.Duration
	CumRowsSent     int
	CumRowsExamined int
	CumBytesSent    int
	Concurrency     float64
	MinTime         time.Duration
	MaxTime         time.Duration
	MeanTime        time.Duration
	P50Time         time.Duration
	P95Time         time.Duration
	StddevTime      time.Duration
}

var orders = []string{"random", "calls", "bytes_sent", "query_time", "lock_time",
	"rows_sent", "rows_examined", "killed"}

func main() {
	var o options
	flag.StringVar(&o.logfile, "f", "", "Slow query log file to digest")
	flag.StringVar(&o.loglevel, "l", "info", "Log level")
	flag.StringVar(&o.kind, "k", "", "Database kind")
	flag.IntVar(&o.top, "top", 3, "Top queries to show")
	flag.StringVar(&o.order, "sort-by", "random", "How to sort queries. use ? to see all the available values")
	flag.BoolVar(&o.dec, "dec", false, "Sort by decreasing order")
	flag.BoolVar(&o.nocache, "no-cache", false, "Do not use cache, if cache exists")
	flag.Parse()

	if o.order == "?" {
		fmt.Println("Available values:")
		for _, val := range orders {
			fmt.Printf("    %s\n", val)
		}
		return
	}

	errs := o.parse()
	if len(errs) != 0 {
		flag.Usage()
		for _, e := range errs {
			logrus.Warn(e)
		}
		logrus.Fatal("cannot parse options")
	}

	a, err := newApp(o.loglevel, o.kind)
	if err != nil {
		logrus.Fatalf("cannot create app: %s", err)
	}

	// if we want to use cache and the cache file exists...
	if !o.nocache && findCache(o.logfile) {
		a.logger.Infof("cache found: %s. Trying to restore it", o.logfile+".cache")
		// ...we try to restore it
		res, err := restoreCache(o.logfile)
		if err != nil {
			a.logger.Errorf("cannot restore cache: %s", err)
			a.logger.Warn("continuing without cache")
		} else {
			a.logger.Infof("%s restored", o.logfile+".cache")
			cacheResults, err := sortResults(res.Data, o.order, o.dec)
			if err != nil {
				a.logger.Errorf("cannot sort results: %s", err)
				o.order = "random"
				cacheResults, err = sortResults(cacheResults, o.order, o.dec)
				if err != nil {
					a.logger.Fatalf("cannot sort results: %s", err)
				}
			}
			a.logger.Infof("cache has timestamp: %s", res.Date)
			showResults(cacheResults, o.order, o.top)
			return
		}
		a.logger.Info("cache will not be used")
	}
	a.fd, err = os.Open(o.logfile)
	if err != nil {
		a.logger.Fatalf("cannot open log file: %s", err)
	}
	a.logger.Debugf("%s successfully opened", o.logfile)

	// no need to compute stuff if it will not be displayed
	if a.logger.Level >= logrus.InfoLevel {
		fd, err := os.Open(o.logfile)
		if err != nil {
			a.logger.Errorf("cannot open log file to count lines: %s", err)
		}
		lines, err := lineCounter(fd)
		if err != nil {
			a.logger.Errorf("cannot count lines in log file: %s", err)
		}
		a.logger.Infof("log file has %d lines", lines)
		fd.Close()
	}

	var q query.Query
	var wg sync.WaitGroup
	a.p = slowql.NewParser(a.kind, a.fd)
	a.logger.Debug("slowql parser created")
	a.logger.Debug("query analysis started")
	start := time.Now()
	for {
		q = a.p.GetNext()
		if q == (query.Query{}) {
			a.logger.Debug("no more queries, breaking for loop")
			break
		}
		a.queriesNumber++
		wg.Add(1)
		go a.digest(q, &wg)
	}
	wg.Wait()
	a.digestDuration = time.Since(start)

	a.logger.Infof("digest duration: %s", a.digestDuration)
	a.logger.Infof("parsed %d queries", a.queriesNumber)
	a.logger.Infof("found %d different queries hashs", len(a.res))

	var res []statistics
	for _, val := range a.res {
		res = append(res, val)
	}
	res, err = sortResults(res, o.order, o.dec)
	if err != nil {
		a.logger.Errorf("cannot sort results: %s", err)
		o.order = "random"
		res, err = sortResults(res, o.order, o.dec)
		if err != nil {
			a.logger.Fatalf("cannot sort results: %s", err)
		}
	}

	showResults(res, o.order, o.top)
	if !o.nocache {
		log.Info("saving results in cache file")
		cache := results{
			File: o.logfile,
			Date: time.Now(),
			Data: res,
		}
		if err := saveCache(cache); err != nil {
			log.Errorf("cannot save results in cache file: %s", err)
		}
	}
	a.logger.Debug("end of program, exiting")
}

func showResults(res []statistics, order string, count int) {
	fmt.Printf("\nSorted by: %s\n", Bold(order))
	fmt.Printf("Showing top %d queries\n", Bold(count))
	for i := 0; i < len(res); i++ {
		if count == 0 {
			break
		}

		fmt.Printf(`
%s%d
Calls:             %d
Hash:              %s
Fingerprint:       %s
Schema:            %s
Cum Bytes sent:    %d
Cum Rows Examined: %d
Cum Rows Sent:     %d
Cum Killed:        %d
Cum Lock Time:     %s
Cum Query Time:    %s
			`,
			Bold(Underline("Query #")),
			Bold(Underline(i+1)),
			res[i].Calls,
			res[i].Hash,
			res[i].Fingerprint,
			res[i].Schema,
			res[i].CumBytesSent,
			res[i].CumRowsExamined,
			res[i].CumRowsSent,
			res[i].CumKilled,
			res[i].CumLockTime,
			res[i].CumQueryTime,
		)

		count--
	}
	fmt.Println()
}

func lineCounter(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0

	lineSep := []byte{'\n'}
	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)
		switch {
		case err == io.EOF:
			return count, nil
		case err != nil:
			return count, err
		}
	}
}

func sortResults(s []statistics, order string, dec bool) ([]statistics, error) {

	switch order {
	case "random":
		break
	case "calls":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].Calls < s[j].Calls
		})
	case "bytes_sent":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].CumBytesSent < s[j].CumBytesSent
		})
	case "query_time":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].CumQueryTime < s[j].CumQueryTime
		})
	case "lock_time":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].CumLockTime < s[j].CumLockTime
		})
	case "rows_sent":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].CumRowsSent < s[j].CumRowsSent
		})
	case "rows_examined":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].CumRowsExamined < s[j].CumRowsExamined
		})
	case "killed":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].CumKilled < s[j].CumKilled
		})
	default:
		return nil, errors.New("unknown order, using 'random'")
	}

	if dec {
		for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
			s[i], s[j] = s[j], s[i]
		}
	}
	return s, nil
}
