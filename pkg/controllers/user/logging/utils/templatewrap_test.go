package utils

import (
	"fmt"
	"testing"
)

func TestEscapeString(t *testing.T) {
	testDate := map[string]string{
		//before: after
		"test":     `"test"`,
		"\"test\"": `"\\\"test\\\""`,
		"\ttest":   `"\\ttest"`,
		"\rtest":   `"\\rtest"`,
		"\ntest":   `"\\ntest"`,
		"\btest":   `"\\btest"`,
		"\ftest":   `"\\ftest"`,
		"\r\ntest": `"\\r\\ntest"`,
		"\\test":   `"\\\\test"`,
	}

	for before, after := range testDate {
		if err := compareEscapeString(before, after); err != nil {
			t.Error(err)
		}
	}
}

func compareEscapeString(input, expected string) error {
	actual := escapeString(input)
	if actual != expected {
		return fmt.Errorf("string %s escape output %s not equal to expected %s", input, actual, expected)
	}
	return nil
}
