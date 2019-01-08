package kafka

import (
	"hash"
	"hash/fnv"
	"sort"
	"sync"
)

// The Balancer interface provides an abstraction of the message distribution
// logic used by Writer instances to route messages to the partitions available
// on a kafka cluster.
//
// Instances of Balancer do not have to be safe to use concurrently by multiple
// goroutines, the Writer implementation ensures that calls to Balance are
// synchronized.
type Balancer interface {
	// Balance receives a message and a set of available partitions and
	// returns the partition number that the message should be routed to.
	//
	// An application should refrain from using a balancer to manage multiple
	// sets of partitions (from different topics for examples), use one balancer
	// instance for each partition set, so the balancer can detect when the
	// partitions change and assume that the kafka topic has been rebalanced.
	Balance(msg Message, partitions ...int) (partition int)
}

// BalancerFunc is an implementation of the Balancer interface that makes it
// possible to use regular functions to distribute messages across partitions.
type BalancerFunc func(Message, ...int) int

// Balance calls f, satisfies the Balancer interface.
func (f BalancerFunc) Balance(msg Message, partitions ...int) int {
	return f(msg, partitions...)
}

// RoundRobin is an Balancer implementation that equally distributes messages
// across all available partitions.
type RoundRobin struct {
	offset uint64
}

// Balance satisfies the Balancer interface.
func (rr *RoundRobin) Balance(msg Message, partitions ...int) int {
	length := uint64(len(partitions))
	offset := rr.offset
	rr.offset++
	return partitions[offset%length]
}

// LeastBytes is a Balancer implementation that routes messages to the partition
// that has received the least amount of data.
//
// Note that no coordination is done between multiple producers, having good
// balancing relies on the fact that each producer using a LeastBytes balancer
// should produce well balanced messages.
type LeastBytes struct {
	counters []leastBytesCounter
}

type leastBytesCounter struct {
	partition int
	bytes     uint64
}

// Balance satisfies the Balancer interface.
func (lb *LeastBytes) Balance(msg Message, partitions ...int) int {
	for _, p := range partitions {
		if c := lb.counterOf(p); c == nil {
			lb.counters = lb.makeCounters(partitions...)
			break
		}
	}

	minBytes := lb.counters[0].bytes
	minIndex := 0

	for i, c := range lb.counters[1:] {
		if c.bytes < minBytes {
			minIndex = i + 1
			minBytes = c.bytes
		}
	}

	c := &lb.counters[minIndex]
	c.bytes += uint64(len(msg.Key)) + uint64(len(msg.Value))
	return c.partition
}

func (lb *LeastBytes) counterOf(partition int) *leastBytesCounter {
	i := sort.Search(len(lb.counters), func(i int) bool {
		return lb.counters[i].partition >= partition
	})
	if i == len(lb.counters) || lb.counters[i].partition != partition {
		return nil
	}
	return &lb.counters[i]
}

func (lb *LeastBytes) makeCounters(partitions ...int) (counters []leastBytesCounter) {
	counters = make([]leastBytesCounter, len(partitions))

	for i, p := range partitions {
		counters[i].partition = p
	}

	sort.Slice(counters, func(i int, j int) bool {
		return counters[i].partition < counters[j].partition
	})
	return
}

var (
	fnv1aPool = &sync.Pool{
		New: func() interface{} {
			return fnv.New32a()
		},
	}
)

// Hash is a Balancer that uses the provided hash function to determine which
// partition to route messages to.  This ensures that messages with the same key
// are routed to the same partition.
//
// The logic to calculate the partition is:
//
// 		hasher.Sum32() % len(partitions) => partition
//
// By default, Hash uses the FNV-1a algorithm.  This is the same algorithm used
// by the Sarama Producer and ensures that messages produced by kafka-go will
// be delivered to the same topics that the Sarama producer would be delivered to
type Hash struct {
	rr     RoundRobin
	Hasher hash.Hash32
}

func (h *Hash) Balance(msg Message, partitions ...int) (partition int) {
	if msg.Key == nil {
		return h.rr.Balance(msg, partitions...)
	}

	hasher := h.Hasher
	if hasher == nil {
		hasher = fnv1aPool.Get().(hash.Hash32)
		defer fnv1aPool.Put(hasher)
	}

	hasher.Reset()
	if _, err := hasher.Write(msg.Key); err != nil {
		panic(err)
	}

	// uses same algorithm that Sarama's hashPartitioner uses
	partition = int(hasher.Sum32()) % len(partitions)
	if partition < 0 {
		partition = -partition
	}

	return
}
