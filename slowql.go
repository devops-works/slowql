// Package slowql provides everything needed to parse slow query logs from
// different databases (such as MySQL, MariaDB).
// Along to a parser, it proposes a simple API with few functions that allow
// you to get everything needed to compute your slow queries.
package slowql

import (
	"regexp"
)

var re *regexp.Regexp

func init() {
	re = regexp.MustCompile(`\[(.*?)\]`)
}
