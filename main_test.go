package main

import (
	"slices"
	"testing"
)

func TestMergeEnvACMEDomains(t *testing.T) {
	tests := []struct {
		name       string
		flagValues []string
		env        string
		expected   []string
	}{
		{
			name:     "environment only",
			env:      " primary.example , secondary.example ",
			expected: []string{"primary.example", "secondary.example"},
		},
		{
			name:       "command line only",
			flagValues: []string{"primary.example"},
			expected:   []string{"primary.example"},
		},
		{
			name:       "environment comes first",
			flagValues: []string{"secondary.example"},
			env:        "primary.example",
			expected:   []string{"primary.example", "secondary.example"},
		},
		{
			name:       "duplicates and blanks are dropped",
			flagValues: []string{"secondary.example", "primary.example"},
			env:        "primary.example,,secondary.example",
			expected:   []string{"primary.example", "secondary.example"},
		},
		{
			name: "unset",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := mergeEnvACMEDomains(test.flagValues, test.env); !slices.Equal(got, test.expected) {
				t.Errorf("expected %q, got %q", test.expected, got)
			}
		})
	}
}
