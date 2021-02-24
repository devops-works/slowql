package slowql

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Parser is the pârser object
type Parser struct {
	Source    io.Reader
	StackSize int
	stack     chan Query
	rawBlocs  chan []string
	scanner   bufio.Scanner
}

// Query contains query informations
type Query struct {
	Time         string
	User         string
	Host         string
	ID           int
	Schema       string
	LastErrNo    int
	Killed       int
	QueryTime    string
	LockTime     string
	RowsSent     int
	RowsExamined int
	RowsAffected int
	BytesSent    int
	Query        string
}

var re *regexp.Regexp

func init() {
	re = regexp.MustCompile(`\[(.*?)\]`)
}

// GetNext returns the next query
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

// Fingerprint returns Query.query's MD5 fingerprint
func (q *Query) Fingerprint() {

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
		if strings.Contains(p.scanner.Text(), "SET timestamp") {
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

// TODO(ezekiel): here do excatly the same thing as before, so instead of sending
// bloc, we could conusme the lines and return the query instead
func (p *Parser) consume() {
	for {
		select {
		case bloc := <-p.rawBlocs:
			var q Query

			// consume each line of the bloc
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

// parseHeader parses everything that begin with #
func (q *Query) parseHeader(line string) {
	var err error
	parts := strings.Split(line, " ")

	for idx, part := range parts {
		part = strings.ToLower(part)

		if strings.Contains(part, "query_time") {
			q.QueryTime = parts[idx+1]
		} else if strings.Contains(part, "lock_time") {
			q.LockTime = parts[idx+1]
		} else if strings.Contains(part, "time") {
			q.Time = parts[idx+1]
		} else if strings.Contains(part, "rows_sent") {
			q.RowsSent, err = strconv.Atoi(parts[idx+1])
			if err != nil {
				logrus.Errorf("error converting %s to int: %s", parts[idx+1], err)
			}
		} else if strings.Contains(part, "rows_examined") {
			q.RowsExamined, err = strconv.Atoi(parts[idx+1])
			if err != nil {
				logrus.Errorf("error converting %s to int: %s", parts[idx+1], err)
			}
		} else if strings.Contains(part, "rows_affected") {
			q.RowsAffected, err = strconv.Atoi(parts[idx+1])
			if err != nil {
				logrus.Errorf("error converting %s to int: %s", parts[idx+1], err)
			}
		} else if strings.Contains(part, "id") {
			q.ID, err = strconv.Atoi(parts[idx+1]) // TODO(ezekiel): this is gross, need to find an alternative
			if err != nil {
				logrus.Errorf("error converting %s to int: %s", parts[idx+1], err)
			}
		} else if strings.Contains(part, "user@host") {
			items := re.FindAllString(line, -1)
			// We remove first and last bytes of the strings because they are
			// square brackets
			q.User = items[0][1 : len(items[0])-1]
			q.Host = items[1][1 : len(items[1])-1]
		} else if strings.Contains(part, "schema") {
			q.Schema = parts[idx+1]
		} else if strings.Contains(part, "last_errno") {
			q.LastErrNo, err = strconv.Atoi(parts[idx+1])
			if err != nil {
				logrus.Errorf("error converting %s to int: %s", parts[idx+1], err)
			}
		} else if strings.Contains(part, "killed") {
			q.Killed, err = strconv.Atoi(parts[idx+1])
			if err != nil {
				logrus.Errorf("error converting %s to int: %s", parts[idx+1], err)
			}
		} else if strings.Contains(part, "bytes_sent") {
			q.BytesSent, err = strconv.Atoi(parts[idx+1])
			if err != nil {
				logrus.Errorf("error converting %s to int: %s", parts[idx+1], err)
			}
		}
	}
}
