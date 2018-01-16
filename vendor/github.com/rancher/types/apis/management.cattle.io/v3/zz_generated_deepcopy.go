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
			in.(*ClusterComponentStatus).DeepCopyInto(out.(*ClusterComponentStatus))
			return nil
		}, InType: reflect.TypeOf(&ClusterComponentStatus{})},
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
			in.(*GithubCredential).DeepCopyInto(out.(*GithubCredential))
			return nil
		}, InType: reflect.TypeOf(&GithubCredential{})},
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
			in.(*LocalCredential).DeepCopyInto(out.(*LocalCredential))
			return nil
		}, InType: reflect.TypeOf(&LocalCredential{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*LoginInput).DeepCopyInto(out.(*LoginInput))
			return nil
		}, InType: reflect.TypeOf(&LoginInput{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Machine).DeepCopyInto(out.(*Machine))
			return nil
		}, InType: reflect.TypeOf(&Machine{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineCommonParams).DeepCopyInto(out.(*MachineCommonParams))
			return nil
		}, InType: reflect.TypeOf(&MachineCommonParams{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineCondition).DeepCopyInto(out.(*MachineCondition))
			return nil
		}, InType: reflect.TypeOf(&MachineCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineConfig).DeepCopyInto(out.(*MachineConfig))
			return nil
		}, InType: reflect.TypeOf(&MachineConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineDriver).DeepCopyInto(out.(*MachineDriver))
			return nil
		}, InType: reflect.TypeOf(&MachineDriver{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineDriverCondition).DeepCopyInto(out.(*MachineDriverCondition))
			return nil
		}, InType: reflect.TypeOf(&MachineDriverCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineDriverList).DeepCopyInto(out.(*MachineDriverList))
			return nil
		}, InType: reflect.TypeOf(&MachineDriverList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineDriverSpec).DeepCopyInto(out.(*MachineDriverSpec))
			return nil
		}, InType: reflect.TypeOf(&MachineDriverSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineDriverStatus).DeepCopyInto(out.(*MachineDriverStatus))
			return nil
		}, InType: reflect.TypeOf(&MachineDriverStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineList).DeepCopyInto(out.(*MachineList))
			return nil
		}, InType: reflect.TypeOf(&MachineList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineSpec).DeepCopyInto(out.(*MachineSpec))
			return nil
		}, InType: reflect.TypeOf(&MachineSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineStatus).DeepCopyInto(out.(*MachineStatus))
			return nil
		}, InType: reflect.TypeOf(&MachineStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineTemplate).DeepCopyInto(out.(*MachineTemplate))
			return nil
		}, InType: reflect.TypeOf(&MachineTemplate{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineTemplateCondition).DeepCopyInto(out.(*MachineTemplateCondition))
			return nil
		}, InType: reflect.TypeOf(&MachineTemplateCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineTemplateList).DeepCopyInto(out.(*MachineTemplateList))
			return nil
		}, InType: reflect.TypeOf(&MachineTemplateList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineTemplateSpec).DeepCopyInto(out.(*MachineTemplateSpec))
			return nil
		}, InType: reflect.TypeOf(&MachineTemplateSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*MachineTemplateStatus).DeepCopyInto(out.(*MachineTemplateStatus))
			return nil
		}, InType: reflect.TypeOf(&MachineTemplateStatus{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*NetworkConfig).DeepCopyInto(out.(*NetworkConfig))
			return nil
		}, InType: reflect.TypeOf(&NetworkConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PodSecurityPolicyTemplate).DeepCopyInto(out.(*PodSecurityPolicyTemplate))
			return nil
		}, InType: reflect.TypeOf(&PodSecurityPolicyTemplate{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PodSecurityPolicyTemplateList).DeepCopyInto(out.(*PodSecurityPolicyTemplateList))
			return nil
		}, InType: reflect.TypeOf(&PodSecurityPolicyTemplateList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Principal).DeepCopyInto(out.(*Principal))
			return nil
		}, InType: reflect.TypeOf(&Principal{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*PrincipalList).DeepCopyInto(out.(*PrincipalList))
			return nil
		}, InType: reflect.TypeOf(&PrincipalList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Project).DeepCopyInto(out.(*Project))
			return nil
		}, InType: reflect.TypeOf(&Project{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectCondition).DeepCopyInto(out.(*ProjectCondition))
			return nil
		}, InType: reflect.TypeOf(&ProjectCondition{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ProjectList).DeepCopyInto(out.(*ProjectList))
			return nil
		}, InType: reflect.TypeOf(&ProjectList{})},
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
			in.(*Question).DeepCopyInto(out.(*Question))
			return nil
		}, InType: reflect.TypeOf(&Question{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RKEConfigNode).DeepCopyInto(out.(*RKEConfigNode))
			return nil
		}, InType: reflect.TypeOf(&RKEConfigNode{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RKEConfigServices).DeepCopyInto(out.(*RKEConfigServices))
			return nil
		}, InType: reflect.TypeOf(&RKEConfigServices{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RancherKubernetesEngineConfig).DeepCopyInto(out.(*RancherKubernetesEngineConfig))
			return nil
		}, InType: reflect.TypeOf(&RancherKubernetesEngineConfig{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*ReleaseInfo).DeepCopyInto(out.(*ReleaseInfo))
			return nil
		}, InType: reflect.TypeOf(&ReleaseInfo{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RoleTemplate).DeepCopyInto(out.(*RoleTemplate))
			return nil
		}, InType: reflect.TypeOf(&RoleTemplate{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*RoleTemplateList).DeepCopyInto(out.(*RoleTemplateList))
			return nil
		}, InType: reflect.TypeOf(&RoleTemplateList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*SchedulerService).DeepCopyInto(out.(*SchedulerService))
			return nil
		}, InType: reflect.TypeOf(&SchedulerService{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*Stack).DeepCopyInto(out.(*Stack))
			return nil
		}, InType: reflect.TypeOf(&Stack{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*StackList).DeepCopyInto(out.(*StackList))
			return nil
		}, InType: reflect.TypeOf(&StackList{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*StackSpec).DeepCopyInto(out.(*StackSpec))
			return nil
		}, InType: reflect.TypeOf(&StackSpec{})},
		conversion.GeneratedDeepCopyFunc{Fn: func(in interface{}, out interface{}, c *conversion.Cloner) error {
			in.(*StackStatus).DeepCopyInto(out.(*StackStatus))
			return nil
		}, InType: reflect.TypeOf(&StackStatus{})},
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
	out.Status = in.Status
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
	out.Subject = in.Subject
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
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = make([]MachineConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
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
			**out = **in
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
func (in *GithubCredential) DeepCopyInto(out *GithubCredential) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GithubCredential.
func (in *GithubCredential) DeepCopy() *GithubCredential {
	if in == nil {
		return nil
	}
	out := new(GithubCredential)
	in.DeepCopyInto(out)
	return out
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
	out.Subject = in.Subject
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
func (in *LocalCredential) DeepCopyInto(out *LocalCredential) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LocalCredential.
func (in *LocalCredential) DeepCopy() *LocalCredential {
	if in == nil {
		return nil
	}
	out := new(LocalCredential)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LoginInput) DeepCopyInto(out *LoginInput) {
	*out = *in
	out.LocalCredential = in.LocalCredential
	out.GithubCredential = in.GithubCredential
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LoginInput.
func (in *LoginInput) DeepCopy() *LoginInput {
	if in == nil {
		return nil
	}
	out := new(LoginInput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Machine) DeepCopyInto(out *Machine) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Machine.
func (in *Machine) DeepCopy() *Machine {
	if in == nil {
		return nil
	}
	out := new(Machine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Machine) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineCommonParams) DeepCopyInto(out *MachineCommonParams) {
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

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineCommonParams.
func (in *MachineCommonParams) DeepCopy() *MachineCommonParams {
	if in == nil {
		return nil
	}
	out := new(MachineCommonParams)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineCondition) DeepCopyInto(out *MachineCondition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineCondition.
func (in *MachineCondition) DeepCopy() *MachineCondition {
	if in == nil {
		return nil
	}
	out := new(MachineCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineConfig) DeepCopyInto(out *MachineConfig) {
	*out = *in
	in.MachineSpec.DeepCopyInto(&out.MachineSpec)
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineConfig.
func (in *MachineConfig) DeepCopy() *MachineConfig {
	if in == nil {
		return nil
	}
	out := new(MachineConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineDriver) DeepCopyInto(out *MachineDriver) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineDriver.
func (in *MachineDriver) DeepCopy() *MachineDriver {
	if in == nil {
		return nil
	}
	out := new(MachineDriver)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MachineDriver) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineDriverCondition) DeepCopyInto(out *MachineDriverCondition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineDriverCondition.
func (in *MachineDriverCondition) DeepCopy() *MachineDriverCondition {
	if in == nil {
		return nil
	}
	out := new(MachineDriverCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineDriverList) DeepCopyInto(out *MachineDriverList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MachineDriver, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineDriverList.
func (in *MachineDriverList) DeepCopy() *MachineDriverList {
	if in == nil {
		return nil
	}
	out := new(MachineDriverList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MachineDriverList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineDriverSpec) DeepCopyInto(out *MachineDriverSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineDriverSpec.
func (in *MachineDriverSpec) DeepCopy() *MachineDriverSpec {
	if in == nil {
		return nil
	}
	out := new(MachineDriverSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineDriverStatus) DeepCopyInto(out *MachineDriverStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]MachineDriverCondition, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineDriverStatus.
func (in *MachineDriverStatus) DeepCopy() *MachineDriverStatus {
	if in == nil {
		return nil
	}
	out := new(MachineDriverStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineList) DeepCopyInto(out *MachineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Machine, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineList.
func (in *MachineList) DeepCopy() *MachineList {
	if in == nil {
		return nil
	}
	out := new(MachineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MachineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineSpec) DeepCopyInto(out *MachineSpec) {
	*out = *in
	in.NodeSpec.DeepCopyInto(&out.NodeSpec)
	if in.Role != nil {
		in, out := &in.Role, &out.Role
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineSpec.
func (in *MachineSpec) DeepCopy() *MachineSpec {
	if in == nil {
		return nil
	}
	out := new(MachineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineStatus) DeepCopyInto(out *MachineStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]MachineCondition, len(*in))
		copy(*out, *in)
	}
	in.NodeStatus.DeepCopyInto(&out.NodeStatus)
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
	if in.MachineTemplateSpec != nil {
		in, out := &in.MachineTemplateSpec, &out.MachineTemplateSpec
		if *in == nil {
			*out = nil
		} else {
			*out = new(MachineTemplateSpec)
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
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineStatus.
func (in *MachineStatus) DeepCopy() *MachineStatus {
	if in == nil {
		return nil
	}
	out := new(MachineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineTemplate) DeepCopyInto(out *MachineTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineTemplate.
func (in *MachineTemplate) DeepCopy() *MachineTemplate {
	if in == nil {
		return nil
	}
	out := new(MachineTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MachineTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineTemplateCondition) DeepCopyInto(out *MachineTemplateCondition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineTemplateCondition.
func (in *MachineTemplateCondition) DeepCopy() *MachineTemplateCondition {
	if in == nil {
		return nil
	}
	out := new(MachineTemplateCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineTemplateList) DeepCopyInto(out *MachineTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MachineTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineTemplateList.
func (in *MachineTemplateList) DeepCopy() *MachineTemplateList {
	if in == nil {
		return nil
	}
	out := new(MachineTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MachineTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineTemplateSpec) DeepCopyInto(out *MachineTemplateSpec) {
	*out = *in
	in.MachineCommonParams.DeepCopyInto(&out.MachineCommonParams)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineTemplateSpec.
func (in *MachineTemplateSpec) DeepCopy() *MachineTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(MachineTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineTemplateStatus) DeepCopyInto(out *MachineTemplateStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]MachineTemplateCondition, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineTemplateStatus.
func (in *MachineTemplateStatus) DeepCopy() *MachineTemplateStatus {
	if in == nil {
		return nil
	}
	out := new(MachineTemplateStatus)
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
func (in *ProjectRoleTemplateBinding) DeepCopyInto(out *ProjectRoleTemplateBinding) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Subject = in.Subject
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
	if in.SystemImages != nil {
		in, out := &in.SystemImages, &out.SystemImages
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.Authorization.DeepCopyInto(&out.Authorization)
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
func (in *ReleaseInfo) DeepCopyInto(out *ReleaseInfo) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReleaseInfo.
func (in *ReleaseInfo) DeepCopy() *ReleaseInfo {
	if in == nil {
		return nil
	}
	out := new(ReleaseInfo)
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
func (in *Stack) DeepCopyInto(out *Stack) {
	*out = *in
	out.Namespaced = in.Namespaced
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Stack.
func (in *Stack) DeepCopy() *Stack {
	if in == nil {
		return nil
	}
	out := new(Stack)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Stack) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StackList) DeepCopyInto(out *StackList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Stack, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StackList.
func (in *StackList) DeepCopy() *StackList {
	if in == nil {
		return nil
	}
	out := new(StackList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *StackList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StackSpec) DeepCopyInto(out *StackSpec) {
	*out = *in
	if in.Templates != nil {
		in, out := &in.Templates, &out.Templates
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Answers != nil {
		in, out := &in.Answers, &out.Answers
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Tag != nil {
		in, out := &in.Tag, &out.Tag
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Groups != nil {
		in, out := &in.Groups, &out.Groups
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StackSpec.
func (in *StackSpec) DeepCopy() *StackSpec {
	if in == nil {
		return nil
	}
	out := new(StackSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StackStatus) DeepCopyInto(out *StackStatus) {
	*out = *in
	if in.Releases != nil {
		in, out := &in.Releases, &out.Releases
		*out = make([]ReleaseInfo, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StackStatus.
func (in *StackStatus) DeepCopy() *StackStatus {
	if in == nil {
		return nil
	}
	out := new(StackStatus)
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
