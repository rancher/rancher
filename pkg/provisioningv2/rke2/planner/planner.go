package planner

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/moby/locker"
	"github.com/rancher/norman/types/values"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/nodeconfig"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	rancherruntime "github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/kv"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/randomtoken"
	"github.com/rancher/wrangler/pkg/summary"
	"github.com/rancher/wrangler/pkg/yaml"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

const (
	clusterRegToken   = "clusterRegToken"
	JoinURLAnnotation = "rke.cattle.io/join-url"

	NodeNameLabel              = "rke.cattle.io/node-name"
	InitNodeLabel              = "rke.cattle.io/init-node"
	InitNodeMachineIDLabel     = "rke.cattle.io/init-node-machine-id"
	InitNodeMachineIDDoneLabel = "rke.cattle.io/init-node-machine-id-done"
	EtcdRoleLabel              = "rke.cattle.io/etcd-role"
	WorkerRoleLabel            = "rke.cattle.io/worker-role"
	ControlPlaneRoleLabel      = "rke.cattle.io/control-plane-role"
	MachineUIDLabel            = "rke.cattle.io/machine"
	MachineIDLabel             = "rke.cattle.io/machine-id"
	CapiMachineLabel           = "cluster.x-k8s.io/cluster-name"

	MachineNameLabel      = "rke.cattle.io/machine-name"
	MachineNamespaceLabel = "rke.cattle.io/machine-namespace"

	LabelsAnnotation = "rke.cattle.io/labels"
	TaintsAnnotation = "rke.cattle.io/taints"

	AddressAnnotation         = "rke.cattle.io/address"
	InternalAddressAnnotation = "rke.cattle.io/internal-address"

	SecretTypeMachinePlan = "rke.cattle.io/machine-plan"

	authnWebhookFileName = "/var/lib/rancher/%s/kube-api-authn-webhook.yaml"
	ConfigYamlFileName   = "/etc/rancher/%s/config.yaml.d/50-rancher.yaml"
	Provisioned          = condition.Cond("Provisioned")
)

var (
	fileParams = []string{
		"audit-policy-file",
		"cloud-provider-config",
		"private-registry",
		"flannel-conf",
	}
	filePaths = map[string]string{
		"private-registry": "/etc/rancher/%s/registries.yaml",
	}
	AuthnWebhook = []byte(`
apiVersion: v1
kind: Config
clusters:
- name: Default
  cluster:
    insecure-skip-tls-verify: true
    server: http://127.0.0.1:6440/v1/authenticate
users:
- name: Default
  user:
    insecure-skip-tls-verify: true
current-context: webhook
contexts:
- name: webhook
  context:
    user: Default
    cluster: Default
`)
)

type ErrWaiting string

func (e ErrWaiting) Error() string {
	return string(e)
}

type errIgnore string

func (e errIgnore) Error() string {
	return string(e)
}

type roleFilter func(machine *capi.Machine) bool

type Planner struct {
	ctx                           context.Context
	store                         *PlanStore
	rkeControlPlanes              rkecontrollers.RKEControlPlaneClient
	secretClient                  corecontrollers.SecretClient
	secretCache                   corecontrollers.SecretCache
	machines                      capicontrollers.MachineClient
	clusterRegistrationTokenCache mgmtcontrollers.ClusterRegistrationTokenCache
	capiClusters                  capicontrollers.ClusterCache
	managementClusters            mgmtcontrollers.ClusterCache
	kubeconfig                    *kubeconfig.Manager
	locker                        locker.Locker
	etcdRestore                   *etcdRestore
	etcdCreate                    *etcdCreate
	etcdArgs                      s3Args
}

func New(ctx context.Context, clients *wrangler.Context) *Planner {
	clients.Mgmt.ClusterRegistrationToken().Cache().AddIndexer(clusterRegToken, func(obj *v3.ClusterRegistrationToken) ([]string, error) {
		return []string{obj.Spec.ClusterName}, nil
	})
	store := NewStore(clients.Core.Secret(),
		clients.CAPI.Machine().Cache())
	return &Planner{
		ctx:                           ctx,
		store:                         store,
		machines:                      clients.CAPI.Machine(),
		secretClient:                  clients.Core.Secret(),
		secretCache:                   clients.Core.Secret().Cache(),
		clusterRegistrationTokenCache: clients.Mgmt.ClusterRegistrationToken().Cache(),
		capiClusters:                  clients.CAPI.Cluster().Cache(),
		managementClusters:            clients.Mgmt.Cluster().Cache(),
		rkeControlPlanes:              clients.RKE.RKEControlPlane(),
		kubeconfig:                    kubeconfig.New(clients),
		etcdRestore:                   newETCDRestore(clients, store),
		etcdCreate:                    newETCDCreate(clients, store),
		etcdArgs: s3Args{
			prefix:      "etcd-",
			secretCache: clients.Core.Secret().Cache(),
		},
	}
}

func PlanSecretFromBootstrapName(bootstrapName string) string {
	return name.SafeConcatName(bootstrapName, "machine", "plan")
}

func (p *Planner) getCAPICluster(controlPlane *rkev1.RKEControlPlane) (*capi.Cluster, error) {
	ref := metav1.GetControllerOf(controlPlane)
	if ref == nil {
		return nil, generic.ErrSkip
	}
	gvk := schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind)
	if gvk.Kind != "Cluster" || gvk.Group != "cluster.x-k8s.io" {
		return nil, fmt.Errorf("RKEControlPlane %s/%s has wrong owner kind %s/%s", controlPlane.Namespace,
			controlPlane.Name, ref.APIVersion, ref.Kind)
	}
	return p.capiClusters.Get(controlPlane.Namespace, ref.Name)
}

func (p *Planner) Process(controlPlane *rkev1.RKEControlPlane) error {
	p.locker.Lock(string(controlPlane.UID))
	defer p.locker.Unlock(string(controlPlane.UID))

	cluster, err := p.getCAPICluster(controlPlane)
	if err != nil {
		return err
	}

	plan, err := p.store.Load(cluster)
	if err != nil {
		return err
	}

	controlPlane, secret, err := p.generateSecrets(controlPlane)
	if err != nil {
		return err
	}

	var (
		firstIgnoreError error
		joinServer       string
	)

	if errs := p.etcdCreate.Create(controlPlane, plan); len(errs) > 0 {
		var errMsg string
		for i, err := range errs {
			if err == nil {
				continue
			}
			if i == 0 {
				errMsg = err.Error()
			} else {
				errMsg = errMsg + ", " + err.Error()
			}
		}
		return ErrWaiting(errMsg)
	}

	if _, err := p.electInitNode(controlPlane, plan); err != nil {
		return err
	}

	if err := p.etcdRestore.Restore(controlPlane, plan); err != nil {
		return err
	}

	err = p.reconcile(controlPlane, secret, plan, "bootstrap", true, isInitNode, isDeleting,
		controlPlane.Spec.UpgradeStrategy.ControlPlaneConcurrency, "",
		controlPlane.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return err
	}

	joinServer, err = p.electInitNode(controlPlane, plan)
	if err != nil {
		return err
	} else if joinServer == "" && firstIgnoreError != nil {
		return ErrWaiting(firstIgnoreError.Error() + " and join url to be available on bootstrap node")
	} else if joinServer == "" {
		return ErrWaiting("waiting for join url to be available on bootstrap node")
	}

	err = p.reconcile(controlPlane, secret, plan, "etcd", true, isEtcd, isInitNodeOrDeleting,
		controlPlane.Spec.UpgradeStrategy.ControlPlaneConcurrency, joinServer,
		controlPlane.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return err
	}

	err = p.reconcile(controlPlane, secret, plan, "control plane", true, isControlPlane, isInitNodeOrDeleting,
		controlPlane.Spec.UpgradeStrategy.ControlPlaneConcurrency, joinServer,
		controlPlane.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return err
	}

	joinServer = p.getControlPlaneJoinURL(plan)
	if joinServer == "" {
		return ErrWaiting("waiting for control plane to be available")
	}

	err = p.reconcile(controlPlane, secret, plan, "worker", false, isOnlyWorker, isInitNodeOrDeleting,
		controlPlane.Spec.UpgradeStrategy.WorkerConcurrency, joinServer,
		controlPlane.Spec.UpgradeStrategy.WorkerDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return err
	}

	if firstIgnoreError != nil {
		return ErrWaiting(firstIgnoreError.Error())
	}

	return nil
}

func ignoreErrors(firstIgnoreError error, err error) (error, error) {
	var errIgnore errIgnore
	if errors.As(err, &errIgnore) {
		if firstIgnoreError == nil {
			return err, nil
		}
		return firstIgnoreError, nil
	}
	return firstIgnoreError, err
}

func (p *Planner) clearInitNodeMark(machine *capi.Machine) error {
	if _, ok := machine.Labels[InitNodeLabel]; !ok {
		return nil
	}
	machine = machine.DeepCopy()
	delete(machine.Labels, InitNodeLabel)
	_, err := p.machines.Update(machine)
	if err != nil {
		return err
	}
	// We've changed state, so let the caches sync up again
	return generic.ErrSkip
}

func (p *Planner) setInitNodeMark(machine *capi.Machine) (*capi.Machine, error) {
	if machine.Labels[InitNodeLabel] == "true" {
		return machine, nil
	}
	machine = machine.DeepCopy()
	if machine.Labels == nil {
		machine.Labels = map[string]string{}
	}
	machine.Labels[InitNodeLabel] = "true"
	if _, err := p.machines.Update(machine); err != nil {
		return nil, err
	}
	// We've changed state, so let the caches sync up again
	return nil, generic.ErrSkip
}

func (p *Planner) getControlPlaneJoinURL(plan *plan.Plan) string {
	entries := collect(plan, isControlPlane)
	for _, entry := range entries {
		if entry.Machine.Annotations[JoinURLAnnotation] != "" {
			return entry.Machine.Annotations[JoinURLAnnotation]
		}
	}

	return ""
}

func (p *Planner) electInitNode(rkeControlPlane *rkev1.RKEControlPlane, plan *plan.Plan) (string, error) {
	// fixedMachineID is used when bootstrapping the local rancherd cluster to ensure the exact machine
	// gets picked for the init-not
	fixedMachineID := rkeControlPlane.Labels[InitNodeMachineIDLabel]
	if fixedMachineID != "" && rkeControlPlane.Labels[InitNodeMachineIDDoneLabel] == "" {
		entries := collect(plan, func(machine *capi.Machine) bool {
			return machine.Labels[MachineIDLabel] == fixedMachineID
		})
		if len(entries) != 1 {
			return "", nil
		}
		_, err := p.setInitNodeMark(entries[0].Machine)
		if err != nil {
			return "", err
		}
		rkeControlPlane = rkeControlPlane.DeepCopy()
		rkeControlPlane.Labels[InitNodeMachineIDDoneLabel] = "true"
		_, err = p.rkeControlPlanes.Update(rkeControlPlane)
		if err != nil {
			return "", err
		}
		return entries[0].Machine.Annotations[JoinURLAnnotation], nil
	}

	entries := collect(plan, isEtcd)
	joinURL := ""
	initNodeFound := false

	// Ensure we set our initNode to the initNode that is specified in the etcd snapshot restore
	if rkeControlPlane.Spec.ETCDSnapshotRestore != nil && rkeControlPlane.Spec.ETCDSnapshotRestore.S3 == nil &&
		rkeControlPlane.Status.ETCDSnapshotRestorePhase != rkev1.ETCDSnapshotPhaseFinished { // In the event that we are restoring a local snapshot, we
		// need to reset our initNode
		cacheInvalidated := false
		for _, entry := range entries {
			if entry.Machine.Status.NodeRef != nil &&
				entry.Machine.Status.NodeRef.Name == rkeControlPlane.Spec.ETCDSnapshotRestore.NodeName {
				// this is our new initNode
				if _, err := p.setInitNodeMark(entry.Machine); err != nil {
					if errors.Is(err, generic.ErrSkip) {
						cacheInvalidated = true
						continue
					}
					return "", err
				}

			} else {
				if err := p.clearInitNodeMark(entry.Machine); err != nil {
					if errors.Is(err, generic.ErrSkip) {
						cacheInvalidated = true
						continue
					}
					return "", err
				}
			}
		}
		if cacheInvalidated {
			return "", generic.ErrSkip
		}
	}

	for _, entry := range entries {
		if !isInitNode(entry.Machine) {
			continue
		}

		// Clear old or misconfigured init nodes
		if entry.Machine.DeletionTimestamp != nil || initNodeFound {
			if err := p.clearInitNodeMark(entry.Machine); err != nil {
				return "", err
			}
			continue
		}

		joinURL = entry.Machine.Annotations[JoinURLAnnotation]
		initNodeFound = true
	}

	if initNodeFound {
		// joinURL could still be blank at this point which is fine, we are just waiting then
		return joinURL, nil
	}

	for _, entry := range entries {
		if entry.Machine.DeletionTimestamp == nil {
			_, err := p.setInitNodeMark(entry.Machine)
			if err != nil {
				return "", err
			}
			return entry.Machine.Annotations[JoinURLAnnotation], nil
		}
	}

	return "", nil
}

func calculateConcurrency(maxUnavailable string, entries []planEntry, exclude roleFilter) (int, int, error) {
	var (
		count, unavailable int
	)

	for _, entry := range entries {
		if !exclude(entry.Machine) {
			count++
		}
		if entry.Plan != nil && !entry.Plan.InSync {
			unavailable++
		}
	}

	num, err := strconv.Atoi(maxUnavailable)
	if err == nil {
		return num, unavailable, nil
	}

	if maxUnavailable == "" {
		return 1, unavailable, nil
	}

	percentage, err := strconv.ParseFloat(strings.TrimSuffix(maxUnavailable, "%"), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("concurrency must be a number or a percentage: %w", err)
	}

	max := float64(count) * (percentage / float64(100))
	return int(math.Ceil(max)), unavailable, nil
}

func detailMessage(machines []string, messages map[string]string) string {
	if len(machines) != 1 {
		return ""
	}
	message := messages[machines[0]]
	if message != "" {
		return ": " + message
	}
	return ""
}

func (p *Planner) reconcile(controlPlane *rkev1.RKEControlPlane, secret plan.Secret, clusterPlan *plan.Plan,
	tierName string,
	required bool,
	include, exclude roleFilter, maxUnavailable string, joinServer string, drainOptions rkev1.DrainOptions) error {
	var (
		outOfSync   []string
		nonReady    []string
		errMachines []string
		draining    []string
		uncordoned  []string
		messages    = map[string]string{}
	)

	entries := collect(clusterPlan, include)

	concurrency, unavailable, err := calculateConcurrency(maxUnavailable, entries, exclude)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// we exclude here and not in collect to ensure that include matched at least one node
		if exclude(entry.Machine) {
			continue
		}

		summary := summary.Summarize(entry.Machine)
		if summary.Error {
			errMachines = append(errMachines, entry.Machine.Name)
		}
		if summary.Transitioning {
			nonReady = append(nonReady, entry.Machine.Name)
		}
		messages[entry.Machine.Name] = strings.Join(summary.Message, ", ")

		plan, err := p.desiredPlan(controlPlane, secret, entry, isInitNode(entry.Machine), joinServer)
		if err != nil {
			return err
		}

		if entry.Plan == nil {
			outOfSync = append(outOfSync, entry.Machine.Name)
			if err := p.store.UpdatePlan(entry.Machine, plan, 0); err != nil {
				return err
			}
		} else if !equality.Semantic.DeepEqual(entry.Plan.Plan, plan) {
			outOfSync = append(outOfSync, entry.Machine.Name)
			// Conditions
			// 1. If plan is not in sync then there is no harm in updating it to something else because
			//    the node will have already been considered unavailable.
			// 2. concurrency == 0 which means infinite concurrency.
			// 3. unavailable < concurrency meaning we have capacity to make something unavailable
			if !entry.Plan.InSync || concurrency == 0 || unavailable < concurrency {
				if entry.Plan.InSync {
					unavailable++
				}
				if ok, err := p.drain(entry.Machine, clusterPlan, drainOptions); err != nil {
					return err
				} else if ok {
					if err := p.store.UpdatePlan(entry.Machine, plan, 0); err != nil {
						return err
					}
				} else {
					draining = append(draining, entry.Machine.Name)
				}
			}
		} else if !entry.Plan.InSync && entry.Machine.Labels["cattle.io/os"] != "windows" {
			outOfSync = append(outOfSync, entry.Machine.Name)
		} else {
			if ok, err := p.undrain(entry.Machine); err != nil {
				return err
			} else if !ok {
				uncordoned = append(uncordoned, entry.Machine.Name)
			}
		}
	}

	if required && len(entries) == 0 {
		return ErrWaiting("waiting for at least one " + tierName + " node")
	}

	errMachines = atMostThree(errMachines)
	if len(errMachines) > 0 {
		// we want these errors to get reported, but not block the process
		return errIgnore("failing " + tierName + " machine(s) " + strings.Join(errMachines, ",") + detailMessage(errMachines, messages))
	}

	outOfSync = atMostThree(outOfSync)
	if len(outOfSync) > 0 {
		return ErrWaiting("provisioning " + tierName + " node(s) " + strings.Join(outOfSync, ",") + detailMessage(outOfSync, messages))
	}

	draining = atMostThree(draining)
	if len(draining) > 0 {
		return ErrWaiting("draining " + tierName + " node(s) " + strings.Join(draining, ",") + detailMessage(draining, messages))
	}

	uncordoned = atMostThree(uncordoned)
	if len(uncordoned) > 0 {
		return ErrWaiting("uncordoning " + tierName + " node(s) " + strings.Join(uncordoned, ",") + detailMessage(uncordoned, messages))
	}

	nonReady = atMostThree(nonReady)
	if len(nonReady) > 0 {
		// we want these errors to get reported, but not block the process
		return errIgnore("non-ready " + tierName + " machine(s) " + strings.Join(nonReady, ",") + detailMessage(nonReady, messages))
	}

	return nil
}

func atMostThree(names []string) []string {
	if len(names) == 0 {
		return names
	}
	sort.Strings(names)
	if len(names) > 3 {
		names = names[:3]
	}
	return names
}

func (p *Planner) addETCD(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (result []plan.File, _ error) {
	if !isEtcd(machine) || controlPlane.Spec.ETCD == nil {
		return nil, nil
	}

	if controlPlane.Spec.ETCD.DisableSnapshots {
		config["etcd-disable-snapshots"] = true
	}
	if controlPlane.Spec.ETCD.SnapshotRetention > 0 {
		config["etcd-snapshot-retention"] = controlPlane.Spec.ETCD.SnapshotRetention
	}
	if controlPlane.Spec.ETCD.SnapshotScheduleCron != "" {
		config["etcd-snapshot-schedule-cron"] = controlPlane.Spec.ETCD.SnapshotScheduleCron
	}

	args, _, files, err := p.etcdArgs.ToArgs(controlPlane.Spec.ETCD.S3, controlPlane)
	if err != nil {
		return nil, err
	}
	for _, arg := range args {
		k, v := kv.Split(arg, "=")
		k = strings.TrimPrefix(k, "--")
		if v == "" {
			config[k] = true
		} else {
			config[k] = v
		}
	}
	result = files

	return
}

func addDefaults(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) {
	if rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion) == rancherruntime.RuntimeRKE2 {
		config["cni"] = "calico"
		if settings.SystemDefaultRegistry.Get() != "" {
			config["system-default-registry"] = settings.SystemDefaultRegistry.Get()
		}
	}
}

func addUserConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) error {
	for k, v := range controlPlane.Spec.MachineGlobalConfig.Data {
		config[k] = v
	}

	for _, opts := range controlPlane.Spec.MachineSelectorConfig {
		sel, err := metav1.LabelSelectorAsSelector(opts.MachineLabelSelector)
		if err != nil {
			return err
		}
		if opts.MachineLabelSelector == nil || sel.Matches(labels.Set(machine.Labels)) {
			for k, v := range opts.Config.Data {
				config[k] = v
			}
		}
	}

	filterConfigData(config, controlPlane, machine)
	return nil
}

func addRoleConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine, initNode bool, joinServer string) {
	if initNode {
		if rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion) == rancherruntime.RuntimeK3S {
			config["cluster-init"] = true
		}
	} else if joinServer != "" {
		// it's very important that the joinServer param isn't used on the initNode. The init node is special
		// because it will be evaluated twice, first with joinServer = "" and then with joinServer == self.
		// If we use the joinServer param then we will get different nodePlan and cause issues.
		config["server"] = joinServer
	}

	if IsOnlyEtcd(machine) {
		config["disable-scheduler"] = true
		config["disable-apiserver"] = true
		config["disable-controller-manager"] = true
	} else if isOnlyControlPlane(machine) {
		config["disable-etcd"] = true
	}

	if nodeName := machine.Labels[NodeNameLabel]; nodeName != "" {
		config["node-name"] = nodeName
	}
}

func addLocalClusterAuthenticationEndpointConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane,
	machine *capi.Machine) {
	if isOnlyWorker(machine) || !controlPlane.Spec.LocalClusterAuthEndpoint.Enabled {
		return
	}

	authFile := fmt.Sprintf(authnWebhookFileName, rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion))
	config["kube-apiserver-arg"] = append(convert.ToStringSlice(config["kube-apiserver-arg"]),
		fmt.Sprintf("authentication-token-webhook-config-file=%s", authFile))
}

func addLocalClusterAuthenticationEndpointFile(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane,
	machine *capi.Machine) plan.NodePlan {
	if isOnlyWorker(machine) || !controlPlane.Spec.LocalClusterAuthEndpoint.Enabled {
		return nodePlan
	}

	authFile := fmt.Sprintf(authnWebhookFileName, rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion))
	nodePlan.Files = append(nodePlan.Files, plan.File{
		Content: base64.StdEncoding.EncodeToString(AuthnWebhook),
		Path:    authFile,
	})

	return nodePlan
}

func (p *Planner) addManifests(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (plan.NodePlan, error) {
	files, err := p.getControlPlaneManifests(controlPlane, machine)
	if err != nil {
		return nodePlan, err
	}
	nodePlan.Files = append(nodePlan.Files, files...)

	return nodePlan, nil
}

func isVSphereProvider(controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (bool, error) {
	data := map[string]interface{}{}
	if err := addUserConfig(data, controlPlane, machine); err != nil {
		return false, err
	}
	return data["cloud-provider-name"] == "rancher-vsphere", nil
}

func addVSphereCharts(controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (map[string]interface{}, error) {
	if isVSphere, err := isVSphereProvider(controlPlane, machine); err != nil {
		return nil, err
	} else if isVSphere && controlPlane.Spec.ChartValues.Data["rancher-vsphere-csi"] == nil {
		// ensure we have this chart config so that the global.cattle.clusterId is set
		newData := controlPlane.Spec.ChartValues.DeepCopy()
		if newData.Data == nil {
			newData.Data = map[string]interface{}{}
		}
		newData.Data["rancher-vsphere-csi"] = map[string]interface{}{}
		return newData.Data, nil
	}

	return controlPlane.Spec.ChartValues.Data, nil
}

func (p *Planner) addChartConfigs(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane,
	machine *capi.Machine) (plan.NodePlan, error) {
	if isOnlyWorker(machine) {
		return nodePlan, nil
	}

	chartValues, err := addVSphereCharts(controlPlane, machine)
	if err != nil {
		return nodePlan, err
	}

	var chartConfigs []runtime.Object
	for chart, values := range chartValues {
		valuesMap := convert.ToMapInterface(values)
		if valuesMap == nil {
			valuesMap = map[string]interface{}{}
		}
		data.PutValue(valuesMap, controlPlane.Spec.ManagementClusterName, "global", "cattle", "clusterId")

		data, err := json.Marshal(valuesMap)
		if err != nil {
			return plan.NodePlan{}, err
		}

		chartConfigs = append(chartConfigs, &helmChartConfig{
			TypeMeta: metav1.TypeMeta{
				Kind:       "HelmChartConfig",
				APIVersion: "helm.cattle.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      chart,
				Namespace: "kube-system",
			},
			Spec: helmChartConfigSpec{
				ValuesContent: string(data),
			},
		})
	}
	contents, err := yaml.ToBytes(chartConfigs)
	if err != nil {
		return plan.NodePlan{}, err
	}

	nodePlan.Files = append(nodePlan.Files, plan.File{
		Content: base64.StdEncoding.EncodeToString(contents),
		Path:    fmt.Sprintf("/var/lib/rancher/%s/server/manifests/rancher/managed-chart-config.yaml", rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion)),
		Dynamic: true,
	})

	return nodePlan, nil
}

func (p *Planner) addOtherFiles(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane,
	machine *capi.Machine) (plan.NodePlan, error) {
	nodePlan = addLocalClusterAuthenticationEndpointFile(nodePlan, controlPlane, machine)
	return nodePlan, nil
}

func restartStamp(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, image string) string {
	restartStamp := sha256.New()
	restartStamp.Write([]byte(image))
	for _, file := range nodePlan.Files {
		if file.Dynamic {
			continue
		}
		restartStamp.Write([]byte(file.Path))
		restartStamp.Write([]byte(file.Content))
	}
	restartStamp.Write([]byte(strconv.FormatInt(controlPlane.Status.ConfigGeneration, 10)))
	return hex.EncodeToString(restartStamp.Sum(nil))
}

func (p *Planner) addInstruction(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (plan.NodePlan, error) {
	image := getInstallerImage(controlPlane)

	instruction := plan.Instruction{
		Image:   image,
		Command: "sh",
		Args:    []string{"-c", "run.sh"},
		Env: []string{
			fmt.Sprintf("RESTART_STAMP=%s", restartStamp(nodePlan, controlPlane, image)),
		},
	}

	if isOnlyWorker(machine) {
		instruction.Env = append(instruction.Env, fmt.Sprintf("INSTALL_%s_EXEC=agent", rancherruntime.GetRuntimeEnv(controlPlane.Spec.KubernetesVersion)))
	}
	nodePlan.Instructions = append(nodePlan.Instructions, instruction)
	return nodePlan, nil
}

func (p *Planner) addInitNodeInstruction(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (plan.NodePlan, error) {
	nodePlan.Instructions = append(nodePlan.Instructions, plan.Instruction{
		Name:       "capture-address",
		Command:    "sh",
		SaveOutput: true,
		Args: []string{
			"-c",
			// the grep here is to make the command fail if we don't get the output we expect, like empty string.
			fmt.Sprintf("curl -f --retry 100 --retry-delay 5 --cacert "+
				"/var/lib/rancher/%s/server/tls/server-ca.crt https://localhost:%d/db/info | grep 'clientURLs'",
				rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion),
				rancherruntime.GetRuntimeSupervisorPort(controlPlane.Spec.KubernetesVersion)),
		},
	})
	return nodePlan, nil
}

func addToken(config map[string]interface{}, machine *capi.Machine, secret plan.Secret) {
	if secret.ServerToken == "" {
		return
	}
	if isOnlyWorker(machine) {
		config["token"] = secret.AgentToken
	} else {
		config["token"] = secret.ServerToken
		config["agent-token"] = secret.AgentToken
	}
}

func PruneEmpty(config map[string]interface{}) {
	for k, v := range config {
		if v == nil {
			delete(config, k)
		}
		switch t := v.(type) {
		case string:
			if t == "" {
				delete(config, k)
			}
		case []interface{}:
			if len(t) == 0 {
				delete(config, k)
			}
		case []string:
			if len(t) == 0 {
				delete(config, k)
			}
		}
	}
}

func addAddresses(secrets corecontrollers.SecretCache, config map[string]interface{}, machine *capi.Machine) {
	internalIPAddress := machine.Annotations[InternalAddressAnnotation]
	ipAddress := machine.Annotations[AddressAnnotation]
	internalAddressProvided, addressProvided := internalIPAddress != "", ipAddress != ""

	secret, err := secrets.Get(machine.Spec.InfrastructureRef.Namespace, name.SafeConcatName(machine.Spec.InfrastructureRef.Name, "machine", "state"))
	if err == nil && len(secret.Data["extractedConfig"]) != 0 {
		driverConfig, err := nodeconfig.ExtractConfigJSON(base64.StdEncoding.EncodeToString(secret.Data["extractedConfig"]))
		if err == nil && len(driverConfig) != 0 {
			if !addressProvided {
				ipAddress = convert.ToString(values.GetValueN(driverConfig, "Driver", "IPAddress"))
			}
			if !internalAddressProvided {
				internalIPAddress = convert.ToString(values.GetValueN(driverConfig, "Driver", "PrivateIPAddress"))
			}
		}
	}

	setNodeExternalIP := ipAddress != "" && internalIPAddress != "" && ipAddress != internalIPAddress

	if setNodeExternalIP && !isOnlyWorker(machine) {
		config["advertise-address"] = internalIPAddress
		config["tls-san"] = append(convert.ToStringSlice(config["tls-san"]), ipAddress)
	}

	if internalIPAddress != "" {
		config["node-ip"] = append(convert.ToStringSlice(config["node-ip"]), internalIPAddress)
	}

	// Cloud provider, if set, will handle external IP
	if convert.ToString(config["cloud-provider-name"]) == "" && (addressProvided || setNodeExternalIP) {
		config["node-external-ip"] = append(convert.ToStringSlice(config["node-external-ip"]), ipAddress)
	}
}

func addLabels(config map[string]interface{}, machine *capi.Machine) error {
	var labels []string
	if data := machine.Annotations[LabelsAnnotation]; data != "" {
		labelMap := map[string]string{}
		if err := json.Unmarshal([]byte(data), &labelMap); err != nil {
			return err
		}
		for k, v := range labelMap {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}
	}

	labels = append(labels, MachineUIDLabel+"="+string(machine.UID))
	sort.Strings(labels)
	if len(labels) > 0 {
		config["node-label"] = labels
	}
	return nil
}

func getTaints(machine *capi.Machine, runtime string) (result []corev1.Taint, _ error) {
	data := machine.Annotations[TaintsAnnotation]
	if data != "" {
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return result, err
		}
	}

	if runtime == rancherruntime.RuntimeRKE2 {
		if isEtcd(machine) && !isWorker(machine) {
			result = append(result, corev1.Taint{
				Key:    "node-role.kubernetes.io/etcd",
				Effect: corev1.TaintEffectNoExecute,
			})
		}

		if isControlPlane(machine) && !isWorker(machine) {
			result = append(result, corev1.Taint{
				Key:    "node-role.kubernetes.io/control-plane",
				Effect: corev1.TaintEffectNoSchedule,
			})
		}
	}

	return
}

func addTaints(config map[string]interface{}, machine *capi.Machine, runtime string) error {
	var (
		taintString []string
	)

	taints, err := getTaints(machine, runtime)
	if err != nil {
		return err
	}

	for _, taint := range taints {
		if taint.Key != "" {
			taintString = append(taintString, taint.ToString())
		}
	}

	sort.Strings(taintString)
	config["node-taint"] = taintString

	return nil
}

func configFile(controlPlane *rkev1.RKEControlPlane, filename string) string {
	if path := filePaths[filename]; path != "" {
		if strings.Contains(path, "%s") {
			return fmt.Sprintf(path, rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion))
		}
		return path
	}
	return fmt.Sprintf("/var/lib/rancher/%s/etc/config-files/%s",
		rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion), filename)
}

func (p *Planner) addConfigFile(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine, secret plan.Secret,
	initNode bool, joinServer string) (plan.NodePlan, error) {
	config := map[string]interface{}{}

	addDefaults(config, controlPlane, machine)

	// Must call addUserConfig first because it will filter out non-kdm data
	if err := addUserConfig(config, controlPlane, machine); err != nil {
		return nodePlan, err
	}

	files, err := p.addRegistryConfig(config, controlPlane)
	if err != nil {
		return nodePlan, err
	}
	nodePlan.Files = append(nodePlan.Files, files...)

	files, err = p.addETCD(config, controlPlane, machine)
	if err != nil {
		return nodePlan, err
	}
	nodePlan.Files = append(nodePlan.Files, files...)

	addRoleConfig(config, controlPlane, machine, initNode, joinServer)
	addLocalClusterAuthenticationEndpointConfig(config, controlPlane, machine)
	addToken(config, machine, secret)
	addAddresses(p.secretCache, config, machine)

	if err := addLabels(config, machine); err != nil {
		return nodePlan, err
	}

	runtime := rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion)
	if err := addTaints(config, machine, runtime); err != nil {
		return nodePlan, err
	}

	for _, fileParam := range fileParams {
		content, ok := config[fileParam]
		if !ok {
			continue
		}
		filePath := configFile(controlPlane, fileParam)
		config[fileParam] = filePath

		nodePlan.Files = append(nodePlan.Files, plan.File{
			Content: base64.StdEncoding.EncodeToString([]byte(convert.ToString(content))),
			Path:    filePath,
		})
	}

	PruneEmpty(config)

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nodePlan, err
	}

	nodePlan.Files = append(nodePlan.Files, plan.File{
		Content: base64.StdEncoding.EncodeToString(configData),
		Path:    fmt.Sprintf(ConfigYamlFileName, rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion)),
	})

	return nodePlan, nil
}

func (p *Planner) desiredPlan(controlPlane *rkev1.RKEControlPlane, secret plan.Secret, entry planEntry, initNode bool, joinServer string) (nodePlan plan.NodePlan, err error) {
	if !controlPlane.Spec.UnmanagedConfig {
		nodePlan, err = commonNodePlan(p.secretCache, controlPlane, plan.NodePlan{})
		if err != nil {
			return nodePlan, err
		}

		nodePlan, err = p.addConfigFile(nodePlan, controlPlane, entry.Machine, secret, initNode, joinServer)
		if err != nil {
			return nodePlan, err
		}

		nodePlan, err = p.addManifests(nodePlan, controlPlane, entry.Machine)
		if err != nil {
			return nodePlan, err
		}

		nodePlan, err = p.addChartConfigs(nodePlan, controlPlane, entry.Machine)
		if err != nil {
			return nodePlan, err
		}

		nodePlan, err = p.addOtherFiles(nodePlan, controlPlane, entry.Machine)
		if err != nil {
			return nodePlan, err
		}
	}

	nodePlan, err = p.addProbes(nodePlan, controlPlane, entry.Machine)
	if err != nil {
		return nodePlan, err
	}

	// Add instruction last because it hashes config content
	nodePlan, err = p.addInstruction(nodePlan, controlPlane, entry.Machine)
	if err != nil {
		return nodePlan, err
	}

	if initNode && IsOnlyEtcd(entry.Machine) {
		nodePlan, err = p.addInitNodeInstruction(nodePlan, controlPlane, entry.Machine)
		if err != nil {
			return nodePlan, err
		}
	}

	return nodePlan, nil
}

func getInstallerImage(controlPlane *rkev1.RKEControlPlane) string {
	runtime := rancherruntime.GetRuntime(controlPlane.Spec.KubernetesVersion)
	image := settings.SystemAgentInstallerImage.Get()
	image = image + runtime + ":" + strings.ReplaceAll(controlPlane.Spec.KubernetesVersion, "+", "-")
	return settings.PrefixPrivateRegistry(image)
}

func isEtcd(machine *capi.Machine) bool {
	return machine.Labels[EtcdRoleLabel] == "true"
}

func isInitNode(machine *capi.Machine) bool {
	return machine.Labels[InitNodeLabel] == "true"
}

func isInitNodeOrDeleting(machine *capi.Machine) bool {
	return isInitNode(machine) || isDeleting(machine)
}

func IsEtcdOnlyInitNode(machine *capi.Machine) bool {
	return isInitNode(machine) && IsOnlyEtcd(machine)
}

func isDeleting(machine *capi.Machine) bool {
	return machine.DeletionTimestamp != nil
}

func isControlPlane(machine *capi.Machine) bool {
	return machine.Labels[ControlPlaneRoleLabel] == "true"
}

func isControlPlaneEtcd(machine *capi.Machine) bool {
	return isControlPlane(machine) || isEtcd(machine)
}

func IsOnlyEtcd(machine *capi.Machine) bool {
	return isEtcd(machine) && !isControlPlane(machine)
}

func isOnlyControlPlane(machine *capi.Machine) bool {
	return !isEtcd(machine) && isControlPlane(machine)
}

func isWorker(machine *capi.Machine) bool {
	return machine.Labels[WorkerRoleLabel] == "true"
}

func noRole(machine *capi.Machine) bool {
	return !isEtcd(machine) && !isControlPlane(machine) && !isWorker(machine)
}

func isOnlyWorker(machine *capi.Machine) bool {
	return !isEtcd(machine) && !isControlPlane(machine) && isWorker(machine)
}

type planEntry struct {
	Machine *capi.Machine
	Plan    *plan.Node
}

func collect(plan *plan.Plan, include func(*capi.Machine) bool) (result []planEntry) {
	for name, machine := range plan.Machines {
		if !include(machine) {
			continue
		}
		result = append(result, planEntry{
			Machine: machine,
			Plan:    plan.Nodes[name],
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Machine.Name < result[j].Machine.Name
	})

	return result
}

func (p *Planner) generateSecrets(controlPlane *rkev1.RKEControlPlane) (*rkev1.RKEControlPlane, plan.Secret, error) {
	_, secret, err := p.ensureRKEStateSecret(controlPlane)
	if err != nil {
		return nil, secret, err
	}

	controlPlane = controlPlane.DeepCopy()
	return controlPlane, secret, nil
}

func (p *Planner) ensureRKEStateSecret(controlPlane *rkev1.RKEControlPlane) (string, plan.Secret, error) {
	if controlPlane.Spec.UnmanagedConfig {
		return "", plan.Secret{}, nil
	}

	name := name.SafeConcatName(controlPlane.Name, "rke", "state")
	secret, err := p.secretCache.Get(controlPlane.Namespace, name)
	if apierror.IsNotFound(err) {
		serverToken, err := randomtoken.Generate()
		if err != nil {
			return "", plan.Secret{}, err
		}

		agentToken, err := randomtoken.Generate()
		if err != nil {
			return "", plan.Secret{}, err
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: controlPlane.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "rke.cattle.io/v1",
						Kind:       "RKEControlPlane",
						Name:       controlPlane.Name,
						UID:        controlPlane.UID,
					},
				},
			},
			Data: map[string][]byte{
				"serverToken": []byte(serverToken),
				"agentToken":  []byte(agentToken),
			},
			Type: "rke.cattle.io/cluster-state",
		}

		_, err = p.secretClient.Create(secret)
		return name, plan.Secret{
			ServerToken: serverToken,
			AgentToken:  agentToken,
		}, err
	} else if err != nil {
		return "", plan.Secret{}, err
	}

	return secret.Name, plan.Secret{
		ServerToken: string(secret.Data["serverToken"]),
		AgentToken:  string(secret.Data["agentToken"]),
	}, nil
}

type helmChartConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec helmChartConfigSpec `json:"spec,omitempty"`
}

type helmChartConfigSpec struct {
	ValuesContent string `json:"valuesContent,omitempty"`
}

func (h *helmChartConfig) DeepCopyObject() runtime.Object {
	panic("unsupported")
}
