package kafka

import (
	"fmt"
	"io"
)

// Error represents the different error codes that may be returned by kafka.
type Error int

const (
	Unknown                            Error = -1
	OffsetOutOfRange                   Error = 1
	InvalidMessage                     Error = 2
	UnknownTopicOrPartition            Error = 3
	InvalidMessageSize                 Error = 4
	LeaderNotAvailable                 Error = 5
	NotLeaderForPartition              Error = 6
	RequestTimedOut                    Error = 7
	BrokerNotAvailable                 Error = 8
	ReplicaNotAvailable                Error = 9
	MessageSizeTooLarge                Error = 10
	StaleControllerEpoch               Error = 11
	OffsetMetadataTooLarge             Error = 12
	GroupLoadInProgress                Error = 14
	GroupCoordinatorNotAvailable       Error = 15
	NotCoordinatorForGroup             Error = 16
	InvalidTopic                       Error = 17
	RecordListTooLarge                 Error = 18
	NotEnoughReplicas                  Error = 19
	NotEnoughReplicasAfterAppend       Error = 20
	InvalidRequiredAcks                Error = 21
	IllegalGeneration                  Error = 22
	InconsistentGroupProtocol          Error = 23
	InvalidGroupId                     Error = 24
	UnknownMemberId                    Error = 25
	InvalidSessionTimeout              Error = 26
	RebalanceInProgress                Error = 27
	InvalidCommitOffsetSize            Error = 28
	TopicAuthorizationFailed           Error = 29
	GroupAuthorizationFailed           Error = 30
	ClusterAuthorizationFailed         Error = 31
	InvalidTimestamp                   Error = 32
	UnsupportedSASLMechanism           Error = 33
	IllegalSASLState                   Error = 34
	UnsupportedVersion                 Error = 35
	TopicAlreadyExists                 Error = 36
	InvalidPartitionNumber             Error = 37
	InvalidReplicationFactor           Error = 38
	InvalidReplicaAssignment           Error = 39
	InvalidConfiguration               Error = 40
	NotController                      Error = 41
	InvalidRequest                     Error = 42
	UnsupportedForMessageFormat        Error = 43
	PolicyViolation                    Error = 44
	OutOfOrderSequenceNumber           Error = 45
	DuplicateSequenceNumber            Error = 46
	InvalidProducerEpoch               Error = 47
	InvalidTransactionState            Error = 48
	InvalidProducerIDMapping           Error = 49
	InvalidTransactionTimeout          Error = 50
	ConcurrentTransactions             Error = 51
	TransactionCoordinatorFenced       Error = 52
	TransactionalIDAuthorizationFailed Error = 53
	SecurityDisabled                   Error = 54
	BrokerAuthorizationFailed          Error = 55
	KafkaStorageError                  Error = 56
	LogDirNotFound                     Error = 57
	SASLAuthenticationFailed           Error = 58
	UnknownProducerId                  Error = 59
	ReassignmentInProgress             Error = 60
	DelegationTokenAuthDisabled        Error = 61
	DelegationTokenNotFound            Error = 62
	DelegationTokenOwnerMismatch       Error = 63
	DelegationTokenRequestNotAllowed   Error = 64
	DelegationTokenAuthorizationFailed Error = 65
	DelegationTokenExpired             Error = 66
	InvalidPrincipalType               Error = 67
	NonEmptyGroup                      Error = 68
	GroupIdNotFound                    Error = 69
	FetchSessionIDNotFound             Error = 70
	InvalidFetchSessionEpoch           Error = 71
	ListenerNotFound                   Error = 72
	TopicDeletionDisabled              Error = 73
	FencedLeaderEpoch                  Error = 74
	UnknownLeaderEpoch                 Error = 75
	UnsupportedCompressionType         Error = 76
)

// Error satisfies the error interface.
func (e Error) Error() string {
	return fmt.Sprintf("[%d] %s: %s", e, e.Title(), e.Description())
}

// Timeout returns true if the error was due to a timeout.
func (e Error) Timeout() bool {
	return e == RequestTimedOut
}

// Temporary returns true if the operation that generated the error may succeed
// if retried at a later time.
func (e Error) Temporary() bool {
	return e == LeaderNotAvailable ||
		e == BrokerNotAvailable ||
		e == ReplicaNotAvailable ||
		e == GroupLoadInProgress ||
		e == GroupCoordinatorNotAvailable ||
		e == RebalanceInProgress ||
		e.Timeout()
}

// Title returns a human readable title for the error.
func (e Error) Title() string {
	switch e {
	case Unknown:
		return "Unknown"
	case OffsetOutOfRange:
		return "Offset Out Of Range"
	case InvalidMessage:
		return "Invalid Message"
	case UnknownTopicOrPartition:
		return "Unknown Topic Or Partition"
	case InvalidMessageSize:
		return "Invalid Message Size"
	case LeaderNotAvailable:
		return "Leader Not Available"
	case NotLeaderForPartition:
		return "Not Leader For Partition"
	case RequestTimedOut:
		return "Request Timed Out"
	case BrokerNotAvailable:
		return "Broker Not Available"
	case ReplicaNotAvailable:
		return "Replica Not Available"
	case MessageSizeTooLarge:
		return "Message Size Too Large"
	case StaleControllerEpoch:
		return "Stale Controller Epoch"
	case OffsetMetadataTooLarge:
		return "Offset Metadata Too Large"
	case GroupLoadInProgress:
		return "Group Load In Progress"
	case GroupCoordinatorNotAvailable:
		return "Group Coordinator Not Available"
	case NotCoordinatorForGroup:
		return "Not Coordinator For Group"
	case InvalidTopic:
		return "Invalid Topic"
	case RecordListTooLarge:
		return "Record List Too Large"
	case NotEnoughReplicas:
		return "Not Enough Replicas"
	case NotEnoughReplicasAfterAppend:
		return "Not Enough Replicas After Append"
	case InvalidRequiredAcks:
		return "Invalid Required Acks"
	case IllegalGeneration:
		return "Illegal Generation"
	case InconsistentGroupProtocol:
		return "Inconsistent Group Protocol"
	case InvalidGroupId:
		return "Invalid Group ID"
	case UnknownMemberId:
		return "Unknown Member ID"
	case InvalidSessionTimeout:
		return "Invalid Session Timeout"
	case RebalanceInProgress:
		return "Rebalance In Progress"
	case InvalidCommitOffsetSize:
		return "Invalid Commit Offset Size"
	case TopicAuthorizationFailed:
		return "Topic Authorization Failed"
	case GroupAuthorizationFailed:
		return "Group Authorization Failed"
	case ClusterAuthorizationFailed:
		return "Cluster Authorization Failed"
	case InvalidTimestamp:
		return "Invalid Timestamp"
	case UnsupportedSASLMechanism:
		return "Unsupported SASL Mechanism"
	case IllegalSASLState:
		return "Illegal SASL State"
	case UnsupportedVersion:
		return "Unsupported Version"
	case TopicAlreadyExists:
		return "Topic Already Exists"
	case InvalidPartitionNumber:
		return "Invalid Partition Number"
	case InvalidReplicationFactor:
		return "Invalid Replication Factor"
	case InvalidReplicaAssignment:
		return "Invalid Replica Assignment"
	case InvalidConfiguration:
		return "Invalid Configuration"
	case NotController:
		return "Not Controller"
	case InvalidRequest:
		return "Invalid Request"
	case UnsupportedForMessageFormat:
		return "Unsupported For Message Format"
	case PolicyViolation:
		return "Policy Violation"
	case OutOfOrderSequenceNumber:
		return "Out Of Order Sequence Number"
	case DuplicateSequenceNumber:
		return "Duplicate Sequence Number"
	case InvalidProducerEpoch:
		return "Invalid Producer Epoch"
	case InvalidTransactionState:
		return "Invalid Transaction State"
	case InvalidProducerIDMapping:
		return "Invalid Producer ID Mapping"
	case InvalidTransactionTimeout:
		return "Invalid Transaction Timeout"
	case ConcurrentTransactions:
		return "Concurrent Transactions"
	case TransactionCoordinatorFenced:
		return "Transaction Coordinator Fenced"
	case TransactionalIDAuthorizationFailed:
		return "Transactional ID Authorization Failed"
	case SecurityDisabled:
		return "Security Disabled"
	case BrokerAuthorizationFailed:
		return "Broker Authorization Failed"
	case KafkaStorageError:
		return "Kafka Storage Error"
	case LogDirNotFound:
		return "Log Dir Not Found"
	case SASLAuthenticationFailed:
		return "SASL Authentication Failed"
	case UnknownProducerId:
		return "Unknown Producer ID"
	case ReassignmentInProgress:
		return "Reassignment In Progress"
	case DelegationTokenAuthDisabled:
		return "Delegation Token Auth Disabled"
	case DelegationTokenNotFound:
		return "Delegation Token Not Found"
	case DelegationTokenOwnerMismatch:
		return "Delegation Token Owner Mismatch"
	case DelegationTokenRequestNotAllowed:
		return "Delegation Token Request Not Allowed"
	case DelegationTokenAuthorizationFailed:
		return "Delegation Token Authorization Failed"
	case DelegationTokenExpired:
		return "Delegation Token Expired"
	case InvalidPrincipalType:
		return "Invalid Principal Type"
	case NonEmptyGroup:
		return "Non Empty Group"
	case GroupIdNotFound:
		return "Group ID Not Found"
	case FetchSessionIDNotFound:
		return "Fetch Session ID Not Found"
	case InvalidFetchSessionEpoch:
		return "Invalid Fetch Session Epoch"
	case ListenerNotFound:
		return "Listener Not Found"
	case TopicDeletionDisabled:
		return "Topic Deletion Disabled"
	case FencedLeaderEpoch:
		return "Fenced Leader Epoch"
	case UnknownLeaderEpoch:
		return "Unknown Leader Epoch"
	case UnsupportedCompressionType:
		return "Unsupported Compression Type"
	}
	return ""
}

// Description returns a human readable description of cause of the error.
func (e Error) Description() string {
	switch e {
	case Unknown:
		return "an unexpected server error occurred"
	case OffsetOutOfRange:
		return "the requested offset is outside the range of offsets maintained by the server for the given topic/partition"
	case InvalidMessage:
		return "the message contents does not match its CRC"
	case UnknownTopicOrPartition:
		return "the request is for a topic or partition that does not exist on this broker"
	case InvalidMessageSize:
		return "the message has a negative size"
	case LeaderNotAvailable:
		return "the cluster is in the middle of a leadership election and there is currently no leader for this partition and hence it is unavailable for writes"
	case NotLeaderForPartition:
		return "the client attempted to send messages to a replica that is not the leader for some partition, the client's metadata are likely out of date"
	case RequestTimedOut:
		return "the request exceeded the user-specified time limit in the request"
	case BrokerNotAvailable:
		return "not a client facing error and is used mostly by tools when a broker is not alive"
	case ReplicaNotAvailable:
		return "a replica is expected on a broker, but is not (this can be safely ignored)"
	case MessageSizeTooLarge:
		return "the server has a configurable maximum message size to avoid unbounded memory allocation and the client attempted to produce a message larger than this maximum"
	case StaleControllerEpoch:
		return "internal error code for broker-to-broker communication"
	case OffsetMetadataTooLarge:
		return "the client specified a string larger than configured maximum for offset metadata"
	case GroupLoadInProgress:
		return "the broker returns this error code for an offset fetch request if it is still loading offsets (after a leader change for that offsets topic partition), or in response to group membership requests (such as heartbeats) when group metadata is being loaded by the coordinator"
	case GroupCoordinatorNotAvailable:
		return "the broker returns this error code for group coordinator requests, offset commits, and most group management requests if the offsets topic has not yet been created, or if the group coordinator is not active"
	case NotCoordinatorForGroup:
		return "the broker returns this error code if it receives an offset fetch or commit request for a group that it is not a coordinator for"
	case InvalidTopic:
		return "a request which attempted to access an invalid topic (e.g. one which has an illegal name), or if an attempt was made to write to an internal topic (such as the consumer offsets topic)"
	case RecordListTooLarge:
		return "a message batch in a produce request exceeds the maximum configured segment size"
	case NotEnoughReplicas:
		return "the number of in-sync replicas is lower than the configured minimum and requiredAcks is -1"
	case NotEnoughReplicasAfterAppend:
		return "the message was written to the log, but with fewer in-sync replicas than required."
	case InvalidRequiredAcks:
		return "the requested requiredAcks is invalid (anything other than -1, 1, or 0)"
	case IllegalGeneration:
		return "the generation id provided in the request is not the current generation"
	case InconsistentGroupProtocol:
		return "the member provided a protocol type or set of protocols which is not compatible with the current group"
	case InvalidGroupId:
		return "the group id is empty or null"
	case UnknownMemberId:
		return "the member id is not in the current generation"
	case InvalidSessionTimeout:
		return "the requested session timeout is outside of the allowed range on the broker"
	case RebalanceInProgress:
		return "the coordinator has begun rebalancing the group, the client should rejoin the group"
	case InvalidCommitOffsetSize:
		return "an offset commit was rejected because of oversize metadata"
	case TopicAuthorizationFailed:
		return "the client is not authorized to access the requested topic"
	case GroupAuthorizationFailed:
		return "the client is not authorized to access a particular group id"
	case ClusterAuthorizationFailed:
		return "the client is not authorized to use an inter-broker or administrative API"
	case InvalidTimestamp:
		return "the timestamp of the message is out of acceptable range"
	case UnsupportedSASLMechanism:
		return "the broker does not support the requested SASL mechanism"
	case IllegalSASLState:
		return "the request is not valid given the current SASL state"
	case UnsupportedVersion:
		return "the version of API is not supported"
	case TopicAlreadyExists:
		return "a topic with this name already exists"
	case InvalidPartitionNumber:
		return "the number of partitions is invalid"
	case InvalidReplicationFactor:
		return "the replication-factor is invalid"
	case InvalidReplicaAssignment:
		return "the replica assignment is invalid"
	case InvalidConfiguration:
		return "the configuration is invalid"
	case NotController:
		return "this is not the correct controller for this cluster"
	case InvalidRequest:
		return "this most likely occurs because of a request being malformed by the client library or the message was sent to an incompatible broker, se the broker logs for more details"
	case UnsupportedForMessageFormat:
		return "the message format version on the broker does not support the request"
	case PolicyViolation:
		return "the request parameters do not satisfy the configured policy"
	case OutOfOrderSequenceNumber:
		return "the broker received an out of order sequence number"
	case DuplicateSequenceNumber:
		return "the broker received a duplicate sequence number"
	case InvalidProducerEpoch:
		return "the producer attempted an operation with an old epoch, either there is a newer producer with the same transactional ID, or the producer's transaction has been expired by the broker"
	case InvalidTransactionState:
		return "the producer attempted a transactional operation in an invalid state"
	case InvalidProducerIDMapping:
		return "the producer attempted to use a producer id which is not currently assigned to its transactional ID"
	case InvalidTransactionTimeout:
		return "the transaction timeout is larger than the maximum value allowed by the broker (as configured by max.transaction.timeout.ms)"
	case ConcurrentTransactions:
		return "the producer attempted to update a transaction while another concurrent operation on the same transaction was ongoing"
	case TransactionCoordinatorFenced:
		return "the transaction coordinator sending a WriteTxnMarker is no longer the current coordinator for a given producer"
	case TransactionalIDAuthorizationFailed:
		return "the transactional ID authorization failed"
	case SecurityDisabled:
		return "the security features are disabled"
	case BrokerAuthorizationFailed:
		return "the broker authorization failed"
	case KafkaStorageError:
		return "disk error when trying to access log file on the disk"
	case LogDirNotFound:
		return "the user-specified log directory is not found in the broker config"
	case SASLAuthenticationFailed:
		return "SASL Authentication failed"
	case UnknownProducerId:
		return "the broker could not locate the producer metadata associated with the producer ID"
	case ReassignmentInProgress:
		return "a partition reassignment is in progress"
	case DelegationTokenAuthDisabled:
		return "delegation token feature is not enabled"
	case DelegationTokenNotFound:
		return "delegation token is not found on server"
	case DelegationTokenOwnerMismatch:
		return "specified principal is not valid owner/renewer"
	case DelegationTokenRequestNotAllowed:
		return "delegation token requests are not allowed on plaintext/1-way ssl channels and on delegation token authenticated channels"
	case DelegationTokenAuthorizationFailed:
		return "delegation token authorization failed"
	case DelegationTokenExpired:
		return "delegation token is expired"
	case InvalidPrincipalType:
		return "supplied principaltype is not supported"
	case NonEmptyGroup:
		return "the group is not empty"
	case GroupIdNotFound:
		return "the group ID does not exist"
	case FetchSessionIDNotFound:
		return "the fetch session ID was not found"
	case InvalidFetchSessionEpoch:
		return "the fetch session epoch is invalid"
	case ListenerNotFound:
		return "there is no listener on the leader broker that matches the listener on which metadata request was processed"
	case TopicDeletionDisabled:
		return "topic deletion is disabled"
	case FencedLeaderEpoch:
		return "the leader epoch in the request is older than the epoch on the broker"
	case UnknownLeaderEpoch:
		return "the leader epoch in the request is newer than the epoch on the broker"
	case UnsupportedCompressionType:
		return "the requesting client does not support the compression type of given partition"
	}
	return ""
}

func isTimeout(err error) bool {
	e, ok := err.(interface {
		Timeout() bool
	})
	return ok && e.Timeout()
}

func isTemporary(err error) bool {
	e, ok := err.(interface {
		Temporary() bool
	})
	return ok && e.Temporary()
}

func silentEOF(err error) error {
	if err == io.EOF {
		err = nil
	}
	return err
}

func dontExpectEOF(err error) error {
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return err
}

func coalesceErrors(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

type MessageTooLargeError struct {
	Message   Message
	Remaining []Message
}

func (e MessageTooLargeError) Error() string {
	return MessageSizeTooLarge.Error()
}
