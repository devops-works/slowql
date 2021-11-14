package query

import "time"

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
	QCHit        bool
}
