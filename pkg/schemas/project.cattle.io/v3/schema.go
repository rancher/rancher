package schema

import (
	"net/http"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	v3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/schemas/factory"
	"github.com/rancher/rancher/pkg/schemas/mapper"
	appsv1 "k8s.io/api/apps/v1"
	k8sappv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	kextv1beta1 "k8s.io/api/extensions/v1beta1"
	knetworkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	Version = types.APIVersion{
		Version:          "v3",
		Group:            "project.cattle.io",
		Path:             "/v3/project",
		SubContext:       true,
		SubContextSchema: "/v3/schemas/project",
	}

	Schemas = factory.Schemas(&Version).
		// volume before pod types.  pod types uses volume things, so need to register mapper
		Init(volumeTypes).
		Init(configMapTypes).
		Init(ingressTypes).
		Init(secretTypes).
		Init(serviceTypes).
		Init(podTypes).
		Init(deploymentTypes).
		Init(replicationControllerTypes).
		Init(replicaSetTypes).
		Init(statefulSetTypes).
		Init(daemonSetTypes).
		Init(jobTypes).
		Init(cronJobTypes).
		Init(podTemplateSpecTypes).
		Init(workloadTypes).
		Init(appTypes).
		Init(monitoringTypes).
		Init(autoscalingTypes)
)

func configMapTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.MustImportAndCustomize(&Version, v1.ConfigMap{}, func(schema *types.Schema) {
		schema.MustCustomizeField("name", func(field types.Field) types.Field {
			field.Type = "hostname"
			field.Nullable = false
			field.Required = true
			return field
		})
	}, projectOverride{})
}

type DeploymentConfig struct {
}

type StatefulSetConfig struct {
}

type ReplicaSetConfig struct {
}

type ReplicationControllerConfig struct {
}

type DaemonSetConfig struct {
}

type CronJobConfig struct {
}

type JobConfig struct {
}

type deploymentConfigOverride struct {
	DeploymentConfig DeploymentConfig
}

type statefulSetConfigOverride struct {
	StatefulSetConfig StatefulSetConfig
}

type replicaSetConfigOverride struct {
	ReplicaSetConfig ReplicaSetConfig
}

type replicationControllerConfigOverride struct {
	ReplicationControllerConfig ReplicationControllerConfig
}

type daemonSetOverride struct {
	DaemonSetConfig DaemonSetConfig
}

type cronJobOverride struct {
	CronJobConfig CronJobConfig
}

type jobOverride struct {
	JobConfig JobConfig
}

func workloadTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.MustImportAndCustomize(&Version, v3.Workload{},
		func(schema *types.Schema) {
			toInclude := []string{"deployment", "replicationController", "statefulSet",
				"daemonSet", "job", "cronJob", "replicaSet"}
			for _, name := range toInclude {
				baseSchema := schemas.Schema(&Version, name)
				if baseSchema == nil {
					continue
				}
				for name, field := range baseSchema.ResourceFields {
					schema.ResourceFields[name] = field
				}
			}
			schema.ResourceActions = map[string]types.Action{
				"rollback": {
					Input: "rollbackRevision",
				},
				"pause":    {},
				"resume":   {},
				"redeploy": {},
			}
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "dnsLabelRestricted"
				field.Nullable = false
				field.Required = true
				return field
			})
		})
}

func statefulSetTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, k8sappv1.StatefulSetUpdateStrategy{},
			&m.Embed{Field: "rollingUpdate"},
			m.Enum{Field: "type", Options: []string{
				"RollingUpdate",
				"OnDelete",
			}},
			m.Move{From: "type", To: "strategy"},
		).
		AddMapperForType(&Version, k8sappv1.StatefulSetSpec{},
			&m.Move{
				From: "replicas",
				To:   "scale",
			},
			&m.Embed{Field: "updateStrategy"},
			&m.Enum{
				Field: "podManagementPolicy",
				Options: []string{
					"OrderedReady",
					"Parallel",
				},
			},
			&m.BatchMove{
				From: []string{
					"partition",
					"strategy",
					"volumeClaimTemplates",
					"serviceName",
					"revisionHistoryLimit",
					"podManagementPolicy",
				},
				To: "statefulSetConfig",
			},
			&m.Embed{Field: "template"},
		).
		MustImport(&Version, v3.WorkloadMetric{}).
		AddMapperForType(&Version, k8sappv1.StatefulSet{},
			&m.Move{
				From: "status",
				To:   "statefulSetStatus",
			},
			NewWorkloadTypeMapper(),
		).
		MustImport(&Version, k8sappv1.StatefulSetSpec{}, statefulSetConfigOverride{}).
		MustImportAndCustomize(&Version, k8sappv1.StatefulSet{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
			schema.ResourceActions = map[string]types.Action{
				"redeploy": {},
			}
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "dnsLabelRestricted"
				field.Nullable = false
				field.Required = true
				return field
			})
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
			WorkloadMetrics string `json:"workloadMetrics" norman:"type=array[workloadMetric]"`
		}{})
}

func replicaSetTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, appsv1.ReplicaSetSpec{},
			&m.Move{
				From: "replicas",
				To:   "scale",
			},
			&m.Move{
				From: "minReadySeconds",
				To:   "replicaSetConfig/minReadySeconds",
			},
		).
		MustImport(&Version, v3.WorkloadMetric{}).
		AddMapperForType(&Version, appsv1.ReplicaSet{},
			&m.Move{
				From: "status",
				To:   "replicaSetStatus",
			},
			&m.Embed{Field: "template"},
			NewWorkloadTypeMapper(),
		).
		MustImport(&Version, appsv1.ReplicaSetSpec{}, replicaSetConfigOverride{}).
		MustImportAndCustomize(&Version, appsv1.ReplicaSet{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
			schema.ResourceActions = map[string]types.Action{
				"redeploy": {},
			}
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "dnsLabelRestricted"
				field.Nullable = false
				field.Required = true
				return field
			})
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
			WorkloadMetrics string `json:"workloadMetrics" norman:"type=array[workloadMetric]"`
		}{})
}

func replicationControllerTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1.ReplicationControllerSpec{},
			&m.Move{
				From: "replicas",
				To:   "scale",
			},
			&m.Move{
				From: "minReadySeconds",
				To:   "replicationControllerConfig/minReadySeconds",
			},
			&m.Embed{Field: "template"},
		).
		MustImport(&Version, v3.WorkloadMetric{}).
		AddMapperForType(&Version, v1.ReplicationController{},
			&m.Move{
				From: "status",
				To:   "replicationControllerStatus",
			},
			NewWorkloadTypeMapper(),
		).
		MustImport(&Version, v1.ReplicationControllerSpec{}, replicationControllerConfigOverride{}).
		MustImportAndCustomize(&Version, v1.ReplicationController{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
			schema.CollectionMethods = []string{http.MethodGet}
			schema.ResourceMethods = []string{http.MethodGet}
			schema.ResourceActions = map[string]types.Action{
				"redeploy": {},
			}
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "dnsLabelRestricted"
				field.Nullable = false
				field.Required = true
				return field
			})
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
			WorkloadMetrics string `json:"workloadMetrics" norman:"type=array[workloadMetric]"`
		}{})
}

func daemonSetTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, k8sappv1.DaemonSetUpdateStrategy{},
			&m.Embed{Field: "rollingUpdate"},
			m.Enum{Field: "type", Options: []string{
				"RollingUpdate",
				"OnDelete",
			}},
			m.Move{From: "type", To: "strategy"},
		).
		AddMapperForType(&Version, k8sappv1.DaemonSetSpec{},
			&m.Embed{Field: "updateStrategy"},
			&m.BatchMove{
				From: []string{
					"strategy",
					"maxUnavailable",
					"minReadySeconds",
					"revisionHistoryLimit",
				},
				To: "daemonSetConfig",
			},
			&m.Embed{Field: "template"},
		).
		MustImport(&Version, v3.WorkloadMetric{}).
		AddMapperForType(&Version, k8sappv1.DaemonSet{},
			&m.Move{
				From: "status",
				To:   "daemonSetStatus",
			},
			NewWorkloadTypeMapper(),
		).
		MustImport(&Version, k8sappv1.DaemonSetSpec{}, daemonSetOverride{}).
		MustImportAndCustomize(&Version, k8sappv1.DaemonSet{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
			schema.ResourceActions = map[string]types.Action{
				"redeploy": {},
			}
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "dnsLabelRestricted"
				field.Nullable = false
				field.Required = true
				return field
			})
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
			WorkloadMetrics string `json:"workloadMetrics" norman:"type=array[workloadMetric]"`
		}{})
}

func jobTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, batchv1.JobSpec{},
			&m.BatchMove{
				From: []string{"parallelism",
					"completions",
					"activeDeadlineSeconds",
					"backoffLimit",
					"manualSelector",
				},
				To: "jobConfig",
			},
			&m.Embed{Field: "template"},
		).
		MustImport(&Version, v3.WorkloadMetric{}).
		AddMapperForType(&Version, batchv1.Job{},
			&m.Move{
				From: "status",
				To:   "jobStatus",
			},
			NewWorkloadTypeMapper(),
		).
		MustImport(&Version, batchv1.JobSpec{}, jobOverride{}).
		MustImportAndCustomize(&Version, batchv1.Job{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "dnsLabelRestricted"
				field.Nullable = false
				field.Required = true
				return field
			})
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
			WorkloadMetrics string `json:"workloadMetrics" norman:"type=array[workloadMetric]"`
		}{})
}

func cronJobTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, batchv1.JobTemplateSpec{},
			&m.Move{
				From: "metadata",
				To:   "jobMetadata",
			},
			&m.Embed{Field: "spec"},
		).
		AddMapperForType(&Version, batchv1.CronJobSpec{},
			&m.Drop{
				Field: "suspend",
			},
			&m.Embed{
				Field: "jobTemplate",
			},
			&m.BatchMove{
				From: []string{
					"schedule",
					"startingDeadlineSeconds",
					"suspend",
					"successfulJobsHistoryLimit",
					"failedJobsHistoryLimit",
					"jobConfig",
				},
				To: "cronJobConfig",
			},
			&m.Enum{Field: "concurrencyPolicy", Options: []string{
				"Allow",
				"Forbid",
				"Replace",
			}},
			&m.Move{
				From: "concurrencyPolicy",
				To:   "cronJobConfig/concurrencyPolicy",
			},
			&m.Move{
				From:              "jobMetadata/labels",
				To:                "cronJobConfig/jobLabels",
				NoDeleteFromField: true,
			},
			&m.Move{
				From:              "jobMetadata/annotations",
				To:                "cronJobConfig/jobAnnotations",
				NoDeleteFromField: true,
			},
			&m.Drop{Field: "jobMetadata"},
		).
		MustImport(&Version, v3.WorkloadMetric{}).
		AddMapperForType(&Version, batchv1.CronJob{},
			&m.Move{
				From: "status",
				To:   "cronJobStatus",
			},
			NewWorkloadTypeMapper(),
		).
		MustImport(&Version, batchv1.CronJobSpec{}, cronJobOverride{}).
		MustImport(&Version, batchv1.JobTemplateSpec{}).
		MustImportAndCustomize(&Version, batchv1.CronJob{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
			schema.ResourceActions = map[string]types.Action{
				"redeploy": {},
			}
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "dnsLabelRestricted"
				field.Nullable = false
				field.Required = true
				return field
			})
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
			WorkloadMetrics string `json:"workloadMetrics" norman:"type=array[workloadMetric]"`
		}{})
}

func deploymentTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, k8sappv1.DeploymentStrategy{},
			&m.Embed{Field: "rollingUpdate", EmptyValueOk: true},
			m.Enum{Field: "type", Options: []string{
				"Recreate",
				"RollingUpdate",
			}},
			m.Move{From: "type", To: "strategy"},
		).
		AddMapperForType(&Version, k8sappv1.DeploymentSpec{},
			&m.Move{
				From: "strategy",
				To:   "upgradeStrategy",
			},
			&m.Embed{Field: "upgradeStrategy"},
			&m.Move{
				From: "replicas",
				To:   "scale",
			},
			&m.BatchMove{
				From: []string{
					"minReadySeconds",
					"strategy",
					"revisionHistoryLimit",
					"progressDeadlineSeconds",
					"maxUnavailable",
					"maxSurge",
				},
				To: "deploymentConfig",
			},
			&m.Embed{Field: "template"},
		).
		MustImport(&Version, v3.WorkloadMetric{}).
		AddMapperForType(&Version, k8sappv1.Deployment{},
			&m.Move{
				From: "status",
				To:   "deploymentStatus",
			},
			NewWorkloadTypeMapper(),
		).
		MustImport(&Version, k8sappv1.DeploymentSpec{}, deploymentConfigOverride{}).
		MustImport(&Version, v3.DeploymentRollbackInput{}).
		MustImportAndCustomize(&Version, k8sappv1.Deployment{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
			schema.ResourceActions = map[string]types.Action{
				"rollback": {
					Input: "deploymentRollbackInput",
				},
				"pause":    {},
				"resume":   {},
				"redeploy": {},
			}
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "dnsLabelRestricted"
				field.Nullable = false
				field.Required = true
				return field
			})
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
			WorkloadMetrics string `json:"workloadMetrics" norman:"type=array[workloadMetric]"`
		}{})
}

func podTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1.PodTemplateSpec{},
			&m.Embed{Field: "spec"},
		).
		AddMapperForType(&Version, v1.Capabilities{},
			m.Move{From: "add", To: "capAdd"},
			m.Move{From: "drop", To: "capDrop"},
		).
		AddMapperForType(&Version, v1.PodSecurityContext{},
			m.Drop{Field: "seLinuxOptions"},
			m.Move{From: "runAsUser", To: "uid"},
			m.Move{From: "supplementalGroups", To: "gids"},
			m.Move{From: "fsGroup", To: "fsgid"},
		).
		AddMapperForType(&Version, v1.SecurityContext{},
			&m.Embed{Field: "capabilities"},
			m.Drop{Field: "seLinuxOptions"},
			m.Move{From: "readOnlyRootFilesystem", To: "readOnly"},
			m.Move{From: "runAsUser", To: "uid"},
		).
		AddMapperForType(&Version, v1.Container{},
			m.Move{From: "command", To: "entrypoint"},
			m.Move{From: "args", To: "command"},
			&m.Embed{Field: "securityContext"},
			&m.Embed{Field: "lifecycle"},
		).
		AddMapperForType(&Version, v1.ContainerPort{},
			m.Move{From: "hostIP", To: "hostIp"},
		).
		AddMapperForType(&Version, v1.LifecycleHandler{},
			mapper.ContainerProbeHandler{}).
		AddMapperForType(&Version, v1.Probe{}, mapper.ContainerProbeHandler{}).
		AddMapperForType(&Version, v1.LifecycleHandler{}, handlerMapper).
		AddMapperForType(&Version, v1.Probe{}, handlerMapper).
		AddMapperForType(&Version, v1.PodStatus{},
			m.Move{From: "hostIP", To: "nodeIp"},
			m.Move{From: "podIP", To: "podIp"},
		).
		AddMapperForType(&Version, v1.PodSpec{},
			mapper.InitContainerMapper{},
			mapper.SchedulingMapper{},
			m.Move{From: "priority", To: "scheduling/priority", DestDefined: true},
			m.Move{From: "priorityClassName", To: "scheduling/priorityClassName", DestDefined: true},
			m.Move{From: "schedulerName", To: "scheduling/scheduler", DestDefined: true},
			m.Move{From: "tolerations", To: "scheduling/tolerate", DestDefined: true},
			&m.Embed{Field: "securityContext"},
			&m.Drop{Field: "serviceAccount"},
		).
		AddMapperForType(&Version, v1.Pod{},
			&m.AnnotationField{Field: "description"},
			&m.AnnotationField{Field: "publicEndpoints", List: true},
			&m.AnnotationField{Field: "workloadMetrics", List: true},
			mapper.ContainerPorts{},
			mapper.ContainerStatus{},
		).
		// Must import handlers before Container
		MustImport(&Version, v1.ContainerPort{}, struct {
			Kind       string `json:"kind,omitempty" norman:"type=enum,options=HostPort|NodePort|ClusterIP|LoadBalancer"`
			SourcePort int    `json:"sourcePort,omitempty"`
			DNSName    string `json:"dnsName,omitempty"`
			Name       string `json:"name,omitempty"`
			Protocol   string `json:"protocol,omitempty"`
		}{}).
		MustImport(&Version, v1.Capabilities{}, struct {
			Add  []string `norman:"type=array[enum],options=AUDIT_CONTROL|AUDIT_WRITE|BLOCK_SUSPEND|CHOWN|DAC_OVERRIDE|DAC_READ_SEARCH|FOWNER|FSETID|IPC_LOCK|IPC_OWNER|KILL|LEASE|LINUX_IMMUTABLE|MAC_ADMIN|MAC_OVERRIDE|MKNOD|NET_ADMIN|NET_BIND_SERVICE|NET_BROADCAST|NET_RAW|SETFCAP|SETGID|SETPCAP|SETUID|SYSLOG|SYS_ADMIN|SYS_BOOT|SYS_CHROOT|SYS_MODULE|SYS_NICE|SYS_PACCT|SYS_PTRACE|SYS_RAWIO|SYS_RESOURCE|SYS_TIME|SYS_TTY_CONFIG|WAKE_ALARM|ALL"`
			Drop []string `norman:"type=array[enum],options=AUDIT_CONTROL|AUDIT_WRITE|BLOCK_SUSPEND|CHOWN|DAC_OVERRIDE|DAC_READ_SEARCH|FOWNER|FSETID|IPC_LOCK|IPC_OWNER|KILL|LEASE|LINUX_IMMUTABLE|MAC_ADMIN|MAC_OVERRIDE|MKNOD|NET_ADMIN|NET_BIND_SERVICE|NET_BROADCAST|NET_RAW|SETFCAP|SETGID|SETPCAP|SETUID|SYSLOG|SYS_ADMIN|SYS_BOOT|SYS_CHROOT|SYS_MODULE|SYS_NICE|SYS_PACCT|SYS_PTRACE|SYS_RAWIO|SYS_RESOURCE|SYS_TIME|SYS_TTY_CONFIG|WAKE_ALARM|ALL"`
		}{}).
		MustImport(&Version, v3.PublicEndpoint{}).
		MustImport(&Version, v3.WorkloadMetric{}).
		MustImport(&Version, v1.LifecycleHandler{}, handlerOverride{}).
		MustImport(&Version, v1.Probe{}, handlerOverride{}).
		MustImport(&Version, v1.Container{}, struct {
			Environment          map[string]string
			EnvironmentFrom      []EnvironmentFrom
			InitContainer        bool
			State                string
			Transitioning        string
			TransitioningMessage string
			ExitCode             *int
			RestartCount         int
		}{}).
		MustImport(&Version, v1.PodSpec{}, struct {
			Scheduling *Scheduling
			NodeName   string `norman:"type=reference[/v3/schemas/node]"`
		}{}).
		MustImport(&Version, v1.Pod{}, projectOverride{}, struct {
			Description     string `json:"description"`
			WorkloadID      string `norman:"type=reference[workload]"`
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
			WorkloadMetrics string `json:"workloadMetrics" norman:"type=array[workloadMetric],nocreate,noupdate"`
		}{})
}

func serviceTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		Init(addServiceType).
		Init(addDNSRecord)
}

func addServiceType(schemas *types.Schemas) *types.Schemas {
	return schemas.AddSchema(*factory.Schemas(&Version).
		Init(addServiceOrDNSRecord(false)).
		Schema(&Version, "service"))
}

func addDNSRecord(schemas *types.Schemas) *types.Schemas {
	return schemas.
		Init(addServiceOrDNSRecord(true))
}

func addServiceOrDNSRecord(dns bool) types.SchemasInitFunc {
	return func(schemas *types.Schemas) *types.Schemas {
		if dns {
			schemas = schemas.
				TypeName("dnsRecord", v1.Service{})
		}

		schemas = schemas.
			AddMapperForType(&Version, v1.ServiceSpec{},
				&m.Move{From: "externalName", To: "hostname"},
				&m.Move{From: "type", To: "serviceKind"},
				&m.SetValue{
					Field: "clusterIP",
					IfEq:  "None",
					Value: nil,
				},
				&m.Move{From: "clusterIP", To: "clusterIp"},
			).
			AddMapperForType(&Version, v1.Service{},
				&m.Drop{Field: "status"},
				&m.LabelField{Field: "workloadId"},
				&m.AnnotationField{Field: "description"},
				&m.AnnotationField{Field: "ipAddresses", List: true},
				&m.AnnotationField{Field: "targetWorkloadIds", List: true},
				&m.AnnotationField{Field: "targetDnsRecordIds", List: true},
				&m.AnnotationField{Field: "publicEndpoints", List: true},
				&m.Move{From: "serviceKind", To: "kind"},
			)

		if dns {
			schemas = schemas.
				AddMapperForType(&Version, v1.Service{},
					&m.Drop{Field: "kind"},
					&m.Drop{Field: "externalIPs"},
					&m.Drop{Field: "externalTrafficPolicy"},
					&m.Drop{Field: "healthCheckNodePort"},
					&m.Drop{Field: "loadBalancerIP"},
					&m.Drop{Field: "loadBalancerSourceRanges"},
					&m.Drop{Field: "publishNotReadyAddresses"},
					&m.Drop{Field: "sessionAffinity"},
					&m.Drop{Field: "sessionAffinityConfig"},
					&m.Drop{Field: "loadBalancerClass"},
					&m.Drop{Field: "internalTrafficPolicy"},
				)
		}

		return schemas.MustImportAndCustomize(&Version, v1.Service{}, func(schema *types.Schema) {
			if dns {
				schema.CodeName = "DNSRecord"
				schema.MustCustomizeField("clusterIp", func(f types.Field) types.Field {
					f.Create = false
					f.Update = false
					return f
				})
				schema.MustCustomizeField("ports", func(f types.Field) types.Field {
					f.Create = false
					f.Update = false
					return f
				})
				schema.MustCustomizeField("name", func(field types.Field) types.Field {
					field.Type = "dnsLabelRestricted"
					field.Nullable = false
					field.Required = true
					return field
				})
			}
		}, projectOverride{}, struct {
			Description        string   `json:"description"`
			IPAddresses        []string `json:"ipAddresses"`
			WorkloadID         string   `json:"workloadId" norman:"type=reference[workload],nocreate,noupdate"`
			TargetWorkloadIDs  string   `json:"targetWorkloadIds" norman:"type=array[reference[workload]]"`
			TargetDNSRecordIDs string   `json:"targetDnsRecordIds" norman:"type=array[reference[dnsRecord]]"`
			PublicEndpoints    string   `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
		}{})
	}
}

func ingressTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, knetworkingv1.HTTPIngressPath{},
			&m.Embed{Field: "backend"},
			&mapper.IngressPath{},
		).
		AddMapperForType(&Version, knetworkingv1.IngressRule{},
			&m.Embed{Field: "http"},
		).
		AddMapperForType(&Version, knetworkingv1.Ingress{},
			&m.AnnotationField{Field: "description"},
			&m.AnnotationField{Field: "publicEndpoints", List: true},
			&mapper.IngressSpec{},
		).
		AddMapperForType(&Version, knetworkingv1.IngressTLS{},
			&m.Move{From: "secretName", To: "certificateName"},
		).
		AddMapperForType(&Version, knetworkingv1.IngressBackend{},
			&m.Move{From: "service/name", To: "serviceId"},
			&mapper.IngressBackend{},
		).
		MustImportAndCustomize(&Version, knetworkingv1.IngressBackend{}, func(schema *types.Schema) {
			schema.MustCustomizeField("serviceId", func(f types.Field) types.Field {
				f.Type = "reference[service]"
				return f
			})
		}, struct {
			WorkloadIDs string             `json:"workloadIds" norman:"type=array[reference[workload]]"`
			TargetPort  intstr.IntOrString `json:"targetPort"`
		}{}).
		MustImport(&Version, knetworkingv1.IngressRule{}).
		MustImport(&Version, knetworkingv1.IngressTLS{}, struct {
			SecretName string `norman:"type=reference[certificate]"`
		}{}).
		MustImport(&Version, knetworkingv1.IngressSpec{}, struct {
			Backend kextv1beta1.IngressBackend `json:"backend"`
		}{}).
		MustImportAndCustomize(&Version, knetworkingv1.Ingress{}, func(schema *types.Schema) {
			schema.MustCustomizeField("name", func(f types.Field) types.Field {
				f.Type = "hostname"
				f.Required = true
				f.Nullable = false
				return f
			})
		}, projectOverride{}, struct {
			Description     string `json:"description"`
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
		}{})
}

func volumeTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1.HostPathVolumeSource{},
			m.Move{From: "type", To: "kind"},
			m.Enum{
				Options: []string{
					"DirectoryOrCreate",
					"Directory",
					"FileOrCreate",
					"File",
					"Socket",
					"CharDevice",
					"BlockDevice",
				},
				Field: "kind",
			},
		).
		AddMapperForType(&Version, v1.PersistentVolumeClaimVolumeSource{},
			&m.Move{From: "claimName", To: "persistentVolumeClaimName"},
		).
		AddMapperForType(&Version, v1.VolumeMount{},
			m.Required{Fields: []string{
				"mountPath",
				"name",
			}},
		).
		AddMapperForType(&Version, v1.PersistentVolumeClaim{},
			mapper.PersistVolumeClaim{},
		).
		MustImport(&Version, v1.PersistentVolumeClaimVolumeSource{}, struct {
			ClaimName string `norman:"type=reference[persistentVolumeClaim]"`
		}{}).
		MustImport(&Version, v1.SecretVolumeSource{}, struct {
		}{}).
		MustImport(&Version, v1.VolumeMount{}, struct {
			MountPath string `json:"mountPath" norman:"required"`
		}{}).
		MustImport(&Version, v1.PersistentVolumeSpec{}, struct {
			StorageClassName *string `json:"storageClassName,omitempty" norman:"type=reference[/v3/cluster/storageClass]"`
		}{}).
		MustImport(&Version, v1.PersistentVolumeClaimSpec{}, struct {
			AccessModes      []string `json:"accessModes,omitempty" norman:"type=array[enum],options=ReadWriteOnce|ReadOnlyMany|ReadWriteMany"`
			VolumeName       string   `json:"volumeName,omitempty" norman:"type=reference[/v3/cluster/persistentVolume]"`
			StorageClassName *string  `json:"storageClassName,omitempty" norman:"type=reference[/v3/cluster/storageClass]"`
		}{}).
		MustImport(&Version, v1.Volume{}, struct {
		}{}).
		MustImportAndCustomize(&Version, v1.PersistentVolumeClaim{}, func(schema *types.Schema) {
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "hostname"
				field.Nullable = false
				field.Required = true
				return field
			})
		}, projectOverride{})
}

func appTypes(schema *types.Schemas) *types.Schemas {
	return schema.
		AddMapperForType(&Version, v3.App{}, &m.Embed{Field: "status"}).
		MustImport(&Version, v3.AppUpgradeConfig{}).
		MustImport(&Version, v3.RollbackRevision{}).
		MustImportAndCustomize(&Version, v3.App{}, func(schema *types.Schema) {
			schema.ResourceActions = map[string]types.Action{
				"upgrade": {
					Input: "appUpgradeConfig",
				},
				"rollback": {
					Input: "rollbackRevision",
				},
			}
		}).
		MustImport(&Version, v3.AppRevision{})
}

func podTemplateSpecTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		MustImport(&Version, v1.PodTemplateSpec{})
}

func NewWorkloadTypeMapper() types.Mapper {
	return &types.Mappers{
		&m.Move{From: "labels", To: "workloadLabels"},
		&m.Move{From: "annotations", To: "workloadAnnotations"},
		&m.Move{From: "metadata/labels", To: "labels", NoDeleteFromField: true},
		&m.Move{From: "metadata/annotations", To: "annotations", NoDeleteFromField: true},
		&m.Drop{Field: "metadata"},
		mapper.ContainerPorts{},
		mapper.WorkloadAnnotations{},
		&m.AnnotationField{Field: "publicEndpoints", List: true},
		&m.AnnotationField{Field: "workloadMetrics", List: true},
	}
}

func monitoringTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, monitoringv1.Prometheus{},
			&m.Drop{Field: "status"},
			&m.AnnotationField{Field: "description"},
		).
		AddMapperForType(&Version, monitoringv1.PrometheusSpec{},
			&m.Drop{Field: "thanos"},
			&m.Drop{Field: "apiserverConfig"},
			&m.Drop{Field: "serviceMonitorNamespaceSelector"},
			&m.Drop{Field: "ruleNamespaceSelector"},
			&m.Drop{Field: "paused"},
			&m.Enum{
				Field: "logLevel",
				Options: []string{
					"all",
					"debug",
					"info",
					"warn",
					"error",
					"none",
				},
			},
		).
		MustImportAndCustomize(&Version, monitoringv1.Prometheus{}, func(schema *types.Schema) {
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "dnsLabelRestricted"
				field.Nullable = false
				field.Required = true
				return field
			})
		}, projectOverride{}, struct {
			Description string `json:"description"`
		}{}).
		AddMapperForType(&Version, monitoringv1.RelabelConfig{},
			&m.Enum{
				Field: "action",
				Options: []string{
					"replace",
					"keep",
					"drop",
					"hashmod",
					"labelmap",
					"labeldrop",
					"labelkeep",
				},
			},
		).
		AddMapperForType(&Version, monitoringv1.Endpoint{},
			&m.Drop{Field: "port"},
			&m.Drop{Field: "tlsConfig"},
			&m.Drop{Field: "bearerTokenFile"},
			&m.Drop{Field: "honorLabels"},
			&m.Drop{Field: "basicAuth"},
			&m.Drop{Field: "metricRelabelings"},
			&m.Drop{Field: "proxyUrl"},
		).
		AddMapperForType(&Version, monitoringv1.ServiceMonitorSpec{},
			&m.Embed{Field: "namespaceSelector"},
			&m.Drop{Field: "any"},
			&m.Move{From: "matchNames", To: "namespaceSelector"},
		).
		AddMapperForType(&Version, monitoringv1.ServiceMonitor{},
			&m.AnnotationField{Field: "displayName"},
			&m.DisplayName{},
			&m.AnnotationField{Field: "targetService"},
			&m.AnnotationField{Field: "targetWorkload"},
		).
		MustImport(&Version, monitoringv1.ServiceMonitor{}, projectOverride{}, struct {
			DisplayName    string `json:"displayName,omitempty"`
			TargetService  string `json:"targetService,omitempty"`
			TargetWorkload string `json:"targetWorkload,omitempty"`
		}{}).
		MustImport(&Version, monitoringv1.PrometheusRule{}, projectOverride{}).
		AddMapperForType(&Version, monitoringv1.Alertmanager{},
			&m.Drop{Field: "status"},
		).
		MustImport(&Version, monitoringv1.Alertmanager{}, projectOverride{})
}

func autoscalingTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, autoscaling.HorizontalPodAutoscaler{},
			&m.ChangeType{Field: "scaleTargetRef", Type: "reference[workload]"},
			&m.Move{From: "scaleTargetRef", To: "workloadId"},
			mapper.CrossVersionObjectToWorkload{Field: "workloadId"},
			&m.Required{Fields: []string{"workloadId", "maxReplicas"}},
			&m.AnnotationField{Field: "displayName"},
			&m.DisplayName{},
			&m.AnnotationField{Field: "description"},
			&m.Embed{Field: "status"},
			mapper.NewMergeListByIndexMapper("currentMetrics", "metrics", "type"),
		).
		AddMapperForType(&Version, autoscaling.MetricTarget{},
			&m.Enum{Field: "type", Options: []string{"Utilization", "Value", "AverageValue"}},
			&m.Move{To: "utilization", From: "averageUtilization"},
		).
		AddMapperForType(&Version, autoscaling.MetricValueStatus{},
			&m.Move{To: "utilization", From: "averageUtilization"},
		).
		AddMapperForType(&Version, autoscaling.MetricSpec{},
			&m.Condition{Field: "type", Value: "Object", Mapper: types.Mappers{
				&m.Move{To: "target", From: "object/target", DestDefined: true, NoDeleteFromField: true},
				&m.Move{To: "metric", From: "object/metric", DestDefined: true, NoDeleteFromField: true},
			}},
			&m.Condition{Field: "type", Value: "Pods", Mapper: types.Mappers{
				&m.Move{To: "target", From: "pods/target", DestDefined: true, NoDeleteFromField: true},
				&m.Move{To: "metric", From: "pods/metric", DestDefined: true, NoDeleteFromField: true},
			}},
			&m.Condition{Field: "type", Value: "Resource", Mapper: types.Mappers{
				&m.Move{To: "metric/name", From: "resource/name", DestDefined: true, NoDeleteFromField: true},
				&m.Move{To: "target", From: "resource/target", DestDefined: true, NoDeleteFromField: true},
			}},
			&m.Condition{Field: "type", Value: "External", Mapper: types.Mappers{
				&m.Move{To: "target", From: "external/target", DestDefined: true, NoDeleteFromField: true},
				&m.Move{To: "metric", From: "external/metric", DestDefined: true, NoDeleteFromField: true},
			}},
			&m.Embed{Field: "object", Ignore: []string{"target", "metric"}},
			&m.Embed{Field: "pods", Ignore: []string{"target", "metric"}},
			&m.Embed{Field: "external", Ignore: []string{"target", "metric"}},
			&m.Embed{Field: "resource", Ignore: []string{"target", "name"}},
			&m.Embed{Field: "metric"},
			&m.Enum{Field: "type", Options: []string{"Object", "Pods", "Resource", "External"}},
		).
		MustImportAndCustomize(&Version, autoscaling.MetricSpec{}, func(s *types.Schema) {
			s.CodeName = "Metric"
			s.PluralName = "metrics"
			s.ID = "metric"
			s.CodeNamePlural = "Metrics"
		}, struct {
			Target  autoscaling.MetricTarget      `json:"target"`
			Metric  autoscaling.MetricIdentifier  `json:"metric"`
			Current autoscaling.MetricValueStatus `json:"current" norman:"nocreate,noupdate"`
		}{}).
		AddMapperForType(&Version, autoscaling.MetricStatus{},
			&m.Condition{Field: "type", Value: "Object", Mapper: &m.Move{To: "current", From: "object/current", DestDefined: true, NoDeleteFromField: true}},
			&m.Condition{Field: "type", Value: "Pods", Mapper: &m.Move{To: "current", From: "pods/current", DestDefined: true, NoDeleteFromField: true}},
			&m.Condition{Field: "type", Value: "Resource", Mapper: &m.Move{To: "current", From: "resource/current"}},
			&m.Condition{Field: "type", Value: "External", Mapper: &m.Move{To: "current", From: "external/current", DestDefined: true, NoDeleteFromField: true}},
		).
		MustImport(&Version, autoscaling.HorizontalPodAutoscaler{}, projectOverride{}, struct {
			DisplayName string `json:"displayName,omitempty"`
			Description string `json:"description,omitempty"`
		}{})
}
