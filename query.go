package slowql

import (
	"crypto/md5"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

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

// Fingerprint returns Query.query's MD5 fingerprint.
func (q Query) Fingerprint() string {
	h := md5.New()
	io.WriteString(h, q.Query)
	return fmt.Sprintf("%x", h.Sum(nil))
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
			q.ID, err = strconv.Atoi(parts[idx+1]) // TODO(ezekiel): find an other way to get the ID, as the number of spaces can vary
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
