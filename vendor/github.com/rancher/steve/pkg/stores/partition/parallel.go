package partition

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/rancher/apiserver/pkg/types"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

type Partition interface {
	Name() string
}

type ParallelPartitionLister struct {
	Lister      PartitionLister
	Concurrency int64
	Partitions  []Partition
	state       *listState
	revision    string
	err         error
}

type PartitionLister func(ctx context.Context, partition Partition, cont string, revision string, limit int) (types.APIObjectList, error)

func (p *ParallelPartitionLister) Err() error {
	return p.err
}

func (p *ParallelPartitionLister) Revision() string {
	return p.revision
}

func (p *ParallelPartitionLister) Continue() string {
	if p.state == nil {
		return ""
	}
	bytes, err := json.Marshal(p.state)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func indexOrZero(partitions []Partition, name string) int {
	if name == "" {
		return 0
	}
	for i, partition := range partitions {
		if partition.Name() == name {
			return i
		}
	}
	return 0
}

func (p *ParallelPartitionLister) List(ctx context.Context, limit int, resume string) (<-chan []types.APIObject, error) {
	var state listState
	if resume != "" {
		bytes, err := base64.StdEncoding.DecodeString(resume)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(bytes, &state); err != nil {
			return nil, err
		}

		if state.Limit > 0 {
			limit = state.Limit
		}
	}

	result := make(chan []types.APIObject)
	go p.feeder(ctx, state, limit, result)
	return result, nil
}

type listState struct {
	Revision      string `json:"r,omitempty"`
	PartitionName string `json:"p,omitempty"`
	Continue      string `json:"c,omitempty"`
	Offset        int    `json:"o,omitempty"`
	Limit         int    `json:"l,omitempty"`
}

func (p *ParallelPartitionLister) feeder(ctx context.Context, state listState, limit int, result chan []types.APIObject) {
	var (
		sem      = semaphore.NewWeighted(p.Concurrency)
		capacity = limit
		last     chan struct{}
	)

	eg, ctx := errgroup.WithContext(ctx)
	defer func() {
		err := eg.Wait()
		if p.err == nil {
			p.err = err
		}
		close(result)
	}()

	for i := indexOrZero(p.Partitions, state.PartitionName); i < len(p.Partitions); i++ {
		if capacity <= 0 || isDone(ctx) {
			break
		}

		var (
			partition = p.Partitions[i]
			tickets   = int64(1)
			turn      = last
			next      = make(chan struct{})
		)

		// setup a linked list of channel to control insertion order
		last = next

		if state.Revision == "" {
			// don't have a revision yet so grab all tickets to set a revision
			tickets = 3
		}
		if err := sem.Acquire(ctx, tickets); err != nil {
			p.err = err
			break
		}

		// make state local
		state := state
		eg.Go(func() error {
			defer sem.Release(tickets)
			defer close(next)

			for {
				cont := ""
				if partition.Name() == state.PartitionName {
					cont = state.Continue
				}
				list, err := p.Lister(ctx, partition, cont, state.Revision, limit)
				if err != nil {
					return err
				}

				waitForTurn(ctx, turn)
				if p.state != nil {
					return nil
				}

				if state.Revision == "" {
					state.Revision = list.Revision
				}

				if p.revision == "" {
					p.revision = list.Revision
				}

				if state.PartitionName == partition.Name() && state.Offset > 0 && state.Offset < len(list.Objects) {
					list.Objects = list.Objects[state.Offset:]
				}

				if len(list.Objects) > capacity {
					result <- list.Objects[:capacity]
					// save state to redo this list at this offset
					p.state = &listState{
						Revision:      list.Revision,
						PartitionName: partition.Name(),
						Continue:      cont,
						Offset:        capacity,
						Limit:         limit,
					}
					capacity = 0
					return nil
				} else {
					result <- list.Objects
					capacity -= len(list.Objects)
					if list.Continue == "" {
						return nil
					}
					// loop again and get more data
					state.Continue = list.Continue
					state.PartitionName = partition.Name()
					state.Offset = 0
				}
			}
		})
	}

	p.err = eg.Wait()
}

func waitForTurn(ctx context.Context, turn chan struct{}) {
	if turn == nil {
		return
	}
	select {
	case <-turn:
	case <-ctx.Done():
	}
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
