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
// If TestSession is closed, it will cause a panic if a new cleanup function is registered.
func (ts *TestSession) RegisterCleanupFunc(f CleanupFunc) {
	if ts.open {
		ts.cleanupQueue = append(ts.cleanupQueue, f)
	} else {
		panic("attempted to register cleanup function to closed test session")
	}

}

// Cleanup this method will call all registered cleanup functions in order and close the test session.
func (ts *TestSession) Cleanup() {
	ts.open = false

	for _, f := range ts.cleanupQueue {
		err := f()
		if err != nil {
			ts.testingT.Logf("error calling cleanup function: %v", err)
		}
	}
}

// NewSession returns a `TestSession` who's cleanup method is registered with this `TestSession`
func (ts *TestSession) NewSession() *TestSession {
	sess := NewTestSession(ts.testingT)

	ts.RegisterCleanupFunc(func() error {
		sess.Cleanup()
		return nil
	})

	return sess
}

// NewRancherClient returns a rancher client registered with this `TestSession`.
//rancherConfig *config.RancherClientConfiguration
// func (ts *TestSession) NewRancherClient() *rancherclient.RancherClient {
// 	panic("impl me")
// }
