package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNonTransientErrorUnwrap(t *testing.T) {
	t.Parallel()

	inner := fmt.Errorf("user not found")
	nte := &NonTransientError{Err: inner}

	assert.Equal(t, inner.Error(), nte.Error())
	assert.ErrorIs(t, nte, inner)
}
