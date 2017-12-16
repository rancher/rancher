package schema

import (
	"github.com/rancher/norman/types"
	m "github.com/rancher/norman/types/mapper"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/factory"
	"github.com/rancher/types/mapper"
	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/staging/src/k8s.io/api/apps/v1beta2"
)

var (
	Version = types.APIVersion{
		Version: "v3",
		Group:   "project.cattle.io",
		Path:    "/v3/projects",
		SubContexts: map[string]bool{
			"projects": true,
		},
	}

	Schemas = factory.Schemas(&Version).
		// Namespace must be first
		Init(namespaceTypes).
		Init(podTypes).
		Init(deploymentTypes).
		Init(statefulSetTypes).
		Init(replicaSet).
		Init(replicationController).
		Init(daemonSet).
		Init(workloadTypes)
)

func namespaceTypes(schemas *types.Schemas) *types.Schemas {
	return NamespaceTypes(&Version, schemas)
}

func NamespaceTypes(version *types.APIVersion, schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(version, v1.NamespaceStatus{},
			&m.Drop{Field: "phase"},
		).
		AddMapperForType(version, v1.NamespaceSpec{},
			&m.Drop{Field: "finalizers"},
		).
		AddMapperForType(version, v1.Namespace{},
			&m.LabelField{Field: "projectId"},
		).
		MustImport(version, v1.Namespace{}, struct {
			ProjectID  string `norman:"type=reference[/v3/schemas/project]"`
			Templates  map[string]string
			Answers    map[string]interface{}
			Prune      bool
			ExternalID string
			Tags       []string
		}{})
}

func workloadTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v3.WorkloadSpec{},
			&m.Embed{Field: "deployConfig"},
			&m.Embed{Field: "template"},
		).
		AddMapperForType(&Version, v3.Workload{}, mapper.NewWorkloadTypeMapper()).
		MustImport(&Version, v3.Workload{}, projectOverride{})
}

func statefulSetTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1beta2.StatefulSetSpec{},
			&m.Move{
				From:        "replicas",
				To:          "scale",
				DestDefined: true,
			},
			&m.Move{
				From: "updateStrategy/rollingUpdate/partition",
				To:   "deploymentStrategy/orderedConfig/partition",
			},
			m.SetValue{
				From:  "updateStrategy/type",
				IfEq:  "OnDelete",
				Value: true,
				To:    "deploymentStrategy/orderedConfig/onDelete",
			},
			m.SetValue{
				From:  "podManagementPolicy",
				IfEq:  "Parallel",
				Value: "Parallel",
				To:    "deploymentStrategy/kind",
			},
			m.SetValue{
				From:  "podManagementPolicy",
				IfEq:  "OrderedReady",
				Value: "Ordered",
				To:    "deploymentStrategy/kind",
			},
			m.Drop{Field: "selector"},
			&m.Embed{Field: "template"},
		).
		AddMapperForType(&Version, v1beta2.StatefulSet{}, mapper.NewWorkloadTypeMapper()).
		MustImport(&Version, v1beta2.StatefulSetSpec{}, deployOverride{}).
		MustImport(&Version, v1beta2.StatefulSet{}, projectOverride{})
}

func replicaSet(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1beta2.ReplicaSetSpec{},
			&m.Move{
				From:        "replicas",
				To:          "scale",
				DestDefined: true,
			},
			&m.Move{
				From: "minReadySeconds",
				To:   "deploymentStrategy/parallelConfig/minReadySeconds",
			},
			m.Drop{Field: "selector"},
			&m.Embed{Field: "template"},
		).
		AddMapperForType(&Version, v1beta2.ReplicaSet{}, mapper.NewWorkloadTypeMapper()).
		MustImport(&Version, v1beta2.ReplicaSetSpec{}, deployOverride{}).
		MustImportAndCustomize(&Version, v1beta2.ReplicaSet{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
		}, projectOverride{})
}

func replicationController(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1.ReplicationControllerSpec{},
			&m.Move{
				From:        "replicas",
				To:          "scale",
				DestDefined: true,
			},
			&m.Move{
				From: "minReadySeconds",
				To:   "deploymentStrategy/parallelConfig/minReadySeconds",
			},
			m.Drop{Field: "selector"},
			&m.Embed{Field: "template"},
		).
		AddMapperForType(&Version, v1.ReplicationController{}, mapper.NewWorkloadTypeMapper()).
		MustImport(&Version, v1.ReplicationControllerSpec{}, deployOverride{}).
		MustImportAndCustomize(&Version, v1.ReplicationController{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
		}, projectOverride{})
}

func daemonSet(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1beta2.DaemonSetSpec{},
			m.SetValue{
				From:  "updateStrategy/type",
				IfEq:  "OnDelete",
				Value: true,
				To:    "deploymentStrategy/globalConfig/onDelete",
			},
			&m.Move{
				From: "minReadySeconds",
				To:   "deploymentStrategy/globalConfig/minReadySeconds",
			},
			m.Drop{Field: "selector"},
			&m.Embed{Field: "template"},
		).
		AddMapperForType(&Version, v1beta2.DaemonSet{}, mapper.NewWorkloadTypeMapper()).
		MustImport(&Version, v1beta2.DaemonSetSpec{}, deployOverride{}).
		MustImportAndCustomize(&Version, v1beta2.DaemonSet{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
		}, projectOverride{})
}

func deploymentTypes(schemas *types.Schemas) *types.Schemas {
	return schemas.
		AddMapperForType(&Version, v1beta2.DeploymentSpec{},
			&m.Move{
				From:        "replicas",
				To:          "scale",
				DestDefined: true,
			},
			&m.Move{
				From: "minReadySeconds",
				To:   "deploymentStrategy/parallelConfig/minReadySeconds",
			},
			&m.Move{
				From: "progressDeadlineSeconds",
				To:   "deploymentStrategy/parallelConfig/progressDeadlineSeconds",
			},
			mapper.DeploymentStrategyMapper{},
			m.Drop{Field: "selector"},
			m.Drop{Field: "strategy"},
			&m.Embed{Field: "template"},
		).
		AddMapperForType(&Version, v1beta2.Deployment{}, mapper.NewWorkloadTypeMapper()).
		MustImport(&Version, v1beta2.DeploymentSpec{}, deployOverride{}).
		MustImportAndCustomize(&Version, v1beta2.Deployment{}, func(schema *types.Schema) {
			schema.BaseType = "workload"
		}, projectOverride{})
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
			m.Move{From: "livenessProbe", To: "healthcheck"},
			m.Move{From: "readinessProbe", To: "readycheck"},
			m.Move{From: "imagePullPolicy", To: "pullPolicy"},
			mapper.EnvironmentMapper{},
			&m.Embed{Field: "securityContext"},
			&m.Embed{Field: "lifecycle"},
		).
		AddMapperForType(&Version, v1.ContainerPort{},
			m.Drop{Field: "name"},
			m.Move{From: "hostIP", To: "hostIp"},
		).
		AddMapperForType(&Version, v1.VolumeMount{},
			m.Enum{
				Field: "mountPropagation",
				Values: map[string][]string{
					"HostToContainer": {"rslave"},
					"Bidirectional":   {"rshared", "shared"},
				},
			},
		).
		AddMapperForType(&Version, v1.Handler{}, handlerMapper).
		AddMapperForType(&Version, v1.Probe{}, handlerMapper).
		AddMapperForType(&Version, v1.PodStatus{},
			m.Move{From: "hostIP", To: "nodeIp"},
			m.Move{From: "podIP", To: "podIp"},
		).
		AddMapperForType(&Version, v1.PodSpec{},
			m.Move{From: "restartPolicy", To: "restart"},
			m.Move{From: "imagePullSecrets", To: "pullSecrets"},
			mapper.NamespaceMapper{},
			mapper.InitContainerMapper{},
			mapper.SchedulingMapper{},
			&m.Embed{Field: "securityContext"},
			&m.Drop{Field: "serviceAccount"},
			&m.SliceToMap{
				Field: "volumes",
				Key:   "name",
			},
			&m.SliceToMap{
				Field: "containers",
				Key:   "name",
			},
			&m.SliceToMap{
				Field: "hostAliases",
				Key:   "ip",
			},
		).
		AddMapperForType(&Version, v1.ResourceRequirements{},
			mapper.PivotMapper{Plural: true},
		).
		AddMapperForType(&Version, v1.Pod{},
			&mapper.NamespaceIDMapper{},
		).
		// Must import handlers before Container
		MustImport(&Version, v1.Capabilities{}, struct {
			Add  []string `norman:"type=array[enum],options=AUDIT_CONTROL|AUDIT_WRITE|BLOCK_SUSPEND|CHOWN|DAC_OVERRIDE|DAC_READ_SEARCH|FOWNER|FSETID|IPC_LOCK|IPC_OWNER|KILL|LEASE|LINUX_IMMUTABLE|MAC_ADMIN|MAC_OVERRIDE|MKNOD|NET_ADMIN|NET_BIND_SERVICE|NET_BROADCAST|NET_RAW|SETFCAP|SETGID|SETPCAP|SETUID|SYSLOG|SYS_ADMIN|SYS_BOOT|SYS_CHROOT|SYS_MODULE|SYS_NICE|SYS_PACCT|SYS_PTRACE|SYS_RAWIO|SYS_RESOURCE|SYS_TIME|SYS_TTY_CONFIG|WAKE_ALARM"`
			Drop []string `norman:"type=array[enum],options=AUDIT_CONTROL|AUDIT_WRITE|BLOCK_SUSPEND|CHOWN|DAC_OVERRIDE|DAC_READ_SEARCH|FOWNER|FSETID|IPC_LOCK|IPC_OWNER|KILL|LEASE|LINUX_IMMUTABLE|MAC_ADMIN|MAC_OVERRIDE|MKNOD|NET_ADMIN|NET_BIND_SERVICE|NET_BROADCAST|NET_RAW|SETFCAP|SETGID|SETPCAP|SETUID|SYSLOG|SYS_ADMIN|SYS_BOOT|SYS_CHROOT|SYS_MODULE|SYS_NICE|SYS_PACCT|SYS_PTRACE|SYS_RAWIO|SYS_RESOURCE|SYS_TIME|SYS_TTY_CONFIG|WAKE_ALARM"`
		}{}).
		MustImport(&Version, v1.Handler{}, handlerOverride{}).
		MustImport(&Version, v1.Probe{}, handlerOverride{}).
		MustImport(&Version, v1.Container{}, struct {
			Scheduling      *Scheduling
			Resources       *Resources
			Environment     map[string]string
			EnvironmentFrom []EnvironmentFrom
			InitContainer   bool
		}{}).
		MustImport(&Version, v1.PodSpec{}, struct {
			NodeName string `norman:"type=reference[node]"`
			Net      string
			PID      string
			IPC      string
		}{}).
		MustImport(&Version, v1.Pod{}, projectOverride{}, struct {
			WorkloadID string `norman:"type=reference[workload]"`
		}{})
}
