package embedded

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	assert.False(t, Has("nonexistent", "1.0.0"))

	rc, ok := Get("nonexistent", "1.0.0")
	assert.False(t, ok)
	assert.Nil(t, rc)
}
