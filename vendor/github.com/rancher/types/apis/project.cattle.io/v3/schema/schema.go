package schema

import (
	"net/http"

	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/factory"
	"github.com/rancher/types/mapper"
	"k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
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
		Init(configMapTypes)
)

func configMapTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.MustImport(&Version, v1.ConfigMap{})
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
			schema.MustCustomizeField("name", func(field types.Field) types.Field {
				field.Type = "dnsLabel"
				field.Nullable = false
				field.Required = true
				return field
			})
		})
}

func statefulSetTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1beta2.StatefulSetUpdateStrategy{},
			&m.Embed{Field: "rollingUpdate"},
			m.Enum{Field: "type", Options: []string{
				"RollingUpdate",
				"OnDelete",
			}},
			m.Move{From: "type", To: "strategy"},
		).
		AddMapperForType(&Version, v1beta2.StatefulSetSpec{},
			&m.Move{
				From:        "replicas",
				To:          "scale",
				DestDefined: true,
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
		AddMapperForType(&Version, v1beta2.StatefulSet{},
			&m.Move{
				From: "status",
				To:   "statefulSetStatus",
			},
			NewWorkloadTypeMapper(),
		).
		MustImport(&Version, v1beta2.StatefulSetSpec{}, statefulSetConfigOverride{}).
		MustImportAndCustomize(&Version, v1beta2.StatefulSet{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
		}{})
}

func replicaSetTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1beta1.ReplicaSetSpec{},
			&m.Move{
				From:        "replicas",
				To:          "scale",
				DestDefined: true,
			},
			&m.Move{
				From: "minReadySeconds",
				To:   "replicaSetConfig/minReadySeconds",
			},
		).
		AddMapperForType(&Version, v1beta1.ReplicaSet{},
			&m.Move{
				From: "status",
				To:   "replicaSetStatus",
			},
			&m.Embed{Field: "template"},
			NewWorkloadTypeMapper(),
		).
		MustImport(&Version, v1beta1.ReplicaSetSpec{}, replicaSetConfigOverride{}).
		MustImportAndCustomize(&Version, v1beta1.ReplicaSet{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
		}{})
}

func replicationControllerTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1.ReplicationControllerSpec{},
			&m.Move{
				From:        "replicas",
				To:          "scale",
				DestDefined: true,
			},
			&m.Move{
				From: "minReadySeconds",
				To:   "replicationControllerConfig/minReadySeconds",
			},
			&m.Embed{Field: "template"},
		).
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
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
		}{})
}

func daemonSetTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1beta2.DaemonSetUpdateStrategy{},
			&m.Embed{Field: "rollingUpdate"},
			m.Enum{Field: "type", Options: []string{
				"RollingUpdate",
				"OnDelete",
			}},
			m.Move{From: "type", To: "strategy"},
		).
		AddMapperForType(&Version, v1beta2.DaemonSetSpec{},
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
		AddMapperForType(&Version, v1beta2.DaemonSet{},
			&m.Move{
				From: "status",
				To:   "daemonSetStatus",
			},
			NewWorkloadTypeMapper(),
		).
		MustImport(&Version, v1beta2.DaemonSetSpec{}, daemonSetOverride{}).
		MustImportAndCustomize(&Version, v1beta2.DaemonSet{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
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
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
		}{})
}

func cronJobTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, batchv1beta1.JobTemplateSpec{},
			&m.Move{
				From: "metadata",
				To:   "jobMetadata",
			},
			&m.Embed{Field: "spec"},
		).
		AddMapperForType(&Version, batchv1beta1.CronJobSpec{},
			&m.Embed{
				Field: "jobTemplate",
			},
			&m.BatchMove{
				From: []string{
					"schedule",
					"startingDeadlineSeconds",
					"concurrencyPolicy",
					"suspend",
					"successfulJobsHistoryLimit",
					"failedJobsHistoryLimit",
					"jobConfig",
				},
				To: "cronJobConfig",
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
		AddMapperForType(&Version, batchv1beta1.CronJob{},
			&m.Move{
				From: "status",
				To:   "cronJobStatus",
			},
			NewWorkloadTypeMapper(),
		).
		MustImport(&Version, batchv1beta1.CronJobSpec{}, cronJobOverride{}).
		MustImport(&Version, batchv1beta1.JobTemplateSpec{}).
		MustImportAndCustomize(&Version, batchv1beta1.CronJob{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
		}{})
}

func deploymentTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1beta2.DeploymentStrategy{},
			&m.Embed{Field: "rollingUpdate"},
			m.Enum{Field: "type", Options: []string{
				"Recreate",
				"RollingUpdate",
			}},
			m.Move{From: "type", To: "strategy"},
		).
		AddMapperForType(&Version, v1beta2.DeploymentSpec{},
			&m.Embed{Field: "strategy"},
			&m.Move{
				From: "replicas",
				To:   "scale",
			},
			&m.BatchMove{
				From: []string{
					"minReadySeconds",
					"strategy",
					"revisionHistoryLimit",
					"paused",
					"progressDeadlineSeconds",
					"maxUnavailable",
					"maxSurge",
				},
				To: "deploymentConfig",
			},
			&m.Embed{Field: "template"},
		).
		AddMapperForType(&Version, v1beta2.Deployment{},
			&m.Move{
				From: "status",
				To:   "deploymentStatus",
			},
			NewWorkloadTypeMapper(),
		).
		MustImport(&Version, v1beta2.DeploymentSpec{}, deploymentConfigOverride{}).
		MustImportAndCustomize(&Version, v1beta2.Deployment{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
		}, projectOverride{}, struct {
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
		}{})
}

func podTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1.PodTemplateSpec{},
			&m.Embed{Field: "spec"},
		).
		AddMapperForType(&Version, v1.HTTPGetAction{},
			&m.Drop{Field: "host"},
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
			mapper.EnvironmentMapper{},
			&m.Embed{Field: "securityContext"},
			&m.Embed{Field: "lifecycle"},
		).
		AddMapperForType(&Version, v1.ContainerPort{},
			m.Move{From: "hostIP", To: "hostIp"},
			m.Drop{Field: "hostPort"},
		).
		AddMapperForType(&Version, v1.Handler{}, handlerMapper).
		AddMapperForType(&Version, v1.Probe{}, handlerMapper).
		AddMapperForType(&Version, v1.PodStatus{},
			m.Move{From: "hostIP", To: "nodeIp"},
			m.Move{From: "podIP", To: "podIp"},
		).
		AddMapperForType(&Version, v1.PodSpec{},
			mapper.InitContainerMapper{},
			mapper.SchedulingMapper{},
			m.Move{From: "tolerations", To: "scheduling/tolerations", DestDefined: true},
			&m.Embed{Field: "securityContext"},
			&m.Drop{Field: "serviceAccount"},
		).
		AddMapperForType(&Version, v1.ResourceRequirements{},
			mapper.PivotMapper{Plural: true},
		).
		AddMapperForType(&Version, v1.Pod{},
			&m.AnnotationField{Field: "description"},
			&m.AnnotationField{Field: "publicEndpoints", List: true},
			mapper.ContainerPorts{},
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
			Add  []string `norman:"type=array[enum],options=AUDIT_CONTROL|AUDIT_WRITE|BLOCK_SUSPEND|CHOWN|DAC_OVERRIDE|DAC_READ_SEARCH|FOWNER|FSETID|IPC_LOCK|IPC_OWNER|KILL|LEASE|LINUX_IMMUTABLE|MAC_ADMIN|MAC_OVERRIDE|MKNOD|NET_ADMIN|NET_BIND_SERVICE|NET_BROADCAST|NET_RAW|SETFCAP|SETGID|SETPCAP|SETUID|SYSLOG|SYS_ADMIN|SYS_BOOT|SYS_CHROOT|SYS_MODULE|SYS_NICE|SYS_PACCT|SYS_PTRACE|SYS_RAWIO|SYS_RESOURCE|SYS_TIME|SYS_TTY_CONFIG|WAKE_ALARM"`
			Drop []string `norman:"type=array[enum],options=AUDIT_CONTROL|AUDIT_WRITE|BLOCK_SUSPEND|CHOWN|DAC_OVERRIDE|DAC_READ_SEARCH|FOWNER|FSETID|IPC_LOCK|IPC_OWNER|KILL|LEASE|LINUX_IMMUTABLE|MAC_ADMIN|MAC_OVERRIDE|MKNOD|NET_ADMIN|NET_BIND_SERVICE|NET_BROADCAST|NET_RAW|SETFCAP|SETGID|SETPCAP|SETUID|SYSLOG|SYS_ADMIN|SYS_BOOT|SYS_CHROOT|SYS_MODULE|SYS_NICE|SYS_PACCT|SYS_PTRACE|SYS_RAWIO|SYS_RESOURCE|SYS_TIME|SYS_TTY_CONFIG|WAKE_ALARM"`
		}{}).
		MustImport(&Version, v3.PublicEndpoint{}).
		MustImport(&Version, v1.Handler{}, handlerOverride{}).
		MustImport(&Version, v1.Probe{}, handlerOverride{}).
		MustImport(&Version, v1.Container{}, struct {
			Resources       *Resources
			Environment     map[string]string
			EnvironmentFrom []EnvironmentFrom
			InitContainer   bool
		}{}).
		MustImport(&Version, v1.PodSpec{}, struct {
			Scheduling *Scheduling
			NodeName   string `norman:"type=reference[/v3/schemas/node]"`
		}{}).
		MustImport(&Version, v1.Pod{}, projectOverride{}, struct {
			Description     string `json:"description"`
			WorkloadID      string `norman:"type=reference[workload]"`
			PublicEndpoints string `json:"publicEndpoints" norman:"type=array[publicEndpoint],nocreate,noupdate"`
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
				&ServiceSpecMapper{},
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
		AddMapperForType(&Version, v1beta1.HTTPIngressRuleValue{},
			&m.SliceToMap{Field: "paths", Key: "path"},
		).
		AddMapperForType(&Version, v1beta1.HTTPIngressPath{},
			&m.Embed{Field: "backend"},
		).
		AddMapperForType(&Version, v1beta1.IngressRule{},
			&m.Embed{Field: "http"},
		).
		AddMapperForType(&Version, v1beta1.Ingress{},
			&m.AnnotationField{Field: "description"},
			&m.Move{From: "backend", To: "defaultBackend"},
		).
		AddMapperForType(&Version, v1beta1.IngressTLS{},
			&m.Move{From: "secretName", To: "certificateName"},
		).
		AddMapperForType(&Version, v1beta1.IngressBackend{},
			&m.Move{From: "servicePort", To: "targetPort"},
		).
		MustImport(&Version, v1beta1.IngressBackend{}, struct {
			WorkloadIDs string `json:"workloadIds" norman:"type=array[reference[workload]]"`
			ServiceName string `norman:"type=reference[service]"`
		}{}).
		MustImportAndCustomize(&Version, v1beta1.IngressRule{}, func(schema *types.Schema) {
			schema.MustCustomizeField("paths", func(f types.Field) types.Field {
				f.Type = "map[ingressBackend]"
				return f
			})
		}).
		MustImport(&Version, v1beta1.IngressTLS{}, struct {
			SecretName string `norman:"type=reference[certificate]"`
		}{}).
		MustImport(&Version, v1beta1.Ingress{}, projectOverride{}, struct {
			Description string `json:"description"`
		}{})
}

func volumeTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1.ResourceRequirements{},
			mapper.PivotMapper{Plural: true},
		).
		AddMapperForType(&Version, v1.PersistentVolumeClaimVolumeSource{},
			&m.Move{From: "claimName", To: "persistentVolumeClaimName"},
		).
		MustImport(&Version, v1.PersistentVolumeClaimVolumeSource{}, struct {
			ClaimName string `norman:"type=reference[persistentVolumeClaim]"`
		}{}).
		MustImport(&Version, v1.SecretVolumeSource{}, struct {
			SecretName string `norman:"type=reference[secret]"`
		}{}).
		MustImport(&Version, v1.Volume{}, struct {
		}{}).
		MustImport(&Version, v1.PersistentVolumeClaimSpec{}, struct {
			AccessModes      []string `json:"accessModes,omitempty" norman:"type=array[enum],options=ReadWriteOnce|ReadOnlyMany|ReadWriteMany"`
			VolumeName       string   `json:"volumeName,omitempty" norman:"type=reference[/v3/cluster/persistentVolume]"`
			StorageClassName *string  `json:"storageClassName,omitempty" norman:"type=reference[/v3/cluster/storageClass]"`
		}{}).
		MustImport(&Version, v1.PersistentVolumeClaim{}, projectOverride{})
}

func appTypes(schema *types.Schemas) *types.Schemas {
	return schema.
		MustImportAndCustomize(&Version, v3.App{}, func(schema *types.Schema) {
			schema.ResourceActions = map[string]types.Action{
				"upgrade": {
					Input: "templateVersionId",
				},
				"rollback": {
					Input: "revision",
				},
			}
		})
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
		&m.AnnotationField{Field: "publicEndpoints", List: true},
	}
}
