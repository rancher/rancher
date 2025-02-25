package ext

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBlockingListener(t *testing.T) {
	l := NewBlockingListener()

	start := time.Now()

	go func() {
		time.Sleep(time.Second * 2)
		err := l.Close()
		assert.NoError(t, err)
	}()

	_, err := l.Accept()
	end := time.Now()
	assert.ErrorContains(t, err, "listener is closed")

	blockTime := end.Sub(start)
	assert.GreaterOrEqual(t, blockTime, time.Second*2)

	_, err = l.Accept()
	assert.ErrorContains(t, err, "listener is closed")

	err = l.Close()
	assert.ErrorContains(t, err, "listener is already closed")
}
