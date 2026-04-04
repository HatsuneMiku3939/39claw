package main

import "testing"

func TestGreeting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{
			name: "returns dummy hello world message",
			want: "hello world",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := greeting(); got != tt.want {
				t.Fatalf("greeting() = %q, want %q", got, tt.want)
			}
		})
	}
}
