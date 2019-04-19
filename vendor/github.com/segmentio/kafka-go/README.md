# kafka-go [![CircleCI](https://circleci.com/gh/segmentio/kafka-go.svg?style=shield)](https://circleci.com/gh/segmentio/kafka-go) [![Go Report Card](https://goreportcard.com/badge/github.com/segmentio/kafka-go)](https://goreportcard.com/report/github.com/segmentio/kafka-go) [![GoDoc](https://godoc.org/github.com/segmentio/kafka-go?status.svg)](https://godoc.org/github.com/segmentio/kafka-go)

## Motivations

We rely on both Go and Kafka a lot at Segment. Unfortunately, the state of the Go
client libraries for Kafka at the time of this writing was not ideal. The available
options were:

- [sarama](https://github.com/Shopify/sarama), which is by far the most popular
but is quite difficult to work with. It is poorly documented, the API exposes
low level concepts of the Kafka protocol, and it doesn't support recent Go features
like [contexts](https://golang.org/pkg/context/). It also passes all values as
pointers which causes large numbers of dynamic memory allocations, more frequent
garbage collections, and higher memory usage.

- [confluent-kafka-go](https://github.com/confluentinc/confluent-kafka-go) is a
cgo based wrapper around [librdkafka](https://github.com/edenhill/librdkafka),
which means it introduces a dependency to a C library on all Go code that uses
the package. It has much better documentation than sarama but still lacks support
for Go contexts.

- [goka](https://github.com/lovoo/goka) is a more recent Kafka client for Go
which focuses on a specific usage pattern. It provides abstractions for using Kafka
as a message passing bus between services rather than an ordered log of events, but
this is not the typical use case of Kafka for us at Segment. The package also
depends on sarama for all interactions with Kafka.

This is where `kafka-go` comes into play. It provides both low and high level
APIs for interacting with Kafka, mirroring concepts and implementing interfaces of
the Go standard library to make it easy to use and integrate with existing
software.

## Kafka versions

`kafka-go` is currently compatible with Kafka versions from 0.10.1.0 to 2.1.0. While latest versions will be working,
some features available from the Kafka API may not be implemented yet.

## Connection [![GoDoc](https://godoc.org/github.com/segmentio/kafka-go?status.svg)](https://godoc.org/github.com/segmentio/kafka-go#Conn)

The `Conn` type is the core of the `kafka-go` package. It wraps around a raw
network connection to expose a low-level API to a Kafka server.

Here are some examples showing typical use of a connection object:
```go
// to produce messages
topic := "my-topic"
partition := 0

conn, _ := kafka.DialLeader(context.Background(), "tcp", "localhost:9092", topic, partition)

conn.SetWriteDeadline(time.Now().Add(10*time.Second))
conn.WriteMessages(
    kafka.Message{Value: []byte("one!")},
    kafka.Message{Value: []byte("two!")},
    kafka.Message{Value: []byte("three!")},
)

conn.Close()
```
```go
// to consume messages
topic := "my-topic"
partition := 0

conn, _ := kafka.DialLeader(context.Background(), "tcp", "localhost:9092", topic, partition)

conn.SetReadDeadline(time.Now().Add(10*time.Second))
batch := conn.ReadBatch(10e3, 1e6) // fetch 10KB min, 1MB max

b := make([]byte, 10e3) // 10KB max per message
for {
    _, err := batch.Read(b)
    if err != nil {
        break
    }
    fmt.Println(string(b))
}

batch.Close()
conn.Close()
```

Because it is low level, the `Conn` type turns out to be a great building block
for higher level abstractions, like the `Reader` for example.

## Reader [![GoDoc](https://godoc.org/github.com/segmentio/kafka-go?status.svg)](https://godoc.org/github.com/segmentio/kafka-go#Reader)

A `Reader` is another concept exposed by the `kafka-go` package, which intends
to make it simpler to implement the typical use case of consuming from a single
topic-partition pair.
A `Reader` also automatically handles reconnections and offset management, and
exposes an API that supports asynchronous cancellations and timeouts using Go
contexts.

```go
// make a new reader that consumes from topic-A, partition 0, at offset 42
r := kafka.NewReader(kafka.ReaderConfig{
    Brokers:   []string{"localhost:9092"},
    Topic:     "topic-A",
    Partition: 0,
    MinBytes:  10e3, // 10KB
    MaxBytes:  10e6, // 10MB
})
r.SetOffset(42)

for {
    m, err := r.ReadMessage(context.Background())
    if err != nil {
        break
    }
    fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))
}

r.Close()
```

### Consumer Groups

```kafka-go``` also supports Kafka consumer groups including broker managed offsets.
To enable consumer groups, simply specify the GroupID in the ReaderConfig.

ReadMessage automatically commits offsets when using consumer groups.

```go
// make a new reader that consumes from topic-A
r := kafka.NewReader(kafka.ReaderConfig{
    Brokers:   []string{"localhost:9092"},
    GroupID:   "consumer-group-id",
    Topic:     "topic-A",
    MinBytes:  10e3, // 10KB
    MaxBytes:  10e6, // 10MB
})

for {
    m, err := r.ReadMessage(context.Background())
    if err != nil {
        break
    }
    fmt.Printf("message at topic/partition/offset %v/%v/%v: %s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
}

r.Close()
```

There are a number of limitations when using consumer groups:

* ```(*Reader).SetOffset``` will return an error when GroupID is set
* ```(*Reader).Offset``` will always return ```-1``` when GroupID is set
* ```(*Reader).Lag``` will always return ```-1``` when GroupID is set
* ```(*Reader).ReadLag``` will return an error when GroupID is set
* ```(*Reader).Stats``` will return a partition of ```-1``` when GroupID is set

### Explicit Commits

```kafka-go``` also supports explicit commits.  Instead of calling ```ReadMessage```,
call ```FetchMessage``` followed by ```CommitMessages```.

```go
ctx := context.Background()
for {
    m, err := r.FetchMessage(ctx)
    if err != nil {
        break
    }
    fmt.Printf("message at topic/partition/offset %v/%v/%v: %s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
    r.CommitMessages(ctx, m)
}
```

### Managing Commits

By default, CommitMessages will synchronously commit offsets to Kafka.  For
improved performance, you can instead periodically commit offsets to Kafka
by setting CommitInterval on the ReaderConfig.


```go
// make a new reader that consumes from topic-A
r := kafka.NewReader(kafka.ReaderConfig{
    Brokers:        []string{"localhost:9092"},
    GroupID:        "consumer-group-id",
    Topic:          "topic-A",
    MinBytes:       10e3, // 10KB
    MaxBytes:       10e6, // 10MB
    CommitInterval: time.Second, // flushes commits to Kafka every second
})
```

## Writer [![GoDoc](https://godoc.org/github.com/segmentio/kafka-go?status.svg)](https://godoc.org/github.com/segmentio/kafka-go#Writer)

To produce messages to Kafka, a program may use the low-level `Conn` API, but
the package also provides a higher level `Writer` type which is more appropriate
to use in most cases as it provides additional features:

- Automatic retries and reconnections on errors.
- Configurable distribution of messages across available partitions.
- Synchronous or asynchronous writes of messages to Kafka.
- Asynchronous cancellation using contexts.
- Flushing of pending messages on close to support graceful shutdowns.

```go
// make a writer that produces to topic-A, using the least-bytes distribution
w := kafka.NewWriter(kafka.WriterConfig{
	Brokers: []string{"localhost:9092"},
	Topic:   "topic-A",
	Balancer: &kafka.LeastBytes{},
})

w.WriteMessages(context.Background(),
	kafka.Message{
		Key:   []byte("Key-A"),
		Value: []byte("Hello World!"),
	},
	kafka.Message{
		Key:   []byte("Key-B"),
		Value: []byte("One!"),
	},
	kafka.Message{
		Key:   []byte("Key-C"),
		Value: []byte("Two!"),
	},
)

w.Close()
```

**Note:** Even though kafka.Message contain ```Topic``` and ```Partition``` fields, they **MUST NOT** be
set when writing messages.  They are intended for read use only.

### Compatibility with Sarama

If you're switching from Sarama and need/want to use the same algorithm for message
partitioning, you can use the ```kafka.Hash``` balancer.  ```kafka.Hash``` routes
messages to the same partitions that sarama's default partitioner would route to.

```go
w := kafka.NewWriter(kafka.WriterConfig{
	Brokers: []string{"localhost:9092"},
	Topic:   "topic-A",
	Balancer: &kafka.Hash{},
})
```

### Compression

Compression can be enabled on the `Writer` by configuring the `CompressionCodec`:

```go
w := kafka.NewWriter(kafka.WriterConfig{
	Brokers: []string{"localhost:9092"},
	Topic:   "topic-A",
	CompressionCodec: snappy.NewCompressionCodec(),
})
```

The `Reader` will by determine if the consumed messages are compressed by 
examining the message attributes.  However, the package(s) for all expected 
codecs must be imported so that they get loaded correctly.  For example, if you 
are going to be receiving messages compressed with Snappy, add the following
import:

```go
import _ "github.com/segmentio/kafka-go/snappy"
```

## TLS Support

For a bare bones Conn type or in the Reader/Writer configs you can specify a dialer option for TLS support. If the TLS field is nil, it will not connect with TLS.

### Connection

```go
dialer := &kafka.Dialer{
    Timeout:   10 * time.Second,
    DualStack: true,
    TLS:       &tls.Config{...tls config...},
}

conn, err := dialer.DialContext(ctx, "tcp", "localhost:9093")
```

### Reader

```go
dialer := &kafka.Dialer{
    Timeout:   10 * time.Second,
    DualStack: true,
    TLS:       &tls.Config{...tls config...},
}

r := kafka.NewReader(kafka.ReaderConfig{
    Brokers:        []string{"localhost:9093"},
    GroupID:        "consumer-group-id",
    Topic:          "topic-A",
    Dialer:         dialer,
})
```

### Writer

```go
dialer := &kafka.Dialer{
    Timeout:   10 * time.Second,
    DualStack: true,
    TLS:       &tls.Config{...tls config...},
}

w := kafka.NewWriter(kafka.WriterConfig{
	Brokers: []string{"localhost:9093"},
	Topic:   "topic-A",
	Balancer: &kafka.Hash{},
	Dialer:   dialer,
})
```
