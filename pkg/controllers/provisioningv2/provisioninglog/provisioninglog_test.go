package provisioninglog

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

var runes = []byte("abcdefghijklmnopqrstuvwxyz")

// generateTestLog creates a sample provisioning log, exceeding the maximum supplied length by one line. Each line is
// prefixed with the results of calling prefix with the current line number.
func generateTestLog(prefix func(int) string, length int) string {
	l := make([]byte, 0, length)
	for i := 0; ; i++ {
		l = append(l, []byte(prefix(i))...)
		for j := 0; j < len(runes); j++ {
			l = append(l, runes[j])
		}
		l = append(l, '\n')
		if len(l) > length {
			break
		}
	}
	return string(l)
}

func TestAppendLog(t *testing.T) {
	tests := []struct {
		name     string
		log      string
		msg      string
		expected string
	}{
		{
			name:     "first log",
			log:      "",
			msg:      "first log",
			expected: "first log\n",
		},
		{
			name:     "second log",
			log:      "first log\n",
			msg:      "second log",
			expected: "first log\nsecond log\n",
		},
		{
			name: "log exceeding max length", // strips the first 2 lines
			log: generateTestLog(func(i int) string {
				return fmt.Sprintf("[INFO] %d: ", i)
			}, maxLen),
			msg: "log exceeding max length",
			expected: generateTestLog(func(i int) string {
				return fmt.Sprintf("[INFO] %d: ", i+2)
			}, maxLen-45) + "log exceeding max length\n",
		},
		{
			name:     "long log without newline",
			log:      strings.Repeat("a", maxLen),
			msg:      "test",
			expected: "test\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, appendLog(tt.log, tt.msg))
		})
	}
}
