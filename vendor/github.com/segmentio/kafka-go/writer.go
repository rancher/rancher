package kafka

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// The Writer type provides the implementation of a producer of kafka messages
// that automatically distributes messages across partitions of a single topic
// using a configurable balancing policy.
//
// Instances of Writer are safe to use concurrently from multiple goroutines.
type Writer struct {
	config WriterConfig

	mutex  sync.RWMutex
	closed bool

	join sync.WaitGroup
	msgs chan writerMessage
	done chan struct{}

	// writer stats are all made of atomic values, no need for synchronization.
	// Use a pointer to ensure 64-bit alignment of the values.
	stats *writerStats
}

// WriterConfig is a configuration type used to create new instances of Writer.
type WriterConfig struct {
	// The list of brokers used to discover the partitions available on the
	// kafka cluster.
	//
	// This field is required, attempting to create a writer with an empty list
	// of brokers will panic.
	Brokers []string

	// The topic that the writer will produce messages to.
	//
	// This field is required, attempting to create a writer with an empty topic
	// will panic.
	Topic string

	// The dialer used by the writer to establish connections to the kafka
	// cluster.
	//
	// If nil, the default dialer is used instead.
	Dialer *Dialer

	// The balancer used to distribute messages across partitions.
	//
	// The default is to use a round-robin distribution.
	Balancer Balancer

	// Limit on how many attempts will be made to deliver a message.
	//
	// The default is to try at most 10 times.
	MaxAttempts int

	// A hint on the capacity of the writer's internal message queue.
	//
	// The default is to use a queue capacity of 100 messages.
	QueueCapacity int

	// Limit on how many messages will be buffered before being sent to a
	// partition.
	//
	// The default is to use a target batch size of 100 messages.
	BatchSize int

	// Limit the maximum size of a request in bytes before being sent to
	// a partition.
	//
	// The default is to use a kafka default value of 1048576.
	BatchBytes int

	// Time limit on how often incomplete message batches will be flushed to
	// kafka.
	//
	// The default is to flush at least every second.
	BatchTimeout time.Duration

	// Timeout for read operations performed by the Writer.
	//
	// Defaults to 10 seconds.
	ReadTimeout time.Duration

	// Timeout for write operation performed by the Writer.
	//
	// Defaults to 10 seconds.
	WriteTimeout time.Duration

	// This interval defines how often the list of partitions is refreshed from
	// kafka. It allows the writer to automatically handle when new partitions
	// are added to a topic.
	//
	// The default is to refresh partitions every 15 seconds.
	RebalanceInterval time.Duration

	// Connections that were idle for this duration will not be reused.
	//
	// Defaults to 9 minutes.
	IdleConnTimeout time.Duration

	// Number of acknowledges from partition replicas required before receiving
	// a response to a produce request (default to -1, which means to wait for
	// all replicas).
	RequiredAcks int

	// Setting this flag to true causes the WriteMessages method to never block.
	// It also means that errors are ignored since the caller will not receive
	// the returned value. Use this only if you don't care about guarantees of
	// whether the messages were written to kafka.
	Async bool

	// CompressionCodec set the codec to be used to compress Kafka messages.
	// Note that messages are allowed to overwrite the compression codec individually.
	CompressionCodec

	// If not nil, specifies a logger used to report internal changes within the
	// writer.
	Logger Logger

	// ErrorLogger is the logger used to report errors. If nil, the writer falls
	// back to using Logger instead.
	ErrorLogger Logger

	newPartitionWriter func(partition int, config WriterConfig, stats *writerStats) partitionWriter
}

// WriterStats is a data structure returned by a call to Writer.Stats that
// exposes details about the behavior of the writer.
type WriterStats struct {
	Dials      int64 `metric:"kafka.writer.dial.count"      type:"counter"`
	Writes     int64 `metric:"kafka.writer.write.count"     type:"counter"`
	Messages   int64 `metric:"kafka.writer.message.count"   type:"counter"`
	Bytes      int64 `metric:"kafka.writer.message.bytes"   type:"counter"`
	Rebalances int64 `metric:"kafka.writer.rebalance.count" type:"counter"`
	Errors     int64 `metric:"kafka.writer.error.count"     type:"counter"`

	DialTime   DurationStats `metric:"kafka.writer.dial.seconds"`
	WriteTime  DurationStats `metric:"kafka.writer.write.seconds"`
	WaitTime   DurationStats `metric:"kafka.writer.wait.seconds"`
	Retries    SummaryStats  `metric:"kafka.writer.retries.count"`
	BatchSize  SummaryStats  `metric:"kafka.writer.batch.size"`
	BatchBytes SummaryStats  `metric:"kafka.writer.batch.bytes"`

	MaxAttempts       int64         `metric:"kafka.writer.attempts.max"       type:"gauge"`
	MaxBatchSize      int64         `metric:"kafka.writer.batch.max"          type:"gauge"`
	BatchTimeout      time.Duration `metric:"kafka.writer.batch.timeout"      type:"gauge"`
	ReadTimeout       time.Duration `metric:"kafka.writer.read.timeout"       type:"gauge"`
	WriteTimeout      time.Duration `metric:"kafka.writer.write.timeout"      type:"gauge"`
	RebalanceInterval time.Duration `metric:"kafka.writer.rebalance.interval" type:"gauge"`
	RequiredAcks      int64         `metric:"kafka.writer.acks.required"      type:"gauge"`
	Async             bool          `metric:"kafka.writer.async"              type:"gauge"`
	QueueLength       int64         `metric:"kafka.writer.queue.length"       type:"gauge"`
	QueueCapacity     int64         `metric:"kafka.writer.queue.capacity"     type:"gauge"`

	ClientID string `tag:"client_id"`
	Topic    string `tag:"topic"`
}

// writerStats is a struct that contains statistics on a writer.
//
// Since atomic is used to mutate the statistics the values must be 64-bit aligned.
// This is easily accomplished by always allocating this struct directly, (i.e. using a pointer to the struct).
// See https://golang.org/pkg/sync/atomic/#pkg-note-BUG
type writerStats struct {
	dials          counter
	writes         counter
	messages       counter
	bytes          counter
	rebalances     counter
	errors         counter
	dialTime       summary
	writeTime      summary
	waitTime       summary
	retries        summary
	batchSize      summary
	batchSizeBytes summary
}

// Validate method validates WriterConfig properties.
func (config *WriterConfig) Validate() error {

	if len(config.Brokers) == 0 {
		return errors.New("cannot create a kafka writer with an empty list of brokers")
	}

	if len(config.Topic) == 0 {
		return errors.New("cannot create a kafka writer with an empty topic")
	}

	return nil
}

// NewWriter creates and returns a new Writer configured with config.
func NewWriter(config WriterConfig) *Writer {

	if err := config.Validate(); err != nil {
		panic(err)
	}

	if config.Dialer == nil {
		config.Dialer = DefaultDialer
	}

	if config.Balancer == nil {
		config.Balancer = &RoundRobin{}
	}

	if config.newPartitionWriter == nil {
		config.newPartitionWriter = func(partition int, config WriterConfig, stats *writerStats) partitionWriter {
			return newWriter(partition, config, stats)
		}
	}

	if config.MaxAttempts == 0 {
		config.MaxAttempts = 10
	}

	if config.QueueCapacity == 0 {
		config.QueueCapacity = 100
	}

	if config.BatchSize == 0 {
		config.BatchSize = 100
	}

	if config.BatchBytes == 0 {
		// 1048576 == 1MB which is the Kafka default.
		config.BatchBytes = 1048576
	}

	if config.BatchTimeout == 0 {
		config.BatchTimeout = 1 * time.Second
	}

	if config.ReadTimeout == 0 {
		config.ReadTimeout = 10 * time.Second
	}

	if config.WriteTimeout == 0 {
		config.WriteTimeout = 10 * time.Second
	}

	if config.RebalanceInterval == 0 {
		config.RebalanceInterval = 15 * time.Second
	}
	if config.IdleConnTimeout == 0 {
		config.IdleConnTimeout = 9 * time.Minute
	}

	w := &Writer{
		config: config,
		msgs:   make(chan writerMessage, config.QueueCapacity),
		done:   make(chan struct{}),
		stats: &writerStats{
			dialTime:  makeSummary(),
			writeTime: makeSummary(),
			waitTime:  makeSummary(),
			retries:   makeSummary(),
		},
	}

	w.join.Add(1)
	go w.run()
	return w
}

// WriteMessages writes a batch of messages to the kafka topic configured on this
// writer.
//
// Unless the writer was configured to write messages asynchronously, the method
// blocks until all messages have been written, or until the maximum number of
// attempts was reached.
//
// When sending synchronously and the writer's batch size is configured to be
// greater than 1, this method blocks until either a full batch can be assembled
// or the batch timeout is reached.  The batch size and timeouts are evaluated
// per partition, so the choice of Balancer can also influence the flushing
// behavior.  For example, the Hash balancer will require on average N * batch
// size messages to trigger a flush where N is the number of partitions.  The
// best way to achieve good batching behavior is to share one Writer amongst
// multiple go routines.
//
// When the method returns an error, there's no way to know yet which messages
// have succeeded of failed.
//
// The context passed as first argument may also be used to asynchronously
// cancel the operation. Note that in this case there are no guarantees made on
// whether messages were written to kafka. The program should assume that the
// whole batch failed and re-write the messages later (which could then cause
// duplicates).
func (w *Writer) WriteMessages(ctx context.Context, msgs ...Message) error {
	if len(msgs) == 0 {
		return nil
	}

	var err error
	var res chan error
	if !w.config.Async {
		res = make(chan error, len(msgs))
	}
	t0 := time.Now()

	for attempt := 0; attempt < w.config.MaxAttempts; attempt++ {
		w.mutex.RLock()

		if w.closed {
			w.mutex.RUnlock()
			return io.ErrClosedPipe
		}

		for i, msg := range msgs {
			if int(msg.size()) > w.config.BatchBytes {
				err := MessageTooLargeError{
					Message:   msg,
					Remaining: msgs[i+1:],
				}
				w.mutex.RUnlock()
				return err
			}
			select {
			case w.msgs <- writerMessage{
				msg: msg,
				res: res,
			}:
			case <-ctx.Done():
				w.mutex.RUnlock()
				return ctx.Err()
			}
		}

		w.mutex.RUnlock()

		if w.config.Async {
			break
		}

		var retry []Message

		for i := 0; i != len(msgs); i++ {
			select {
			case e := <-res:
				if e != nil {
					if we, ok := e.(*writerError); ok {
						w.stats.retries.observe(1)
						retry, err = append(retry, we.msg), we.err
					} else {
						err = e
					}
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if msgs = retry; len(msgs) == 0 {
			break
		}

		timer := time.NewTimer(backoff(attempt+1, 100*time.Millisecond, 1*time.Second))
		select {
		case <-timer.C:
			// Only clear the error (so we retry the loop) if we have more retries, otherwise
			// we risk silencing the error.
			if attempt < w.config.MaxAttempts-1 {
				err = nil
			}
		case <-ctx.Done():
			err = ctx.Err()
		case <-w.done:
			err = io.ErrClosedPipe
		}
		timer.Stop()

		if err != nil {
			break
		}
	}
	w.stats.writeTime.observeDuration(time.Since(t0))
	return err
}

// Stats returns a snapshot of the writer stats since the last time the method
// was called, or since the writer was created if it is called for the first
// time.
//
// A typical use of this method is to spawn a goroutine that will periodically
// call Stats on a kafka writer and report the metrics to a stats collection
// system.
func (w *Writer) Stats() WriterStats {
	return WriterStats{
		Dials:             w.stats.dials.snapshot(),
		Writes:            w.stats.writes.snapshot(),
		Messages:          w.stats.messages.snapshot(),
		Bytes:             w.stats.bytes.snapshot(),
		Rebalances:        w.stats.rebalances.snapshot(),
		Errors:            w.stats.errors.snapshot(),
		DialTime:          w.stats.dialTime.snapshotDuration(),
		WriteTime:         w.stats.writeTime.snapshotDuration(),
		WaitTime:          w.stats.waitTime.snapshotDuration(),
		Retries:           w.stats.retries.snapshot(),
		BatchSize:         w.stats.batchSize.snapshot(),
		BatchBytes:        w.stats.batchSizeBytes.snapshot(),
		MaxAttempts:       int64(w.config.MaxAttempts),
		MaxBatchSize:      int64(w.config.BatchSize),
		BatchTimeout:      w.config.BatchTimeout,
		ReadTimeout:       w.config.ReadTimeout,
		WriteTimeout:      w.config.WriteTimeout,
		RebalanceInterval: w.config.RebalanceInterval,
		RequiredAcks:      int64(w.config.RequiredAcks),
		Async:             w.config.Async,
		QueueLength:       int64(len(w.msgs)),
		QueueCapacity:     int64(cap(w.msgs)),
		ClientID:          w.config.Dialer.ClientID,
		Topic:             w.config.Topic,
	}
}

// Close flushes all buffered messages and closes the writer. The call to Close
// aborts any concurrent calls to WriteMessages, which then return with the
// io.ErrClosedPipe error.
func (w *Writer) Close() (err error) {
	w.mutex.Lock()

	if !w.closed {
		w.closed = true
		close(w.msgs)
		close(w.done)
	}

	w.mutex.Unlock()
	w.join.Wait()
	return
}

func (w *Writer) run() {
	defer w.join.Done()

	ticker := time.NewTicker(w.config.RebalanceInterval)
	defer ticker.Stop()

	var rebalance = true
	var writers = make(map[int]partitionWriter)
	var partitions []int
	var err error

	for {
		if rebalance {
			w.stats.rebalances.observe(1)
			rebalance = false

			var newPartitions []int
			var oldPartitions = partitions

			if newPartitions, err = w.partitions(); err == nil {
				for _, partition := range diffp(oldPartitions, newPartitions) {
					w.close(writers[partition])
					delete(writers, partition)
				}

				for _, partition := range diffp(newPartitions, oldPartitions) {
					writers[partition] = w.open(partition)
				}

				partitions = newPartitions
			}
		}

		select {
		case wm, ok := <-w.msgs:
			if !ok {
				for _, writer := range writers {
					w.close(writer)
				}
				return
			}

			if len(partitions) != 0 {
				selectedPartition := w.config.Balancer.Balance(wm.msg, partitions...)
				writers[selectedPartition].messages() <- wm
			} else {
				// No partitions were found because the topic doesn't exist.
				if err == nil {
					err = fmt.Errorf("failed to find any partitions for topic %s", w.config.Topic)
				}
				if wm.res != nil {
					wm.res <- &writerError{msg: wm.msg, err: err}
				}
			}

		case <-ticker.C:
			rebalance = true
		}
	}
}

func (w *Writer) partitions() (partitions []int, err error) {
	for _, broker := range shuffledStrings(w.config.Brokers) {
		var conn *Conn
		var plist []Partition

		if conn, err = w.config.Dialer.Dial("tcp", broker); err != nil {
			continue
		}

		conn.SetReadDeadline(time.Now().Add(w.config.ReadTimeout))
		plist, err = conn.ReadPartitions(w.config.Topic)
		conn.Close()

		if err == nil {
			partitions = make([]int, len(plist))
			for i, p := range plist {
				partitions[i] = p.ID
			}
			break
		}
	}

	sort.Ints(partitions)
	return
}

func (w *Writer) open(partition int) partitionWriter {
	return w.config.newPartitionWriter(partition, w.config, w.stats)
}

func (w *Writer) close(writer partitionWriter) {
	w.join.Add(1)
	go func() {
		writer.close()
		w.join.Done()
	}()
}

func diffp(new []int, old []int) (diff []int) {
	for _, p := range new {
		if i := sort.SearchInts(old, p); i == len(old) || old[i] != p {
			diff = append(diff, p)
		}
	}
	return
}

type partitionWriter interface {
	messages() chan<- writerMessage
	close()
}

type writer struct {
	brokers         []string
	topic           string
	partition       int
	requiredAcks    int
	batchSize       int
	maxMessageBytes int
	batchTimeout    time.Duration
	writeTimeout    time.Duration
	idleConnTimeout time.Duration
	dialer          *Dialer
	msgs            chan writerMessage
	join            sync.WaitGroup
	stats           *writerStats
	codec           CompressionCodec
	logger          Logger
	errorLogger     Logger
}

func newWriter(partition int, config WriterConfig, stats *writerStats) *writer {
	w := &writer{
		brokers:         config.Brokers,
		topic:           config.Topic,
		partition:       partition,
		requiredAcks:    config.RequiredAcks,
		batchSize:       config.BatchSize,
		maxMessageBytes: config.BatchBytes,
		batchTimeout:    config.BatchTimeout,
		writeTimeout:    config.WriteTimeout,
		idleConnTimeout: config.IdleConnTimeout,
		dialer:          config.Dialer,
		msgs:            make(chan writerMessage, config.QueueCapacity),
		stats:           stats,
		codec:           config.CompressionCodec,
		logger:          config.Logger,
		errorLogger:     config.ErrorLogger,
	}
	w.join.Add(1)
	go w.run()
	return w
}

func (w *writer) close() {
	close(w.msgs)
	w.join.Wait()
}

func (w *writer) messages() chan<- writerMessage {
	return w.msgs
}

func (w *writer) withLogger(do func(Logger)) {
	if w.logger != nil {
		do(w.logger)
	}
}

func (w *writer) withErrorLogger(do func(Logger)) {
	if w.errorLogger != nil {
		do(w.errorLogger)
	} else {
		w.withLogger(do)
	}
}

func (w *writer) run() {
	defer w.join.Done()

	batchTimer := time.NewTimer(0)
	<-batchTimer.C
	batchTimerRunning := false
	defer batchTimer.Stop()

	var conn *Conn
	var done bool
	var batch = make([]Message, 0, w.batchSize)
	var resch = make([](chan<- error), 0, w.batchSize)
	var lastMsg writerMessage
	var batchSizeBytes int
	var idleConnDeadline time.Time

	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	for !done {
		var mustFlush bool
		// lstMsg gets set when the next message would put the maxMessageBytes  over the limit.
		// If a lstMsg exists we need to add it to the batch so we don't lose it.
		if len(lastMsg.msg.Value) != 0 {
			batch = append(batch, lastMsg.msg)
			if lastMsg.res != nil {
				resch = append(resch, lastMsg.res)
			}
			batchSizeBytes += int(lastMsg.msg.size())
			lastMsg = writerMessage{}
			if !batchTimerRunning {
				batchTimer.Reset(w.batchTimeout)
				batchTimerRunning = true
			}
		}
		select {
		case wm, ok := <-w.msgs:
			if !ok {
				done, mustFlush = true, true
			} else {
				if int(wm.msg.size())+batchSizeBytes > w.maxMessageBytes {
					// If the size of the current message puts us over the maxMessageBytes limit,
					// store the message but don't send it in this batch.
					mustFlush = true
					lastMsg = wm
					break
				}
				batch = append(batch, wm.msg)
				if wm.res != nil {
					resch = append(resch, wm.res)
				}
				batchSizeBytes += int(wm.msg.size())
				mustFlush = len(batch) >= w.batchSize || batchSizeBytes >= w.maxMessageBytes
			}
			if !batchTimerRunning {
				batchTimer.Reset(w.batchTimeout)
				batchTimerRunning = true
			}

		case <-batchTimer.C:
			mustFlush = true
			batchTimerRunning = false
		}

		if mustFlush {
			w.stats.batchSizeBytes.observe(int64(batchSizeBytes))
			if batchTimerRunning {
				if stopped := batchTimer.Stop(); !stopped {
					<-batchTimer.C
				}
				batchTimerRunning = false
			}
			if conn != nil && time.Now().After(idleConnDeadline) {
				conn.Close()
				conn = nil
			}
			if len(batch) == 0 {
				continue
			}
			var err error
			if conn, err = w.write(conn, batch, resch); err != nil {
				if conn != nil {
					conn.Close()
					conn = nil
				}
			}
			idleConnDeadline = time.Now().Add(w.idleConnTimeout)
			for i := range batch {
				batch[i] = Message{}
			}

			for i := range resch {
				resch[i] = nil
			}
			batch = batch[:0]
			resch = resch[:0]
			batchSizeBytes = 0
		}
	}
}

func (w *writer) dial() (conn *Conn, err error) {
	for _, broker := range shuffledStrings(w.brokers) {
		t0 := time.Now()
		if conn, err = w.dialer.DialLeader(context.Background(), "tcp", broker, w.topic, w.partition); err == nil {
			t1 := time.Now()
			w.stats.dials.observe(1)
			w.stats.dialTime.observeDuration(t1.Sub(t0))
			conn.SetRequiredAcks(w.requiredAcks)
			break
		}
	}
	return
}

func (w *writer) write(conn *Conn, batch []Message, resch [](chan<- error)) (ret *Conn, err error) {
	w.stats.writes.observe(1)
	if conn == nil {
		if conn, err = w.dial(); err != nil {
			w.stats.errors.observe(1)
			w.withErrorLogger(func(logger Logger) {
				logger.Printf("error dialing kafka brokers for topic %s (partition %d): %s", w.topic, w.partition, err)
			})
			for i, res := range resch {
				res <- &writerError{msg: batch[i], err: err}
			}
			return
		}
	}

	t0 := time.Now()
	conn.SetWriteDeadline(time.Now().Add(w.writeTimeout))
	if _, err = conn.WriteCompressedMessages(w.codec, batch...); err != nil {
		w.stats.errors.observe(1)
		w.withErrorLogger(func(logger Logger) {
			logger.Printf("error writing messages to %s (partition %d): %s", w.topic, w.partition, err)
		})
		for i, res := range resch {
			res <- &writerError{msg: batch[i], err: err}
		}
	} else {
		for _, m := range batch {
			w.stats.messages.observe(1)
			w.stats.bytes.observe(int64(len(m.Key) + len(m.Value)))
		}
		for _, res := range resch {
			res <- nil
		}
	}
	t1 := time.Now()
	w.stats.waitTime.observeDuration(t1.Sub(t0))
	w.stats.batchSize.observe(int64(len(batch)))

	ret = conn
	return
}

type writerMessage struct {
	msg Message
	res chan<- error
}

type writerError struct {
	msg Message
	err error
}

func (e *writerError) Cause() error {
	return e.err
}

func (e *writerError) Error() string {
	return e.err.Error()
}

func (e *writerError) Temporary() bool {
	return isTemporary(e.err)
}

func (e *writerError) Timeout() bool {
	return isTimeout(e.err)
}

func shuffledStrings(list []string) []string {
	shuffledList := make([]string, len(list))
	copy(shuffledList, list)

	shufflerMutex.Lock()

	for i := range shuffledList {
		j := shuffler.Intn(i + 1)
		shuffledList[i], shuffledList[j] = shuffledList[j], shuffledList[i]
	}

	shufflerMutex.Unlock()
	return shuffledList
}

var (
	shufflerMutex = sync.Mutex{}
	shuffler      = rand.New(rand.NewSource(time.Now().Unix()))
)
