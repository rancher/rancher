package kafka

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"
)

// ErrGroupClosed is returned by ConsumerGroup.Next when the group has already
// been closed.
var ErrGroupClosed = errors.New("consumer group is closed")

// ErrGenerationEnded is returned by the context.Context issued by the
// Generation's Start function when the context has been closed.
var ErrGenerationEnded = errors.New("consumer group generation has ended")

const (
	// defaultProtocolType holds the default protocol type documented in the
	// kafka protocol
	//
	// See https://cwiki.apache.org/confluence/display/KAFKA/A+Guide+To+The+Kafka+Protocol#AGuideToTheKafkaProtocol-GroupMembershipAPI
	defaultProtocolType = "consumer"

	// defaultHeartbeatInterval contains the default time between heartbeats.  If
	// the coordinator does not receive a heartbeat within the session timeout interval,
	// the consumer will be considered dead and the coordinator will rebalance the
	// group.
	//
	// As a rule, the heartbeat interval should be no greater than 1/3 the session timeout
	defaultHeartbeatInterval = 3 * time.Second

	// defaultSessionTimeout contains the default interval the coordinator will wait
	// for a heartbeat before marking a consumer as dead
	defaultSessionTimeout = 30 * time.Second

	// defaultRebalanceTimeout contains the amount of time the coordinator will wait
	// for consumers to issue a join group once a rebalance has been requested
	defaultRebalanceTimeout = 30 * time.Second

	// defaultJoinGroupBackoff is the amount of time to wait after a failed
	// consumer group generation before attempting to re-join.
	defaultJoinGroupBackoff = 5 * time.Second

	// defaultRetentionTime holds the length of time a the consumer group will be
	// saved by kafka
	defaultRetentionTime = time.Hour * 24

	// defaultPartitionWatchTime contains the amount of time the kafka-go will wait to
	// query the brokers looking for partition changes.
	defaultPartitionWatchTime = 5 * time.Second
)

// ConsumerGroupConfig is a configuration object used to create new instances of
// ConsumerGroup.
type ConsumerGroupConfig struct {
	// ID is the consumer group ID.  It must not be empty.
	ID string

	// The list of broker addresses used to connect to the kafka cluster.  It
	// must not be empty.
	Brokers []string

	// An dialer used to open connections to the kafka server. This field is
	// optional, if nil, the default dialer is used instead.
	Dialer *Dialer

	// Topics is the list of topics that will be consumed by this group.  It
	// will usually have a single value, but it is permitted to have multiple
	// for more complex use cases.
	Topics []string

	// GroupBalancers is the priority-ordered list of client-side consumer group
	// balancing strategies that will be offered to the coordinator.  The first
	// strategy that all group members support will be chosen by the leader.
	//
	// Default: [Range, RoundRobin]
	GroupBalancers []GroupBalancer

	// HeartbeatInterval sets the optional frequency at which the reader sends the consumer
	// group heartbeat update.
	//
	// Default: 3s
	HeartbeatInterval time.Duration

	// PartitionWatchInterval indicates how often a reader checks for partition changes.
	// If a reader sees a partition change (such as a partition add) it will rebalance the group
	// picking up new partitions.
	//
	// Default: 5s
	PartitionWatchInterval time.Duration

	// WatchForPartitionChanges is used to inform kafka-go that a consumer group should be
	// polling the brokers and rebalancing if any partition changes happen to the topic.
	WatchPartitionChanges bool

	// SessionTimeout optionally sets the length of time that may pass without a heartbeat
	// before the coordinator considers the consumer dead and initiates a rebalance.
	//
	// Default: 30s
	SessionTimeout time.Duration

	// RebalanceTimeout optionally sets the length of time the coordinator will wait
	// for members to join as part of a rebalance.  For kafka servers under higher
	// load, it may be useful to set this value higher.
	//
	// Default: 30s
	RebalanceTimeout time.Duration

	// JoinGroupBackoff optionally sets the length of time to wait before re-joining
	// the consumer group after an error.
	//
	// Default: 5s
	JoinGroupBackoff time.Duration

	// RetentionTime optionally sets the length of time the consumer group will be saved
	// by the broker
	//
	// Default: 24h
	RetentionTime time.Duration

	// StartOffset determines from whence the consumer group should begin
	// consuming when it finds a partition without a committed offset.  If
	// non-zero, it must be set to one of FirstOffset or LastOffset.
	//
	// Default: FirstOffset
	StartOffset int64

	// If not nil, specifies a logger used to report internal changes within the
	// reader.
	Logger Logger

	// ErrorLogger is the logger used to report errors. If nil, the reader falls
	// back to using Logger instead.
	ErrorLogger Logger

	// connect is a function for dialing the coordinator.  This is provided for
	// unit testing to mock broker connections.
	connect func(dialer *Dialer, brokers ...string) (coordinator, error)
}

// Validate method validates ConsumerGroupConfig properties and sets relevant
// defaults.
func (config *ConsumerGroupConfig) Validate() error {

	if len(config.Brokers) == 0 {
		return errors.New("cannot create a consumer group with an empty list of broker addresses")
	}

	if len(config.Topics) == 0 {
		return errors.New("cannot create a consumer group without a topic")
	}

	if config.ID == "" {
		return errors.New("cannot create a consumer group without an ID")
	}

	if config.Dialer == nil {
		config.Dialer = DefaultDialer
	}

	if len(config.GroupBalancers) == 0 {
		config.GroupBalancers = []GroupBalancer{
			RangeGroupBalancer{},
			RoundRobinGroupBalancer{},
		}
	}

	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = defaultHeartbeatInterval
	}

	if config.SessionTimeout == 0 {
		config.SessionTimeout = defaultSessionTimeout
	}

	if config.PartitionWatchInterval == 0 {
		config.PartitionWatchInterval = defaultPartitionWatchTime
	}

	if config.RebalanceTimeout == 0 {
		config.RebalanceTimeout = defaultRebalanceTimeout
	}

	if config.JoinGroupBackoff == 0 {
		config.JoinGroupBackoff = defaultJoinGroupBackoff
	}

	if config.RetentionTime == 0 {
		config.RetentionTime = defaultRetentionTime
	}

	if config.HeartbeatInterval < 0 || (config.HeartbeatInterval/time.Millisecond) >= math.MaxInt32 {
		return errors.New(fmt.Sprintf("HeartbeatInterval out of bounds: %d", config.HeartbeatInterval))
	}

	if config.SessionTimeout < 0 || (config.SessionTimeout/time.Millisecond) >= math.MaxInt32 {
		return errors.New(fmt.Sprintf("SessionTimeout out of bounds: %d", config.SessionTimeout))
	}

	if config.RebalanceTimeout < 0 || (config.RebalanceTimeout/time.Millisecond) >= math.MaxInt32 {
		return errors.New(fmt.Sprintf("RebalanceTimeout out of bounds: %d", config.RebalanceTimeout))
	}

	if config.JoinGroupBackoff < 0 || (config.JoinGroupBackoff/time.Millisecond) >= math.MaxInt32 {
		return errors.New(fmt.Sprintf("JoinGroupBackoff out of bounds: %d", config.JoinGroupBackoff))
	}

	if config.RetentionTime < 0 {
		return errors.New(fmt.Sprintf("RetentionTime out of bounds: %d", config.RetentionTime))
	}

	if config.PartitionWatchInterval < 0 || (config.PartitionWatchInterval/time.Millisecond) >= math.MaxInt32 {
		return errors.New(fmt.Sprintf("PartitionWachInterval out of bounds %d", config.PartitionWatchInterval))
	}

	if config.StartOffset == 0 {
		config.StartOffset = FirstOffset
	}

	if config.StartOffset != FirstOffset && config.StartOffset != LastOffset {
		return errors.New(fmt.Sprintf("StartOffset is not valid %d", config.StartOffset))
	}

	if config.connect == nil {
		config.connect = connect
	}

	return nil
}

// PartitionAssignment represents the starting state of a partition that has
// been assigned to a consumer.
type PartitionAssignment struct {
	// ID is the partition ID.
	ID int

	// Offset is the initial offset at which this assignment begins.  It will
	// either be an absolute offset if one has previously been committed for
	// the consumer group or a relative offset such as FirstOffset when this
	// is the first time the partition have been assigned to a member of the
	// group.
	Offset int64
}

// genCtx adapts the done channel of the generation to a context.Context.  This
// is used by Generation.Start so that we can pass a context to go routines
// instead of passing around channels.
type genCtx struct {
	gen *Generation
}

func (c genCtx) Done() <-chan struct{} {
	return c.gen.done
}

func (c genCtx) Err() error {
	select {
	case <-c.gen.done:
		return ErrGenerationEnded
	default:
		return nil
	}
}

func (c genCtx) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (c genCtx) Value(interface{}) interface{} {
	return nil
}

// Generation represents a single consumer group generation.  The generation
// carries the topic+partition assignments for the given.  It also provides
// facilities for committing offsets and for running functions whose lifecycles
// are bound to the generation.
type Generation struct {
	// ID is the generation ID as assigned by the consumer group coordinator.
	ID int32

	// GroupID is the name of the consumer group.
	GroupID string

	// MemberID is the ID assigned to this consumer by the consumer group
	// coordinator.
	MemberID string

	// Assignments is the initial state of this Generation.  The partition
	// assignments are grouped by topic.
	Assignments map[string][]PartitionAssignment

	conn coordinator

	once sync.Once
	done chan struct{}
	wg   sync.WaitGroup

	retentionMillis int64
	log             func(func(Logger))
	logError        func(func(Logger))
}

// close stops the generation and waits for all functions launched via Start to
// terminate.
func (g *Generation) close() {
	g.once.Do(func() {
		close(g.done)
	})
	g.wg.Wait()
}

// Start launches the provided function in a go routine and adds accounting such
// that when the function exits, it stops the current generation (if not
// already in the process of doing so).
//
// The provided function MUST support cancellation via the ctx argument and exit
// in a timely manner once the ctx is complete.  When the context is closed, the
// context's Error() function will return ErrGenerationEnded.
//
// When closing out a generation, the consumer group will wait for all functions
// launched by Start to exit before the group can move on and join the next
// generation.  If the function does not exit promptly, it will stop forward
// progress for this consumer and potentially cause consumer group membership
// churn.
func (g *Generation) Start(fn func(ctx context.Context)) {
	g.wg.Add(1)
	go func() {
		fn(genCtx{g})
		// shut down the generation as soon as one function exits.  this is
		// different from close() in that it doesn't wait on the wg.
		g.once.Do(func() {
			close(g.done)
		})
		g.wg.Done()
	}()
}

// CommitOffsets commits the provided topic+partition+offset combos to the
// consumer group coordinator.  This can be used to reset the consumer to
// explicit offsets.
func (g *Generation) CommitOffsets(offsets map[string]map[int]int64) error {
	if len(offsets) == 0 {
		return nil
	}

	topics := make([]offsetCommitRequestV2Topic, 0, len(offsets))
	for topic, partitions := range offsets {
		t := offsetCommitRequestV2Topic{Topic: topic}
		for partition, offset := range partitions {
			t.Partitions = append(t.Partitions, offsetCommitRequestV2Partition{
				Partition: int32(partition),
				Offset:    offset,
			})
		}
		topics = append(topics, t)
	}

	request := offsetCommitRequestV2{
		GroupID:       g.GroupID,
		GenerationID:  g.ID,
		MemberID:      g.MemberID,
		RetentionTime: g.retentionMillis,
		Topics:        topics,
	}

	_, err := g.conn.offsetCommit(request)
	if err == nil {
		// if logging is enabled, print out the partitions that were committed.
		g.log(func(l Logger) {
			var report []string
			for _, t := range request.Topics {
				report = append(report, fmt.Sprintf("\ttopic: %s", t.Topic))
				for _, p := range t.Partitions {
					report = append(report, fmt.Sprintf("\t\tpartition %d: %d", p.Partition, p.Offset))
				}
			}
			l.Printf("committed offsets for group %s: \n%s", g.GroupID, strings.Join(report, "\n"))
		})
	}

	return err
}

// heartbeatLoop checks in with the consumer group coordinator at the provided
// interval.  It exits if it ever encounters an error, which would signal the
// end of the generation.
func (g *Generation) heartbeatLoop(interval time.Duration) {
	g.Start(func(ctx context.Context) {
		g.log(func(l Logger) {
			l.Printf("started heartbeat for group, %v [%v]", g.GroupID, interval)
		})
		defer g.log(func(l Logger) {
			l.Printf("stopped heartbeat for group %s\n", g.GroupID)
		})

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := g.conn.heartbeat(heartbeatRequestV0{
					GroupID:      g.GroupID,
					GenerationID: g.ID,
					MemberID:     g.MemberID,
				})
				if err != nil {
					return
				}
			}
		}
	})
}

// partitionWatcher queries kafka and watches for partition changes, triggering
// a rebalance if changes are found. Similar to heartbeat it's okay to return on
// error here as if you are unable to ask a broker for basic metadata you're in
// a bad spot and should rebalance. Commonly you will see an error here if there
// is a problem with the connection to the coordinator and a rebalance will
// establish a new connection to the coordinator.
func (g *Generation) partitionWatcher(interval time.Duration, topic string) {
	g.Start(func(ctx context.Context) {
		g.log(func(l Logger) {
			l.Printf("started partition watcher for group, %v, topic %v [%v]", g.GroupID, topic, interval)
		})
		defer g.log(func(l Logger) {
			l.Printf("stopped partition watcher for group, %v, topic %v", g.GroupID, topic)
		})

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		ops, err := g.conn.ReadPartitions(topic)
		if err != nil {
			g.logError(func(l Logger) {
				l.Printf("Problem getting partitions during startup, %v\n, Returning and setting up nextGeneration", err)
			})
			return
		}
		oParts := len(ops)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ops, err := g.conn.ReadPartitions(topic)
				switch err {
				case nil, UnknownTopicOrPartition:
					if len(ops) != oParts {
						g.log(func(l Logger) {
							l.Printf("Partition changes found, reblancing group: %v.", g.GroupID)
						})
						return
					}
				default:
					g.logError(func(l Logger) {
						l.Printf("Problem getting partitions while checking for changes, %v", err)
					})
					if _, ok := err.(Error); ok {
						continue
					}
					// other errors imply that we lost the connection to the coordinator, so we
					// should abort and reconnect.
					return
				}
			}
		}
	})
}

var _ coordinator = &Conn{}

// coordinator is a subset of the functionality in Conn in order to facilitate
// testing the consumer group...especially for error conditions that are
// difficult to instigate with a live broker running in docker.
type coordinator interface {
	io.Closer
	findCoordinator(findCoordinatorRequestV0) (findCoordinatorResponseV0, error)
	joinGroup(joinGroupRequestV1) (joinGroupResponseV1, error)
	syncGroup(syncGroupRequestV0) (syncGroupResponseV0, error)
	leaveGroup(leaveGroupRequestV0) (leaveGroupResponseV0, error)
	heartbeat(heartbeatRequestV0) (heartbeatResponseV0, error)
	offsetFetch(offsetFetchRequestV1) (offsetFetchResponseV1, error)
	offsetCommit(offsetCommitRequestV2) (offsetCommitResponseV2, error)
	ReadPartitions(...string) ([]Partition, error)
}

// NewConsumerGroup creates a new ConsumerGroup.  It returns an error if the
// provided configuration is invalid.  It does not attempt to connect to the
// Kafka cluster.  That happens asynchronously, and any errors will be reported
// by Next.
func NewConsumerGroup(config ConsumerGroupConfig) (*ConsumerGroup, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	cg := &ConsumerGroup{
		config: config,
		next:   make(chan *Generation),
		errs:   make(chan error),
		done:   make(chan struct{}),
	}
	cg.wg.Add(1)
	go func() {
		cg.run()
		cg.wg.Done()
	}()
	return cg, nil
}

// ConsumerGroup models a Kafka consumer group.  A caller doesn't interact with
// the group directly.  Rather, they interact with a Generation.  Every time a
// member enters or exits the group, it results in a new Generation.  The
// Generation is where partition assignments and offset management occur.
// Callers will use Next to get a handle to the Generation.
type ConsumerGroup struct {
	config ConsumerGroupConfig
	next   chan *Generation
	errs   chan error

	closeOnce sync.Once
	wg        sync.WaitGroup
	done      chan struct{}
}

// Close terminates the current generation by causing this member to leave and
// releases all local resources used to participate in the consumer group.
// Close will also end the current generation if it is still active.
func (cg *ConsumerGroup) Close() error {
	cg.closeOnce.Do(func() {
		close(cg.done)
	})
	cg.wg.Wait()
	return nil
}

// Next waits for the next consumer group generation.  There will never be two
// active generations.  Next will never return a new generation until the
// previous one has completed.
//
// If there are errors setting up the next generation, they will be surfaced
// here.
//
// If the ConsumerGroup has been closed, then Next will return ErrGroupClosed.
func (cg *ConsumerGroup) Next(ctx context.Context) (*Generation, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-cg.done:
		return nil, ErrGroupClosed
	case err := <-cg.errs:
		return nil, err
	case next := <-cg.next:
		return next, nil
	}
}

func (cg *ConsumerGroup) run() {
	// the memberID is the only piece of information that is maintained across
	// generations.  it starts empty and will be assigned on the first nextGeneration
	// when the joinGroup request is processed.  it may change again later if
	// the CG coordinator fails over or if the member is evicted.  otherwise, it
	// will be constant for the lifetime of this group.
	var memberID string
	var err error
	for {
		memberID, err = cg.nextGeneration(memberID)

		// backoff will be set if this go routine should sleep before continuing
		// to the next generation.  it will be non-nil in the case of an error
		// joining or syncing the group.
		var backoff <-chan time.Time
		switch err {
		case nil:
			// no error...the previous generation finished normally.
			continue
		case ErrGroupClosed:
			// the CG has been closed...leave the group and exit loop.
			_ = cg.leaveGroup(memberID)
			return
		case RebalanceInProgress:
			// in case of a RebalanceInProgress, don't leave the group or
			// change the member ID, but report the error.  the next attempt
			// to join the group will then be subject to the rebalance
			// timeout, so the broker will be responsible for throttling
			// this loop.
		default:
			// leave the group and report the error if we had gotten far
			// enough so as to have a member ID.  also clear the member id
			// so we don't attempt to use it again.  in order to avoid
			// a tight error loop, backoff before the next attempt to join
			// the group.
			_ = cg.leaveGroup(memberID)
			memberID = ""
			backoff = time.After(cg.config.JoinGroupBackoff)
		}
		// ensure that we exit cleanly in case the CG is done and no one is
		// waiting to receive on the unbuffered error channel.
		select {
		case <-cg.done:
			return
		case cg.errs <- err:
		}
		// backoff if needed, being sure to exit cleanly if the CG is done.
		if backoff != nil {
			select {
			case <-cg.done:
				// exit cleanly if the group is closed.
				return
			case <-backoff:
			}
		}
	}
}

func (cg *ConsumerGroup) nextGeneration(memberID string) (string, error) {
	// get a new connection to the coordinator on each loop.  the previous
	// generation could have exited due to losing the connection, so this
	// ensures that we always have a clean starting point.  it means we will
	// re-connect in certain cases, but that shouldn't be an issue given that
	// rebalances are relatively infrequent under normal operating
	// conditions.
	conn, err := cg.coordinator()
	if err != nil {
		cg.withErrorLogger(func(log Logger) {
			log.Printf("Unable to establish connection to consumer group coordinator for group %s: %v", cg.config.ID, err)
		})
		return memberID, err // a prior memberID may still be valid, so don't return ""
	}
	defer conn.Close()

	var generationID int32
	var groupAssignments GroupMemberAssignments
	var assignments map[string][]int32

	// join group.  this will join the group and prepare assignments if our
	// consumer is elected leader.  it may also change or assign the member ID.
	memberID, generationID, groupAssignments, err = cg.joinGroup(conn, memberID)
	if err != nil {
		cg.withErrorLogger(func(log Logger) {
			log.Printf("Failed to join group %s: %v", cg.config.ID, err)
		})
		return memberID, err
	}
	cg.withLogger(func(log Logger) {
		log.Printf("Joined group %s as member %s in generation %d", cg.config.ID, memberID, generationID)
	})

	// sync group
	assignments, err = cg.syncGroup(conn, memberID, generationID, groupAssignments)
	if err != nil {
		cg.withErrorLogger(func(log Logger) {
			log.Printf("Failed to sync group %s: %v", cg.config.ID, err)
		})
		return memberID, err
	}

	// fetch initial offsets.
	var offsets map[string]map[int]int64
	offsets, err = cg.fetchOffsets(conn, assignments)
	if err != nil {
		cg.withErrorLogger(func(log Logger) {
			log.Printf("Failed to fetch offsets for group %s: %v", cg.config.ID, err)
		})
		return memberID, err
	}

	// create the generation.
	gen := Generation{
		ID:              generationID,
		GroupID:         cg.config.ID,
		MemberID:        memberID,
		Assignments:     cg.makeAssignments(assignments, offsets),
		conn:            conn,
		done:            make(chan struct{}),
		retentionMillis: int64(cg.config.RetentionTime / time.Millisecond),
		log:             cg.withLogger,
		logError:        cg.withErrorLogger,
	}

	// spawn all of the go routines required to facilitate this generation.  if
	// any of these functions exit, then the generation is determined to be
	// complete.
	gen.heartbeatLoop(cg.config.HeartbeatInterval)
	if cg.config.WatchPartitionChanges {
		for _, topic := range cg.config.Topics {
			gen.partitionWatcher(cg.config.PartitionWatchInterval, topic)
		}
	}

	// make this generation available for retrieval.  if the CG is closed before
	// we can send it on the channel, exit.  that case is required b/c the next
	// channel is unbuffered.  if the caller to Next has already bailed because
	// it's own teardown logic has been invoked, this would deadlock otherwise.
	select {
	case <-cg.done:
		gen.close()
		return memberID, ErrGroupClosed // ErrGroupClosed will trigger leave logic.
	case cg.next <- &gen:
	}

	// wait for generation to complete.  if the CG is closed before the
	// generation is finished, exit and leave the group.
	select {
	case <-cg.done:
		gen.close()
		return memberID, ErrGroupClosed // ErrGroupClosed will trigger leave logic.
	case <-gen.done:
		// time for next generation!  make sure all the current go routines exit
		// before continuing onward.
		gen.close()
		return memberID, nil
	}
}

// connect returns a connection to ANY broker
func connect(dialer *Dialer, brokers ...string) (conn coordinator, err error) {
	for _, broker := range brokers {
		if conn, err = dialer.Dial("tcp", broker); err == nil {
			return
		}
	}
	return // err will be non-nil
}

// coordinator establishes a connection to the coordinator for this consumer
// group.
func (cg *ConsumerGroup) coordinator() (coordinator, error) {
	// NOTE : could try to cache the coordinator to avoid the double connect
	//        here.  since consumer group balances happen infrequently and are
	//        an expensive operation, we're not currently optimizing that case
	//        in order to keep the code simpler.
	conn, err := cg.config.connect(cg.config.Dialer, cg.config.Brokers...)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	out, err := conn.findCoordinator(findCoordinatorRequestV0{
		CoordinatorKey: cg.config.ID,
	})
	if err == nil && out.ErrorCode != 0 {
		err = Error(out.ErrorCode)
	}
	if err != nil {
		return nil, err
	}

	address := fmt.Sprintf("%v:%v", out.Coordinator.Host, out.Coordinator.Port)
	return cg.config.connect(cg.config.Dialer, address)
}

// joinGroup attempts to join the reader to the consumer group.
// Returns GroupMemberAssignments is this Reader was selected as
// the leader.  Otherwise, GroupMemberAssignments will be nil.
//
// Possible kafka error codes returned:
//  * GroupLoadInProgress:
//  * GroupCoordinatorNotAvailable:
//  * NotCoordinatorForGroup:
//  * InconsistentGroupProtocol:
//  * InvalidSessionTimeout:
//  * GroupAuthorizationFailed:
func (cg *ConsumerGroup) joinGroup(conn coordinator, memberID string) (string, int32, GroupMemberAssignments, error) {
	request, err := cg.makeJoinGroupRequestV1(memberID)
	if err != nil {
		return "", 0, nil, err
	}

	response, err := conn.joinGroup(request)
	if err == nil && response.ErrorCode != 0 {
		err = Error(response.ErrorCode)
	}
	if err != nil {
		return "", 0, nil, err
	}

	memberID = response.MemberID
	generationID := response.GenerationID

	cg.withLogger(func(l Logger) {
		l.Printf("joined group %s as member %s in generation %d", cg.config.ID, memberID, generationID)
	})

	var assignments GroupMemberAssignments
	if iAmLeader := response.MemberID == response.LeaderID; iAmLeader {
		v, err := cg.assignTopicPartitions(conn, response)
		if err != nil {
			return memberID, 0, nil, err
		}
		assignments = v

		cg.withLogger(func(l Logger) {
			for memberID, assignment := range assignments {
				for topic, partitions := range assignment {
					l.Printf("assigned member/topic/partitions %v/%v/%v", memberID, topic, partitions)
				}
			}
		})
	}

	cg.withLogger(func(l Logger) {
		l.Printf("joinGroup succeeded for response, %v.  generationID=%v, memberID=%v", cg.config.ID, response.GenerationID, response.MemberID)
	})

	return memberID, generationID, assignments, nil
}

// makeJoinGroupRequestV1 handles the logic of constructing a joinGroup
// request
func (cg *ConsumerGroup) makeJoinGroupRequestV1(memberID string) (joinGroupRequestV1, error) {
	request := joinGroupRequestV1{
		GroupID:          cg.config.ID,
		MemberID:         memberID,
		SessionTimeout:   int32(cg.config.SessionTimeout / time.Millisecond),
		RebalanceTimeout: int32(cg.config.RebalanceTimeout / time.Millisecond),
		ProtocolType:     defaultProtocolType,
	}

	for _, balancer := range cg.config.GroupBalancers {
		userData, err := balancer.UserData()
		if err != nil {
			return joinGroupRequestV1{}, fmt.Errorf("unable to construct protocol metadata for member, %v: %v", balancer.ProtocolName(), err)
		}
		request.GroupProtocols = append(request.GroupProtocols, joinGroupRequestGroupProtocolV1{
			ProtocolName: balancer.ProtocolName(),
			ProtocolMetadata: groupMetadata{
				Version:  1,
				Topics:   cg.config.Topics,
				UserData: userData,
			}.bytes(),
		})
	}

	return request, nil
}

// assignTopicPartitions uses the selected GroupBalancer to assign members to
// their various partitions
func (cg *ConsumerGroup) assignTopicPartitions(conn coordinator, group joinGroupResponseV1) (GroupMemberAssignments, error) {
	cg.withLogger(func(l Logger) {
		l.Printf("selected as leader for group, %s\n", cg.config.ID)
	})

	balancer, ok := findGroupBalancer(group.GroupProtocol, cg.config.GroupBalancers)
	if !ok {
		// NOTE : this shouldn't happen in practice...the broker should not
		//        return successfully from joinGroup unless all members support
		//        at least one common protocol.
		return nil, fmt.Errorf("unable to find selected balancer, %v, for group, %v", group.GroupProtocol, cg.config.ID)
	}

	members, err := cg.makeMemberProtocolMetadata(group.Members)
	if err != nil {
		return nil, err
	}

	topics := extractTopics(members)
	partitions, err := conn.ReadPartitions(topics...)

	// it's not a failure if the topic doesn't exist yet.  it results in no
	// assignments for the topic.  this matches the behavior of the official
	// clients: java, python, and librdkafka.
	// a topic watcher can trigger a rebalance when the topic comes into being.
	if err != nil && err != UnknownTopicOrPartition {
		return nil, err
	}

	cg.withLogger(func(l Logger) {
		l.Printf("using '%v' balancer to assign group, %v", group.GroupProtocol, cg.config.ID)
		for _, member := range members {
			l.Printf("found member: %v/%#v", member.ID, member.UserData)
		}
		for _, partition := range partitions {
			l.Printf("found topic/partition: %v/%v", partition.Topic, partition.ID)
		}
	})

	return balancer.AssignGroups(members, partitions), nil
}

// makeMemberProtocolMetadata maps encoded member metadata ([]byte) into []GroupMember
func (cg *ConsumerGroup) makeMemberProtocolMetadata(in []joinGroupResponseMemberV1) ([]GroupMember, error) {
	members := make([]GroupMember, 0, len(in))
	for _, item := range in {
		metadata := groupMetadata{}
		reader := bufio.NewReader(bytes.NewReader(item.MemberMetadata))
		if remain, err := (&metadata).readFrom(reader, len(item.MemberMetadata)); err != nil || remain != 0 {
			return nil, fmt.Errorf("unable to read metadata for member, %v: %v", item.MemberID, err)
		}

		members = append(members, GroupMember{
			ID:       item.MemberID,
			Topics:   metadata.Topics,
			UserData: metadata.UserData,
		})
	}
	return members, nil
}

// syncGroup completes the consumer group nextGeneration by accepting the
// memberAssignments (if this Reader is the leader) and returning this
// Readers subscriptions topic => partitions
//
// Possible kafka error codes returned:
//  * GroupCoordinatorNotAvailable:
//  * NotCoordinatorForGroup:
//  * IllegalGeneration:
//  * RebalanceInProgress:
//  * GroupAuthorizationFailed:
func (cg *ConsumerGroup) syncGroup(conn coordinator, memberID string, generationID int32, memberAssignments GroupMemberAssignments) (map[string][]int32, error) {
	request := cg.makeSyncGroupRequestV0(memberID, generationID, memberAssignments)
	response, err := conn.syncGroup(request)
	if err == nil && response.ErrorCode != 0 {
		err = Error(response.ErrorCode)
	}
	if err != nil {
		return nil, err
	}

	assignments := groupAssignment{}
	reader := bufio.NewReader(bytes.NewReader(response.MemberAssignments))
	if _, err := (&assignments).readFrom(reader, len(response.MemberAssignments)); err != nil {
		return nil, err
	}

	if len(assignments.Topics) == 0 {
		cg.withLogger(func(l Logger) {
			l.Printf("received empty assignments for group, %v as member %s for generation %d", cg.config.ID, memberID, generationID)
		})
	}

	cg.withLogger(func(l Logger) {
		l.Printf("sync group finished for group, %v", cg.config.ID)
	})

	return assignments.Topics, nil
}

func (cg *ConsumerGroup) makeSyncGroupRequestV0(memberID string, generationID int32, memberAssignments GroupMemberAssignments) syncGroupRequestV0 {
	request := syncGroupRequestV0{
		GroupID:      cg.config.ID,
		GenerationID: generationID,
		MemberID:     memberID,
	}

	if memberAssignments != nil {
		request.GroupAssignments = make([]syncGroupRequestGroupAssignmentV0, 0, 1)

		for memberID, topics := range memberAssignments {
			topics32 := make(map[string][]int32)
			for topic, partitions := range topics {
				partitions32 := make([]int32, len(partitions))
				for i := range partitions {
					partitions32[i] = int32(partitions[i])
				}
				topics32[topic] = partitions32
			}
			request.GroupAssignments = append(request.GroupAssignments, syncGroupRequestGroupAssignmentV0{
				MemberID: memberID,
				MemberAssignments: groupAssignment{
					Version: 1,
					Topics:  topics32,
				}.bytes(),
			})
		}

		cg.withErrorLogger(func(logger Logger) {
			logger.Printf("Syncing %d assignments for generation %d as member %s", len(request.GroupAssignments), generationID, memberID)
		})
	}

	return request
}

func (cg *ConsumerGroup) fetchOffsets(conn coordinator, subs map[string][]int32) (map[string]map[int]int64, error) {
	req := offsetFetchRequestV1{
		GroupID: cg.config.ID,
		Topics:  make([]offsetFetchRequestV1Topic, 0, len(cg.config.Topics)),
	}
	for _, topic := range cg.config.Topics {
		req.Topics = append(req.Topics, offsetFetchRequestV1Topic{
			Topic:      topic,
			Partitions: subs[topic],
		})
	}
	offsets, err := conn.offsetFetch(req)
	if err != nil {
		return nil, err
	}

	offsetsByTopic := make(map[string]map[int]int64)
	for _, res := range offsets.Responses {
		offsetsByPartition := map[int]int64{}
		offsetsByTopic[res.Topic] = offsetsByPartition
		for _, pr := range res.PartitionResponses {
			for _, partition := range subs[res.Topic] {
				if partition == pr.Partition {
					offset := pr.Offset
					if offset < 0 {
						offset = cg.config.StartOffset
					}
					offsetsByPartition[int(partition)] = offset
				}
			}
		}
	}

	return offsetsByTopic, nil
}

func (cg *ConsumerGroup) makeAssignments(assignments map[string][]int32, offsets map[string]map[int]int64) map[string][]PartitionAssignment {
	topicAssignments := make(map[string][]PartitionAssignment)
	for _, topic := range cg.config.Topics {
		topicPartitions := assignments[topic]
		topicAssignments[topic] = make([]PartitionAssignment, 0, len(topicPartitions))
		for _, partition := range topicPartitions {
			var offset int64
			partitionOffsets, ok := offsets[topic]
			if ok {
				offset, ok = partitionOffsets[int(partition)]
			}
			if !ok {
				offset = cg.config.StartOffset
			}
			topicAssignments[topic] = append(topicAssignments[topic], PartitionAssignment{
				ID:     int(partition),
				Offset: offset,
			})
		}
	}
	return topicAssignments
}

func (cg *ConsumerGroup) leaveGroup(memberID string) error {
	// don't attempt to leave the group if no memberID was ever assigned.
	if memberID == "" {
		return nil
	}

	cg.withLogger(func(log Logger) {
		log.Printf("Leaving group %s, member %s", cg.config.ID, memberID)
	})

	// IMPORTANT : leaveGroup establishes its own connection to the coordinator
	//             because it is often called after some other operation failed.
	//             said failure could be the result of connection-level issues,
	//             so we want to re-establish the connection to ensure that we
	//             are able to process the cleanup step.
	coordinator, err := cg.coordinator()
	if err != nil {
		return err
	}

	_, err = coordinator.leaveGroup(leaveGroupRequestV0{
		GroupID:  cg.config.ID,
		MemberID: memberID,
	})
	if err != nil {
		cg.withErrorLogger(func(log Logger) {
			log.Printf("leave group failed for group, %v, and member, %v: %v", cg.config.ID, memberID, err)
		})
	}

	_ = coordinator.Close()

	return err
}

func (cg *ConsumerGroup) withLogger(do func(Logger)) {
	if cg.config.Logger != nil {
		do(cg.config.Logger)
	}
}

func (cg *ConsumerGroup) withErrorLogger(do func(Logger)) {
	if cg.config.ErrorLogger != nil {
		do(cg.config.ErrorLogger)
	} else {
		cg.withLogger(do)
	}
}
