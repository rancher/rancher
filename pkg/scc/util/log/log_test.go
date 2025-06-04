package log

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewLog(t *testing.T) {
	assert.Nil(t, rootLog)
	_ = NewLog()
	assert.NotNil(t, rootLog)
}
