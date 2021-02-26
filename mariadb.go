package slowql

import (
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// MariaDB is the MariaDB kind
const MariaDB Kind = 1

type mariadbParser struct {
	wl chan Query
}

func (p *mariadbParser) parseBlocs(rawBlocs chan []string) {
	for {
		select {
		case bloc := <-rawBlocs:
			var q Query

			for _, line := range bloc {
				if strings.HasPrefix(line, "#") {
					q.parseMariaDBHeader(line)
				} else {
					q.Query = q.Query + line
				}
			}
			p.wl <- q
		}
	}
}

func (p *mariadbParser) GetNext() Query {
	var q Query
	select {
	case q = <-p.wl:
		return q
	case <-time.After(2 * time.Second):
		close(p.wl)
	}
	return q
}

func (q *Query) parseMariaDBHeader(line string) {
	var err error
	parts := strings.Split(line, " ")

	for idx, part := range parts {
		part = strings.ToLower(part)

		if strings.Contains(part, "query_time:") {
			time := parts[idx+1]
			q.QueryTime, err = strconv.ParseFloat(time, 64)
			if err != nil {
				logrus.Errorf("query_time: error converting %s to time: %s", parts[idx+1], err)
			}

		} else if strings.Contains(part, "lock_time:") {
			time := parts[idx+1]
			q.LockTime, err = strconv.ParseFloat(time, 64)
			if err != nil {
				logrus.Errorf("lock_time: error converting %s to time: %s", parts[idx+1], err)
			}

		} else if strings.Contains(part, "time:") {
			date := parts[idx+1] + " " + parts[idx+2]
			q.Time, err = time.Parse("060102 15:04:05", date)
			if err != nil {
				logrus.Errorf("time: error converting %s to time: %s", parts[idx+1], err)
			}

		} else if strings.Contains(part, "rows_sent:") {
			q.RowsSent, err = strconv.Atoi(parts[idx+1])
			if err != nil {
				logrus.Errorf("row_sent: error converting %s to int: %s", parts[idx+1], err)
			}

		} else if strings.Contains(part, "rows_examined:") {
			q.RowsExamined, err = strconv.Atoi(parts[idx+1])
			if err != nil {
				logrus.Errorf("rows_examined: error converting %s to int: %s", parts[idx+1], err)
			}

		} else if strings.Contains(part, "rows_affected:") {
			q.RowsAffected, err = strconv.Atoi(parts[idx+1])
			if err != nil {
				logrus.Errorf("rows_affected: error converting %s to int: %s", parts[idx+1], err)
			}

		} else if strings.Contains(part, "id:") {
			// Some IDs can have multiple spaces, so we try to bruteforce the
			// number of spaces. I tried implementing a version that keeps in
			// memory the correct index after the first pass, but it was not
			// faster that re-calculating it at each pass
			item := ""
			for item == "" {
				idx++
				item = parts[idx]
			}
			q.ID, err = strconv.Atoi(parts[idx])
			if err != nil {
				logrus.Errorf("id: error converting %s to int: %s", parts[idx+1], err)
			}

		} else if strings.Contains(part, "user@host:") {
			items := stringInBrackets.FindAllString(line, -1)
			// We remove first and last bytes of the strings because they are
			// square brackets
			q.User = items[0][1 : len(items[0])-1]
			q.Host = items[1][1 : len(items[1])-1]

		} else if strings.Contains(part, "schema:") {
			q.Schema = parts[idx+1]

		} else if strings.Contains(part, "last_errno:") {
			q.LastErrNo, err = strconv.Atoi(parts[idx+1])
			if err != nil {
				logrus.Errorf("last_errno: error converting %s to int: %s", parts[idx+1], err)
			}

		} else if strings.Contains(part, "killed:") {
			q.Killed, err = strconv.Atoi(parts[idx+1])
			if err != nil {
				logrus.Errorf("killed: error converting %s to int: %s", parts[idx+1], err)
			}

		} else if strings.Contains(part, "bytes_sent:") {
			q.BytesSent, err = strconv.Atoi(parts[idx+1])
			if err != nil {
				logrus.Errorf("bytes_sent: error converting %s to int: %s", parts[idx+1], err)
			}
		}
	}
}
