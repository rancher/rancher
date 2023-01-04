package planner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/moby/locker"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	ranchercontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/randomtoken"
	"github.com/rancher/wrangler/pkg/summary"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

const (
	clusterRegToken = "clusterRegToken"

	EtcdSnapshotConfigMapKey = "provisioning-cluster-spec"

	KubeControllerManagerArg                      = "kube-controller-manager-arg"
	KubeControllerManagerExtraMount               = "kube-controller-manager-extra-mount"
	DefaultKubeControllerManagerCertDir           = "/var/lib/rancher/%s/server/tls/kube-controller-manager"
	DefaultKubeControllerManagerDefaultSecurePort = "10257"
	DefaultKubeControllerManagerCert              = "kube-controller-manager.crt"
	KubeSchedulerArg                              = "kube-scheduler-arg"
	KubeSchedulerExtraMount                       = "kube-scheduler-extra-mount"
	DefaultKubeSchedulerCertDir                   = "/var/lib/rancher/%s/server/tls/kube-scheduler"
	DefaultKubeSchedulerDefaultSecurePort         = "10259"
	DefaultKubeSchedulerCert                      = "kube-scheduler.crt"
	SecurePortArgument                            = "secure-port"
	CertDirArgument                               = "cert-dir"
	TLSCertFileArgument                           = "tls-cert-file"

	authnWebhookFileName = "/var/lib/rancher/%s/kube-api-authn-webhook.yaml"
	ConfigYamlFileName   = "/etc/rancher/%s/config.yaml.d/50-rancher.yaml"

	windows = "windows"

	bootstrapTier    = "bootstrap"
	etcdTier         = "etcd"
	controlPlaneTier = "control plane"
	workerTier       = "worker"
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

func ErrWaitingf(format string, a ...interface{}) ErrWaiting {
	return ErrWaiting(fmt.Sprintf(format, a...))
}

type errIgnore string

func (e errIgnore) Error() string {
	return string(e)
}

type roleFilter func(*planEntry) bool

type Planner struct {
	ctx                           context.Context
	store                         *PlanStore
	rkeControlPlanes              rkecontrollers.RKEControlPlaneController
	etcdSnapshotCache             rkecontrollers.ETCDSnapshotCache
	secretClient                  corecontrollers.SecretClient
	secretCache                   corecontrollers.SecretCache
	machines                      capicontrollers.MachineClient
	machinesCache                 capicontrollers.MachineCache
	clusterRegistrationTokenCache mgmtcontrollers.ClusterRegistrationTokenCache
	capiClient                    capicontrollers.ClusterClient
	capiClusters                  capicontrollers.ClusterCache
	managementClusters            mgmtcontrollers.ClusterCache
	rancherClusterCache           ranchercontrollers.ClusterCache
	locker                        locker.Locker
	etcdS3Args                    s3Args
	certificateRotation           *certificateRotation
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
		machinesCache:                 clients.CAPI.Machine().Cache(),
		secretClient:                  clients.Core.Secret(),
		secretCache:                   clients.Core.Secret().Cache(),
		clusterRegistrationTokenCache: clients.Mgmt.ClusterRegistrationToken().Cache(),
		capiClient:                    clients.CAPI.Cluster(),
		capiClusters:                  clients.CAPI.Cluster().Cache(),
		managementClusters:            clients.Mgmt.Cluster().Cache(),
		rancherClusterCache:           clients.Provisioning.Cluster().Cache(),
		rkeControlPlanes:              clients.RKE.RKEControlPlane(),
		etcdSnapshotCache:             clients.RKE.ETCDSnapshot().Cache(),
		etcdS3Args: s3Args{
			secretCache: clients.Core.Secret().Cache(),
		},
		certificateRotation: newCertificateRotation(clients, store),
	}
}

func (p *Planner) setMachineConditionStatus(clusterPlan *plan.Plan, machineNames []string, messagePrefix string, messages map[string][]string) error {
	var waiting bool
	for _, machineName := range machineNames {
		machine := clusterPlan.Machines[machineName]
		if machine == nil {
			return fmt.Errorf("found unexpected machine %s that is not in cluster plan", machineName)
		}

		if !rke2.InfrastructureReady.IsTrue(machine) {
			waiting = true
			continue
		}

		machine = machine.DeepCopy()
		if message := messages[machineName]; len(message) > 0 {
			msg := strings.Join(message, ", ")
			waiting = true
			if rke2.Reconciled.GetMessage(machine) == msg {
				continue
			}
			conditions.MarkUnknown(machine, capi.ConditionType(rke2.Reconciled), "Waiting", msg)
		} else if !rke2.Reconciled.IsTrue(machine) {
			// Since there is no status message, then the condition should be set to true.
			conditions.MarkTrue(machine, capi.ConditionType(rke2.Reconciled))

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
		return ErrWaiting(messagePrefix + atMostThree(machineNames) + detailMessage(machineNames, messages))
	}
	return nil
}

func atMostThree(names []string) string {
	sort.Strings(names)
	if len(names) > 3 {
		return fmt.Sprintf("%s and %d more", strings.Join(names[:3], ","), len(names)-3)
	}
	return strings.Join(names, ",")
}

func detailMessage(machines []string, messages map[string][]string) string {
	if len(machines) != 1 {
		return ""
	}
	message := messages[machines[0]]
	if len(message) != 0 {
		return fmt.Sprintf(": %s", strings.Join(message, ", "))
	}
	return ""
}

func removeConfiguredCondition(machine *capi.Machine) *capi.Machine {
	if machine == nil || len(machine.Status.Conditions) == 0 {
		return machine
	}

	conds := make([]capi.Condition, 0, len(machine.Status.Conditions))
	for _, c := range machine.Status.Conditions {
		if string(c.Type) != string(rke2.Reconciled) {
			conds = append(conds, c)
		}
	}

	if len(conds) == len(machine.Status.Conditions) {
		return machine
	}

	machine = machine.DeepCopy()
	machine.SetConditions(conds)
	return machine
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
	logrus.Debugf("[planner] rkecluster %s/%s: attempting to lock %s for processing", controlPlane.Namespace, controlPlane.Name, string(controlPlane.UID))
	p.locker.Lock(string(controlPlane.UID))
	defer func(namespace, name, uid string) error {
		logrus.Debugf("[planner] rkecluster %s/%s: unlocking %s", namespace, name, uid)
		return p.locker.Unlock(uid)
	}(controlPlane.Namespace, controlPlane.Name, string(controlPlane.UID))

	releaseData := rke2.GetKDMReleaseData(p.ctx, controlPlane)
	if releaseData == nil {
		return ErrWaitingf("rkecluster %s/%s: releaseData nil for version %s", controlPlane.Namespace, controlPlane.Name, controlPlane.Spec.KubernetesVersion)
	}

	cluster, err := p.getCAPICluster(controlPlane)
	if err != nil || !cluster.DeletionTimestamp.IsZero() {
		return err
	}

	plan, err := p.store.Load(cluster, controlPlane)
	if err != nil {
		return err
	}

	controlPlane, clusterSecretTokens, err := p.generateSecrets(controlPlane)
	if err != nil {
		return err
	}

	var (
		firstIgnoreError error
		joinServer       string
	)

	if errs := p.createEtcdSnapshot(controlPlane, clusterSecretTokens, plan); len(errs) > 0 {
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

	if err = p.restoreEtcdSnapshot(controlPlane, clusterSecretTokens, plan); err != nil {
		return err
	}

	// on the first run through, electInitNode will return a `generic.ErrSkip` as it is attempting to wait for the cache to catch up.
	joinServer, err = p.electInitNode(controlPlane, plan)
	if err != nil {
		return err
	}

	if err = p.certificateRotation.RotateCertificates(controlPlane, plan); err != nil {
		return err
	}

	if err = p.rotateEncryptionKeys(controlPlane, releaseData, plan); err != nil {
		return err
	}

	// select all etcd and then filter to just initNodes to that unavailable count is correct
	err = p.reconcile(controlPlane, clusterSecretTokens, plan, true, bootstrapTier, isEtcd, isNotInitNodeOrIsDeleting,
		"1", "",
		controlPlane.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return err
	}

	if joinServer == "" {
		_, joinServer, _, err = p.findInitNode(controlPlane, plan)
		if err != nil {
			return err
		} else if joinServer == "" && firstIgnoreError != nil {
			return ErrWaiting(firstIgnoreError.Error() + " and join url to be available on bootstrap node")
		} else if joinServer == "" {
			return ErrWaiting("waiting for join url to be available on bootstrap node")
		}
	}

	err = p.reconcile(controlPlane, clusterSecretTokens, plan, true, etcdTier, isEtcd, isInitNodeOrDeleting,
		"1", joinServer,
		controlPlane.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return err
	}

	err = p.reconcile(controlPlane, clusterSecretTokens, plan, true, controlPlaneTier, isControlPlane, isInitNodeOrDeleting,
		controlPlane.Spec.UpgradeStrategy.ControlPlaneConcurrency, joinServer,
		controlPlane.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return err
	}

	joinServer = getControlPlaneJoinURL(plan)
	if joinServer == "" {
		return ErrWaiting("waiting for control plane to be available")
	}

	err = p.reconcile(controlPlane, clusterSecretTokens, plan, false, workerTier, isOnlyWorker, isInitNodeOrDeleting,
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

// ignoreErrors accepts two errors. If the err is type errIgnore, it will return (err, nil) if firstIgnoreErr is nil or (firstIgnoreErr, nil).
// Otherwise, it will simply return (firstIgnoreErr, err)
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

// getControlPlaneJoinURL will return the first encountered join URL based on machine annotations for machines that are
// marked as control plane nodes
func getControlPlaneJoinURL(plan *plan.Plan) string {
	entries := collect(plan, isControlPlane)
	if len(entries) == 0 {
		return ""
	}

	return entries[0].Metadata.Annotations[rke2.JoinURLAnnotation]
}

// isUnavailable returns a boolean indicating whether the machine/node corresponding to the planEntry is available
// If the plan is not in sync or the machine is being drained, it will return true.
func isUnavailable(entry *planEntry) bool {
	return !entry.Plan.InSync || isInDrain(entry)
}

func isInDrain(entry *planEntry) bool {
	return entry.Metadata.Annotations[rke2.PreDrainAnnotation] != "" ||
		entry.Metadata.Annotations[rke2.PostDrainAnnotation] != "" ||
		entry.Metadata.Annotations[rke2.DrainAnnotation] != "" ||
		entry.Metadata.Annotations[rke2.UnCordonAnnotation] != ""
}

// planAppliedButWaitingForProbes returns a boolean indicating whether a plan was successfully able to be applied, but
// the probes have not been successful. This indicates that while the overall plan hasn't completed yet, it's
// instructions have and can now be overridden if necessary without causing thrashing.
func planAppliedButWaitingForProbes(entry *planEntry) bool {
	return entry.Plan.AppliedPlan != nil && reflect.DeepEqual(entry.Plan.Plan, *entry.Plan.AppliedPlan) && !entry.Plan.Healthy
}

func calculateConcurrency(maxUnavailable string, entries []*planEntry, exclude roleFilter) (int, int, error) {
	var (
		count, unavailable int
	)

	for _, entry := range entries {
		if !exclude(entry) {
			count++
		}
		if entry.Plan != nil && isUnavailable(entry) {
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

func (p *Planner) reconcile(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, clusterPlan *plan.Plan, required bool,
	tierName string, include, exclude roleFilter, maxUnavailable string, joinServer string, drainOptions rkev1.DrainOptions) error {
	var (
		ready, outOfSync, reconciling, nonReady, errMachines, draining, uncordoned []string
		messages                                                                   = map[string][]string{}
	)

	entries := collect(clusterPlan, include)

	concurrency, unavailable, err := calculateConcurrency(maxUnavailable, entries, exclude)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		logrus.Tracef("[planner] rkecluster %s/%s reconcile tier %s - processing machine entry: %s/%s", controlPlane.Namespace, controlPlane.Name, tierName, entry.Machine.Namespace, entry.Machine.Name)
		// we exclude here and not in collect to ensure that include matched at least one node
		if exclude(entry) {
			logrus.Tracef("[planner] rkecluster %s/%s reconcile tier %s - excluding machine entry: %s/%s", controlPlane.Namespace, controlPlane.Name, tierName, entry.Machine.Namespace, entry.Machine.Name)
			continue
		}

		// The Reconciled condition should be removed when summarizing so that the messages are not duplicated.
		summary := summary.Summarize(removeConfiguredCondition(entry.Machine))
		if summary.Error {
			errMachines = append(errMachines, entry.Machine.Name)
		}
		if summary.Transitioning {
			nonReady = append(nonReady, entry.Machine.Name)
		}

		planStatusMessage := getPlanStatusReasonMessage(entry)
		if planStatusMessage != "" {
			summary.Message = append(summary.Message, planStatusMessage)
		}
		messages[entry.Machine.Name] = summary.Message

		plan, err := p.desiredPlan(controlPlane, tokensSecret, entry, joinServer)
		if err != nil {
			return err
		}

		if entry.Plan == nil {
			logrus.Debugf("[planner] rkecluster %s/%s reconcile tier %s - setting initial plan for machine %s/%s", controlPlane.Namespace, controlPlane.Name, tierName, entry.Machine.Namespace, entry.Machine.Name)
			logrus.Tracef("[planner] rkecluster %s/%s reconcile tier %s - initial plan for machine %s/%s new: %+v", controlPlane.Namespace, controlPlane.Name, tierName, entry.Machine.Namespace, entry.Machine.Name, plan)
			outOfSync = append(outOfSync, entry.Machine.Name)
			if err := p.store.UpdatePlan(entry, plan, -1, 1); err != nil {
				return err
			}
		} else if minorPlanChangeDetected(entry.Plan.Plan, plan) {
			logrus.Debugf("[planner] rkecluster %s/%s reconcile tier %s - minor plan change detected for machine %s/%s, updating plan immediately", controlPlane.Namespace, controlPlane.Name, tierName, entry.Machine.Namespace, entry.Machine.Name)
			logrus.Tracef("[planner] rkecluster %s/%s reconcile tier %s - minor plan change for machine %s/%s old: %+v, new: %+v", controlPlane.Namespace, controlPlane.Name, tierName, entry.Machine.Namespace, entry.Machine.Name, entry.Plan.Plan, plan)
			outOfSync = append(outOfSync, entry.Machine.Name)
			if err := p.store.UpdatePlan(entry, plan, -1, 1); err != nil {
				return err
			}
		} else if !equality.Semantic.DeepEqual(entry.Plan.Plan, plan) {
			logrus.Debugf("[planner] rkecluster %s/%s reconcile tier %s - plan for machine %s/%s did not match, appending to outOfSync", controlPlane.Namespace, controlPlane.Name, tierName, entry.Machine.Namespace, entry.Machine.Name)
			outOfSync = append(outOfSync, entry.Machine.Name)
			// Conditions
			// 1. If the node is already draining then the plan is out of sync.  There is no harm in updating it if
			// the node is currently drained.
			// 2. If the plan has failed to apply. Note that the `Failed` will only be `true` if the max failure count has passed, or (if max-failures is not set) the plan has failed to apply at least once.
			// 3. concurrency == 0 which means infinite concurrency.
			// 4. unavailable < concurrency meaning we have capacity to make something unavailable
			// 5. If the plans are in sync, but we are still waiting for probes, it is safe to apply new instructions
			logrus.Debugf("[planner] rkecluster %s/%s reconcile tier %s - concurrency: %d, unavailable: %d", controlPlane.Namespace, controlPlane.Name, tierName, concurrency, unavailable)
			if isInDrain(entry) || entry.Plan.Failed || concurrency == 0 || unavailable < concurrency || planAppliedButWaitingForProbes(entry) {
				reconciling = append(reconciling, entry.Machine.Name)
				if !isUnavailable(entry) {
					unavailable++
				}
				if ok, err := p.drain(entry.Plan.AppliedPlan, plan, entry, clusterPlan, drainOptions); !ok && err != nil {
					return err
				} else if ok && err == nil {
					// Drain is done (or didn't need to be done) and there are no errors, so the plan should be updated to enact the reason the node was drained.
					logrus.Debugf("[planner] rkecluster %s/%s reconcile tier %s - major plan change for machine %s/%s", controlPlane.Namespace, controlPlane.Name, tierName, entry.Machine.Namespace, entry.Machine.Name)
					logrus.Tracef("[planner] rkecluster %s/%s reconcile tier %s - major plan change for machine %s/%s old: %+v, new: %+v", controlPlane.Namespace, controlPlane.Name, tierName, entry.Machine.Namespace, entry.Machine.Name, entry.Plan.Plan, plan)
					if err = p.store.UpdatePlan(entry, plan, -1, 1); err != nil {
						return err
					} else if entry.Metadata.Annotations[rke2.DrainDoneAnnotation] != "" {
						messages[entry.Machine.Name] = append(messages[entry.Machine.Name], "drain completed")
					} else if planStatusMessage == "" {
						messages[entry.Machine.Name] = append(messages[entry.Machine.Name], WaitingPlanStatusMessage)
					}
				} else {
					// In this case, it is true that ((ok == true && err != nil) || (ok == false && err == nil))
					// The first case indicates that there is an error trying to drain the node.
					// The second case indicates that the node is draining.
					draining = append(draining, entry.Machine.Name)
					if err != nil {
						messages[entry.Machine.Name] = append(messages[entry.Machine.Name], err.Error())
					} else {
						messages[entry.Machine.Name] = append(messages[entry.Machine.Name], "draining node")
					}
				}
			}
		} else if planStatusMessage != "" {
			outOfSync = append(outOfSync, entry.Machine.Name)
		} else if ok, err := p.undrain(entry); !ok && err != nil {
			return err
		} else if !ok || err != nil {
			// The uncordoning is happening or there was an error.
			// Either way, the planner should wait for the result and display the message on the machine.
			uncordoned = append(uncordoned, entry.Machine.Name)
			if err != nil {
				messages[entry.Machine.Name] = append(messages[entry.Machine.Name], err.Error())
			} else {
				messages[entry.Machine.Name] = append(messages[entry.Machine.Name], "waiting for uncordon to finish")
			}
		} else if !kubeletVersionUpToDate(controlPlane, entry.Machine) {
			outOfSync = append(outOfSync, entry.Machine.Name)
			messages[entry.Machine.Name] = append(messages[entry.Machine.Name], "waiting for kubelet to update")
		} else if isControlPlane(entry) && !controlPlane.Status.AgentConnected {
			// If the control plane nodes are currently being provisioned/updated, then it should be ensured that cluster-agent is connected.
			// Without the agent connected, the controllers running in Rancher, including CAPI, can't communicate with the downstream cluster.
			outOfSync = append(outOfSync, entry.Machine.Name)
			messages[entry.Machine.Name] = append(messages[entry.Machine.Name], "waiting for cluster agent to connect")
		} else {
			ready = append(ready, entry.Machine.Name)
		}
	}

	if required && len(entries) == 0 {
		return ErrWaiting("waiting for at least one " + tierName + " node")
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

	if err := p.setMachineConditionStatus(clusterPlan, reconciling, fmt.Sprintf("configuring %s node(s) ", tierName), messages); err != nil && firstError == nil {
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
		return errIgnore("failing " + tierName + " machine(s) " + atMostThree(errMachines) + detailMessage(errMachines, messages))
	}

	if len(nonReady) > 0 {
		return errIgnore("non-ready " + tierName + " machine(s) " + atMostThree(nonReady) + detailMessage(nonReady, messages))
	}

	return nil
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
func renderArgAndMount(existingArg interface{}, existingMount interface{}, runtime string, defaultSecurePort string, defaultCertDir string) ([]string, []string) {
	retArg := convertInterfaceToStringSlice(existingArg)
	retMount := convertInterfaceToStringSlice(existingMount)
	renderedCertDir := fmt.Sprintf(defaultCertDir, runtime)
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
	if runtime == rke2.RuntimeRKE2 {
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
	data := entry.Metadata.Annotations[rke2.TaintsAnnotation]
	if data != "" {
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return result, err
		}
	}

	if !isWorker(entry) {
		// k3s charts do not have correct tolerations when the master node is both controlplane and etcd
		if isEtcd(entry) && (rke2.GetRuntime(cp.Spec.KubernetesVersion) != rke2.RuntimeK3S || !isControlPlane(entry)) {
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

// generatePlanWithConfigFiles will generate a node plan with the corresponding config files for the entry in question.
// Notably, it will discard the existing nodePlan in the given entry.
func (p *Planner) generatePlanWithConfigFiles(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, entry *planEntry, joinServer string) (nodePlan plan.NodePlan, config map[string]interface{}, err error) {
	if !controlPlane.Spec.UnmanagedConfig {
		nodePlan, err = p.commonNodePlan(controlPlane, plan.NodePlan{})
		if err != nil {
			return nodePlan, map[string]interface{}{}, err
		}

		nodePlan, config, err = p.addConfigFile(nodePlan, controlPlane, entry, tokensSecret, isInitNode(entry), joinServer)
		if err != nil {
			return nodePlan, config, err
		}

		nodePlan, err = p.addManifests(nodePlan, controlPlane, entry)
		if err != nil {
			return nodePlan, config, err
		}

		nodePlan, err = p.addChartConfigs(nodePlan, controlPlane, entry)
		if err != nil {
			return nodePlan, config, err
		}

		nodePlan, err = addOtherFiles(nodePlan, controlPlane, entry)
		if err != nil {
			return nodePlan, config, err
		}
		return
	}
	return plan.NodePlan{}, map[string]interface{}{}, nil
}

func (p *Planner) ensureInstalledPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, entry *planEntry, joinServer string) (nodePlan plan.NodePlan, err error) {
	nodePlan, _, err = p.generatePlanWithConfigFiles(controlPlane, tokensSecret, entry, joinServer)
	if err != nil {
		return
	}

	// Add instruction last because it hashes config content
	nodePlan, err = p.addInstallInstructionWithRestartStamp(nodePlan, controlPlane, entry)
	if err != nil {
		return nodePlan, err
	}

	return nodePlan, nil
}

func (p *Planner) desiredPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, entry *planEntry, joinServer string) (nodePlan plan.NodePlan, err error) {
	nodePlan, config, err := p.generatePlanWithConfigFiles(controlPlane, tokensSecret, entry, joinServer)
	if err != nil {
		return nodePlan, err
	}

	nodePlan, err = p.addProbes(nodePlan, controlPlane, entry, config)
	if err != nil {
		return nodePlan, err
	}

	// Add instruction last because it hashes config content
	nodePlan, err = p.addInstallInstructionWithRestartStamp(nodePlan, controlPlane, entry)
	if err != nil {
		return nodePlan, err
	}

	if isInitNode(entry) && IsOnlyEtcd(entry) {
		nodePlan, err = p.addInitNodePeriodicInstruction(nodePlan, controlPlane)
		if err != nil {
			return nodePlan, err
		}
	}

	if isEtcd(entry) {
		nodePlan, err = p.addEtcdSnapshotListLocalPeriodicInstruction(nodePlan, controlPlane)
		if err != nil {
			return nodePlan, err
		}
		if controlPlane != nil && controlPlane.Spec.ETCD != nil && S3Enabled(controlPlane.Spec.ETCD.S3) && isInitNode(entry) {
			nodePlan, err = p.addEtcdSnapshotListS3PeriodicInstruction(nodePlan, controlPlane)
			if err != nil {
				return nodePlan, err
			}
		}
	}
	return nodePlan, nil
}

func getInstallerImage(controlPlane *rkev1.RKEControlPlane) string {
	runtime := rke2.GetRuntime(controlPlane.Spec.KubernetesVersion)
	installerImage := settings.SystemAgentInstallerImage.Get()
	installerImage = installerImage + runtime + ":" + strings.ReplaceAll(controlPlane.Spec.KubernetesVersion, "+", "-")
	return image.ResolveWithControlPlane(installerImage, controlPlane)
}

func isEtcd(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Labels[rke2.EtcdRoleLabel] == "true"
}

func isInitNode(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Labels[rke2.InitNodeLabel] == "true"
}

func isInitNodeOrDeleting(entry *planEntry) bool {
	return isInitNode(entry) || isDeleting(entry)
}

func IsEtcdOnlyInitNode(entry *planEntry) bool {
	return isInitNode(entry) && IsOnlyEtcd(entry)
}

func isNotInitNodeOrIsDeleting(entry *planEntry) bool {
	return !isInitNode(entry) || isDeleting(entry)
}

func isDeleting(entry *planEntry) bool {
	return entry.Machine.DeletionTimestamp != nil
}

// isFailed returns true if the provided entry machine.status.phase is failed
func isFailed(entry *planEntry) bool {
	return entry.Machine.Status.Phase == string(capi.MachinePhaseFailed)
}

// canBeInitNode returns true if the provided entry is an etcd node, is not deleting, is not failed, and has its infrastructure ready
// We should wait for the infrastructure condition to be marked as ready because we need the IP address(es) set prior to bootstrapping the node.
func canBeInitNode(entry *planEntry) bool {
	return isEtcd(entry) && !isDeleting(entry) && !isFailed(entry) && rke2.InfrastructureReady.IsTrue(entry.Machine)
}

func isControlPlane(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Labels[rke2.ControlPlaneRoleLabel] == "true"
}

func isControlPlaneAndNotInitNode(entry *planEntry) bool {
	return isControlPlane(entry) && !isInitNode(entry)
}

func isControlPlaneEtcd(entry *planEntry) bool {
	return isControlPlane(entry) || isEtcd(entry)
}

func IsOnlyEtcd(entry *planEntry) bool {
	return isEtcd(entry) && !isControlPlane(entry)
}

func isOnlyControlPlane(entry *planEntry) bool {
	return !isEtcd(entry) && isControlPlane(entry)
}

func isWorker(entry *planEntry) bool {
	return entry.Metadata != nil && entry.Metadata.Labels[rke2.WorkerRoleLabel] == "true"
}

func noRole(entry *planEntry) bool {
	return !isEtcd(entry) && !isControlPlane(entry) && !isWorker(entry)
}

func anyRole(entry *planEntry) bool {
	return !noRole(entry)
}

func anyRoleWithoutWindows(entry *planEntry) bool {
	return !noRole(entry) && notWindows(entry)
}

func isOnlyWorker(entry *planEntry) bool {
	return !isEtcd(entry) && !isControlPlane(entry) && isWorker(entry)
}

func notWindows(entry *planEntry) bool {
	return entry.Machine.Status.NodeInfo.OperatingSystem != windows
}

type planEntry struct {
	Machine  *capi.Machine
	Plan     *plan.Node
	Metadata *plan.Metadata
}

func collect(plan *plan.Plan, include roleFilter) (result []*planEntry) {
	for machineName, machine := range plan.Machines {
		entry := &planEntry{
			Machine:  machine,
			Plan:     plan.Nodes[machineName],
			Metadata: plan.Metadata[machineName],
		}
		if !include(entry) {
			continue
		}
		result = append(result, entry)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Machine.Name < result[j].Machine.Name
	})

	return result
}

// generateSecrets generates the server/agent tokens for a v2prov cluster
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
						APIVersion: rke2.RKEAPIVersion,
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

func (p *Planner) enqueueAndSkip(cp *rkev1.RKEControlPlane) error {
	p.rkeControlPlanes.EnqueueAfter(cp.Namespace, cp.Name, 10*time.Second)
	return generic.ErrSkip
}

// enqueueIfErrWaiting will enqueue the control plan if err is ErrWaiting, otherwise err is returned.
// Some plan store functions as well as internal functions can return ErrWaiting, for instance when the plan has been
// applied, but output is not yet available for periodic status.
func (p *Planner) enqueueIfErrWaiting(cp *rkev1.RKEControlPlane, err error) error {
	if err != nil {
		if isErrWaiting(err) {
			logrus.Tracef("Enqueuing [%s] because of ErrWaiting: %s", cp.Spec.ClusterName, err.Error())
			return p.enqueueAndSkip(cp)
		}
		return err
	}
	return nil
}

func isErrWaiting(err error) bool {
	var errWaiting ErrWaiting
	return errors.As(err, &errWaiting)
}
