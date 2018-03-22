package podsecuritypolicy

const (
	apiGroup                                   = "rbac.authorization.k8s.io"
	apiVersion                                 = "extensions/v1beta1"
	podSecurityPolicyTemplateParentAnnotation  = "serviceaccount.cluster.cattle.io/pod-security"
	podSecurityPolicyTemplateVersionAnnotation = "serviceaccount.cluster.cattle.io/pod-security-version"
	projectIDAnnotation                        = "field.cattle.io/projectId"
	podSecurityPolicy                          = "PodSecurityPolicy"
)
