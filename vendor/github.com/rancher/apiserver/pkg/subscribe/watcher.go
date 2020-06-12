package subscribe

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rancher/apiserver/pkg/types"
)

type WatchSession struct {
	sync.Mutex

	apiOp    *types.APIRequest
	watchers map[string]func()
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   func()
}

func (s *WatchSession) stop(id string, resp chan<- types.APIEvent) {
	s.Lock()
	defer s.Unlock()
	if cancel, ok := s.watchers[id]; ok {
		cancel()
		resp <- types.APIEvent{
			Name:         "resource.stop",
			ResourceType: id,
		}
	}
	delete(s.watchers, id)
}

func (s *WatchSession) add(resourceType, revision string, resp chan<- types.APIEvent) {
	s.Lock()
	defer s.Unlock()

	ctx, cancel := context.WithCancel(s.ctx)
	s.watchers[resourceType] = cancel

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.stop(resourceType, resp)

		if err := s.stream(ctx, resourceType, revision, resp); err != nil {
			sendErr(resp, err, resourceType)
		}
	}()
}

func (s *WatchSession) stream(ctx context.Context, resourceType, revision string, result chan<- types.APIEvent) error {
	schema := s.apiOp.Schemas.LookupSchema(resourceType)
	if schema == nil {
		return fmt.Errorf("failed to find schema %s", resourceType)
	} else if schema.Store == nil {
		return fmt.Errorf("schema %s does not support watching", resourceType)
	}

	if err := s.apiOp.AccessControl.CanWatch(s.apiOp, schema); err != nil {
		return err
	}

	c, err := schema.Store.Watch(s.apiOp.WithContext(ctx), schema, types.WatchRequest{Revision: revision})
	if err != nil {
		return err
	}

	result <- types.APIEvent{
		Name:         "resource.start",
		ResourceType: resourceType,
	}

	if c == nil {
		<-s.apiOp.Context().Done()
	} else {
		for event := range c {
			result <- event
		}
	}

	return nil
}

func NewWatchSession(apiOp *types.APIRequest) *WatchSession {
	ws := &WatchSession{
		apiOp:    apiOp,
		watchers: map[string]func(){},
	}

	ws.ctx, ws.cancel = context.WithCancel(apiOp.Request.Context())
	return ws
}

func (s *WatchSession) Watch(conn *websocket.Conn) <-chan types.APIEvent {
	result := make(chan types.APIEvent, 100)
	go func() {
		defer close(result)

		if err := s.watch(conn, result); err != nil {
			sendErr(result, err, "")
		}
	}()
	return result
}

func (s *WatchSession) Close() {
	s.cancel()
	s.wg.Wait()
}

func (s *WatchSession) watch(conn *websocket.Conn, resp chan types.APIEvent) error {
	defer s.wg.Wait()
	defer s.cancel()

	for {
		_, r, err := conn.NextReader()
		if err != nil {
			return err
		}

		var sub Subscribe

		if err := json.NewDecoder(r).Decode(&sub); err != nil {
			sendErr(resp, err, "")
			continue
		}

		if sub.Stop {
			s.stop(sub.ResourceType, resp)
		} else {
			s.Lock()
			_, ok := s.watchers[sub.ResourceType]
			s.Unlock()
			if !ok {
				s.add(sub.ResourceType, sub.ResourceVersion, resp)
			}
		}
	}
}

func sendErr(resp chan<- types.APIEvent, err error, resourceType string) {
	resp <- types.APIEvent{
		ResourceType: resourceType,
		Error:        err,
	}
}
