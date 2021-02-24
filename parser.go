package slowql

import (
	"bufio"
	"io"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Parser holds all the informations to read and consume a log file.
type Parser struct {
	// Source is the input io.Reader
	Source io.Reader

	// StackSize is the size of the buffered channel from which GetNext() will
	// fetch queries. The larger the number, the more CPU will be used at startup
	StackSize int

	stack    chan Query
	rawBlocs chan []string
	scanner  bufio.Scanner
}

// NewParser returns a new slowql.Parser instance.
// It also starts everything needed to parse logs file, which are 2 goroutines.
// They do what they have to and require no interaction from the user. Once the
// job terminated, they terminate gracefully.
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

// GetNext returns the next query. Some fields of Query may be empty, depending
// on what has been parsed from the log file.
func (p Parser) GetNext() (Query, error) {
	var q Query
	select {
	case q := <-p.stack:
		return q, nil
	case <-time.After(time.Second * 10):
		close(p.stack)
	}
	return q, nil
}

// scan reads the source line by line, and send those lines to the parser one
// after each other
func (p *Parser) scan() {
	var bloc []string
	inHeader, inQuery := false, false

	for p.scanner.Scan() {
		line := p.scanner.Text()
		// Drop useless lines
		if strings.Contains(p.scanner.Text(), "SET timestamp") {
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
					p.rawBlocs <- bloc
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
	if err := p.scanner.Err(); err != nil {
		logrus.Error(err)
	}

	// Send the last bloc
	p.rawBlocs <- bloc

	close(p.rawBlocs)
}

// consume consumes the received line to extract the informations, and send the
// Query object to the stack
func (p *Parser) consume() {
	for {
		select {
		case bloc := <-p.rawBlocs:
			var q Query

			for _, line := range bloc {
				if strings.HasPrefix(line, "#") {
					q.parseHeader(line)
				} else {
					q.Query = q.Query + line
				}
			}

			p.stack <- q
		}
	}
}
