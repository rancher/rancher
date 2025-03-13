package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_calculateBackoff(t *testing.T) {
	tests := []struct {
		name            string
		numberOfRetries int
	}{
		{
			name:            "initial retry",
			numberOfRetries: 0,
		},
		{
			name:            "second retry",
			numberOfRetries: 1,
		},
		{
			name:            "third retry",
			numberOfRetries: 2,
		},
		{
			name:            "large number of retries",
			numberOfRetries: 100,
		},
		{
			name:            "very large number of retries",
			numberOfRetries: 10000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateBackoff(tt.numberOfRetries)
			assert.GreaterOrEqual(t, got, 10*time.Second, "calculateBackoff() = %v, want greater than or equal to 10 seconds", got)
			assert.LessOrEqual(t, got, 30*time.Minute, "calculateBackoff() = %v, want less than or equal to 30 minutes", got)
		})
	}
}
