package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName = "project.cattle.io"
	Version   = "v3"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}

// Kind takes an unqualified kind and returns a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	// TODO this gets cleaned up when the types are fixed
	scheme.AddKnownTypes(SchemeGroupVersion,

		&ServiceAccountToken{},
		&ServiceAccountTokenList{},
		&DockerCredential{},
		&DockerCredentialList{},
		&Certificate{},
		&CertificateList{},
		&BasicAuth{},
		&BasicAuthList{},
		&SSHAuth{},
		&SSHAuthList{},
		&NamespacedServiceAccountToken{},
		&NamespacedServiceAccountTokenList{},
		&NamespacedDockerCredential{},
		&NamespacedDockerCredentialList{},
		&NamespacedCertificate{},
		&NamespacedCertificateList{},
		&NamespacedBasicAuth{},
		&NamespacedBasicAuthList{},
		&NamespacedSSHAuth{},
		&NamespacedSSHAuthList{},
		&Workload{},
		&WorkloadList{},
		&App{},
		&AppList{},
		&AppRevision{},
		&AppRevisionList{},
		&SourceCodeProvider{},
		&SourceCodeProviderList{},
		&SourceCodeProviderConfig{},
		&SourceCodeProviderConfigList{},
		&SourceCodeCredential{},
		&SourceCodeCredentialList{},
		&Pipeline{},
		&PipelineList{},
		&PipelineExecution{},
		&PipelineExecutionList{},
		&PipelineSetting{},
		&PipelineSettingList{},
		&SourceCodeRepository{},
		&SourceCodeRepositoryList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
