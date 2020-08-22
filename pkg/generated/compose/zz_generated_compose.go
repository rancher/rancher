package compose

import (
	clusterClient "github.com/rancher/rancher/pkg/client/generated/cluster/v3"
	managementClient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	projectClient "github.com/rancher/rancher/pkg/client/generated/project/v3"
)

type Config struct {
	Version string `yaml:"version,omitempty"`

	// Management Client
	NodePools                                map[string]managementClient.NodePool                                `json:"nodePools,omitempty" yaml:"nodePools,omitempty"`
	Nodes                                    map[string]managementClient.Node                                    `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	NodeDrivers                              map[string]managementClient.NodeDriver                              `json:"nodeDrivers,omitempty" yaml:"nodeDrivers,omitempty"`
	NodeTemplates                            map[string]managementClient.NodeTemplate                            `json:"nodeTemplates,omitempty" yaml:"nodeTemplates,omitempty"`
	Projects                                 map[string]managementClient.Project                                 `json:"projects,omitempty" yaml:"projects,omitempty"`
	GlobalRoles                              map[string]managementClient.GlobalRole                              `json:"globalRoles,omitempty" yaml:"globalRoles,omitempty"`
	GlobalRoleBindings                       map[string]managementClient.GlobalRoleBinding                       `json:"globalRoleBindings,omitempty" yaml:"globalRoleBindings,omitempty"`
	RoleTemplates                            map[string]managementClient.RoleTemplate                            `json:"roleTemplates,omitempty" yaml:"roleTemplates,omitempty"`
	PodSecurityPolicyTemplates               map[string]managementClient.PodSecurityPolicyTemplate               `json:"podSecurityPolicyTemplates,omitempty" yaml:"podSecurityPolicyTemplates,omitempty"`
	PodSecurityPolicyTemplateProjectBindings map[string]managementClient.PodSecurityPolicyTemplateProjectBinding `json:"podSecurityPolicyTemplateProjectBindings,omitempty" yaml:"podSecurityPolicyTemplateProjectBindings,omitempty"`
	ClusterRoleTemplateBindings              map[string]managementClient.ClusterRoleTemplateBinding              `json:"clusterRoleTemplateBindings,omitempty" yaml:"clusterRoleTemplateBindings,omitempty"`
	ProjectRoleTemplateBindings              map[string]managementClient.ProjectRoleTemplateBinding              `json:"projectRoleTemplateBindings,omitempty" yaml:"projectRoleTemplateBindings,omitempty"`
	Clusters                                 map[string]managementClient.Cluster                                 `json:"clusters,omitempty" yaml:"clusters,omitempty"`
	ClusterRegistrationTokens                map[string]managementClient.ClusterRegistrationToken                `json:"clusterRegistrationTokens,omitempty" yaml:"clusterRegistrationTokens,omitempty"`
	Catalogs                                 map[string]managementClient.Catalog                                 `json:"catalogs,omitempty" yaml:"catalogs,omitempty"`
	Templates                                map[string]managementClient.Template                                `json:"templates,omitempty" yaml:"templates,omitempty"`
	CatalogTemplates                         map[string]managementClient.CatalogTemplate                         `json:"catalogTemplates,omitempty" yaml:"catalogTemplates,omitempty"`
	CatalogTemplateVersions                  map[string]managementClient.CatalogTemplateVersion                  `json:"catalogTemplateVersions,omitempty" yaml:"catalogTemplateVersions,omitempty"`
	TemplateVersions                         map[string]managementClient.TemplateVersion                         `json:"templateVersions,omitempty" yaml:"templateVersions,omitempty"`
	TemplateContents                         map[string]managementClient.TemplateContent                         `json:"templateContents,omitempty" yaml:"templateContents,omitempty"`
	Groups                                   map[string]managementClient.Group                                   `json:"groups,omitempty" yaml:"groups,omitempty"`
	GroupMembers                             map[string]managementClient.GroupMember                             `json:"groupMembers,omitempty" yaml:"groupMembers,omitempty"`
	SamlTokens                               map[string]managementClient.SamlToken                               `json:"samlTokens,omitempty" yaml:"samlTokens,omitempty"`
	Users                                    map[string]managementClient.User                                    `json:"users,omitempty" yaml:"users,omitempty"`
	LdapConfigs                              map[string]managementClient.LdapConfig                              `json:"ldapConfigs,omitempty" yaml:"ldapConfigs,omitempty"`
	Tokens                                   map[string]managementClient.Token                                   `json:"tokens,omitempty" yaml:"tokens,omitempty"`
	DynamicSchemas                           map[string]managementClient.DynamicSchema                           `json:"dynamicSchemas,omitempty" yaml:"dynamicSchemas,omitempty"`
	Preferences                              map[string]managementClient.Preference                              `json:"preferences,omitempty" yaml:"preferences,omitempty"`
	ClusterLoggings                          map[string]managementClient.ClusterLogging                          `json:"clusterLoggings,omitempty" yaml:"clusterLoggings,omitempty"`
	ProjectLoggings                          map[string]managementClient.ProjectLogging                          `json:"projectLoggings,omitempty" yaml:"projectLoggings,omitempty"`
	Settings                                 map[string]managementClient.Setting                                 `json:"settings,omitempty" yaml:"settings,omitempty"`
	Features                                 map[string]managementClient.Feature                                 `json:"features,omitempty" yaml:"features,omitempty"`
	ClusterAlerts                            map[string]managementClient.ClusterAlert                            `json:"clusterAlerts,omitempty" yaml:"clusterAlerts,omitempty"`
	ProjectAlerts                            map[string]managementClient.ProjectAlert                            `json:"projectAlerts,omitempty" yaml:"projectAlerts,omitempty"`
	Notifiers                                map[string]managementClient.Notifier                                `json:"notifiers,omitempty" yaml:"notifiers,omitempty"`
	ClusterAlertGroups                       map[string]managementClient.ClusterAlertGroup                       `json:"clusterAlertGroups,omitempty" yaml:"clusterAlertGroups,omitempty"`
	ProjectAlertGroups                       map[string]managementClient.ProjectAlertGroup                       `json:"projectAlertGroups,omitempty" yaml:"projectAlertGroups,omitempty"`
	ClusterAlertRules                        map[string]managementClient.ClusterAlertRule                        `json:"clusterAlertRules,omitempty" yaml:"clusterAlertRules,omitempty"`
	ProjectAlertRules                        map[string]managementClient.ProjectAlertRule                        `json:"projectAlertRules,omitempty" yaml:"projectAlertRules,omitempty"`
	ComposeConfigs                           map[string]managementClient.ComposeConfig                           `json:"composeConfigs,omitempty" yaml:"composeConfigs,omitempty"`
	ProjectCatalogs                          map[string]managementClient.ProjectCatalog                          `json:"projectCatalogs,omitempty" yaml:"projectCatalogs,omitempty"`
	ClusterCatalogs                          map[string]managementClient.ClusterCatalog                          `json:"clusterCatalogs,omitempty" yaml:"clusterCatalogs,omitempty"`
	MultiClusterApps                         map[string]managementClient.MultiClusterApp                         `json:"multiClusterApps,omitempty" yaml:"multiClusterApps,omitempty"`
	MultiClusterAppRevisions                 map[string]managementClient.MultiClusterAppRevision                 `json:"multiClusterAppRevisions,omitempty" yaml:"multiClusterAppRevisions,omitempty"`
	GlobalDnss                               map[string]managementClient.GlobalDns                               `json:"globalDnses,omitempty" yaml:"globalDnses,omitempty"`
	GlobalDNSProviders                       map[string]managementClient.GlobalDNSProvider                       `json:"globalDnsProviders,omitempty" yaml:"globalDnsProviders,omitempty"`
	KontainerDrivers                         map[string]managementClient.KontainerDriver                         `json:"kontainerDrivers,omitempty" yaml:"kontainerDrivers,omitempty"`
	EtcdBackups                              map[string]managementClient.EtcdBackup                              `json:"etcdBackups,omitempty" yaml:"etcdBackups,omitempty"`
	MonitorMetrics                           map[string]managementClient.MonitorMetric                           `json:"monitorMetrics,omitempty" yaml:"monitorMetrics,omitempty"`
	ClusterMonitorGraphs                     map[string]managementClient.ClusterMonitorGraph                     `json:"clusterMonitorGraphs,omitempty" yaml:"clusterMonitorGraphs,omitempty"`
	ProjectMonitorGraphs                     map[string]managementClient.ProjectMonitorGraph                     `json:"projectMonitorGraphs,omitempty" yaml:"projectMonitorGraphs,omitempty"`
	CloudCredentials                         map[string]managementClient.CloudCredential                         `json:"cloudCredentials,omitempty" yaml:"cloudCredentials,omitempty"`
	ManagementSecrets                        map[string]managementClient.ManagementSecret                        `json:"managementSecrets,omitempty" yaml:"managementSecrets,omitempty"`
	ClusterTemplates                         map[string]managementClient.ClusterTemplate                         `json:"clusterTemplates,omitempty" yaml:"clusterTemplates,omitempty"`
	ClusterTemplateRevisions                 map[string]managementClient.ClusterTemplateRevision                 `json:"clusterTemplateRevisions,omitempty" yaml:"clusterTemplateRevisions,omitempty"`
	RkeK8sSystemImages                       map[string]managementClient.RkeK8sSystemImage                       `json:"rkeK8sSystemImages,omitempty" yaml:"rkeK8sSystemImages,omitempty"`
	RkeK8sServiceOptions                     map[string]managementClient.RkeK8sServiceOption                     `json:"rkeK8sServiceOptions,omitempty" yaml:"rkeK8sServiceOptions,omitempty"`
	RkeAddons                                map[string]managementClient.RkeAddon                                `json:"rkeAddons,omitempty" yaml:"rkeAddons,omitempty"`
	CisConfigs                               map[string]managementClient.CisConfig                               `json:"cisConfigs,omitempty" yaml:"cisConfigs,omitempty"`
	CisBenchmarkVersions                     map[string]managementClient.CisBenchmarkVersion                     `json:"cisBenchmarkVersions,omitempty" yaml:"cisBenchmarkVersions,omitempty"`

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
	Apps                           map[string]projectClient.App                           `json:"apps,omitempty" yaml:"apps,omitempty"`
	AppRevisions                   map[string]projectClient.AppRevision                   `json:"appRevisions,omitempty" yaml:"appRevisions,omitempty"`
	SourceCodeProviders            map[string]projectClient.SourceCodeProvider            `json:"sourceCodeProviders,omitempty" yaml:"sourceCodeProviders,omitempty"`
	SourceCodeProviderConfigs      map[string]projectClient.SourceCodeProviderConfig      `json:"sourceCodeProviderConfigs,omitempty" yaml:"sourceCodeProviderConfigs,omitempty"`
	SourceCodeCredentials          map[string]projectClient.SourceCodeCredential          `json:"sourceCodeCredentials,omitempty" yaml:"sourceCodeCredentials,omitempty"`
	Pipelines                      map[string]projectClient.Pipeline                      `json:"pipelines,omitempty" yaml:"pipelines,omitempty"`
	PipelineExecutions             map[string]projectClient.PipelineExecution             `json:"pipelineExecutions,omitempty" yaml:"pipelineExecutions,omitempty"`
	PipelineSettings               map[string]projectClient.PipelineSetting               `json:"pipelineSettings,omitempty" yaml:"pipelineSettings,omitempty"`
	SourceCodeRepositorys          map[string]projectClient.SourceCodeRepository          `json:"sourceCodeRepositories,omitempty" yaml:"sourceCodeRepositories,omitempty"`
	Prometheuss                    map[string]projectClient.Prometheus                    `json:"prometheuses,omitempty" yaml:"prometheuses,omitempty"`
	ServiceMonitors                map[string]projectClient.ServiceMonitor                `json:"serviceMonitors,omitempty" yaml:"serviceMonitors,omitempty"`
	PrometheusRules                map[string]projectClient.PrometheusRule                `json:"prometheusRules,omitempty" yaml:"prometheusRules,omitempty"`
	Alertmanagers                  map[string]projectClient.Alertmanager                  `json:"alertmanagers,omitempty" yaml:"alertmanagers,omitempty"`
	HorizontalPodAutoscalers       map[string]projectClient.HorizontalPodAutoscaler       `json:"horizontalPodAutoscalers,omitempty" yaml:"horizontalPodAutoscalers,omitempty"`
	VirtualServices                map[string]projectClient.VirtualService                `json:"virtualServices,omitempty" yaml:"virtualServices,omitempty"`
	DestinationRules               map[string]projectClient.DestinationRule               `json:"destinationRules,omitempty" yaml:"destinationRules,omitempty"`
	Gateways                       map[string]projectClient.Gateway                       `json:"gateways,omitempty" yaml:"gateways,omitempty"`
}
