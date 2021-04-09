package main

import (
	"crypto/md5"
	"fmt"
	"regexp"
	"strings"
)

var regexeps []replacements

// replacements holds list of regexps we'll apply to queries for normalization
type replacements struct {
	Rexp *regexp.Regexp
	Repl string
}

// returns MD5 hash of the query
func hash(q string) string {
	data := []byte(q)
	return fmt.Sprintf("%x", md5.Sum(data))
}

// returns fingerprint of the query, which a generalised version of it acquired
// thanks to regexes
func fingerprint(q string) string {
	fingerprint := strings.ToLower(q)
	for _, r := range regexeps {
		fingerprint = r.Rexp.ReplaceAllString(fingerprint, r.Repl)
	}
	return fingerprint
}

func init() {
	// Regexps initialization
	// Create regexps entries for query normalization
	//
	// From pt-query-digest man page (package QueryRewriter section)
	//
	// 1·   Group all SELECT queries from mysqldump together, even if they are against different tables.
	//      The same applies to all queries from pt-table-checksum.
	// 2·   Shorten multi-value INSERT statements to a single VALUES() list.
	// 3·   Strip comments.
	// 4·   Abstract the databases in USE statements, so all USE statements are grouped together.
	// 5·   Replace all literals, such as quoted strings.  For efficiency, the code that replaces literal numbers is
	//      somewhat non-selective, and might replace some things as numbers when they really are not.
	//      Hexadecimal literals are also replaced.  NULL is treated as a literal.  Numbers embedded in identifiers are
	//	    also replaced, so tables named similarly will be fingerprinted to the same values
	//      (e.g. users_2009 and users_2010 will fingerprint identically).
	// 6·   Collapse all whitespace into a single space.
	// 7·   Lowercase the entire query.
	// 8·   Replace all literals inside of IN() and VALUES() lists with a single placeholder, regardless of cardinality.
	// 9·   Collapse multiple identical UNION queries into a single one.
	regexeps = []replacements{
		// 1·   Group all SELECT queries from mysqldump together
		// ... not implemented ...
		// 3·   Strip comments.
		{regexp.MustCompile(`(.*)/\*.*\*/(.*)`), "$1$2"},
		{regexp.MustCompile(`(.*) --.*`), "$1"},
		// 2·   Shorten multi-value INSERT statements to a single VALUES() list.
		{regexp.MustCompile(`^(insert .*) values.*`), "$1 values (?)"},
		// 4·   Abstract the databases in USE statements
		// ... not implemented ... since I don't really get it
		// 5·   Sort of...
		{regexp.MustCompile(`\s*([!><=]{1,2})\s*'[^']+'`), " $1 ?"},
		{regexp.MustCompile(`\s*([!><=]{1,2})\s*\x60[^\x60]+\x60`), " $1 ?"},
		{regexp.MustCompile(`\s*([!><=]{1,2})\s*[\.a-zA-Z0-9_-]+`), " $1 ?"},
		{regexp.MustCompile(`\s*(not)?\s+like\s+'[^']+'`), " not like ?"},
		// {regexp.MustCompile(`\s*(not)?\s+like\s+\x60[^\x60]+\x60`), " not like ?"}, // Not sure this one (LIKE `somestuff`) is necessary
		// 6·   Collapse all whitespace into a single space.
		{regexp.MustCompile(`[\s]{2,}`), " "},
		{regexp.MustCompile(`\s$`), ""}, // trim space at end
		// 7·   Lowercase the entire query.
		// ... implemented elsewhere ...
		// 8·   Replace all literals inside of IN() and VALUES() lists with a single placeholder
		// IN (...), VALUES, OFFSET
		{regexp.MustCompile(`in\s+\([^\)]+\)`), "in (?)"},
		// {regexp.MustCompile(`values\s+\([^\)]+\)`), "values (?)"},
		{regexp.MustCompile(`offset\s+\d+`), "offset ?"},
		// 9·   Collapse multiple identical UNION queries into a single one.
		// ... not implemented ...

	}
}
