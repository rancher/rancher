package broadcast

import (
	"context"
	"sync"
)

type GenericConnectFunc func() (chan interface{}, error)

type GenericBroadcaster struct {
	sync.Mutex
	running bool
	subs    map[chan interface{}]struct{}
}

func (b *GenericBroadcaster) Subscribe(ctx context.Context, connect GenericConnectFunc) (chan interface{}, error) {
	b.Lock()
	defer b.Unlock()

	if !b.running {
		if err := b.start(connect); err != nil {
			return nil, err
		}
	}

	sub := make(chan interface{}, 100)
	if b.subs == nil {
		b.subs = map[chan interface{}]struct{}{}
	}
	b.subs[sub] = struct{}{}
	go func() {
		<-ctx.Done()
		b.unsub(sub, true)
	}()

	return sub, nil
}

func (b *GenericBroadcaster) unsub(sub chan interface{}, lock bool) {
	if lock {
		b.Lock()
	}
	if _, ok := b.subs[sub]; ok {
		close(sub)
		delete(b.subs, sub)
	}
	if lock {
		b.Unlock()
	}
}

func (b *GenericBroadcaster) start(connect GenericConnectFunc) error {
	c, err := connect()
	if err != nil {
		return err
	}

	go b.stream(c)
	b.running = true
	return nil
}

func (b *GenericBroadcaster) stream(input chan interface{}) {
	for item := range input {
		b.Lock()
		for sub := range b.subs {
			select {
			case sub <- item:
			default:
				// Slow consumer, drop
				go b.unsub(sub, true)
			}
		}
		b.Unlock()
	}

	b.Lock()
	for sub := range b.subs {
		b.unsub(sub, false)
	}
	b.running = false
	b.Unlock()
}
