package planner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"math"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/moby/locker"
	"github.com/rancher/channelserver/pkg/model"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	ranchercontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	"github.com/rancher/wrangler/v3/pkg/summary"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
)

const (
	ClusterRegToken = "clusterRegToken"

	EtcdSnapshotConfigMapKey = "provisioning-cluster-spec"

	KubeControllerManagerArg                      = "kube-controller-manager-arg"
	KubeControllerManagerExtraMount               = "kube-controller-manager-extra-mount"
	DefaultKubeControllerManagerCertDir           = "server/tls/kube-controller-manager"
	DefaultKubeControllerManagerDefaultSecurePort = "10257"
	DefaultKubeControllerManagerCert              = "kube-controller-manager.crt"
	KubeSchedulerArg                              = "kube-scheduler-arg"
	KubeSchedulerExtraMount                       = "kube-scheduler-extra-mount"
	DefaultKubeSchedulerCertDir                   = "server/tls/kube-scheduler"
	DefaultKubeSchedulerDefaultSecurePort         = "10259"
	DefaultKubeSchedulerCert                      = "kube-scheduler.crt"
	SecurePortArgument                            = "secure-port"
	CertDirArgument                               = "cert-dir"
	TLSCertFileArgument                           = "tls-cert-file"

	authnWebhookFileName = "kube-api-authn-webhook.yaml"
	ConfigYamlFileName   = "/etc/rancher/%s/config.yaml.d/50-rancher.yaml"

	bootstrapTier    = "bootstrap"
	etcdTier         = "etcd"
	controlPlaneTier = "control plane"
	workerTier       = "worker"

	auditPolicyArg         = "audit-policy-file"
	cloudProviderConfigArg = "cloud-provider-config"
	privateRegistryArg     = "private-registry"
	flannelConfArg         = "flannel-conf"

	AuthnWebhook = `
apiVersion: v1
kind: Config
clusters:
- name: Default
  cluster:
    insecure-skip-tls-verify: true
    server: http://%s:6440/v1/authenticate
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
`
)

var (
	fileParams = []string{
		auditPolicyArg,
		cloudProviderConfigArg,
		privateRegistryArg,
		flannelConfArg,
	}
	filePaths = map[string]string{
		privateRegistryArg: "/etc/rancher/%s/registries.yaml",
	}
)

type Planner struct {
	ctx                           context.Context
	store                         *PlanStore
	rkeBootstrap                  rkecontrollers.RKEBootstrapClient
	rkeBootstrapCache             rkecontrollers.RKEBootstrapCache
	rkeControlPlanes              rkecontrollers.RKEControlPlaneController
	etcdSnapshotCache             rkecontrollers.ETCDSnapshotCache
	secretClient                  corecontrollers.SecretClient
	secretCache                   corecontrollers.SecretCache
	configMapCache                corecontrollers.ConfigMapCache
	machines                      capicontrollers.MachineClient
	machinesCache                 capicontrollers.MachineCache
	clusterRegistrationTokenCache mgmtcontrollers.ClusterRegistrationTokenCache
	capiClient                    capicontrollers.ClusterClient
	capiClusters                  capicontrollers.ClusterCache
	managementClusters            mgmtcontrollers.ClusterCache
	rancherClusterCache           ranchercontrollers.ClusterCache
	locker                        locker.Locker
	etcdS3Args                    s3Args
	retrievalFunctions            InfoFunctions
}

// InfoFunctions is a struct that contains various dynamic functions that allow for abstracting out Rancher-specific
// logic from the Planner
type InfoFunctions struct {
	ImageResolver           func(image string, cp *rkev1.RKEControlPlane) string
	ReleaseData             func(context.Context, *rkev1.RKEControlPlane) *model.Release
	SystemAgentImage        func() string
	SystemPodLabelSelectors func(plane *rkev1.RKEControlPlane) []string
	GetBootstrapManifests   func(plane *rkev1.RKEControlPlane) ([]plan.File, error)
}

func New(ctx context.Context, clients *wrangler.Context, functions InfoFunctions) *Planner {
	clients.Mgmt.ClusterRegistrationToken().Cache().AddIndexer(ClusterRegToken, func(obj *v3.ClusterRegistrationToken) ([]string, error) {
		return []string{obj.Spec.ClusterName}, nil
	})
	store := NewStore(clients.Core.Secret(),
		clients.CAPI.Machine().Cache())
	return &Planner{
		ctx:                           ctx,
		store:                         store,
		machines:                      clients.CAPI.Machine(),
		machinesCache:                 clients.CAPI.Machine().Cache(),
		secretClient:                  clients.Core.Secret(),
		secretCache:                   clients.Core.Secret().Cache(),
		configMapCache:                clients.Core.ConfigMap().Cache(),
		clusterRegistrationTokenCache: clients.Mgmt.ClusterRegistrationToken().Cache(),
		capiClient:                    clients.CAPI.Cluster(),
		capiClusters:                  clients.CAPI.Cluster().Cache(),
		managementClusters:            clients.Mgmt.Cluster().Cache(),
		rancherClusterCache:           clients.Provisioning.Cluster().Cache(),
		rkeControlPlanes:              clients.RKE.RKEControlPlane(),
		rkeBootstrap:                  clients.RKE.RKEBootstrap(),
		rkeBootstrapCache:             clients.RKE.RKEBootstrap().Cache(),
		etcdSnapshotCache:             clients.RKE.ETCDSnapshot().Cache(),
		etcdS3Args: s3Args{
			secretCache: clients.Core.Secret().Cache(),
		},
		retrievalFunctions: functions,
	}
}

func (p *Planner) setMachineConditionStatus(clusterPlan *plan.Plan, machineNames []string, messagePrefix string, messages map[string][]string) error {
	var waiting bool
	for _, machineName := range machineNames {
		machine := clusterPlan.Machines[machineName]
		if machine == nil {
			return fmt.Errorf("found unexpected machine %s that is not in cluster plan", machineName)
		}

		if !capr.InfrastructureReady.IsTrue(machine) {
			waiting = true
			continue
		}

		machine = machine.DeepCopy()
		if message := messages[machineName]; len(message) > 0 {
			msg := strings.Join(message, ", ")
			waiting = true
			if capr.Reconciled.GetMessage(machine) == msg {
				continue
			}
			conditions.MarkUnknown(machine, capi.ConditionType(capr.Reconciled), "Waiting", msg)
		} else if !capr.Reconciled.IsTrue(machine) {
			// Since there is no status message, then the condition should be set to true.
			conditions.MarkTrue(machine, capi.ConditionType(capr.Reconciled))

			// Even though we are technically not waiting for something, an error should be returned so that the planner will retry.
			// The machine being updated will cause the planner to re-enqueue with the new data.
			waiting = true
		} else {
			continue
		}

		if _, err := p.machines.UpdateStatus(machine); err != nil {
			return err
		}
	}

	if waiting {
		return errWaiting(messagePrefix + atMostThree(machineNames) + detailedMessage(machineNames, messages))
	}
	return nil
}

func (p *Planner) Process(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	logrus.Debugf("[planner] rkecluster %s/%s: attempting to lock %s for processing", cp.Namespace, cp.Name, string(cp.UID))
	p.locker.Lock(string(cp.UID))
	defer func(namespace, name, uid string) {
		logrus.Debugf("[planner] rkecluster %s/%s: unlocking %s", namespace, name, uid)
		_ = p.locker.Unlock(uid)
	}(cp.Namespace, cp.Name, string(cp.UID))

	currentVersion, err := semver.NewVersion(cp.Spec.KubernetesVersion)
	if err != nil {
		return status, fmt.Errorf("rkecluster %s/%s: error semver parsing kubernetes version %s: %v", cp.Namespace, cp.Name, cp.Spec.KubernetesVersion, err)
	}

	releaseData := p.retrievalFunctions.ReleaseData(p.ctx, cp)
	if releaseData == nil {
		return status, errWaitingf("%s/%s: KDM release data is empty for %s", cp.Namespace, cp.Name, cp.Spec.KubernetesVersion)
	}

	capiCluster, err := capr.GetOwnerCAPICluster(cp, p.capiClusters)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return status, errWaiting("CAPI cluster does not exist")
		}
		return status, err
	}

	if capiCluster == nil {
		return status, errWaiting("CAPI cluster does not exist")
	}

	if !capiCluster.DeletionTimestamp.IsZero() {
		// because we pause reconciliation during encryption key rotation and cert rotation, unpause it. This is effectively
		// a hack since the planner pauses the entire cluster during
		if capiannotations.IsPaused(capiCluster, cp) {
			err = p.pauseCAPICluster(cp, false)
			if err != nil {
				logrus.Errorf("error unpausing CAPI cluster during deletion: %s", err)
			}
		}
		logrus.Infof("[planner] %s/%s: reconciliation stopped: CAPI cluster is deleting", cp.Namespace, cp.Name)
		return status, nil
	}

	if !capiCluster.Status.InfrastructureReady {
		return status, errWaiting("waiting for infrastructure ready")
	}

	plan, anyPlansDelivered, err := p.store.Load(capiCluster, cp)
	if err != nil {
		return status, err
	}

	// Check for cluster sanity to ensure we can properly deliver plans to this cluster.
	if !clusterIsSane(plan) {
		// Set the Stable condition on the controlplane to False. This will be used to indicate that the Ready condition
		// on the v1 cluster object should be set from the rkecontrolplane Provisioned condition rather than the v3
		// cluster objects Ready condition.
		capr.Stable.False(&status)

		// Set the `initialized` and `ready` status fields on the status to false, as the cluster is not sane and cannot
		// be considered initialized. This is to also prevent CAPI from setting the ControlPlaneInitialized condition
		// using fallback logic from the status field.
		if status.Initialized || status.Ready {
			status.Initialized = false
			status.Ready = false
			logrus.Debugf("[planner] rkecluster %s/%s: setting controlplane ready/initialized to false as cluster was not sane", cp.Namespace, cp.Name)
			return status, errWaitingf("uninitializing rkecontrolplane %s/%s", cp.Namespace, cp.Name)
		}

		// Uninitialize the CAPI ClusterControlPlaneInitialized condition so that CAPI controllers don't get hung up and will take ownership of new RKEBootstraps (amongst other objects)
		if err := p.ensureCAPIClusterControlPlaneInitializedFalse(cp); err != nil {
			return status, errWaitingf("uninitializing CAPI cluster: %v", err)
		}

		// Collect all nodes that are etcd and deleting. At this point, if we have any etcd nodes left in the cluster,
		// they will be deleting, so force delete them to prevent quorum loss.
		etcdDeleting, err := p.forceDeleteAllDeletingEtcdMachines(cp, plan)
		if err != nil {
			return status, err
		} else if etcdDeleting != 0 {
			return status, errWaiting("waiting for all etcd machines to be deleted")
		}
		return status, errWaiting("waiting for at least one control plane, etcd, and worker node to be registered")
	}

	capr.Provisioned.True(&status)
	capr.Provisioned.Message(&status, "")
	capr.Provisioned.Reason(&status, "")

	_, clusterSecretTokens, err := p.ensureRKEStateSecret(cp, !anyPlansDelivered)
	if err != nil {
		return status, err
	}

	if status, err = p.createEtcdSnapshot(cp, status, clusterSecretTokens, plan); err != nil {
		return status, err
	}

	if status, err = p.restoreEtcdSnapshot(cp, status, clusterSecretTokens, plan, currentVersion); err != nil {
		return status, err
	}

	if status, err = p.rotateCertificates(cp, status, clusterSecretTokens, plan); err != nil {
		return status, err
	}

	if status, err = p.rotateEncryptionKeys(cp, status, clusterSecretTokens, plan, releaseData); err != nil {
		return status, err
	}

	// pausing the control plane only affects machine reconciliation: etcd snapshot/restore, encryption key & cert
	// rotation are not interruptable processes, and therefore must always be completed when requested
	if capiannotations.IsPaused(capiCluster, cp) {
		return status, errWaitingf("CAPI cluster or RKEControlPlane is paused")
	}

	// In the case where the cluster has been bootstrapped and no plans have been
	// delivered to any etcd nodes, don't proceed with electing a new init node.
	// The only way out of this is to restore an etcd snapshot.
	if (capr.Bootstrapped.IsTrue(&status) || len(collect(plan, roleOr(hasJoinURL, hasJoinedTo))) != 0) && len(collect(plan, roleAnd(isEtcd, anyPlanDataExists))) == 0 {
		// deliver an etcd snapshot list command to the etcd nodes.
		capr.Stable.False(&status) // Set the Stable condition on the controlplane to False. This will be used to hide the v3.Cluster Ready condition from the UI.
		return status, errWaiting("rkecontrolplane was already initialized but no etcd machines exist that have plans, indicating the etcd plane has been entirely replaced. Restoration from etcd snapshot is required.")
	}

	return p.fullReconcile(cp, status, clusterSecretTokens, plan, false)
}

func (p *Planner) fullReconcile(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, clusterSecretTokens plan.Secret, plan *plan.Plan, ignoreDrainAndConcurrency bool) (rkev1.RKEControlPlaneStatus, error) {
	// on the first run through, electInitNode will return a `generic.ErrSkip` as it is attempting to wait for the cache to catch up.
	joinServer, err := p.electInitNode(cp, plan, true)
	if err != nil {
		return status, err
	}

	var (
		firstIgnoreError                             error
		controlPlaneDrainOptions, workerDrainOptions rkev1.DrainOptions
		controlPlaneConcurrency, workerConcurrency   string
	)

	if !ignoreDrainAndConcurrency {
		controlPlaneDrainOptions = cp.Spec.UpgradeStrategy.ControlPlaneDrainOptions
		workerDrainOptions = cp.Spec.UpgradeStrategy.WorkerDrainOptions
		controlPlaneConcurrency = cp.Spec.UpgradeStrategy.ControlPlaneConcurrency
		workerConcurrency = cp.Spec.UpgradeStrategy.WorkerConcurrency
	}

	// select all etcd and then filter to just initNodes so that unavailable count is correct
	err = p.reconcile(cp, clusterSecretTokens, plan, true, bootstrapTier, isEtcd, isNotInitNodeOrIsDeleting,
		"1", "",
		controlPlaneDrainOptions)
	capr.Bootstrapped.True(&status)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return status, err
	}

	if joinServer == "" {
		_, joinServer, _, err = p.findInitNode(cp, plan)
		if err != nil {
			return status, err
		} else if joinServer == "" && firstIgnoreError != nil {
			return status, errWaiting(firstIgnoreError.Error() + " and join url to be available on bootstrap node")
		} else if joinServer == "" {
			return status, errWaiting("waiting for join url to be available on bootstrap node")
		}
	}

	// Process all nodes that have the etcd role and are NOT an init node or deleting. Only process 1 node at a time.
	err = p.reconcile(cp, clusterSecretTokens, plan, true, etcdTier, isEtcd, isInitNodeOrDeleting,
		"1", joinServer,
		controlPlaneDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return status, err
	}

	// Process all nodes that have the controlplane role and are NOT an init node or deleting.
	err = p.reconcile(cp, clusterSecretTokens, plan, true, controlPlaneTier, isControlPlane, isInitNodeOrDeleting,
		controlPlaneConcurrency, joinServer,
		controlPlaneDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return status, err
	}

	// If there are any suitable controlplane nodes with join URL annotations
	if len(collect(plan, roleAnd(isControlPlane, roleAnd(hasJoinURL, roleNot(isDeleting))))) == 0 {
		return status, errWaiting("waiting for control plane to be available")
	}

	if status.Initialized != true || status.Ready != true {
		status.Initialized = true
		status.Ready = true
		return status, errWaiting("marking control plane as initialized and ready")
	}

	// Process all nodes that are ONLY worker nodes.
	err = p.reconcile(cp, clusterSecretTokens, plan, false, workerTier, isOnlyWorker, isInitNodeOrDeleting,
		workerConcurrency, "",
		workerDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return status, err
	}

	if firstIgnoreError != nil {
		return status, errWaiting(firstIgnoreError.Error())
	}
	return status, nil
}

// getLowestMachineK8sVersion determines the lowest kubelet version in the plan
func getLowestMachineKubeletVersion(plan *plan.Plan) *semver.Version {
	var lowestVersion *semver.Version
	for _, machine := range plan.Machines {
		if machine.Status.NodeInfo != nil {
			ver, err := semver.NewVersion(machine.Status.NodeInfo.KubeletVersion)
			if err != nil {
				logrus.Errorf("error while parsing node kubelet version (%s): %v", machine.Status.NodeInfo.KubeletVersion, err)
				continue
			}
			if lowestVersion == nil {
				lowestVersion = ver
			} else {
				if ver.LessThan(lowestVersion) {
					lowestVersion = ver
				}
			}
		}
	}
	return lowestVersion
}

// clusterIsSane ensures that there is at least one controlplane, etcd, and worker node that are not deleting for the cluster.
func clusterIsSane(plan *plan.Plan) bool {
	if len(collect(plan, roleAnd(isEtcd, roleNot(isDeleting)))) == 0 || len(collect(plan, roleAnd(isControlPlane, roleNot(isDeleting)))) == 0 || len(collect(plan, roleAnd(isWorker, roleNot(isDeleting)))) == 0 {
		return false
	}
	return true
}

// calculateJoinURL will return a join URL based on calculating the checksum of the given machine UID. This is somewhat deterministic but will change when suitable machine lists change.
func calculateJoinURL(cp *rkev1.RKEControlPlane, entry *planEntry, plan *plan.Plan) string {
	if isInitNode(entry) {
		return "-"
	}

	entries := collect(plan, roleAnd(isControlPlane, roleAnd(hasJoinURL, roleNot(isDeleting))))

	if len(entries) == 0 {
		return ""
	}

	ck := crc32.ChecksumIEEE([]byte(entry.Machine.UID))
	if ck == math.MaxUint32 {
		ck--
	}

	scaled := int(ck) * len(entries) / math.MaxUint32
	logrus.Debugf("[planner] %s/%s: For machine %s/%s, determined join URL: %s (calculation of index: (%v * %v) / %v = [%v])", cp.Namespace, cp.Name, entry.Machine.Namespace, entry.Machine.Name, entries[scaled].Metadata.Annotations[capr.JoinURLAnnotation], ck, uint32(len(entries)), math.MaxUint32, scaled)
	return entries[scaled].Metadata.Annotations[capr.JoinURLAnnotation]
}

// determineJoinURL determines the join URL for the given entry. It will return different join URLs based on the entry passed in. If the joinURL is specified in the arguments, it will simply return the join URL without validation.
// If the entry is a worker-only node and joinURL is empty, it will validate the existing node the worker is joined to and return if valid. If the existing node is no longer valid, it will calculate a new join URL and return the new join URL.
func determineJoinURL(cp *rkev1.RKEControlPlane, entry *planEntry, plan *plan.Plan, joinURL string) (string, error) {
	if cp == nil || entry == nil || plan == nil {
		return "", fmt.Errorf("determineJoinURL arguments cannot be nil")
	}
	if !isOnlyWorker(entry) {
		return joinURL, nil
	}
	if joinURL == "" {
		// use the joinServer as specified ONLY if the existing joinServer is not valid for the cluster anymore. This is to prevent plan thrashing when a controlplane host is deleted.
		if entry.Plan != nil && entry.Plan.JoinedTo != "" {
			if validJoinURL(plan, entry.Plan.JoinedTo) {
				joinURL = entry.Plan.JoinedTo
			}
		}

		if joinURL == "" {
			// calculate the next join server for this node
			joinURL = calculateJoinURL(cp, entry, plan)
			joinedTo := ""
			if entry.Plan != nil {
				joinedTo = entry.Plan.JoinedTo
			}
			logrus.Infof("[planner] rkecluster %s/%s - machine %s/%s - previous join server (%s) was not valid, using new join server (%s)", cp.Namespace, cp.Name, entry.Machine.Namespace, entry.Machine.Name, joinedTo, joinURL)
			if joinURL == "" {
				return "", fmt.Errorf("no suitable join URL found to join machine %s/%s in rkecluster %s/%s to", entry.Machine.Namespace, entry.Machine.Name, cp.Namespace, cp.Name)
			}
		}
	}
	return joinURL, nil
}

// isUnavailable returns a boolean indicating whether the machine/node corresponding to the planEntry is available
// If the plan is not in sync, the machine is being drained, or there are is no new change expected and the probes are failing, it will return true.
func isUnavailable(r *reconcilable) bool {
	return !r.entry.Plan.InSync || isInDrain(r.entry) || (!r.change && !r.minorChange && !r.entry.Plan.Healthy)
}

// isInDrain returns a boolean indicating whether the machine/node corresponding to the planEntry is currently in any
// part of the drain process
func isInDrain(entry *planEntry) bool {
	return entry.Metadata.Annotations[capr.PreDrainAnnotation] != "" ||
		entry.Metadata.Annotations[capr.PostDrainAnnotation] != "" ||
		entry.Metadata.Annotations[capr.DrainAnnotation] != "" ||
		entry.Metadata.Annotations[capr.UnCordonAnnotation] != ""
}

// planAppliedButWaitingForProbes returns a boolean indicating whether a plan was successfully able to be applied, but
// the probes have not been successful. This indicates that while the overall plan hasn't completed yet, it's
// instructions have and can now be overridden if necessary without causing thrashing.
func planAppliedButWaitingForProbes(entry *planEntry) bool {
	return entry.Plan.AppliedPlan != nil && reflect.DeepEqual(entry.Plan.Plan, *entry.Plan.AppliedPlan) && !entry.Plan.Healthy
}

// planAppliedButProbesNeverHealthy returns a boolean indicating whether a plan was successfully able to be applied, but
// the probes have never been successful for the applied plan. This indicates that while the overall plan hasn't completed yet, it's
// instructions have and can now be overridden as there is likely a bad configuration applied.
func planAppliedButProbesNeverHealthy(entry *planEntry) bool {
	return entry.Plan.AppliedPlan != nil && reflect.DeepEqual(entry.Plan.Plan, *entry.Plan.AppliedPlan) && !entry.Plan.Healthy && !entry.Plan.ProbesUsable
}

func calculateConcurrency(maxUnavailable string, reconcilables []*reconcilable, exclude roleFilter) (int, int, error) {
	var (
		count, unavailable int
	)

	for _, r := range reconcilables {
		if !exclude(r.entry) {
			count++
		}
		if r.entry.Plan != nil && isUnavailable(r) {
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

func minorPlanChangeDetected(old, new plan.NodePlan) bool {
	if !equality.Semantic.DeepEqual(old.Instructions, new.Instructions) ||
		!equality.Semantic.DeepEqual(old.PeriodicInstructions, new.PeriodicInstructions) ||
		!equality.Semantic.DeepEqual(old.Probes, new.Probes) ||
		old.Error != new.Error {
		return false
	}

	if len(old.Files) == 0 && len(new.Files) == 0 {
		// if the old plan had no files and no new files were found, there was no plan change detected
		return false
	}

	newFiles := make(map[string]plan.File)
	for _, newFile := range new.Files {
		newFiles[newFile.Path] = newFile
	}

	for _, oldFile := range old.Files {
		if newFile, ok := newFiles[oldFile.Path]; ok {
			if oldFile.Content == newFile.Content {
				// If the file already exists, we don't care if it is minor
				delete(newFiles, oldFile.Path)
			}
		} else {
			// the old file didn't exist in the new file map,
			// so check to see if the old file is major and if it is, this is not a minor change.
			if !oldFile.Minor {
				return false
			}
		}
	}

	if len(newFiles) > 0 {
		// If we still have new files in the list, check to see if any of them are major, and if they are, this is not a major change
		for _, newFile := range newFiles {
			// if we find a new major file, there is not a minor change
			if !newFile.Minor {
				return false
			}
		}
		// There were new files and all were not major
		return true
	}
	return false
}

func kubeletVersionUpToDate(controlPlane *rkev1.RKEControlPlane, machine *capi.Machine) bool {
	if controlPlane == nil || machine == nil || machine.Status.NodeInfo == nil || !controlPlane.Status.AgentConnected {
		// If any of controlPlane, machine, or machine.Status.NodeInfo are nil, then provisioning is still happening.
		// If controlPlane.Status.AgentConnected is false, then it cannot be reliably determined if the kubelet is up-to-date.
		// Return true so that provisioning is not slowed down.
		return true
	}

	kubeletVersion, err := semver.NewVersion(strings.TrimPrefix(machine.Status.NodeInfo.KubeletVersion, "v"))
	if err != nil {
		return false
	}

	kubernetesVersion, err := semver.NewVersion(strings.TrimPrefix(controlPlane.Spec.KubernetesVersion, "v"))
	if err != nil {
		return false
	}

	// Compare and ignore pre-release and build metadata
	return kubeletVersion.Major() == kubernetesVersion.Major() && kubeletVersion.Minor() == kubernetesVersion.Minor() && kubeletVersion.Patch() == kubernetesVersion.Patch()
}

// splitArgKeyVal takes a value and returns a pair (key, value) of the argument, or two empty strings if there was not
// a parsed key/val.
func splitArgKeyVal(val string, delim string) (string, string) {
	if splitSubArg := strings.SplitN(val, delim, 2); len(splitSubArg) == 2 {
		return splitSubArg[0], splitSubArg[1]
	}
	return "", ""
}

// getArgValue will search the passed in interface (arg) for a key that matches the searchArg. If a match is found, it
// returns the value of the argument, otherwise it returns an empty string.
func getArgValue(arg interface{}, searchArg string, delim string) string {
	logrus.Tracef("getArgValue (searchArg: %s, delim: %s) type of %v is %T", searchArg, delim, arg, arg)
	switch arg := arg.(type) {
	case []interface{}:
		logrus.Tracef("getArgValue (searchArg: %s, delim: %s) encountered interface slice %v", searchArg, delim, arg)
		return getArgValue(convertInterfaceSliceToStringSlice(arg), searchArg, delim)
	case []string:
		logrus.Tracef("getArgValue (searchArg: %s, delim: %s) found string array: %v", searchArg, delim, arg)
		for _, v := range arg {
			argKey, argVal := splitArgKeyVal(v, delim)
			if argKey == searchArg {
				return argVal
			}
		}
	case string:
		logrus.Tracef("getArgValue (searchArg: %s, delim: %s) found string: %v", searchArg, delim, arg)
		argKey, argVal := splitArgKeyVal(arg, delim)
		if argKey == searchArg {
			return argVal
		}
	}
	logrus.Tracef("getArgValue (searchArg: %s, delim: %s) did not find searchArg in: %v", searchArg, delim, arg)
	return ""
}

// convertInterfaceSliceToStringSlice converts an input interface slice to a string slice by iterating through the
// interface slice and converting each entry to a string using Sprintf.
func convertInterfaceSliceToStringSlice(input []interface{}) []string {
	var stringArr []string
	for _, v := range input {
		stringArr = append(stringArr, fmt.Sprintf("%v", v))
	}
	return stringArr
}

// appendToInterface will return an interface that has the value appended to it. The interface returned will always be
// a slice of strings, and will convert a raw string to a slice of strings.
func appendToInterface(input interface{}, elem string) []string {
	switch input := input.(type) {
	case []interface{}:
		stringArr := convertInterfaceSliceToStringSlice(input)
		return appendToInterface(stringArr, elem)
	case []string:
		return append(input, elem)
	case string:
		return []string{input, elem}
	}
	return []string{elem}
}

// convertInterfaceToStringSlice converts an input interface to a string slice by determining its type and converting
// it accordingly. If it is not a known convertible type, an empty string slice is returned.
func convertInterfaceToStringSlice(input interface{}) []string {
	switch input := input.(type) {
	case []interface{}:
		return convertInterfaceSliceToStringSlice(input)
	case []string:
		return input
	case string:
		return []string{input}
	}
	return []string{}
}

// renderArgAndMount takes the value of the existing value of the argument and mount and renders an output argument and
// mount based on the value of the input interfaces. It will always return a set of slice of strings.
func renderArgAndMount(existingArg interface{}, existingMount interface{}, controlPlane *rkev1.RKEControlPlane, defaultSecurePort string, defaultCertDir string) ([]string, []string) {
	retArg := convertInterfaceToStringSlice(existingArg)
	retMount := convertInterfaceToStringSlice(existingMount)
	renderedCertDir := path.Join(capr.GetDistroDataDir(controlPlane), defaultCertDir)
	// Set a default value for certDirArg and certDirMount (for the case where the user does not set these values)
	// If a user sets these values, we will set them to an empty string and check to make sure they are not empty
	// strings before adding them to the rendered arg/mount slices.
	certDirMount := fmt.Sprintf("%s:%s", renderedCertDir, renderedCertDir)
	certDirArg := fmt.Sprintf("%s=%s", CertDirArgument, renderedCertDir)
	securePortArg := fmt.Sprintf("%s=%s", SecurePortArgument, defaultSecurePort)
	if len(retArg) > 0 {
		tlsCF := getArgValue(retArg, TLSCertFileArgument, "=")
		if tlsCF == "" {
			// If the --tls-cert-file Argument was not set in the config for this component, we can look to see if
			// the --cert-dir was set. --tls-cert-file (if set) will take precedence over --tls-cert-file
			certDir := getArgValue(retArg, CertDirArgument, "=")
			if certDir != "" {
				// If --cert-dir was set, we use the --cert-dir that the user provided and should set certDirArg to ""
				// so that we don't append it.
				certDirArg = ""
				// Set certDirMount to an intelligently interpolated value based off of the custom certDir set by the
				// user.
				certDirMount = fmt.Sprintf("%s:%s", certDir, certDir)
			}
		} else {
			// If the --tls-cert-file argument was set by the user, we don't need to set --cert-dir, but still should
			// render a --cert-dir-mount that is based on the --tls-cert-file argument to map the files necessary
			// to the static pod (in the RKE2 case)
			certDirArg = ""
			dir := filepath.Dir(tlsCF)
			certDirMount = fmt.Sprintf("%s:%s", dir, dir)
		}
		sPA := getArgValue(retArg, SecurePortArgument, "=")
		if sPA != "" {
			// If the user set a custom --secure-port, set --secure-port to an empty string so we don't override
			// their custom value
			securePortArg = ""
		}
	}
	if certDirArg != "" {
		logrus.Debugf("renderArgAndMount adding %s to component arguments", certDirArg)
		retArg = appendToInterface(existingArg, certDirArg)
	}
	if securePortArg != "" {
		logrus.Debugf("renderArgAndMount adding %s to component arguments", securePortArg)
		retArg = appendToInterface(retArg, securePortArg)
	}
	if capr.GetRuntime(controlPlane.Spec.KubernetesVersion) == capr.RuntimeRKE2 {
		// todo: make sure the certDirMount is not already set by the user to some custom value before we set it for the static pod extraMount
		logrus.Debugf("renderArgAndMount adding %s to component mounts", certDirMount)
		retMount = appendToInterface(existingMount, certDirMount)
	}
	return retArg, retMount
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

// getTaints returns a slice of taints for the machine in question
func getTaints(entry *planEntry, cp *rkev1.RKEControlPlane) (result []corev1.Taint, _ error) {
	data := entry.Metadata.Annotations[capr.TaintsAnnotation]
	if data != "" {
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return result, err
		}
	}

	if !isWorker(entry) {
		// k3s charts do not have correct tolerations when the master node is both controlplane and etcd
		if isEtcd(entry) && (capr.GetRuntime(cp.Spec.KubernetesVersion) != capr.RuntimeK3S || !isControlPlane(entry)) {
			result = append(result, corev1.Taint{
				Key:    "node-role.kubernetes.io/etcd",
				Effect: corev1.TaintEffectNoExecute,
			})
		}
		if isControlPlane(entry) {
			result = append(result, corev1.Taint{
				Key:    "node-role.kubernetes.io/control-plane",
				Effect: corev1.TaintEffectNoSchedule,
			})
		}
	}

	return
}

type reconcilable struct {
	entry       *planEntry
	desiredPlan plan.NodePlan
	joinedURL   string
	change      bool
	minorChange bool
}

func (p *Planner) reconcile(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, clusterPlan *plan.Plan, required bool,
	tierName string, include, exclude roleFilter, maxUnavailable string, forcedJoinURL string, drainOptions rkev1.DrainOptions) error {
	var (
		ready, outOfSync, nonReady, errMachines, draining, uncordoned []string
		messages                                                      = map[string][]string{}
	)

	entries := collect(clusterPlan, include)

	var reconcilables []*reconcilable

	for _, entry := range entries {
		if exclude(entry) {
			continue
		}

		joinURL, err := determineJoinURL(controlPlane, entry, clusterPlan, forcedJoinURL)
		if err != nil {
			return err
		}

		logrus.Debugf("[planner] rkecluster %s/%s reconcile tier %s - rendering desired plan for machine %s/%s with join URL: (%s)", controlPlane.Namespace, controlPlane.Name, tierName, entry.Machine.Namespace, entry.Machine.Name, joinURL)
		plan, joinedURL, err := p.desiredPlan(controlPlane, tokensSecret, entry, joinURL)
		if err != nil {
			return err
		}
		reconcilables = append(reconcilables, &reconcilable{
			entry:       entry,
			desiredPlan: plan,
			joinedURL:   joinedURL,
			change:      entry.Plan != nil && !equality.Semantic.DeepEqual(entry.Plan.Plan, plan),
			minorChange: entry.Plan != nil && minorPlanChangeDetected(entry.Plan.Plan, plan),
		})
	}

	concurrency, unavailable, err := calculateConcurrency(maxUnavailable, reconcilables, exclude)
	if err != nil {
		return err
	}

	preBootstrapManifests, err := p.retrievalFunctions.GetBootstrapManifests(controlPlane)
	if err != nil {
		return err
	}

	for _, r := range reconcilables {
		logrus.Tracef("[planner] rkecluster %s/%s reconcile tier %s - processing machine entry: %s/%s", controlPlane.Namespace, controlPlane.Name, tierName, r.entry.Machine.Namespace, r.entry.Machine.Name)
		// we exclude here and not in collect to ensure that include matched at least one node
		if exclude(r.entry) {
			logrus.Tracef("[planner] rkecluster %s/%s reconcile tier %s - excluding machine entry: %s/%s", controlPlane.Namespace, controlPlane.Name, tierName, r.entry.Machine.Namespace, r.entry.Machine.Name)
			continue
		}

		// The Reconciled condition should be removed when summarizing so that the messages are not duplicated.
		summary := summary.Summarize(removeReconciledCondition(r.entry.Machine))
		if summary.Error {
			errMachines = append(errMachines, r.entry.Machine.Name)
		}
		if summary.Transitioning {
			nonReady = append(nonReady, r.entry.Machine.Name)
		}

		planStatusMessage := getPlanStatusReasonMessage(r.entry)
		if planStatusMessage != "" {
			summary.Message = append(summary.Message, planStatusMessage)
		}
		messages[r.entry.Machine.Name] = summary.Message

		if r.entry.Plan == nil {
			logrus.Debugf("[planner] rkecluster %s/%s reconcile tier %s - setting initial plan for machine %s/%s", controlPlane.Namespace, controlPlane.Name, tierName, r.entry.Machine.Namespace, r.entry.Machine.Name)
			logrus.Tracef("[planner] rkecluster %s/%s reconcile tier %s - initial plan for machine %s/%s new: %+v", controlPlane.Namespace, controlPlane.Name, tierName, r.entry.Machine.Namespace, r.entry.Machine.Name, r.desiredPlan)
			outOfSync = append(outOfSync, r.entry.Machine.Name)
			if err := p.store.UpdatePlan(r.entry, r.desiredPlan, r.joinedURL, -1, 1); err != nil {
				return err
			}
		} else if r.minorChange {
			logrus.Debugf("[planner] rkecluster %s/%s reconcile tier %s - minor plan change detected for machine %s/%s, updating plan immediately", controlPlane.Namespace, controlPlane.Name, tierName, r.entry.Machine.Namespace, r.entry.Machine.Name)
			logrus.Tracef("[planner] rkecluster %s/%s reconcile tier %s - minor plan change for machine %s/%s old: %+v, new: %+v", controlPlane.Namespace, controlPlane.Name, tierName, r.entry.Machine.Namespace, r.entry.Machine.Name, r.entry.Plan.Plan, r.desiredPlan)
			outOfSync = append(outOfSync, r.entry.Machine.Name)
			if err := p.store.UpdatePlan(r.entry, r.desiredPlan, r.joinedURL, -1, 1); err != nil {
				return err
			}
		} else if r.change {
			logrus.Debugf("[planner] rkecluster %s/%s reconcile tier %s - plan for machine %s/%s did not match, appending to outOfSync", controlPlane.Namespace, controlPlane.Name, tierName, r.entry.Machine.Namespace, r.entry.Machine.Name)
			outOfSync = append(outOfSync, r.entry.Machine.Name)
			// Conditions
			// 1. If the node is already draining then the plan is out of sync.  There is no harm in updating it if
			// the node is currently drained.
			// 2. If the plan has failed to apply. Note that the `Failed` will only be `true` if the max failure count has passed, or (if max-failures is not set) the plan has failed to apply at least once.
			// 3. concurrency == 0 which means infinite concurrency.
			// 4. unavailable < concurrency meaning we have capacity to make something unavailable
			// 5. If the plan was successful in application but the probes never went healthy
			logrus.Debugf("[planner] rkecluster %s/%s reconcile tier %s - concurrency: %d, unavailable: %d", controlPlane.Namespace, controlPlane.Name, tierName, concurrency, unavailable)
			if isInDrain(r.entry) || r.entry.Plan.Failed || concurrency == 0 || unavailable < concurrency || planAppliedButProbesNeverHealthy(r.entry) {
				if !isUnavailable(r) {
					unavailable++
				}
				if ok, err := p.drain(r.entry.Plan.AppliedPlan, r.desiredPlan, r.entry, clusterPlan, drainOptions); !ok && err != nil {
					return err
				} else if ok && err == nil {
					// Drain is done (or didn't need to be done) and there are no errors, so the plan should be updated to enact the reason the node was drained.
					logrus.Debugf("[planner] rkecluster %s/%s reconcile tier %s - major plan change for machine %s/%s", controlPlane.Namespace, controlPlane.Name, tierName, r.entry.Machine.Namespace, r.entry.Machine.Name)
					logrus.Tracef("[planner] rkecluster %s/%s reconcile tier %s - major plan change for machine %s/%s old: %+v, new: %+v", controlPlane.Namespace, controlPlane.Name, tierName, r.entry.Machine.Namespace, r.entry.Machine.Name, r.entry.Plan.Plan, r.desiredPlan)
					if err = p.store.UpdatePlan(r.entry, r.desiredPlan, r.joinedURL, -1, 1); err != nil {
						return err
					} else if r.entry.Metadata.Annotations[capr.DrainDoneAnnotation] != "" {
						messages[r.entry.Machine.Name] = append(messages[r.entry.Machine.Name], "drain completed")
					} else if planStatusMessage == "" {
						messages[r.entry.Machine.Name] = append(messages[r.entry.Machine.Name], WaitingPlanStatusMessage)
					}
				} else {
					// In this case, it is true that ((ok == true && err != nil) || (ok == false && err == nil))
					// The first case indicates that there is an error trying to drain the node.
					// The second case indicates that the node is draining.
					draining = append(draining, r.entry.Machine.Name)
					if err != nil {
						messages[r.entry.Machine.Name] = append(messages[r.entry.Machine.Name], err.Error())
					} else {
						messages[r.entry.Machine.Name] = append(messages[r.entry.Machine.Name], "draining node")
					}
				}
			}
		} else if planStatusMessage != "" {
			outOfSync = append(outOfSync, r.entry.Machine.Name)
		} else if ok, err := p.undrain(r.entry); !ok && err != nil {
			return err
		} else if !ok || err != nil {
			// The uncordoning is happening or there was an error.
			// Either way, the planner should wait for the result and display the message on the machine.
			uncordoned = append(uncordoned, r.entry.Machine.Name)
			if err != nil {
				messages[r.entry.Machine.Name] = append(messages[r.entry.Machine.Name], err.Error())
			} else {
				messages[r.entry.Machine.Name] = append(messages[r.entry.Machine.Name], "waiting for uncordon to finish")
			}
		} else if !kubeletVersionUpToDate(controlPlane, r.entry.Machine) {
			outOfSync = append(outOfSync, r.entry.Machine.Name)
			messages[r.entry.Machine.Name] = append(messages[r.entry.Machine.Name], "waiting for kubelet to update")
		} else if isControlPlane(r.entry) && len(preBootstrapManifests) > 0 {
			outOfSync = append(outOfSync, r.entry.Machine.Name)
			messages[r.entry.Machine.Name] = append(messages[r.entry.Machine.Name], "waiting for cluster pre-bootstrap to complete")
		} else if isControlPlane(r.entry) && !controlPlane.Status.AgentConnected {
			// If the control plane nodes are currently being provisioned/updated, then it should be ensured that cluster-agent is connected.
			// Without the agent connected, the controllers running in Rancher, including CAPI, can't communicate with the downstream cluster.
			outOfSync = append(outOfSync, r.entry.Machine.Name)
			messages[r.entry.Machine.Name] = append(messages[r.entry.Machine.Name], "waiting for cluster agent to connect")
		} else {
			ready = append(ready, r.entry.Machine.Name)
		}
	}

	if required && len(entries) == 0 {
		return errWaiting("waiting for at least one " + tierName + " node")
	}

	// If multiple machines are changing status, then all of their statuses should be updated to avoid having stale conditions.
	// However, only the first one will be returned so that status goes on the control plane and cluster objects.
	var firstError error
	if err := p.setMachineConditionStatus(clusterPlan, uncordoned, fmt.Sprintf("uncordoning %s node(s) ", tierName), messages); err != nil && firstError == nil {
		firstError = err
	}

	if err := p.setMachineConditionStatus(clusterPlan, draining, fmt.Sprintf("draining %s node(s) ", tierName), messages); err != nil && firstError == nil {
		firstError = err
	}

	if err := p.setMachineConditionStatus(clusterPlan, outOfSync, fmt.Sprintf("configuring %s node(s) ", tierName), messages); err != nil && firstError == nil {
		firstError = err
	}

	// Ensure that the conditions that we control are updated.
	if err := p.setMachineConditionStatus(clusterPlan, ready, "", nil); err != nil && firstError == nil {
		firstError = err
	}

	if firstError != nil {
		return firstError
	}

	// The messages for these machines come from the machine itself, so nothing needs to be added.
	// we want these errors to get reported, but not block the process
	if len(errMachines) > 0 {
		return errIgnore("failing " + tierName + " machine(s) " + atMostThree(errMachines) + detailedMessage(errMachines, messages))
	}

	if len(nonReady) > 0 {
		return errIgnore("non-ready " + tierName + " machine(s) " + atMostThree(nonReady) + detailedMessage(nonReady, messages))
	}

	return nil
}

// generatePlanWithConfigFiles will generate a node plan with the corresponding config files for the entry in question.
// Notably, it will discard the existing nodePlan in the given entry. It returns the new node plan, the config that was
// rendered, the rendered join server ("-" in the case that the plan is generated for an init node), and an error (if one exists).
func (p *Planner) generatePlanWithConfigFiles(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, entry *planEntry, joinServer string, renderS3 bool) (plan.NodePlan, map[string]interface{}, string, error) {
	var (
		reg      registries
		nodePlan plan.NodePlan
		err      error
	)

	if !controlPlane.Spec.UnmanagedConfig {
		nodePlan, reg, err = p.commonNodePlan(controlPlane, plan.NodePlan{})
		if err != nil {
			return nodePlan, map[string]interface{}{}, "", err
		}
		var (
			joinedServer string
			config       map[string]interface{}
		)

		nodePlan, config, joinedServer, err = p.addConfigFile(nodePlan, controlPlane, entry, tokensSecret, joinServer, reg, renderS3)
		if err != nil {
			return nodePlan, config, joinedServer, err
		}

		nodePlan, err = p.addManifests(nodePlan, controlPlane, entry)
		if err != nil {
			return nodePlan, config, joinedServer, err
		}

		nodePlan, err = p.addChartConfigs(nodePlan, controlPlane, entry)
		if err != nil {
			return nodePlan, config, joinedServer, err
		}

		nodePlan, err = addOtherFiles(nodePlan, controlPlane, entry)

		idempotentScriptFile := plan.File{
			Content: base64.StdEncoding.EncodeToString([]byte(idempotentActionScript)),
			Path:    idempotentActionScriptPath(controlPlane),
			Dynamic: true,
			Minor:   true,
		}

		nodePlan.Files = append(nodePlan.Files, idempotentScriptFile)

		return nodePlan, config, joinedServer, err
	}

	return plan.NodePlan{}, map[string]interface{}{}, "", nil
}

func (p *Planner) desiredPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, entry *planEntry, joinServer string) (plan.NodePlan, string, error) {
	nodePlan, config, joinedTo, err := p.generatePlanWithConfigFiles(controlPlane, tokensSecret, entry, joinServer, true)
	if err != nil {
		return nodePlan, joinedTo, err
	}

	probes, err := p.generateProbes(controlPlane, entry, config)
	if err != nil {
		return nodePlan, joinedTo, err
	}
	nodePlan.Probes = probes

	// Add instruction last because it hashes config content
	nodePlan, err = p.addInstallInstructionWithRestartStamp(nodePlan, controlPlane, entry)
	if err != nil {
		return nodePlan, joinedTo, err
	}

	if isInitNode(entry) && IsOnlyEtcd(entry) {
		// If the annotation to disable autosetting the join URL is enabled, don't deliver a plan to add the periodic instruction to scrape init node.
		if _, autosetDisabled := entry.Metadata.Annotations[capr.JoinURLAutosetDisabled]; !autosetDisabled {
			nodePlan, err = p.addInitNodePeriodicInstruction(nodePlan, controlPlane)
			if err != nil {
				return nodePlan, joinedTo, err
			}
		}
	}

	if windows(entry) {
		// We need to wait for the controlPlane to be ready before sending this plan
		// to ensure that the initial installation has fully completed
		if controlPlane.Status.Ready {
			nodePlan.Files = append(nodePlan.Files, setPermissionsWindowsScriptFile)
			nodePlan.Instructions = append(nodePlan.Instructions, setPermissionsWindowsScriptInstruction)
		}
	}

	if isEtcd(entry) {
		nodePlan, err = p.addEtcdSnapshotListLocalPeriodicInstruction(nodePlan, controlPlane)
		if err != nil {
			return nodePlan, joinedTo, err
		}
		if controlPlane != nil && controlPlane.Spec.ETCD != nil && S3Enabled(controlPlane.Spec.ETCD.S3) && isInitNode(entry) {
			nodePlan, err = p.addEtcdSnapshotListS3PeriodicInstruction(nodePlan, controlPlane)
			if err != nil {
				return nodePlan, joinedTo, err
			}
		}
	}
	return nodePlan, joinedTo, nil
}

// getInstallerImage returns the correct system-agent-installer image for a given controlplane
func (p *Planner) getInstallerImage(controlPlane *rkev1.RKEControlPlane) string {
	runtime := capr.GetRuntime(controlPlane.Spec.KubernetesVersion)
	installerImage := p.retrievalFunctions.SystemAgentImage() + runtime + ":" + strings.ReplaceAll(controlPlane.Spec.KubernetesVersion, "+", "-")
	return p.retrievalFunctions.ImageResolver(installerImage, controlPlane)
}

// ensureRKEStateSecret ensures that the RKE state secret for the given RKEControlPlane exists. This secret contains the
// serverToken and agentToken used for server/agent registration.
func (p *Planner) ensureRKEStateSecret(controlPlane *rkev1.RKEControlPlane, newCluster bool) (string, plan.Secret, error) {
	if controlPlane.Spec.UnmanagedConfig {
		return "", plan.Secret{}, nil
	}

	name := name.SafeConcatName(controlPlane.Name, "rke", "state")
	secret, err := p.secretCache.Get(controlPlane.Namespace, name)
	if apierror.IsNotFound(err) {
		if !newCluster {
			return "", plan.Secret{}, fmt.Errorf("newCluster was false and secret does not exist: %w", err)
		}
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
						APIVersion: capr.RKEAPIVersion,
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
			Type: capr.SecretTypeClusterState,
		}

		_, err = p.secretClient.Create(secret)
		return name, plan.Secret{
			ServerToken: serverToken,
			AgentToken:  agentToken,
		}, err
	} else if err != nil {
		return "", plan.Secret{}, err
	}

	if secret.Type != capr.SecretTypeClusterState {
		return "", plan.Secret{}, fmt.Errorf("secret %s/%s type %s did not match expected type %s", secret.Namespace, secret.Name, secret.Type, capr.SecretTypeClusterState)
	}

	return secret.Name, plan.Secret{
		ServerToken: string(secret.Data["serverToken"]),
		AgentToken:  string(secret.Data["agentToken"]),
	}, nil
}

// pauseCAPICluster reconciles the given boolean to the owning CAPI cluster. Notably, it retries if there is a conflict
// editing the CAPI cluster as there are many controllers that may be racing to edit the object, but there is only one
// controller that should actively be toggling the paused field on the CAPI cluster object.
func (p *Planner) pauseCAPICluster(cp *rkev1.RKEControlPlane, pause bool) error {
	if cp == nil {
		return fmt.Errorf("cannot pause CAPI cluster for nil controlplane")
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cluster, err := capr.GetOwnerCAPICluster(cp, p.capiClusters)
		if err != nil {
			return err
		}
		if cluster == nil {
			return fmt.Errorf("CAPI cluster does not exist for %s/%s", cp.Namespace, cp.Name)
		}
		cluster = cluster.DeepCopy()
		if cluster.Spec.Paused == pause {
			return nil
		}
		cluster.Spec.Paused = pause
		_, err = p.capiClient.Update(cluster)
		return err
	})
}

// ensureCAPIClusterControlPlaneInitializedFalse retrieves the CAPI cluster from cache and sets the ControlPlaneInitializedCondition
// to False if it is not already False.
func (p *Planner) ensureCAPIClusterControlPlaneInitializedFalse(cp *rkev1.RKEControlPlane) error {
	if cp == nil {
		return fmt.Errorf("cannot uninitialize CAPI cluster for nil controlplane")
	}
	cluster, err := capr.GetOwnerCAPICluster(cp, p.capiClusters)
	if err != nil {
		return err
	}
	if cluster == nil {
		return fmt.Errorf("CAPI cluster does not exist for %s/%s", cp.Namespace, cp.Name)
	}
	cluster = cluster.DeepCopy()
	if !conditions.IsFalse(cluster, capi.ControlPlaneInitializedCondition) {
		conditions.MarkFalse(cluster, capi.ControlPlaneInitializedCondition, capi.WaitingForControlPlaneProviderInitializedReason, capi.ConditionSeverityInfo, "Waiting for control plane provider to indicate the control plane has been initialized")
		_, err = p.capiClient.UpdateStatus(cluster)
	}
	return err
}
