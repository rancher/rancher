package v3

import (
	reflect "reflect"

	v1 "k8s.io/api/core/v1"
	rbac_v1 "k8s.io/api/rbac/v1"
	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func init() {
	SchemeBuilder.Register(RegisterDeepCopies)
}

// RegisterDeepCopies adds deep-copy functions to the given scheme. Public
// to allow building arbitrary schemes.
//
// Deprecated: deepcopy registration will go away when static deepcopy is fully implemented.
func RegisterDeepCopies(scheme *runtime.Scheme) error {
	return scheme.AddGeneratedDeepCopyFuncs(
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Action).DeepCopyInto(out.(*Action))
			return nil
		}, InType: reflect.TypeOf(&Action{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ActiveDirectoryConfig).DeepCopyInto(out.(*ActiveDirectoryConfig))
			return nil
		}, InType: reflect.TypeOf(&ActiveDirectoryConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ActiveDirectoryTestAndApplyInput).DeepCopyInto(out.(*ActiveDirectoryTestAndApplyInput))
			return nil
		}, InType: reflect.TypeOf(&ActiveDirectoryTestAndApplyInput{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*AlertCommonSpec).DeepCopyInto(out.(*AlertCommonSpec))
			return nil
		}, InType: reflect.TypeOf(&AlertCommonSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*AlertStatus).DeepCopyInto(out.(*AlertStatus))
			return nil
		}, InType: reflect.TypeOf(&AlertStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*AuthAppInput).DeepCopyInto(out.(*AuthAppInput))
			return nil
		}, InType: reflect.TypeOf(&AuthAppInput{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*AuthConfig).DeepCopyInto(out.(*AuthConfig))
			return nil
		}, InType: reflect.TypeOf(&AuthConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*AuthConfigList).DeepCopyInto(out.(*AuthConfigList))
			return nil
		}, InType: reflect.TypeOf(&AuthConfigList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*AuthUserInput).DeepCopyInto(out.(*AuthUserInput))
			return nil
		}, InType: reflect.TypeOf(&AuthUserInput{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*AuthnConfig).DeepCopyInto(out.(*AuthnConfig))
			return nil
		}, InType: reflect.TypeOf(&AuthnConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*AuthzConfig).DeepCopyInto(out.(*AuthzConfig))
			return nil
		}, InType: reflect.TypeOf(&AuthzConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*AzureKubernetesServiceConfig).DeepCopyInto(out.(*AzureKubernetesServiceConfig))
			return nil
		}, InType: reflect.TypeOf(&AzureKubernetesServiceConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*BaseService).DeepCopyInto(out.(*BaseService))
			return nil
		}, InType: reflect.TypeOf(&BaseService{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Catalog).DeepCopyInto(out.(*Catalog))
			return nil
		}, InType: reflect.TypeOf(&Catalog{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CatalogCondition).DeepCopyInto(out.(*CatalogCondition))
			return nil
		}, InType: reflect.TypeOf(&CatalogCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CatalogList).DeepCopyInto(out.(*CatalogList))
			return nil
		}, InType: reflect.TypeOf(&CatalogList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CatalogSpec).DeepCopyInto(out.(*CatalogSpec))
			return nil
		}, InType: reflect.TypeOf(&CatalogSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CatalogStatus).DeepCopyInto(out.(*CatalogStatus))
			return nil
		}, InType: reflect.TypeOf(&CatalogStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ChangePasswordInput).DeepCopyInto(out.(*ChangePasswordInput))
			return nil
		}, InType: reflect.TypeOf(&ChangePasswordInput{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Cluster).DeepCopyInto(out.(*Cluster))
			return nil
		}, InType: reflect.TypeOf(&Cluster{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterAlert).DeepCopyInto(out.(*ClusterAlert))
			return nil
		}, InType: reflect.TypeOf(&ClusterAlert{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterAlertList).DeepCopyInto(out.(*ClusterAlertList))
			return nil
		}, InType: reflect.TypeOf(&ClusterAlertList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterAlertSpec).DeepCopyInto(out.(*ClusterAlertSpec))
			return nil
		}, InType: reflect.TypeOf(&ClusterAlertSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterComponentStatus).DeepCopyInto(out.(*ClusterComponentStatus))
			return nil
		}, InType: reflect.TypeOf(&ClusterComponentStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterComposeConfig).DeepCopyInto(out.(*ClusterComposeConfig))
			return nil
		}, InType: reflect.TypeOf(&ClusterComposeConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterComposeConfigList).DeepCopyInto(out.(*ClusterComposeConfigList))
			return nil
		}, InType: reflect.TypeOf(&ClusterComposeConfigList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterComposeSpec).DeepCopyInto(out.(*ClusterComposeSpec))
			return nil
		}, InType: reflect.TypeOf(&ClusterComposeSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterCondition).DeepCopyInto(out.(*ClusterCondition))
			return nil
		}, InType: reflect.TypeOf(&ClusterCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterEvent).DeepCopyInto(out.(*ClusterEvent))
			return nil
		}, InType: reflect.TypeOf(&ClusterEvent{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterEventList).DeepCopyInto(out.(*ClusterEventList))
			return nil
		}, InType: reflect.TypeOf(&ClusterEventList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterList).DeepCopyInto(out.(*ClusterList))
			return nil
		}, InType: reflect.TypeOf(&ClusterList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterLogging).DeepCopyInto(out.(*ClusterLogging))
			return nil
		}, InType: reflect.TypeOf(&ClusterLogging{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterLoggingList).DeepCopyInto(out.(*ClusterLoggingList))
			return nil
		}, InType: reflect.TypeOf(&ClusterLoggingList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterLoggingSpec).DeepCopyInto(out.(*ClusterLoggingSpec))
			return nil
		}, InType: reflect.TypeOf(&ClusterLoggingSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterPipeline).DeepCopyInto(out.(*ClusterPipeline))
			return nil
		}, InType: reflect.TypeOf(&ClusterPipeline{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterPipelineList).DeepCopyInto(out.(*ClusterPipelineList))
			return nil
		}, InType: reflect.TypeOf(&ClusterPipelineList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterPipelineSpec).DeepCopyInto(out.(*ClusterPipelineSpec))
			return nil
		}, InType: reflect.TypeOf(&ClusterPipelineSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterPipelineStatus).DeepCopyInto(out.(*ClusterPipelineStatus))
			return nil
		}, InType: reflect.TypeOf(&ClusterPipelineStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterRegistrationToken).DeepCopyInto(out.(*ClusterRegistrationToken))
			return nil
		}, InType: reflect.TypeOf(&ClusterRegistrationToken{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterRegistrationTokenList).DeepCopyInto(out.(*ClusterRegistrationTokenList))
			return nil
		}, InType: reflect.TypeOf(&ClusterRegistrationTokenList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterRegistrationTokenSpec).DeepCopyInto(out.(*ClusterRegistrationTokenSpec))
			return nil
		}, InType: reflect.TypeOf(&ClusterRegistrationTokenSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterRegistrationTokenStatus).DeepCopyInto(out.(*ClusterRegistrationTokenStatus))
			return nil
		}, InType: reflect.TypeOf(&ClusterRegistrationTokenStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterRoleTemplateBinding).DeepCopyInto(out.(*ClusterRoleTemplateBinding))
			return nil
		}, InType: reflect.TypeOf(&ClusterRoleTemplateBinding{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterRoleTemplateBindingList).DeepCopyInto(out.(*ClusterRoleTemplateBindingList))
			return nil
		}, InType: reflect.TypeOf(&ClusterRoleTemplateBindingList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterSpec).DeepCopyInto(out.(*ClusterSpec))
			return nil
		}, InType: reflect.TypeOf(&ClusterSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ClusterStatus).DeepCopyInto(out.(*ClusterStatus))
			return nil
		}, InType: reflect.TypeOf(&ClusterStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ComposeCondition).DeepCopyInto(out.(*ComposeCondition))
			return nil
		}, InType: reflect.TypeOf(&ComposeCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ComposeSpec).DeepCopyInto(out.(*ComposeSpec))
			return nil
		}, InType: reflect.TypeOf(&ComposeSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ComposeStatus).DeepCopyInto(out.(*ComposeStatus))
			return nil
		}, InType: reflect.TypeOf(&ComposeStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Condition).DeepCopyInto(out.(*Condition))
			return nil
		}, InType: reflect.TypeOf(&Condition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*CustomConfig).DeepCopyInto(out.(*CustomConfig))
			return nil
		}, InType: reflect.TypeOf(&CustomConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*DynamicSchema).DeepCopyInto(out.(*DynamicSchema))
			return nil
		}, InType: reflect.TypeOf(&DynamicSchema{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*DynamicSchemaList).DeepCopyInto(out.(*DynamicSchemaList))
			return nil
		}, InType: reflect.TypeOf(&DynamicSchemaList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*DynamicSchemaSpec).DeepCopyInto(out.(*DynamicSchemaSpec))
			return nil
		}, InType: reflect.TypeOf(&DynamicSchemaSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*DynamicSchemaStatus).DeepCopyInto(out.(*DynamicSchemaStatus))
			return nil
		}, InType: reflect.TypeOf(&DynamicSchemaStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ETCDService).DeepCopyInto(out.(*ETCDService))
			return nil
		}, InType: reflect.TypeOf(&ETCDService{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ElasticsearchConfig).DeepCopyInto(out.(*ElasticsearchConfig))
			return nil
		}, InType: reflect.TypeOf(&ElasticsearchConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*EmbeddedConfig).DeepCopyInto(out.(*EmbeddedConfig))
			return nil
		}, InType: reflect.TypeOf(&EmbeddedConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Field).DeepCopyInto(out.(*Field))
			return nil
		}, InType: reflect.TypeOf(&Field{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*File).DeepCopyInto(out.(*File))
			return nil
		}, InType: reflect.TypeOf(&File{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Filter).DeepCopyInto(out.(*Filter))
			return nil
		}, InType: reflect.TypeOf(&Filter{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GithubClusterConfig).DeepCopyInto(out.(*GithubClusterConfig))
			return nil
		}, InType: reflect.TypeOf(&GithubClusterConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GithubConfig).DeepCopyInto(out.(*GithubConfig))
			return nil
		}, InType: reflect.TypeOf(&GithubConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GithubConfigApplyInput).DeepCopyInto(out.(*GithubConfigApplyInput))
			return nil
		}, InType: reflect.TypeOf(&GithubConfigApplyInput{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GithubConfigTestOutput).DeepCopyInto(out.(*GithubConfigTestOutput))
			return nil
		}, InType: reflect.TypeOf(&GithubConfigTestOutput{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GlobalComposeConfig).DeepCopyInto(out.(*GlobalComposeConfig))
			return nil
		}, InType: reflect.TypeOf(&GlobalComposeConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GlobalComposeConfigList).DeepCopyInto(out.(*GlobalComposeConfigList))
			return nil
		}, InType: reflect.TypeOf(&GlobalComposeConfigList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GlobalRole).DeepCopyInto(out.(*GlobalRole))
			return nil
		}, InType: reflect.TypeOf(&GlobalRole{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GlobalRoleBinding).DeepCopyInto(out.(*GlobalRoleBinding))
			return nil
		}, InType: reflect.TypeOf(&GlobalRoleBinding{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GlobalRoleBindingList).DeepCopyInto(out.(*GlobalRoleBindingList))
			return nil
		}, InType: reflect.TypeOf(&GlobalRoleBindingList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GlobalRoleList).DeepCopyInto(out.(*GlobalRoleList))
			return nil
		}, InType: reflect.TypeOf(&GlobalRoleList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GoogleKubernetesEngineConfig).DeepCopyInto(out.(*GoogleKubernetesEngineConfig))
			return nil
		}, InType: reflect.TypeOf(&GoogleKubernetesEngineConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Group).DeepCopyInto(out.(*Group))
			return nil
		}, InType: reflect.TypeOf(&Group{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GroupList).DeepCopyInto(out.(*GroupList))
			return nil
		}, InType: reflect.TypeOf(&GroupList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GroupMember).DeepCopyInto(out.(*GroupMember))
			return nil
		}, InType: reflect.TypeOf(&GroupMember{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*GroupMemberList).DeepCopyInto(out.(*GroupMemberList))
			return nil
		}, InType: reflect.TypeOf(&GroupMemberList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*HealthCheck).DeepCopyInto(out.(*HealthCheck))
			return nil
		}, InType: reflect.TypeOf(&HealthCheck{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ImportedConfig).DeepCopyInto(out.(*ImportedConfig))
			return nil
		}, InType: reflect.TypeOf(&ImportedConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*IngressConfig).DeepCopyInto(out.(*IngressConfig))
			return nil
		}, InType: reflect.TypeOf(&IngressConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*KafkaConfig).DeepCopyInto(out.(*KafkaConfig))
			return nil
		}, InType: reflect.TypeOf(&KafkaConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*KubeAPIService).DeepCopyInto(out.(*KubeAPIService))
			return nil
		}, InType: reflect.TypeOf(&KubeAPIService{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*KubeControllerService).DeepCopyInto(out.(*KubeControllerService))
			return nil
		}, InType: reflect.TypeOf(&KubeControllerService{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*KubeletService).DeepCopyInto(out.(*KubeletService))
			return nil
		}, InType: reflect.TypeOf(&KubeletService{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*KubeproxyService).DeepCopyInto(out.(*KubeproxyService))
			return nil
		}, InType: reflect.TypeOf(&KubeproxyService{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ListOpts).DeepCopyInto(out.(*ListOpts))
			return nil
		}, InType: reflect.TypeOf(&ListOpts{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ListenConfig).DeepCopyInto(out.(*ListenConfig))
			return nil
		}, InType: reflect.TypeOf(&ListenConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ListenConfigList).DeepCopyInto(out.(*ListenConfigList))
			return nil
		}, InType: reflect.TypeOf(&ListenConfigList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*LocalConfig).DeepCopyInto(out.(*LocalConfig))
			return nil
		}, InType: reflect.TypeOf(&LocalConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*LoggingCommonSpec).DeepCopyInto(out.(*LoggingCommonSpec))
			return nil
		}, InType: reflect.TypeOf(&LoggingCommonSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*LoggingCondition).DeepCopyInto(out.(*LoggingCondition))
			return nil
		}, InType: reflect.TypeOf(&LoggingCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*LoggingStatus).DeepCopyInto(out.(*LoggingStatus))
			return nil
		}, InType: reflect.TypeOf(&LoggingStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NetworkConfig).DeepCopyInto(out.(*NetworkConfig))
			return nil
		}, InType: reflect.TypeOf(&NetworkConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Node).DeepCopyInto(out.(*Node))
			return nil
		}, InType: reflect.TypeOf(&Node{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeCommonParams).DeepCopyInto(out.(*NodeCommonParams))
			return nil
		}, InType: reflect.TypeOf(&NodeCommonParams{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeCondition).DeepCopyInto(out.(*NodeCondition))
			return nil
		}, InType: reflect.TypeOf(&NodeCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeDriver).DeepCopyInto(out.(*NodeDriver))
			return nil
		}, InType: reflect.TypeOf(&NodeDriver{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeDriverList).DeepCopyInto(out.(*NodeDriverList))
			return nil
		}, InType: reflect.TypeOf(&NodeDriverList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeDriverSpec).DeepCopyInto(out.(*NodeDriverSpec))
			return nil
		}, InType: reflect.TypeOf(&NodeDriverSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeDriverStatus).DeepCopyInto(out.(*NodeDriverStatus))
			return nil
		}, InType: reflect.TypeOf(&NodeDriverStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeList).DeepCopyInto(out.(*NodeList))
			return nil
		}, InType: reflect.TypeOf(&NodeList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodePool).DeepCopyInto(out.(*NodePool))
			return nil
		}, InType: reflect.TypeOf(&NodePool{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodePoolList).DeepCopyInto(out.(*NodePoolList))
			return nil
		}, InType: reflect.TypeOf(&NodePoolList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodePoolSpec).DeepCopyInto(out.(*NodePoolSpec))
			return nil
		}, InType: reflect.TypeOf(&NodePoolSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodePoolStatus).DeepCopyInto(out.(*NodePoolStatus))
			return nil
		}, InType: reflect.TypeOf(&NodePoolStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeSpec).DeepCopyInto(out.(*NodeSpec))
			return nil
		}, InType: reflect.TypeOf(&NodeSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeStatus).DeepCopyInto(out.(*NodeStatus))
			return nil
		}, InType: reflect.TypeOf(&NodeStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeTemplate).DeepCopyInto(out.(*NodeTemplate))
			return nil
		}, InType: reflect.TypeOf(&NodeTemplate{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeTemplateCondition).DeepCopyInto(out.(*NodeTemplateCondition))
			return nil
		}, InType: reflect.TypeOf(&NodeTemplateCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeTemplateList).DeepCopyInto(out.(*NodeTemplateList))
			return nil
		}, InType: reflect.TypeOf(&NodeTemplateList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeTemplateSpec).DeepCopyInto(out.(*NodeTemplateSpec))
			return nil
		}, InType: reflect.TypeOf(&NodeTemplateSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NodeTemplateStatus).DeepCopyInto(out.(*NodeTemplateStatus))
			return nil
		}, InType: reflect.TypeOf(&NodeTemplateStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Notification).DeepCopyInto(out.(*Notification))
			return nil
		}, InType: reflect.TypeOf(&Notification{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Notifier).DeepCopyInto(out.(*Notifier))
			return nil
		}, InType: reflect.TypeOf(&Notifier{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NotifierList).DeepCopyInto(out.(*NotifierList))
			return nil
		}, InType: reflect.TypeOf(&NotifierList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NotifierSpec).DeepCopyInto(out.(*NotifierSpec))
			return nil
		}, InType: reflect.TypeOf(&NotifierSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NotifierStatus).DeepCopyInto(out.(*NotifierStatus))
			return nil
		}, InType: reflect.TypeOf(&NotifierStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PagerdutyConfig).DeepCopyInto(out.(*PagerdutyConfig))
			return nil
		}, InType: reflect.TypeOf(&PagerdutyConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Pipeline).DeepCopyInto(out.(*Pipeline))
			return nil
		}, InType: reflect.TypeOf(&Pipeline{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PipelineExecution).DeepCopyInto(out.(*PipelineExecution))
			return nil
		}, InType: reflect.TypeOf(&PipelineExecution{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PipelineExecutionList).DeepCopyInto(out.(*PipelineExecutionList))
			return nil
		}, InType: reflect.TypeOf(&PipelineExecutionList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PipelineExecutionLog).DeepCopyInto(out.(*PipelineExecutionLog))
			return nil
		}, InType: reflect.TypeOf(&PipelineExecutionLog{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PipelineExecutionLogList).DeepCopyInto(out.(*PipelineExecutionLogList))
			return nil
		}, InType: reflect.TypeOf(&PipelineExecutionLogList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PipelineExecutionLogSpec).DeepCopyInto(out.(*PipelineExecutionLogSpec))
			return nil
		}, InType: reflect.TypeOf(&PipelineExecutionLogSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PipelineExecutionSpec).DeepCopyInto(out.(*PipelineExecutionSpec))
			return nil
		}, InType: reflect.TypeOf(&PipelineExecutionSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PipelineExecutionStatus).DeepCopyInto(out.(*PipelineExecutionStatus))
			return nil
		}, InType: reflect.TypeOf(&PipelineExecutionStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PipelineList).DeepCopyInto(out.(*PipelineList))
			return nil
		}, InType: reflect.TypeOf(&PipelineList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PipelineSpec).DeepCopyInto(out.(*PipelineSpec))
			return nil
		}, InType: reflect.TypeOf(&PipelineSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PipelineStatus).DeepCopyInto(out.(*PipelineStatus))
			return nil
		}, InType: reflect.TypeOf(&PipelineStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PodSecurityPolicyTemplate).DeepCopyInto(out.(*PodSecurityPolicyTemplate))
			return nil
		}, InType: reflect.TypeOf(&PodSecurityPolicyTemplate{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PodSecurityPolicyTemplateList).DeepCopyInto(out.(*PodSecurityPolicyTemplateList))
			return nil
		}, InType: reflect.TypeOf(&PodSecurityPolicyTemplateList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PortCheck).DeepCopyInto(out.(*PortCheck))
			return nil
		}, InType: reflect.TypeOf(&PortCheck{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Preference).DeepCopyInto(out.(*Preference))
			return nil
		}, InType: reflect.TypeOf(&Preference{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PreferenceList).DeepCopyInto(out.(*PreferenceList))
			return nil
		}, InType: reflect.TypeOf(&PreferenceList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Principal).DeepCopyInto(out.(*Principal))
			return nil
		}, InType: reflect.TypeOf(&Principal{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PrincipalList).DeepCopyInto(out.(*PrincipalList))
			return nil
		}, InType: reflect.TypeOf(&PrincipalList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PrivateRegistry).DeepCopyInto(out.(*PrivateRegistry))
			return nil
		}, InType: reflect.TypeOf(&PrivateRegistry{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Process).DeepCopyInto(out.(*Process))
			return nil
		}, InType: reflect.TypeOf(&Process{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Project).DeepCopyInto(out.(*Project))
			return nil
		}, InType: reflect.TypeOf(&Project{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectAlert).DeepCopyInto(out.(*ProjectAlert))
			return nil
		}, InType: reflect.TypeOf(&ProjectAlert{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectAlertList).DeepCopyInto(out.(*ProjectAlertList))
			return nil
		}, InType: reflect.TypeOf(&ProjectAlertList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectAlertSpec).DeepCopyInto(out.(*ProjectAlertSpec))
			return nil
		}, InType: reflect.TypeOf(&ProjectAlertSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectCondition).DeepCopyInto(out.(*ProjectCondition))
			return nil
		}, InType: reflect.TypeOf(&ProjectCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectList).DeepCopyInto(out.(*ProjectList))
			return nil
		}, InType: reflect.TypeOf(&ProjectList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectLogging).DeepCopyInto(out.(*ProjectLogging))
			return nil
		}, InType: reflect.TypeOf(&ProjectLogging{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectLoggingList).DeepCopyInto(out.(*ProjectLoggingList))
			return nil
		}, InType: reflect.TypeOf(&ProjectLoggingList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectLoggingSpec).DeepCopyInto(out.(*ProjectLoggingSpec))
			return nil
		}, InType: reflect.TypeOf(&ProjectLoggingSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectNetworkPolicy).DeepCopyInto(out.(*ProjectNetworkPolicy))
			return nil
		}, InType: reflect.TypeOf(&ProjectNetworkPolicy{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectNetworkPolicyList).DeepCopyInto(out.(*ProjectNetworkPolicyList))
			return nil
		}, InType: reflect.TypeOf(&ProjectNetworkPolicyList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectNetworkPolicySpec).DeepCopyInto(out.(*ProjectNetworkPolicySpec))
			return nil
		}, InType: reflect.TypeOf(&ProjectNetworkPolicySpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectNetworkPolicyStatus).DeepCopyInto(out.(*ProjectNetworkPolicyStatus))
			return nil
		}, InType: reflect.TypeOf(&ProjectNetworkPolicyStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectRoleTemplateBinding).DeepCopyInto(out.(*ProjectRoleTemplateBinding))
			return nil
		}, InType: reflect.TypeOf(&ProjectRoleTemplateBinding{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectRoleTemplateBindingList).DeepCopyInto(out.(*ProjectRoleTemplateBindingList))
			return nil
		}, InType: reflect.TypeOf(&ProjectRoleTemplateBindingList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectSpec).DeepCopyInto(out.(*ProjectSpec))
			return nil
		}, InType: reflect.TypeOf(&ProjectSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectStatus).DeepCopyInto(out.(*ProjectStatus))
			return nil
		}, InType: reflect.TypeOf(&ProjectStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PublishImageConfig).DeepCopyInto(out.(*PublishImageConfig))
			return nil
		}, InType: reflect.TypeOf(&PublishImageConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Question).DeepCopyInto(out.(*Question))
			return nil
		}, InType: reflect.TypeOf(&Question{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RKEConfigNode).DeepCopyInto(out.(*RKEConfigNode))
			return nil
		}, InType: reflect.TypeOf(&RKEConfigNode{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RKEConfigNodePlan).DeepCopyInto(out.(*RKEConfigNodePlan))
			return nil
		}, InType: reflect.TypeOf(&RKEConfigNodePlan{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RKEConfigServices).DeepCopyInto(out.(*RKEConfigServices))
			return nil
		}, InType: reflect.TypeOf(&RKEConfigServices{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RKEPlan).DeepCopyInto(out.(*RKEPlan))
			return nil
		}, InType: reflect.TypeOf(&RKEPlan{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RKESystemImages).DeepCopyInto(out.(*RKESystemImages))
			return nil
		}, InType: reflect.TypeOf(&RKESystemImages{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RancherKubernetesEngineConfig).DeepCopyInto(out.(*RancherKubernetesEngineConfig))
			return nil
		}, InType: reflect.TypeOf(&RancherKubernetesEngineConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RancherSystemImages).DeepCopyInto(out.(*RancherSystemImages))
			return nil
		}, InType: reflect.TypeOf(&RancherSystemImages{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Recipient).DeepCopyInto(out.(*Recipient))
			return nil
		}, InType: reflect.TypeOf(&Recipient{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RepoPerm).DeepCopyInto(out.(*RepoPerm))
			return nil
		}, InType: reflect.TypeOf(&RepoPerm{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RoleTemplate).DeepCopyInto(out.(*RoleTemplate))
			return nil
		}, InType: reflect.TypeOf(&RoleTemplate{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RoleTemplateList).DeepCopyInto(out.(*RoleTemplateList))
			return nil
		}, InType: reflect.TypeOf(&RoleTemplateList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RunPipelineInput).DeepCopyInto(out.(*RunPipelineInput))
			return nil
		}, InType: reflect.TypeOf(&RunPipelineInput{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RunScriptConfig).DeepCopyInto(out.(*RunScriptConfig))
			return nil
		}, InType: reflect.TypeOf(&RunScriptConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SMTPConfig).DeepCopyInto(out.(*SMTPConfig))
			return nil
		}, InType: reflect.TypeOf(&SMTPConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SchedulerService).DeepCopyInto(out.(*SchedulerService))
			return nil
		}, InType: reflect.TypeOf(&SchedulerService{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SearchPrincipalsInput).DeepCopyInto(out.(*SearchPrincipalsInput))
			return nil
		}, InType: reflect.TypeOf(&SearchPrincipalsInput{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SetPasswordInput).DeepCopyInto(out.(*SetPasswordInput))
			return nil
		}, InType: reflect.TypeOf(&SetPasswordInput{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Setting).DeepCopyInto(out.(*Setting))
			return nil
		}, InType: reflect.TypeOf(&Setting{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SettingList).DeepCopyInto(out.(*SettingList))
			return nil
		}, InType: reflect.TypeOf(&SettingList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SlackConfig).DeepCopyInto(out.(*SlackConfig))
			return nil
		}, InType: reflect.TypeOf(&SlackConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SourceCodeConfig).DeepCopyInto(out.(*SourceCodeConfig))
			return nil
		}, InType: reflect.TypeOf(&SourceCodeConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SourceCodeCredential).DeepCopyInto(out.(*SourceCodeCredential))
			return nil
		}, InType: reflect.TypeOf(&SourceCodeCredential{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SourceCodeCredentialList).DeepCopyInto(out.(*SourceCodeCredentialList))
			return nil
		}, InType: reflect.TypeOf(&SourceCodeCredentialList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SourceCodeCredentialSpec).DeepCopyInto(out.(*SourceCodeCredentialSpec))
			return nil
		}, InType: reflect.TypeOf(&SourceCodeCredentialSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SourceCodeCredentialStatus).DeepCopyInto(out.(*SourceCodeCredentialStatus))
			return nil
		}, InType: reflect.TypeOf(&SourceCodeCredentialStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SourceCodeRepository).DeepCopyInto(out.(*SourceCodeRepository))
			return nil
		}, InType: reflect.TypeOf(&SourceCodeRepository{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SourceCodeRepositoryList).DeepCopyInto(out.(*SourceCodeRepositoryList))
			return nil
		}, InType: reflect.TypeOf(&SourceCodeRepositoryList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SourceCodeRepositorySpec).DeepCopyInto(out.(*SourceCodeRepositorySpec))
			return nil
		}, InType: reflect.TypeOf(&SourceCodeRepositorySpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SourceCodeRepositoryStatus).DeepCopyInto(out.(*SourceCodeRepositoryStatus))
			return nil
		}, InType: reflect.TypeOf(&SourceCodeRepositoryStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SplunkConfig).DeepCopyInto(out.(*SplunkConfig))
			return nil
		}, InType: reflect.TypeOf(&SplunkConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Stage).DeepCopyInto(out.(*Stage))
			return nil
		}, InType: reflect.TypeOf(&Stage{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*StageStatus).DeepCopyInto(out.(*StageStatus))
			return nil
		}, InType: reflect.TypeOf(&StageStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Step).DeepCopyInto(out.(*Step))
			return nil
		}, InType: reflect.TypeOf(&Step{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*StepStatus).DeepCopyInto(out.(*StepStatus))
			return nil
		}, InType: reflect.TypeOf(&StepStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SyslogConfig).DeepCopyInto(out.(*SyslogConfig))
			return nil
		}, InType: reflect.TypeOf(&SyslogConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TargetEvent).DeepCopyInto(out.(*TargetEvent))
			return nil
		}, InType: reflect.TypeOf(&TargetEvent{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TargetNode).DeepCopyInto(out.(*TargetNode))
			return nil
		}, InType: reflect.TypeOf(&TargetNode{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TargetPod).DeepCopyInto(out.(*TargetPod))
			return nil
		}, InType: reflect.TypeOf(&TargetPod{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TargetSystemService).DeepCopyInto(out.(*TargetSystemService))
			return nil
		}, InType: reflect.TypeOf(&TargetSystemService{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TargetWorkload).DeepCopyInto(out.(*TargetWorkload))
			return nil
		}, InType: reflect.TypeOf(&TargetWorkload{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Template).DeepCopyInto(out.(*Template))
			return nil
		}, InType: reflect.TypeOf(&Template{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TemplateList).DeepCopyInto(out.(*TemplateList))
			return nil
		}, InType: reflect.TypeOf(&TemplateList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TemplateSpec).DeepCopyInto(out.(*TemplateSpec))
			return nil
		}, InType: reflect.TypeOf(&TemplateSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TemplateStatus).DeepCopyInto(out.(*TemplateStatus))
			return nil
		}, InType: reflect.TypeOf(&TemplateStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TemplateVersion).DeepCopyInto(out.(*TemplateVersion))
			return nil
		}, InType: reflect.TypeOf(&TemplateVersion{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TemplateVersionList).DeepCopyInto(out.(*TemplateVersionList))
			return nil
		}, InType: reflect.TypeOf(&TemplateVersionList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TemplateVersionSpec).DeepCopyInto(out.(*TemplateVersionSpec))
			return nil
		}, InType: reflect.TypeOf(&TemplateVersionSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TemplateVersionStatus).DeepCopyInto(out.(*TemplateVersionStatus))
			return nil
		}, InType: reflect.TypeOf(&TemplateVersionStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Token).DeepCopyInto(out.(*Token))
			return nil
		}, InType: reflect.TypeOf(&Token{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*TokenList).DeepCopyInto(out.(*TokenList))
			return nil
		}, InType: reflect.TypeOf(&TokenList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*User).DeepCopyInto(out.(*User))
			return nil
		}, InType: reflect.TypeOf(&User{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*UserList).DeepCopyInto(out.(*UserList))
			return nil
		}, InType: reflect.TypeOf(&UserList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Values).DeepCopyInto(out.(*Values))
			return nil
		}, InType: reflect.TypeOf(&Values{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*VersionCommits).DeepCopyInto(out.(*VersionCommits))
			return nil
		}, InType: reflect.TypeOf(&VersionCommits{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*WebhookConfig).DeepCopyInto(out.(*WebhookConfig))
			return nil
		}, InType: reflect.TypeOf(&WebhookConfig{})},
	)
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Action) DeepCopyInto(out *Action) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Action.
func (in *Action) DeepCopy() *Action {
	if in == nil {
		return nil
	}
	out := new(Action)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ActiveDirectoryConfig) DeepCopyInto(out *ActiveDirectoryConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.AuthConfig.DeepCopyInto(&out.AuthConfig)
	if in.Servers != nil {
		in, out := &in.Servers, &out.Servers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ActiveDirectoryConfig.
func (in *ActiveDirectoryConfig) DeepCopy() *ActiveDirectoryConfig {
	if in == nil {
		return nil
	}
	out := new(ActiveDirectoryConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ActiveDirectoryConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ActiveDirectoryTestAndApplyInput) DeepCopyInto(out *ActiveDirectoryTestAndApplyInput) {
	*out = *in
	in.ActiveDirectoryConfig.DeepCopyInto(&out.ActiveDirectoryConfig)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ActiveDirectoryTestAndApplyInput.
func (in *ActiveDirectoryTestAndApplyInput) DeepCopy() *ActiveDirectoryTestAndApplyInput {
	if in == nil {
		return nil
	}
	out := new(ActiveDirectoryTestAndApplyInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertCommonSpec) DeepCopyInto(out *AlertCommonSpec) {
	*out = *in
	if in.Recipients != nil {
		in, out := &in.Recipients, &out.Recipients
		*out = make([]Recipient, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertCommonSpec.
func (in *AlertCommonSpec) DeepCopy() *AlertCommonSpec {
	if in == nil {
		return nil
	}
	out := new(AlertCommonSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertStatus) DeepCopyInto(out *AlertStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertStatus.
func (in *AlertStatus) DeepCopy() *AlertStatus {
	if in == nil {
		return nil
	}
	out := new(AlertStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthAppInput) DeepCopyInto(out *AuthAppInput) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthAppInput.
func (in *AuthAppInput) DeepCopy() *AuthAppInput {
	if in == nil {
		return nil
	}
	out := new(AuthAppInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthConfig) DeepCopyInto(out *AuthConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.AllowedPrincipalIDs != nil {
		in, out := &in.AllowedPrincipalIDs, &out.AllowedPrincipalIDs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthConfig.
func (in *AuthConfig) DeepCopy() *AuthConfig {
	if in == nil {
		return nil
	}
	out := new(AuthConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AuthConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthConfigList) DeepCopyInto(out *AuthConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AuthConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthConfigList.
func (in *AuthConfigList) DeepCopy() *AuthConfigList {
	if in == nil {
		return nil
	}
	out := new(AuthConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AuthConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthUserInput) DeepCopyInto(out *AuthUserInput) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthUserInput.
func (in *AuthUserInput) DeepCopy() *AuthUserInput {
	if in == nil {
		return nil
	}
	out := new(AuthUserInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthnConfig) DeepCopyInto(out *AuthnConfig) {
	*out = *in
	if in.Options != nil {
		in, out := &in.Options, &out.Options
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthnConfig.
func (in *AuthnConfig) DeepCopy() *AuthnConfig {
	if in == nil {
		return nil
	}
	out := new(AuthnConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthzConfig) DeepCopyInto(out *AuthzConfig) {
	*out = *in
	if in.Options != nil {
		in, out := &in.Options, &out.Options
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthzConfig.
func (in *AuthzConfig) DeepCopy() *AuthzConfig {
	if in == nil {
		return nil
	}
	out := new(AuthzConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureKubernetesServiceConfig) DeepCopyInto(out *AzureKubernetesServiceConfig) {
	*out = *in
	if in.Tag != nil {
		in, out := &in.Tag, &out.Tag
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureKubernetesServiceConfig.
func (in *AzureKubernetesServiceConfig) DeepCopy() *AzureKubernetesServiceConfig {
	if in == nil {
		return nil
	}
	out := new(AzureKubernetesServiceConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BaseService) DeepCopyInto(out *BaseService) {
	*out = *in
	if in.ExtraArgs != nil {
		in, out := &in.ExtraArgs, &out.ExtraArgs
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BaseService.
func (in *BaseService) DeepCopy() *BaseService {
	if in == nil {
		return nil
	}
	out := new(BaseService)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Catalog) DeepCopyInto(out *Catalog) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Catalog.
func (in *Catalog) DeepCopy() *Catalog {
	if in == nil {
		return nil
	}
	out := new(Catalog)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Catalog) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CatalogCondition) DeepCopyInto(out *CatalogCondition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CatalogCondition.
func (in *CatalogCondition) DeepCopy() *CatalogCondition {
	if in == nil {
		return nil
	}
	out := new(CatalogCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CatalogList) DeepCopyInto(out *CatalogList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Catalog, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CatalogList.
func (in *CatalogList) DeepCopy() *CatalogList {
	if in == nil {
		return nil
	}
	out := new(CatalogList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CatalogList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CatalogSpec) DeepCopyInto(out *CatalogSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CatalogSpec.
func (in *CatalogSpec) DeepCopy() *CatalogSpec {
	if in == nil {
		return nil
	}
	out := new(CatalogSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CatalogStatus) DeepCopyInto(out *CatalogStatus) {
	*out = *in
	if in.HelmVersionCommits != nil {
		in, out := &in.HelmVersionCommits, &out.HelmVersionCommits
		*out = make(map[string]VersionCommits, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]CatalogCondition, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CatalogStatus.
func (in *CatalogStatus) DeepCopy() *CatalogStatus {
	if in == nil {
		return nil
	}
	out := new(CatalogStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChangePasswordInput) DeepCopyInto(out *ChangePasswordInput) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChangePasswordInput.
func (in *ChangePasswordInput) DeepCopy() *ChangePasswordInput {
	if in == nil {
		return nil
	}
	out := new(ChangePasswordInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Cluster) DeepCopyInto(out *Cluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Cluster.
func (in *Cluster) DeepCopy() *Cluster {
	if in == nil {
		return nil
	}
	out := new(Cluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Cluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterAlert) DeepCopyInto(out *ClusterAlert) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterAlert.
func (in *ClusterAlert) DeepCopy() *ClusterAlert {
	if in == nil {
		return nil
	}
	out := new(ClusterAlert)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterAlert) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterAlertList) DeepCopyInto(out *ClusterAlertList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterAlert, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterAlertList.
func (in *ClusterAlertList) DeepCopy() *ClusterAlertList {
	if in == nil {
		return nil
	}
	out := new(ClusterAlertList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterAlertList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterAlertSpec) DeepCopyInto(out *ClusterAlertSpec) {
	*out = *in
	in.AlertCommonSpec.DeepCopyInto(&out.AlertCommonSpec)
	in.TargetNode.DeepCopyInto(&out.TargetNode)
	out.TargetSystemService = in.TargetSystemService
	out.TargetEvent = in.TargetEvent
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterAlertSpec.
func (in *ClusterAlertSpec) DeepCopy() *ClusterAlertSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterAlertSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterComponentStatus) DeepCopyInto(out *ClusterComponentStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.ComponentCondition, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterComponentStatus.
func (in *ClusterComponentStatus) DeepCopy() *ClusterComponentStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterComponentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterComposeConfig) DeepCopyInto(out *ClusterComposeConfig) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterComposeConfig.
func (in *ClusterComposeConfig) DeepCopy() *ClusterComposeConfig {
	if in == nil {
		return nil
	}
	out := new(ClusterComposeConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterComposeConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterComposeConfigList) DeepCopyInto(out *ClusterComposeConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterComposeConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterComposeConfigList.
func (in *ClusterComposeConfigList) DeepCopy() *ClusterComposeConfigList {
	if in == nil {
		return nil
	}
	out := new(ClusterComposeConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterComposeConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterComposeSpec) DeepCopyInto(out *ClusterComposeSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterComposeSpec.
func (in *ClusterComposeSpec) DeepCopy() *ClusterComposeSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterComposeSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterCondition) DeepCopyInto(out *ClusterCondition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterCondition.
func (in *ClusterCondition) DeepCopy() *ClusterCondition {
	if in == nil {
		return nil
	}
	out := new(ClusterCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterEvent) DeepCopyInto(out *ClusterEvent) {
	*out = *in
	out.Namespaced = in.Namespaced
	in.Event.DeepCopyInto(&out.Event)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterEvent.
func (in *ClusterEvent) DeepCopy() *ClusterEvent {
	if in == nil {
		return nil
	}
	out := new(ClusterEvent)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterEvent) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterEventList) DeepCopyInto(out *ClusterEventList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterEvent, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterEventList.
func (in *ClusterEventList) DeepCopy() *ClusterEventList {
	if in == nil {
		return nil
	}
	out := new(ClusterEventList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterEventList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterList) DeepCopyInto(out *ClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Cluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterList.
func (in *ClusterList) DeepCopy() *ClusterList {
	if in == nil {
		return nil
	}
	out := new(ClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterLogging) DeepCopyInto(out *ClusterLogging) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterLogging.
func (in *ClusterLogging) DeepCopy() *ClusterLogging {
	if in == nil {
		return nil
	}
	out := new(ClusterLogging)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterLogging) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterLoggingList) DeepCopyInto(out *ClusterLoggingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterLogging, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterLoggingList.
func (in *ClusterLoggingList) DeepCopy() *ClusterLoggingList {
	if in == nil {
		return nil
	}
	out := new(ClusterLoggingList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterLoggingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterLoggingSpec) DeepCopyInto(out *ClusterLoggingSpec) {
	*out = *in
	in.LoggingCommonSpec.DeepCopyInto(&out.LoggingCommonSpec)
	if in.EmbeddedConfig != nil {
		in, out := &in.EmbeddedConfig, &out.EmbeddedConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(EmbeddedConfig)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterLoggingSpec.
func (in *ClusterLoggingSpec) DeepCopy() *ClusterLoggingSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterLoggingSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterPipeline) DeepCopyInto(out *ClusterPipeline) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterPipeline.
func (in *ClusterPipeline) DeepCopy() *ClusterPipeline {
	if in == nil {
		return nil
	}
	out := new(ClusterPipeline)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterPipeline) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterPipelineList) DeepCopyInto(out *ClusterPipelineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterPipeline, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterPipelineList.
func (in *ClusterPipelineList) DeepCopy() *ClusterPipelineList {
	if in == nil {
		return nil
	}
	out := new(ClusterPipelineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterPipelineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterPipelineSpec) DeepCopyInto(out *ClusterPipelineSpec) {
	*out = *in
	if in.GithubConfig != nil {
		in, out := &in.GithubConfig, &out.GithubConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(GithubClusterConfig)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterPipelineSpec.
func (in *ClusterPipelineSpec) DeepCopy() *ClusterPipelineSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterPipelineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterPipelineStatus) DeepCopyInto(out *ClusterPipelineStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterPipelineStatus.
func (in *ClusterPipelineStatus) DeepCopy() *ClusterPipelineStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterPipelineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterRegistrationToken) DeepCopyInto(out *ClusterRegistrationToken) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterRegistrationToken.
func (in *ClusterRegistrationToken) DeepCopy() *ClusterRegistrationToken {
	if in == nil {
		return nil
	}
	out := new(ClusterRegistrationToken)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterRegistrationToken) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterRegistrationTokenList) DeepCopyInto(out *ClusterRegistrationTokenList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterRegistrationToken, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterRegistrationTokenList.
func (in *ClusterRegistrationTokenList) DeepCopy() *ClusterRegistrationTokenList {
	if in == nil {
		return nil
	}
	out := new(ClusterRegistrationTokenList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterRegistrationTokenList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterRegistrationTokenSpec) DeepCopyInto(out *ClusterRegistrationTokenSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterRegistrationTokenSpec.
func (in *ClusterRegistrationTokenSpec) DeepCopy() *ClusterRegistrationTokenSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterRegistrationTokenSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterRegistrationTokenStatus) DeepCopyInto(out *ClusterRegistrationTokenStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterRegistrationTokenStatus.
func (in *ClusterRegistrationTokenStatus) DeepCopy() *ClusterRegistrationTokenStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterRegistrationTokenStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterRoleTemplateBinding) DeepCopyInto(out *ClusterRoleTemplateBinding) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterRoleTemplateBinding.
func (in *ClusterRoleTemplateBinding) DeepCopy() *ClusterRoleTemplateBinding {
	if in == nil {
		return nil
	}
	out := new(ClusterRoleTemplateBinding)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterRoleTemplateBinding) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterRoleTemplateBindingList) DeepCopyInto(out *ClusterRoleTemplateBindingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterRoleTemplateBinding, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterRoleTemplateBindingList.
func (in *ClusterRoleTemplateBindingList) DeepCopy() *ClusterRoleTemplateBindingList {
	if in == nil {
		return nil
	}
	out := new(ClusterRoleTemplateBindingList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterRoleTemplateBindingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSpec) DeepCopyInto(out *ClusterSpec) {
	*out = *in
	if in.ImportedConfig != nil {
		in, out := &in.ImportedConfig, &out.ImportedConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(ImportedConfig)
			**out = **in
		}
	}
	if in.GoogleKubernetesEngineConfig != nil {
		in, out := &in.GoogleKubernetesEngineConfig, &out.GoogleKubernetesEngineConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(GoogleKubernetesEngineConfig)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.AzureKubernetesServiceConfig != nil {
		in, out := &in.AzureKubernetesServiceConfig, &out.AzureKubernetesServiceConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(AzureKubernetesServiceConfig)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.RancherKubernetesEngineConfig != nil {
		in, out := &in.RancherKubernetesEngineConfig, &out.RancherKubernetesEngineConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(RancherKubernetesEngineConfig)
			(*in).DeepCopyInto(*out)
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSpec.
func (in *ClusterSpec) DeepCopy() *ClusterSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterStatus) DeepCopyInto(out *ClusterStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]ClusterCondition, len(*in))
		copy(*out, *in)
	}
	if in.ComponentStatuses != nil {
		in, out := &in.ComponentStatuses, &out.ComponentStatuses
		*out = make([]ClusterComponentStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Capacity != nil {
		in, out := &in.Capacity, &out.Capacity
		*out = make(v1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	if in.Allocatable != nil {
		in, out := &in.Allocatable, &out.Allocatable
		*out = make(v1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	in.AppliedSpec.DeepCopyInto(&out.AppliedSpec)
	if in.FailedSpec != nil {
		in, out := &in.FailedSpec, &out.FailedSpec
		if *in == nil {
			*out = nil
		} else {
			*out = new(ClusterSpec)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.Requested != nil {
		in, out := &in.Requested, &out.Requested
		*out = make(v1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	if in.Limits != nil {
		in, out := &in.Limits, &out.Limits
		*out = make(v1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterStatus.
func (in *ClusterStatus) DeepCopy() *ClusterStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ComposeCondition) DeepCopyInto(out *ComposeCondition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComposeCondition.
func (in *ComposeCondition) DeepCopy() *ComposeCondition {
	if in == nil {
		return nil
	}
	out := new(ComposeCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ComposeSpec) DeepCopyInto(out *ComposeSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComposeSpec.
func (in *ComposeSpec) DeepCopy() *ComposeSpec {
	if in == nil {
		return nil
	}
	out := new(ComposeSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ComposeStatus) DeepCopyInto(out *ComposeStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]ComposeCondition, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComposeStatus.
func (in *ComposeStatus) DeepCopy() *ComposeStatus {
	if in == nil {
		return nil
	}
	out := new(ComposeStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Condition) DeepCopyInto(out *Condition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Condition.
func (in *Condition) DeepCopy() *Condition {
	if in == nil {
		return nil
	}
	out := new(Condition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CustomConfig) DeepCopyInto(out *CustomConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CustomConfig.
func (in *CustomConfig) DeepCopy() *CustomConfig {
	if in == nil {
		return nil
	}
	out := new(CustomConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DynamicSchema) DeepCopyInto(out *DynamicSchema) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DynamicSchema.
func (in *DynamicSchema) DeepCopy() *DynamicSchema {
	if in == nil {
		return nil
	}
	out := new(DynamicSchema)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DynamicSchema) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DynamicSchemaList) DeepCopyInto(out *DynamicSchemaList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DynamicSchema, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DynamicSchemaList.
func (in *DynamicSchemaList) DeepCopy() *DynamicSchemaList {
	if in == nil {
		return nil
	}
	out := new(DynamicSchemaList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DynamicSchemaList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DynamicSchemaSpec) DeepCopyInto(out *DynamicSchemaSpec) {
	*out = *in
	if in.ResourceMethods != nil {
		in, out := &in.ResourceMethods, &out.ResourceMethods
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ResourceFields != nil {
		in, out := &in.ResourceFields, &out.ResourceFields
		*out = make(map[string]Field, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.ResourceActions != nil {
		in, out := &in.ResourceActions, &out.ResourceActions
		*out = make(map[string]Action, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.CollectionMethods != nil {
		in, out := &in.CollectionMethods, &out.CollectionMethods
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.CollectionFields != nil {
		in, out := &in.CollectionFields, &out.CollectionFields
		*out = make(map[string]Field, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.CollectionActions != nil {
		in, out := &in.CollectionActions, &out.CollectionActions
		*out = make(map[string]Action, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.CollectionFilters != nil {
		in, out := &in.CollectionFilters, &out.CollectionFilters
		*out = make(map[string]Filter, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.IncludeableLinks != nil {
		in, out := &in.IncludeableLinks, &out.IncludeableLinks
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DynamicSchemaSpec.
func (in *DynamicSchemaSpec) DeepCopy() *DynamicSchemaSpec {
	if in == nil {
		return nil
	}
	out := new(DynamicSchemaSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DynamicSchemaStatus) DeepCopyInto(out *DynamicSchemaStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DynamicSchemaStatus.
func (in *DynamicSchemaStatus) DeepCopy() *DynamicSchemaStatus {
	if in == nil {
		return nil
	}
	out := new(DynamicSchemaStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ETCDService) DeepCopyInto(out *ETCDService) {
	*out = *in
	in.BaseService.DeepCopyInto(&out.BaseService)
	if in.ExternalURLs != nil {
		in, out := &in.ExternalURLs, &out.ExternalURLs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ETCDService.
func (in *ETCDService) DeepCopy() *ETCDService {
	if in == nil {
		return nil
	}
	out := new(ETCDService)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ElasticsearchConfig) DeepCopyInto(out *ElasticsearchConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ElasticsearchConfig.
func (in *ElasticsearchConfig) DeepCopy() *ElasticsearchConfig {
	if in == nil {
		return nil
	}
	out := new(ElasticsearchConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EmbeddedConfig) DeepCopyInto(out *EmbeddedConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EmbeddedConfig.
func (in *EmbeddedConfig) DeepCopy() *EmbeddedConfig {
	if in == nil {
		return nil
	}
	out := new(EmbeddedConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Field) DeepCopyInto(out *Field) {
	*out = *in
	in.Default.DeepCopyInto(&out.Default)
	if in.Options != nil {
		in, out := &in.Options, &out.Options
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Field.
func (in *Field) DeepCopy() *Field {
	if in == nil {
		return nil
	}
	out := new(Field)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *File) DeepCopyInto(out *File) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new File.
func (in *File) DeepCopy() *File {
	if in == nil {
		return nil
	}
	out := new(File)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Filter) DeepCopyInto(out *Filter) {
	*out = *in
	if in.Modifiers != nil {
		in, out := &in.Modifiers, &out.Modifiers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Filter.
func (in *Filter) DeepCopy() *Filter {
	if in == nil {
		return nil
	}
	out := new(Filter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GithubClusterConfig) DeepCopyInto(out *GithubClusterConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GithubClusterConfig.
func (in *GithubClusterConfig) DeepCopy() *GithubClusterConfig {
	if in == nil {
		return nil
	}
	out := new(GithubClusterConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GithubConfig) DeepCopyInto(out *GithubConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.AuthConfig.DeepCopyInto(&out.AuthConfig)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GithubConfig.
func (in *GithubConfig) DeepCopy() *GithubConfig {
	if in == nil {
		return nil
	}
	out := new(GithubConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GithubConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GithubConfigApplyInput) DeepCopyInto(out *GithubConfigApplyInput) {
	*out = *in
	in.GithubConfig.DeepCopyInto(&out.GithubConfig)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GithubConfigApplyInput.
func (in *GithubConfigApplyInput) DeepCopy() *GithubConfigApplyInput {
	if in == nil {
		return nil
	}
	out := new(GithubConfigApplyInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GithubConfigTestOutput) DeepCopyInto(out *GithubConfigTestOutput) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GithubConfigTestOutput.
func (in *GithubConfigTestOutput) DeepCopy() *GithubConfigTestOutput {
	if in == nil {
		return nil
	}
	out := new(GithubConfigTestOutput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GlobalComposeConfig) DeepCopyInto(out *GlobalComposeConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GlobalComposeConfig.
func (in *GlobalComposeConfig) DeepCopy() *GlobalComposeConfig {
	if in == nil {
		return nil
	}
	out := new(GlobalComposeConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GlobalComposeConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GlobalComposeConfigList) DeepCopyInto(out *GlobalComposeConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]GlobalComposeConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GlobalComposeConfigList.
func (in *GlobalComposeConfigList) DeepCopy() *GlobalComposeConfigList {
	if in == nil {
		return nil
	}
	out := new(GlobalComposeConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GlobalComposeConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GlobalRole) DeepCopyInto(out *GlobalRole) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]rbac_v1.PolicyRule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GlobalRole.
func (in *GlobalRole) DeepCopy() *GlobalRole {
	if in == nil {
		return nil
	}
	out := new(GlobalRole)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GlobalRole) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GlobalRoleBinding) DeepCopyInto(out *GlobalRoleBinding) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GlobalRoleBinding.
func (in *GlobalRoleBinding) DeepCopy() *GlobalRoleBinding {
	if in == nil {
		return nil
	}
	out := new(GlobalRoleBinding)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GlobalRoleBinding) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GlobalRoleBindingList) DeepCopyInto(out *GlobalRoleBindingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]GlobalRoleBinding, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GlobalRoleBindingList.
func (in *GlobalRoleBindingList) DeepCopy() *GlobalRoleBindingList {
	if in == nil {
		return nil
	}
	out := new(GlobalRoleBindingList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GlobalRoleBindingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GlobalRoleList) DeepCopyInto(out *GlobalRoleList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]GlobalRole, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GlobalRoleList.
func (in *GlobalRoleList) DeepCopy() *GlobalRoleList {
	if in == nil {
		return nil
	}
	out := new(GlobalRoleList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GlobalRoleList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GoogleKubernetesEngineConfig) DeepCopyInto(out *GoogleKubernetesEngineConfig) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Locations != nil {
		in, out := &in.Locations, &out.Locations
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GoogleKubernetesEngineConfig.
func (in *GoogleKubernetesEngineConfig) DeepCopy() *GoogleKubernetesEngineConfig {
	if in == nil {
		return nil
	}
	out := new(GoogleKubernetesEngineConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Group) DeepCopyInto(out *Group) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Group.
func (in *Group) DeepCopy() *Group {
	if in == nil {
		return nil
	}
	out := new(Group)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Group) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GroupList) DeepCopyInto(out *GroupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Group, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GroupList.
func (in *GroupList) DeepCopy() *GroupList {
	if in == nil {
		return nil
	}
	out := new(GroupList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GroupList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GroupMember) DeepCopyInto(out *GroupMember) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GroupMember.
func (in *GroupMember) DeepCopy() *GroupMember {
	if in == nil {
		return nil
	}
	out := new(GroupMember)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GroupMember) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GroupMemberList) DeepCopyInto(out *GroupMemberList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]GroupMember, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GroupMemberList.
func (in *GroupMemberList) DeepCopy() *GroupMemberList {
	if in == nil {
		return nil
	}
	out := new(GroupMemberList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GroupMemberList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HealthCheck) DeepCopyInto(out *HealthCheck) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HealthCheck.
func (in *HealthCheck) DeepCopy() *HealthCheck {
	if in == nil {
		return nil
	}
	out := new(HealthCheck)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImportedConfig) DeepCopyInto(out *ImportedConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImportedConfig.
func (in *ImportedConfig) DeepCopy() *ImportedConfig {
	if in == nil {
		return nil
	}
	out := new(ImportedConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IngressConfig) DeepCopyInto(out *IngressConfig) {
	*out = *in
	if in.Options != nil {
		in, out := &in.Options, &out.Options
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IngressConfig.
func (in *IngressConfig) DeepCopy() *IngressConfig {
	if in == nil {
		return nil
	}
	out := new(IngressConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KafkaConfig) DeepCopyInto(out *KafkaConfig) {
	*out = *in
	if in.BrokerEndpoints != nil {
		in, out := &in.BrokerEndpoints, &out.BrokerEndpoints
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KafkaConfig.
func (in *KafkaConfig) DeepCopy() *KafkaConfig {
	if in == nil {
		return nil
	}
	out := new(KafkaConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeAPIService) DeepCopyInto(out *KubeAPIService) {
	*out = *in
	in.BaseService.DeepCopyInto(&out.BaseService)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeAPIService.
func (in *KubeAPIService) DeepCopy() *KubeAPIService {
	if in == nil {
		return nil
	}
	out := new(KubeAPIService)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeControllerService) DeepCopyInto(out *KubeControllerService) {
	*out = *in
	in.BaseService.DeepCopyInto(&out.BaseService)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeControllerService.
func (in *KubeControllerService) DeepCopy() *KubeControllerService {
	if in == nil {
		return nil
	}
	out := new(KubeControllerService)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeletService) DeepCopyInto(out *KubeletService) {
	*out = *in
	in.BaseService.DeepCopyInto(&out.BaseService)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeletService.
func (in *KubeletService) DeepCopy() *KubeletService {
	if in == nil {
		return nil
	}
	out := new(KubeletService)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KubeproxyService) DeepCopyInto(out *KubeproxyService) {
	*out = *in
	in.BaseService.DeepCopyInto(&out.BaseService)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KubeproxyService.
func (in *KubeproxyService) DeepCopy() *KubeproxyService {
	if in == nil {
		return nil
	}
	out := new(KubeproxyService)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ListOpts) DeepCopyInto(out *ListOpts) {
	*out = *in
	if in.Filters != nil {
		in, out := &in.Filters, &out.Filters
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ListOpts.
func (in *ListOpts) DeepCopy() *ListOpts {
	if in == nil {
		return nil
	}
	out := new(ListOpts)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ListenConfig) DeepCopyInto(out *ListenConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Domains != nil {
		in, out := &in.Domains, &out.Domains
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.TOS != nil {
		in, out := &in.TOS, &out.TOS
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.KnownIPs != nil {
		in, out := &in.KnownIPs, &out.KnownIPs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.GeneratedCerts != nil {
		in, out := &in.GeneratedCerts, &out.GeneratedCerts
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.SubjectAlternativeNames != nil {
		in, out := &in.SubjectAlternativeNames, &out.SubjectAlternativeNames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ListenConfig.
func (in *ListenConfig) DeepCopy() *ListenConfig {
	if in == nil {
		return nil
	}
	out := new(ListenConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ListenConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ListenConfigList) DeepCopyInto(out *ListenConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ListenConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ListenConfigList.
func (in *ListenConfigList) DeepCopy() *ListenConfigList {
	if in == nil {
		return nil
	}
	out := new(ListenConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ListenConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LocalConfig) DeepCopyInto(out *LocalConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.AuthConfig.DeepCopyInto(&out.AuthConfig)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LocalConfig.
func (in *LocalConfig) DeepCopy() *LocalConfig {
	if in == nil {
		return nil
	}
	out := new(LocalConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LocalConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LoggingCommonSpec) DeepCopyInto(out *LoggingCommonSpec) {
	*out = *in
	if in.OutputTags != nil {
		in, out := &in.OutputTags, &out.OutputTags
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.ElasticsearchConfig != nil {
		in, out := &in.ElasticsearchConfig, &out.ElasticsearchConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(ElasticsearchConfig)
			**out = **in
		}
	}
	if in.SplunkConfig != nil {
		in, out := &in.SplunkConfig, &out.SplunkConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(SplunkConfig)
			**out = **in
		}
	}
	if in.KafkaConfig != nil {
		in, out := &in.KafkaConfig, &out.KafkaConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(KafkaConfig)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.SyslogConfig != nil {
		in, out := &in.SyslogConfig, &out.SyslogConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(SyslogConfig)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LoggingCommonSpec.
func (in *LoggingCommonSpec) DeepCopy() *LoggingCommonSpec {
	if in == nil {
		return nil
	}
	out := new(LoggingCommonSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LoggingCondition) DeepCopyInto(out *LoggingCondition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LoggingCondition.
func (in *LoggingCondition) DeepCopy() *LoggingCondition {
	if in == nil {
		return nil
	}
	out := new(LoggingCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LoggingStatus) DeepCopyInto(out *LoggingStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]LoggingCondition, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LoggingStatus.
func (in *LoggingStatus) DeepCopy() *LoggingStatus {
	if in == nil {
		return nil
	}
	out := new(LoggingStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkConfig) DeepCopyInto(out *NetworkConfig) {
	*out = *in
	if in.Options != nil {
		in, out := &in.Options, &out.Options
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkConfig.
func (in *NetworkConfig) DeepCopy() *NetworkConfig {
	if in == nil {
		return nil
	}
	out := new(NetworkConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Node) DeepCopyInto(out *Node) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Node.
func (in *Node) DeepCopy() *Node {
	if in == nil {
		return nil
	}
	out := new(Node)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Node) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeCommonParams) DeepCopyInto(out *NodeCommonParams) {
	*out = *in
	if in.EngineOpt != nil {
		in, out := &in.EngineOpt, &out.EngineOpt
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.EngineInsecureRegistry != nil {
		in, out := &in.EngineInsecureRegistry, &out.EngineInsecureRegistry
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.EngineRegistryMirror != nil {
		in, out := &in.EngineRegistryMirror, &out.EngineRegistryMirror
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.EngineLabel != nil {
		in, out := &in.EngineLabel, &out.EngineLabel
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.EngineEnv != nil {
		in, out := &in.EngineEnv, &out.EngineEnv
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeCommonParams.
func (in *NodeCommonParams) DeepCopy() *NodeCommonParams {
	if in == nil {
		return nil
	}
	out := new(NodeCommonParams)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeCondition) DeepCopyInto(out *NodeCondition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeCondition.
func (in *NodeCondition) DeepCopy() *NodeCondition {
	if in == nil {
		return nil
	}
	out := new(NodeCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeDriver) DeepCopyInto(out *NodeDriver) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeDriver.
func (in *NodeDriver) DeepCopy() *NodeDriver {
	if in == nil {
		return nil
	}
	out := new(NodeDriver)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeDriver) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeDriverList) DeepCopyInto(out *NodeDriverList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NodeDriver, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeDriverList.
func (in *NodeDriverList) DeepCopy() *NodeDriverList {
	if in == nil {
		return nil
	}
	out := new(NodeDriverList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeDriverList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeDriverSpec) DeepCopyInto(out *NodeDriverSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeDriverSpec.
func (in *NodeDriverSpec) DeepCopy() *NodeDriverSpec {
	if in == nil {
		return nil
	}
	out := new(NodeDriverSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeDriverStatus) DeepCopyInto(out *NodeDriverStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]Condition, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeDriverStatus.
func (in *NodeDriverStatus) DeepCopy() *NodeDriverStatus {
	if in == nil {
		return nil
	}
	out := new(NodeDriverStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeList) DeepCopyInto(out *NodeList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Node, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeList.
func (in *NodeList) DeepCopy() *NodeList {
	if in == nil {
		return nil
	}
	out := new(NodeList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodePool) DeepCopyInto(out *NodePool) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodePool.
func (in *NodePool) DeepCopy() *NodePool {
	if in == nil {
		return nil
	}
	out := new(NodePool)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodePool) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodePoolList) DeepCopyInto(out *NodePoolList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NodePool, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodePoolList.
func (in *NodePoolList) DeepCopy() *NodePoolList {
	if in == nil {
		return nil
	}
	out := new(NodePoolList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodePoolList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodePoolSpec) DeepCopyInto(out *NodePoolSpec) {
	*out = *in
	if in.NodeLabels != nil {
		in, out := &in.NodeLabels, &out.NodeLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.NodeAnnotations != nil {
		in, out := &in.NodeAnnotations, &out.NodeAnnotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodePoolSpec.
func (in *NodePoolSpec) DeepCopy() *NodePoolSpec {
	if in == nil {
		return nil
	}
	out := new(NodePoolSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodePoolStatus) DeepCopyInto(out *NodePoolStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]Condition, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodePoolStatus.
func (in *NodePoolStatus) DeepCopy() *NodePoolStatus {
	if in == nil {
		return nil
	}
	out := new(NodePoolStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeSpec) DeepCopyInto(out *NodeSpec) {
	*out = *in
	if in.CustomConfig != nil {
		in, out := &in.CustomConfig, &out.CustomConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(CustomConfig)
			**out = **in
		}
	}
	in.InternalNodeSpec.DeepCopyInto(&out.InternalNodeSpec)
	if in.DesiredNodeLabels != nil {
		in, out := &in.DesiredNodeLabels, &out.DesiredNodeLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.DesiredNodeAnnotations != nil {
		in, out := &in.DesiredNodeAnnotations, &out.DesiredNodeAnnotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeSpec.
func (in *NodeSpec) DeepCopy() *NodeSpec {
	if in == nil {
		return nil
	}
	out := new(NodeSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeStatus) DeepCopyInto(out *NodeStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]NodeCondition, len(*in))
		copy(*out, *in)
	}
	in.InternalNodeStatus.DeepCopyInto(&out.InternalNodeStatus)
	if in.Requested != nil {
		in, out := &in.Requested, &out.Requested
		*out = make(v1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	if in.Limits != nil {
		in, out := &in.Limits, &out.Limits
		*out = make(v1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	if in.NodeTemplateSpec != nil {
		in, out := &in.NodeTemplateSpec, &out.NodeTemplateSpec
		if *in == nil {
			*out = nil
		} else {
			*out = new(NodeTemplateSpec)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.NodeConfig != nil {
		in, out := &in.NodeConfig, &out.NodeConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(RKEConfigNode)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.NodeAnnotations != nil {
		in, out := &in.NodeAnnotations, &out.NodeAnnotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.NodeLabels != nil {
		in, out := &in.NodeLabels, &out.NodeLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.NodeTaints != nil {
		in, out := &in.NodeTaints, &out.NodeTaints
		*out = make([]v1.Taint, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeStatus.
func (in *NodeStatus) DeepCopy() *NodeStatus {
	if in == nil {
		return nil
	}
	out := new(NodeStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeTemplate) DeepCopyInto(out *NodeTemplate) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeTemplate.
func (in *NodeTemplate) DeepCopy() *NodeTemplate {
	if in == nil {
		return nil
	}
	out := new(NodeTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeTemplateCondition) DeepCopyInto(out *NodeTemplateCondition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeTemplateCondition.
func (in *NodeTemplateCondition) DeepCopy() *NodeTemplateCondition {
	if in == nil {
		return nil
	}
	out := new(NodeTemplateCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeTemplateList) DeepCopyInto(out *NodeTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NodeTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeTemplateList.
func (in *NodeTemplateList) DeepCopy() *NodeTemplateList {
	if in == nil {
		return nil
	}
	out := new(NodeTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NodeTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeTemplateSpec) DeepCopyInto(out *NodeTemplateSpec) {
	*out = *in
	in.NodeCommonParams.DeepCopyInto(&out.NodeCommonParams)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeTemplateSpec.
func (in *NodeTemplateSpec) DeepCopy() *NodeTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(NodeTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeTemplateStatus) DeepCopyInto(out *NodeTemplateStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]NodeTemplateCondition, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeTemplateStatus.
func (in *NodeTemplateStatus) DeepCopy() *NodeTemplateStatus {
	if in == nil {
		return nil
	}
	out := new(NodeTemplateStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Notification) DeepCopyInto(out *Notification) {
	*out = *in
	if in.SMTPConfig != nil {
		in, out := &in.SMTPConfig, &out.SMTPConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(SMTPConfig)
			**out = **in
		}
	}
	if in.SlackConfig != nil {
		in, out := &in.SlackConfig, &out.SlackConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(SlackConfig)
			**out = **in
		}
	}
	if in.PagerdutyConfig != nil {
		in, out := &in.PagerdutyConfig, &out.PagerdutyConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(PagerdutyConfig)
			**out = **in
		}
	}
	if in.WebhookConfig != nil {
		in, out := &in.WebhookConfig, &out.WebhookConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(WebhookConfig)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Notification.
func (in *Notification) DeepCopy() *Notification {
	if in == nil {
		return nil
	}
	out := new(Notification)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Notifier) DeepCopyInto(out *Notifier) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Notifier.
func (in *Notifier) DeepCopy() *Notifier {
	if in == nil {
		return nil
	}
	out := new(Notifier)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Notifier) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NotifierList) DeepCopyInto(out *NotifierList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Notifier, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NotifierList.
func (in *NotifierList) DeepCopy() *NotifierList {
	if in == nil {
		return nil
	}
	out := new(NotifierList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NotifierList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NotifierSpec) DeepCopyInto(out *NotifierSpec) {
	*out = *in
	if in.SMTPConfig != nil {
		in, out := &in.SMTPConfig, &out.SMTPConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(SMTPConfig)
			**out = **in
		}
	}
	if in.SlackConfig != nil {
		in, out := &in.SlackConfig, &out.SlackConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(SlackConfig)
			**out = **in
		}
	}
	if in.PagerdutyConfig != nil {
		in, out := &in.PagerdutyConfig, &out.PagerdutyConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(PagerdutyConfig)
			**out = **in
		}
	}
	if in.WebhookConfig != nil {
		in, out := &in.WebhookConfig, &out.WebhookConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(WebhookConfig)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NotifierSpec.
func (in *NotifierSpec) DeepCopy() *NotifierSpec {
	if in == nil {
		return nil
	}
	out := new(NotifierSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NotifierStatus) DeepCopyInto(out *NotifierStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NotifierStatus.
func (in *NotifierStatus) DeepCopy() *NotifierStatus {
	if in == nil {
		return nil
	}
	out := new(NotifierStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PagerdutyConfig) DeepCopyInto(out *PagerdutyConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PagerdutyConfig.
func (in *PagerdutyConfig) DeepCopy() *PagerdutyConfig {
	if in == nil {
		return nil
	}
	out := new(PagerdutyConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Pipeline) DeepCopyInto(out *Pipeline) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Pipeline.
func (in *Pipeline) DeepCopy() *Pipeline {
	if in == nil {
		return nil
	}
	out := new(Pipeline)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Pipeline) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineExecution) DeepCopyInto(out *PipelineExecution) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineExecution.
func (in *PipelineExecution) DeepCopy() *PipelineExecution {
	if in == nil {
		return nil
	}
	out := new(PipelineExecution)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineExecution) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineExecutionList) DeepCopyInto(out *PipelineExecutionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PipelineExecution, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineExecutionList.
func (in *PipelineExecutionList) DeepCopy() *PipelineExecutionList {
	if in == nil {
		return nil
	}
	out := new(PipelineExecutionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineExecutionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineExecutionLog) DeepCopyInto(out *PipelineExecutionLog) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineExecutionLog.
func (in *PipelineExecutionLog) DeepCopy() *PipelineExecutionLog {
	if in == nil {
		return nil
	}
	out := new(PipelineExecutionLog)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineExecutionLog) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineExecutionLogList) DeepCopyInto(out *PipelineExecutionLogList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PipelineExecutionLog, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineExecutionLogList.
func (in *PipelineExecutionLogList) DeepCopy() *PipelineExecutionLogList {
	if in == nil {
		return nil
	}
	out := new(PipelineExecutionLogList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineExecutionLogList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineExecutionLogSpec) DeepCopyInto(out *PipelineExecutionLogSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineExecutionLogSpec.
func (in *PipelineExecutionLogSpec) DeepCopy() *PipelineExecutionLogSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineExecutionLogSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineExecutionSpec) DeepCopyInto(out *PipelineExecutionSpec) {
	*out = *in
	in.Pipeline.DeepCopyInto(&out.Pipeline)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineExecutionSpec.
func (in *PipelineExecutionSpec) DeepCopy() *PipelineExecutionSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineExecutionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineExecutionStatus) DeepCopyInto(out *PipelineExecutionStatus) {
	*out = *in
	if in.Stages != nil {
		in, out := &in.Stages, &out.Stages
		*out = make([]StageStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineExecutionStatus.
func (in *PipelineExecutionStatus) DeepCopy() *PipelineExecutionStatus {
	if in == nil {
		return nil
	}
	out := new(PipelineExecutionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineList) DeepCopyInto(out *PipelineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Pipeline, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineList.
func (in *PipelineList) DeepCopy() *PipelineList {
	if in == nil {
		return nil
	}
	out := new(PipelineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PipelineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineSpec) DeepCopyInto(out *PipelineSpec) {
	*out = *in
	if in.Stages != nil {
		in, out := &in.Stages, &out.Stages
		*out = make([]Stage, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Templates != nil {
		in, out := &in.Templates, &out.Templates
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineSpec.
func (in *PipelineSpec) DeepCopy() *PipelineSpec {
	if in == nil {
		return nil
	}
	out := new(PipelineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PipelineStatus) DeepCopyInto(out *PipelineStatus) {
	*out = *in
	if in.SourceCodeCredential != nil {
		in, out := &in.SourceCodeCredential, &out.SourceCodeCredential
		if *in == nil {
			*out = nil
		} else {
			*out = new(SourceCodeCredential)
			(*in).DeepCopyInto(*out)
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PipelineStatus.
func (in *PipelineStatus) DeepCopy() *PipelineStatus {
	if in == nil {
		return nil
	}
	out := new(PipelineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodSecurityPolicyTemplate) DeepCopyInto(out *PodSecurityPolicyTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodSecurityPolicyTemplate.
func (in *PodSecurityPolicyTemplate) DeepCopy() *PodSecurityPolicyTemplate {
	if in == nil {
		return nil
	}
	out := new(PodSecurityPolicyTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PodSecurityPolicyTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodSecurityPolicyTemplateList) DeepCopyInto(out *PodSecurityPolicyTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PodSecurityPolicyTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodSecurityPolicyTemplateList.
func (in *PodSecurityPolicyTemplateList) DeepCopy() *PodSecurityPolicyTemplateList {
	if in == nil {
		return nil
	}
	out := new(PodSecurityPolicyTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PodSecurityPolicyTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PortCheck) DeepCopyInto(out *PortCheck) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PortCheck.
func (in *PortCheck) DeepCopy() *PortCheck {
	if in == nil {
		return nil
	}
	out := new(PortCheck)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Preference) DeepCopyInto(out *Preference) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Preference.
func (in *Preference) DeepCopy() *Preference {
	if in == nil {
		return nil
	}
	out := new(Preference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Preference) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PreferenceList) DeepCopyInto(out *PreferenceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Preference, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PreferenceList.
func (in *PreferenceList) DeepCopy() *PreferenceList {
	if in == nil {
		return nil
	}
	out := new(PreferenceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PreferenceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Principal) DeepCopyInto(out *Principal) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.ExtraInfo != nil {
		in, out := &in.ExtraInfo, &out.ExtraInfo
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Principal.
func (in *Principal) DeepCopy() *Principal {
	if in == nil {
		return nil
	}
	out := new(Principal)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Principal) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrincipalList) DeepCopyInto(out *PrincipalList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Principal, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrincipalList.
func (in *PrincipalList) DeepCopy() *PrincipalList {
	if in == nil {
		return nil
	}
	out := new(PrincipalList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PrincipalList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrivateRegistry) DeepCopyInto(out *PrivateRegistry) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrivateRegistry.
func (in *PrivateRegistry) DeepCopy() *PrivateRegistry {
	if in == nil {
		return nil
	}
	out := new(PrivateRegistry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Process) DeepCopyInto(out *Process) {
	*out = *in
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.VolumesFrom != nil {
		in, out := &in.VolumesFrom, &out.VolumesFrom
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Binds != nil {
		in, out := &in.Binds, &out.Binds
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	out.HealthCheck = in.HealthCheck
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Process.
func (in *Process) DeepCopy() *Process {
	if in == nil {
		return nil
	}
	out := new(Process)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Project) DeepCopyInto(out *Project) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Project.
func (in *Project) DeepCopy() *Project {
	if in == nil {
		return nil
	}
	out := new(Project)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Project) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectAlert) DeepCopyInto(out *ProjectAlert) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectAlert.
func (in *ProjectAlert) DeepCopy() *ProjectAlert {
	if in == nil {
		return nil
	}
	out := new(ProjectAlert)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectAlert) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectAlertList) DeepCopyInto(out *ProjectAlertList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ProjectAlert, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectAlertList.
func (in *ProjectAlertList) DeepCopy() *ProjectAlertList {
	if in == nil {
		return nil
	}
	out := new(ProjectAlertList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectAlertList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectAlertSpec) DeepCopyInto(out *ProjectAlertSpec) {
	*out = *in
	in.AlertCommonSpec.DeepCopyInto(&out.AlertCommonSpec)
	in.TargetWorkload.DeepCopyInto(&out.TargetWorkload)
	out.TargetPod = in.TargetPod
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectAlertSpec.
func (in *ProjectAlertSpec) DeepCopy() *ProjectAlertSpec {
	if in == nil {
		return nil
	}
	out := new(ProjectAlertSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectCondition) DeepCopyInto(out *ProjectCondition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectCondition.
func (in *ProjectCondition) DeepCopy() *ProjectCondition {
	if in == nil {
		return nil
	}
	out := new(ProjectCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectList) DeepCopyInto(out *ProjectList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Project, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectList.
func (in *ProjectList) DeepCopy() *ProjectList {
	if in == nil {
		return nil
	}
	out := new(ProjectList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectLogging) DeepCopyInto(out *ProjectLogging) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectLogging.
func (in *ProjectLogging) DeepCopy() *ProjectLogging {
	if in == nil {
		return nil
	}
	out := new(ProjectLogging)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectLogging) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectLoggingList) DeepCopyInto(out *ProjectLoggingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ProjectLogging, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectLoggingList.
func (in *ProjectLoggingList) DeepCopy() *ProjectLoggingList {
	if in == nil {
		return nil
	}
	out := new(ProjectLoggingList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectLoggingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectLoggingSpec) DeepCopyInto(out *ProjectLoggingSpec) {
	*out = *in
	in.LoggingCommonSpec.DeepCopyInto(&out.LoggingCommonSpec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectLoggingSpec.
func (in *ProjectLoggingSpec) DeepCopy() *ProjectLoggingSpec {
	if in == nil {
		return nil
	}
	out := new(ProjectLoggingSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectNetworkPolicy) DeepCopyInto(out *ProjectNetworkPolicy) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		if *in == nil {
			*out = nil
		} else {
			*out = new(ProjectNetworkPolicyStatus)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectNetworkPolicy.
func (in *ProjectNetworkPolicy) DeepCopy() *ProjectNetworkPolicy {
	if in == nil {
		return nil
	}
	out := new(ProjectNetworkPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectNetworkPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectNetworkPolicyList) DeepCopyInto(out *ProjectNetworkPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ProjectNetworkPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectNetworkPolicyList.
func (in *ProjectNetworkPolicyList) DeepCopy() *ProjectNetworkPolicyList {
	if in == nil {
		return nil
	}
	out := new(ProjectNetworkPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectNetworkPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectNetworkPolicySpec) DeepCopyInto(out *ProjectNetworkPolicySpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectNetworkPolicySpec.
func (in *ProjectNetworkPolicySpec) DeepCopy() *ProjectNetworkPolicySpec {
	if in == nil {
		return nil
	}
	out := new(ProjectNetworkPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectNetworkPolicyStatus) DeepCopyInto(out *ProjectNetworkPolicyStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectNetworkPolicyStatus.
func (in *ProjectNetworkPolicyStatus) DeepCopy() *ProjectNetworkPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(ProjectNetworkPolicyStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectRoleTemplateBinding) DeepCopyInto(out *ProjectRoleTemplateBinding) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectRoleTemplateBinding.
func (in *ProjectRoleTemplateBinding) DeepCopy() *ProjectRoleTemplateBinding {
	if in == nil {
		return nil
	}
	out := new(ProjectRoleTemplateBinding)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectRoleTemplateBinding) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectRoleTemplateBindingList) DeepCopyInto(out *ProjectRoleTemplateBindingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ProjectRoleTemplateBinding, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectRoleTemplateBindingList.
func (in *ProjectRoleTemplateBindingList) DeepCopy() *ProjectRoleTemplateBindingList {
	if in == nil {
		return nil
	}
	out := new(ProjectRoleTemplateBindingList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ProjectRoleTemplateBindingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectSpec) DeepCopyInto(out *ProjectSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectSpec.
func (in *ProjectSpec) DeepCopy() *ProjectSpec {
	if in == nil {
		return nil
	}
	out := new(ProjectSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectStatus) DeepCopyInto(out *ProjectStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]ProjectCondition, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectStatus.
func (in *ProjectStatus) DeepCopy() *ProjectStatus {
	if in == nil {
		return nil
	}
	out := new(ProjectStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PublishImageConfig) DeepCopyInto(out *PublishImageConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PublishImageConfig.
func (in *PublishImageConfig) DeepCopy() *PublishImageConfig {
	if in == nil {
		return nil
	}
	out := new(PublishImageConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Question) DeepCopyInto(out *Question) {
	*out = *in
	if in.Options != nil {
		in, out := &in.Options, &out.Options
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Question.
func (in *Question) DeepCopy() *Question {
	if in == nil {
		return nil
	}
	out := new(Question)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RKEConfigNode) DeepCopyInto(out *RKEConfigNode) {
	*out = *in
	if in.Role != nil {
		in, out := &in.Role, &out.Role
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RKEConfigNode.
func (in *RKEConfigNode) DeepCopy() *RKEConfigNode {
	if in == nil {
		return nil
	}
	out := new(RKEConfigNode)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RKEConfigNodePlan) DeepCopyInto(out *RKEConfigNodePlan) {
	*out = *in
	if in.Processes != nil {
		in, out := &in.Processes, &out.Processes
		*out = make(map[string]Process, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.PortChecks != nil {
		in, out := &in.PortChecks, &out.PortChecks
		*out = make([]PortCheck, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RKEConfigNodePlan.
func (in *RKEConfigNodePlan) DeepCopy() *RKEConfigNodePlan {
	if in == nil {
		return nil
	}
	out := new(RKEConfigNodePlan)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RKEConfigServices) DeepCopyInto(out *RKEConfigServices) {
	*out = *in
	in.Etcd.DeepCopyInto(&out.Etcd)
	in.KubeAPI.DeepCopyInto(&out.KubeAPI)
	in.KubeController.DeepCopyInto(&out.KubeController)
	in.Scheduler.DeepCopyInto(&out.Scheduler)
	in.Kubelet.DeepCopyInto(&out.Kubelet)
	in.Kubeproxy.DeepCopyInto(&out.Kubeproxy)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RKEConfigServices.
func (in *RKEConfigServices) DeepCopy() *RKEConfigServices {
	if in == nil {
		return nil
	}
	out := new(RKEConfigServices)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RKEPlan) DeepCopyInto(out *RKEPlan) {
	*out = *in
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = make([]RKEConfigNodePlan, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RKEPlan.
func (in *RKEPlan) DeepCopy() *RKEPlan {
	if in == nil {
		return nil
	}
	out := new(RKEPlan)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RKESystemImages) DeepCopyInto(out *RKESystemImages) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RKESystemImages.
func (in *RKESystemImages) DeepCopy() *RKESystemImages {
	if in == nil {
		return nil
	}
	out := new(RKESystemImages)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RancherKubernetesEngineConfig) DeepCopyInto(out *RancherKubernetesEngineConfig) {
	*out = *in
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = make([]RKEConfigNode, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Services.DeepCopyInto(&out.Services)
	in.Network.DeepCopyInto(&out.Network)
	in.Authentication.DeepCopyInto(&out.Authentication)
	out.SystemImages = in.SystemImages
	in.Authorization.DeepCopyInto(&out.Authorization)
	if in.PrivateRegistries != nil {
		in, out := &in.PrivateRegistries, &out.PrivateRegistries
		*out = make([]PrivateRegistry, len(*in))
		copy(*out, *in)
	}
	in.Ingress.DeepCopyInto(&out.Ingress)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RancherKubernetesEngineConfig.
func (in *RancherKubernetesEngineConfig) DeepCopy() *RancherKubernetesEngineConfig {
	if in == nil {
		return nil
	}
	out := new(RancherKubernetesEngineConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RancherSystemImages) DeepCopyInto(out *RancherSystemImages) {
	*out = *in
	out.RKESystemImages = in.RKESystemImages
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RancherSystemImages.
func (in *RancherSystemImages) DeepCopy() *RancherSystemImages {
	if in == nil {
		return nil
	}
	out := new(RancherSystemImages)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Recipient) DeepCopyInto(out *Recipient) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Recipient.
func (in *Recipient) DeepCopy() *Recipient {
	if in == nil {
		return nil
	}
	out := new(Recipient)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RepoPerm) DeepCopyInto(out *RepoPerm) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RepoPerm.
func (in *RepoPerm) DeepCopy() *RepoPerm {
	if in == nil {
		return nil
	}
	out := new(RepoPerm)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RoleTemplate) DeepCopyInto(out *RoleTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]rbac_v1.PolicyRule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.RoleTemplateNames != nil {
		in, out := &in.RoleTemplateNames, &out.RoleTemplateNames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RoleTemplate.
func (in *RoleTemplate) DeepCopy() *RoleTemplate {
	if in == nil {
		return nil
	}
	out := new(RoleTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RoleTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RoleTemplateList) DeepCopyInto(out *RoleTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]RoleTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RoleTemplateList.
func (in *RoleTemplateList) DeepCopy() *RoleTemplateList {
	if in == nil {
		return nil
	}
	out := new(RoleTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RoleTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RunPipelineInput) DeepCopyInto(out *RunPipelineInput) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RunPipelineInput.
func (in *RunPipelineInput) DeepCopy() *RunPipelineInput {
	if in == nil {
		return nil
	}
	out := new(RunPipelineInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RunScriptConfig) DeepCopyInto(out *RunScriptConfig) {
	*out = *in
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RunScriptConfig.
func (in *RunScriptConfig) DeepCopy() *RunScriptConfig {
	if in == nil {
		return nil
	}
	out := new(RunScriptConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SMTPConfig) DeepCopyInto(out *SMTPConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SMTPConfig.
func (in *SMTPConfig) DeepCopy() *SMTPConfig {
	if in == nil {
		return nil
	}
	out := new(SMTPConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SchedulerService) DeepCopyInto(out *SchedulerService) {
	*out = *in
	in.BaseService.DeepCopyInto(&out.BaseService)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SchedulerService.
func (in *SchedulerService) DeepCopy() *SchedulerService {
	if in == nil {
		return nil
	}
	out := new(SchedulerService)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SearchPrincipalsInput) DeepCopyInto(out *SearchPrincipalsInput) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SearchPrincipalsInput.
func (in *SearchPrincipalsInput) DeepCopy() *SearchPrincipalsInput {
	if in == nil {
		return nil
	}
	out := new(SearchPrincipalsInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SetPasswordInput) DeepCopyInto(out *SetPasswordInput) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SetPasswordInput.
func (in *SetPasswordInput) DeepCopy() *SetPasswordInput {
	if in == nil {
		return nil
	}
	out := new(SetPasswordInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Setting) DeepCopyInto(out *Setting) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Setting.
func (in *Setting) DeepCopy() *Setting {
	if in == nil {
		return nil
	}
	out := new(Setting)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Setting) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SettingList) DeepCopyInto(out *SettingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Setting, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SettingList.
func (in *SettingList) DeepCopy() *SettingList {
	if in == nil {
		return nil
	}
	out := new(SettingList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SettingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SlackConfig) DeepCopyInto(out *SlackConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SlackConfig.
func (in *SlackConfig) DeepCopy() *SlackConfig {
	if in == nil {
		return nil
	}
	out := new(SlackConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SourceCodeConfig) DeepCopyInto(out *SourceCodeConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SourceCodeConfig.
func (in *SourceCodeConfig) DeepCopy() *SourceCodeConfig {
	if in == nil {
		return nil
	}
	out := new(SourceCodeConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SourceCodeCredential) DeepCopyInto(out *SourceCodeCredential) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SourceCodeCredential.
func (in *SourceCodeCredential) DeepCopy() *SourceCodeCredential {
	if in == nil {
		return nil
	}
	out := new(SourceCodeCredential)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SourceCodeCredential) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SourceCodeCredentialList) DeepCopyInto(out *SourceCodeCredentialList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SourceCodeCredential, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SourceCodeCredentialList.
func (in *SourceCodeCredentialList) DeepCopy() *SourceCodeCredentialList {
	if in == nil {
		return nil
	}
	out := new(SourceCodeCredentialList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SourceCodeCredentialList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SourceCodeCredentialSpec) DeepCopyInto(out *SourceCodeCredentialSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SourceCodeCredentialSpec.
func (in *SourceCodeCredentialSpec) DeepCopy() *SourceCodeCredentialSpec {
	if in == nil {
		return nil
	}
	out := new(SourceCodeCredentialSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SourceCodeCredentialStatus) DeepCopyInto(out *SourceCodeCredentialStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SourceCodeCredentialStatus.
func (in *SourceCodeCredentialStatus) DeepCopy() *SourceCodeCredentialStatus {
	if in == nil {
		return nil
	}
	out := new(SourceCodeCredentialStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SourceCodeRepository) DeepCopyInto(out *SourceCodeRepository) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SourceCodeRepository.
func (in *SourceCodeRepository) DeepCopy() *SourceCodeRepository {
	if in == nil {
		return nil
	}
	out := new(SourceCodeRepository)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SourceCodeRepository) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SourceCodeRepositoryList) DeepCopyInto(out *SourceCodeRepositoryList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SourceCodeRepository, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SourceCodeRepositoryList.
func (in *SourceCodeRepositoryList) DeepCopy() *SourceCodeRepositoryList {
	if in == nil {
		return nil
	}
	out := new(SourceCodeRepositoryList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SourceCodeRepositoryList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SourceCodeRepositorySpec) DeepCopyInto(out *SourceCodeRepositorySpec) {
	*out = *in
	out.Permissions = in.Permissions
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SourceCodeRepositorySpec.
func (in *SourceCodeRepositorySpec) DeepCopy() *SourceCodeRepositorySpec {
	if in == nil {
		return nil
	}
	out := new(SourceCodeRepositorySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SourceCodeRepositoryStatus) DeepCopyInto(out *SourceCodeRepositoryStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SourceCodeRepositoryStatus.
func (in *SourceCodeRepositoryStatus) DeepCopy() *SourceCodeRepositoryStatus {
	if in == nil {
		return nil
	}
	out := new(SourceCodeRepositoryStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SplunkConfig) DeepCopyInto(out *SplunkConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SplunkConfig.
func (in *SplunkConfig) DeepCopy() *SplunkConfig {
	if in == nil {
		return nil
	}
	out := new(SplunkConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Stage) DeepCopyInto(out *Stage) {
	*out = *in
	if in.Steps != nil {
		in, out := &in.Steps, &out.Steps
		*out = make([]Step, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Stage.
func (in *Stage) DeepCopy() *Stage {
	if in == nil {
		return nil
	}
	out := new(Stage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StageStatus) DeepCopyInto(out *StageStatus) {
	*out = *in
	if in.Steps != nil {
		in, out := &in.Steps, &out.Steps
		*out = make([]StepStatus, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StageStatus.
func (in *StageStatus) DeepCopy() *StageStatus {
	if in == nil {
		return nil
	}
	out := new(StageStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Step) DeepCopyInto(out *Step) {
	*out = *in
	if in.SourceCodeConfig != nil {
		in, out := &in.SourceCodeConfig, &out.SourceCodeConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(SourceCodeConfig)
			**out = **in
		}
	}
	if in.RunScriptConfig != nil {
		in, out := &in.RunScriptConfig, &out.RunScriptConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(RunScriptConfig)
			(*in).DeepCopyInto(*out)
		}
	}
	if in.PublishImageConfig != nil {
		in, out := &in.PublishImageConfig, &out.PublishImageConfig
		if *in == nil {
			*out = nil
		} else {
			*out = new(PublishImageConfig)
			**out = **in
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Step.
func (in *Step) DeepCopy() *Step {
	if in == nil {
		return nil
	}
	out := new(Step)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StepStatus) DeepCopyInto(out *StepStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StepStatus.
func (in *StepStatus) DeepCopy() *StepStatus {
	if in == nil {
		return nil
	}
	out := new(StepStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SyslogConfig) DeepCopyInto(out *SyslogConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SyslogConfig.
func (in *SyslogConfig) DeepCopy() *SyslogConfig {
	if in == nil {
		return nil
	}
	out := new(SyslogConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TargetEvent) DeepCopyInto(out *TargetEvent) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TargetEvent.
func (in *TargetEvent) DeepCopy() *TargetEvent {
	if in == nil {
		return nil
	}
	out := new(TargetEvent)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TargetNode) DeepCopyInto(out *TargetNode) {
	*out = *in
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TargetNode.
func (in *TargetNode) DeepCopy() *TargetNode {
	if in == nil {
		return nil
	}
	out := new(TargetNode)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TargetPod) DeepCopyInto(out *TargetPod) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TargetPod.
func (in *TargetPod) DeepCopy() *TargetPod {
	if in == nil {
		return nil
	}
	out := new(TargetPod)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TargetSystemService) DeepCopyInto(out *TargetSystemService) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TargetSystemService.
func (in *TargetSystemService) DeepCopy() *TargetSystemService {
	if in == nil {
		return nil
	}
	out := new(TargetSystemService)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TargetWorkload) DeepCopyInto(out *TargetWorkload) {
	*out = *in
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TargetWorkload.
func (in *TargetWorkload) DeepCopy() *TargetWorkload {
	if in == nil {
		return nil
	}
	out := new(TargetWorkload)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Template) DeepCopyInto(out *Template) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Template.
func (in *Template) DeepCopy() *Template {
	if in == nil {
		return nil
	}
	out := new(Template)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Template) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TemplateList) DeepCopyInto(out *TemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Template, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TemplateList.
func (in *TemplateList) DeepCopy() *TemplateList {
	if in == nil {
		return nil
	}
	out := new(TemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TemplateSpec) DeepCopyInto(out *TemplateSpec) {
	*out = *in
	if in.Categories != nil {
		in, out := &in.Categories, &out.Categories
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Versions != nil {
		in, out := &in.Versions, &out.Versions
		*out = make([]TemplateVersionSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TemplateSpec.
func (in *TemplateSpec) DeepCopy() *TemplateSpec {
	if in == nil {
		return nil
	}
	out := new(TemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TemplateStatus) DeepCopyInto(out *TemplateStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TemplateStatus.
func (in *TemplateStatus) DeepCopy() *TemplateStatus {
	if in == nil {
		return nil
	}
	out := new(TemplateStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TemplateVersion) DeepCopyInto(out *TemplateVersion) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TemplateVersion.
func (in *TemplateVersion) DeepCopy() *TemplateVersion {
	if in == nil {
		return nil
	}
	out := new(TemplateVersion)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TemplateVersion) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TemplateVersionList) DeepCopyInto(out *TemplateVersionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]TemplateVersion, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TemplateVersionList.
func (in *TemplateVersionList) DeepCopy() *TemplateVersionList {
	if in == nil {
		return nil
	}
	out := new(TemplateVersionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TemplateVersionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TemplateVersionSpec) DeepCopyInto(out *TemplateVersionSpec) {
	*out = *in
	if in.Revision != nil {
		in, out := &in.Revision, &out.Revision
		if *in == nil {
			*out = nil
		} else {
			*out = new(int)
			**out = **in
		}
	}
	if in.UpgradeVersionLinks != nil {
		in, out := &in.UpgradeVersionLinks, &out.UpgradeVersionLinks
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Files != nil {
		in, out := &in.Files, &out.Files
		*out = make([]File, len(*in))
		copy(*out, *in)
	}
	if in.Questions != nil {
		in, out := &in.Questions, &out.Questions
		*out = make([]Question, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TemplateVersionSpec.
func (in *TemplateVersionSpec) DeepCopy() *TemplateVersionSpec {
	if in == nil {
		return nil
	}
	out := new(TemplateVersionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TemplateVersionStatus) DeepCopyInto(out *TemplateVersionStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TemplateVersionStatus.
func (in *TemplateVersionStatus) DeepCopy() *TemplateVersionStatus {
	if in == nil {
		return nil
	}
	out := new(TemplateVersionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Token) DeepCopyInto(out *Token) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.UserPrincipal.DeepCopyInto(&out.UserPrincipal)
	if in.GroupPrincipals != nil {
		in, out := &in.GroupPrincipals, &out.GroupPrincipals
		*out = make([]Principal, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ProviderInfo != nil {
		in, out := &in.ProviderInfo, &out.ProviderInfo
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Token.
func (in *Token) DeepCopy() *Token {
	if in == nil {
		return nil
	}
	out := new(Token)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Token) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TokenList) DeepCopyInto(out *TokenList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Token, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TokenList.
func (in *TokenList) DeepCopy() *TokenList {
	if in == nil {
		return nil
	}
	out := new(TokenList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TokenList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *User) DeepCopyInto(out *User) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.PrincipalIDs != nil {
		in, out := &in.PrincipalIDs, &out.PrincipalIDs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new User.
func (in *User) DeepCopy() *User {
	if in == nil {
		return nil
	}
	out := new(User)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *User) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UserList) DeepCopyInto(out *UserList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]User, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UserList.
func (in *UserList) DeepCopy() *UserList {
	if in == nil {
		return nil
	}
	out := new(UserList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *UserList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Values) DeepCopyInto(out *Values) {
	*out = *in
	if in.StringSliceValue != nil {
		in, out := &in.StringSliceValue, &out.StringSliceValue
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Values.
func (in *Values) DeepCopy() *Values {
	if in == nil {
		return nil
	}
	out := new(Values)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VersionCommits) DeepCopyInto(out *VersionCommits) {
	*out = *in
	if in.Value != nil {
		in, out := &in.Value, &out.Value
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VersionCommits.
func (in *VersionCommits) DeepCopy() *VersionCommits {
	if in == nil {
		return nil
	}
	out := new(VersionCommits)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WebhookConfig) DeepCopyInto(out *WebhookConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WebhookConfig.
func (in *WebhookConfig) DeepCopy() *WebhookConfig {
	if in == nil {
		return nil
	}
	out := new(WebhookConfig)
	in.DeepCopyInto(out)
	return out
}
