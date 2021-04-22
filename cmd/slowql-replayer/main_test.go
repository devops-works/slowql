package main

import (
	"errors"
	"reflect"
	"testing"
)

func Test_options_parse(t *testing.T) {
	type opt struct {
		user       string
		host       string
		pass       string
		file       string
		kind       string
		database   string
		loglvl     string
		pprof      string
		workers    int
		factor     float64
		usePass    bool
		noDryRun   bool
		showErrors bool
	}
	tests := []struct {
		name string
		opt  opt
		want []error
	}{
		{
			name: "no errors",
			opt: opt{
				user:     "foo",
				host:     "bar",
				file:     "myfile",
				kind:     "mykind",
				database: "mydb",
				workers:  42,
				factor:   2,
			},
			want: nil,
		},
		{
			name: "one error",
			opt: opt{
				user:     "foo",
				host:     "bar",
				file:     "myfile",
				kind:     "mykind",
				database: "mydb",
				// workers:  42,
				factor: 2,
			},
			want: []error{errors.New("cannot create negative number or zero workers")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &options{
				user:       tt.opt.user,
				host:       tt.opt.host,
				pass:       tt.opt.pass,
				file:       tt.opt.file,
				kind:       tt.opt.kind,
				database:   tt.opt.database,
				loglvl:     tt.opt.loglvl,
				pprof:      tt.opt.pprof,
				workers:    tt.opt.workers,
				factor:     tt.opt.factor,
				usePass:    tt.opt.usePass,
				noDryRun:   tt.opt.noDryRun,
				showErrors: tt.opt.showErrors,
			}
			if got := o.parse(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("options.parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
