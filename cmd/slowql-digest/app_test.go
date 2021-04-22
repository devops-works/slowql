package main

import (
	"testing"

	"github.com/devops-works/slowql"
	"github.com/sirupsen/logrus"
)

func Test_newApp(t *testing.T) {
	type args struct {
		loglevel string
		kind     string
	}
	tests := []struct {
		name     string
		args     args
		loglevel logrus.Level
		kind     slowql.Kind
		wantErr  bool
	}{
		{name: "trace - mariadb", args: args{loglevel: "trace", kind: "mariadb"},
			loglevel: logrus.TraceLevel, kind: slowql.MariaDB, wantErr: false},
		{name: "debug - pxc", args: args{loglevel: "debug", kind: "pxc"},
			loglevel: logrus.DebugLevel, kind: slowql.PXC, wantErr: false},
		{name: "info - mysql", args: args{loglevel: "info", kind: "mysql"},
			loglevel: logrus.InfoLevel, kind: slowql.MySQL, wantErr: false},
		{name: "warn - mysql", args: args{loglevel: "warn", kind: "mysql"},
			loglevel: logrus.WarnLevel, kind: slowql.MySQL, wantErr: false},
		{name: "error - mysql", args: args{loglevel: "error", kind: "mysql"},
			loglevel: logrus.ErrorLevel, kind: slowql.MySQL, wantErr: false},
		{name: "fatal - mysql", args: args{loglevel: "fatal", kind: "mysql"},
			loglevel: logrus.FatalLevel, kind: slowql.MySQL, wantErr: false},
		{name: "unknown - mysql", args: args{loglevel: "foobar", kind: "mysql"},
			loglevel: logrus.InfoLevel, kind: slowql.MySQL, wantErr: true},
		{name: "info - unknown", args: args{loglevel: "info", kind: "plop"},
			loglevel: logrus.InfoLevel, kind: slowql.MySQL, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newApp(tt.args.loglevel, tt.args.kind)
			if (err != nil) != tt.wantErr {
				t.Errorf("newApp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr == true {
				return
			}
			if got.logger.Level != tt.loglevel {
				t.Errorf("wrong log level: newApp() = %v, want %v", got.logger.Level, tt.loglevel)
			}
			if got.kind != tt.kind {
				t.Errorf("wrong kind: newApp() = %v, want %v", got.kind, tt.kind)
			}
		})
	}
}
