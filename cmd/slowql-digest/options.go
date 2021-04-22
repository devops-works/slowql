package main

import "errors"

func (o *options) parse() []error {
	var errs []error
	if o.logfile == "" {
		errs = append(errs, errors.New("no slow query log file provided"))
	} else if o.kind == "" {
		errs = append(errs, errors.New("no database kind provided"))
	} else if o.top <= 0 {
		errs = append(errs, errors.New("top cannot be negative or equal to zero"))
	} else if !stringInSlice(o.order, orders) {
		errs = append(errs, errors.New("unknown order"))
	}

	return errs
}

func stringInSlice(s string, sl []string) bool {
	for _, v := range sl {
		if s == v {
			return true
		}
	}
	return false
}
