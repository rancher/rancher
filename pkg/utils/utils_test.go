package utils

import "testing"

func TestFormatPrefix(t *testing.T) {
	testStrings := []struct {
		s    string
		want string
	}{
		{
			"example", "example-",
		},
		{
			"Test", "test-",
		},
		{
			"another-", "another-",
		},
		{
			"", "",
		},
	}

	for _, tt := range testStrings {
		t.Run(tt.s, func(t *testing.T) {
			if v := FormatPrefix(tt.s); v != tt.want {
				t.Errorf("FormatPrefix() got %v, want %v", v, tt.want)
			}
		})
	}
}
