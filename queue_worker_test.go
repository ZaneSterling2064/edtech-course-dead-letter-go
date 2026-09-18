package main

import "testing"

func TestDecideDeadLetter(t *testing.T) {
	tests := []struct {
		name     string
		attempts int
		want     Decision
	}{
		{"first failure retries", 1, Retry},
		{"last allowed attempt retries", 2, Retry},
		{"third failure is dead letter", 3, DeadLetter},
		{"later failures stay dead letter", 5, DeadLetter},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decide(tt.attempts); got != tt.want {
				t.Fatalf("decide(%d) = %q, want %q", tt.attempts, got, tt.want)
			}
		})
	}
}
