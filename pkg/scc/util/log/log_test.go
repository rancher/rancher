package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLog(t *testing.T) {
	assert.Nil(t, rootLog)
	_ = NewLog()
	assert.NotNil(t, rootLog)
}
