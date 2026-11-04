package config

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDeferredStartEscalatesError(t *testing.T) {
	failed := make(chan struct{})
	w := &UserContext{
		ClusterName:          "c-m-test",
		OnDeferredStartError: func() { close(failed) },
	}

	starter := w.deferredStart(context.Background(), func() error {
		return errors.New("apiserver is unreachable")
	})

	starter()

	select {
	case <-failed:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for OnDeferredStartError")
	}
}
