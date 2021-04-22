package main

import (
	"reflect"
	"testing"

	"github.com/devops-works/slowql/server"
)

func Test_getMeta(t *testing.T) {
	type args struct {
		srv server.Server
	}
	tests := []struct {
		name string
		args args
		want serverMeta
	}{
		{
			name: "test",
			args: args{
				srv: server.Server{
					Binary:             "mybin",
					Port:               1337,
					Socket:             "nice socks dude",
					Version:            "1.2.3-leet",
					VersionShort:       "1.2.3",
					VersionDescription: "some leet version",
				},
			},
			want: serverMeta{
				Binary:             "mybin",
				Port:               1337,
				Socket:             "nice socks dude",
				Version:            "1.2.3-leet",
				VersionShort:       "1.2.3",
				VersionDescription: "some leet version",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getMeta(tt.args.srv); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getMeta() = %v, want %v", got, tt.want)
			}
		})
	}
}
