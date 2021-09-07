package testsession

import (
	"testing"
)

type CleanupFunc func() error

type TestSession struct {
	cleanupQueue []CleanupFunc
	open         bool
	testingT     *testing.T
}

func NewTestSession(t *testing.T) *TestSession {
	return &TestSession{
		cleanupQueue: []CleanupFunc{},
		open:         true,
		testingT:     t,
	}
}

// RegisterCleanupFunc functions passed to this method will be called in the order they are added when `Cleanup` is called.
// If TestSession is locked, it will cause a panic if a new cleanup function is registered.
func (ts *TestSession) RegisterCleanupFunc(f CleanupFunc) {
	if ts.open {
		count := len(ts.cleanupQueue)
		ts.cleanupQueue = append(ts.cleanupQueue[:count], f)
	} else {
		panic("TestSession is locked, if function cannot be registered")
	}

}

// Cleanup this method will call all registered cleanup functions in order. Calling cleanup will lock the test session.  If a cleanup function returns an error it will be logged.
func (ts *TestSession) Cleanup() {
	ts.open = false
	count := len(ts.cleanupQueue) - 1
	for count >= 0 {
		err := ts.cleanupQueue[count]()
		count--
		if err != nil {
			ts.testingT.Errorf("error with cleanup: %v", err)
		}
	}
}

// NewSubSession returns a `TestSession` who's cleanup method is registered with this `TestSession`
func (ts *TestSession) NewSubSession(t *testing.T) *TestSession {
	testSession := &TestSession{
		cleanupQueue: []CleanupFunc{},
		open:         true,
		testingT:     t,
	}

	subSessionCleanupFunc := func() error {
		testSession.Cleanup()
		return nil
	}

	ts.RegisterCleanupFunc(subSessionCleanupFunc)
	return nil
}
