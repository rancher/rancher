package kafka

// A commit represents the instruction of publishing an update of the last
// offset read by a program for a topic and partition.
type commit struct {
	topic     string
	partition int
	offset    int64
}

// makeCommit builds a commit value from a message, the resulting commit takes
// its topic, partition, and offset from the message.
func makeCommit(msg Message) commit {
	return commit{
		topic:     msg.Topic,
		partition: msg.Partition,
		offset:    msg.Offset + 1,
	}
}

// makeCommits generates a slice of commits from a list of messages, it extracts
// the topic, partition, and offset of each message and builds the corresponding
// commit slice.
func makeCommits(msgs ...Message) []commit {
	commits := make([]commit, len(msgs))

	for i, m := range msgs {
		commits[i] = makeCommit(m)
	}

	return commits
}

// commitRequest is the data type exchanged between the CommitMessages method
// and internals of the reader's implementation.
type commitRequest struct {
	commits []commit
	errch   chan<- error
}
