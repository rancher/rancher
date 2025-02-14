package listener

import (
	"sync"
)

// WaitLoader blocks until the underlying value is set.
type WaitLoader[T any] struct {
	locker sync.RWMutex

	vMu sync.RWMutex
	v   *T
}

func NewWaitLoader[T any]() *WaitLoader[T] {
	wl := &WaitLoader[T]{}
	wl.Unset()
	return wl
}

// Load blocks until there is a value to return.
func (w *WaitLoader[T]) Load() T {
	w.locker.RLock()
	defer w.locker.RUnlock()

	return *w.v
}

// Set the value and unblock any calls to Load.
func (w *WaitLoader[T]) Set(v T) {
	w.vMu.Lock()
	defer w.vMu.Unlock()

	w.v = &v

	// To avoid unlocking an unlocked mutex, frist try to lock it.
	// If unlocked this will take the lock before unlocking
	// If locked this will just unlock
	_ = w.locker.TryLock()
	w.locker.Unlock()
}

// Unset the value and unublock calls to Load.
func (w *WaitLoader[T]) Unset() {
	w.vMu.Lock()
	defer w.vMu.Unlock()

	_ = w.locker.TryLock()
	w.v = nil
}
