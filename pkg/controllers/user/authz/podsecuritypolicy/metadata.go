package podsecuritypolicy

const (
	apiGroup                            = "rbac.authorization.k8s.io"
	apiVersion                          = "extensions/v1beta1"
	podSecurityTemplateParentAnnotation = "serviceaccount.cluster.cattle.io/pod-security"
	podSecurityVersionAnnotation        = "serviceaccount.cluster.cattle.io/pod-security-version"
	projectIDAnnotation                 = "field.cattle.io/projectId"
	podSecurityPolicy                   = "PodSecurityPolicy"
)
