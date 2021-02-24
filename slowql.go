package slowql

import (
	"bufio"
	"io"
	"strings"
	"time"

	"github.com/eze-kiel/dbg"
	"github.com/sirupsen/logrus"
)

type Parser struct {
	Source    io.Reader
	StackSize int
	stack     chan Query
	rawBlocs  chan []string
	scanner   bufio.Scanner
}

// Query contains query informations
type Query struct {
	Time         time.Time
	User         string
	Host         string
	IP           string
	ID           int
	QueryTime    time.Time
	LockTime     time.Time
	RowsSent     int
	RowsExamined int
	Timestamp    time.Time
	Query        string
}

// GetNext returns the next query
func (p Parser) GetNext() (Query, error) {
	var q Query

	select {
	case q := <-p.stack:
		return q, nil
	default:
		return q, io.EOF
	}
}

// NewParser creates the stack channel and launches background goroutines
func NewParser(r io.Reader) *Parser {
	var p Parser

	p.StackSize = 1024
	p.Source = r
	p.scanner = *bufio.NewScanner(r)

	p.stack = make(chan Query, p.StackSize)
	p.rawBlocs = make(chan []string, p.StackSize)

	go p.consume()
	go p.scan()

	return &p
}

// scan reads the source line by line, and send those lines to the parser one
// after each other
func (p *Parser) scan() {
	var bloc []string
	inHeader, inQuery := false, false

	for p.scanner.Scan() {
		line := p.scanner.Text()

		// Drop useless lines
		if p.scanner.Text()[0] == '#' && strings.Contains(p.scanner.Text(), "Quit;") {
			continue
		}

		/*
			This big if/else statement detects if the curernt line in a header
			or a request, and if it belongs to the same bloc or not
		*/
		// In header
		if strings.HasPrefix(line, "#") {
			inHeader = true
			if inQuery {
				// A new bloc is starting, we send the previous one if it is not
				// the first one
				inQuery = false
				dbg.Point(len(bloc))
				if len(bloc) != 0 {
					p.rawBlocs <- bloc
				}
			} else {
				// We are in the same header as before
				bloc = append(bloc, line)
			}
		} else { // In request
			inQuery = true
			if inHeader {
				// We were in an header, and now are in a request, but in the
				// same bloc
				inHeader = false
			}
			bloc = append(bloc, line)
		}
	}

	// In case of error, log it
	if err := p.scanner.Err(); err != nil {
		logrus.Error(err)
	}

	logrus.Infof("all the file has been parsed")

	// Send the last bloc
	p.rawBlocs <- bloc

	close(p.rawBlocs)
}

// consume consumes the received line to extract the informations, and send the
// Query object to the stack

// TODO(ezekiel): here do excatly the same thing as before, so instead of sending
// bloc, we could conusme the lines and return the query instead
func (p *Parser) consume() {
	select {
	case bloc := <-p.rawBlocs:
		dbg.Printf("received new bloc\n")
		var q Query

		// consume each line of the bloc
		for _, line := range bloc {
			if strings.HasPrefix(line, "#") {
				q.parseHeader(line)
			} else {
				q.parseRequest(line)
			}
		}
	}
}

func (q *Query) parseHeader(line string) {

}

func (q *Query) parseRequest(line string) {

}
