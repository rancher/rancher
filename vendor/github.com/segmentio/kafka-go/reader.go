package kafka

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	LastOffset  int64 = -1 // The most recent offset available for a partition.
	FirstOffset       = -2 // The least recent offset available for a partition.
)

const (
	// defaultCommitRetries holds the number commit attempts to make
	// before giving up
	defaultCommitRetries = 3
)

var (
	errOnlyAvailableWithGroup = errors.New("unavailable when GroupID is not set")
	errNotAvailableWithGroup  = errors.New("unavailable when GroupID is set")
)

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

	// defaultRetentionTime holds the length of time a the consumer group will be
	// saved by kafka
	defaultRetentionTime = time.Hour * 24
)

// Reader provides a high-level API for consuming messages from kafka.
//
// A Reader automatically manages reconnections to a kafka server, and
// blocking methods have context support for asynchronous cancellations.
type Reader struct {
	// immutable fields of the reader
	config ReaderConfig

	// communication channels between the parent reader and its subreaders
	msgs chan readerMessage

	// mutable fields of the reader (synchronized on the mutex)
	mutex        sync.Mutex
	join         sync.WaitGroup
	cancel       context.CancelFunc
	stop         context.CancelFunc
	done         chan struct{}
	commits      chan commitRequest
	version      int64 // version holds the generation of the spawned readers
	offset       int64
	lag          int64
	closed       bool
	address      string // address of group coordinator
	generationID int32  // generationID of group
	memberID     string // memberID of group

	// offsetStash should only be managed by the commitLoopInterval.  We store
	// it here so that it survives rebalances
	offsetStash offsetStash

	// reader stats are all made of atomic values, no need for synchronization.
	once  uint32
	stctx context.Context
	// reader stats are all made of atomic values, no need for synchronization.
	// Use a pointer to ensure 64-bit alignment of the values.
	stats *readerStats
}

// useConsumerGroup indicates whether the Reader is part of a consumer group.
func (r *Reader) useConsumerGroup() bool { return r.config.GroupID != "" }

// useSyncCommits indicates whether the Reader is configured to perform sync or
// async commits.
func (r *Reader) useSyncCommits() bool { return r.config.CommitInterval == 0 }

// membership returns the group generationID and memberID of the reader.
//
// Only used when config.GroupID != ""
func (r *Reader) membership() (generationID int32, memberID string) {
	r.mutex.Lock()
	generationID = r.generationID
	memberID = r.memberID
	r.mutex.Unlock()
	return
}

// lookupCoordinator scans the brokers and looks up the address of the
// coordinator for the group.
//
// Only used when config.GroupID != ""
func (r *Reader) lookupCoordinator() (string, error) {
	conn, err := r.connect()
	if err != nil {
		return "", fmt.Errorf("unable to coordinator to any connect for group, %v: %v\n", r.config.GroupID, err)
	}
	defer conn.Close()

	out, err := conn.findCoordinator(findCoordinatorRequestV0{
		CoordinatorKey: r.config.GroupID,
	})
	if err != nil {
		return "", fmt.Errorf("unable to find coordinator for group, %v: %v", r.config.GroupID, err)
	}

	address := fmt.Sprintf("%v:%v", out.Coordinator.Host, out.Coordinator.Port)
	return address, nil
}

// refreshCoordinator updates the value of r.address
func (r *Reader) refreshCoordinator() (err error) {
	const (
		backoffDelayMin = 100 * time.Millisecond
		backoffDelayMax = 1 * time.Second
	)

	for attempt := 0; true; attempt++ {
		if attempt != 0 {
			if !sleep(r.stctx, backoff(attempt, backoffDelayMin, backoffDelayMax)) {
				return r.stctx.Err()
			}
		}

		address, err := r.lookupCoordinator()
		if err != nil {
			continue
		}

		r.mutex.Lock()
		oldAddress := r.address
		r.address = address
		r.mutex.Unlock()

		if address != oldAddress {
			r.withLogger(func(l *log.Logger) {
				l.Printf("coordinator for group, %v, set to %v\n", r.config.GroupID, address)
			})
		}

		break
	}

	return nil
}

// makejoinGroupRequestV1 handles the logic of constructing a joinGroup
// request
func (r *Reader) makejoinGroupRequestV1() (joinGroupRequestV1, error) {
	_, memberID := r.membership()

	request := joinGroupRequestV1{
		GroupID:          r.config.GroupID,
		MemberID:         memberID,
		SessionTimeout:   int32(r.config.SessionTimeout / time.Millisecond),
		RebalanceTimeout: int32(r.config.RebalanceTimeout / time.Millisecond),
		ProtocolType:     defaultProtocolType,
	}

	for _, balancer := range r.config.GroupBalancers {
		userData, err := balancer.UserData()
		if err != nil {
			return joinGroupRequestV1{}, fmt.Errorf("unable to construct protocol metadata for member, %v: %v\n", balancer.ProtocolName(), err)
		}
		request.GroupProtocols = append(request.GroupProtocols, joinGroupRequestGroupProtocolV1{
			ProtocolName: balancer.ProtocolName(),
			ProtocolMetadata: groupMetadata{
				Version:  1,
				Topics:   []string{r.config.Topic},
				UserData: userData,
			}.bytes(),
		})
	}

	return request, nil
}

// makeMemberProtocolMetadata maps encoded member metadata ([]byte) into []GroupMember
func (r *Reader) makeMemberProtocolMetadata(in []joinGroupResponseMemberV1) ([]GroupMember, error) {
	members := make([]GroupMember, 0, len(in))
	for _, item := range in {
		metadata := groupMetadata{}
		reader := bufio.NewReader(bytes.NewReader(item.MemberMetadata))
		if remain, err := (&metadata).readFrom(reader, len(item.MemberMetadata)); err != nil || remain != 0 {
			return nil, fmt.Errorf("unable to read metadata for member, %v: %v\n", item.MemberID, err)
		}

		members = append(members, GroupMember{
			ID:       item.MemberID,
			Topics:   metadata.Topics,
			UserData: metadata.UserData,
		})
	}
	return members, nil
}

// partitionReader is an internal interface used to simplify unit testing
type partitionReader interface {
	// ReadPartitions mirrors Conn.ReadPartitions
	ReadPartitions(topics ...string) (partitions []Partition, err error)
}

// assignTopicPartitions uses the selected GroupBalancer to assign members to
// their various partitions
func (r *Reader) assignTopicPartitions(conn partitionReader, group joinGroupResponseV1) (GroupMemberAssignments, error) {
	r.withLogger(func(l *log.Logger) {
		l.Println("selected as leader for group,", r.config.GroupID)
	})

	balancer, ok := findGroupBalancer(group.GroupProtocol, r.config.GroupBalancers)
	if !ok {
		return nil, fmt.Errorf("unable to find selected balancer, %v, for group, %v", group.GroupProtocol, r.config.GroupID)
	}

	members, err := r.makeMemberProtocolMetadata(group.Members)
	if err != nil {
		return nil, fmt.Errorf("unable to construct MemberProtocolMetadata: %v", err)
	}

	topics := extractTopics(members)
	partitions, err := conn.ReadPartitions(topics...)
	if err != nil {
		return nil, fmt.Errorf("unable to read partitions: %v", err)
	}

	r.withLogger(func(l *log.Logger) {
		l.Printf("using '%v' balancer to assign group, %v\n", group.GroupProtocol, r.config.GroupID)
		for _, member := range members {
			l.Printf("found member: %v/%#v", member.ID, member.UserData)
		}
		for _, partition := range partitions {
			l.Printf("found topic/partition: %v/%v", partition.Topic, partition.ID)
		}
	})

	return balancer.AssignGroups(members, partitions), nil
}

func (r *Reader) leaveGroup(conn *Conn) error {
	_, memberID := r.membership()
	_, err := conn.leaveGroup(leaveGroupRequestV0{
		GroupID:  r.config.GroupID,
		MemberID: memberID,
	})
	if err != nil {
		return fmt.Errorf("leave group failed for group, %v, and member, %v: %v", r.config.GroupID, memberID, err)
	}

	return nil
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
func (r *Reader) joinGroup() (GroupMemberAssignments, error) {
	conn, err := r.coordinator()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	request, err := r.makejoinGroupRequestV1()
	if err != nil {
		return nil, err
	}

	response, err := conn.joinGroup(request)
	if err != nil {
		switch err {
		case UnknownMemberId:
			r.mutex.Lock()
			r.memberID = ""
			r.mutex.Unlock()
			return nil, fmt.Errorf("joinGroup failed: %v", err)

		default:
			return nil, fmt.Errorf("joinGroup failed: %v", err)
		}
	}

	// Extract our membership and generationID from the response
	r.mutex.Lock()
	oldGenerationID := r.generationID
	oldMemberID := r.memberID
	r.generationID = response.GenerationID
	r.memberID = response.MemberID
	r.mutex.Unlock()

	if oldGenerationID != response.GenerationID || oldMemberID != response.MemberID {
		r.withLogger(func(l *log.Logger) {
			l.Printf("response membership changed.  generationID: %v => %v, memberID: '%v' => '%v'\n",
				oldGenerationID,
				response.GenerationID,
				oldMemberID,
				response.MemberID,
			)
		})
	}

	var assignments GroupMemberAssignments
	if iAmLeader := response.MemberID == response.LeaderID; iAmLeader {
		v, err := r.assignTopicPartitions(conn, response)
		if err != nil {
			_ = r.leaveGroup(conn)
			return nil, err
		}
		assignments = v

		r.withLogger(func(l *log.Logger) {
			for memberID, assignment := range assignments {
				for topic, partitions := range assignment {
					l.Printf("assigned member/topic/partitions %v/%v/%v\n", memberID, topic, partitions)
				}
			}
		})
	}

	r.withLogger(func(l *log.Logger) {
		l.Printf("joinGroup succeeded for response, %v.  generationID=%v, memberID=%v\n", r.config.GroupID, response.GenerationID, response.MemberID)
	})

	return assignments, nil
}

func (r *Reader) makeSyncGroupRequestV0(memberAssignments GroupMemberAssignments) syncGroupRequestV0 {
	generationID, memberID := r.membership()
	request := syncGroupRequestV0{
		GroupID:      r.config.GroupID,
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

		r.withErrorLogger(func(logger *log.Logger) {
			logger.Printf("Syncing %d assignments for generation %d as member %s", len(request.GroupAssignments), generationID, memberID)
		})
	}

	return request
}

// syncGroup completes the consumer group handshake by accepting the
// memberAssignments (if this Reader is the leader) and returning this
// Readers subscriptions topic => partitions
//
// Possible kafka error codes returned:
//  * GroupCoordinatorNotAvailable:
//  * NotCoordinatorForGroup:
//  * IllegalGeneration:
//  * RebalanceInProgress:
//  * GroupAuthorizationFailed:
func (r *Reader) syncGroup(memberAssignments GroupMemberAssignments) (map[string][]int32, error) {
	conn, err := r.coordinator()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	request := r.makeSyncGroupRequestV0(memberAssignments)
	response, err := conn.syncGroups(request)
	if err != nil {
		switch err {
		case RebalanceInProgress:
			// don't leave the group
			return nil, fmt.Errorf("syncGroup failed: %v", err)

		case UnknownMemberId:
			r.mutex.Lock()
			r.memberID = ""
			r.mutex.Unlock()
			_ = r.leaveGroup(conn)
			return nil, fmt.Errorf("syncGroup failed: %v", err)

		default:
			_ = r.leaveGroup(conn)
			return nil, fmt.Errorf("syncGroup failed: %v", err)
		}
	}

	assignments := groupAssignment{}
	reader := bufio.NewReader(bytes.NewReader(response.MemberAssignments))
	if _, err := (&assignments).readFrom(reader, len(response.MemberAssignments)); err != nil {
		_ = r.leaveGroup(conn)
		return nil, fmt.Errorf("unable to read SyncGroup response for group, %v: %v\n", r.config.GroupID, err)
	}

	if len(assignments.Topics) == 0 {
		generation, memberID := r.membership()
		return nil, fmt.Errorf("received empty assignments for group, %v as member %s for generation %d", r.config.GroupID, memberID, generation)
	}

	r.withLogger(func(l *log.Logger) {
		l.Printf("sync group finished for group, %v\n", r.config.GroupID)
	})

	return assignments.Topics, nil
}

func (r *Reader) rebalance() (map[string][]int32, error) {
	r.withLogger(func(l *log.Logger) {
		l.Printf("rebalancing consumer group, %v", r.config.GroupID)
	})

	if err := r.refreshCoordinator(); err != nil {
		return nil, err
	}

	members, err := r.joinGroup()
	if err != nil {
		return nil, err
	}

	assignments, err := r.syncGroup(members)
	if err != nil {
		return nil, err
	}

	return assignments, nil
}

func (r *Reader) unsubscribe() error {
	r.cancel()
	r.join.Wait()
	return nil
}

func (r *Reader) fetchOffsets(subs map[string][]int32) (map[int]int64, error) {
	conn, err := r.coordinator()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	partitions := subs[r.config.Topic]
	offsets, err := conn.offsetFetch(offsetFetchRequestV1{
		GroupID: r.config.GroupID,
		Topics: []offsetFetchRequestV1Topic{
			{
				Topic:      r.config.Topic,
				Partitions: partitions,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	offsetsByPartition := map[int]int64{}
	for _, pr := range offsets.Responses[0].PartitionResponses {
		for _, partition := range partitions {
			if partition == pr.Partition {
				offset := pr.Offset
				if offset < 0 {
					// No offset stored
					offset = FirstOffset
				}
				offsetsByPartition[int(partition)] = offset
			}
		}
	}

	return offsetsByPartition, nil
}

func (r *Reader) subscribe(subs map[string][]int32) error {
	if len(subs[r.config.Topic]) == 0 {
		return nil
	}

	offsetsByPartition, err := r.fetchOffsets(subs)
	if err != nil {
		if conn, err := r.coordinator(); err == nil {
			// make an attempt at leaving the group
			_ = r.leaveGroup(conn)
			conn.Close()
		}

		return err
	}

	r.mutex.Lock()
	r.start(offsetsByPartition)
	r.mutex.Unlock()

	r.withLogger(func(l *log.Logger) {
		l.Printf("subscribed to partitions: %+v", offsetsByPartition)
	})

	return nil
}

// connect returns a connection to ANY broker
func (r *Reader) connect() (conn *Conn, err error) {
	for _, broker := range r.config.Brokers {
		if conn, err = r.config.Dialer.Dial("tcp", broker); err == nil {
			return
		}
	}
	return // err will be non-nil
}

// coordinator returns a connection to the coordinator for this group
func (r *Reader) coordinator() (*Conn, error) {
	r.mutex.Lock()
	address := r.address
	r.mutex.Unlock()

	conn, err := r.config.Dialer.DialContext(r.stctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to coordinator, %v", address)
	}

	return conn, nil
}

func (r *Reader) waitThrottleTime(throttleTimeMS int32) {
	if throttleTimeMS == 0 {
		return
	}

	t := time.NewTimer(time.Duration(throttleTimeMS) * time.Millisecond)
	defer t.Stop()

	select {
	case <-r.stctx.Done():
		return
	case <-t.C:
	}
}

// heartbeat sends heartbeat to coordinator at the interval defined by
// ReaderConfig.HeartbeatInterval
func (r *Reader) heartbeat(conn *Conn) error {
	generationID, memberID := r.membership()
	if generationID == 0 && memberID == "" {
		return nil
	}

	_, err := conn.heartbeat(heartbeatRequestV0{
		GroupID:      r.config.GroupID,
		GenerationID: generationID,
		MemberID:     memberID,
	})
	if err != nil {
		return fmt.Errorf("heartbeat failed: %v", err)
	}

	return nil
}

func (r *Reader) heartbeatLoop(conn *Conn) func(stop <-chan struct{}) {
	return func(stop <-chan struct{}) {
		r.withLogger(func(l *log.Logger) {
			l.Printf("started heartbeat for group, %v [%v]", r.config.GroupID, r.config.HeartbeatInterval)
		})
		defer r.withLogger(func(l *log.Logger) {
			l.Println("stopped heartbeat for group,", r.config.GroupID)
		})

		ticker := time.NewTicker(r.config.HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := r.heartbeat(conn); err != nil {
					return
				}

			case <-stop:
				return
			}
		}
	}
}

type offsetCommitter interface {
	offsetCommit(request offsetCommitRequestV2) (offsetCommitResponseV2, error)
}

func (r *Reader) commitOffsets(conn offsetCommitter, offsetStash offsetStash) error {
	if len(offsetStash) == 0 {
		return nil
	}

	generationID, memberID := r.membership()
	request := offsetCommitRequestV2{
		GroupID:       r.config.GroupID,
		GenerationID:  generationID,
		MemberID:      memberID,
		RetentionTime: int64(r.config.RetentionTime / time.Millisecond),
	}

	for topic, partitions := range offsetStash {
		t := offsetCommitRequestV2Topic{Topic: topic}
		for partition, offset := range partitions {
			t.Partitions = append(t.Partitions, offsetCommitRequestV2Partition{
				Partition: int32(partition),
				Offset:    offset,
			})
		}
		request.Topics = append(request.Topics, t)
	}

	if _, err := conn.offsetCommit(request); err != nil {
		return fmt.Errorf("unable to commit offsets for group, %v: %v", r.config.GroupID, err)
	}

	r.withLogger(func(l *log.Logger) {
		l.Printf("committed offsets: %v", offsetStash)
	})

	return nil
}

// commitOffsetsWithRetry attempts to commit the specified offsets and retries
// up to the specified number of times
func (r *Reader) commitOffsetsWithRetry(conn offsetCommitter, offsetStash offsetStash, retries int) (err error) {
	const (
		backoffDelayMin = 100 * time.Millisecond
		backoffDelayMax = 5 * time.Second
	)

	for attempt := 0; attempt < retries; attempt++ {
		if attempt != 0 {
			if !sleep(r.stctx, backoff(attempt, backoffDelayMin, backoffDelayMax)) {
				return
			}
		}

		if err = r.commitOffsets(conn, offsetStash); err == nil {
			return
		}
	}

	return // err will not be nil
}

// offsetStash holds offsets by topic => partition => offset
type offsetStash map[string]map[int]int64

// merge updates the offsetStash with the offsets from the provided messages
func (o offsetStash) merge(commits []commit) {
	for _, c := range commits {
		offsetsByPartition, ok := o[c.topic]
		if !ok {
			offsetsByPartition = map[int]int64{}
			o[c.topic] = offsetsByPartition
		}

		if offset, ok := offsetsByPartition[c.partition]; !ok || c.offset > offset {
			offsetsByPartition[c.partition] = c.offset
		}
	}
}

// reset clears the contents of the offsetStash
func (o offsetStash) reset() {
	for key := range o {
		delete(o, key)
	}
}

// commitLoopImmediate handles each commit synchronously
func (r *Reader) commitLoopImmediate(conn offsetCommitter, stop <-chan struct{}) {
	offsetsByTopicAndPartition := offsetStash{}

	for {
		select {
		case <-stop:
			return

		case req := <-r.commits:
			offsetsByTopicAndPartition.merge(req.commits)
			req.errch <- r.commitOffsetsWithRetry(conn, offsetsByTopicAndPartition, defaultCommitRetries)
			offsetsByTopicAndPartition.reset()
		}
	}
}

// commitLoopInterval handles each commit asynchronously with a period defined
// by ReaderConfig.CommitInterval
func (r *Reader) commitLoopInterval(conn offsetCommitter, stop <-chan struct{}) {
	ticker := time.NewTicker(r.config.HeartbeatInterval)
	defer ticker.Stop()

	commit := func() {
		if err := r.commitOffsetsWithRetry(conn, r.offsetStash, defaultCommitRetries); err != nil {
			r.withErrorLogger(func(l *log.Logger) { l.Print(err) })
		} else {
			r.offsetStash.reset()
		}
	}

	for {
		select {
		case <-stop:
			commit()
			return

		case <-ticker.C:
			commit()

		case req := <-r.commits:
			r.offsetStash.merge(req.commits)
		}
	}
}

// commitLoop processes commits off the commit chan
func (r *Reader) commitLoop(conn *Conn) func(stop <-chan struct{}) {
	return func(stop <-chan struct{}) {
		r.withLogger(func(l *log.Logger) {
			l.Println("started commit for group,", r.config.GroupID)
		})
		defer r.withLogger(func(l *log.Logger) {
			l.Println("stopped commit for group,", r.config.GroupID)
		})

		if r.config.CommitInterval == 0 {
			r.commitLoopImmediate(conn, stop)
		} else {
			r.commitLoopInterval(conn, stop)
		}
	}
}

// handshake performs the necessary incantations to join this Reader to the desired
// consumer group.  handshake will be called whenever the group is disrupted
// (member join, member leave, coordinator changed, etc)
func (r *Reader) handshake() error {
	// always clear prior to subscribe
	r.unsubscribe()

	// rebalance and fetch assignments
	assignments, err := r.rebalance()
	if err != nil {
		return fmt.Errorf("rebalance failed for consumer group, %v: %v", r.config.GroupID, err)
	}

	conn, err := r.coordinator()
	if err != nil {
		return fmt.Errorf("heartbeat: unable to connect to coordinator: %v", err)
	}
	defer conn.Close()

	rg := &runGroup{}
	rg = rg.WithContext(r.stctx)
	rg.Go(r.heartbeatLoop(conn))
	rg.Go(r.commitLoop(conn))

	// subscribe to assignments
	if err := r.subscribe(assignments); err != nil {
		rg.Stop()
		return fmt.Errorf("subscribe failed for consumer group, %v: %v\n", r.config.GroupID, err)
	}

	rg.Wait()

	return nil
}

// run provides the main consumer group management loop.  Each iteration performs the
// handshake to join the Reader to the consumer group.
func (r *Reader) run() {
	defer close(r.done)

	if !r.useConsumerGroup() {
		return
	}

	r.withLogger(func(l *log.Logger) {
		l.Printf("entering loop for consumer group, %v\n", r.config.GroupID)
	})

	for {
		if err := r.handshake(); err != nil {
			r.withErrorLogger(func(l *log.Logger) {
				l.Println(err)
			})
		}

		select {
		case <-r.stctx.Done():
			return
		default:
		}
	}
}

// ReaderConfig is a configuration object used to create new instances of
// Reader.
type ReaderConfig struct {
	// The list of broker addresses used to connect to the kafka cluster.
	Brokers []string

	// GroupID holds the optional consumer group id.  If GroupID is specified, then
	// Partition should NOT be specified e.g. 0
	GroupID string

	// The topic to read messages from.
	Topic string

	// Partition to read messages from.  Either Partition or GroupID may
	// be assigned, but not both
	Partition int

	// An dialer used to open connections to the kafka server. This field is
	// optional, if nil, the default dialer is used instead.
	Dialer *Dialer

	// The capacity of the internal message queue, defaults to 100 if none is
	// set.
	QueueCapacity int

	// Min and max number of bytes to fetch from kafka in each request.
	MinBytes int
	MaxBytes int

	// Maximum amount of time to wait for new data to come when fetching batches
	// of messages from kafka.
	MaxWait time.Duration

	// ReadLagInterval sets the frequency at which the reader lag is updated.
	// Setting this field to a negative value disables lag reporting.
	ReadLagInterval time.Duration

	// GroupBalancers is the priority-ordered list of client-side consumer group
	// balancing strategies that will be offered to the coordinator.  The first
	// strategy that all group members support will be chosen by the leader.
	//
	// Default: [Range, RoundRobin]
	//
	// Only used when GroupID is set
	GroupBalancers []GroupBalancer

	// HeartbeatInterval sets the optional frequency at which the reader sends the consumer
	// group heartbeat update.
	//
	// Default: 3s
	//
	// Only used when GroupID is set
	HeartbeatInterval time.Duration

	// CommitInterval indicates the interval at which offsets are committed to
	// the broker.  If 0, commits will be handled synchronously.
	//
	// Defaults to 1s
	//
	// Only used when GroupID is set
	CommitInterval time.Duration

	// SessionTimeout optionally sets the length of time that may pass without a heartbeat
	// before the coordinator considers the consumer dead and initiates a rebalance.
	//
	// Default: 30s
	//
	// Only used when GroupID is set
	SessionTimeout time.Duration

	// RebalanceTimeout optionally sets the length of time the coordinator will wait
	// for members to join as part of a rebalance.  For kafka servers under higher
	// load, it may be useful to set this value higher.
	//
	// Default: 30s
	//
	// Only used when GroupID is set
	RebalanceTimeout time.Duration

	// RetentionTime optionally sets the length of time the consumer group will be saved
	// by the broker
	//
	// Default: 24h
	//
	// Only used when GroupID is set
	RetentionTime time.Duration

	// If not nil, specifies a logger used to report internal changes within the
	// reader.
	Logger *log.Logger

	// ErrorLogger is the logger used to report errors. If nil, the reader falls
	// back to using Logger instead.
	ErrorLogger *log.Logger
}

// ReaderStats is a data structure returned by a call to Reader.Stats that exposes
// details about the behavior of the reader.
type ReaderStats struct {
	Dials      int64 `metric:"kafka.reader.dial.count"      type:"counter"`
	Fetches    int64 `metric:"kafka.reader.fetch.count"     type:"counter"`
	Messages   int64 `metric:"kafka.reader.message.count"   type:"counter"`
	Bytes      int64 `metric:"kafka.reader.message.bytes"   type:"counter"`
	Rebalances int64 `metric:"kafka.reader.rebalance.count" type:"counter"`
	Timeouts   int64 `metric:"kafka.reader.timeout.count"   type:"counter"`
	Errors     int64 `metric:"kafka.reader.error.count"     type:"counter"`

	DialTime   DurationStats `metric:"kafka.reader.dial.seconds"`
	ReadTime   DurationStats `metric:"kafka.reader.read.seconds"`
	WaitTime   DurationStats `metric:"kafka.reader.wait.seconds"`
	FetchSize  SummaryStats  `metric:"kafka.reader.fetch.size"`
	FetchBytes SummaryStats  `metric:"kafka.reader.fetch.bytes"`

	Offset        int64         `metric:"kafka.reader.offset"          type:"gauge"`
	Lag           int64         `metric:"kafka.reader.lag"             type:"gauge"`
	MinBytes      int64         `metric:"kafka.reader.fetch_bytes.min" type:"gauge"`
	MaxBytes      int64         `metric:"kafka.reader.fetch_bytes.max" type:"gauge"`
	MaxWait       time.Duration `metric:"kafka.reader.fetch_wait.max"  type:"gauge"`
	QueueLength   int64         `metric:"kafka.reader.queue.length"    type:"gauge"`
	QueueCapacity int64         `metric:"kafka.reader.queue.capacity"  type:"gauge"`

	ClientID  string `tag:"client_id"`
	Topic     string `tag:"topic"`
	Partition string `tag:"partition"`

	// The original `Fetches` field had a typo where the metric name was called
	// "kafak..." instead of "kafka...", in order to offer time to fix monitors
	// that may be relying on this mistake we are temporarily introducing this
	// field.
	DeprecatedFetchesWithTypo int64 `metric:"kafak.reader.fetch.count" type:"counter"`
}

// readerStats is a struct that contains statistics on a reader.
type readerStats struct {
	dials      counter
	fetches    counter
	messages   counter
	bytes      counter
	rebalances counter
	timeouts   counter
	errors     counter
	dialTime   summary
	readTime   summary
	waitTime   summary
	fetchSize  summary
	fetchBytes summary
	offset     gauge
	lag        gauge
	partition  string
}

// NewReader creates and returns a new Reader configured with config.
// The offset is initialized to FirstOffset.
func NewReader(config ReaderConfig) *Reader {
	if len(config.Brokers) == 0 {
		panic("cannot create a new kafka reader with an empty list of broker addresses")
	}

	if len(config.Topic) == 0 {
		panic("cannot create a new kafka reader with an empty topic")
	}

	if config.Partition < 0 || config.Partition >= math.MaxInt32 {
		panic(fmt.Sprintf("partition number out of bounds: %d", config.Partition))
	}

	if config.MinBytes > config.MaxBytes {
		panic(fmt.Sprintf("minimum batch size greater than the maximum (min = %d, max = %d)", config.MinBytes, config.MaxBytes))
	}

	if config.MinBytes < 0 {
		panic(fmt.Sprintf("invalid negative minimum batch size (min = %d)", config.MinBytes))
	}

	if config.MaxBytes < 0 {
		panic(fmt.Sprintf("invalid negative maximum batch size (max = %d)", config.MaxBytes))
	}

	if config.GroupID != "" && config.Partition != 0 {
		panic("either Partition or GroupID may be specified, but not both")
	}

	if config.GroupID != "" {
		if len(config.GroupBalancers) == 0 {
			config.GroupBalancers = []GroupBalancer{
				RangeGroupBalancer{},
				RoundRobinGroupBalancer{},
			}
		}

		if config.HeartbeatInterval < 0 || (config.HeartbeatInterval/time.Millisecond) >= math.MaxInt32 {
			panic(fmt.Sprintf("HeartbeatInterval out of bounds: %d", config.HeartbeatInterval))
		}

		if config.SessionTimeout < 0 || (config.SessionTimeout/time.Millisecond) >= math.MaxInt32 {
			panic(fmt.Sprintf("SessionTimeout out of bounds: %d", config.SessionTimeout))
		}

		if config.RebalanceTimeout < 0 || (config.RebalanceTimeout/time.Millisecond) >= math.MaxInt32 {
			panic(fmt.Sprintf("RebalanceTimeout out of bounds: %d", config.RebalanceTimeout))
		}

		if config.RetentionTime < 0 {
			panic(fmt.Sprintf("RetentionTime out of bounds: %d", config.RetentionTime))
		}

		if config.CommitInterval < 0 || (config.CommitInterval/time.Millisecond) >= math.MaxInt32 {
			panic(fmt.Sprintf("CommitInterval out of bounds: %d", config.CommitInterval))
		}
	}

	if config.Dialer == nil {
		config.Dialer = DefaultDialer
	}

	if config.MaxBytes == 0 {
		config.MaxBytes = 1e6 // 1 MB
	}

	if config.MinBytes == 0 {
		config.MinBytes = config.MaxBytes
	}

	if config.MaxWait == 0 {
		config.MaxWait = 10 * time.Second
	}

	if config.ReadLagInterval == 0 {
		config.ReadLagInterval = 1 * time.Minute
	}

	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = defaultHeartbeatInterval
	}

	if config.SessionTimeout == 0 {
		config.SessionTimeout = defaultSessionTimeout
	}

	if config.RebalanceTimeout == 0 {
		config.RebalanceTimeout = defaultRebalanceTimeout
	}

	if config.RetentionTime == 0 {
		config.RetentionTime = defaultRetentionTime
	}

	if config.QueueCapacity == 0 {
		config.QueueCapacity = 100
	}

	// when configured as a consumer group; stats should report a partition of -1
	readerStatsPartition := config.Partition
	if config.GroupID != "" {
		readerStatsPartition = -1
	}

	// when configured as a consume group, start version as 1 to ensure that only
	// the rebalance function will start readers
	version := int64(0)
	if config.GroupID != "" {
		version = 1
	}

	stctx, stop := context.WithCancel(context.Background())
	r := &Reader{
		config:  config,
		msgs:    make(chan readerMessage, config.QueueCapacity),
		cancel:  func() {},
		done:    make(chan struct{}),
		commits: make(chan commitRequest, config.QueueCapacity),
		stop:    stop,
		offset:  FirstOffset,
		stctx:   stctx,
		stats: &readerStats{
			dialTime:   makeSummary(),
			readTime:   makeSummary(),
			waitTime:   makeSummary(),
			fetchSize:  makeSummary(),
			fetchBytes: makeSummary(),
			// Generate the string representation of the partition number only
			// once when the reader is created.
			partition: strconv.Itoa(readerStatsPartition),
		},
		version:     version,
		offsetStash: offsetStash{},
	}

	go r.run()

	return r
}

// Config returns the reader's configuration.
func (r *Reader) Config() ReaderConfig {
	return r.config
}

// Close closes the stream, preventing the program from reading any more
// messages from it.
func (r *Reader) Close() error {
	atomic.StoreUint32(&r.once, 1)

	r.mutex.Lock()
	closed := r.closed
	r.closed = true
	r.mutex.Unlock()

	r.cancel()
	r.stop()
	r.join.Wait()

	if r.useConsumerGroup() {
		// gracefully attempt to leave the consumer group on close
		if generationID, membershipID := r.membership(); generationID > 0 && membershipID != "" {
			if conn, err := r.coordinator(); err == nil {
				_ = r.leaveGroup(conn)
			}
		}
	}

	<-r.done

	if !closed {
		close(r.msgs)
	}

	return nil
}

// ReadMessage reads and return the next message from the r. The method call
// blocks until a message becomes available, or an error occurs. The program
// may also specify a context to asynchronously cancel the blocking operation.
//
// The method returns io.EOF to indicate that the reader has been closed.
//
// If consumer groups are used, ReadMessage will automatically commit the
// offset when called.
func (r *Reader) ReadMessage(ctx context.Context) (Message, error) {
	m, err := r.FetchMessage(ctx)
	if err != nil {
		return Message{}, err
	}

	if r.useConsumerGroup() {
		if err := r.CommitMessages(ctx, m); err != nil {
			return Message{}, err
		}
	}

	return m, nil
}

// FetchMessage reads and return the next message from the r. The method call
// blocks until a message becomes available, or an error occurs. The program
// may also specify a context to asynchronously cancel the blocking operation.
//
// The method returns io.EOF to indicate that the reader has been closed.
//
// FetchMessage does not commit offsets automatically when using consumer groups.
// Use CommitMessages to commit the offset.
func (r *Reader) FetchMessage(ctx context.Context) (Message, error) {
	r.activateReadLag()

	for {
		r.mutex.Lock()

		if !r.closed && r.version == 0 {
			r.start(map[int]int64{r.config.Partition: r.offset})
		}

		version := r.version
		r.mutex.Unlock()

		select {
		case <-ctx.Done():
			return Message{}, ctx.Err()

		case m, ok := <-r.msgs:
			if !ok {
				return Message{}, io.EOF
			}

			if m.version >= version {
				r.mutex.Lock()

				switch {
				case m.error != nil:
				case version == r.version:
					r.offset = m.message.Offset + 1
					r.lag = m.watermark - r.offset
				}

				r.mutex.Unlock()

				switch m.error {
				case nil:
				case io.EOF:
					// io.EOF is used as a marker to indicate that the stream
					// has been closed, in case it was received from the inner
					// reader we don't want to confuse the program and replace
					// the error with io.ErrUnexpectedEOF.
					m.error = io.ErrUnexpectedEOF
				}

				return m.message, m.error
			}
		}
	}
}

// CommitMessages commits the list of messages passed as argument. The program
// may pass a context to asynchronously cancel the commit operation when it was
// configured to be blocking.
func (r *Reader) CommitMessages(ctx context.Context, msgs ...Message) error {
	if !r.useConsumerGroup() {
		return errOnlyAvailableWithGroup
	}

	var errch <-chan error
	var sync = r.useSyncCommits()
	var creq = commitRequest{
		commits: makeCommits(msgs...),
	}

	if sync {
		ch := make(chan error, 1)
		errch, creq.errch = ch, ch
	}

	select {
	case r.commits <- creq:
	case <-ctx.Done():
		return ctx.Err()
	case <-r.stctx.Done():
		// This context is used to ensure we don't allow commits after the
		// reader was closed.
		return io.ErrClosedPipe
	}

	if !sync {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errch:
		return err
	}
}

// ReadLag returns the current lag of the reader by fetching the last offset of
// the topic and partition and computing the difference between that value and
// the offset of the last message returned by ReadMessage.
//
// This method is intended to be used in cases where a program may be unable to
// call ReadMessage to update the value returned by Lag, but still needs to get
// an up to date estimation of how far behind the reader is. For example when
// the consumer is not ready to process the next message.
//
// The function returns a lag of zero when the reader's current offset is
// negative.
func (r *Reader) ReadLag(ctx context.Context) (lag int64, err error) {
	if r.useConsumerGroup() {
		return 0, errNotAvailableWithGroup
	}

	type offsets struct {
		first int64
		last  int64
	}

	offch := make(chan offsets, 1)
	errch := make(chan error, 1)

	go func() {
		var off offsets
		var err error

		for _, broker := range r.config.Brokers {
			var conn *Conn

			if conn, err = r.config.Dialer.DialLeader(ctx, "tcp", broker, r.config.Topic, r.config.Partition); err != nil {
				continue
			}

			deadline, _ := ctx.Deadline()
			conn.SetDeadline(deadline)

			off.first, off.last, err = conn.ReadOffsets()
			conn.Close()

			if err == nil {
				break
			}
		}

		if err != nil {
			errch <- err
		} else {
			offch <- off
		}
	}()

	select {
	case off := <-offch:
		switch cur := r.Offset(); {
		case cur == FirstOffset:
			lag = off.last - off.first

		case cur == LastOffset:
			lag = 0

		default:
			lag = off.last - cur
		}
	case err = <-errch:
	case <-ctx.Done():
		err = ctx.Err()
	}

	return
}

// Offset returns the current absolute offset of the reader, or -1
// if r is backed by a consumer group.
func (r *Reader) Offset() int64 {
	if r.useConsumerGroup() {
		return -1
	}

	r.mutex.Lock()
	offset := r.offset
	r.mutex.Unlock()
	r.withLogger(func(log *log.Logger) {
		log.Printf("looking up offset of kafka reader for partition %d of %s: %d", r.config.Partition, r.config.Topic, offset)
	})
	return offset
}

// Lag returns the lag of the last message returned by ReadMessage, or -1
// if r is backed by a consumer group.
func (r *Reader) Lag() int64 {
	if r.useConsumerGroup() {
		return -1
	}

	r.mutex.Lock()
	lag := r.lag
	r.mutex.Unlock()
	return lag
}

// SetOffset changes the offset from which the next batch of messages will be
// read. The method fails with io.ErrClosedPipe if the reader has already been closed.
//
// From version 0.2.0, FirstOffset and LastOffset can be used to indicate the first
// or last available offset in the partition. Please note while -1 and -2 were accepted
// to indicate the first or last offset in previous versions, the meanings of the numbers
// were swapped in 0.2.0 to match the meanings in other libraries and the Kafka protocol
// specification.
func (r *Reader) SetOffset(offset int64) error {
	if r.useConsumerGroup() {
		return errNotAvailableWithGroup
	}

	var err error
	r.mutex.Lock()

	if r.closed {
		err = io.ErrClosedPipe
	} else if offset != r.offset {
		r.withLogger(func(log *log.Logger) {
			log.Printf("setting the offset of the kafka reader for partition %d of %s from %d to %d",
				r.config.Partition, r.config.Topic, r.offset, offset)
		})
		r.offset = offset

		if r.version != 0 {
			r.start(map[int]int64{r.config.Partition: r.offset})
		}

		r.activateReadLag()
	}

	r.mutex.Unlock()
	return err
}

// Stats returns a snapshot of the reader stats since the last time the method
// was called, or since the reader was created if it is called for the first
// time.
//
// A typical use of this method is to spawn a goroutine that will periodically
// call Stats on a kafka reader and report the metrics to a stats collection
// system.
func (r *Reader) Stats() ReaderStats {
	stats := ReaderStats{
		Dials:         r.stats.dials.snapshot(),
		Fetches:       r.stats.fetches.snapshot(),
		Messages:      r.stats.messages.snapshot(),
		Bytes:         r.stats.bytes.snapshot(),
		Rebalances:    r.stats.rebalances.snapshot(),
		Timeouts:      r.stats.timeouts.snapshot(),
		Errors:        r.stats.errors.snapshot(),
		DialTime:      r.stats.dialTime.snapshotDuration(),
		ReadTime:      r.stats.readTime.snapshotDuration(),
		WaitTime:      r.stats.waitTime.snapshotDuration(),
		FetchSize:     r.stats.fetchSize.snapshot(),
		FetchBytes:    r.stats.fetchBytes.snapshot(),
		Offset:        r.stats.offset.snapshot(),
		Lag:           r.stats.lag.snapshot(),
		MinBytes:      int64(r.config.MinBytes),
		MaxBytes:      int64(r.config.MaxBytes),
		MaxWait:       r.config.MaxWait,
		QueueLength:   int64(len(r.msgs)),
		QueueCapacity: int64(cap(r.msgs)),
		ClientID:      r.config.Dialer.ClientID,
		Topic:         r.config.Topic,
		Partition:     r.stats.partition,
	}
	// TODO: remove when we get rid of the deprecated field.
	stats.DeprecatedFetchesWithTypo = stats.Fetches
	return stats
}

func (r *Reader) withLogger(do func(*log.Logger)) {
	if r.config.Logger != nil {
		do(r.config.Logger)
	}
}

func (r *Reader) withErrorLogger(do func(*log.Logger)) {
	if r.config.ErrorLogger != nil {
		do(r.config.ErrorLogger)
	} else {
		r.withLogger(do)
	}
}

func (r *Reader) activateReadLag() {
	if r.config.ReadLagInterval > 0 && atomic.CompareAndSwapUint32(&r.once, 0, 1) {
		// read lag will only be calculated when not using consumer groups
		// todo discuss how capturing read lag should interact with rebalancing
		if !r.useConsumerGroup() {
			go r.readLag(r.stctx)
		}
	}
}

func (r *Reader) readLag(ctx context.Context) {
	ticker := time.NewTicker(r.config.ReadLagInterval)
	defer ticker.Stop()

	for {
		timeout, cancel := context.WithTimeout(ctx, r.config.ReadLagInterval/2)
		lag, err := r.ReadLag(timeout)
		cancel()

		if err != nil {
			r.stats.errors.observe(1)
			r.withErrorLogger(func(log *log.Logger) {
				log.Printf("kafka reader failed to read lag of partition %d of %s", r.config.Partition, r.config.Topic)
			})
		} else {
			r.stats.lag.observe(lag)
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

func (r *Reader) start(offsetsByPartition map[int]int64) {
	if r.closed {
		// don't start child reader if parent Reader is closed
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	r.cancel() // always cancel the previous reader
	r.cancel = cancel
	r.version++

	r.join.Add(len(offsetsByPartition))
	for partition, offset := range offsetsByPartition {
		go func(ctx context.Context, partition int, offset int64, join *sync.WaitGroup) {
			defer join.Done()

			(&reader{
				dialer:      r.config.Dialer,
				logger:      r.config.Logger,
				errorLogger: r.config.ErrorLogger,
				brokers:     r.config.Brokers,
				topic:       r.config.Topic,
				partition:   partition,
				minBytes:    r.config.MinBytes,
				maxBytes:    r.config.MaxBytes,
				maxWait:     r.config.MaxWait,
				version:     r.version,
				msgs:        r.msgs,
				stats:       r.stats,
			}).run(ctx, offset)
		}(ctx, partition, offset, &r.join)
	}
}

// A reader reads messages from kafka and produces them on its channels, it's
// used as an way to asynchronously fetch messages while the main program reads
// them using the high level reader API.
type reader struct {
	dialer      *Dialer
	logger      *log.Logger
	errorLogger *log.Logger
	brokers     []string
	topic       string
	partition   int
	minBytes    int
	maxBytes    int
	maxWait     time.Duration
	version     int64
	msgs        chan<- readerMessage
	stats       *readerStats
}

type readerMessage struct {
	version   int64
	message   Message
	watermark int64
	error     error
}

func (r *reader) run(ctx context.Context, offset int64) {
	const backoffDelayMin = 100 * time.Millisecond
	const backoffDelayMax = 1 * time.Second

	// This is the reader's main loop, it only ends if the context is canceled
	// and will keep attempting to reader messages otherwise.
	//
	// Retrying indefinitely has the nice side effect of preventing Read calls
	// on the parent reader to block if connection to the kafka server fails,
	// the reader keeps reporting errors on the error channel which will then
	// be surfaced to the program.
	// If the reader wasn't retrying then the program would block indefinitely
	// on a Read call after reading the first error.
	for attempt := 0; true; attempt++ {
		if attempt != 0 {
			if !sleep(ctx, backoff(attempt, backoffDelayMin, backoffDelayMax)) {
				return
			}
		}

		r.withLogger(func(log *log.Logger) {
			log.Printf("initializing kafka reader for partition %d of %s starting at offset %d", r.partition, r.topic, offset)
		})

		conn, start, err := r.initialize(ctx, offset)
		switch err {
		case nil:
		case OffsetOutOfRange:
			// This would happen if the requested offset is passed the last
			// offset on the partition leader. In that case we're just going
			// to retry later hoping that enough data has been produced.
			r.withErrorLogger(func(log *log.Logger) {
				log.Printf("error initializing the kafka reader for partition %d of %s: %s", r.partition, r.topic, OffsetOutOfRange)
			})
			continue
		default:
			// Wait 4 attempts before reporting the first errors, this helps
			// mitigate situations where the kafka server is temporarily
			// unavailable.
			if attempt >= 3 {
				r.sendError(ctx, err)
			} else {
				r.stats.errors.observe(1)
				r.withErrorLogger(func(log *log.Logger) {
					log.Printf("error initializing the kafka reader for partition %d of %s: %s", r.partition, r.topic, err)
				})
			}
			continue
		}

		// Resetting the attempt counter ensures that if a failure occurs after
		// a successful initialization we don't keep increasing the backoff
		// timeout.
		attempt = 0

		// Now we're sure to have an absolute offset number, may anything happen
		// to the connection we know we'll want to restart from this offset.
		offset = start

		errcount := 0
	readLoop:
		for {
			if !sleep(ctx, backoff(errcount, backoffDelayMin, backoffDelayMax)) {
				conn.Close()
				return
			}

			switch offset, err = r.read(ctx, offset, conn); err {
			case nil:
				errcount = 0

			case NotLeaderForPartition:
				r.withErrorLogger(func(log *log.Logger) {
					log.Printf("failed to read from current broker for partition %d of %s at offset %d, not the leader", r.partition, r.topic, offset)
				})

				conn.Close()

				// The next call to .initialize will re-establish a connection to the proper
				// partition leader.
				r.stats.rebalances.observe(1)
				break readLoop

			case RequestTimedOut:
				// Timeout on the kafka side, this can be safely retried.
				errcount = 0
				r.withErrorLogger(func(log *log.Logger) {
					log.Printf("no messages received from kafka within the allocated time for partition %d of %s at offset %d", r.partition, r.topic, offset)
				})
				r.stats.timeouts.observe(1)
				continue

			case OffsetOutOfRange:
				first, last, err := r.readOffsets(conn)

				if err != nil {
					r.withErrorLogger(func(log *log.Logger) {
						log.Printf("the kafka reader got an error while attempting to determine whether it was reading before the first offset or after the last offset of partition %d of %s: %s", r.partition, r.topic, err)
					})
					conn.Close()
					break readLoop
				}

				switch {
				case offset < first:
					r.withErrorLogger(func(log *log.Logger) {
						log.Printf("the kafka reader is reading before the first offset for partition %d of %s, skipping from offset %d to %d (%d messages)", r.partition, r.topic, offset, first, first-offset)
					})
					offset, errcount = first, 0
					continue // retry immediately so we don't keep falling behind due to the backoff

				case offset < last:
					errcount = 0
					continue // more messages have already become available, retry immediately

				default:
					// We may be reading past the last offset, will retry later.
					r.withErrorLogger(func(log *log.Logger) {
						log.Printf("the kafka reader is reading passed the last offset for partition %d of %s at offset %d", r.partition, r.topic, offset)
					})
				}

			case context.Canceled:
				// Another reader has taken over, we can safely quit.
				conn.Close()
				return

			default:
				if _, ok := err.(Error); ok {
					r.sendError(ctx, err)
				} else {
					r.withErrorLogger(func(log *log.Logger) {
						log.Printf("the kafka reader got an unknown error reading partition %d of %s at offset %d: %s", r.partition, r.topic, offset, err)
					})
					r.stats.errors.observe(1)
					conn.Close()
					break readLoop
				}
			}

			errcount++
		}
	}
}

func (r *reader) initialize(ctx context.Context, offset int64) (conn *Conn, start int64, err error) {
	for i := 0; i != len(r.brokers) && conn == nil; i++ {
		var broker = r.brokers[i]
		var first, last int64

		t0 := time.Now()
		conn, err = r.dialer.DialLeader(ctx, "tcp", broker, r.topic, r.partition)
		t1 := time.Now()
		r.stats.dials.observe(1)
		r.stats.dialTime.observeDuration(t1.Sub(t0))

		if err != nil {
			continue
		}

		if first, last, err = r.readOffsets(conn); err != nil {
			conn.Close()
			conn = nil
			break
		}

		switch {
		case offset == FirstOffset:
			offset = first

		case offset == LastOffset:
			offset = last

		case offset < first:
			offset = first
		}

		r.withLogger(func(log *log.Logger) {
			log.Printf("the kafka reader for partition %d of %s is seeking to offset %d", r.partition, r.topic, offset)
		})

		if start, err = conn.Seek(offset, SeekAbsolute); err != nil {
			conn.Close()
			conn = nil
			break
		}

		conn.SetDeadline(time.Time{})
	}

	return
}

func (r *reader) read(ctx context.Context, offset int64, conn *Conn) (int64, error) {
	r.stats.fetches.observe(1)
	r.stats.offset.observe(offset)

	t0 := time.Now()
	conn.SetReadDeadline(t0.Add(r.maxWait))

	batch := conn.ReadBatch(r.minBytes, r.maxBytes)
	highWaterMark := batch.HighWaterMark()

	t1 := time.Now()
	r.stats.waitTime.observeDuration(t1.Sub(t0))

	var msg Message
	var err error
	var size int64
	var bytes int64

	const safetyTimeout = 10 * time.Second
	deadline := time.Now().Add(safetyTimeout)
	conn.SetReadDeadline(deadline)

	for {
		if now := time.Now(); deadline.Sub(now) < (safetyTimeout / 2) {
			deadline = now.Add(safetyTimeout)
			conn.SetReadDeadline(deadline)
		}

		if msg, err = batch.ReadMessage(); err != nil {
			err = batch.Close()
			break
		}

		n := int64(len(msg.Key) + len(msg.Value))
		r.stats.messages.observe(1)
		r.stats.bytes.observe(n)

		if err = r.sendMessage(ctx, msg, highWaterMark); err != nil {
			err = batch.Close()
			break
		}

		offset = msg.Offset + 1
		r.stats.offset.observe(offset)
		r.stats.lag.observe(highWaterMark - offset)

		size++
		bytes += n
	}

	conn.SetReadDeadline(time.Time{})

	t2 := time.Now()
	r.stats.readTime.observeDuration(t2.Sub(t1))
	r.stats.fetchSize.observe(size)
	r.stats.fetchBytes.observe(bytes)
	return offset, err
}

func (r *reader) readOffsets(conn *Conn) (first, last int64, err error) {
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	return conn.ReadOffsets()
}

func (r *reader) sendMessage(ctx context.Context, msg Message, watermark int64) error {
	select {
	case r.msgs <- readerMessage{version: r.version, message: msg, watermark: watermark}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *reader) sendError(ctx context.Context, err error) error {
	select {
	case r.msgs <- readerMessage{version: r.version, error: err}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *reader) withLogger(do func(*log.Logger)) {
	if r.logger != nil {
		do(r.logger)
	}
}

func (r *reader) withErrorLogger(do func(*log.Logger)) {
	if r.errorLogger != nil {
		do(r.errorLogger)
	} else {
		r.withLogger(do)
	}
}

// extractTopics returns the unique list of topics represented by the set of
// provided members
func extractTopics(members []GroupMember) []string {
	var visited = map[string]struct{}{}
	var topics []string

	for _, member := range members {
		for _, topic := range member.Topics {
			if _, seen := visited[topic]; seen {
				continue
			}

			topics = append(topics, topic)
			visited[topic] = struct{}{}
		}
	}

	sort.Strings(topics)

	return topics
}
