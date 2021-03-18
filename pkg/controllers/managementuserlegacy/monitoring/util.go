package monitoring

import (
	"regexp"
)

var (
	numberRegex = regexp.MustCompile("^[0-9]+$")
)

// detectNotForcedStringKeys builds a list of known keys that should NOT be
// forced to a string, e.g. keys that are required to be "true", "false", "123",
// "2379"
func detectNotForcedStringKeys(answers map[string]string) []string {
	var notForcedStringKeys []string
	for k, v := range answers {
		// all keys with boolean or number values must NOT be forced to be a string
		if v == "true" || v == "false" || numberRegex.MatchString(v) {
			notForcedStringKeys = append(notForcedStringKeys, k)
		}
	}
	return notForcedStringKeys
}
