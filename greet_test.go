package deplorch

import "testing"

func TestGreet(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "named", input: "Val", want: "Hello, Val!"},
		{name: "empty", input: "", want: "Hello, stranger!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Greet(tt.input); got != tt.want {
				t.Errorf("Greet(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
