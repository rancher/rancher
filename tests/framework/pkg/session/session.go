package session

import (
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

// CleanupFunc is the type RegisterCleanupFunc accepts
type CleanupFunc func() error

// Session is used to track resources created by tests by having a LIFO queue the keeps track of the delete functions.
type Session struct {
	CleanupEnabled bool
	cleanupQueue   []CleanupFunc
	open           bool
}

// NewSession is a constructor instantiates a new `Session`
func NewSession() *Session {
	return &Session{
		CleanupEnabled: true,
		cleanupQueue:   []CleanupFunc{},
		open:           true,
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

		// sometimes it is necessary to retry cleanup due to the webhook validator block initial delete attempts
		// due to using a stale cache
		var backoff = wait.Backoff{
			Duration: 100 * time.Millisecond,
			Factor:   1,
			Jitter:   0,
			Steps:    5,
		}

		for i := len(ts.cleanupQueue) - 1; i >= 0; i-- {
			var cleanupErr error
			err := wait.ExponentialBackoff(backoff, func() (done bool, err error) {
				cleanupErr = ts.cleanupQueue[i]()
				if cleanupErr != nil {
					return false, nil
				}
				return true, nil
			})
			if err != nil {
				logrus.Errorf("failed to cleanup resource. Backoff error: %v. Cleanup error: %v", err, cleanupErr)
			}
		}
		ts.cleanupQueue = []CleanupFunc{}
	}
}

// NewSession returns a `Session` who's cleanup method is registered with this `Session`
func (ts *Session) NewSession() *Session {
	sess := NewSession()

	ts.RegisterCleanupFunc(func() error {
		sess.Cleanup()
		return nil
	})

	return sess
}
