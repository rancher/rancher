package broadcast

import (
	"context"
	"sync"
)

type ConnectFunc func() (chan map[string]interface{}, error)

type Broadcaster struct {
	sync.Mutex
	running bool
	subs    map[chan map[string]interface{}]struct{}
}

func (b *Broadcaster) Subscribe(ctx context.Context, connect ConnectFunc) (chan map[string]interface{}, error) {
	b.Lock()
	defer b.Unlock()

	if !b.running {
		if err := b.start(connect); err != nil {
			return nil, err
		}
	}

	sub := make(chan map[string]interface{}, 100)
	if b.subs == nil {
		b.subs = map[chan map[string]interface{}]struct{}{}
	}
	b.subs[sub] = struct{}{}
	go func() {
		<-ctx.Done()
		b.Lock()
		b.unsub(sub)
		b.Unlock()
	}()

	return sub, nil
}

func (b *Broadcaster) unsub(sub chan map[string]interface{}) {
	if _, ok := b.subs[sub]; ok {
		close(sub)
		delete(b.subs, sub)
	}
}

func (b *Broadcaster) start(connect ConnectFunc) error {
	c, err := connect()
	if err != nil {
		return err
	}

	go b.stream(c)
	b.running = true
	return nil
}

func (b *Broadcaster) stream(input chan map[string]interface{}) {
	for item := range input {
		b.Lock()
		for sub := range b.subs {
			item = cloneMap(item)
			select {
			case sub <- item:
			default:
				// Slow consumer, drop
				go b.unsub(sub)
			}
		}
		b.Unlock()
	}

	b.Lock()
	for sub := range b.subs {
		b.unsub(sub)
	}
	b.running = false
	b.Unlock()
}

func cloneMap(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	result := map[string]interface{}{}
	for k, v := range data {
		switch t := v.(type) {
		case []interface{}:
			result[k] = cloneSlice(t)
		case []map[string]interface{}:
			result[k] = cloneMapSlice(t)
		case map[string]interface{}:
			result[k] = cloneMap(t)
		default:
			result[k] = v
		}
	}

	return result
}

func cloneMapSlice(data []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, len(data))
	copy(result, data)
	return result
}

func cloneSlice(data []interface{}) []interface{} {
	result := make([]interface{}, len(data))
	copy(result, data)
	return result
}
