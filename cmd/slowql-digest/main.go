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

	"github.com/devops-works/slowql"
	"github.com/devops-works/slowql/query"
	"github.com/devops-works/slowql/server"
	ar "github.com/logrusorgru/aurora"
	"github.com/sirupsen/logrus"
)

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
	CumQueryTime    float64
	CumLockTime     float64
	CumRowsSent     int
	CumRowsExamined int
	CumBytesSent    int
	Concurrency     float64
	MinTime         float64
	MaxTime         float64
	MeanTime        float64
	P50Time         float64
	P95Time         float64
	StddevTime      float64
	QueryTimes      []float64
}

type serverMeta struct {
	Binary             string
	Port               int
	Socket             string
	Version            string
	VersionShort       string
	VersionDescription string

	Duration     time.Duration
	RealDuration time.Duration
	Bytes        int
}

var orders = []string{"bytes_sent", "calls", "concurrency", "killed", "lock_time",
	"max_time", "mean_time", "min_time", "p50", "p90", "query_time", "random", "rows_examined", "rows_sent"}

func main() {
	var o options
	flag.StringVar(&o.logfile, "f", "/log/slowquery.log", "Slow query log file to digest "+ar.Red("(required)").String())
	flag.StringVar(&o.loglevel, "l", "info", "Log level")
	flag.StringVar(&o.kind, "k", "mysql", "Database kind. Use ? to see all the available values.")
	flag.IntVar(&o.top, "top", 3, "Top queries to show")
	flag.StringVar(&o.order, "sort-by", "random", "How to sort queries. Use ? to see all the available values")
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

	dbKinds := []string{"mariadb", "mysql", "pxc", "percona"}
	if o.kind == "?" {
		fmt.Println("Available values:")
		for _, val := range dbKinds {
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
			stats, err := computeStats(cacheResults, res.TotalDuration)
			if err != nil {
				a.logger.Errorf("cannot compute statistics: %s. This can lead to inacurrate stats", err)
			}
			stats, err = sortResults(stats, o.order, o.dec)
			if err != nil {
				a.logger.Errorf("cannot sort results: %s", err)
				o.order = "random"
				stats, err = sortResults(stats, o.order, o.dec)
				if err != nil {
					a.logger.Fatalf("cannot sort results: %s", err)
				}
			}
			showResults(stats, res.ServerMeta, o.order, o.top, o.dec)
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
		defer fd.Close()

		lines, err := lineCounter(fd)
		if err != nil {
			a.logger.Errorf("cannot count lines in log file: %s", err)
		}
		a.logger.Infof("log file has %d lines", lines)
	}

	var q query.Query
	var wg sync.WaitGroup
	var realStart, realEnd time.Time
	firstPass := true
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
		if firstPass {
			realStart = q.Time
			firstPass = false
		}
		realEnd = q.Time
		a.queriesNumber++
		wg.Add(1)
		go a.digest(q, &wg)
	}
	wg.Wait()
	a.digestDuration = time.Since(start)

	a.logger.Infof("digest duration: %s", a.digestDuration)
	a.logger.Infof("parsed %d queries", a.queriesNumber)
	a.logger.Infof("found %d different queries hashs", len(a.res))

	srv := a.p.GetServerMeta()
	srvMeta := getMeta(srv)
	srvMeta.Duration = a.digestDuration

	var res []statistics
	for _, val := range a.res {
		res = append(res, val)
		srvMeta.Bytes += val.CumBytesSent
	}

	realDuration := realEnd.Sub(realStart)
	srvMeta.RealDuration = realDuration

	res, err = computeStats(res, realDuration)
	if err != nil {
		a.logger.Errorf("cannot compute statistics: %s. This can lead to inacurrate stats", err)
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

	showResults(res, srvMeta, o.order, o.top, o.dec)
	if !o.nocache {
		a.logger.Info("saving results in cache file")
		cache := results{
			File:          o.logfile,
			Date:          time.Now(),
			TotalDuration: realDuration,
			Data:          res,
			ServerMeta:    srvMeta,
		}
		if err := saveCache(cache); err != nil {
			a.logger.Errorf("cannot save results in cache file: %s", err)
		}
	}
	a.logger.Debug("end of program, exiting")
}

func showResults(res []statistics, sm serverMeta, order string, count int, dec bool) {
	howTo := "increasing"
	if dec {
		howTo = "decreasing"
	}

	// show server's meta
	fmt.Printf(`
=-= Server meta =-=
	
Binary              : %s
Port                : %d
Socket              : %s
Version             : %s
Version short       : %s
Version description : %s

Digest duration     : %s
Real duration       : %s

Bytes handled       : %d
	`,
		sm.Binary,
		sm.Port,
		sm.Socket,
		sm.Version,
		sm.VersionShort,
		sm.VersionDescription,
		sm.Duration,
		sm.RealDuration,
		sm.Bytes)

	// show queries stats
	fmt.Printf("\n=-= Queries stats =-=\n")
	fmt.Printf("\nSorted by: %s, %s\n", ar.Bold(order), ar.Bold(howTo))
	fmt.Printf("Showing top %d queries\n", ar.Bold(count))
	for i := 0; i < len(res); i++ {
		if count == 0 {
			break
		}

		fmt.Printf(`
%s%d
Calls                  : %d
Hash                   : %s
Fingerprint            : %s
Schema                 : %s
Min/Max/Mean time      : %s/%s/%s
p50/p95                : %s/%s
Concurrency            : %2.4f%%
Standard deviation     : %s
Cum Query Time         : %s
Cum Lock Time          : %s
Cum Bytes sent         : %d
Cum Rows Examined/Sent : %d/%d
Cum Killed             : %d
			`,
			ar.Bold(ar.Underline("Query #")),
			ar.Bold(ar.Underline(i+1)),
			res[i].Calls,
			res[i].Hash,
			res[i].Fingerprint,
			res[i].Schema,
			fsecsToDuration(res[i].MinTime),
			fsecsToDuration(res[i].MaxTime),
			fsecsToDuration(res[i].MeanTime),
			fsecsToDuration(res[i].P50Time),
			fsecsToDuration(res[i].P95Time),
			res[i].Concurrency,
			fsecsToDuration(res[i].StddevTime),
			fsecsToDuration(res[i].CumQueryTime),
			fsecsToDuration(res[i].CumLockTime),
			res[i].CumBytesSent,
			res[i].CumRowsExamined,
			res[i].CumRowsSent,
			res[i].CumKilled,
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
	case "min_time":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].MinTime < s[j].MinTime
		})
	case "max_time":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].MaxTime < s[j].MaxTime
		})
	case "mean_time":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].MeanTime < s[j].MeanTime
		})
	case "p50", "P50":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].P50Time < s[j].P50Time
		})
	case "p95", "P95":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].P95Time < s[j].P95Time
		})
	case "concurrency":
		sort.SliceStable(s, func(i, j int) bool {
			return s[i].Concurrency < s[j].Concurrency
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

func getMeta(srv server.Server) serverMeta {
	var sm serverMeta
	sm.Binary = srv.Binary
	sm.Port = srv.Port
	sm.Socket = srv.Socket
	sm.Version = srv.Version
	sm.VersionShort = srv.VersionShort
	sm.VersionDescription = srv.VersionDescription
	return sm
}
