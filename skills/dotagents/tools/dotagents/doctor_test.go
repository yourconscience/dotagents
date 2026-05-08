package main

import "testing"

func TestParseMemsearchChunks(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   int
		okWant bool
	}{
		{
			name:   "parses chunks line",
			input:  "Total indexed chunks: 734\n",
			want:   734,
			okWant: true,
		},
		{
			name:   "parses with extra lines",
			input:  "foo\nTotal indexed chunks: 12\nbar\n",
			want:   12,
			okWant: true,
		},
		{
			name:   "missing line",
			input:  "no stats here",
			want:   0,
			okWant: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseMemsearchChunks(tc.input)
			if ok != tc.okWant {
				t.Fatalf("ok=%v want %v", ok, tc.okWant)
			}
			if got != tc.want {
				t.Fatalf("got=%d want %d", got, tc.want)
			}
		})
	}
}
