package client

const (
	DaemonSetSpecType                               = "daemonSetSpec"
	DaemonSetSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	DaemonSetSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	DaemonSetSpecFieldBatchSize                     = "batchSize"
	DaemonSetSpecFieldContainers                    = "containers"
	DaemonSetSpecFieldDNSPolicy                     = "dnsPolicy"
	DaemonSetSpecFieldDeploymentStrategy            = "deploymentStrategy"
	DaemonSetSpecFieldFsgid                         = "fsgid"
	DaemonSetSpecFieldGids                          = "gids"
	DaemonSetSpecFieldHostAliases                   = "hostAliases"
	DaemonSetSpecFieldHostname                      = "hostname"
	DaemonSetSpecFieldIPC                           = "ipc"
	DaemonSetSpecFieldNet                           = "net"
	DaemonSetSpecFieldNodeId                        = "nodeId"
	DaemonSetSpecFieldObjectMeta                    = "metadata"
	DaemonSetSpecFieldPID                           = "pid"
	DaemonSetSpecFieldPriority                      = "priority"
	DaemonSetSpecFieldPriorityClassName             = "priorityClassName"
	DaemonSetSpecFieldPullSecrets                   = "pullSecrets"
	DaemonSetSpecFieldRestart                       = "restart"
	DaemonSetSpecFieldRevisionHistoryLimit          = "revisionHistoryLimit"
	DaemonSetSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	DaemonSetSpecFieldScale                         = "scale"
	DaemonSetSpecFieldSchedulerName                 = "schedulerName"
	DaemonSetSpecFieldScheduling                    = "scheduling"
	DaemonSetSpecFieldServiceAccountName            = "serviceAccountName"
	DaemonSetSpecFieldSubdomain                     = "subdomain"
	DaemonSetSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	DaemonSetSpecFieldUid                           = "uid"
	DaemonSetSpecFieldUpdateStrategy                = "updateStrategy"
	DaemonSetSpecFieldVolumes                       = "volumes"
)

type DaemonSetSpec struct {
	ActiveDeadlineSeconds         *int64                   `json:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                    `json:"automountServiceAccountToken,omitempty"`
	BatchSize                     string                   `json:"batchSize,omitempty"`
	Containers                    []Container              `json:"containers,omitempty"`
	DNSPolicy                     string                   `json:"dnsPolicy,omitempty"`
	DeploymentStrategy            *DeployStrategy          `json:"deploymentStrategy,omitempty"`
	Fsgid                         *int64                   `json:"fsgid,omitempty"`
	Gids                          []int64                  `json:"gids,omitempty"`
	HostAliases                   map[string]HostAlias     `json:"hostAliases,omitempty"`
	Hostname                      string                   `json:"hostname,omitempty"`
	IPC                           string                   `json:"ipc,omitempty"`
	Net                           string                   `json:"net,omitempty"`
	NodeId                        string                   `json:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta              `json:"metadata,omitempty"`
	PID                           string                   `json:"pid,omitempty"`
	Priority                      *int64                   `json:"priority,omitempty"`
	PriorityClassName             string                   `json:"priorityClassName,omitempty"`
	PullSecrets                   []LocalObjectReference   `json:"pullSecrets,omitempty"`
	Restart                       string                   `json:"restart,omitempty"`
	RevisionHistoryLimit          *int64                   `json:"revisionHistoryLimit,omitempty"`
	RunAsNonRoot                  *bool                    `json:"runAsNonRoot,omitempty"`
	Scale                         *int64                   `json:"scale,omitempty"`
	SchedulerName                 string                   `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling              `json:"scheduling,omitempty"`
	ServiceAccountName            string                   `json:"serviceAccountName,omitempty"`
	Subdomain                     string                   `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                   `json:"terminationGracePeriodSeconds,omitempty"`
	Uid                           *int64                   `json:"uid,omitempty"`
	UpdateStrategy                *DaemonSetUpdateStrategy `json:"updateStrategy,omitempty"`
	Volumes                       map[string]Volume        `json:"volumes,omitempty"`
}
