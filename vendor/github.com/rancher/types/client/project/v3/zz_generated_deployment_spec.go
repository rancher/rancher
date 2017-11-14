package client

const (
	DeploymentSpecType                               = "deploymentSpec"
	DeploymentSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	DeploymentSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	DeploymentSpecFieldBatchSize                     = "batchSize"
	DeploymentSpecFieldContainers                    = "containers"
	DeploymentSpecFieldDNSPolicy                     = "dnsPolicy"
	DeploymentSpecFieldDeploymentStrategy            = "deploymentStrategy"
	DeploymentSpecFieldFsgid                         = "fsgid"
	DeploymentSpecFieldGids                          = "gids"
	DeploymentSpecFieldHostAliases                   = "hostAliases"
	DeploymentSpecFieldHostname                      = "hostname"
	DeploymentSpecFieldIPC                           = "ipc"
	DeploymentSpecFieldNet                           = "net"
	DeploymentSpecFieldNodeId                        = "nodeId"
	DeploymentSpecFieldObjectMeta                    = "metadata"
	DeploymentSpecFieldPID                           = "pid"
	DeploymentSpecFieldPaused                        = "paused"
	DeploymentSpecFieldPriority                      = "priority"
	DeploymentSpecFieldPriorityClassName             = "priorityClassName"
	DeploymentSpecFieldPullSecrets                   = "pullSecrets"
	DeploymentSpecFieldRestart                       = "restart"
	DeploymentSpecFieldRevisionHistoryLimit          = "revisionHistoryLimit"
	DeploymentSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	DeploymentSpecFieldScale                         = "scale"
	DeploymentSpecFieldSchedulerName                 = "schedulerName"
	DeploymentSpecFieldServiceAccountName            = "serviceAccountName"
	DeploymentSpecFieldSubdomain                     = "subdomain"
	DeploymentSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	DeploymentSpecFieldTolerations                   = "tolerations"
	DeploymentSpecFieldUid                           = "uid"
	DeploymentSpecFieldVolumes                       = "volumes"
)

type DeploymentSpec struct {
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
	Paused                        *bool                  `json:"paused,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty"`
	PullSecrets                   []LocalObjectReference `json:"pullSecrets,omitempty"`
	Restart                       string                 `json:"restart,omitempty"`
	RevisionHistoryLimit          *int64                 `json:"revisionHistoryLimit,omitempty"`
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
