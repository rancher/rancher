package session

import (
	"testing"
)

// CleanupFunc is the type RegisterCleanupFunc accepts
type CleanupFunc func() error

// Session is used to track resources created by tests by having a LIFO queue the keeps track of the delete functions.
type Session struct {
	CleanupEnabled bool
	cleanupQueue   []CleanupFunc
	open           bool
	TestingT       *testing.T
}

// NewSession is a constructor instantiates a new `Session`
func NewSession(t *testing.T) *Session {
	return &Session{
		CleanupEnabled: true,
		cleanupQueue:   []CleanupFunc{},
		open:           true,
		TestingT:       t,
	}
}

// RegisterCleanupFunc is function registers clean up functions in the `Session` queue.
// Functions passed to this method will be called in the order they are added when `Cleanup` is called.
// If Session is closed, it will cause a panic if a new cleanup function is registered.
func (ts *Session) RegisterCleanupFunc(f CleanupFunc) {
	if ts.open {
		ts.cleanupQueue = append(ts.cleanupQueue, f)
	} else {
		panic("attempted to register cleanup function to closed test session")
	}

}

// Cleanup this method will call all registered cleanup functions in order and close the test session.
func (ts *Session) Cleanup() {
	if ts.CleanupEnabled {
		ts.open = false
		for i := len(ts.cleanupQueue) - 1; i >= 0; i-- {
			err := ts.cleanupQueue[i]()
			if err != nil {
				ts.TestingT.Logf("error calling cleanup function: %v", err)
			}
		}
		ts.cleanupQueue = []CleanupFunc{}
	}
}

// NewSession returns a `Session` who's cleanup method is registered with this `Session`
func (ts *Session) NewSession() *Session {
	sess := NewSession(ts.TestingT)

	ts.RegisterCleanupFunc(func() error {
		sess.Cleanup()
		return nil
	})

	return sess
}
