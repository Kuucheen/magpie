package checker

import "testing"

func TestCheckerHTTPClientCacheEntriesForThreads(t *testing.T) {
	tests := []struct {
		name    string
		threads uint32
		want    int
	}{
		{name: "minimum clamp at low thread count", threads: 1, want: 2048},
		{name: "2000 threads", threads: 2000, want: 8192},
		{name: "5000 threads", threads: 5000, want: 16384},
		{name: "max clamp at very high thread count", threads: 7000, want: 16384},
	}

	for _, tc := range tests {
		if got := checkerHTTPClientCacheEntriesForThreads(tc.threads); got != tc.want {
			t.Fatalf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestNextPow2(t *testing.T) {
	tests := []struct {
		in   uint64
		want uint64
	}{
		{in: 0, want: 1},
		{in: 1, want: 1},
		{in: 2, want: 2},
		{in: 3, want: 4},
		{in: 7, want: 8},
		{in: 8, want: 8},
		{in: 9, want: 16},
	}

	for _, tc := range tests {
		if got := nextPow2(tc.in); got != tc.want {
			t.Fatalf("nextPow2(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
