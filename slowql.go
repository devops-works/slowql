package slowql

import (
	"regexp"
)

var re *regexp.Regexp

func init() {
	re = regexp.MustCompile(`\[(.*?)\]`)
}
