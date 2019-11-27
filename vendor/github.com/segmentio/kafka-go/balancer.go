package kafka

import (
	"hash"
	"hash/crc32"
	"hash/fnv"
	"math/rand"
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

	// lock protects Hasher while calculating the hash code.  It is assumed that
	// the Hasher field is read-only once the Balancer is created, so as a
	// performance optimization, reads of the field are not protected.
	lock sync.Mutex
}

func (h *Hash) Balance(msg Message, partitions ...int) int {
	if msg.Key == nil {
		return h.rr.Balance(msg, partitions...)
	}

	hasher := h.Hasher
	if hasher != nil {
		h.lock.Lock()
		defer h.lock.Unlock()
	} else {
		hasher = fnv1aPool.Get().(hash.Hash32)
		defer fnv1aPool.Put(hasher)
	}

	hasher.Reset()
	if _, err := hasher.Write(msg.Key); err != nil {
		panic(err)
	}

	// uses same algorithm that Sarama's hashPartitioner uses
	// note the type conversions here.  if the uint32 hash code is not cast to
	// an int32, we do not get the same result as sarama.
	partition := int32(hasher.Sum32()) % int32(len(partitions))
	if partition < 0 {
		partition = -partition
	}

	return int(partition)
}

type randomBalancer struct {
	mock int // mocked return value, used for testing
}

func (b randomBalancer) Balance(msg Message, partitions ...int) (partition int) {
	if b.mock != 0 {
		return b.mock
	}
	return partitions[rand.Int()%len(partitions)]
}

// CRC32Balancer is a Balancer that uses the CRC32 hash function to determine
// which partition to route messages to.  This ensures that messages with the
// same key are routed to the same partition.  This balancer is compatible with
// the built-in hash partitioners in librdkafka and the language bindings that
// are built on top of it, including the
// github.com/confluentinc/confluent-kafka-go Go package.
//
// With the Consistent field false (default), this partitioner is equivalent to
// the "consistent_random" setting in librdkafka.  When Consistent is true, this
// partitioner is equivalent to the "consistent" setting.  The latter will hash
// empty or nil keys into the same partition.
//
// Unless you are absolutely certain that all your messages will have keys, it's
// best to leave the Consistent flag off.  Otherwise, you run the risk of
// creating a very hot partition.
type CRC32Balancer struct {
	Consistent bool
	random     randomBalancer
}

func (b CRC32Balancer) Balance(msg Message, partitions ...int) (partition int) {
	// NOTE: the crc32 balancers in librdkafka don't differentiate between nil
	//       and empty keys.  both cases are treated as unset.
	if len(msg.Key) == 0 && !b.Consistent {
		return b.random.Balance(msg, partitions...)
	}

	idx := crc32.ChecksumIEEE(msg.Key) % uint32(len(partitions))
	return partitions[idx]
}

// Murmur2Balancer is a Balancer that uses the Murmur2 hash function to
// determine which partition to route messages to.  This ensures that messages
// with the same key are routed to the same partition.  This balancer is
// compatible with the partitioner used by the Java library and by librdkafka's
// "murmur2" and "murmur2_random" partitioners. /
//
// With the Consistent field false (default), this partitioner is equivalent to
// the "murmur2_random" setting in librdkafka.  When Consistent is true, this
// partitioner is equivalent to the "murmur2" setting.  The latter will hash
// nil keys into the same partition.  Empty, non-nil keys are always hashed to
// the same partition regardless of configuration.
//
// Unless you are absolutely certain that all your messages will have keys, it's
// best to leave the Consistent flag off.  Otherwise, you run the risk of
// creating a very hot partition.
//
// Note that the librdkafka documentation states that the "murmur2_random" is
// functionally equivalent to the default Java partitioner.  That's because the
// Java partitioner will use a round robin balancer instead of random on nil
// keys.  We choose librdkafka's implementation because it arguably has a larger
// install base.
type Murmur2Balancer struct {
	Consistent bool
	random     randomBalancer
}

func (b Murmur2Balancer) Balance(msg Message, partitions ...int) (partition int) {
	// NOTE: the murmur2 balancers in java and librdkafka treat a nil key as
	//       non-existent while treating an empty slice as a defined value.
	if msg.Key == nil && !b.Consistent {
		return b.random.Balance(msg, partitions...)
	}

	idx := (murmur2(msg.Key) & 0x7fffffff) % uint32(len(partitions))
	return partitions[idx]
}

// Go port of the Java library's murmur2 function.
// https://github.com/apache/kafka/blob/1.0/clients/src/main/java/org/apache/kafka/common/utils/Utils.java#L353
func murmur2(data []byte) uint32 {
	length := len(data)
	const (
		seed uint32 = 0x9747b28c
		// 'm' and 'r' are mixing constants generated offline.
		// They're not really 'magic', they just happen to work well.
		m = 0x5bd1e995
		r = 24
	)

	// Initialize the hash to a random value
	h := seed ^ uint32(length)
	length4 := length / 4

	for i := 0; i < length4; i++ {
		i4 := i * 4
		k := (uint32(data[i4+0]) & 0xff) + ((uint32(data[i4+1]) & 0xff) << 8) + ((uint32(data[i4+2]) & 0xff) << 16) + ((uint32(data[i4+3]) & 0xff) << 24)
		k *= m
		k ^= k >> r
		k *= m
		h *= m
		h ^= k
	}

	// Handle the last few bytes of the input array
	extra := length % 4
	if extra >= 3 {
		h ^= (uint32(data[(length & ^3)+2]) & 0xff) << 16
	}
	if extra >= 2 {
		h ^= (uint32(data[(length & ^3)+1]) & 0xff) << 8
	}
	if extra >= 1 {
		h ^= uint32(data[length & ^3]) & 0xff
		h *= m
	}

	h ^= h >> 13
	h *= m
	h ^= h >> 15

	return h
}
