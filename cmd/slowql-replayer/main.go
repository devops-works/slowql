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
	usePass  bool
	dryRun   bool
}

type database struct {
	kind       slowql.Kind
	datasource string
	drv        *sql.DB
	dryRun     bool
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
	flag.BoolVar(&opt.usePass, "p", false, "Use a password to connect to database")
	flag.BoolVar(&opt.dryRun, "dry", false, "Replay the requests but don't write in the database")
	flag.Parse()

	if err := opt.parse(); err != nil {
		flag.Usage()
		logrus.Fatalf("cannot parse options: %s", err)
	}
	logrus.Infof("options parsed successfully")

	db, err := opt.createDB()
	if err != nil {
		logrus.Fatalf("cannot create databse object: %s", err)
	}
	defer db.drv.Close()
	logrus.Infof("database object created successfully")

	f, err := os.Open(opt.file)
	if err != nil {
		logrus.Fatalf("cannot open slow query log file: %s", err)
	}

	logrus.Infof("starting replay")
	r, err := db.replay(f)
	if err != nil {
		logrus.Fatalf("cannot replay %s: %s", opt.kind, err)
	}
	logrus.Infof("replay ended")

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
	default:
		return nil, errors.New("unknown kind " + o.kind)
	}

	db.datasource = fmt.Sprintf("%s:%s@tcp(%s)/%s", o.user, o.pass, o.host, o.database)
	db.drv, err = sql.Open("mysql", db.datasource)
	if err != nil {
		return nil, err
	}
	db.dryRun = o.dryRun
	return &db, nil
}

// replay replays the queries from a slow query log file to a database
func (db *database) replay(f io.Reader) (results, error) {
	var r results

	p := slowql.NewParser(db.kind, f)

	start := time.Now()
	s := newSpinner(34)
	s.Start()

	firstPass := true
	var previousDate time.Time
	for {
		q := p.GetNext()
		if q == (slowql.Query{}) {
			break
		}
		r.queries++
		s.Suffix = " queries replayed: " + strconv.Itoa(r.queries)

		if !db.dryRun {
			conn, err := db.drv.Query(q.Query)
			if err != nil {
				r.errors++
				// logrus.Errorf("failed to execute query:\n%s\nerror: %s", q.Query, err)
			}
			if conn != nil {
				conn.Close()
			}
		}

		// We need a reference time
		if firstPass {
			firstPass = false
			previousDate = q.Time
			continue
		}

		now := q.Time
		sleeping := now.Sub(previousDate)
		time.Sleep(sleeping)

		// For MariaDB, when there is multiple queries in a short amount of
		// time, the Time field is not repeated, so we do not have to update
		// the previous date.
		if now != (time.Time{}) {
			previousDate = now
		}
	}

	s.Stop()
	r.duration = time.Since(start)
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
