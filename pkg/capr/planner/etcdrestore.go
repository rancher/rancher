package planner

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/capr/managesystemagent"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	etcdRestoreInstallRoot = "/var/lib/rancher"
	etcdRestoreBinPrefix   = "capr_etcd_restore/bin"

	etcdRestorePostRestoreWaitForPodListCleanupPath   = "wait_for_pod_list.sh"
	etcdRestorePostRestoreWaitForPodListCleanupScript = `
#!/bin/sh

i=0

while [ $i -lt 30 ]; do
	$@ &>/dev/null
	if [ $? -eq 0 ]; then
		exit 0
	fi
	sleep 10
	i=$((i + 1))
done
exit 1
`
)

const ETCDRestoreMessage = "etcd restore"

// setEtcdSnapshotRestoreState sets the restore schema and phase to the given restore and phase and returns an errWaiting if a change was made. Notably this function does not persist the change.
func (p *Planner) setEtcdSnapshotRestoreState(status rkev1.RKEControlPlaneStatus, restore *rkev1.ETCDSnapshotRestore, phase rkev1.ETCDSnapshotPhase) (rkev1.RKEControlPlaneStatus, error) {
	if !equality.Semantic.DeepEqual(status.ETCDSnapshotRestore, restore) || status.ETCDSnapshotRestorePhase != phase {
		status.ETCDSnapshotRestore = restore
		status.ETCDSnapshotRestorePhase = phase
		return status, errWaiting("refreshing etcd restore state")
	}
	return status, nil
}

// resetEtcdSnapshotRestoreState will unset the restore field and phase
func (p *Planner) resetEtcdSnapshotRestoreState(status rkev1.RKEControlPlaneStatus) (rkev1.RKEControlPlaneStatus, error) {
	if status.ETCDSnapshotRestore == nil && status.ETCDSnapshotRestorePhase == "" {
		return status, nil
	}
	return p.setEtcdSnapshotRestoreState(status, nil, "")
}

// startOrRestartEtcdSnapshotRestore sets the started phase
func (p *Planner) startOrRestartEtcdSnapshotRestore(status rkev1.RKEControlPlaneStatus, restore *rkev1.ETCDSnapshotRestore) (rkev1.RKEControlPlaneStatus, error) {
	if status.ETCDSnapshotRestore == nil || !equality.Semantic.DeepEqual(restore, status.ETCDSnapshotRestore) {
		return p.setEtcdSnapshotRestoreState(status, restore, rkev1.ETCDSnapshotPhaseStarted)
	}
	return status, nil
}

// runEtcdSnapshotRestorePlan runs the snapshot restoration plan by electing an init node (or designating the init node
// that is specified on the snapshot), and renders/delivers the etcd restoration plan to that node.
func (p *Planner) runEtcdSnapshotRestorePlan(controlPlane *rkev1.RKEControlPlane, snapshot *rkev1.ETCDSnapshot, snapshotName string, tokensSecret plan.Secret, clusterPlan *plan.Plan) error {
	var joinServer string
	var err error

	joinServer, err = p.runEtcdRestoreInitNodeElection(controlPlane, snapshot, clusterPlan)
	if err != nil {
		return err
	}

	servers := collect(clusterPlan, isInitNode)

	if len(servers) != 1 {
		return fmt.Errorf("more than one init node existed, cannot run etcd snapshot restore")
	}

	if err := p.pauseCAPICluster(controlPlane, true); err != nil {
		return err
	}

	restorePlan, joinedServer, err := p.generateEtcdSnapshotRestorePlan(controlPlane, snapshot, snapshotName, tokensSecret, servers[0], joinServer)
	if err != nil {
		return err
	}
	return assignAndCheckPlan(p.store, ETCDRestoreMessage, servers[0], restorePlan, joinedServer, 1, 1)
}

func (p *Planner) runEtcdSnapshotPostRestoreCleanupPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, clusterPlan *plan.Plan) error {
	initNodes := collect(clusterPlan, isInitNode)
	if len(initNodes) != 1 {
		return fmt.Errorf("multiple init nodes found")
	}
	initNode := initNodes[0]

	initNodePlan, _, err := p.desiredPlan(controlPlane, tokensSecret, initNode, "")
	if err != nil {
		return err
	}

	cleanupScriptFile, cleanupInstructions := p.generateEtcdRestorePodCleanupFilesAndInstruction(controlPlane)

	// If the init node is a controlplane node, deliver the desired plan + pod cleanup instruction
	if isControlPlane(initNode) {
		initNodePlan.Files = append(initNodePlan.Files, cleanupScriptFile)
		initNodePlan.Instructions = append(initNodePlan.Instructions, cleanupInstructions...)
		return assignAndCheckPlan(p.store, ETCDRestoreMessage, initNode, initNodePlan, "", 5, 5)
	}

	if err := assignAndCheckPlan(p.store, ETCDRestoreMessage, initNode, initNodePlan, "", 5, 5); err != nil {
		return err
	}

	_, joinServer, _, err := p.findInitNode(controlPlane, clusterPlan)
	if joinServer == "" {
		return errWaitingf("waiting for join server")
	}
	if err != nil {
		return err
	}

	controlPlaneEntries := collect(clusterPlan, roleAnd(isControlPlane, roleNot(isDeleting)))
	if len(controlPlaneEntries) == 0 {
		return fmt.Errorf("no suitable controlplane entries found for post restore cleanup during etcd restoration")
	}

	controlPlaneEntry := controlPlaneEntries[0]

	firstControlPlanePlan, joinedServer, err := p.desiredPlan(controlPlane, tokensSecret, controlPlaneEntry, joinServer)
	if err != nil {
		return err
	}
	firstControlPlanePlan.Files = append(firstControlPlanePlan.Files, cleanupScriptFile)
	firstControlPlanePlan.Instructions = append(firstControlPlanePlan.Instructions, cleanupInstructions...)
	return assignAndCheckPlan(p.store, ETCDRestoreMessage, controlPlaneEntry, firstControlPlanePlan, joinedServer, 5, 5)
}

// generateEtcdSnapshotRestorePlan returns a node plan that contains instructions to stop etcd, remove the tombstone file (if one exists), then restore etcd in that order.
func (p *Planner) generateEtcdSnapshotRestorePlan(controlPlane *rkev1.RKEControlPlane, snapshot *rkev1.ETCDSnapshot, snapshotName string, tokensSecret plan.Secret, entry *planEntry, joinServer string) (plan.NodePlan, string, error) {
	if controlPlane.Spec.ETCDSnapshotRestore == nil {
		return plan.NodePlan{}, "", fmt.Errorf("ETCD Snapshot restore was not defined")
	}

	// Notably, if we are generating a restore plan for an S3 snapshot, we will render S3 arguments, environment variables, and files from the snapshot metadata.
	nodePlan, _, joinedServer, err := p.generatePlanWithConfigFiles(controlPlane, tokensSecret, entry, joinServer, false)
	if err != nil {
		return nodePlan, joinedServer, err
	}

	args := []string{
		"server",
		"--cluster-reset",
		"--etcd-arg=advertise-client-urls=https://127.0.0.1:2379", // this is a workaround for: https://github.com/rancher/rke2/issues/4052 and can likely remain indefinitely (unless IPv6-only becomes a requirement)
	}

	var env []string

	if snapshot == nil {
		// If the snapshot is nil, then we will assume the passed in snapshot name is a local snapshot.
		args = append(args, fmt.Sprintf("--cluster-reset-restore-path=db/snapshots/%s", snapshotName), "--etcd-s3=false")
	} else if snapshot.SnapshotFile.S3 == nil {
		args = append(args, fmt.Sprintf("--cluster-reset-restore-path=db/snapshots/%s", snapshot.SnapshotFile.Name), "--etcd-s3=false")
	} else {
		args = append(args, fmt.Sprintf("--cluster-reset-restore-path=%s", snapshot.SnapshotFile.Name))
		s3, s3Env, s3Files, err := p.etcdS3Args.ToArgs(snapshot.SnapshotFile.S3, controlPlane, "etcd-", true)
		if err != nil {
			return plan.NodePlan{}, "", err
		}
		args = append(args, s3...)
		env = s3Env
		nodePlan.Files = append(nodePlan.Files, s3Files...)
	}

	// This is likely redundant but can make sense in the event that there is an external watchdog.
	stopPlan, joinedServer, err := p.generateStopServiceAndKillAllPlan(controlPlane, tokensSecret, entry, joinServer)
	if err != nil {
		return plan.NodePlan{}, joinedServer, err
	}

	// make sure to install the desired version before performing restore
	stopPlan.Instructions = append(stopPlan.Instructions, p.generateInstallInstructionWithSkipStart(controlPlane, entry))

	planInstructions := append(stopPlan.Instructions,
		plan.OneTimeInstruction{
			Name:    "remove-etcd-db-dir",
			Command: "rm",
			Args: []string{
				"-rf",
				fmt.Sprintf("/var/lib/rancher/%s/server/db/etcd", capr.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion)),
			}})

	nodePlan.Instructions = append(planInstructions, plan.OneTimeInstruction{
		Name:    "restore",
		Args:    args,
		Env:     env,
		Command: capr.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
	})

	return nodePlan, joinedServer, nil
}

func (p *Planner) generateStopServiceAndKillAllPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, server *planEntry, joinServer string) (plan.NodePlan, string, error) {
	nodePlan, _, joinedServer, err := p.generatePlanWithConfigFiles(controlPlane, tokensSecret, server, joinServer, true)
	if err != nil {
		return nodePlan, joinedServer, err
	}
	runtime := capr.GetRuntime(controlPlane.Spec.KubernetesVersion)
	nodePlan.Instructions = append(nodePlan.Instructions,
		generateKillAllInstruction(runtime))
	if runtime == capr.RuntimeRKE2 {
		if generated, instruction := generateManifestRemovalInstruction(runtime, server); generated {
			nodePlan.Instructions = append(nodePlan.Instructions, instruction)
		}
	}
	return nodePlan, joinedServer, nil
}

// generateManifestRemovalInstruction generates a rm -rf command for the manifests of a server. This was created in response to https://github.com/rancher/rancher/issues/41174
func generateManifestRemovalInstruction(runtime string, entry *planEntry) (bool, plan.OneTimeInstruction) {
	if runtime == "" || entry == nil || roleNot(roleOr(isEtcd, isControlPlane))(entry) {
		return false, plan.OneTimeInstruction{}
	}
	return true, plan.OneTimeInstruction{
		Name:    "remove server manifests",
		Command: "rm",
		Args: []string{
			"-rf",
			fmt.Sprintf("/var/lib/rancher/%s/server/manifests/%s-*.yaml", runtime, runtime),
		},
	}
}

func generateKillAllInstruction(runtime string) plan.OneTimeInstruction {
	killAllScript := runtime + "-killall.sh"
	return plan.OneTimeInstruction{
		Name:    "shutdown",
		Command: "/bin/sh",
		Args: []string{
			"-c",
			fmt.Sprintf("if [ -z $(command -v %s) ] && [ -z $(command -v %s) ]; then echo %s does not appear to be installed; exit 0; else %s; fi", runtime, killAllScript, runtime, killAllScript),
		},
	}
}

func generateCreateEtcdTombstoneInstruction(controlPlane *rkev1.RKEControlPlane) plan.OneTimeInstruction {
	return plan.OneTimeInstruction{
		Name:    "create-etcd-tombstone",
		Command: "touch",
		Args: []string{
			fmt.Sprintf("/var/lib/rancher/%s/server/db/etcd/tombstone", capr.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion)),
		},
	}
}

func etcdRestoreScriptPath(controlPlane *rkev1.RKEControlPlane, file string) string {
	return fmt.Sprintf("%s/%s/%s/%s", etcdRestoreInstallRoot, capr.GetRuntime(controlPlane.Spec.KubernetesVersion), etcdRestoreBinPrefix, file)
}

// generateEtcdRestorePodCleanupFilesAndInstruction generates a file that contains a script that checks API server health and a slice of instructions that cleans up system pods on etcd restore.
func (p *Planner) generateEtcdRestorePodCleanupFilesAndInstruction(controlPlane *rkev1.RKEControlPlane) (plan.File, []plan.OneTimeInstruction) {
	runtime := capr.GetRuntime(controlPlane.Spec.KubernetesVersion)

	kubectl := "/usr/local/bin/kubectl"
	kubeconfig := "/etc/rancher/k3s/k3s.yaml"

	if runtime == capr.RuntimeRKE2 {
		kubectl = "/var/lib/rancher/rke2/bin/kubectl"
		kubeconfig = "/etc/rancher/rke2/rke2.yaml"
	}

	instructions := []plan.OneTimeInstruction{
		{
			Name:    "post-restore-cleanup-pods-wait-for-podlist",
			Command: "/bin/sh",
			Args: []string{
				"-x",
				etcdRestoreScriptPath(controlPlane, etcdRestorePostRestoreWaitForPodListCleanupPath),
				kubectl,
				"--kubeconfig",
				kubeconfig,
				"get",
				"pods",
				"--all-namespaces",
			},
		},
	}

	// These are the pod selectors that are common between K3s/RKE2 (CoreDNS/KubeDNS)
	podSelectors := []string{
		"kube-system:k8s-app=kube-dns",
		"kube-system:k8s-app=kube-dns-autoscaler",
	}

	// RKE2 charts come from: https://github.com/rancher/rke2/blob/253af9ca3115de691f5fdb8ed8dcb284287b1856/Dockerfile#L109-L123 and are deployed into `kube-system` as the Helm {{ .Release.Namespace }} by default
	if runtime == capr.RuntimeRKE2 {
		podSelectors = append(podSelectors,
			"kube-system:app=rke2-metrics-server",                   // rke2-metrics-server is deployed into `{{ .Release.Namespace }}` which is kube-system in RKE2: https://github.com/rancher/rke2-charts/blob/237251fccd793df825de0f27804ca7b6ad6e2981/charts/rke2-metrics-server/rke2-metrics-server/2.11.100/templates/metrics-server-deployment.yaml#L5
			"tigera-operator:k8s-app=tigera-operator",               // https://github.com/rancher/rke2-charts/blob/237251fccd793df825de0f27804ca7b6ad6e2981/charts/rke2-calico/rke2-calico/v3.25.002/templates/tigera-operator/00-namespace-tigera-operator.yaml#L4
			"calico-system:k8s-app=calico-node",                     // Managed by tigera-operator https://github.com/tigera/operator/blob/08cdc5df85fda2ebe69ffafded1953744409c554/pkg/common/common.go#L20
			"calico-system:k8s-app=calico-kube-controllers",         // Managed by tigera-operator https://github.com/tigera/operator/blob/08cdc5df85fda2ebe69ffafded1953744409c554/pkg/common/common.go#L21
			"calico-system:k8s-app=calico-typha",                    // Managed by tigera-operator https://github.com/tigera/operator/blob/08cdc5df85fda2ebe69ffafded1953744409c554/pkg/common/common.go#L19
			"kube-system:k8s-app=canal",                             // Canal is hardcode deployed into `kube-system` https://github.com/rancher/rke2-charts/blob/237251fccd793df825de0f27804ca7b6ad6e2981/charts/rke2-canal/rke2-canal/v3.25.0-build2023020902/templates/daemonset.yaml#L10
			"kube-system:k8s-app=cilium",                            // Cilium agent is deployed into `{{ .Release.Namespace }}` which is kube-system in RKE2: https://github.com/rancher/rke2-charts/blob/237251fccd793df825de0f27804ca7b6ad6e2981/charts/rke2-cilium/rke2-cilium/1.13.200/templates/cilium-agent/daemonset.yaml#L26
			"kube-system:app=rke2-multus",                           // Multus is deployed into `{{ .Release.Namespace }}` which is kube-system in RKE2: https://github.com/rancher/rke2-charts/blob/237251fccd793df825de0f27804ca7b6ad6e2981/charts/rke2-multus/rke2-multus/v3.9.3-build2023010902/templates/daemonSet.yaml#L20
			"kube-system:app.kubernetes.io/name=rke2-ingress-nginx", // rke2-ingress-nginx is deployed into `{{ .Release.Namespace }}` which is in kube-system in RKE2: https://github.com/rancher/rke2-charts/blob/237251fccd793df825de0f27804ca7b6ad6e2981/charts/rke2-ingress-nginx/rke2-ingress-nginx/4.5.201/templates/controller-daemonset.yaml#L13
		)
	}

	if p.retrievalFunctions.SystemPodLabelSelectors != nil {
		podSelectors = append(podSelectors, p.retrievalFunctions.SystemPodLabelSelectors(controlPlane)...)
	}

	for i, podSelector := range podSelectors {
		if namespace, labelSelector, usable := strings.Cut(podSelector, ":"); usable {
			instructions = append(instructions, plan.OneTimeInstruction{
				Name:    fmt.Sprintf("post-restore-cleanup-pods-%d", i),
				Command: kubectl,
				Args: []string{
					"--kubeconfig",
					kubeconfig,
					"delete",
					"pods",
					"-n",
					namespace,
					"-l",
					labelSelector,
					"--wait=false",
				},
			})
		}
	}

	return plan.File{
		Content: base64.StdEncoding.EncodeToString([]byte(etcdRestorePostRestoreWaitForPodListCleanupScript)),
		Path:    etcdRestoreScriptPath(controlPlane, etcdRestorePostRestoreWaitForPodListCleanupPath),
		Dynamic: true,
	}, instructions
}

func generateRemoveTLSAndCredDirInstructions(controlPlane *rkev1.RKEControlPlane) []plan.OneTimeInstruction {
	return []plan.OneTimeInstruction{
		{
			Name:    "remove-tls-directory",
			Command: "rm",
			Args: []string{
				"-rf",
				fmt.Sprintf("/var/lib/rancher/%s/server/tls", capr.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion)),
			},
		},
		{
			Name:    "remove-cred-directory",
			Command: "rm",
			Args: []string{
				"-rf",
				fmt.Sprintf("/var/lib/rancher/%s/server/cred", capr.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion)),
			},
		},
	}
}

// runEtcdRestoreInitNodeElection runs an election for an init node. Notably, it accepts a nil snapshot, and will
func (p *Planner) runEtcdRestoreInitNodeElection(controlPlane *rkev1.RKEControlPlane, snapshot *rkev1.ETCDSnapshot, clusterPlan *plan.Plan) (string, error) {
	if snapshot != nil { // If the snapshot CR is not nil, then find an init node.
		if snapshot.SnapshotFile.S3 == nil {
			// If the snapshot is not an S3 snapshot, then designate the init node by machine ID defined.
			if id, ok := snapshot.Labels[capr.MachineIDLabel]; ok {
				logrus.Infof("[planner] rkecluster %s/%s: designating init node with machine ID: %s for local snapshot %s/%s restoration", controlPlane.Namespace, controlPlane.Name, id, snapshot.Namespace, snapshot.Name)
				return p.designateInitNodeByMachineID(controlPlane, clusterPlan, id)
			}
			return "", fmt.Errorf("unable to designate machine as label %s on snapshot %s/%s did not exist", capr.MachineIDLabel, snapshot.Namespace, snapshot.Name)
		}
		logrus.Infof("[planner] rkecluster %s/%s: electing init node for S3 snapshot %s/%s restoration", controlPlane.Namespace, controlPlane.Name, snapshot.Namespace, snapshot.Name)
		return p.electInitNode(controlPlane, clusterPlan)
	}
	// make sure that we only have one suitable init node, and elect it.
	if len(collect(clusterPlan, canBeInitNode)) != 1 {
		return "", fmt.Errorf("more than one init node existed and no corresponding etcd snapshot CR found, no assumption can be made for the machine that contains the snapshot")
	}
	logrus.Infof("[planner] rkecluster %s/%s: electing init node for local snapshot with no associated CR", controlPlane.Namespace, controlPlane.Name)
	return p.electInitNode(controlPlane, clusterPlan)
}

// runEtcdRestoreServiceStop generates service stop plans for every non-windows node in the cluster and
// assigns/checks the plans to ensure they were successful
func (p *Planner) runEtcdRestoreServiceStop(controlPlane *rkev1.RKEControlPlane, snapshot *rkev1.ETCDSnapshot, tokensSecret plan.Secret, clusterPlan *plan.Plan) error {
	var joinServer string
	var err error

	joinServer, err = p.runEtcdRestoreInitNodeElection(controlPlane, snapshot, clusterPlan)
	if err != nil {
		return err
	}

	deletingEtcdNodes, err := p.forceDeleteAllDeletingEtcdMachines(controlPlane, clusterPlan)
	if err != nil {
		return err
	} else if deletingEtcdNodes != 0 {
		return errWaitingf("waiting for %d etcd machines to delete", deletingEtcdNodes)
	}

	servers := collect(clusterPlan, anyRoleWithoutWindows)
	updated := false
	for _, server := range servers {
		if server.Plan == nil {
			continue
		}
		stopPlan, joinedServer, err := p.generateStopServiceAndKillAllPlan(controlPlane, tokensSecret, server, joinServer)
		if err != nil {
			return err
		}
		if isEtcd(server) {
			stopPlan.Instructions = append(stopPlan.Instructions, generateCreateEtcdTombstoneInstruction(controlPlane))
		}
		if roleOr(isEtcd, isControlPlane)(server) {
			stopPlan.Instructions = append(stopPlan.Instructions, generateRemoveTLSAndCredDirInstructions(controlPlane)...)
		}
		if !equality.Semantic.DeepEqual(server.Plan.Plan, stopPlan) {
			if err := p.store.UpdatePlan(server, stopPlan, joinedServer, 0, 0); err != nil {
				return err
			}
			updated = true
		}
	}

	// If any of the node plans were updated, return an errWaiting message for shutting down control plane and etcd
	if updated {
		return errWaiting("stopping " + capr.GetRuntime(controlPlane.Spec.KubernetesVersion) + " services on control plane and etcd machines/nodes")
	}

	for _, server := range servers {
		if server.Plan == nil {
			continue
		}
		if !server.Plan.InSync {
			if server.Machine.Status.NodeRef == nil {
				return errWaiting(fmt.Sprintf("waiting to stop %s services on machine [%s]", capr.GetRuntime(controlPlane.Spec.KubernetesVersion), server.Machine.Name))
			}
			return errWaiting(fmt.Sprintf("waiting to stop %s services on node [%s]", capr.GetRuntime(controlPlane.Spec.KubernetesVersion), server.Machine.Status.NodeRef.Name))
		}
	}

	if len(collect(clusterPlan, roleAnd(isEtcd, roleNot(isDeleting)))) == 0 {
		return errWaiting("waiting for suitable etcd nodes for etcd restore continuation")
	}
	return nil
}

// retrieveEtcdSnapshot attempts to retrieve the etcdsnapshot CR that corresponds to the etcd snapshot restore name specified on the controlplane.
func (p *Planner) retrieveEtcdSnapshot(controlPlane *rkev1.RKEControlPlane) (*rkev1.ETCDSnapshot, error) {
	if controlPlane == nil {
		return nil, fmt.Errorf("controlplane was nil")
	}
	if controlPlane.Spec.ETCDSnapshotRestore == nil {
		return nil, fmt.Errorf("etcdsnapshotrestore spec was nil")
	}
	if controlPlane.Spec.ClusterName == "" {
		return nil, fmt.Errorf("cluster name on rkecontrolplane %s/%s was blank", controlPlane.Namespace, controlPlane.Name)
	}
	snapshot, err := p.etcdSnapshotCache.Get(controlPlane.Namespace, controlPlane.Spec.ETCDSnapshotRestore.Name)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	return snapshot, err
}

// forceDeleteAllDeletingEtcdMachines collects the etcd machines that are deleting for the given plan and force-deletes them.
// This is helpful for the case where an etcd restore operation is happening on a cluster with "stuck" deleting etcd machines (quorum loss).
func (p *Planner) forceDeleteAllDeletingEtcdMachines(cp *rkev1.RKEControlPlane, plan *plan.Plan) (int, error) {
	etcdDeleting := collect(plan, roleAnd(isEtcd, isDeleting))
	for _, deletingEtcdNode := range etcdDeleting {
		if deletingEtcdNode.Machine == nil {
			logrus.Warnf("[planner] rkecluster %s/%s: did not find CAPI machine for entry when deleting etcd nodes", cp.Namespace, cp.Name)
			continue
		}
		if deletingEtcdNode.Machine.Spec.Bootstrap.ConfigRef == nil {
			logrus.Warnf("[planner] rkecluster %s/%s: did not find a corresponding CAPI machine for %s/%s", cp.Namespace, cp.Name, deletingEtcdNode.Machine.Namespace, deletingEtcdNode.Machine.Name)
			continue
		}
		if !strings.Contains(deletingEtcdNode.Machine.Spec.Bootstrap.ConfigRef.APIVersion, "rke.cattle.io") {
			logrus.Warnf("[planner] rkecluster %s/%s: CAPI machine %s/%s had a bootstrap ref with an unexpected API version: %s", cp.Namespace, cp.Name, deletingEtcdNode.Machine.Namespace, deletingEtcdNode.Machine.Name, deletingEtcdNode.Machine.Spec.Bootstrap.ConfigRef.APIVersion)
			continue
		}
		logrus.Infof("[planner] rkecluster %s/%s: force deleting etcd machine %s/%s as cluster was not sane and machine was deleting", cp.Namespace, cp.Name, deletingEtcdNode.Machine.Namespace, deletingEtcdNode.Machine.Name)
		// Update the CAPI machine annotation for exclude node draining and set it to true to get the CAPI controllers to not try to drain this node.
		deletingEtcdNode.Machine.Annotations[capi.ExcludeNodeDrainingAnnotation] = "true"
		var err error
		deletingEtcdNode.Machine, err = p.machines.Update(deletingEtcdNode.Machine)
		if err != nil {
			// If we get an error here, go ahead and return the error as this will re-enqueue and we can try again.
			return -1, err
		}
		rb, err := p.rkeBootstrapCache.Get(deletingEtcdNode.Machine.Spec.Bootstrap.ConfigRef.Namespace, deletingEtcdNode.Machine.Spec.Bootstrap.ConfigRef.Name)
		if err != nil {
			return -1, err
		}
		rb = rb.DeepCopy()
		// Annotate the rkebootstrap with a "force remove" annotation. This will short-circuit the "safe etcd removal"
		// logic because at this point we are completely taking the cluster down.
		rb.Annotations[capr.ForceRemoveEtcdAnnotation] = "true"
		_, err = p.rkeBootstrap.Update(rb)
		if err != nil {
			return -1, err
		}
	}
	return len(etcdDeleting), nil
}

// restoreEtcdSnapshot is called multiple times during an etcd snapshot restoration.
// restoreEtcdSnapshot utilizes the status of the corresponding control plane object of the cluster to track state
// The phases are in order:
// Started -> When the phase is started, it gets set to shutdown
// Shutdown -> When the phase is shutdown, it attempts to shut down etcd on all nodes (stop etcd)
// Restore ->  When the phase is restore, it attempts to restore etcd
// Finished -> When the phase is finished, Restore returns nil.
func (p *Planner) restoreEtcdSnapshot(cp *rkev1.RKEControlPlane, status rkev1.RKEControlPlaneStatus, tokensSecret plan.Secret, clusterPlan *plan.Plan, currentVersion *semver.Version) (rkev1.RKEControlPlaneStatus, error) {
	if cp.Spec.ETCDSnapshotRestore == nil || cp.Spec.ETCDSnapshotRestore.Name == "" {
		return p.resetEtcdSnapshotRestoreState(status)
	}

	if status, err := p.startOrRestartEtcdSnapshotRestore(status, cp.Spec.ETCDSnapshotRestore); err != nil {
		return status, err
	}

	snapshot, err := p.retrieveEtcdSnapshot(cp)
	if err != nil {
		return status, err
	}

	// validate the snapshot can be restored by checking to see if the snapshot version is < 1.25.x and the current version is 1.25 or newer.
	if snapshot != nil {
		clusterSpec, err := capr.ParseSnapshotClusterSpecOrError(snapshot)
		if err != nil || clusterSpec == nil {
			logrus.Errorf("[planner] rkecluster %s/%s: error parsing snapshot cluster spec for snapshot %s/%s during etcd restoration: %v", cp.Namespace, cp.Name, snapshot.Namespace, snapshot.Name, err)
		} else {
			snapshotK8sVersion, err := semver.NewVersion(clusterSpec.KubernetesVersion)
			if err != nil {
				return status, err
			}
			if !currentVersion.LessThan(managesystemagent.Kubernetes125) && snapshotK8sVersion.LessThan(managesystemagent.Kubernetes125) {
				return status, fmt.Errorf("unable to restore etcd snapshot -- recorded Kubernetes version on snapshot was <= v1.25.0 and current cluster version was v1.25.0 or newer")
			}
		}
	}

	switch cp.Status.ETCDSnapshotRestorePhase {
	case rkev1.ETCDSnapshotPhaseStarted:
		if status.Initialized || status.Ready {
			status.Initialized = false
			status.Ready = false
			logrus.Debugf("[planner] rkecluster %s/%s: setting controlplane ready/initialized to false during etcd restore", cp.Namespace, cp.Name)
		}
		status, _ = p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseShutdown)
		return status, errWaitingf("shutting down cluster")
	case rkev1.ETCDSnapshotPhaseShutdown:
		if err = p.runEtcdRestoreServiceStop(cp, snapshot, tokensSecret, clusterPlan); err != nil {
			return status, err
		}
		capr.Bootstrapped.False(&status)
		// the error returned from setEtcdSnapshotRestoreState is set based on etcd snapshot restore fields, but we are
		// manipulating other fields so we should unconditionally return a waiting error.
		status, _ = p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseRestore)
		return status, errWaiting("cluster shutdown complete, running etcd restore")
	case rkev1.ETCDSnapshotPhaseRestore:
		if err = p.runEtcdSnapshotRestorePlan(cp, snapshot, cp.Spec.ETCDSnapshotRestore.Name, tokensSecret, clusterPlan); err != nil {
			return status, err
		}
		status.ConfigGeneration++ // Increment config generation to cause the restart_stamp to change
		return p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhasePostRestoreCleanup)
	case rkev1.ETCDSnapshotPhasePostRestoreCleanup:
		if err = p.runEtcdSnapshotPostRestoreCleanupPlan(cp, tokensSecret, clusterPlan); err != nil {
			return status, err
		}
		return p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseRestartCluster)
	case rkev1.ETCDSnapshotPhaseRestartCluster:
		if err := p.pauseCAPICluster(cp, false); err != nil {
			return status, err
		}
		logrus.Infof("[planner] rkecluster %s/%s: running full reconcile during etcd restore to restart cluster", cp.Namespace, cp.Name)
		// Run a full reconcile of the cluster at this point, ignoring drain and concurrency.
		if status, err := p.fullReconcile(cp, status, tokensSecret, clusterPlan, true); err != nil {
			return status, err
		}
		return p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseFinished)
	case rkev1.ETCDSnapshotPhaseFinished:
		return status, nil
	default:
		return p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseStarted)
	}
}
