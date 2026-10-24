package main

import (
	"slices"
	"testing"
)

func TestAppendEnvACMEDomains(t *testing.T) {
	tests := []struct {
		name     string
		domains  []string
		env      string
		expected []string
	}{
		{
			name:     "environment only, already parsed by cli",
			domains:  []string{"primary.example", "secondary.example"},
			env:      " primary.example , secondary.example ",
			expected: []string{"primary.example", "secondary.example"},
		},
		{
			name:     "command line only",
			domains:  []string{"primary.example"},
			expected: []string{"primary.example"},
		},
		{
			name:     "environment and command line are merged",
			domains:  []string{"secondary.example"},
			env:      "primary.example,,secondary.example",
			expected: []string{"secondary.example", "primary.example"},
		},
		{
			name: "unset",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := appendEnvACMEDomains(test.domains, test.env); !slices.Equal(got, test.expected) {
				t.Errorf("expected %q, got %q", test.expected, got)
			}
		})
	}
}
