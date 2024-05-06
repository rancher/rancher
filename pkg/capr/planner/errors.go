package planner

import (
	"errors"
	"fmt"
)

// errWaiting will not cause a re-enqueue of the object being processed and should be used when waiting for other objects/controllers
type errWaiting string

func (e errWaiting) Error() string {
	return string(e)
}

// errWaitingf renders an error of type errWaiting that will not cause a re-enqueue of the object being processed and should be used when waiting for other objects/controllers
func errWaitingf(format string, a ...interface{}) errWaiting {
	return errWaiting(fmt.Sprintf(format, a...))
}

func IsErrWaiting(err error) bool {
	var errWaiting errWaiting
	return errors.As(err, &errWaiting)
}

// errIgnore is specifically used during plan processing to ignore internal processing errors
type errIgnore string

func (e errIgnore) Error() string {
	return string(e)
}

// ignoreErrors accepts two errors. If the err is type errIgnore, it will return (err, nil) if firstIgnoreErr is nil or (firstIgnoreErr, nil).
// Otherwise, it will simply return (firstIgnoreErr, err)
func ignoreErrors(firstIgnoreError error, err error) (error, error) {
	var errIgnore errIgnore
	if errors.As(err, &errIgnore) {
		if firstIgnoreError == nil {
			return err, nil
		}
		return firstIgnoreError, nil
	}
	return firstIgnoreError, err
}
