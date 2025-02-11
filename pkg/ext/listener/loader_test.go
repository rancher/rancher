package listener

import (
	"testing"
	"time"
)

const (
	DefaultBlockingTestTimeout = time.Second * 5
)

func assertBlocksUntil(t *testing.T, blocker func(), unblocker func(), timeout time.Duration) {
	t.Helper()

	blockChan := make(chan struct{})

	go func() {
		blocker()
		close(blockChan)
	}()

	unblocker()

	select {
	case <-blockChan:
	case <-time.After(timeout):
		t.Error("blocker func in not unblocked by unblocker func")
	}
}

func TestLoaderLoadBlocksUntilSet(t *testing.T) {
	wl := NewWaitLoader[int]()

	assertBlocksUntil(
		t,
		func() {
			wl.Load()
		},
		func() {
			wl.Set(0)
		},
		DefaultBlockingTestTimeout,
	)
}

func TestLoaderLoadDoesNotBlockAfterSec(t *testing.T) {
	wl := NewWaitLoader[int]()

	assertBlocksUntil(
		t,
		func() {
			wl.Load()
			wl.Load()
		},
		func() {
			wl.Set(0)
		},
		DefaultBlockingTestTimeout,
	)
}

func TestLoaderUnsetBlocksLoadUntilSet(t *testing.T) {
	wl := NewWaitLoader[int]()
	wl.Set(0)
	wl.Unset()

	assertBlocksUntil(
		t,
		func() {
			wl.Load()
		},
		func() {
			wl.Set(0)
		},
		DefaultBlockingTestTimeout,
	)
}
