package planner

import (
	"encoding/base64"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/capr/managesystemagent"
	"github.com/rancher/rancher/pkg/utils"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	etcdRestoreBinPrefix = "capr/etcd-restore/bin"

	etcdRestorePostRestoreWaitForPodListCleanupPath   = "wait_for_pod_list.sh"
	etcdRestorePostRestoreWaitForPodListCleanupScript = `
#!/bin/sh

i=0

while [ $i -lt 30 ]; do
	if $@ >/dev/null 2>&1; then
		exit 0
	fi
	sleep 10
	i=$((i + 1))
done
exit 1
`
	etcdRestoreNodeCleanUpPath   = "clean_up_nodes.sh"
	etcdRestoreNodeCleanUpScript = `
#!/bin/sh

if [ -z "$KUBECTL" ]; then
        echo "Must define KUBECTL environment variable"
        exit 1
fi

if [ -z "$KUBECONFIG" ]; then
        echo "Must define KUBECONFIG environment variable"
        exit 1
fi

MACHINEIDSFILE="$1"
NODENAMESFILE="$2"

if [ -z "$MACHINEIDSFILE" ] || [ -z "$NODENAMESFILE" ]; then
        echo "Must define nodenames file and machineids file"
fi

TMPALLNODES=$(mktemp)
TMPSAVENODES=$(mktemp)

cat "$NODENAMESFILE" > "$TMPSAVENODES"

if ! ${KUBECTL} get nodes --no-headers -o=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' > "$TMPALLNODES"; then
        echo "Error listing all nodes"
        exit 1
fi

while IFS='' read -r IDENTIFIER; do
        if NODENAME=$(${KUBECTL} get node --no-headers -o=jsonpath='{.items[0].metadata.name}' -l rke.cattle.io/machine="$IDENTIFIER"); then
                echo "$NODENAME" >> "$TMPSAVENODES"
        fi
done < "$MACHINEIDSFILE"

echo "Saving nodes:"
cat "$TMPSAVENODES"

while IFS='' read -r NODE; do
        if [ "${NODE}" = "" ]; then
                continue
        fi
        FOUND=false
        while IFS='' read -r KEEP; do
                if [ "${NODE}" = "${KEEP}" ]; then
                        FOUND=true
                        break
                fi
        done < "$TMPSAVENODES"
        if [ "${FOUND}" != "true" ]; then
                echo "Deleting node ${NODE}"
                ${KUBECTL} delete node "${NODE}" --wait=false
        fi
done < "$TMPALLNODES"
rm "$TMPALLNODES"
rm "$TMPSAVENODES"

rm "$MACHINEIDSFILE"
rm "$NODENAMESFILE"
`
	etcdRestoreNodeWaitForReadyPath   = "wait_for_ready.sh"
	etcdRestoreNodeWaitForReadyScript = `
#!/bin/sh

if [ -z "$KUBECTL" ]; then
        echo "Must define KUBECTL environment variable"
        exit 1
fi

if [ -z "$KUBECONFIG" ]; then
        echo "Must define KUBECONFIG environment variable"
        exit 1
fi

TMPMACHINEIDS=$(mktemp)

printf '%s\n' "$@" > "$TMPMACHINEIDS"

DESIREDREADYCOUNT=$(wc -l < "$TMPMACHINEIDS")

DESIREDNODESREADY=false

while ! $DESIREDNODESREADY; do
        DESIREDNODESREADY=true
        while IFS='' read -r MID; do
                if [ "$MID" = "" ]; then
                        exit
                fi
                if NODEREADY=$(${KUBECTL} get node --no-headers -o=custom-columns=STATUS:status.conditions\[\?\(\@.type==\"Ready\"\)\].status -l rke.cattle.io/machine="$MID"); then
                        if [ "$NODEREADY" != "True" ]; then
                                DESIREDNODESREADY=false
                                sleep 5
                                break
                        fi
                fi
        done < "$TMPMACHINEIDS"
done


DESIREDREADYCOUNT=$(wc -l < "$TMPMACHINEIDS")
rm "$TMPMACHINEIDS"

TMPALLREADY=$(mktemp)

ITERCOUNT=0
while [ "$ITERCOUNT" != 60 ]; do
        ACTUALREADYCOUNT=0
        ITERCOUNT=$((ITERCOUNT+1))
        if ! ${KUBECTL} get nodes --no-headers -o=custom-columns=STATUS:status.conditions\[\?\(\@.type==\"Ready\"\)\].status > "$TMPALLREADY"; then
                sleep 5
                continue
        fi
        while IFS='' read -r STATUS; do
                if [ "$STATUS" = "True" ]; then
                        ACTUALREADYCOUNT=$((ACTUALREADYCOUNT+1))
                fi
        done < "$TMPALLREADY"
        if [ "$DESIREDREADYCOUNT" = "$ACTUALREADYCOUNT" ]; then
                break
        fi
        sleep 5
done

rm "$TMPALLREADY"
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

func (p *Planner) runEtcdSnapshotPostRestorePodCleanupPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, clusterPlan *plan.Plan) error {
	initNodes := collect(clusterPlan, isInitNode)
	if len(initNodes) != 1 {
		return fmt.Errorf("multiple init nodes found")
	}
	initNode := initNodes[0]

	initNodePlan, _, err := p.desiredPlan(controlPlane, tokensSecret, initNode, "")
	if err != nil {
		return err
	}

	// If the init node is a controlplane node, deliver the desired plan + pod cleanup instruction
	if isControlPlane(initNode) {
		cleanupScriptFiles, cleanupInstructions := p.generateEtcdRestorePodCleanupFilesAndInstruction(controlPlane, []string{string(initNode.Machine.UID)})
		initNodePlan.Files = append(initNodePlan.Files, cleanupScriptFiles...)
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

	cleanupScriptFiles, cleanupInstructions := p.generateEtcdRestorePodCleanupFilesAndInstruction(controlPlane, []string{string(initNode.Machine.UID), string(controlPlaneEntry.Machine.UID)})
	firstControlPlanePlan.Files = append(firstControlPlanePlan.Files, cleanupScriptFiles...)
	firstControlPlanePlan.Instructions = append(firstControlPlanePlan.Instructions, cleanupInstructions...)
	return assignAndCheckPlan(p.store, ETCDRestoreMessage, controlPlaneEntry, firstControlPlanePlan, joinedServer, 5, 5)
}

func (p *Planner) runEtcdSnapshotPostRestoreNodeCleanupPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, clusterPlan *plan.Plan) error {
	initNodes := collect(clusterPlan, isInitNode)
	if len(initNodes) != 1 {
		return fmt.Errorf("multiple init nodes found")
	}
	initNode := initNodes[0]

	initNodePlan, _, err := p.desiredPlan(controlPlane, tokensSecret, initNode, "")
	if err != nil {
		return err
	}

	var allMachineUIDs, allNodeNames []string
	for _, n := range collect(clusterPlan, isNotDeleting) {
		if n.Machine != nil && n.Machine.UID != "" {
			allMachineUIDs = append(allMachineUIDs, string(n.Machine.UID))
		}
		if n.Machine != nil && n.Machine.Status.NodeRef != nil && n.Machine.Status.NodeRef.Name != "" {
			allNodeNames = append(allNodeNames, n.Machine.Status.NodeRef.Name)
		}
	}

	cleanupScriptFiles, cleanupInstructions := p.generateEtcdRestoreNodeCleanupFilesAndInstruction(controlPlane, allMachineUIDs, allNodeNames)
	initNodePlan.Files = append(initNodePlan.Files, cleanupScriptFiles...)
	initNodePlan.Instructions = append(initNodePlan.Instructions, cleanupInstructions...)
	return assignAndCheckPlan(p.store, ETCDRestoreMessage, initNode, initNodePlan, "", 5, 5)
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

	loopbackAddress := capr.GetLoopbackAddress(controlPlane)

	if utils.IsPlainIPV6(loopbackAddress) {
		loopbackAddress = fmt.Sprintf("[%s]", loopbackAddress)
	}

	args := []string{
		"server",
		"--cluster-reset",
		fmt.Sprintf("--etcd-arg=advertise-client-urls=https://%s:2379", loopbackAddress), // this is a workaround for: https://github.com/rancher/rke2/issues/4052 and can likely remain indefinitely (unless IPv6-only becomes a requirement)
		"--etcd-disable-snapshots=false",                                                 // this is a workaround for https://github.com/k3s-io/k3s/issues/8031
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

	runtime := capr.GetRuntime(controlPlane.Spec.KubernetesVersion)

	nodePlan.Instructions = append(nodePlan.Instructions, convertToIdempotentInstruction(
		controlPlane,
		"etcd-restore/restore-kill-all",
		fmt.Sprintf("%v", controlPlane.Status.ETCDSnapshotRestore),
		generateKillAllInstruction(controlPlane)))

	if runtime == capr.RuntimeRKE2 {
		if generated, instruction := generateManifestRemovalInstruction(controlPlane, entry); generated {
			nodePlan.Instructions = append(nodePlan.Instructions, convertToIdempotentInstruction(
				controlPlane,
				"etcd-restore/restore-manifest-removal",
				fmt.Sprintf("%v", controlPlane.Status.ETCDSnapshotRestore),
				instruction))
		}
	}

	// make sure to install the desired version before performing restore
	nodePlan.Instructions = append(nodePlan.Instructions,
		p.generateInstallInstructionWithSkipStart(controlPlane, entry),
		convertToIdempotentInstruction(
			controlPlane,
			"etcd-restore/clean-etcd-dir",
			fmt.Sprintf("%v", controlPlane.Status.ETCDSnapshotRestore), plan.OneTimeInstruction{
				Name:    "remove-etcd-db-dir",
				Command: "rm",
				Args: []string{
					"-rf",
					path.Join(capr.GetDistroDataDir(controlPlane), "server/db/etcd"),
				}}),
		idempotentInstruction(
			controlPlane,
			"etcd-restore/restore",
			fmt.Sprintf("%v", controlPlane.Status.ETCDSnapshotRestore),
			capr.GetRuntimeCommand(controlPlane.Spec.KubernetesVersion),
			args,
			env),
	)

	return nodePlan, joinedServer, nil
}

func (p *Planner) generateStopServiceAndKillAllPlan(controlPlane *rkev1.RKEControlPlane, tokensSecret plan.Secret, server *planEntry, joinServer string) (plan.NodePlan, string, error) {
	nodePlan, _, joinedServer, err := p.generatePlanWithConfigFiles(controlPlane, tokensSecret, server, joinServer, true)
	if err != nil {
		return nodePlan, joinedServer, err
	}

	runtime := capr.GetRuntime(controlPlane.Spec.KubernetesVersion)
	nodePlan.Instructions = append(nodePlan.Instructions, generateKillAllInstruction(controlPlane))

	if runtime == capr.RuntimeRKE2 {
		if generated, instruction := generateManifestRemovalInstruction(controlPlane, server); generated {
			nodePlan.Instructions = append(nodePlan.Instructions, instruction)
		}
	}

	return nodePlan, joinedServer, nil
}

func generateKillAllInstruction(controlPlane *rkev1.RKEControlPlane) plan.OneTimeInstruction {
	runtime := capr.GetRuntime(controlPlane.Spec.KubernetesVersion)
	killAllScript := runtime + "-killall.sh"

	return plan.OneTimeInstruction{
		Name:    "shutdown",
		Command: "/bin/sh",
		Env: []string{
			fmt.Sprintf("%s_DATA_DIR=%s", strings.ToUpper(runtime), capr.GetDistroDataDir(controlPlane)),
		},
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
			path.Join(capr.GetDistroDataDir(controlPlane), "server/db/etcd/tombstone"),
		},
	}
}

func etcdRestoreScriptPath(controlPlane *rkev1.RKEControlPlane, file string) string {
	return path.Join(capr.GetDistroDataDir(controlPlane), etcdRestoreBinPrefix, file)
}

// generateEtcdRestorePodCleanupFilesAndInstruction generates a file that contains a script that checks API server health and a slice of instructions that cleans up system pods on etcd restore.
func (p *Planner) generateEtcdRestorePodCleanupFilesAndInstruction(controlPlane *rkev1.RKEControlPlane, cleanupMachineUIDs []string) ([]plan.File, []plan.OneTimeInstruction) {
	runtime := capr.GetRuntime(controlPlane.Spec.KubernetesVersion)

	kubectl, kubeconfig := capr.GetKubectlAndKubeconfigPaths(controlPlane)
	if kubectl == "" || kubeconfig == "" {
		return nil, nil
	}

	instructions := []plan.OneTimeInstruction{
		idempotentInstruction(
			controlPlane,
			"etcd-restore/pods-wait-for-podlist",
			fmt.Sprintf("%v", controlPlane.Status.ETCDSnapshotRestore),
			"/bin/sh",
			[]string{
				"-x",
				etcdRestoreScriptPath(controlPlane, etcdRestorePostRestoreWaitForPodListCleanupPath),
				kubectl,
				"--kubeconfig",
				kubeconfig,
				"get",
				"pods",
				"--all-namespaces",
			},
			[]string{}),
		idempotentInstruction(
			controlPlane,
			"etcd-restore/wait-for-desired-ready-nodes",
			fmt.Sprintf("%v", controlPlane.Status.ETCDSnapshotRestore),
			"/bin/sh",
			append([]string{etcdRestoreScriptPath(controlPlane, etcdRestoreNodeWaitForReadyPath)}, cleanupMachineUIDs...),
			[]string{
				fmt.Sprintf("%s=%s", "KUBECTL", kubectl),
				fmt.Sprintf("%s=%s", "KUBECONFIG", kubeconfig),
			}),
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
			instructions = append(instructions, idempotentInstruction(
				controlPlane,
				fmt.Sprintf("etcd-restore/post-restore-cleanup-pods-%d", i),
				fmt.Sprintf("%v", controlPlane.Status.ETCDSnapshotRestore),
				kubectl,
				[]string{
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
				[]string{}))
		}
	}

	return []plan.File{
		{
			Content: base64.StdEncoding.EncodeToString([]byte(etcdRestorePostRestoreWaitForPodListCleanupScript)),
			Path:    etcdRestoreScriptPath(controlPlane, etcdRestorePostRestoreWaitForPodListCleanupPath),
			Dynamic: true,
		},
		{
			Content: base64.StdEncoding.EncodeToString([]byte(etcdRestoreNodeWaitForReadyScript)),
			Path:    etcdRestoreScriptPath(controlPlane, etcdRestoreNodeWaitForReadyPath),
			Dynamic: true,
		},
	}, instructions
}

// generateEtcdRestorePodCleanupFilesAndInstruction generates a file that contains a script that checks API server health and a slice of instructions that cleans up system pods on etcd restore.
func (p *Planner) generateEtcdRestoreNodeCleanupFilesAndInstruction(controlPlane *rkev1.RKEControlPlane, allMachineUIDs []string, allNodeNames []string) ([]plan.File, []plan.OneTimeInstruction) {
	kubectl, kubeconfig := capr.GetKubectlAndKubeconfigPaths(controlPlane)
	if kubectl == "" || kubeconfig == "" {
		return nil, nil
	}

	var nodeNames, machineIDs []byte

	for _, mid := range allMachineUIDs {
		machineIDs = fmt.Appendf(machineIDs, "%s\n", mid)
	}

	for _, nodeName := range allNodeNames {
		nodeNames = fmt.Appendf(nodeNames, "%s\n", nodeName)
	}

	identifier := name.Hex(controlPlane.Spec.ETCDSnapshotRestore.Name+controlPlane.Spec.ETCDSnapshotRestore.RestoreRKEConfig+strconv.Itoa(controlPlane.Spec.ETCDSnapshotRestore.Generation), 10)

	machineIDsFile := fmt.Sprintf("machine-ids-%s", identifier)
	nodeNamesFile := fmt.Sprintf("node-names-%s", identifier)

	instructions := []plan.OneTimeInstruction{
		idempotentInstruction(
			controlPlane,
			"etcd-restore/cleanup-nodes",
			fmt.Sprintf("%v", controlPlane.Status.ETCDSnapshotRestore),
			"/bin/sh",
			[]string{etcdRestoreScriptPath(controlPlane, etcdRestoreNodeCleanUpPath), etcdRestoreScriptPath(controlPlane, machineIDsFile), etcdRestoreScriptPath(controlPlane, nodeNamesFile)},
			[]string{
				fmt.Sprintf("%s=%s", "KUBECTL", kubectl),
				fmt.Sprintf("%s=%s", "KUBECONFIG", kubeconfig),
			}),
	}

	return []plan.File{
		{
			Content: base64.StdEncoding.EncodeToString([]byte(etcdRestoreNodeCleanUpScript)),
			Path:    etcdRestoreScriptPath(controlPlane, etcdRestoreNodeCleanUpPath),
			Dynamic: true,
		},
		{
			Content: base64.StdEncoding.EncodeToString(machineIDs),
			Path:    etcdRestoreScriptPath(controlPlane, machineIDsFile),
			Dynamic: true,
		},
		{
			Content: base64.StdEncoding.EncodeToString(nodeNames),
			Path:    etcdRestoreScriptPath(controlPlane, nodeNamesFile),
			Dynamic: true,
		},
	}, instructions
}

func generateRemoveTLSAndCredDirInstructions(controlPlane *rkev1.RKEControlPlane) []plan.OneTimeInstruction {
	return []plan.OneTimeInstruction{
		{
			Name:    "remove-tls-directory",
			Command: "rm",
			Args: []string{
				"-rf",
				path.Join(capr.GetDistroDataDir(controlPlane), "server/tls"),
			},
		},
		{
			Name:    "remove-cred-directory",
			Command: "rm",
			Args: []string{
				"-rf",
				path.Join(capr.GetDistroDataDir(controlPlane), "server/cred"),
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
		return p.electInitNode(controlPlane, clusterPlan, true)
	}
	// make sure that we only have one suitable init node, and elect it.
	count := len(collect(clusterPlan, canBeInitNode))
	if count == 0 {
		return "", fmt.Errorf("no init node existed and no corresponding etcd snapshot CR found, no assumption can be made for the machine that contains the snapshot")
	} else if count > 1 {
		return "", fmt.Errorf("more than one init node existed and no corresponding etcd snapshot CR found, no assumption can be made for the machine that contains the snapshot")
	}
	logrus.Infof("[planner] rkecluster %s/%s: electing init node for local snapshot with no associated CR", controlPlane.Namespace, controlPlane.Name)
	return p.electInitNode(controlPlane, clusterPlan, true)
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
		// Clean up previous restoration tracking attempts before starting this restoration.
		stopPlan.Instructions = append(stopPlan.Instructions, generateIdempotencyCleanupInstruction(controlPlane, "etcd-restore"))
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
		if deletingEtcdNode.Machine.Annotations == nil {
			deletingEtcdNode.Machine.Annotations = map[string]string{}
		}
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
		if rb.Annotations == nil {
			rb.Annotations = map[string]string{}
		}
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
		return p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhasePostRestorePodCleanup)
	case rkev1.ETCDSnapshotPhasePostRestorePodCleanup:
		if err = p.runEtcdSnapshotPostRestorePodCleanupPlan(cp, tokensSecret, clusterPlan); err != nil {
			return status, err
		}
		return p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhaseInitialRestartCluster)
	case rkev1.ETCDSnapshotPhaseInitialRestartCluster:
		if err := p.pauseCAPICluster(cp, false); err != nil {
			return status, err
		}
		logrus.Infof("[planner] rkecluster %s/%s: running full reconcile during etcd restore to initially restart cluster", cp.Namespace, cp.Name)
		// Run a full reconcile of the cluster at this point, ignoring drain and concurrency.
		if status, err := p.fullReconcile(cp, status, tokensSecret, clusterPlan, true); err != nil {
			return status, err
		}
		return p.setEtcdSnapshotRestoreState(status, cp.Spec.ETCDSnapshotRestore, rkev1.ETCDSnapshotPhasePostRestoreNodeCleanup)
	case rkev1.ETCDSnapshotPhasePostRestoreNodeCleanup:
		if err = p.runEtcdSnapshotPostRestoreNodeCleanupPlan(cp, tokensSecret, clusterPlan); err != nil {
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
