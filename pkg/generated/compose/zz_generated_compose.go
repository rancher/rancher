package compose

import (
	clusterClient "github.com/rancher/rancher/pkg/client/generated/cluster/v3"
	managementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	projectClient "github.com/rancher/rancher/pkg/client/generated/project/v3"
)

type Config struct {
	Version string `yaml:"version,omitempty"`

	// Management Client
	NodePools                                  map[string]managementClient.NodePool                                  `json:"nodePools,omitempty" yaml:"nodePools,omitempty"`
	Nodes                                      map[string]managementClient.Node                                      `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	NodeDrivers                                map[string]managementClient.NodeDriver                                `json:"nodeDrivers,omitempty" yaml:"nodeDrivers,omitempty"`
	NodeTemplates                              map[string]managementClient.NodeTemplate                              `json:"nodeTemplates,omitempty" yaml:"nodeTemplates,omitempty"`
	PodSecurityAdmissionConfigurationTemplates map[string]managementClient.PodSecurityAdmissionConfigurationTemplate `json:"podSecurityAdmissionConfigurationTemplates,omitempty" yaml:"podSecurityAdmissionConfigurationTemplates,omitempty"`
	Projects                                   map[string]managementClient.Project                                   `json:"projects,omitempty" yaml:"projects,omitempty"`
	GlobalRoles                                map[string]managementClient.GlobalRole                                `json:"globalRoles,omitempty" yaml:"globalRoles,omitempty"`
	GlobalRoleBindings                         map[string]managementClient.GlobalRoleBinding                         `json:"globalRoleBindings,omitempty" yaml:"globalRoleBindings,omitempty"`
	RoleTemplates                              map[string]managementClient.RoleTemplate                              `json:"roleTemplates,omitempty" yaml:"roleTemplates,omitempty"`
	ClusterRoleTemplateBindings                map[string]managementClient.ClusterRoleTemplateBinding                `json:"clusterRoleTemplateBindings,omitempty" yaml:"clusterRoleTemplateBindings,omitempty"`
	ProjectRoleTemplateBindings                map[string]managementClient.ProjectRoleTemplateBinding                `json:"projectRoleTemplateBindings,omitempty" yaml:"projectRoleTemplateBindings,omitempty"`
	Clusters                                   map[string]managementClient.Cluster                                   `json:"clusters,omitempty" yaml:"clusters,omitempty"`
	ClusterRegistrationTokens                  map[string]managementClient.ClusterRegistrationToken                  `json:"clusterRegistrationTokens,omitempty" yaml:"clusterRegistrationTokens,omitempty"`
	Groups                                     map[string]managementClient.Group                                     `json:"groups,omitempty" yaml:"groups,omitempty"`
	GroupMembers                               map[string]managementClient.GroupMember                               `json:"groupMembers,omitempty" yaml:"groupMembers,omitempty"`
	SamlTokens                                 map[string]managementClient.SamlToken                                 `json:"samlTokens,omitempty" yaml:"samlTokens,omitempty"`
	Users                                      map[string]managementClient.User                                      `json:"users,omitempty" yaml:"users,omitempty"`
	LdapConfigs                                map[string]managementClient.LdapConfig                                `json:"ldapConfigs,omitempty" yaml:"ldapConfigs,omitempty"`
	Tokens                                     map[string]managementClient.Token                                     `json:"tokens,omitempty" yaml:"tokens,omitempty"`
	DynamicSchemas                             map[string]managementClient.DynamicSchema                             `json:"dynamicSchemas,omitempty" yaml:"dynamicSchemas,omitempty"`
	Preferences                                map[string]managementClient.Preference                                `json:"preferences,omitempty" yaml:"preferences,omitempty"`
	Settings                                   map[string]managementClient.Setting                                   `json:"settings,omitempty" yaml:"settings,omitempty"`
	Features                                   map[string]managementClient.Feature                                   `json:"features,omitempty" yaml:"features,omitempty"`
	ComposeConfigs                             map[string]managementClient.ComposeConfig                             `json:"composeConfigs,omitempty" yaml:"composeConfigs,omitempty"`
	KontainerDrivers                           map[string]managementClient.KontainerDriver                           `json:"kontainerDrivers,omitempty" yaml:"kontainerDrivers,omitempty"`
	EtcdBackups                                map[string]managementClient.EtcdBackup                                `json:"etcdBackups,omitempty" yaml:"etcdBackups,omitempty"`
	CloudCredentials                           map[string]managementClient.CloudCredential                           `json:"cloudCredentials,omitempty" yaml:"cloudCredentials,omitempty"`
	ManagementSecrets                          map[string]managementClient.ManagementSecret                          `json:"managementSecrets,omitempty" yaml:"managementSecrets,omitempty"`
	ClusterTemplates                           map[string]managementClient.ClusterTemplate                           `json:"clusterTemplates,omitempty" yaml:"clusterTemplates,omitempty"`
	ClusterTemplateRevisions                   map[string]managementClient.ClusterTemplateRevision                   `json:"clusterTemplateRevisions,omitempty" yaml:"clusterTemplateRevisions,omitempty"`
	RkeK8sSystemImages                         map[string]managementClient.RkeK8sSystemImage                         `json:"rkeK8sSystemImages,omitempty" yaml:"rkeK8sSystemImages,omitempty"`
	RkeK8sServiceOptions                       map[string]managementClient.RkeK8sServiceOption                       `json:"rkeK8sServiceOptions,omitempty" yaml:"rkeK8sServiceOptions,omitempty"`
	RkeAddons                                  map[string]managementClient.RkeAddon                                  `json:"rkeAddons,omitempty" yaml:"rkeAddons,omitempty"`
	FleetWorkspaces                            map[string]managementClient.FleetWorkspace                            `json:"fleetWorkspaces,omitempty" yaml:"fleetWorkspaces,omitempty"`
	RancherUserNotifications                   map[string]managementClient.RancherUserNotification                   `json:"rancherUserNotifications,omitempty" yaml:"rancherUserNotifications,omitempty"`

	// Cluster Client
	Namespaces        map[string]clusterClient.Namespace        `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	PersistentVolumes map[string]clusterClient.PersistentVolume `json:"persistentVolumes,omitempty" yaml:"persistentVolumes,omitempty"`
	StorageClasss     map[string]clusterClient.StorageClass     `json:"storageClasses,omitempty" yaml:"storageClasses,omitempty"`
	APIServices       map[string]clusterClient.APIService       `json:"apiServices,omitempty" yaml:"apiServices,omitempty"`

	// Project Client
	PersistentVolumeClaims         map[string]projectClient.PersistentVolumeClaim         `json:"persistentVolumeClaims,omitempty" yaml:"persistentVolumeClaims,omitempty"`
	ConfigMaps                     map[string]projectClient.ConfigMap                     `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
	Ingresss                       map[string]projectClient.Ingress                       `json:"ingresses,omitempty" yaml:"ingresses,omitempty"`
	Secrets                        map[string]projectClient.Secret                        `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	ServiceAccountTokens           map[string]projectClient.ServiceAccountToken           `json:"serviceAccountTokens,omitempty" yaml:"serviceAccountTokens,omitempty"`
	DockerCredentials              map[string]projectClient.DockerCredential              `json:"dockerCredentials,omitempty" yaml:"dockerCredentials,omitempty"`
	Certificates                   map[string]projectClient.Certificate                   `json:"certificates,omitempty" yaml:"certificates,omitempty"`
	BasicAuths                     map[string]projectClient.BasicAuth                     `json:"basicAuths,omitempty" yaml:"basicAuths,omitempty"`
	SSHAuths                       map[string]projectClient.SSHAuth                       `json:"sshAuths,omitempty" yaml:"sshAuths,omitempty"`
	NamespacedSecrets              map[string]projectClient.NamespacedSecret              `json:"namespacedSecrets,omitempty" yaml:"namespacedSecrets,omitempty"`
	NamespacedServiceAccountTokens map[string]projectClient.NamespacedServiceAccountToken `json:"namespacedServiceAccountTokens,omitempty" yaml:"namespacedServiceAccountTokens,omitempty"`
	NamespacedDockerCredentials    map[string]projectClient.NamespacedDockerCredential    `json:"namespacedDockerCredentials,omitempty" yaml:"namespacedDockerCredentials,omitempty"`
	NamespacedCertificates         map[string]projectClient.NamespacedCertificate         `json:"namespacedCertificates,omitempty" yaml:"namespacedCertificates,omitempty"`
	NamespacedBasicAuths           map[string]projectClient.NamespacedBasicAuth           `json:"namespacedBasicAuths,omitempty" yaml:"namespacedBasicAuths,omitempty"`
	NamespacedSSHAuths             map[string]projectClient.NamespacedSSHAuth             `json:"namespacedSshAuths,omitempty" yaml:"namespacedSshAuths,omitempty"`
	Services                       map[string]projectClient.Service                       `json:"services,omitempty" yaml:"services,omitempty"`
	DNSRecords                     map[string]projectClient.DNSRecord                     `json:"dnsRecords,omitempty" yaml:"dnsRecords,omitempty"`
	Pods                           map[string]projectClient.Pod                           `json:"pods,omitempty" yaml:"pods,omitempty"`
	Deployments                    map[string]projectClient.Deployment                    `json:"deployments,omitempty" yaml:"deployments,omitempty"`
	ReplicationControllers         map[string]projectClient.ReplicationController         `json:"replicationControllers,omitempty" yaml:"replicationControllers,omitempty"`
	ReplicaSets                    map[string]projectClient.ReplicaSet                    `json:"replicaSets,omitempty" yaml:"replicaSets,omitempty"`
	StatefulSets                   map[string]projectClient.StatefulSet                   `json:"statefulSets,omitempty" yaml:"statefulSets,omitempty"`
	DaemonSets                     map[string]projectClient.DaemonSet                     `json:"daemonSets,omitempty" yaml:"daemonSets,omitempty"`
	Jobs                           map[string]projectClient.Job                           `json:"jobs,omitempty" yaml:"jobs,omitempty"`
	CronJobs                       map[string]projectClient.CronJob                       `json:"cronJobs,omitempty" yaml:"cronJobs,omitempty"`
	Workloads                      map[string]projectClient.Workload                      `json:"workloads,omitempty" yaml:"workloads,omitempty"`
	HorizontalPodAutoscalers       map[string]projectClient.HorizontalPodAutoscaler       `json:"horizontalPodAutoscalers,omitempty" yaml:"horizontalPodAutoscalers,omitempty"`
}
