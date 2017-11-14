package client

const (
	ReplicationControllerSpecType                               = "replicationControllerSpec"
	ReplicationControllerSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	ReplicationControllerSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	ReplicationControllerSpecFieldBatchSize                     = "batchSize"
	ReplicationControllerSpecFieldContainers                    = "containers"
	ReplicationControllerSpecFieldDNSPolicy                     = "dnsPolicy"
	ReplicationControllerSpecFieldDeploymentStrategy            = "deploymentStrategy"
	ReplicationControllerSpecFieldFsgid                         = "fsgid"
	ReplicationControllerSpecFieldGids                          = "gids"
	ReplicationControllerSpecFieldHostAliases                   = "hostAliases"
	ReplicationControllerSpecFieldHostname                      = "hostname"
	ReplicationControllerSpecFieldIPC                           = "ipc"
	ReplicationControllerSpecFieldNet                           = "net"
	ReplicationControllerSpecFieldNodeId                        = "nodeId"
	ReplicationControllerSpecFieldObjectMeta                    = "metadata"
	ReplicationControllerSpecFieldPID                           = "pid"
	ReplicationControllerSpecFieldPriority                      = "priority"
	ReplicationControllerSpecFieldPriorityClassName             = "priorityClassName"
	ReplicationControllerSpecFieldPullSecrets                   = "pullSecrets"
	ReplicationControllerSpecFieldRestart                       = "restart"
	ReplicationControllerSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	ReplicationControllerSpecFieldScale                         = "scale"
	ReplicationControllerSpecFieldSchedulerName                 = "schedulerName"
	ReplicationControllerSpecFieldServiceAccountName            = "serviceAccountName"
	ReplicationControllerSpecFieldSubdomain                     = "subdomain"
	ReplicationControllerSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	ReplicationControllerSpecFieldTolerations                   = "tolerations"
	ReplicationControllerSpecFieldUid                           = "uid"
	ReplicationControllerSpecFieldVolumes                       = "volumes"
)

type ReplicationControllerSpec struct {
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
