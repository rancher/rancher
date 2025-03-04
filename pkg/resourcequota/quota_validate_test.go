package resourcequota

import (
	"sync"
	"testing"
	"time"
)

func TestGetProjectLock(t *testing.T) {
	if c := projectLockProfile.Count(); c != 0 {
		t.Errorf("Count() = %d, want 0", c)
	}

	readyChan := make(chan struct{}, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func(ready chan<- struct{}) {
		defer wg.Done()
		pl := GetProjectLock("test-project")
		pl.Lock()
		defer pl.Unlock()
		ready <- struct{}{}

		time.Sleep(time.Second * 2)
	}(readyChan)

	go func(ready chan<- struct{}) {
		defer wg.Done()
		pl := GetProjectLock("other-project")
		pl.Lock()
		defer pl.Unlock()
		ready <- struct{}{}

		time.Sleep(time.Second * 1)
	}(readyChan)

	// Wait for the go-routines to lock.
	for i := 0; i < 2; i++ {
		_ = <-readyChan
	}

	if c := projectLockProfile.Count(); c != 2 {
		t.Errorf("Count() = %d, want 2", c)
	}

	wg.Wait()

	if c := projectLockProfile.Count(); c != 0 {
		t.Errorf("Count() = %d, want 0", c)
	}
}
