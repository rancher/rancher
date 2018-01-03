package client

const (
	WorkloadSpecType                               = "workloadSpec"
	WorkloadSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	WorkloadSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	WorkloadSpecFieldBatchSize                     = "batchSize"
	WorkloadSpecFieldContainers                    = "containers"
	WorkloadSpecFieldDNSPolicy                     = "dnsPolicy"
	WorkloadSpecFieldDeploymentStrategy            = "deploymentStrategy"
	WorkloadSpecFieldDescription                   = "description"
	WorkloadSpecFieldFsgid                         = "fsgid"
	WorkloadSpecFieldGids                          = "gids"
	WorkloadSpecFieldHostAliases                   = "hostAliases"
	WorkloadSpecFieldHostname                      = "hostname"
	WorkloadSpecFieldIPC                           = "ipc"
	WorkloadSpecFieldNet                           = "net"
	WorkloadSpecFieldNodeId                        = "nodeId"
	WorkloadSpecFieldObjectMeta                    = "metadata"
	WorkloadSpecFieldPID                           = "pid"
	WorkloadSpecFieldPriority                      = "priority"
	WorkloadSpecFieldPriorityClassName             = "priorityClassName"
	WorkloadSpecFieldPullSecrets                   = "pullSecrets"
	WorkloadSpecFieldRestart                       = "restart"
	WorkloadSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	WorkloadSpecFieldScale                         = "scale"
	WorkloadSpecFieldSchedulerName                 = "schedulerName"
	WorkloadSpecFieldScheduling                    = "scheduling"
	WorkloadSpecFieldServiceAccountName            = "serviceAccountName"
	WorkloadSpecFieldServiceLinks                  = "serviceLinks"
	WorkloadSpecFieldSubdomain                     = "subdomain"
	WorkloadSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	WorkloadSpecFieldUid                           = "uid"
	WorkloadSpecFieldVolumes                       = "volumes"
)

type WorkloadSpec struct {
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	BatchSize                     string                 `json:"batchSize,omitempty"`
	Containers                    []Container            `json:"containers,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	DeploymentStrategy            *DeployStrategy        `json:"deploymentStrategy,omitempty"`
	Description                   string                 `json:"description,omitempty"`
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
	Scheduling                    *Scheduling            `json:"scheduling,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty"`
	ServiceLinks                  []Link                 `json:"serviceLinks,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Volumes                       map[string]Volume      `json:"volumes,omitempty"`
}
