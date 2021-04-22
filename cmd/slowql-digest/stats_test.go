package main

import (
	"testing"
	"time"
)

func Test_fsecsToDuration(t *testing.T) {
	tests := []struct {
		name string
		d    float64
		want time.Duration
	}{
		{name: "second", d: 1.0, want: time.Second},
		{name: "millisecond", d: 0.001, want: time.Millisecond},
		{name: "microsecond", d: 0.000001, want: time.Microsecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fsecsToDuration(tt.d); got != tt.want {
				t.Errorf("fsecsToDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hash(t *testing.T) {
	tests := []struct {
		name string
		q    string
		want string
	}{
		{name: "foobar", q: "foobar", want: "3858f62230ac3c915f300c664312c63f"},
		{name: "some long string", q: "some long string", want: "2fb66bbfb88cdf9e07a3f1d1dfad71ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hash(tt.q); got != tt.want {
				t.Errorf("hash() = %v, want %v", got, tt.want)
			}
		})
	}
}
