package rkenodeconfigserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_compareTag(t *testing.T) {
	assert.False(t, compareTag("v1.11.0-xx", "v1.12.0-xx"))
	assert.True(t, compareTag("v1.12.0-xx", "v1.12.0-xx"))
	assert.True(t, compareTag("v1.13.0-xx", "v1.12.0"))
}
