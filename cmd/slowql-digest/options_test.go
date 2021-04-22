package main

import (
	"testing"
)

func Test_options_parse(t *testing.T) {
	type fields struct {
		logfile  string
		loglevel string
		kind     string
		top      int
		order    string
		dec      bool
		nocache  bool
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{name: "working", fields: fields{logfile: "file", kind: "mysql", top: 1337, order: "random"}, wantErr: false},
		{name: "no logfile", fields: fields{kind: "mysql", top: 1337, order: "random"}, wantErr: true},
		{name: "no kind", fields: fields{logfile: "file", top: 1337, order: "random"}, wantErr: true},
		{name: "incorrect top", fields: fields{logfile: "file", kind: "mysql", top: -1000, order: "random"}, wantErr: true},
		{name: "incorrect order", fields: fields{logfile: "file", kind: "mysql", top: -1000, order: "incorrect"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &options{
				logfile:  tt.fields.logfile,
				loglevel: tt.fields.loglevel,
				kind:     tt.fields.kind,
				top:      tt.fields.top,
				order:    tt.fields.order,
				dec:      tt.fields.dec,
				nocache:  tt.fields.nocache,
			}
			got := o.parse()

			if len(got) > 0 && tt.wantErr == false {
				t.Errorf("options.parse() = %v, want %v", got, tt.wantErr)
			}
		})
	}
}
