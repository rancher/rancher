package planner

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/moby/locker"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	ranchercontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/randomtoken"
	"github.com/rancher/wrangler/pkg/summary"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
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

func (p *Planner) process(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	logrus.Debugf("[planner] %s/%s: attempting to lock %s for processing", cp.Namespace, cp.Name, string(cp.UID))
	p.locker.Lock(string(cp.UID))
	defer func(namespace, name, uid string) {
		logrus.Debugf("[planner] %s/%s: unlocking %s", namespace, name, uid)
		_ = p.locker.Unlock(uid)
	}(cp.Namespace, cp.Name, string(cp.UID))

	releaseData := rke2.GetKDMReleaseData(p.ctx, cp)
	if releaseData == nil {
		return status, ErrWaitingf("%s/%s: releaseData nil for version %s", cp.Namespace, cp.Name, cp.Spec.KubernetesVersion)
	}

	capiCluster, err := rke2.FindOwnerCAPICluster(cp, p.capiClusters)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return status, ErrWaiting("CAPI cluster does not exist")
		}
		return status, err
	}

	if capiCluster == nil {
		return status, ErrWaiting("CAPI cluster does not exist")
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
		return status, ErrWaitingf("rkecluster %s/%s: waiting for infrastructure ready", cp.Namespace, cp.Name)
	}

	plan, err := p.store.Load(capiCluster, cp)
	if err != nil {
		return status, err
	}

	clusterSecretTokens, err := p.generateSecrets(cp)
	if err != nil {
		return status, err
	}

	var (
		firstIgnoreError error
		joinServer       string
	)

	if status, err = p.createEtcdSnapshot(cp, status, clusterSecretTokens, plan); err != nil {
		return status, err
	}

	if status, err = p.restoreEtcdSnapshot(cp, status, clusterSecretTokens, plan); err != nil {
		return status, err
	}

	if status, err = p.rotateCertificates(cp, status, plan); err != nil {
		return status, err
	}

	if status, err = p.rotateEncryptionKeys(cp, status, releaseData, plan); err != nil {
		return status, err
	}

	// pausing the control plane only affects machine reconciliation: etcd snapshot/restore, encryption key & cert
	// rotation are not interruptable processes, and therefore must always be completed when requested
	if capiannotations.IsPaused(capiCluster, cp) {
		return status, ErrWaitingf("rkecluster %s/%s: CAPI cluster or RKEControlPlane is paused", cp.Namespace, cp.Name)
	}

	// on the first run through, electInitNode will return a `generic.ErrSkip` as it is attempting to wait for the cache to catch up.
	joinServer, err = p.electInitNode(cp, plan)
	if err != nil {
		return status, err
	}

	// select all etcd and then filter to just initNodes so that unavailable count is correct
	err = p.reconcile(cp, clusterSecretTokens, plan, true, bootstrapTier, isEtcd, isNotInitNodeOrIsDeleting,
		"1", "",
		cp.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return status, err
	}

	if joinServer == "" {
		_, joinServer, _, err = p.findInitNode(cp, plan)
		if err != nil {
			return status, err
		} else if joinServer == "" && firstIgnoreError != nil {
			return status, ErrWaiting(firstIgnoreError.Error() + " and join url to be available on bootstrap node")
		} else if joinServer == "" {
			return status, ErrWaiting("waiting for join url to be available on bootstrap node")
		}
	}

	err = p.reconcile(cp, clusterSecretTokens, plan, true, etcdTier, isEtcd, isInitNodeOrDeleting,
		"1", joinServer,
		cp.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return status, err
	}

	err = p.reconcile(cp, clusterSecretTokens, plan, true, controlPlaneTier, isControlPlane, isInitNodeOrDeleting,
		cp.Spec.UpgradeStrategy.ControlPlaneConcurrency, joinServer,
		cp.Spec.UpgradeStrategy.ControlPlaneDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return status, err
	}

	joinServer = getControlPlaneJoinURL(plan)
	if joinServer == "" {
		return status, ErrWaiting("waiting for control plane to be available")
	}

	if status.Initialized != true {
		status.Initialized = true
		return status, ErrWaiting("marking control plane as initialized")
	}

	err = p.reconcile(cp, clusterSecretTokens, plan, false, workerTier, isOnlyWorker, isInitNodeOrDeleting,
		cp.Spec.UpgradeStrategy.WorkerConcurrency, joinServer,
		cp.Spec.UpgradeStrategy.WorkerDrainOptions)
	firstIgnoreError, err = ignoreErrors(firstIgnoreError, err)
	if err != nil {
		return status, err
	}

	if firstIgnoreError != nil {
		return status, ErrWaiting(firstIgnoreError.Error())
	}

	return status, nil
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
		summary := summary.Summarize(removeReconciledCondition(entry.Machine))
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
func (p *Planner) generateSecrets(controlPlane *rkev1.RKEControlPlane) (plan.Secret, error) {
	_, secret, err := p.ensureRKEStateSecret(controlPlane)
	if err != nil {
		return secret, err
	}

	return secret, nil
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

func (p *Planner) pauseCAPICluster(cp *rkev1.RKEControlPlane, pause bool) error {
	if cp == nil {
		return fmt.Errorf("cannot toggle health checks for nil controlplane")
	}
	cluster, err := rke2.FindOwnerCAPICluster(cp, p.capiClusters)
	if err != nil {
		return err
	}
	if cluster == nil {
		return fmt.Errorf("CAPI cluster does not exist for %s/%s", cp.Namespace, cp.Name)
	}
	if cluster.Spec.Paused == pause {
		return nil
	}
	cluster.Spec.Paused = pause
	_, err = p.capiClient.Update(cluster)
	return err
}
