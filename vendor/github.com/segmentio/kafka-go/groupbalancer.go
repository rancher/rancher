package kafka

import "sort"

// GroupMember describes a single participant in a consumer group.
type GroupMember struct {
	// ID is the unique ID for this member as taken from the JoinGroup response.
	ID string

	// Topics is a list of topics that this member is consuming.
	Topics []string

	// UserData contains any information that the GroupBalancer sent to the
	// consumer group coordinator.
	UserData []byte
}

// GroupMemberAssignments holds MemberID => topic => partitions
type GroupMemberAssignments map[string]map[string][]int

// GroupBalancer encapsulates the client side rebalancing logic
type GroupBalancer interface {
	// ProtocolName of the GroupBalancer
	ProtocolName() string

	// UserData provides the GroupBalancer an opportunity to embed custom
	// UserData into the metadata.
	//
	// Will be used by JoinGroup to begin the consumer group handshake.
	//
	// See https://cwiki.apache.org/confluence/display/KAFKA/A+Guide+To+The+Kafka+Protocol#AGuideToTheKafkaProtocol-JoinGroupRequest
	UserData() ([]byte, error)

	// DefineMemberships returns which members will be consuming
	// which topic partitions
	AssignGroups(members []GroupMember, partitions []Partition) GroupMemberAssignments
}

// RangeGroupBalancer groups consumers by partition
//
// Example: 5 partitions, 2 consumers
// 		C0: [0, 1, 2]
// 		C1: [3, 4]
//
// Example: 6 partitions, 3 consumers
// 		C0: [0, 1]
// 		C1: [2, 3]
// 		C2: [4, 5]
//
type RangeGroupBalancer struct{}

func (r RangeGroupBalancer) ProtocolName() string {
	return "range"
}

func (r RangeGroupBalancer) UserData() ([]byte, error) {
	return nil, nil
}

func (r RangeGroupBalancer) AssignGroups(members []GroupMember, topicPartitions []Partition) GroupMemberAssignments {
	groupAssignments := GroupMemberAssignments{}
	membersByTopic := findMembersByTopic(members)

	for topic, members := range membersByTopic {
		partitions := findPartitions(topic, topicPartitions)
		partitionCount := len(partitions)
		memberCount := len(members)

		for memberIndex, member := range members {
			assignmentsByTopic, ok := groupAssignments[member.ID]
			if !ok {
				assignmentsByTopic = map[string][]int{}
				groupAssignments[member.ID] = assignmentsByTopic
			}

			minIndex := memberIndex * partitionCount / memberCount
			maxIndex := (memberIndex + 1) * partitionCount / memberCount

			for partitionIndex, partition := range partitions {
				if partitionIndex >= minIndex && partitionIndex < maxIndex {
					assignmentsByTopic[topic] = append(assignmentsByTopic[topic], partition)
				}
			}
		}
	}

	return groupAssignments
}

// RoundrobinGroupBalancer divides partitions evenly among consumers
//
// Example: 5 partitions, 2 consumers
// 		C0: [0, 2, 4]
// 		C1: [1, 3]
//
// Example: 6 partitions, 3 consumers
// 		C0: [0, 3]
// 		C1: [1, 4]
// 		C2: [2, 5]
//
type RoundRobinGroupBalancer struct{}

func (r RoundRobinGroupBalancer) ProtocolName() string {
	return "roundrobin"
}

func (r RoundRobinGroupBalancer) UserData() ([]byte, error) {
	return nil, nil
}

func (r RoundRobinGroupBalancer) AssignGroups(members []GroupMember, topicPartitions []Partition) GroupMemberAssignments {
	groupAssignments := GroupMemberAssignments{}
	membersByTopic := findMembersByTopic(members)
	for topic, members := range membersByTopic {
		partitionIDs := findPartitions(topic, topicPartitions)
		memberCount := len(members)

		for memberIndex, member := range members {
			assignmentsByTopic, ok := groupAssignments[member.ID]
			if !ok {
				assignmentsByTopic = map[string][]int{}
				groupAssignments[member.ID] = assignmentsByTopic
			}

			for partitionIndex, partition := range partitionIDs {
				if (partitionIndex % memberCount) == memberIndex {
					assignmentsByTopic[topic] = append(assignmentsByTopic[topic], partition)
				}
			}
		}
	}

	return groupAssignments
}

// findPartitions extracts the partition ids associated with the topic from the
// list of Partitions provided
func findPartitions(topic string, partitions []Partition) []int {
	var ids []int
	for _, partition := range partitions {
		if partition.Topic == topic {
			ids = append(ids, partition.ID)
		}
	}
	return ids
}

// findMembersByTopic groups the memberGroupMetadata by topic
func findMembersByTopic(members []GroupMember) map[string][]GroupMember {
	membersByTopic := map[string][]GroupMember{}
	for _, member := range members {
		for _, topic := range member.Topics {
			membersByTopic[topic] = append(membersByTopic[topic], member)
		}
	}

	// normalize ordering of members to enabling grouping across topics by partitions
	//
	// Want:
	// 		C0 [T0/P0, T1/P0]
	// 		C1 [T0/P1, T1/P1]
	//
	// Not:
	// 		C0 [T0/P0, T1/P1]
	// 		C1 [T0/P1, T1/P0]
	//
	// Even though the later is still round robin, the partitions are crossed
	//
	for _, members := range membersByTopic {
		sort.Slice(members, func(i, j int) bool {
			return members[i].ID < members[j].ID
		})
	}

	return membersByTopic
}

// findGroupBalancer returns the GroupBalancer with the specified protocolName
// from the slice provided
func findGroupBalancer(protocolName string, balancers []GroupBalancer) (GroupBalancer, bool) {
	for _, balancer := range balancers {
		if balancer.ProtocolName() == protocolName {
			return balancer, true
		}
	}
	return nil, false
}
