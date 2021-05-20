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
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/data/convert"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
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

	InitNodeLabel         = "rke.cattle.io/init-node"
	EtcdRoleLabel         = "rke.cattle.io/etcd-role"
	WorkerRoleLabel       = "rke.cattle.io/worker-role"
	ControlPlaneRoleLabel = "rke.cattle.io/control-plane-role"
	MachineUIDLabel       = "rke.cattle.io/machine"
	capiMachineLabel      = "cluster.x-k8s.io/cluster-name"

	MachineNameLabel      = "rke.cattle.io/machine-name"
	MachineNamespaceLabel = "rke.cattle.io/machine-namespace"

	LabelsAnnotation = "rke.cattle.io/labels"
	TaintsAnnotation = "rke.cattle.io/taints"

	SecretTypeMachinePlan = "rke.cattle.io/machine-plan"

	authnWebhookFileName = "/var/lib/rancher/%s/kube-api-authn-webhook.yaml"
)

var (
	fileParams = []string{
		"audit-policy-file",
		"cloud-provider-config",
		"private-registry",
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
		kubeconfig:                    kubeconfig.New(clients),
		etcdRestore:                   newETCDRestore(clients, store),
		etcdCreate:                    newETCDCreate(clients, store),
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

	controlPlane, secret, err := p.generateSecrets(controlPlane, plan)
	if err != nil {
		return err
	}

	var (
		firstIgnoreError error
		joinServer       string
	)

	if secret.ServerToken == "" {
		// This is logic for clusters that are formed outside of Rancher
		// In this situation you either have nodes with worker role or no role
		err = p.reconcile(controlPlane, secret, plan, "etcd and control plane", true, noRole, isInitNode,
			controlPlane.Spec.UpgradeStrategy.ControlPlaneConcurrency, joinServer,
			controlPlane.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
		firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
		if err != nil {
			return err
		}
	} else {
		if err := p.etcdCreate.Create(controlPlane, plan); err != nil {
			return err
		}

		if err := p.etcdRestore.Restore(controlPlane, plan); err != nil {
			return err
		}

		if _, err := p.electInitNode(plan); err != nil {
			return err
		}

		err = p.reconcile(controlPlane, secret, plan, "bootstrap", true, isInitNode, none,
			controlPlane.Spec.UpgradeStrategy.ControlPlaneConcurrency, "",
			controlPlane.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
		firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
		if err != nil {
			return err
		}

		joinServer, err = p.electInitNode(plan)
		if err != nil || joinServer == "" {
			return err
		}

		err = p.reconcile(controlPlane, secret, plan, "etcd", true, isEtcd, isInitNode,
			controlPlane.Spec.UpgradeStrategy.ControlPlaneConcurrency, joinServer,
			controlPlane.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
		firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
		if err != nil {
			return err
		}

		err = p.reconcile(controlPlane, secret, plan, "control plane", true, isControlPlane, isInitNode,
			controlPlane.Spec.UpgradeStrategy.ControlPlaneConcurrency, joinServer,
			controlPlane.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
		firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
		if err != nil {
			return err
		}
	}

	joinServer = p.getControlPlaneJoinURL(plan)
	if joinServer == "" {
		return ErrWaiting("waiting for control plane to be available")
	}

	err = p.reconcile(controlPlane, secret, plan, "worker", false, isOnlyWorker, isInitNode,
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
	return err
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
	return p.machines.Update(machine)
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

func (p *Planner) electInitNode(plan *plan.Plan) (string, error) {
	entries := collect(plan, isEtcd)
	joinURL := ""
	initNodeFound := false
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

		plan, err := p.desiredPlan(controlPlane, secret, entry, isInitNode(entry.Machine), joinServer)
		if err != nil {
			return err
		}

		if entry.Plan == nil {
			outOfSync = append(outOfSync, entry.Machine.Name)
			if err := p.store.UpdatePlan(entry.Machine, plan); err != nil {
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
					if err := p.store.UpdatePlan(entry.Machine, plan); err != nil {
						return err
					}
				} else {
					draining = append(draining, entry.Machine.Name)
				}
			}
		} else if !entry.Plan.InSync {
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
		return errIgnore("failing " + tierName + " machine(s) " + strings.Join(errMachines, ","))
	}

	outOfSync = atMostThree(outOfSync)
	if len(outOfSync) > 0 {
		return ErrWaiting("provisioning " + tierName + " node(s) " + strings.Join(outOfSync, ","))
	}

	draining = atMostThree(draining)
	if len(draining) > 0 {
		return ErrWaiting("draining " + tierName + " node(s) " + strings.Join(outOfSync, ","))
	}

	uncordoned = atMostThree(uncordoned)
	if len(uncordoned) > 0 {
		return ErrWaiting("uncordoning " + tierName + " node(s) " + strings.Join(outOfSync, ","))
	}

	nonReady = atMostThree(nonReady)
	if len(nonReady) > 0 {
		// we want these errors to get reported, but not block the process
		return errIgnore("non-ready " + tierName + " machine(s) " + strings.Join(nonReady, ","))
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

func (p *Planner) addETCDSnapshotCredential(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) error {
	if !isEtcd(machine) || controlPlane.Spec.ETCDSnapshotCloudCredentialName == "" {
		return nil
	}

	cred, err := getS3Credential(p.secretCache,
		controlPlane.Namespace,
		controlPlane.Spec.ETCDSnapshotCloudCredentialName,
		convert.ToString(config["etcd-s3-region"]))
	if err != nil {
		return err
	}

	config["etcd-s3-access-key"] = cred.AccessKey
	config["etcd-s3-secret-key"] = cred.SecretKey
	config["etcd-s3-region"] = cred.Region
	return nil
}

func addDefaults(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) {
	if GetRuntime(controlPlane.Spec.KubernetesVersion) == RuntimeRKE2 {
		config["cni"] = "calico"
	}
}

func addUserConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) error {
	for _, opts := range controlPlane.Spec.NodeConfig {
		sel, err := metav1.LabelSelectorAsSelector(opts.MachineLabelSelector)
		if err != nil {
			return err
		}
		if sel.Matches(labels.Set(machine.Labels)) {
			for k, v := range opts.Config.Data {
				config[k] = v
			}
			break
		}
	}

	for k, v := range controlPlane.Spec.ControlPlaneConfig.Data {
		config[k] = v
	}

	filterConfigData(config, controlPlane, machine)
	return nil
}

func addRoleConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine, initNode bool, joinServer string) {
	if initNode {
		if GetRuntime(controlPlane.Spec.KubernetesVersion) == RuntimeK3S {
			config["cluster-init"] = true
		}
	} else if joinServer != "" {
		// it's very important that the joinServer param isn't used on the initNode. The init node is special
		// because it will be evaluated twice, first with joinServer = "" and then with joinServer == self.
		// If we use the joinServer param then we will get different nodePlan and cause issues.
		config["server"] = joinServer
	}

	if isOnlyEtcd(machine) {
		config["disable-scheduler"] = true
		config["disable-apiserver"] = true
		config["disable-controller-manager"] = true
	} else if isOnlyControlPlane(machine) {
		config["disable-etcd"] = true
	}
}

func addLocalClusterAuthenticationEndpointConfig(config map[string]interface{}, controlPlane *rkev1.RKEControlPlane,
	machine *capi.Machine) {
	if isOnlyWorker(machine) || !controlPlane.Spec.LocalClusterAuthEndpoint.Enabled {
		return
	}

	authFile := fmt.Sprintf(authnWebhookFileName, GetRuntime(controlPlane.Spec.KubernetesVersion))
	config["kube-apiserver-arg"] = append(convert.ToStringSlice(config["kube-apiserver-arg"]),
		fmt.Sprintf("authentication-token-webhook-config-file=%s", authFile))
}

func addLocalClusterAuthenticationEndpointFile(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane,
	machine *capi.Machine) plan.NodePlan {
	if isOnlyWorker(machine) || !controlPlane.Spec.LocalClusterAuthEndpoint.Enabled {
		return nodePlan
	}

	authFile := fmt.Sprintf(authnWebhookFileName, GetRuntime(controlPlane.Spec.KubernetesVersion))
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

func (p *Planner) addChartConfigs(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane,
	machine *capi.Machine) (plan.NodePlan, error) {
	if isOnlyWorker(machine) {
		return nodePlan, nil
	}

	var chartConfigs []runtime.Object
	for chart, values := range controlPlane.Spec.ChartValues.Data {
		data, err := json.Marshal(values)
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
		Path:    fmt.Sprintf("/var/lib/rancher/%s/server/manifests/rancher/managed-chart-config.yaml", GetRuntime(controlPlane.Spec.KubernetesVersion)),
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
		if GetRuntime(controlPlane.Spec.KubernetesVersion) == RuntimeRKE2 {
			instruction.Env = append(instruction.Env, fmt.Sprintf("INSTALL_%s_TYPE=agent", GetRuntimeEnv(controlPlane.Spec.KubernetesVersion)))
		} else {
			instruction.Env = append(instruction.Env, fmt.Sprintf("INSTALL_%s_EXEC=agent", GetRuntimeEnv(controlPlane.Spec.KubernetesVersion)))
		}
	}
	nodePlan.Instructions = append(nodePlan.Instructions, instruction)
	return nodePlan, nil
}

func (p *Planner) addInitNodeInstruction(nodePlan plan.NodePlan, controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) (plan.NodePlan, error) {
	nodePlan.Instructions = append(nodePlan.Instructions, plan.Instruction{
		Name:       "capture-address",
		Image:      getInstallerImage(controlPlane),
		Command:    "curl",
		SaveOutput: true,
		Args: []string{
			"-f",
			"--retry", "20",
			"--retry-delay", "5",
			"--cacert", fmt.Sprintf("/var/lib/rancher/%s/server/tls/server-ca.crt",
				GetRuntime(controlPlane.Spec.KubernetesVersion)),
			fmt.Sprintf("https://localhost:%d/db/info", GetRuntimeSupervisorPort(controlPlane.Spec.KubernetesVersion)),
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

func addTaints(config map[string]interface{}, machine *capi.Machine) error {
	var (
		taints      []corev1.Taint
		taintString []string
	)

	data := machine.Annotations[TaintsAnnotation]
	if data == "" {
		return nil
	}

	if err := json.Unmarshal([]byte(data), &taints); err != nil {
		return err
	}

	for _, taint := range taints {
		taintString = append(taintString, fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect))
	}

	sort.Strings(taintString)
	config["node-taint"] = taintString

	return nil
}

func configFile(controlPlane *rkev1.RKEControlPlane, filename string) string {
	return fmt.Sprintf("/var/lib/rancher/%s/etc/config-files/%s",
		GetRuntime(controlPlane.Spec.KubernetesVersion), filename)
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

	if err := p.addETCDSnapshotCredential(config, controlPlane, machine); err != nil {
		return nodePlan, err
	}

	addRoleConfig(config, controlPlane, machine, initNode, joinServer)
	addLocalClusterAuthenticationEndpointConfig(config, controlPlane, machine)
	addToken(config, machine, secret)

	if err := addLabels(config, machine); err != nil {
		return nodePlan, err
	}

	if err := addTaints(config, machine); err != nil {
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

	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nodePlan, err
	}

	nodePlan.Files = append(nodePlan.Files, plan.File{
		Content: base64.StdEncoding.EncodeToString(configData),
		Path:    fmt.Sprintf("/etc/rancher/%s/config.yaml.d/50-rancher.yaml", GetRuntime(controlPlane.Spec.KubernetesVersion)),
	})

	return nodePlan, nil
}

func (p *Planner) desiredPlan(controlPlane *rkev1.RKEControlPlane, secret plan.Secret, entry planEntry, initNode bool, joinServer string) (plan.NodePlan, error) {
	nodePlan, err := commonNodePlan(p.secretCache, controlPlane, plan.NodePlan{})
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

	// Add instruction last because it hashes config content
	nodePlan, err = p.addInstruction(nodePlan, controlPlane, entry.Machine)
	if err != nil {
		return nodePlan, err
	}

	if initNode && isOnlyEtcd(entry.Machine) {
		nodePlan, err = p.addInitNodeInstruction(nodePlan, controlPlane, entry.Machine)
		if err != nil {
			return nodePlan, err
		}
	}

	return nodePlan, nil
}

func getInstallerImage(controlPlane *rkev1.RKEControlPlane) string {
	runtime := GetRuntime(controlPlane.Spec.KubernetesVersion)
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

func IsEtcdOnlyInitNode(machine *capi.Machine) bool {
	return isInitNode(machine) && isOnlyEtcd(machine)
}

func none(_ *capi.Machine) bool {
	return false
}

func isControlPlane(machine *capi.Machine) bool {
	return machine.Labels[ControlPlaneRoleLabel] == "true"
}

func isOnlyEtcd(machine *capi.Machine) bool {
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

func (p *Planner) generateSecrets(controlPlane *rkev1.RKEControlPlane, fullPlan *plan.Plan) (*rkev1.RKEControlPlane, plan.Secret, error) {
	_, secret, err := p.ensureRKEStateSecret(controlPlane, fullPlan)
	if err != nil {
		return nil, secret, err
	}

	controlPlane = controlPlane.DeepCopy()
	return controlPlane, secret, nil
}

func (p *Planner) ensureRKEStateSecret(controlPlane *rkev1.RKEControlPlane, fullPlan *plan.Plan) (string, plan.Secret, error) {
	hasControlPlaneOrEtcd := false
	for _, machine := range fullPlan.Machines {
		if isControlPlane(machine) || isEtcd(machine) {
			hasControlPlaneOrEtcd = true
			break
		}
	}

	if !hasControlPlaneOrEtcd {
		// In this situation we are either waiting for control plane/etcd to be created or
		// this is an externally formed cluster and we don't manage the token
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
