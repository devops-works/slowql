package main

import (
	"bufio"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/devops-works/slowql"
	"github.com/devops-works/slowql/cmd/slowql-replayer/pprof"
	"github.com/devops-works/slowql/query"
	. "github.com/logrusorgru/aurora"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"

	_ "github.com/go-sql-driver/mysql"
)

type options struct {
	user     string
	host     string
	pass     string
	file     string
	kind     string
	database string
	loglvl   string
	pprof    string
	workers  int
	usePass  bool
	noDryRun bool
}

type database struct {
	kind       slowql.Kind
	datasource string
	drv        *sql.DB
	noDryRun   bool
	logger     *logrus.Logger
	wrks       int
}

type results struct {
	kind         string
	dryRun       bool
	queries      int
	errors       int
	duration     time.Duration
	realDuration time.Duration
}

type job struct {
	query string
	idle  time.Time
}

func main() {
	var opt options

	flag.StringVar(&opt.user, "u", "", "User to use to connect to database")
	flag.StringVar(&opt.host, "h", "", "Addres of the database, with IP and port")
	flag.StringVar(&opt.file, "f", "", "Slow query log file to use")
	flag.StringVar(&opt.kind, "k", "", "Kind of the database (mysql, mariadb...)")
	flag.StringVar(&opt.database, "db", "", "Name of the database to use")
	flag.StringVar(&opt.loglvl, "l", "info", "Logging level")
	flag.StringVar(&opt.pprof, "pprof", "", "pprof server address")
	flag.IntVar(&opt.workers, "w", 100, "Number of maximum simultaneous connections to database")
	flag.BoolVar(&opt.usePass, "p", false, "Use a password to connect to database")
	flag.BoolVar(&opt.noDryRun, "no-dry-run", false, "Replay the requests on the database for real")
	flag.Parse()

	if err := opt.parse(); err != nil {
		flag.Usage()
		logrus.Fatalf("cannot parse options: %s", err)
	}

	db, err := opt.createDB()
	if err != nil {
		logrus.Fatalf("cannot create databse object: %s", err)
	}
	defer db.drv.Close()
	db.logger.Debug("database object successfully created")

	f, err := os.Open(opt.file)
	if err != nil {
		logrus.Fatalf("cannot open slow query log file: %s", err)
	}
	db.logger.Debugf("file %s successfully opened", opt.file)

	if opt.pprof != "" {
		pprofServer, err := pprof.New(opt.pprof)
		if err != nil {
			db.logger.Fatalf("unable to create pprof server: %s", err)
		}
		go pprofServer.Run()
		db.logger.Infof("pprof started on 'http://%s'", pprofServer.Addr)
	}

	var r results

	db.logger.Info("getting real execution time")
	realExec, err := getRealTime(opt.kind, opt.file)
	if err != nil {
		db.logger.Fatalf("cannot get real duration from log file: %s", err)
	}

	db.logger.Infof("%d workers will be created", opt.workers)
	if opt.noDryRun {
		db.logger.Warn("no-dry-run flag found, queries will be executed")
		r, err = db.replay(f)
	} else {
		db.logger.Warn("replaying with dry run")
		// r, err = db.dryRun(f)
	}
	if err != nil {
		db.logger.Fatalf("cannot replay %s: %s", opt.kind, err)
	}

	r.dryRun = !opt.noDryRun
	r.kind = opt.kind
	r.realDuration = realExec
	r.show(opt)
}

// parse ensures that no options has been omitted. It also asks for a password
// if it is required
func (o *options) parse() error {
	if o.user == "" {
		return errors.New("no user provided")
	} else if o.host == "" {
		return errors.New("no host provided")
	} else if o.file == "" {
		return errors.New("no slow query log file provided")
	} else if o.kind == "" {
		return errors.New("no database kind provided")
	} else if o.database == "" {
		return errors.New("no database provided")
	} else if o.workers <= 0 {
		return errors.New("cannot create negative number or zero workers")
	}

	if o.usePass {
		fmt.Printf("Password: ")
		bytes, err := term.ReadPassword(syscall.Stdin)
		if err != nil {
			return err
		}
		fmt.Println()

		o.pass = string(bytes)
	}
	return nil
}

// createDB creates a database object according to what has been specified in
// the options
func (o options) createDB() (*database, error) {
	var db database
	var err error

	switch strings.ToLower(o.kind) {
	case "mysql":
		db.kind = slowql.MySQL
	case "mariadb":
		db.kind = slowql.MariaDB
	case "pxc":
		db.kind = slowql.PXC
	default:
		return nil, errors.New("unknown kind " + o.kind)
	}

	db.datasource = fmt.Sprintf("%s:%s@tcp(%s)/%s", o.user, o.pass, o.host, o.database)
	db.drv, err = sql.Open("mysql", db.datasource)
	if err != nil {
		return nil, err
	}
	db.noDryRun = o.noDryRun

	if err = db.drv.Ping(); err != nil {
		return nil, err
	}

	db.logger = logrus.New()
	switch o.loglvl {
	case "trace":
		db.logger.SetLevel(logrus.TraceLevel)
	case "debug":
		db.logger.SetLevel(logrus.DebugLevel)
	case "info":
		db.logger.SetLevel(logrus.InfoLevel)
	case "warn":
		db.logger.SetLevel(logrus.WarnLevel)
	case "error", "err":
		db.logger.SetLevel(logrus.ErrorLevel)
	case "fatal":
		db.logger.SetLevel(logrus.FatalLevel)
	case "panic":
		db.logger.SetLevel(logrus.PanicLevel)
	default:
		logrus.Errorf("unknown log level %s, using 'info'", o.loglvl)
		db.logger.SetLevel(logrus.InfoLevel)
	}
	db.logger.Debugf("log level set to %s", db.logger.GetLevel())

	db.wrks = o.workers
	db.logger.Debugf("workers number set to %d", db.wrks)

	maxOpen := db.wrks * 2
	maxIdle := db.wrks / 4

	db.drv.SetMaxOpenConns(maxOpen)
	db.drv.SetMaxIdleConns(maxIdle)
	db.drv.SetConnMaxLifetime(50 * time.Millisecond)
	db.logger.Debugf("db max open conns: %d", maxOpen)
	db.logger.Debugf("db max idle conns: %d", maxIdle)

	return &db, nil
}

// func (db *database) dryRun(f io.Reader) (results, error) {
// 	var r results

// 	p := slowql.NewParser(db.kind, f)

// 	queries := make(chan string, 16384)
// 	errors := make(chan error, 16384)
// 	var wg sync.WaitGroup

// 	db.logger.Debug("starting workers pool")
// 	var workersCounter int
// 	for i := 0; i < db.wrks; i++ {
// 		wg.Add(1)
// 		workersCounter++
// 		go db.worker(queries, errors, &wg)
// 	}
// 	db.logger.Debugf("created %d workers successfully", workersCounter)
// 	db.logger.Debug("starting errors collector")
// 	go r.errorsCollector(errors)

// 	firstPass := true
// 	var previousDate, now time.Time
// 	var sleeping time.Duration

// 	db.logger.Infof("replay started on %s", time.Now().Format("Mon Jan 2 15:04:05"))
// 	s := newSpinner(34)
// 	s.Start()

// 	start := time.Now()
// 	for {
// 		q := p.GetNext()
// 		if q == (query.Query{}) {
// 			s.Stop()
// 			break
// 		}
// 		db.logger.Tracef("query: %s", q.Query)

// 		r.queries++
// 		s.Suffix = " queries replayed: " + strconv.Itoa(r.queries)

// 		// We need a reference time
// 		if firstPass {
// 			firstPass = false
// 			previousDate = q.Time
// 			continue
// 		}

// 		now = q.Time
// 		sleeping = now.Sub(previousDate)
// 		db.logger.Tracef("next sleeping time: %s", sleeping)
// 		time.Sleep(sleeping)

// 		// For MariaDB, when there is multiple queries in a short amount of
// 		// time, the Time field is not repeated, so we do not have to update
// 		// the previous date.
// 		if now != (time.Time{}) {
// 			previousDate = now
// 		}
// 	}
// 	close(queries)
// 	db.logger.Debug("closed queries channel")

// 	wg.Wait()
// 	close(errors)
// 	db.logger.Debug("closed errors channel")

// 	r.duration = time.Since(start)
// 	db.logger.Infof("replay ended on %s", time.Now().Format("Mon Jan 2 15:04:05"))
// 	return r, nil
// }

// replay replays the queries from a slow query log file to a database
func (db *database) replay(f io.Reader) (results, error) {
	var r results

	p := slowql.NewParser(db.kind, f)

	jobs := make(chan job, 65535)
	errors := make(chan error, 16384)
	var wg sync.WaitGroup

	db.logger.Debug("starting workers pool")
	var workersCounter int
	for i := 0; i < db.wrks; i++ {
		wg.Add(1)
		workersCounter++
		go db.worker(jobs, errors, &wg)
	}
	db.logger.Debugf("created %d workers successfully", workersCounter)
	db.logger.Debug("starting errors collector")
	go r.errorsCollector(errors)

	db.logger.Infof("replay started on %s", time.Now().Format("Mon Jan 2 15:04:05"))
	s := newSpinner(34)
	s.Start()

	firstPass := true

	// first timestamp that appears in the log file
	var reference time.Time
	start := time.Now()
	for {
		q := p.GetNext()
		if q == (query.Query{}) {
			s.Stop()
			break
		}
		db.logger.Tracef("query: %s", q.Query)

		r.queries++
		s.Suffix = " queries replayed: " + strconv.Itoa(r.queries)

		// We need a reference time
		if firstPass {
			firstPass = false
			reference = q.Time
		}

		var j job
		delta := q.Time.Sub(reference)
		j.idle = start.Add(delta)
		j.query = q.Query
		db.logger.Tracef("next sleeping time: %s", j.idle)
		// time.Sleep(sleeping)
		jobs <- j
		// For MariaDB, when there is multiple queries in a short amount of
		// time, the Time field is not repeated, so we do not have to update
		// the previous date.
		// if now != (time.Time{}) {
		// 	previousDate = now
		// }
	}
	close(jobs)
	db.logger.Debug("closed queries channel")

	wg.Wait()
	close(errors)
	db.logger.Debug("closed errors channel")

	r.duration = time.Since(start)
	db.logger.Infof("replay ended on %s", time.Now().Format("Mon Jan 2 15:04:05"))
	return r, nil
}

func (r results) show(o options) {
	prcSuccess := fmt.Sprintf("%.4f%%", (float64(r.queries)-float64(r.errors))*100.0/float64(r.queries))
	durationDelta := fmt.Sprint(r.duration - r.realDuration)
	if durationDelta == r.duration.String() {
		durationDelta = "n/a"
	} else if r.duration > r.realDuration {
		durationDelta = "replayer took " + durationDelta + " more"
	} else if r.duration < r.realDuration {
		durationDelta = "replayer took " + durationDelta + " less"
	}

	var prcSpeedStr string
	prcSpeed := float64(r.duration) * 100.0 / float64(r.realDuration)
	if prcSpeed > 100.0 {
		prcSpeed = prcSpeed - 100.0
		prcSpeedStr = fmt.Sprintf("-%.4f%%", prcSpeed)
	} else {
		prcSpeedStr = fmt.Sprintf("+%.4f%%", prcSpeed)
	}
	fmt.Printf(`
=-= Results =-=

Replay duration:  %s
Log file:         %s
Dry run:          %v
Workers:          %d

Database
  ├─ kind:      %s
  ├─ user:      %s
  ├─ use pass:  %v
  └─ address:   %s

Statistics
  ├─ Queries:                %d
  ├─ Errors:                 %d
  ├─ Queries success rate:   %s
  ├─ Duration difference:    %s
  └─ Replayer speed:         %s

%s: the replayer may take a little more time due to the numerous conditions that are verified during the replay.
`,
		Bold(r.duration),
		Bold(o.file),
		Bold(r.dryRun),
		Bold(o.workers),
		// database
		Bold(r.kind),
		Bold(o.user),
		Bold(o.usePass),
		Bold(o.host),
		// statistics
		Bold(r.queries),
		Bold(r.errors),
		Bold(prcSuccess),
		Bold(durationDelta),
		Bold(prcSpeedStr),
		// footnote
		Bold(Yellow("Note")),
	)
}

func newSpinner(t int) *spinner.Spinner {
	return spinner.New(spinner.CharSets[t], 100*time.Millisecond)
}

func (db database) worker(jobs chan job, errors chan error, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		j, ok := <-jobs
		if !ok {
			db.logger.Trace("channel closed, worker exiting")
			return
		}
		sleep := time.Until(j.idle)
		if sleep > 0 {
			time.Sleep(sleep)
		}
		rows, err := db.drv.Query(j.query)
		if err != nil {
			errors <- err
			db.logger.Debugf("failed to execute query:\n%s\nerror: %s", j.query, err)
		}
		if rows != nil {
			rows.Close()
		}
	}
}

func (r *results) errorsCollector(errors chan error) {
	for {
		_, ok := <-errors
		if !ok {
			return
		}
		r.errors++
	}
}

// getRealTime returns the real duration of the slow query log based on the Time
// tags in the headers
func getRealTime(k, file string) (time.Duration, error) {
	f, err := os.Open(file)
	if err != nil {
		return time.Duration(0), err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	var start, end string

	switch k {
	case "mysql", "pxc":
		// Get the first Time
		for s.Scan() {
			if strings.Contains(s.Text(), "Time:") {
				parts := strings.Split(s.Text(), " ")
				start = parts[2]
				break
			}
		}

		// Get the last time
		for s.Scan() {
			if strings.Contains(s.Text(), "Time:") {
				parts := strings.Split(s.Text(), " ")
				end = parts[2]
			}
		}

		timeStart, err := time.Parse("2006-01-02T15:04:05.999999Z", start)
		if err != nil {
			return time.Duration(0), err
		}
		timeEnd, err := time.Parse("2006-01-02T15:04:05.999999Z", end)
		if err != nil {
			return time.Duration(0), err
		}
		return timeEnd.Sub(timeStart), nil
	case "mariadb":
		// Get the first Time
		for s.Scan() {
			if strings.Contains(s.Text(), "Time:") {
				parts := strings.Split(s.Text(), " ")
				start = parts[2] + " " + parts[3]
				break
			}
		}
		// Get the last time
		for s.Scan() {
			if strings.Contains(s.Text(), "Time:") {
				parts := strings.Split(s.Text(), " ")
				end = parts[2] + " " + parts[3]
			}
		}

		timeStart, err := time.Parse("060102 15:04:05", start)
		if err != nil {
			return time.Duration(0), err
		}
		timeEnd, err := time.Parse("060102 15:04:05", end)
		if err != nil {
			return time.Duration(0), err
		}
		return timeEnd.Sub(timeStart), nil
	default:
		return time.Duration(0), nil
	}
}
