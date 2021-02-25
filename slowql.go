// Package slowql provides everything needed to parse slow query logs from
// different databases (such as MySQL, MariaDB).
// Along to a parser, it proposes a simple API with few functions that allow
// you to get everything needed to compute your slow queries.
package slowql

import (
	"bufio"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var stringInBrackets *regexp.Regexp

func init() {
	stringInBrackets = regexp.MustCompile(`\[(.*?)\]`)
}

// Query is a single SQL query and the data associated
type Query struct {
	Time         time.Time
	QueryTime    float64
	LockTime     float64
	ID           int
	RowsSent     int
	RowsExamined int
	RowsAffected int
	LastErrNo    int
	Killed       int
	BytesSent    int
	User         string
	Host         string
	Schema       string
	Query        string
}

// Kind is a database kind
type Kind int

// Parser is the parser interface
type Parser interface {
	// GetNext returns the next query of the parser
	GetNext() Query
	parseBlocs(rawBlocs chan []string)
}

// NewParser returns a new parser depending on the desired kind
func NewParser(k Kind, r io.Reader) Parser {
	var p Parser

	rawBlocs := make(chan []string, 1024)
	waitingList := make(chan Query, 1024)
	go scan(*bufio.NewScanner(r), rawBlocs)

	switch k {
	case 0:
		p = &mysqlParser{
			wl: waitingList,
		}
	case 1:
		p = &mariadbParser{
			wl: waitingList,
		}
	}

	go p.parseBlocs(rawBlocs)
	return p
}

func scan(s bufio.Scanner, rawBlocs chan []string) {
	var bloc []string
	inHeader, inQuery := false, false

	// Skip the first three lines of the log file
	for i := 0; i < 3; i++ {
		s.Scan()
	}

	for s.Scan() {
		line := s.Text()
		// Drop useless lines
		if strings.Contains(s.Text(), "SET timestamp") {
			continue
		}

		// This big if/else statement detects if the curernt line in a header
		// or a request, and if it belongs to the same bloc or not
		// In header
		if strings.HasPrefix(line, "#") {
			inHeader = true
			if inQuery {
				// A new bloc is starting, we send the previous one if it is not
				// the first one
				inQuery = false
				if len(bloc) > 0 {
					rawBlocs <- bloc
					bloc = nil
				}
			}
		} else { // In request
			inQuery = true
			if inHeader {
				// We were in an header, and now are in a request, but in the
				// same bloc
				inHeader = false
			}
		}
		bloc = append(bloc, line)
	}

	// In case of error, log it
	if err := s.Err(); err != nil {
		logrus.Error(err)
	}

	// Send the last bloc
	rawBlocs <- bloc

	close(rawBlocs)
}
