/*
Copyright 2024 Rancher Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by main. DO NOT EDIT.

// +k8s:deepcopy-gen=package
// +groupName=management.cattle.io
package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// APIServiceList is a list of APIService resources
type APIServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []APIService `json:"items"`
}

func NewAPIService(namespace, name string, obj APIService) *APIService {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("APIService").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ActiveDirectoryProviderList is a list of ActiveDirectoryProvider resources
type ActiveDirectoryProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ActiveDirectoryProvider `json:"items"`
}

func NewActiveDirectoryProvider(namespace, name string, obj ActiveDirectoryProvider) *ActiveDirectoryProvider {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ActiveDirectoryProvider").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuthConfigList is a list of AuthConfig resources
type AuthConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AuthConfig `json:"items"`
}

func NewAuthConfig(namespace, name string, obj AuthConfig) *AuthConfig {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("AuthConfig").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuthProviderList is a list of AuthProvider resources
type AuthProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AuthProvider `json:"items"`
}

func NewAuthProvider(namespace, name string, obj AuthProvider) *AuthProvider {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("AuthProvider").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuthTokenList is a list of AuthToken resources
type AuthTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AuthToken `json:"items"`
}

func NewAuthToken(namespace, name string, obj AuthToken) *AuthToken {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("AuthToken").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AzureADProviderList is a list of AzureADProvider resources
type AzureADProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AzureADProvider `json:"items"`
}

func NewAzureADProvider(namespace, name string, obj AzureADProvider) *AzureADProvider {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("AzureADProvider").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CatalogList is a list of Catalog resources
type CatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Catalog `json:"items"`
}

func NewCatalog(namespace, name string, obj Catalog) *Catalog {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Catalog").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CatalogTemplateList is a list of CatalogTemplate resources
type CatalogTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CatalogTemplate `json:"items"`
}

func NewCatalogTemplate(namespace, name string, obj CatalogTemplate) *CatalogTemplate {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("CatalogTemplate").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CatalogTemplateVersionList is a list of CatalogTemplateVersion resources
type CatalogTemplateVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CatalogTemplateVersion `json:"items"`
}

func NewCatalogTemplateVersion(namespace, name string, obj CatalogTemplateVersion) *CatalogTemplateVersion {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("CatalogTemplateVersion").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudCredentialList is a list of CloudCredential resources
type CloudCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CloudCredential `json:"items"`
}

func NewCloudCredential(namespace, name string, obj CloudCredential) *CloudCredential {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("CloudCredential").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterList is a list of Cluster resources
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Cluster `json:"items"`
}

func NewCluster(namespace, name string, obj Cluster) *Cluster {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Cluster").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterAlertList is a list of ClusterAlert resources
type ClusterAlertList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterAlert `json:"items"`
}

func NewClusterAlert(namespace, name string, obj ClusterAlert) *ClusterAlert {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ClusterAlert").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterAlertGroupList is a list of ClusterAlertGroup resources
type ClusterAlertGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterAlertGroup `json:"items"`
}

func NewClusterAlertGroup(namespace, name string, obj ClusterAlertGroup) *ClusterAlertGroup {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ClusterAlertGroup").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterAlertRuleList is a list of ClusterAlertRule resources
type ClusterAlertRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterAlertRule `json:"items"`
}

func NewClusterAlertRule(namespace, name string, obj ClusterAlertRule) *ClusterAlertRule {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ClusterAlertRule").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterLoggingList is a list of ClusterLogging resources
type ClusterLoggingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterLogging `json:"items"`
}

func NewClusterLogging(namespace, name string, obj ClusterLogging) *ClusterLogging {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ClusterLogging").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterMonitorGraphList is a list of ClusterMonitorGraph resources
type ClusterMonitorGraphList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterMonitorGraph `json:"items"`
}

func NewClusterMonitorGraph(namespace, name string, obj ClusterMonitorGraph) *ClusterMonitorGraph {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ClusterMonitorGraph").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterRegistrationTokenList is a list of ClusterRegistrationToken resources
type ClusterRegistrationTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterRegistrationToken `json:"items"`
}

func NewClusterRegistrationToken(namespace, name string, obj ClusterRegistrationToken) *ClusterRegistrationToken {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ClusterRegistrationToken").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterRoleTemplateBindingList is a list of ClusterRoleTemplateBinding resources
type ClusterRoleTemplateBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterRoleTemplateBinding `json:"items"`
}

func NewClusterRoleTemplateBinding(namespace, name string, obj ClusterRoleTemplateBinding) *ClusterRoleTemplateBinding {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ClusterRoleTemplateBinding").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterTemplateList is a list of ClusterTemplate resources
type ClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterTemplate `json:"items"`
}

func NewClusterTemplate(namespace, name string, obj ClusterTemplate) *ClusterTemplate {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ClusterTemplate").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterTemplateRevisionList is a list of ClusterTemplateRevision resources
type ClusterTemplateRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterTemplateRevision `json:"items"`
}

func NewClusterTemplateRevision(namespace, name string, obj ClusterTemplateRevision) *ClusterTemplateRevision {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ClusterTemplateRevision").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComposeConfigList is a list of ComposeConfig resources
type ComposeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ComposeConfig `json:"items"`
}

func NewComposeConfig(namespace, name string, obj ComposeConfig) *ComposeConfig {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ComposeConfig").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DynamicSchemaList is a list of DynamicSchema resources
type DynamicSchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []DynamicSchema `json:"items"`
}

func NewDynamicSchema(namespace, name string, obj DynamicSchema) *DynamicSchema {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("DynamicSchema").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EtcdBackupList is a list of EtcdBackup resources
type EtcdBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []EtcdBackup `json:"items"`
}

func NewEtcdBackup(namespace, name string, obj EtcdBackup) *EtcdBackup {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("EtcdBackup").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FeatureList is a list of Feature resources
type FeatureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Feature `json:"items"`
}

func NewFeature(namespace, name string, obj Feature) *Feature {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Feature").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FleetWorkspaceList is a list of FleetWorkspace resources
type FleetWorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []FleetWorkspace `json:"items"`
}

func NewFleetWorkspace(namespace, name string, obj FleetWorkspace) *FleetWorkspace {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("FleetWorkspace").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FreeIpaProviderList is a list of FreeIpaProvider resources
type FreeIpaProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []FreeIpaProvider `json:"items"`
}

func NewFreeIpaProvider(namespace, name string, obj FreeIpaProvider) *FreeIpaProvider {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("FreeIpaProvider").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GithubProviderList is a list of GithubProvider resources
type GithubProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GithubProvider `json:"items"`
}

func NewGithubProvider(namespace, name string, obj GithubProvider) *GithubProvider {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("GithubProvider").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalDnsList is a list of GlobalDns resources
type GlobalDnsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GlobalDns `json:"items"`
}

func NewGlobalDns(namespace, name string, obj GlobalDns) *GlobalDns {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("GlobalDns").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalDnsProviderList is a list of GlobalDnsProvider resources
type GlobalDnsProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GlobalDnsProvider `json:"items"`
}

func NewGlobalDnsProvider(namespace, name string, obj GlobalDnsProvider) *GlobalDnsProvider {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("GlobalDnsProvider").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalRoleList is a list of GlobalRole resources
type GlobalRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GlobalRole `json:"items"`
}

func NewGlobalRole(namespace, name string, obj GlobalRole) *GlobalRole {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("GlobalRole").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalRoleBindingList is a list of GlobalRoleBinding resources
type GlobalRoleBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GlobalRoleBinding `json:"items"`
}

func NewGlobalRoleBinding(namespace, name string, obj GlobalRoleBinding) *GlobalRoleBinding {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("GlobalRoleBinding").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GoogleOAuthProviderList is a list of GoogleOAuthProvider resources
type GoogleOAuthProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GoogleOAuthProvider `json:"items"`
}

func NewGoogleOAuthProvider(namespace, name string, obj GoogleOAuthProvider) *GoogleOAuthProvider {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("GoogleOAuthProvider").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GroupList is a list of Group resources
type GroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Group `json:"items"`
}

func NewGroup(namespace, name string, obj Group) *Group {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Group").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GroupMemberList is a list of GroupMember resources
type GroupMemberList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GroupMember `json:"items"`
}

func NewGroupMember(namespace, name string, obj GroupMember) *GroupMember {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("GroupMember").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KontainerDriverList is a list of KontainerDriver resources
type KontainerDriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []KontainerDriver `json:"items"`
}

func NewKontainerDriver(namespace, name string, obj KontainerDriver) *KontainerDriver {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("KontainerDriver").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LocalProviderList is a list of LocalProvider resources
type LocalProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []LocalProvider `json:"items"`
}

func NewLocalProvider(namespace, name string, obj LocalProvider) *LocalProvider {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("LocalProvider").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedChartList is a list of ManagedChart resources
type ManagedChartList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ManagedChart `json:"items"`
}

func NewManagedChart(namespace, name string, obj ManagedChart) *ManagedChart {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ManagedChart").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MonitorMetricList is a list of MonitorMetric resources
type MonitorMetricList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MonitorMetric `json:"items"`
}

func NewMonitorMetric(namespace, name string, obj MonitorMetric) *MonitorMetric {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("MonitorMetric").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MultiClusterAppList is a list of MultiClusterApp resources
type MultiClusterAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MultiClusterApp `json:"items"`
}

func NewMultiClusterApp(namespace, name string, obj MultiClusterApp) *MultiClusterApp {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("MultiClusterApp").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MultiClusterAppRevisionList is a list of MultiClusterAppRevision resources
type MultiClusterAppRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MultiClusterAppRevision `json:"items"`
}

func NewMultiClusterAppRevision(namespace, name string, obj MultiClusterAppRevision) *MultiClusterAppRevision {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("MultiClusterAppRevision").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeList is a list of Node resources
type NodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Node `json:"items"`
}

func NewNode(namespace, name string, obj Node) *Node {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Node").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeDriverList is a list of NodeDriver resources
type NodeDriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NodeDriver `json:"items"`
}

func NewNodeDriver(namespace, name string, obj NodeDriver) *NodeDriver {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("NodeDriver").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodePoolList is a list of NodePool resources
type NodePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NodePool `json:"items"`
}

func NewNodePool(namespace, name string, obj NodePool) *NodePool {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("NodePool").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NodeTemplateList is a list of NodeTemplate resources
type NodeTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NodeTemplate `json:"items"`
}

func NewNodeTemplate(namespace, name string, obj NodeTemplate) *NodeTemplate {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("NodeTemplate").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NotifierList is a list of Notifier resources
type NotifierList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Notifier `json:"items"`
}

func NewNotifier(namespace, name string, obj Notifier) *Notifier {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Notifier").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OIDCProviderList is a list of OIDCProvider resources
type OIDCProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []OIDCProvider `json:"items"`
}

func NewOIDCProvider(namespace, name string, obj OIDCProvider) *OIDCProvider {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("OIDCProvider").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OpenLdapProviderList is a list of OpenLdapProvider resources
type OpenLdapProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []OpenLdapProvider `json:"items"`
}

func NewOpenLdapProvider(namespace, name string, obj OpenLdapProvider) *OpenLdapProvider {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("OpenLdapProvider").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodSecurityAdmissionConfigurationTemplateList is a list of PodSecurityAdmissionConfigurationTemplate resources
type PodSecurityAdmissionConfigurationTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PodSecurityAdmissionConfigurationTemplate `json:"items"`
}

func NewPodSecurityAdmissionConfigurationTemplate(namespace, name string, obj PodSecurityAdmissionConfigurationTemplate) *PodSecurityAdmissionConfigurationTemplate {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("PodSecurityAdmissionConfigurationTemplate").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PreferenceList is a list of Preference resources
type PreferenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Preference `json:"items"`
}

func NewPreference(namespace, name string, obj Preference) *Preference {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Preference").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PrincipalList is a list of Principal resources
type PrincipalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Principal `json:"items"`
}

func NewPrincipal(namespace, name string, obj Principal) *Principal {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Principal").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectList is a list of Project resources
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Project `json:"items"`
}

func NewProject(namespace, name string, obj Project) *Project {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Project").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectAlertList is a list of ProjectAlert resources
type ProjectAlertList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ProjectAlert `json:"items"`
}

func NewProjectAlert(namespace, name string, obj ProjectAlert) *ProjectAlert {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ProjectAlert").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectAlertGroupList is a list of ProjectAlertGroup resources
type ProjectAlertGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ProjectAlertGroup `json:"items"`
}

func NewProjectAlertGroup(namespace, name string, obj ProjectAlertGroup) *ProjectAlertGroup {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ProjectAlertGroup").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectAlertRuleList is a list of ProjectAlertRule resources
type ProjectAlertRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ProjectAlertRule `json:"items"`
}

func NewProjectAlertRule(namespace, name string, obj ProjectAlertRule) *ProjectAlertRule {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ProjectAlertRule").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectLoggingList is a list of ProjectLogging resources
type ProjectLoggingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ProjectLogging `json:"items"`
}

func NewProjectLogging(namespace, name string, obj ProjectLogging) *ProjectLogging {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ProjectLogging").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectMonitorGraphList is a list of ProjectMonitorGraph resources
type ProjectMonitorGraphList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ProjectMonitorGraph `json:"items"`
}

func NewProjectMonitorGraph(namespace, name string, obj ProjectMonitorGraph) *ProjectMonitorGraph {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ProjectMonitorGraph").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectNetworkPolicyList is a list of ProjectNetworkPolicy resources
type ProjectNetworkPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ProjectNetworkPolicy `json:"items"`
}

func NewProjectNetworkPolicy(namespace, name string, obj ProjectNetworkPolicy) *ProjectNetworkPolicy {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ProjectNetworkPolicy").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectRoleTemplateBindingList is a list of ProjectRoleTemplateBinding resources
type ProjectRoleTemplateBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ProjectRoleTemplateBinding `json:"items"`
}

func NewProjectRoleTemplateBinding(namespace, name string, obj ProjectRoleTemplateBinding) *ProjectRoleTemplateBinding {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ProjectRoleTemplateBinding").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RancherUserNotificationList is a list of RancherUserNotification resources
type RancherUserNotificationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []RancherUserNotification `json:"items"`
}

func NewRancherUserNotification(namespace, name string, obj RancherUserNotification) *RancherUserNotification {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("RancherUserNotification").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RkeAddonList is a list of RkeAddon resources
type RkeAddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []RkeAddon `json:"items"`
}

func NewRkeAddon(namespace, name string, obj RkeAddon) *RkeAddon {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("RkeAddon").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RkeK8sServiceOptionList is a list of RkeK8sServiceOption resources
type RkeK8sServiceOptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []RkeK8sServiceOption `json:"items"`
}

func NewRkeK8sServiceOption(namespace, name string, obj RkeK8sServiceOption) *RkeK8sServiceOption {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("RkeK8sServiceOption").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RkeK8sSystemImageList is a list of RkeK8sSystemImage resources
type RkeK8sSystemImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []RkeK8sSystemImage `json:"items"`
}

func NewRkeK8sSystemImage(namespace, name string, obj RkeK8sSystemImage) *RkeK8sSystemImage {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("RkeK8sSystemImage").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RoleTemplateList is a list of RoleTemplate resources
type RoleTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []RoleTemplate `json:"items"`
}

func NewRoleTemplate(namespace, name string, obj RoleTemplate) *RoleTemplate {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("RoleTemplate").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SamlProviderList is a list of SamlProvider resources
type SamlProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SamlProvider `json:"items"`
}

func NewSamlProvider(namespace, name string, obj SamlProvider) *SamlProvider {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("SamlProvider").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SamlTokenList is a list of SamlToken resources
type SamlTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SamlToken `json:"items"`
}

func NewSamlToken(namespace, name string, obj SamlToken) *SamlToken {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("SamlToken").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SettingList is a list of Setting resources
type SettingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Setting `json:"items"`
}

func NewSetting(namespace, name string, obj Setting) *Setting {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Setting").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TemplateList is a list of Template resources
type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Template `json:"items"`
}

func NewTemplate(namespace, name string, obj Template) *Template {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Template").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TemplateContentList is a list of TemplateContent resources
type TemplateContentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TemplateContent `json:"items"`
}

func NewTemplateContent(namespace, name string, obj TemplateContent) *TemplateContent {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("TemplateContent").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TemplateVersionList is a list of TemplateVersion resources
type TemplateVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TemplateVersion `json:"items"`
}

func NewTemplateVersion(namespace, name string, obj TemplateVersion) *TemplateVersion {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("TemplateVersion").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TokenList is a list of Token resources
type TokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Token `json:"items"`
}

func NewToken(namespace, name string, obj Token) *Token {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("Token").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UserList is a list of User resources
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []User `json:"items"`
}

func NewUser(namespace, name string, obj User) *User {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("User").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UserAttributeList is a list of UserAttribute resources
type UserAttributeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []UserAttribute `json:"items"`
}

func NewUserAttribute(namespace, name string, obj UserAttribute) *UserAttribute {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("UserAttribute").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProjectCatalogList is a list of ProjectCatalog resources
type ProjectCatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ProjectCatalog `json:"items"`
}

func NewProjectCatalog(namespace, name string, obj ProjectCatalog) *ProjectCatalog {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ProjectCatalog").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterCatalogList is a list of ClusterCatalog resources
type ClusterCatalogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterCatalog `json:"items"`
}

func NewClusterCatalog(namespace, name string, obj ClusterCatalog) *ClusterCatalog {
	obj.APIVersion, obj.Kind = SchemeGroupVersion.WithKind("ClusterCatalog").ToAPIVersionAndKind()
	obj.Name = name
	obj.Namespace = namespace
	return &obj
}
