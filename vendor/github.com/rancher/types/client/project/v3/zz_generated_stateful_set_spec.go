package client

const (
	StatefulSetSpecType                               = "statefulSetSpec"
	StatefulSetSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	StatefulSetSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	StatefulSetSpecFieldContainers                    = "containers"
	StatefulSetSpecFieldDNSPolicy                     = "dnsPolicy"
	StatefulSetSpecFieldFsgid                         = "fsgid"
	StatefulSetSpecFieldGids                          = "gids"
	StatefulSetSpecFieldHostAliases                   = "hostAliases"
	StatefulSetSpecFieldHostIPC                       = "hostIPC"
	StatefulSetSpecFieldHostNetwork                   = "hostNetwork"
	StatefulSetSpecFieldHostPID                       = "hostPID"
	StatefulSetSpecFieldHostname                      = "hostname"
	StatefulSetSpecFieldImagePullSecrets              = "imagePullSecrets"
	StatefulSetSpecFieldNodeId                        = "nodeId"
	StatefulSetSpecFieldObjectMeta                    = "metadata"
	StatefulSetSpecFieldPriority                      = "priority"
	StatefulSetSpecFieldPriorityClassName             = "priorityClassName"
	StatefulSetSpecFieldRestartPolicy                 = "restartPolicy"
	StatefulSetSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	StatefulSetSpecFieldSchedulerName                 = "schedulerName"
	StatefulSetSpecFieldScheduling                    = "scheduling"
	StatefulSetSpecFieldSelector                      = "selector"
	StatefulSetSpecFieldServiceAccountName            = "serviceAccountName"
	StatefulSetSpecFieldStatefulSet                   = "statefulSet"
	StatefulSetSpecFieldSubdomain                     = "subdomain"
	StatefulSetSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	StatefulSetSpecFieldUid                           = "uid"
	StatefulSetSpecFieldVolumes                       = "volumes"
)

type StatefulSetSpec struct {
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	Fsgid                         *int64                 `json:"fsgid,omitempty"`
	Gids                          []int64                `json:"gids,omitempty"`
	HostAliases                   []HostAlias            `json:"hostAliases,omitempty"`
	HostIPC                       bool                   `json:"hostIPC,omitempty"`
	HostNetwork                   bool                   `json:"hostNetwork,omitempty"`
	HostPID                       bool                   `json:"hostPID,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference `json:"imagePullSecrets,omitempty"`
	NodeId                        string                 `json:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta            `json:"metadata,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty"`
	RestartPolicy                 string                 `json:"restartPolicy,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty"`
	Selector                      *LabelSelector         `json:"selector,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty"`
	StatefulSet                   *StatefulSetConfig     `json:"statefulSet,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Volumes                       []Volume               `json:"volumes,omitempty"`
}
