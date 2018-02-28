package client

const (
	DaemonSetSpecType                               = "daemonSetSpec"
	DaemonSetSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	DaemonSetSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	DaemonSetSpecFieldContainers                    = "containers"
	DaemonSetSpecFieldDNSPolicy                     = "dnsPolicy"
	DaemonSetSpecFieldDaemonSetConfig               = "daemonSetConfig"
	DaemonSetSpecFieldFsgid                         = "fsgid"
	DaemonSetSpecFieldGids                          = "gids"
	DaemonSetSpecFieldHostAliases                   = "hostAliases"
	DaemonSetSpecFieldHostIPC                       = "hostIPC"
	DaemonSetSpecFieldHostNetwork                   = "hostNetwork"
	DaemonSetSpecFieldHostPID                       = "hostPID"
	DaemonSetSpecFieldHostname                      = "hostname"
	DaemonSetSpecFieldImagePullSecrets              = "imagePullSecrets"
	DaemonSetSpecFieldNodeId                        = "nodeId"
	DaemonSetSpecFieldObjectMeta                    = "metadata"
	DaemonSetSpecFieldPriority                      = "priority"
	DaemonSetSpecFieldPriorityClassName             = "priorityClassName"
	DaemonSetSpecFieldRestartPolicy                 = "restartPolicy"
	DaemonSetSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	DaemonSetSpecFieldSchedulerName                 = "schedulerName"
	DaemonSetSpecFieldScheduling                    = "scheduling"
	DaemonSetSpecFieldSelector                      = "selector"
	DaemonSetSpecFieldServiceAccountName            = "serviceAccountName"
	DaemonSetSpecFieldSubdomain                     = "subdomain"
	DaemonSetSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	DaemonSetSpecFieldUid                           = "uid"
	DaemonSetSpecFieldVolumes                       = "volumes"
)

type DaemonSetSpec struct {
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty" yaml:"containers,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty" yaml:"dnsPolicy,omitempty"`
	DaemonSetConfig               *DaemonSetConfig       `json:"daemonSetConfig,omitempty" yaml:"daemonSetConfig,omitempty"`
	Fsgid                         *int64                 `json:"fsgid,omitempty" yaml:"fsgid,omitempty"`
	Gids                          []int64                `json:"gids,omitempty" yaml:"gids,omitempty"`
	HostAliases                   []HostAlias            `json:"hostAliases,omitempty" yaml:"hostAliases,omitempty"`
	HostIPC                       bool                   `json:"hostIPC,omitempty" yaml:"hostIPC,omitempty"`
	HostNetwork                   bool                   `json:"hostNetwork,omitempty" yaml:"hostNetwork,omitempty"`
	HostPID                       bool                   `json:"hostPID,omitempty" yaml:"hostPID,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	NodeId                        string                 `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta            `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty" yaml:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	RestartPolicy                 string                 `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty" yaml:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	Selector                      *LabelSelector         `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty" yaml:"uid,omitempty"`
	Volumes                       []Volume               `json:"volumes,omitempty" yaml:"volumes,omitempty"`
}
