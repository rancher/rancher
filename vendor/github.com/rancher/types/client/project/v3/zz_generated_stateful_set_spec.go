package client

const (
	StatefulSetSpecType                               = "statefulSetSpec"
	StatefulSetSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	StatefulSetSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	StatefulSetSpecFieldBatchSize                     = "batchSize"
	StatefulSetSpecFieldContainers                    = "containers"
	StatefulSetSpecFieldDNSPolicy                     = "dnsPolicy"
	StatefulSetSpecFieldDeploymentStrategy            = "deploymentStrategy"
	StatefulSetSpecFieldFsgid                         = "fsgid"
	StatefulSetSpecFieldGids                          = "gids"
	StatefulSetSpecFieldHostAliases                   = "hostAliases"
	StatefulSetSpecFieldHostname                      = "hostname"
	StatefulSetSpecFieldIPC                           = "ipc"
	StatefulSetSpecFieldNet                           = "net"
	StatefulSetSpecFieldNodeId                        = "nodeId"
	StatefulSetSpecFieldObjectMeta                    = "metadata"
	StatefulSetSpecFieldPID                           = "pid"
	StatefulSetSpecFieldPodManagementPolicy           = "podManagementPolicy"
	StatefulSetSpecFieldPriority                      = "priority"
	StatefulSetSpecFieldPriorityClassName             = "priorityClassName"
	StatefulSetSpecFieldPullSecrets                   = "pullSecrets"
	StatefulSetSpecFieldRestart                       = "restart"
	StatefulSetSpecFieldRevisionHistoryLimit          = "revisionHistoryLimit"
	StatefulSetSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	StatefulSetSpecFieldScale                         = "scale"
	StatefulSetSpecFieldSchedulerName                 = "schedulerName"
	StatefulSetSpecFieldScheduling                    = "scheduling"
	StatefulSetSpecFieldServiceAccountName            = "serviceAccountName"
	StatefulSetSpecFieldServiceName                   = "serviceName"
	StatefulSetSpecFieldSubdomain                     = "subdomain"
	StatefulSetSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	StatefulSetSpecFieldTolerations                   = "tolerations"
	StatefulSetSpecFieldUid                           = "uid"
	StatefulSetSpecFieldUpdateStrategy                = "updateStrategy"
	StatefulSetSpecFieldVolumeClaimTemplates          = "volumeClaimTemplates"
	StatefulSetSpecFieldVolumes                       = "volumes"
)

type StatefulSetSpec struct {
	ActiveDeadlineSeconds         *int64                     `json:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                      `json:"automountServiceAccountToken,omitempty"`
	BatchSize                     string                     `json:"batchSize,omitempty"`
	Containers                    []Container                `json:"containers,omitempty"`
	DNSPolicy                     string                     `json:"dnsPolicy,omitempty"`
	DeploymentStrategy            *DeployStrategy            `json:"deploymentStrategy,omitempty"`
	Fsgid                         *int64                     `json:"fsgid,omitempty"`
	Gids                          []int64                    `json:"gids,omitempty"`
	HostAliases                   map[string]HostAlias       `json:"hostAliases,omitempty"`
	Hostname                      string                     `json:"hostname,omitempty"`
	IPC                           string                     `json:"ipc,omitempty"`
	Net                           string                     `json:"net,omitempty"`
	NodeId                        string                     `json:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta                `json:"metadata,omitempty"`
	PID                           string                     `json:"pid,omitempty"`
	PodManagementPolicy           string                     `json:"podManagementPolicy,omitempty"`
	Priority                      *int64                     `json:"priority,omitempty"`
	PriorityClassName             string                     `json:"priorityClassName,omitempty"`
	PullSecrets                   []LocalObjectReference     `json:"pullSecrets,omitempty"`
	Restart                       string                     `json:"restart,omitempty"`
	RevisionHistoryLimit          *int64                     `json:"revisionHistoryLimit,omitempty"`
	RunAsNonRoot                  *bool                      `json:"runAsNonRoot,omitempty"`
	Scale                         *int64                     `json:"scale,omitempty"`
	SchedulerName                 string                     `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling                `json:"scheduling,omitempty"`
	ServiceAccountName            string                     `json:"serviceAccountName,omitempty"`
	ServiceName                   string                     `json:"serviceName,omitempty"`
	Subdomain                     string                     `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                     `json:"terminationGracePeriodSeconds,omitempty"`
	Tolerations                   []Toleration               `json:"tolerations,omitempty"`
	Uid                           *int64                     `json:"uid,omitempty"`
	UpdateStrategy                *StatefulSetUpdateStrategy `json:"updateStrategy,omitempty"`
	VolumeClaimTemplates          []PersistentVolumeClaim    `json:"volumeClaimTemplates,omitempty"`
	Volumes                       map[string]Volume          `json:"volumes,omitempty"`
}
