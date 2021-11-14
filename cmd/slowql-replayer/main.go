package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/devops-works/slowql"
	"github.com/devops-works/slowql/cmd/slowql-replayer/pprof"
	"github.com/devops-works/slowql/query"
	ar "github.com/logrusorgru/aurora"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"

	_ "github.com/go-sql-driver/mysql"
)

type options struct {
	user       string
	host       string
	pass       string
	file       string
	kind       string
	database   string
	loglvl     string
	pprof      string
	workers    int
	factor     float64
	usePass    bool
	noDryRun   bool
	showErrors bool
	hidePB     bool
}

type database struct {
	kind        slowql.Kind
	datasource  string
	drv         *sql.DB
	noDryRun    bool
	logger      *logrus.Logger
	wrks        int
	speedFactor float64
	showErrors  bool
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
	flag.StringVar(&opt.host, "h", "", "Address of the database, with IP and port")
	flag.StringVar(&opt.file, "f", "/log/slowquery.log", "Slow query log file to use")
	flag.StringVar(&opt.kind, "k", "", "Kind of the database (mysql, mariadb...)")
	flag.StringVar(&opt.database, "db", "", "Name of the database to use")
	flag.StringVar(&opt.loglvl, "l", "info", "Logging level")
	flag.StringVar(&opt.pprof, "pprof", "", "pprof server address")
	flag.IntVar(&opt.workers, "w", 100, "Number of maximum simultaneous connections to database")
	flag.Float64Var(&opt.factor, "x", 1, "Speed factor")
	flag.BoolVar(&opt.usePass, "p", false, "Use a password to connect to database")
	flag.BoolVar(&opt.noDryRun, "no-dry-run", false, "Replay the requests on the database for real")
	flag.BoolVar(&opt.showErrors, "show-errors", false, "Show SQL errors when they occur")
	flag.BoolVar(&opt.hidePB, "hide-progress", false, "Hide progress bar while replaying")
	flag.Parse()

	if errs := opt.parse(); len(errs) > 0 {
		flag.Usage()
		for _, e := range errs {
			logrus.Warn(e)
		}
		logrus.Fatal("cannot parse options")
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

	db.logger.Info("getting real execution time")
	num, realExec, err := getReferences(db.kind, opt.file)
	if err != nil {
		db.logger.Fatalf("cannot get references from log file: %s", err)
	}

	db.logger.Infof("%d workers will be created", opt.workers)
	if opt.noDryRun {
		db.logger.Warn("no-dry-run flag found, queries will be executed")
	} else {
		db.logger.Warn("replaying with dry run")
	}

	db.logger.Infof("replay started on %s", time.Now().Format("Mon Jan 2 15:04:05"))
	db.logger.Infof("estimated time of end: %s", time.Now().
		Add(time.Duration(float64(realExec)/db.speedFactor)).Format("Mon Jan 2 15:04:05"))

	r, err := db.replay(f, num, opt.hidePB)
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
func (o *options) parse() []error {
	var errs []error
	if o.user == "" {
		errs = append(errs, errors.New("no user provided"))
	} else if o.host == "" {
		errs = append(errs, errors.New("no host provided"))
	} else if o.file == "" {
		errs = append(errs, errors.New("no slow query log file provided"))
	} else if o.kind == "" {
		errs = append(errs, errors.New("no database kind provided"))
	} else if o.database == "" {
		errs = append(errs, errors.New("no database provided"))
	} else if o.workers <= 0 {
		errs = append(errs, errors.New("cannot create negative number or zero workers"))
	} else if o.factor <= 0 {
		errs = append(errs, errors.New("cannot use a speed factor inferior or equal to 0"))
	}

	if len(errs) != 0 {
		return errs
	}

	if o.usePass {
		fmt.Printf("Password: ")
		bytes, err := term.ReadPassword(syscall.Stdin)
		if err != nil {
			errs = append(errs, err)
		}
		fmt.Println()

		o.pass = string(bytes)
	}
	return errs
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

	db.speedFactor = o.factor
	db.logger.Debugf("speed factor: %f", db.speedFactor)

	maxOpen := db.wrks * 2
	maxIdle := db.wrks / 4

	db.drv.SetMaxOpenConns(maxOpen)
	db.drv.SetMaxIdleConns(maxIdle)
	db.drv.SetConnMaxLifetime(50 * time.Millisecond)
	db.logger.Debugf("db max open conns: %d", maxOpen)
	db.logger.Debugf("db max idle conns: %d", maxIdle)

	db.showErrors = o.showErrors
	db.logger.Debugf("show errors: %v", db.showErrors)

	return &db, nil
}

// replay replays the queries from a slow query log file to a database
func (db *database) replay(f io.Reader, totQ int, hidePB bool) (results, error) {
	var r results

	p := slowql.NewParser(db.kind, f)

	jobs := make(chan job, 65535)
	errors := make(chan error, 16384)
	queries := make(chan int, 65535)
	var wg sync.WaitGroup

	db.logger.Debug("starting workers pool")
	var workersCounter int
	for i := 0; i < db.wrks; i++ {
		wg.Add(1)
		workersCounter++
		go db.worker(jobs, errors, queries, db.noDryRun, &wg)
	}
	db.logger.Debugf("created %d workers successfully", workersCounter)

	db.logger.Debug("starting errors collector")
	go r.errorsCollector(errors, db.showErrors)

	var bar *pb.ProgressBar
	// start all the progress bar stuff is asked
	if !hidePB {
		tmpl := `{{counters .}} {{ bar . "[" ("▉" | green) (cycle . "▉" " " | green ) "." "]"}} {{speed .}} {{rtime . "ETA %s"}} {{percent .}}`
		bar = pb.ProgressBarTemplate(tmpl).Start(totQ)
		bar.SetRefreshRate(400 * time.Millisecond)
		go updateBar(bar, queries)
	}

	firstPass := true

	// first timestamp that appears in the log file
	var reference time.Time
	start := time.Now()
	for {
		q := p.GetNext()
		if q == (query.Query{}) {
			break
		}
		db.logger.Tracef("query: %s", q.Query)

		r.queries++

		// we need a reference time
		if firstPass {
			firstPass = false
			reference = q.Time
		}

		var j job
		delta := q.Time.Sub(reference)
		j.idle = start.Add(time.Duration(float64(delta) / db.speedFactor))
		j.query = q.Query
		db.logger.Tracef("next sleeping time: %s", j.idle)

		jobs <- j
	}
	close(jobs)
	db.logger.Debug("closed jobs channel")

	wg.Wait()
	close(errors)
	db.logger.Debug("closed errors channel")

	// terminate all the stuff related to the progress bar if it has been
	// created
	if !hidePB {
		bar.Finish()
		close(queries)
		db.logger.Debug("progress bar stopped and update channel closed")
	}

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
Real duration:    %s
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
  ├─ Speed factor:           %.4f
  ├─ Duration difference:    %s
  └─ Replayer speed:         %s

%s: the replayer may take a little more time due to the numerous conditions that are verified during the replay.
`,
		ar.Bold(r.duration),
		ar.Bold(r.realDuration),
		ar.Bold(o.file),
		ar.Bold(r.dryRun),
		ar.Bold(o.workers),
		// database
		ar.Bold(r.kind),
		ar.Bold(o.user),
		ar.Bold(o.usePass),
		ar.Bold(o.host),
		// statistics
		ar.Bold(r.queries),
		ar.Bold(r.errors),
		ar.Bold(prcSuccess),
		ar.Bold(o.factor),
		ar.Bold(durationDelta),
		ar.Bold(prcSpeedStr),
		// footnote
		ar.Bold(ar.Yellow("Note")),
	)
}

func updateBar(bar *pb.ProgressBar, newQueries chan int) {
	for {
		_, ok := <-newQueries
		if !ok {
			return
		}
		bar.Increment()
	}
}

func (db database) worker(jobs chan job, errors chan error, queries chan int, noDryRun bool, wg *sync.WaitGroup) {
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
		if noDryRun {
			rows, err := db.drv.Query(j.query)
			if err != nil {
				errors <- err
				db.logger.Tracef("failed to execute query:\n%s\nerror: %s", j.query, err)
			}
			if rows != nil {
				rows.Close()
			}
		}
		queries <- 42
	}
}

func (r *results) errorsCollector(errors chan error, showErrors bool) {
	for {
		e, ok := <-errors
		if !ok {
			return
		}
		r.errors++
		if showErrors {
			fmt.Printf("\n%s: %s\n", ar.Red("SQL error"), e.Error())
		}
	}
}

// getReferences returns the reference log duration and the number of queries
func getReferences(k slowql.Kind, f string) (int, time.Duration, error) {
	var queriesCounter int

	fd, err := os.Open(f)
	if err != nil {
		return -1, 0, err
	}

	p := slowql.NewParser(k, fd)

	var q query.Query
	firstPass := true
	var reference, lastTime time.Time
	for {
		q = p.GetNext()
		if (q == query.Query{}) {
			break
		}

		if firstPass {
			firstPass = false
			reference = q.Time
		}
		queriesCounter++
		lastTime = q.Time
	}
	fd.Close()
	duration := lastTime.Sub(reference)
	return queriesCounter, duration, nil
}
