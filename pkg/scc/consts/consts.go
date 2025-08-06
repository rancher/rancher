package consts

const (
	DefaultSCCNamespace = "cattle-scc-system"
)

// Secret names and name prefixes
const (
	DeploymentName         = "rancher-scc-operator"
	PodNamePrefix          = "rancher-scc-operator-"
	ServiceAccountName     = DeploymentName + "-sa"
	ClusterRoleName        = "cluster-admin"
	ClusterRoleBindingName = DeploymentName + "-crb"
)

const (
	FinalizerSccOperatorDeployment = "scc.cattle.io/scc-operator-deployment"
	FinalizerSccOperatorPod        = "scc.cattle.io/scc-operator-pod"
)

const (
	// LabelSccOperatorHash is used to determine if the deployment needs to be updated
	LabelSccOperatorHash = "scc.cattle.io/operator-config-hash"
	LabelK8sManagedBy    = "app.kubernetes.io/managed-by"
	LabelK8sPartOf       = "app.kubernetes.io/part-of"
	LabelK8sInstance     = "app.kubernetes.io/instance"
	LabelK8sName         = "app.kubernetes.io/name"
	LabelK8sComponent    = "app.kubernetes.io/component"
)
