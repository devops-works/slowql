package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"time"

	"github.com/devops-works/slowql"
	"github.com/devops-works/slowql/query"
	"github.com/sirupsen/logrus"
)

type options struct {
	filepath string
	file     *os.File
	debug    bool
	quiet    bool
}

type statistics struct {
	hash            string
	fingerprint     string
	schema          string
	calls           int
	cumErrored      int
	cumKilled       int
	cumQueryTime    time.Time
	cumLockTime     time.Time
	cumRowsSent     int
	cumRowsExamined int
	cumBytesSent    int
	concurrency     float64
	minTime         time.Time
	maxTime         time.Time
	meanTime        time.Time
	p50Time         time.Time
	p95Time         time.Time
	stddevTime      time.Time
}

type results map[string]statistics

func main() {
	var opt options
	flag.StringVar(&opt.filepath, "file", "", "Slow query log file")
	flag.BoolVar(&opt.debug, "debug", false, "Show debug logs")
	flag.BoolVar(&opt.quiet, "quiet", false, "Quiet mode: show only errors")
	flag.Parse()

	if err := opt.parse(); err != nil {
		logrus.Fatal(err)
	}

	lines, err := lineCounter(opt.file)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("file has %d lines", lines)

	p := slowql.NewParser(slowql.PXC, opt.file)
	logrus.Debug("slowql parser created successfully")

	for {
		q := p.GetNext()

		// If the query is empty, there is no more queries to get
		if q == (query.Query{}) {
			logrus.Debug("no more queries to get from the slow query log file")
			break
		}
	}
}

// parse parses the different flags
func (o *options) parse() error {
	var err error
	if o.filepath == "" {
		return errors.New("no file provided")
	}

	// Open file to obtain io.Reader
	o.file, err = os.Open(o.filepath)
	if err != nil {
		return err
	}
	logrus.Infof("using %s as input file", o.file.Name())

	// Set global log level depending on the different options that have been
	// provided
	logrus.SetLevel(logrus.InfoLevel)
	if o.debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if o.quiet {
		logrus.SetLevel(logrus.ErrorLevel)
	}

	return nil
}

// lineCounter returns the number of new line caracters that have been found in
// the io.Reader content
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
