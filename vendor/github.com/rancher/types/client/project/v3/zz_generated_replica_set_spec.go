package client

const (
	ReplicaSetSpecType                               = "replicaSetSpec"
	ReplicaSetSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	ReplicaSetSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	ReplicaSetSpecFieldBatchSize                     = "batchSize"
	ReplicaSetSpecFieldContainers                    = "containers"
	ReplicaSetSpecFieldDNSPolicy                     = "dnsPolicy"
	ReplicaSetSpecFieldDeploymentStrategy            = "deploymentStrategy"
	ReplicaSetSpecFieldFsgid                         = "fsgid"
	ReplicaSetSpecFieldGids                          = "gids"
	ReplicaSetSpecFieldHostAliases                   = "hostAliases"
	ReplicaSetSpecFieldHostname                      = "hostname"
	ReplicaSetSpecFieldIPC                           = "ipc"
	ReplicaSetSpecFieldNet                           = "net"
	ReplicaSetSpecFieldNodeId                        = "nodeId"
	ReplicaSetSpecFieldObjectMeta                    = "metadata"
	ReplicaSetSpecFieldPID                           = "pid"
	ReplicaSetSpecFieldPriority                      = "priority"
	ReplicaSetSpecFieldPriorityClassName             = "priorityClassName"
	ReplicaSetSpecFieldPullSecrets                   = "pullSecrets"
	ReplicaSetSpecFieldRestart                       = "restart"
	ReplicaSetSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	ReplicaSetSpecFieldScale                         = "scale"
	ReplicaSetSpecFieldSchedulerName                 = "schedulerName"
	ReplicaSetSpecFieldServiceAccountName            = "serviceAccountName"
	ReplicaSetSpecFieldSubdomain                     = "subdomain"
	ReplicaSetSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	ReplicaSetSpecFieldTolerations                   = "tolerations"
	ReplicaSetSpecFieldUid                           = "uid"
	ReplicaSetSpecFieldVolumes                       = "volumes"
)

type ReplicaSetSpec struct {
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	BatchSize                     string                 `json:"batchSize,omitempty"`
	Containers                    map[string]Container   `json:"containers,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	DeploymentStrategy            *DeployStrategy        `json:"deploymentStrategy,omitempty"`
	Fsgid                         *int64                 `json:"fsgid,omitempty"`
	Gids                          []int64                `json:"gids,omitempty"`
	HostAliases                   map[string]HostAlias   `json:"hostAliases,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty"`
	IPC                           string                 `json:"ipc,omitempty"`
	Net                           string                 `json:"net,omitempty"`
	NodeId                        string                 `json:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta            `json:"metadata,omitempty"`
	PID                           string                 `json:"pid,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty"`
	PullSecrets                   []LocalObjectReference `json:"pullSecrets,omitempty"`
	Restart                       string                 `json:"restart,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty"`
	Scale                         *int64                 `json:"scale,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Tolerations                   []Toleration           `json:"tolerations,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Volumes                       map[string]Volume      `json:"volumes,omitempty"`
}
