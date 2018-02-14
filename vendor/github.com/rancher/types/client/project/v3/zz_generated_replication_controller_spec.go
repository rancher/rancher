package client

const (
	ReplicationControllerSpecType                               = "replicationControllerSpec"
	ReplicationControllerSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	ReplicationControllerSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	ReplicationControllerSpecFieldContainers                    = "containers"
	ReplicationControllerSpecFieldDNSPolicy                     = "dnsPolicy"
	ReplicationControllerSpecFieldFsgid                         = "fsgid"
	ReplicationControllerSpecFieldGids                          = "gids"
	ReplicationControllerSpecFieldHostAliases                   = "hostAliases"
	ReplicationControllerSpecFieldHostIPC                       = "hostIPC"
	ReplicationControllerSpecFieldHostNetwork                   = "hostNetwork"
	ReplicationControllerSpecFieldHostPID                       = "hostPID"
	ReplicationControllerSpecFieldHostname                      = "hostname"
	ReplicationControllerSpecFieldImagePullSecrets              = "imagePullSecrets"
	ReplicationControllerSpecFieldNodeId                        = "nodeId"
	ReplicationControllerSpecFieldObjectMeta                    = "metadata"
	ReplicationControllerSpecFieldPriority                      = "priority"
	ReplicationControllerSpecFieldPriorityClassName             = "priorityClassName"
	ReplicationControllerSpecFieldReplicationController         = "replicationController"
	ReplicationControllerSpecFieldRestartPolicy                 = "restartPolicy"
	ReplicationControllerSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	ReplicationControllerSpecFieldSchedulerName                 = "schedulerName"
	ReplicationControllerSpecFieldScheduling                    = "scheduling"
	ReplicationControllerSpecFieldSelector                      = "selector"
	ReplicationControllerSpecFieldServiceAccountName            = "serviceAccountName"
	ReplicationControllerSpecFieldSubdomain                     = "subdomain"
	ReplicationControllerSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	ReplicationControllerSpecFieldUid                           = "uid"
	ReplicationControllerSpecFieldVolumes                       = "volumes"
)

type ReplicationControllerSpec struct {
	ActiveDeadlineSeconds         *int64                       `json:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                        `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container                  `json:"containers,omitempty"`
	DNSPolicy                     string                       `json:"dnsPolicy,omitempty"`
	Fsgid                         *int64                       `json:"fsgid,omitempty"`
	Gids                          []int64                      `json:"gids,omitempty"`
	HostAliases                   []HostAlias                  `json:"hostAliases,omitempty"`
	HostIPC                       bool                         `json:"hostIPC,omitempty"`
	HostNetwork                   bool                         `json:"hostNetwork,omitempty"`
	HostPID                       bool                         `json:"hostPID,omitempty"`
	Hostname                      string                       `json:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference       `json:"imagePullSecrets,omitempty"`
	NodeId                        string                       `json:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta                  `json:"metadata,omitempty"`
	Priority                      *int64                       `json:"priority,omitempty"`
	PriorityClassName             string                       `json:"priorityClassName,omitempty"`
	ReplicationController         *ReplicationControllerConfig `json:"replicationController,omitempty"`
	RestartPolicy                 string                       `json:"restartPolicy,omitempty"`
	RunAsNonRoot                  *bool                        `json:"runAsNonRoot,omitempty"`
	SchedulerName                 string                       `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling                  `json:"scheduling,omitempty"`
	Selector                      map[string]string            `json:"selector,omitempty"`
	ServiceAccountName            string                       `json:"serviceAccountName,omitempty"`
	Subdomain                     string                       `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                       `json:"terminationGracePeriodSeconds,omitempty"`
	Uid                           *int64                       `json:"uid,omitempty"`
	Volumes                       []Volume                     `json:"volumes,omitempty"`
}
