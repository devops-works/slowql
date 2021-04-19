package mariadb

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/devops-works/slowql/query"
	"github.com/devops-works/slowql/server"
)

// Database holds parser structure
type Database struct {
	WaitingList      chan query.Query
	ServerMeta       chan server.Server
	stringInBrackets *regexp.Regexp
	srv              server.Server
}

// New instance of parser
func New(qc chan query.Query) *Database {
	p := Database{
		WaitingList:      qc,
		stringInBrackets: regexp.MustCompile(`\[(.*?)\]`),
	}

	return &p
}

func (db *Database) ParseBlocs(rawBlocs chan []string) {
	for {
		select {
		case bloc := <-rawBlocs:
			var q query.Query

			for _, line := range bloc {
				if line[0] == '#' {
					db.parseMariaDBHeader(line, &q)
				} else {
					q.Query = q.Query + line
				}
			}
			db.WaitingList <- q
		}
	}
}

func (db *Database) parseMariaDBHeader(line string, q *query.Query) {
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
			items := db.stringInBrackets.FindAllString(line, -1)
			// We remove first and last bytes of the strings because they are
			// square brackets
			q.User = items[0][1 : len(items[0])-1]
			q.Host = items[1][1 : len(items[1])-1]

		} else if strings.Contains(part, "schema:") {
			q.Schema = parts[idx+1]

		} else if strings.Contains(part, "QC_hit") {
			q.QC_hit = true
			if parts[idx+1] == "No" {
				q.QC_hit = false
			}
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

func (db *Database) ParseServerMeta(lines chan []string) {
	header := <-lines
	versions := header[0]
	net := header[1]

	// Parse server information
	versionre := regexp.MustCompile(`^([^,]+),\s+Version:\s+([0-9\.]+)([A-Za-z0-9-]+)\s+\((.*)\)\. started`)
	matches := versionre.FindStringSubmatch(versions)

	if len(matches) != 5 {
		db.srv.Binary = "unable to parse line"
		db.srv.VersionShort = db.srv.Binary
		db.srv.Version = db.srv.Binary
		db.srv.VersionDescription = db.srv.Binary
		db.srv.Port = 0
		db.srv.Socket = db.srv.Binary
	} else {
		db.srv.Binary = matches[1]
		db.srv.VersionShort = matches[2]
		db.srv.Version = db.srv.VersionShort + matches[3]
		db.srv.VersionDescription = matches[4]
		db.srv.Port, _ = strconv.Atoi(strings.Split(net, " ")[2])
		db.srv.Socket = strings.TrimLeft(strings.Split(net, ":")[2], " ")
	}
}

func (d *Database) GetServerMeta() server.Server {
	return d.srv
}
