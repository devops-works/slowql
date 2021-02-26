package main

import (
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
	"github.com/olekukonko/tablewriter"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"

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
	workers  int
	usePass  bool
	dryRun   bool
}

type database struct {
	kind       slowql.Kind
	datasource string
	drv        *sql.DB
	dryRun     bool
	logger     *logrus.Logger
	wrks       int
}

type results struct {
	kind     string
	dryRun   string
	queries  int
	errors   int
	duration time.Duration
}

func main() {
	var opt options

	flag.StringVar(&opt.user, "u", "", "User to use to connect to database")
	flag.StringVar(&opt.host, "h", "", "Addres of the database, with IP and port")
	flag.StringVar(&opt.file, "f", "", "Slow query log file to use")
	flag.StringVar(&opt.kind, "k", "", "Kind of the database (mysql, mariadb...)")
	flag.StringVar(&opt.database, "db", "", "Name of the database to use")
	flag.StringVar(&opt.loglvl, "l", "info", "Logging level")
	flag.IntVar(&opt.workers, "w", 100, "Number of maximum simultaneous connections to database")
	flag.BoolVar(&opt.usePass, "p", false, "Use a password to connect to database")
	flag.BoolVar(&opt.dryRun, "dry", false, "Replay the requests but don't write in the database")
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

	r, err := db.replay(f)
	if err != nil {
		logrus.Fatalf("cannot replay %s: %s", opt.kind, err)
	}

	if opt.dryRun {
		r.dryRun = "true"
	} else {
		r.dryRun = "false"
	}

	r.kind = opt.kind
	r.show()
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
	}

	if o.usePass {
		fmt.Printf("Password: ")
		bytes, err := terminal.ReadPassword(syscall.Stdin)
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
		db.kind = slowql.PCX
	default:
		return nil, errors.New("unknown kind " + o.kind)
	}

	db.datasource = fmt.Sprintf("%s:%s@tcp(%s)/%s", o.user, o.pass, o.host, o.database)
	db.drv, err = sql.Open("mysql", db.datasource)
	if err != nil {
		return nil, err
	}
	db.dryRun = o.dryRun

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
	db.logger.Debugf("workers number set to %s", db.wrks)

	return &db, nil
}

// replay replays the queries from a slow query log file to a database
func (db *database) replay(f io.Reader) (results, error) {
	var r results

	p := slowql.NewParser(db.kind, f)

	queries := make(chan string, 16384)
	errors := make(chan error, 16384)
	var wg sync.WaitGroup

	db.logger.Debug("starting workers pool")
	for i := 0; i < db.wrks; i++ {
		wg.Add(1)
		go db.worker(queries, errors, &wg)
	}

	db.logger.Debug("starting errors collector")
	go r.errorsCollector(errors)

	start := time.Now()
	db.logger.Infof("replay started on %s", time.Now().Format("Mon Jan 2 15:04:05"))
	s := newSpinner(34)
	s.Start()

	firstPass := true
	var previousDate time.Time
	for {
		q := p.GetNext()
		if q == (slowql.Query{}) {
			s.Stop()
			break
		}
		db.logger.Tracef("query: %s", q.Query)

		r.queries++
		s.Suffix = " queries replayed: " + strconv.Itoa(r.queries)

		if !db.dryRun {
			// Feed the workers with the query
			queries <- q.Query
		}

		// We need a reference time
		if firstPass {
			firstPass = false
			previousDate = q.Time
			continue
		}

		now := q.Time
		sleeping := now.Sub(previousDate)
		db.logger.Tracef("next sleeping time: %s", sleeping)
		time.Sleep(sleeping)

		// For MariaDB, when there is multiple queries in a short amount of
		// time, the Time field is not repeated, so we do not have to update
		// the previous date.
		if now != (time.Time{}) {
			previousDate = now
		}
	}
	close(queries)
	db.logger.Debug("closed queries channel")

	wg.Wait()
	close(errors)
	db.logger.Debug("closed errors channel")

	r.duration = time.Since(start)
	db.logger.Infof("replay ended on %s", time.Now().Format("Mon Jan 2 15:04:05"))
	return r, nil
}

func (r results) show() {
	data := []string{
		r.kind,
		r.dryRun,
		strconv.Itoa(r.queries),
		strconv.Itoa(r.errors),
		r.duration.String(),
	}
	t := newTable()
	t.Append(data)
	t.Render()
}

func newSpinner(t int) *spinner.Spinner {
	return spinner.New(spinner.CharSets[t], 100*time.Millisecond)
}

func newTable() *tablewriter.Table {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAlignment(tablewriter.ALIGN_CENTER)
	table.SetHeader([]string{"DB", "dry run", "Queries", "Errors", "Duration"})
	return table
}

func (db database) worker(queries chan string, errors chan error, wg *sync.WaitGroup) {
	for {
		select {
		case q, ok := <-queries:
			if !ok {
				db.logger.Trace("channel closed, worker exiting")
				wg.Done()
				return
			}
			rows, err := db.drv.Query(q)
			if err != nil {
				errors <- err
				db.logger.Debugf("failed to execute query:\n%s\nerror: %s", q, err)
			}
			if rows != nil {
				rows.Close()

			}
		}
	}
}

func (r *results) errorsCollector(errors chan error) {
	for {
		select {
		case _, ok := <-errors:
			if !ok {
				return
			}
			r.errors++
		}
	}
}
