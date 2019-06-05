package nodepool

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parsePrefix(t *testing.T) {

	tests := []struct {
		name          string
		fullPrefix    string
		wantPrefix    string
		wantMinLength int
		wantStart     int
	}{{
		name:          "prefix with an 2 digit integer",
		fullPrefix:    "my-worker25",
		wantPrefix:    "my-worker",
		wantMinLength: 2,
		wantStart:     25,
	}, {
		name:          "prefix with an 1 digit integer",
		fullPrefix:    "pool4",
		wantPrefix:    "pool",
		wantMinLength: 1,
		wantStart:     4,
	}, {
		name:          "default case",
		fullPrefix:    "genericNodepool",
		wantPrefix:    "genericNodepool",
		wantMinLength: 1,
		wantStart:     1,
	},
	}
	for _, tt := range tests {

		gotPrefix, gotMinLength, gotStart := parsePrefix(tt.fullPrefix)

		assert.Equal(t, tt.wantPrefix, gotPrefix)
		assert.Equal(t, tt.wantMinLength, gotMinLength)
		assert.Equal(t, tt.wantStart, gotStart)
	}
}
