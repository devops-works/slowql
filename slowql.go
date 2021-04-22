// Package slowql provides everything needed to parse slow query logs from
// different databases (such as MySQL, MariaDB).
// Along to a parser, it proposes a simple API with few functions that allow
// you to get everything needed to compute your slow queries.
package slowql

import (
	"bufio"
	"io"
	"strings"
	"time"

	"github.com/devops-works/slowql/database/mariadb"
	"github.com/devops-works/slowql/database/mysql"
	"github.com/devops-works/slowql/query"
	"github.com/devops-works/slowql/server"
	"github.com/sirupsen/logrus"
)

// Kind is a database kind
type Kind int

const (
	// Unknown type
	Unknown Kind = iota
	// MySQL type
	MySQL
	// MariaDB type
	MariaDB
	// PXC type
	PXC
)

// Database is the parser interface
type Database interface {
	// // GetNext returns the next query of the parser
	// GetNext() Query
	// // GetServerMeta returns informations about the SQL server in usage
	// GetServerMeta() Server
	ParseBlocks(rawBlocks chan []string)
	ParseServerMeta(chan []string)
	GetServerMeta() server.Server
}

// Parser holds a slowql parser
type Parser struct {
	db          Database
	waitingList chan query.Query
	rawBlocks   chan []string
	servermeta  chan []string
}

// NewParser returns a new parser depending on the desired kind
func NewParser(k Kind, r io.Reader) Parser {
	var p Parser

	p.rawBlocks = make(chan []string, 4096)
	p.servermeta = make(chan []string)
	p.waitingList = make(chan query.Query, 4096)

	go scan(*bufio.NewScanner(r), p.rawBlocks, p.servermeta)

	switch k {
	case MySQL, PXC:
		p.db = mysql.New(p.waitingList)
	case MariaDB:
		p.db = mariadb.New(p.waitingList)
	}

	p.db.ParseServerMeta(p.servermeta)
	go p.db.ParseBlocks(p.rawBlocks)

	// This is gross but we are sure that some queries will be already parsed at
	// when the user will call the package's functions
	time.Sleep(10 * time.Millisecond)
	return p
}

// GetNext returns the next query in line
func (p *Parser) GetNext() query.Query {
	var q query.Query
	select {
	case q = <-p.waitingList:
		return q
	case <-time.After(2 * time.Second):
		close(p.waitingList)
	}
	return q
}

// GetServerMeta returns server meta information
func (p *Parser) GetServerMeta() server.Server {
	return p.db.GetServerMeta()
}

func scan(s bufio.Scanner, rawBlocks, servermeta chan []string) {
	var bloc []string
	inHeader, inQuery := false, false

	// Parse the server informations
	var lines []string
	for i := 0; i < 3; i++ {
		s.Scan()
		lines = append(lines, s.Text())
	}
	servermeta <- lines

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
					rawBlocks <- bloc
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
	rawBlocks <- bloc

	close(rawBlocks)
}
