package kafka

import (
	"context"
	"sync"
)

// runGroup is a collection of goroutines working together. If any one goroutine
// stops, then all goroutines will be stopped.
//
// A zero runGroup is valid
type runGroup struct {
	initOnce sync.Once

	ctx    context.Context
	cancel context.CancelFunc

	wg sync.WaitGroup
}

func (r *runGroup) init() {
	if r.cancel == nil {
		r.ctx, r.cancel = context.WithCancel(context.Background())
	}
}

func (r *runGroup) WithContext(ctx context.Context) *runGroup {
	ctx, cancel := context.WithCancel(ctx)
	return &runGroup{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Wait blocks until all function calls have returned.
func (r *runGroup) Wait() {
	r.wg.Wait()
}

// Stop stops the goroutines and waits for them to complete
func (r *runGroup) Stop() {
	r.initOnce.Do(r.init)
	r.cancel()
	r.Wait()
}

// Go calls the given function in a new goroutine.
//
// The first call to return a non-nil error cancels the group; its error will be
// returned by Wait.
func (r *runGroup) Go(f func(stop <-chan struct{})) {
	r.initOnce.Do(r.init)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer r.cancel()

		f(r.ctx.Done())
	}()
}
