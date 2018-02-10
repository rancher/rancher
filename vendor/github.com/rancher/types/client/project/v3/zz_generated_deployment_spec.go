package client

const (
	DeploymentSpecType                               = "deploymentSpec"
	DeploymentSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	DeploymentSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	DeploymentSpecFieldContainers                    = "containers"
	DeploymentSpecFieldDNSPolicy                     = "dnsPolicy"
	DeploymentSpecFieldDeployment                    = "deployment"
	DeploymentSpecFieldFsgid                         = "fsgid"
	DeploymentSpecFieldGids                          = "gids"
	DeploymentSpecFieldHostAliases                   = "hostAliases"
	DeploymentSpecFieldHostIPC                       = "hostIPC"
	DeploymentSpecFieldHostNetwork                   = "hostNetwork"
	DeploymentSpecFieldHostPID                       = "hostPID"
	DeploymentSpecFieldHostname                      = "hostname"
	DeploymentSpecFieldImagePullSecrets              = "imagePullSecrets"
	DeploymentSpecFieldNodeId                        = "nodeId"
	DeploymentSpecFieldObjectMeta                    = "metadata"
	DeploymentSpecFieldPriority                      = "priority"
	DeploymentSpecFieldPriorityClassName             = "priorityClassName"
	DeploymentSpecFieldRestartPolicy                 = "restartPolicy"
	DeploymentSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	DeploymentSpecFieldScale                         = "scale"
	DeploymentSpecFieldSchedulerName                 = "schedulerName"
	DeploymentSpecFieldScheduling                    = "scheduling"
	DeploymentSpecFieldSelector                      = "selector"
	DeploymentSpecFieldServiceAccountName            = "serviceAccountName"
	DeploymentSpecFieldSubdomain                     = "subdomain"
	DeploymentSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	DeploymentSpecFieldUid                           = "uid"
	DeploymentSpecFieldVolumes                       = "volumes"
)

type DeploymentSpec struct {
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	Deployment                    *DeploymentConfig      `json:"deployment,omitempty"`
	Fsgid                         *int64                 `json:"fsgid,omitempty"`
	Gids                          []int64                `json:"gids,omitempty"`
	HostAliases                   []HostAlias            `json:"hostAliases,omitempty"`
	HostIPC                       *bool                  `json:"hostIPC,omitempty"`
	HostNetwork                   *bool                  `json:"hostNetwork,omitempty"`
	HostPID                       *bool                  `json:"hostPID,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference `json:"imagePullSecrets,omitempty"`
	NodeId                        string                 `json:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta            `json:"metadata,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty"`
	RestartPolicy                 string                 `json:"restartPolicy,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty"`
	Scale                         *int64                 `json:"scale,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty"`
	Selector                      *LabelSelector         `json:"selector,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Volumes                       []Volume               `json:"volumes,omitempty"`
}
