package main

import "testing"

func TestGreeting(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "named", in: "ann", want: "hello, ann"},
		{name: "empty", in: "", want: "hello, world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := greeting(tt.in); got != tt.want {
				t.Errorf("greeting(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
