// Package config delivers Rancher-owned distro configuration to the nodes of true imported,
// standalone RKE2/K3s clusters.
//
// Provisioned (CAPR) and turtles-imported (CAPRKE2) clusters have a planner which renders their
// entire config.yaml from the cluster spec. Imported clusters have no planner: their config.yaml
// is owned by whoever installed the distro, so Rancher may only contribute additive drop-ins to
// /etc/rancher/<runtime>/config.yaml.d. This controller renders the cluster's desired etcd
// configuration into the 50-rancher-etcd.yaml drop-in and delivers it — followed by the service
// restart the distro needs to pick it up — through the same beacon + machine-plan mechanism the
// day-2 operation controllers use.
package config

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"path"
	"strconv"
	"strings"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/management/systemagent"
	"github.com/rancher/rancher/pkg/features"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/kv"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
)

const (
	// AppliedETCDConfigHashAnnotation records, on each mgmtv3.Node, the hash of the etcd config
	// drop-in content last confirmed to be written to that node. A node whose annotation matches
	// the desired hash is skipped, so a cluster whose nodes are all up to date never takes the
	// beacon and never restarts a service. Removing the annotation forces redelivery.
	AppliedETCDConfigHashAnnotation = "management.cattle.io/applied-etcd-config-hash"

	// BeaconOwnerKey identifies this controller while it holds a cluster's beacon. Config
	// delivery assigns machine plans and restarts servers, which would otherwise race an
	// in-flight day-2 operation doing the same to the same nodes.
	BeaconOwnerKey = "imported-etcd-config"

	// etcdConfigFileName is the config.yaml.d drop-in this controller owns. The 50- prefix orders
	// it after the distro installer's own drop-ins and before any 90-*.yaml a user may add, so an
	// operator retains the ability to override what Rancher renders.
	etcdConfigFileName = "50-rancher-etcd.yaml"

	// etcdConfigArgPrefix is prepended to the s3 arguments the adapter renders, matching the
	// etcd-s3-* config keys the distro reads.
	etcdConfigArgPrefix = "etcd-"

	// administratedAnnotation marks a mgmt cluster which is a shell for a v2prov (CAPR)
	// provisioning cluster. Those clusters have a planner and are out of scope here.
	administratedAnnotation = "provisioning.cattle.io/administrated"

	// planPollInterval is how often the cluster is re-enqueued while waiting on the beacon or on
	// a node to apply its plan and come back healthy.
	planPollInterval = 5 * time.Second
)

type handler struct {
	clusters mgmtcontrollers.ClusterController

	nodes     mgmtcontrollers.NodeClient
	nodeCache mgmtcontrollers.NodeCache

	secrets corecontrollers.SecretClient

	beacons     plancontrollers.BeaconClient
	beaconCache plancontrollers.BeaconCache

	store *planapi.Store

	clients *wrangler.CAPIContext
}

// Register wires the imported etcd config controller into the given CAPI-scoped wrangler context.
// It needs the CAPI context because delivery goes through the same ops.Adapter the day-2
// operation controllers use — the adapter is what knows a cluster's config directory, data
// directory, server unit, and probes.
func Register(ctx context.Context, clients *wrangler.CAPIContext) {
	h := &handler{
		clusters:    clients.Mgmt.Cluster(),
		nodes:       clients.Mgmt.Node(),
		nodeCache:   clients.Mgmt.Node().Cache(),
		secrets:     clients.Core.Secret(),
		beacons:     clients.Plan.Beacon(),
		beaconCache: clients.Plan.Beacon().Cache(),
		store:       planapi.NewStore(clients.Core.Secret()),
		clients:     clients,
	}

	clients.Mgmt.Cluster().OnChange(ctx, "config", h.onChange)

	// mgmtv3.Nodes are namespaced by cluster name and carry the applied-hash annotation, so any
	// node event (a new etcd node joining, a node losing the annotation) is a reason to reconcile
	// the owning cluster.
	relatedresource.WatchClusterScoped(ctx, "config-node-enqueuer", enqueueClusterForNode, clients.Mgmt.Cluster(), clients.Mgmt.Node())

	// Machine-plan secrets are how the system-agent reports back that a plan was applied and that
	// its probes pass. Without this watch, each step of a rolling delivery would have to wait for
	// the next poll interval.
	relatedresource.WatchClusterScoped(ctx, "config-machine-plan-enqueuer", enqueueClusterForMachinePlan, clients.Mgmt.Cluster(), clients.Core.Secret())
}

// enqueueClusterForNode maps a mgmtv3.Node to its owning mgmt cluster, which is the namespace the
// node lives in.
func enqueueClusterForNode(namespace, _ string, _ runtime.Object) ([]relatedresource.Key, error) {
	if namespace == "" {
		return nil, nil
	}
	return []relatedresource.Key{{Name: namespace}}, nil
}

// enqueueClusterForMachinePlan maps a machine-plan secret to the cluster it belongs to, ignoring
// every other secret in the cluster namespace.
func enqueueClusterForMachinePlan(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	secret, ok := obj.(*corev1.Secret)
	if !ok || secret.Type != capr.SecretTypeMachinePlan {
		return nil, nil
	}
	clusterName := secret.Labels[capr.ClusterNameLabel]
	if clusterName == "" {
		return nil, nil
	}
	return []relatedresource.Key{{Name: clusterName}}, nil
}

// s3Renderer is the part of ops.Adapter that rendering the drop-in depends on: turning the
// cluster's S3 spec into distro arguments and the files those arguments reference.
type s3Renderer interface {
	ToS3ArgsEnvAndFiles(secret *corev1.Secret, s3 *rkev1.ETCDSnapshotS3, prefix string, secretKeyInEnv bool) ([]string, []string, []planapi.File, error)
}

// target is an etcd node the drop-in has to reach: the machine-plan secret the plan is assigned
// to, the mgmtv3.Node the applied hash is recorded on, and the content that node should have.
type target struct {
	secret *corev1.Secret
	node   *apimgmtv3.Node

	content []byte
	// files are the extra plan files the content refers to — currently the S3 endpoint CA, which
	// the config references by a path that only exists once the file is written.
	files []planapi.File
	hash  string
	// configured is false when the cluster asks for no etcd configuration at all.
	configured bool
}

func (t target) pending() bool {
	applied := t.node.Annotations[AppliedETCDConfigHashAnnotation]
	if applied == t.hash {
		return false
	}
	// Nothing configured and nothing ever delivered: leave the node alone rather than write an
	// empty Rancher-owned file (and restart the server) for a cluster which never asked for one.
	if !t.configured && applied == "" {
		return false
	}
	return true
}

func (h *handler) onChange(_ string, cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster == nil {
		return cluster, nil
	}

	if cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	// only true imported standalone RKE2/K3s clusters, i.e. not managed by CAPR or CAPRKE2
	runtimeName := managedRuntime(cluster)
	if runtimeName == "" {
		return cluster, nil
	}

	// Plans are delivered by the imported system-agent, which is only installed (and only has a
	// beacon and machine-plan secrets) while imported day-2 operations are enabled.
	if !features.ImportedDay2Ops.Enabled() || !systemagent.OperationsEnabledForCluster(cluster) {
		return cluster, nil
	}

	adapter := ops.NewImportedAdapter(h.clients, cluster)

	// gather the etcd nodes and what each of them should have
	targets, err := h.collect(cluster, adapter, runtimeName)
	if err != nil {
		if planapi.IsTransient(err) {
			return cluster, err
		}
		logrus.Errorf("[importedconfig] %s: encountered terminal error collecting machine-plan secrets: %v", cluster.Name, err)
		return cluster, nil
	}

	var pending []target
	for _, t := range targets {
		if t.pending() {
			pending = append(pending, t)
		}
	}

	// The beacon is resolved with the mgmtv3 Cluster's name-as-namespace convention, the same
	// convention ImportedAdapter.BeaconRef uses for imported clusters.
	beaconNamespace, beaconName := adapter.BeaconRef()

	beacon, err := h.beaconCache.Get(beaconNamespace, beaconName)
	if apierrors.IsNotFound(err) {
		beacon = nil
	} else if err != nil {
		return cluster, err
	}

	if len(pending) == 0 {
		// Release on the way out rather than at the end of the delivering reconcile: a restart
		// between stamping the last node and releasing the beacon would otherwise leave the
		// cluster's beacon held by a controller with no work left to do.
		if planapi.IsOwningBeaconHolder(beacon, BeaconOwnerKey) {
			logrus.Debugf("[importedconfig] %s: etcd config is up to date, releasing beacon", cluster.Name)
			return cluster, planapi.ReleaseBeacon(beacon, h.beacons, BeaconOwnerKey)
		}
		return cluster, nil
	}

	// Some nodes are out of date: take the beacon before assigning any plan, so delivery cannot
	// race a day-2 operation assigning its own plans to the same machine-plan secrets.
	if beacon == nil {
		logrus.Debugf("[importedconfig] %s: waiting for beacon creation", cluster.Name)
		h.clusters.EnqueueAfter(cluster.Name, planPollInterval)
		return cluster, nil
	}

	acquired, err := planapi.AcquireBeacon(beacon, h.beacons, BeaconOwnerKey)
	if err != nil {
		return cluster, err
	}
	if acquired == nil {
		// A day-2 operation (or imported day-2 ops teardown) owns the beacon. Config delivery is
		// not time critical, so wait for the current holder to finish rather than preempt it.
		logrus.Debugf("[importedconfig] %s: waiting for beacon release from %q", cluster.Name, beacon.Status.Owner)
		h.clusters.EnqueueAfter(cluster.Name, planPollInterval)
		return cluster, nil
	}

	// The system-agent only downloads its machine plan while the beacon is active.
	beacon, err = planapi.ToggleBeacon(acquired, true, h.beacons)
	if err != nil {
		return cluster, err
	}

	return cluster, h.deliver(cluster, adapter, beacon, pending)
}

// deliver rolls the drop-in out to the pending nodes one at a time, and releases the beacon once
// they are all done (or once one of them has failed).
//
// The roll is strictly serial because applying the drop-in restarts the node's server unit:
// restarting every etcd node at once would take the cluster's control plane down. A node is only
// considered done — and only then stamped and left behind — when its plan has been applied and
// its probes pass again, which is what makes the next node safe to touch.
func (h *handler) deliver(cluster *apimgmtv3.Cluster, adapter ops.Adapter, beacon *planv1alpha1.Beacon, pending []target) error {
	for _, t := range pending {
		probes, err := adapter.RenderProbes(t.secret, true)
		if err != nil {
			return err
		}

		nodePlan := &planapi.Plan{
			Files: append([]planapi.File{
				{
					Content: base64.StdEncoding.EncodeToString(t.content),
					Path:    path.Join(adapter.ConfigDirectory(t.secret), etcdConfigFileName),
				},
			}, t.files...),
			OneTimeInstructions: []planapi.OneTimeInstruction{
				// The distro only reads config.yaml.d at startup, so the drop-in is inert until
				// the server unit rolls.
				{
					CommonInstruction: planapi.CommonInstruction{
						Name:    "restart",
						Command: "systemctl",
						Args: []string{
							"restart",
							adapter.ServerUnit(),
						},
					},
				},
			},
			Probes: probes,
		}

		planStatus, err := h.store.AssignPlan(t.secret, nodePlan, 1, 1)
		if err != nil {
			return err
		}

		if planStatus.Failure() {
			// A failed plan is terminal for the system-agent: it will not retry until the plan
			// content changes. Release the beacon so one node's failure does not also block
			// day-2 operations for the whole cluster, and leave the remaining nodes untouched —
			// rolling on past a node whose server did not come back is how a cluster loses quorum.
			// Delivery is retried when the desired config changes (new content, therefore a new
			// plan) or the node is re-created.
			logrus.Errorf("[importedconfig] %s: failed to apply etcd config to node %s, abandoning rollout and releasing beacon", cluster.Name, t.node.Name)
			return planapi.ReleaseBeacon(beacon, h.beacons, BeaconOwnerKey)
		}

		if !planStatus.Success() {
			logrus.Infof("[importedconfig] %s: %s", cluster.Name, planapi.Message([]planapi.PlanStatus{*planStatus}))
			h.clusters.EnqueueAfter(cluster.Name, planPollInterval)
			return nil
		}

		// the file is on the node and the node is healthy again: record the hash so this node is
		// skipped from here on, and move on to the next one
		if err := h.recordAppliedHash(t.node, t.hash); err != nil {
			return err
		}
	}

	logrus.Infof("[importedconfig] %s: etcd config applied to %d node(s), releasing beacon", cluster.Name, len(pending))

	return planapi.ReleaseBeacon(beacon, h.beacons, BeaconOwnerKey)
}

// collect gathers the etcd machine-plan secrets for the cluster, pairs each with its mgmtv3.Node,
// and renders the content that node should end up with.
//
// Secrets whose node is missing or being deleted are dropped: there is nowhere to record the
// applied hash for them, and a deleting node should neither be restarted nor hold up the beacon.
func (h *handler) collect(cluster *apimgmtv3.Cluster, adapter ops.Adapter, runtimeName string) ([]target, error) {
	secrets, err := planapi.NewCollector(h.secrets, cluster, cluster.Name).
		WithLabels(planapi.Label(capr.EtcdRoleLabel, "true")).
		WithSorter(planapi.DefaultSorter()).
		Collect()
	if err != nil {
		return nil, err
	}

	etcd := etcdSpec(cluster, runtimeName)

	targets := make([]target, 0, len(secrets))
	for _, secret := range secrets {
		machineName := ops.MachineName(secret)
		if machineName == "" {
			logrus.Debugf("[importedconfig] %s: machine-plan secret %s/%s has no %s label, skipping", cluster.Name, secret.Namespace, secret.Name, planv1alpha1.MachineLifecycleNameLabel)
			continue
		}

		node, err := h.nodeCache.Get(cluster.Name, machineName)
		if apierrors.IsNotFound(err) {
			logrus.Debugf("[importedconfig] %s: node %s for machine-plan secret %s/%s not found, skipping", cluster.Name, machineName, secret.Namespace, secret.Name)
			continue
		} else if err != nil {
			return nil, err
		}

		if node.DeletionTimestamp != nil {
			continue
		}

		content, files, configured, err := renderETCDConfig(adapter, secret, etcd)
		if err != nil {
			return nil, err
		}

		targets = append(targets, target{
			secret:     secret,
			node:       node,
			content:    content,
			files:      files,
			hash:       planapi.PlanHash(content),
			configured: configured,
		})
	}

	return targets, nil
}

// recordAppliedHash stamps the applied-hash annotation on the node. The node is re-fetched from
// the API rather than the cache so a conflict retry sees the latest resourceVersion.
func (h *handler) recordAppliedHash(node *apimgmtv3.Node, hash string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest, err := h.nodes.Get(node.Namespace, node.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		} else if err != nil {
			return err
		}

		if latest.Annotations[AppliedETCDConfigHashAnnotation] == hash {
			return nil
		}

		if latest.Annotations == nil {
			latest.Annotations = map[string]string{}
		}
		latest.Annotations[AppliedETCDConfigHashAnnotation] = hash

		_, err = h.nodes.Update(latest)
		return err
	})
}

// managedRuntime returns the distro runtime ("rke2" or "k3s") when cluster is a true imported,
// standalone RKE2/K3s cluster, and "" for every other cluster.
//
// The checks mirror the ImportedAdapter dispatch in pkg/operations/imported.go: the turtles
// capi-cluster-owner labels identify a CAPI-backed cluster and the administrated annotation
// identifies a v2prov shell — both have a planner which owns config.yaml, so Rancher must not
// also write drop-ins for them. Status.Provider is the same signal ImportedAdapter uses to pick
// the runtime, which keeps this gate and everything the adapter derives from the runtime — config
// directory, data directory, server unit — in agreement.
func managedRuntime(cluster *apimgmtv3.Cluster) string {
	// The local cluster is either RancherD-managed or, when embedded in Harvester, treated as a
	// Rancher-provisioned RKE2 cluster. Neither is an imported cluster.
	if cluster.Name == "local" {
		return ""
	}

	if cluster.Labels[capr.CAPIClusterOwnerLabel] != "" || cluster.Labels[capr.CAPIClusterOwnerNSLabel] != "" {
		return ""
	}

	if cluster.Annotations[administratedAnnotation] == "true" {
		return ""
	}

	switch cluster.Status.Provider {
	case capr.RuntimeRKE2:
		return capr.RuntimeRKE2
	case capr.RuntimeK3S:
		return capr.RuntimeK3S
	}

	return ""
}

// etcdSpec returns the etcd configuration the cluster desires for the given runtime. An unset
// distro config is equivalent to an empty etcd config: nothing to render.
func etcdSpec(cluster *apimgmtv3.Cluster, runtimeName string) apimgmtv3.ETCD {
	switch runtimeName {
	case capr.RuntimeRKE2:
		if cluster.Spec.Rke2Config != nil {
			return cluster.Spec.Rke2Config.ETCD
		}
	case capr.RuntimeK3S:
		if cluster.Spec.K3sConfig != nil {
			return cluster.Spec.K3sConfig.ETCD
		}
	}
	return apimgmtv3.ETCD{}
}

// renderETCDConfig renders the etcd drop-in content for one node, any extra files that content
// refers to, and whether the cluster configured anything at all.
//
// The keys mirror pkg/capr/planner's addETCD — including the s3 keys, which are rendered from the
// adapter's arguments the same way the planner folds them into a provisioned cluster's
// config.yaml — so an imported cluster and a provisioned cluster with the same etcd spec end up
// with the same distro configuration. The secret key is rendered into the file rather than an
// environment variable for the same reason: this is configuration read at startup, not an
// instruction with an environment.
//
// The content is JSON, which the distro parses as YAML, and is marshalled from a map so the key
// order — and therefore the hash — is stable across reconciles.
func renderETCDConfig(adapter s3Renderer, secret *corev1.Secret, etcd apimgmtv3.ETCD) ([]byte, []planapi.File, bool, error) {
	config := map[string]any{}

	if etcd.DisableSnapshots {
		config["etcd-disable-snapshots"] = true
	}
	if etcd.SnapshotRetention > 0 {
		config["etcd-snapshot-retention"] = etcd.SnapshotRetention
	}
	if etcd.SnapshotScheduleCron != "" {
		config["etcd-snapshot-schedule-cron"] = etcd.SnapshotScheduleCron
	}

	args, _, files, err := adapter.ToS3ArgsEnvAndFiles(secret, etcd.S3, etcdConfigArgPrefix, false)
	if err != nil {
		return nil, nil, false, err
	}

	for _, arg := range args {
		key, value := kv.Split(strings.TrimPrefix(arg, "--"), "=")
		switch {
		case value == "":
			// A valueless argument (--etcd-s3, --etcd-s3-skip-ssl-verify) is a boolean key.
			config[key] = true
		case key == "etcd-s3-retention":
			retention, err := strconv.Atoi(value)
			if err != nil {
				logrus.Warnf("[importedconfig] failed to convert etcd-s3-retention value %s to int: %v", value, err)
				continue
			}
			config[key] = retention
		default:
			config[key] = value
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, nil, false, err
	}

	return data, files, len(config) > 0, nil
}
