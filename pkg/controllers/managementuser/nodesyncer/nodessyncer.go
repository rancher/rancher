package nodesyncer

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/pkg/errors"
	cond "github.com/rancher/norman/condition"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator/assemblers"
	"github.com/rancher/rancher/pkg/controllers/managementagent/podresources"
	"github.com/rancher/rancher/pkg/controllers/managementlegacy/compose/common"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	provcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/librke"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/systemtokens"
	rketypes "github.com/rancher/rke/types"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	AllNodeKey     = "_machine_all_"
	annotationName = "management.cattle.io/nodesyncer"
)

type nodeSyncer struct {
	machines         v3.NodeInterface
	clusterNamespace string
	nodesSyncer      *nodesSyncer
}

type nodesSyncer struct {
	machines             v3.NodeInterface
	machineLister        v3.NodeLister
	nodeLister           v1.NodeLister
	nodeClient           v1.NodeInterface
	clusterNamespace     string
	clusterLister        v3.ClusterLister
	serviceOptionsLister v3.RkeK8sServiceOptionLister
	serviceOptions       v3.RkeK8sServiceOptionInterface
	sysImagesLister      v3.RkeK8sSystemImageLister
	sysImages            v3.RkeK8sSystemImageInterface
	secretLister         v1.SecretLister
	provClusterCache     provcontrollers.ClusterCache
	capiClusterCache     capicontrollers.ClusterCache
	rkeControlPlaneCache rkecontrollers.RKEControlPlaneCache
}

type nodeDrain struct {
	systemTokens         systemtokens.Interface
	tokenClient          v3.TokenInterface
	userClient           v3.UserInterface
	kubeConfigGetter     common.KubeConfigGetter
	clusterName          string
	systemAccountManager *systemaccount.Manager
	clusterLister        v3.ClusterLister
	machines             v3.NodeInterface
	nodeLister           v1.NodeLister
	ctx                  context.Context
	nodesToContext       map[string]context.CancelFunc
}

type canChangeValuePolicy func(key string) bool

func Register(ctx context.Context, cluster *config.UserContext, kubeConfigGetter common.KubeConfigGetter) {
	m := &nodesSyncer{
		clusterNamespace:     cluster.ClusterName,
		machines:             cluster.Management.Management.Nodes(cluster.ClusterName),
		machineLister:        cluster.Management.Management.Nodes(cluster.ClusterName).Controller().Lister(),
		nodeLister:           cluster.Core.Nodes("").Controller().Lister(),
		nodeClient:           cluster.Core.Nodes(""),
		clusterLister:        cluster.Management.Management.Clusters("").Controller().Lister(),
		serviceOptionsLister: cluster.Management.Management.RkeK8sServiceOptions("").Controller().Lister(),
		serviceOptions:       cluster.Management.Management.RkeK8sServiceOptions(""),
		sysImagesLister:      cluster.Management.Management.RkeK8sSystemImages("").Controller().Lister(),
		sysImages:            cluster.Management.Management.RkeK8sSystemImages(""),
		secretLister:         cluster.Management.Core.Secrets("").Controller().Lister(),
		provClusterCache:     cluster.Management.Wrangler.Provisioning.Cluster().Cache(),
		capiClusterCache:     cluster.Management.Wrangler.CAPI.Cluster().Cache(),
		rkeControlPlaneCache: cluster.Management.Wrangler.RKE.RKEControlPlane().Cache(),
	}

	n := &nodeSyncer{
		clusterNamespace: cluster.ClusterName,
		machines:         cluster.Management.Management.Nodes(cluster.ClusterName),
		nodesSyncer:      m,
	}

	d := &nodeDrain{
		systemTokens:         cluster.Management.SystemTokens,
		tokenClient:          cluster.Management.Management.Tokens(""),
		userClient:           cluster.Management.Management.Users(""),
		kubeConfigGetter:     kubeConfigGetter,
		clusterName:          cluster.ClusterName,
		systemAccountManager: systemaccount.NewManager(cluster.Management),
		clusterLister:        cluster.Management.Management.Clusters("").Controller().Lister(),
		machines:             cluster.Management.Management.Nodes(cluster.ClusterName),
		nodeLister:           cluster.Core.Nodes("").Controller().Lister(),
		ctx:                  ctx,
		nodesToContext:       map[string]context.CancelFunc{},
	}

	cluster.Core.Nodes("").Controller().AddHandler(ctx, "nodesSyncer", n.sync)
	cluster.Management.Management.Nodes(cluster.ClusterName).Controller().AddHandler(ctx, "machinesSyncer", m.sync)
	cluster.Management.Management.Nodes(cluster.ClusterName).Controller().AddHandler(ctx, "machinesLabelSyncer", m.syncLabels)
	cluster.Management.Management.Nodes(cluster.ClusterName).Controller().AddHandler(ctx, "cordonFieldsSyncer", m.syncCordonFields)
	cluster.Management.Management.Nodes(cluster.ClusterName).Controller().AddHandler(ctx, "drainNodeSyncer", d.drainNode)
	cluster.Management.Management.Nodes(cluster.ClusterName).Controller().AddHandler(ctx, "machineTaintSyncer", m.syncTaints)
}

func (n *nodeSyncer) sync(key string, node *corev1.Node) (runtime.Object, error) {
	needUpdate, err := n.needUpdate(key, node)
	if err != nil {
		return nil, err
	}
	if needUpdate {
		n.machines.Controller().Enqueue(n.clusterNamespace, AllNodeKey)
	}

	return nil, nil
}

func (n *nodeSyncer) needUpdate(_ string, node *corev1.Node) (bool, error) {
	if node == nil || node.DeletionTimestamp != nil {
		return true, nil
	}
	existing, err := n.nodesSyncer.getMachineForNodeFromCache(node)
	if err != nil {
		return false, err
	}
	if existing == nil {
		return true, nil
	}
	if existing.Annotations[annotationName] == "" {
		existing = existing.DeepCopy()
		if existing.Annotations == nil {
			existing.Annotations = make(map[string]string)
		}
		existing.Annotations[annotationName] = "true"
		if _, err = n.nodesSyncer.machines.Update(existing); err != nil {
			return false, err
		}
		return true, nil
	}
	toUpdate, err := n.nodesSyncer.convertNodeToMachine(node, existing)
	if err != nil {
		return false, err
	}

	// update only when nothing changed
	if objectsAreEqual(existing, toUpdate) {
		return false, nil
	}
	return true, nil
}

func (m *nodesSyncer) sync(key string, _ *apimgmtv3.Node) (runtime.Object, error) {
	if key == fmt.Sprintf("%s/%s", m.clusterNamespace, AllNodeKey) {
		return nil, m.reconcileAll()
	}
	return nil, nil
}

func (m *nodesSyncer) updateNodeAndNode(node *corev1.Node, obj *apimgmtv3.Node) (*corev1.Node, *apimgmtv3.Node, error) {
	node, err := m.nodeClient.Update(node)
	if err != nil {
		// Return apimgmtv3.Node is nil because it hasn't been persisted and therefor out of sync with cache
		// so we don't want to return it from the handler
		return node, nil, err
	}

	obj, err = m.machines.Update(obj)
	if err != nil {
		// Same logic as above, but more so that we don't know the state of this object
		return node, nil, err
	}

	return node, obj, nil
}

func (m *nodesSyncer) updateLabels(node *corev1.Node, obj *apimgmtv3.Node, nodePlan rketypes.RKEConfigNodePlan) (*corev1.Node, *apimgmtv3.Node, error) {
	finalMap, changed := computeDelta(node.Labels, nodePlan.Labels, obj.Spec.MetadataUpdate.Labels, onlyKubeLabels)
	if !changed {
		return node, obj, nil
	}

	node, obj = node.DeepCopy(), obj.DeepCopy()
	if obj.Status.NodeConfig != nil {
		planValues, changed := computePlanDelta(obj.Status.NodeConfig.Labels, obj.Spec.MetadataUpdate.Labels)
		if changed {
			obj.Status.NodeConfig.Labels = planValues
		}
	}

	node.Labels = finalMap

	obj.Spec.MetadataUpdate.Labels = apimgmtv3.MapDelta{}

	return m.updateNodeAndNode(node, obj)
}

// For any key that already exist in the plan, we should update or delete. For others, do not touch the plan.
func computePlanDelta(planValues map[string]string, delta apimgmtv3.MapDelta) (map[string]string, bool) {
	update := false
	for k, v := range delta.Add {
		if planValues[k] != "" {
			update = true
			planValues[k] = v
		}
	}

	for k := range delta.Delete {
		if planValues[k] != "" {
			update = true
			delete(planValues, k)
		}
	}
	return planValues, update

}

func (m *nodesSyncer) updateAnnotations(node *corev1.Node, obj *apimgmtv3.Node, nodePlan rketypes.RKEConfigNodePlan) (*corev1.Node, *apimgmtv3.Node, error) {
	finalMap, changed := computeDelta(node.Annotations, nodePlan.Annotations, obj.Spec.MetadataUpdate.Annotations, allowAllPolicy)
	if !changed {
		return node, obj, nil
	}

	node, obj = node.DeepCopy(), obj.DeepCopy()
	node.Annotations = finalMap
	obj.Spec.MetadataUpdate.Annotations = apimgmtv3.MapDelta{}

	return m.updateNodeAndNode(node, obj)
}

func (m *nodesSyncer) syncLabels(_ string, obj *apimgmtv3.Node) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}

	node, err := nodehelper.GetNodeForMachine(obj, m.nodeLister)
	if err != nil || node == nil {
		return nil, err
	}

	nodePlan, err := m.getNodePlan(obj)
	if err != nil {
		return obj, err
	}

	node, obj, err = m.updateLabels(node, obj, nodePlan)
	if err != nil {
		return obj, err
	}

	_, obj, err = m.updateAnnotations(node, obj, nodePlan)
	return obj, err
}

func onlyKubeLabels(key string) bool {
	return strings.Contains(key, "kubernetes.io")
}

func allowAllPolicy(_ string) bool {
	return true
}

// computeDelta will return the final updated map to apply and a boolean indicating whether there are changes to be applied.
// If the boolean is false, the caller need not take any action as the data is already in sync.
func computeDelta(currentState map[string]string, planValues map[string]string, delta apimgmtv3.MapDelta, canChangeValue canChangeValuePolicy) (map[string]string, bool) {
	result := map[string]string{}
	changed := false

	// copy map
	for k, v := range currentState {
		result[k] = v
	}

	for k, v := range planValues {
		currentValue, ok := result[k]
		// if the key is missing or the value is allowed to be changed
		if !ok || (currentValue != v && canChangeValue(k)) {
			result[k] = v
			changed = true
		}
	}

	for k, v := range delta.Add {
		if result[k] != v {
			changed = true
			result[k] = v
		}
	}

	for k := range delta.Delete {
		if _, ok := result[k]; ok {
			delete(result, k)
			changed = true
		}
	}

	return result, changed
}

func (m *nodesSyncer) getNodePlan(node *apimgmtv3.Node) (rketypes.RKEConfigNodePlan, error) {
	cluster, err := m.clusterLister.Get("", node.Namespace)
	if err != nil {
		return rketypes.RKEConfigNodePlan{}, err
	}

	if cluster.Status.Driver != apimgmtv3.ClusterDriverRKE || cluster.Status.AppliedSpec.RancherKubernetesEngineConfig == nil {
		return rketypes.RKEConfigNodePlan{}, nil
	}

	if node.Status.NodeConfig == nil {
		return rketypes.RKEConfigNodePlan{}, nil
	}

	dockerInfo, err := librke.GetDockerInfo(node)
	if err != nil {
		return rketypes.RKEConfigNodePlan{}, err
	}

	hostAddress := node.Status.NodeConfig.Address
	hostDockerInfo := dockerInfo[hostAddress]

	svcOptions, err := m.getServiceOptions(cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.Version, hostDockerInfo.OSType)
	if err != nil {
		return rketypes.RKEConfigNodePlan{}, err
	}

	appliedSpec, err := assemblers.AssembleRKEConfigSpec(cluster, *cluster.Status.AppliedSpec.DeepCopy(), m.secretLister)
	if err != nil {
		return rketypes.RKEConfigNodePlan{}, err
	}
	plan, err := librke.New().GeneratePlan(context.Background(), appliedSpec.RancherKubernetesEngineConfig, dockerInfo, svcOptions)
	if err != nil {
		return rketypes.RKEConfigNodePlan{}, err
	}

	for _, nodePlan := range plan.Nodes {
		if nodePlan.Address == node.Status.NodeConfig.Address {
			return nodePlan, nil
		}
	}

	return rketypes.RKEConfigNodePlan{}, nil
}

func (m *nodesSyncer) getServiceOptions(k8sVersion string, osType string) (map[string]interface{}, error) {
	data := map[string]interface{}{}
	svcOptions, err := kd.GetRKEK8sServiceOptions(k8sVersion, m.serviceOptionsLister, m.serviceOptions, m.sysImagesLister, m.sysImages, kd.Linux)
	if err != nil {
		logrus.Errorf("getK8sServiceOptions: k8sVersion %s [%v]", k8sVersion, err)
		return data, err
	}
	if svcOptions != nil {
		data["k8s-service-options"] = svcOptions
	}
	if osType == "windows" {
		svcOptionsWindows, err := kd.GetRKEK8sServiceOptions(k8sVersion, m.serviceOptionsLister, m.serviceOptions, m.sysImagesLister, m.sysImages, kd.Windows)
		if err != nil {
			logrus.Errorf("getK8sServiceOptionsWindows: k8sVersion %s [%v]", k8sVersion, err)
			return data, err
		}
		if svcOptionsWindows != nil {
			data["k8s-windows-service-options"] = svcOptionsWindows
		}
	}
	return data, nil
}

func (m *nodesSyncer) reconcileAll() error {
	// skip reconcile if we are restoring from backup,
	// this is needed to avoid adding/deleting replaced nodes that might be in the
	// snapshots before the cluster restore/reconcile is complete
	if m.clusterNamespace != "local" { // we don't check for local cluster
		if restoring, err := m.isClusterRestoring(); restoring {
			return nil
		} else if err != nil {
			return err
		}
	}

	nodes, err := m.nodeLister.List("", labels.NewSelector())
	if err != nil {
		return err
	}

	nodeMap := make(map[string]*corev1.Node, len(nodes))
	for _, node := range nodes {
		nodeMap[node.Name] = node
	}

	machines, err := m.machineLister.List(m.clusterNamespace, labels.NewSelector())
	if err != nil {
		return err
	}
	machineMap := make(map[string]*apimgmtv3.Node)
	toDelete := make(map[string]*apimgmtv3.Node)
	for _, machine := range machines {
		node, err := nodehelper.GetNodeForMachine(machine, m.nodeLister)
		if err != nil {
			return err
		}
		if node == nil {
			logrus.Debugf("Failed to get node for machine [%s], preparing to delete", machine.Name)
			toDelete[machine.Name] = machine
			continue
		}
		machineMap[node.Name] = machine
	}
	nodeCache := &NodeCache{}

	// reconcile machines for existing nodes
	for name, node := range nodeMap {
		machine := machineMap[name]
		err = m.reconcileNodeForNode(machine, node, nodeCache)
		if err != nil {
			return err
		}
	}
	// run the logic for machine to remove
	for name, machine := range machineMap {
		if _, ok := nodeMap[name]; !ok {
			if err = m.removeNode(machine); err != nil {
				return err
			}
		}
	}

	for _, machine := range toDelete {
		if err = m.removeNode(machine); err != nil {
			return err
		}
	}

	return nil
}

func (m *nodesSyncer) reconcileNodeForNode(machine *apimgmtv3.Node, node *corev1.Node, nodeCache *NodeCache) error {
	if machine == nil {
		return m.createNode(node, nodeCache)
	}
	return m.updateNode(machine, node)
}

func (m *nodesSyncer) removeNode(machine *apimgmtv3.Node) error {
	if machine.DeletionTimestamp != nil {
		return nil
	}
	if machine.Annotations == nil {
		return nil
	}

	if _, ok := machine.Annotations[annotationName]; !ok {
		return nil
	}

	err := m.machines.Delete(machine.ObjectMeta.Name, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to delete machine [%s]", machine.Name)
	}
	logrus.Infof("Deleted cluster node %s [%s]", machine.Name, machine.Status.NodeName)
	return nil
}

func (m *nodesSyncer) updateNode(existing *apimgmtv3.Node, node *corev1.Node) error {
	toUpdate, err := m.convertNodeToMachine(node, existing)
	if err != nil {
		return err
	}
	// update only when nothing changed
	if objectsAreEqual(existing, toUpdate) {
		return nil
	}
	logrus.Debugf("Updating machine for node [%s]", node.Name)
	_, err = m.machines.Update(toUpdate)
	if err != nil {
		return errors.Wrapf(err, "Failed to update machine for node [%s]", node.Name)
	}
	logrus.Debugf("Updated machine for node [%s]", node.Name)
	return nil
}

func (m *nodesSyncer) createNode(node *corev1.Node, nodeCache *NodeCache) error {
	// respect user defined name or label
	if nodehelper.IgnoreNode(node.Name, node.Labels) {
		logrus.Debugf("Skipping apimgmtv3.Node creation for [%v] node", node.Name)
		return nil
	}

	// try to get machine from api, in case cache didn't get the update
	existing, err := m.getMachineForNode(node, nodeCache)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	machine, err := m.convertNodeToMachine(node, existing)
	if err != nil {
		return err
	}

	if machine.Annotations == nil {
		machine.Annotations = make(map[string]string)
	}
	machine.Annotations[annotationName] = "true"

	_, err = m.machines.Create(machine)
	if err != nil {
		return errors.Wrapf(err, "Failed to create machine for node [%s]", node.Name)
	}
	logrus.Infof("Created machine for node [%s]", node.Name)
	return nil
}

func (m *nodesSyncer) getMachineForNodeFromCache(node *corev1.Node) (*apimgmtv3.Node, error) {
	return nodehelper.GetMachineForNode(node, m.clusterNamespace, m.machineLister)
}

type NodeCache struct {
	all    []*apimgmtv3.Node
	byName map[string][]*apimgmtv3.Node
}

func (n *NodeCache) Add(machine *apimgmtv3.Node) {
	if n.byName == nil {
		n.byName = map[string][]*apimgmtv3.Node{}
	}
	if name, ok := machine.Labels[nodehelper.LabelNodeName]; ok {
		n.byName[name] = append(n.byName[name], machine)
	}
	n.all = append(n.all, machine)
}

func (m *nodesSyncer) getMachineForNode(node *corev1.Node, nodeCache *NodeCache) (*apimgmtv3.Node, error) {
	if len(nodeCache.all) == 0 {
		machines, err := m.machines.List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range machines.Items {
			nodeCache.Add(&machines.Items[i])
		}
	}

	machines := nodeCache.byName[node.Name]
	if len(machines) == 0 {
		machines = nodeCache.all
	}

	for _, machine := range machines {
		if machine.Namespace == m.clusterNamespace {
			if nodehelper.IsNodeForNode(node, machine) {
				return machine, nil
			}
		}
	}

	return nil, nil
}

func resetConditions(machine *apimgmtv3.Node) *apimgmtv3.Node {
	if machine.Status.InternalNodeStatus.Conditions == nil {
		return machine
	}
	updated := machine.DeepCopy()
	var toUpdateConds []corev1.NodeCondition
	for _, cond := range machine.Status.InternalNodeStatus.Conditions {
		toUpdateCond := cond.DeepCopy()
		toUpdateCond.LastHeartbeatTime = metav1.Time{}
		toUpdateCond.LastTransitionTime = metav1.Time{}
		toUpdateConds = append(toUpdateConds, *toUpdateCond)
	}
	sort.Slice(toUpdateConds, func(i, j int) bool {
		return toUpdateConds[i].Type < toUpdateConds[j].Type
	})
	updated.Status.InternalNodeStatus.Conditions = toUpdateConds
	return updated
}

func objectsAreEqual(existing *apimgmtv3.Node, toUpdate *apimgmtv3.Node) bool {
	// we are updating spec and status only, so compare them
	toUpdateToCompare := resetConditions(toUpdate)
	existingToCompare := resetConditions(existing)
	statusEqual := statusEqualTest(toUpdateToCompare.Status.InternalNodeStatus, existingToCompare.Status.InternalNodeStatus)
	conditionsEqual := reflect.DeepEqual(toUpdateToCompare.Status.Conditions, existing.Status.Conditions)
	labelsEqual := reflect.DeepEqual(toUpdateToCompare.Status.NodeLabels, existing.Status.NodeLabels) && reflect.DeepEqual(toUpdateToCompare.Labels, existing.Labels)
	annotationsEqual := reflect.DeepEqual(toUpdateToCompare.Status.NodeAnnotations, existing.Status.NodeAnnotations)
	specEqual := reflect.DeepEqual(toUpdateToCompare.Spec.InternalNodeSpec, existingToCompare.Spec.InternalNodeSpec)
	nodeNameEqual := toUpdateToCompare.Status.NodeName == existingToCompare.Status.NodeName
	requestsEqual := isEqual(toUpdateToCompare.Status.Requested, existingToCompare.Status.Requested)
	limitsEqual := isEqual(toUpdateToCompare.Status.Limits, existingToCompare.Status.Limits)
	rolesEqual := toUpdateToCompare.Spec.Worker == existingToCompare.Spec.Worker && toUpdateToCompare.Spec.Etcd == existingToCompare.Spec.Etcd &&
		toUpdateToCompare.Spec.ControlPlane == existingToCompare.Spec.ControlPlane

	retVal := statusEqual && conditionsEqual && specEqual && nodeNameEqual && labelsEqual && annotationsEqual && requestsEqual && limitsEqual && rolesEqual
	if !retVal {
		logrus.Debugf("ObjectsAreEqualResults for %s: statusEqual: %t conditionsEqual: %t specEqual: %t"+
			" nodeNameEqual: %t labelsEqual: %t annotationsEqual: %t requestsEqual: %t limitsEqual: %t rolesEqual: %t",
			toUpdate.Name, statusEqual, conditionsEqual, specEqual, nodeNameEqual, labelsEqual, annotationsEqual, requestsEqual, limitsEqual, rolesEqual)
	}
	return retVal
}

func statusEqualTest(proposed, existing corev1.NodeStatus) bool {
	// Tests here should validate that fields of the corev1.NodeStatus type are equal for Rancher's purposes.
	// The Images field lists would be equal if they contain the same data regardless of order. Using reflect.DeepEqual
	// does not see lists with the same content but different order as equal, and would cause
	// Rancher to update the resource unnecessarily. So if Images becomes a field we need to validate we need to add
	// a custom method to validate the equality.
	//
	// Rancher doesn't use the following NodeStatus data, so for time savings we are skipping, but in the future these tests
	// should be added here.
	//
	// SKIP:
	//   - Images           # Do not use reflect.DeepEquals on this field for testing.
	//   - NodeInfo
	//   - DaemonEndpoints
	//   - Phase

	// Capacity
	if !reflect.DeepEqual(proposed.Capacity, existing.Capacity) {
		logrus.Debugf("Changes in Capacity, proposed %#v, existing: %#v", proposed.Capacity, existing.Capacity)
		return false
	}

	// Allocatable
	if !reflect.DeepEqual(proposed.Allocatable, existing.Allocatable) {
		logrus.Debugf("Changes in Allocatable, proposed %#v, existing: %#v", proposed.Allocatable, existing.Allocatable)
		return false
	}

	// Conditions
	if !reflect.DeepEqual(proposed.Conditions, existing.Conditions) {
		logrus.Debugf("Changes in Conditions, proposed %#v, existing: %#v", proposed.Conditions, existing.Conditions)
		return false
	}

	// Addresses
	if !reflect.DeepEqual(proposed.Addresses, existing.Addresses) {
		logrus.Debugf("Changes in Addresses, proposed %#v, existing: %#v", proposed.Addresses, existing.Addresses)
		return false
	}

	// Volumes in use (This test might prove to be an issue if order is not returned consistently.)
	if !reflect.DeepEqual(proposed.VolumesInUse, existing.VolumesInUse) {
		logrus.Debugf("Changes in VolumesInUse, proposed %#v, existing: %#v", proposed.VolumesInUse, existing.VolumesInUse)
		return false
	}

	// VolumesAttached (This test might prove to cause excessive updates if order is not returned consistently.)
	if !reflect.DeepEqual(proposed.VolumesAttached, existing.VolumesAttached) {
		logrus.Debugf("Changes in VolumesAttached, proposed %#v, existing: %#v", proposed.VolumesAttached, existing.VolumesAttached)
		return false
	}

	// Compare Node's Kubernetes versions
	if proposed.NodeInfo.KubeletVersion != existing.NodeInfo.KubeletVersion ||
		proposed.NodeInfo.KubeProxyVersion != existing.NodeInfo.KubeProxyVersion ||
		proposed.NodeInfo.ContainerRuntimeVersion != existing.NodeInfo.ContainerRuntimeVersion {
		logrus.Debugf("Changes in KubernetesInfo, "+
			"KubeletVersion proposed %#v, existing: %#v"+
			"KubeProxyVersion proposed %#v, existing: %#v"+
			"ContainerRuntimeVersion proposed %#v, existing: %#v",
			proposed.NodeInfo.KubeletVersion, existing.NodeInfo.KubeletVersion,
			proposed.NodeInfo.KubeProxyVersion, existing.NodeInfo.KubeProxyVersion,
			proposed.NodeInfo.ContainerRuntimeVersion, existing.NodeInfo.ContainerRuntimeVersion)
		return false
	}

	return true
}

func cleanStatus(machine *apimgmtv3.Node) {
	var conditions []corev1.NodeCondition
	for _, condition := range machine.Status.InternalNodeStatus.Conditions {
		if condition.Type == "Ready" {
			conditions = append(conditions, condition)
			readyCondition := apimgmtv3.NodeCondition{
				Type:               cond.Cond(condition.Type),
				Status:             condition.Status,
				LastTransitionTime: condition.LastTransitionTime.String(),
				LastUpdateTime:     condition.LastHeartbeatTime.String(),
				Reason:             condition.Reason,
				Message:            condition.Message,
			}
			var exists bool
			for i, cond := range machine.Status.Conditions {
				if cond.Type == "Ready" {
					exists = true
					machine.Status.Conditions[i] = readyCondition
					break
				}
			}
			if !exists {
				machine.Status.Conditions = append(machine.Status.Conditions, readyCondition)
			}
		}
	}

	machine.Status.InternalNodeStatus.Config = nil
	machine.Status.InternalNodeStatus.VolumesInUse = nil
	machine.Status.InternalNodeStatus.VolumesAttached = nil
	machine.Status.InternalNodeStatus.Images = nil
	machine.Status.InternalNodeStatus.Conditions = conditions

	annoMap := make(map[string]string, len(machine.Status.NodeAnnotations))
	for key, val := range machine.Status.NodeAnnotations {
		if key == podresources.LimitsAnnotation || key == podresources.RequestsAnnotation {
			continue
		}
		annoMap[key] = val
	}

	machine.Status.NodeAnnotations = annoMap
}

func getResourceList(annotation string, node *corev1.Node) corev1.ResourceList {
	val := node.Annotations[annotation]
	if val == "" {
		return nil
	}
	result := corev1.ResourceList{}
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return corev1.ResourceList{}
	}
	return result
}

func (m *nodesSyncer) convertNodeToMachine(node *corev1.Node, existing *apimgmtv3.Node) (*apimgmtv3.Node, error) {
	var machine *apimgmtv3.Node
	if existing == nil {
		machine = &apimgmtv3.Node{
			Spec:   apimgmtv3.NodeSpec{},
			Status: apimgmtv3.NodeStatus{},
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "machine-"},
		}
		machine.Namespace = m.clusterNamespace
		machine.Status.Requested = make(map[corev1.ResourceName]resource.Quantity)
		machine.Status.Limits = make(map[corev1.ResourceName]resource.Quantity)
		machine.Spec.InternalNodeSpec = *node.Spec.DeepCopy()
		machine.Status.InternalNodeStatus = *node.Status.DeepCopy()
		machine.Spec.RequestedHostname = node.Name
	} else {
		machine = existing.DeepCopy()
		machine.Spec.InternalNodeSpec = *node.Spec.DeepCopy()
		machine.Status.InternalNodeStatus = *node.Status.DeepCopy()
	}

	requests := getResourceList(podresources.RequestsAnnotation, node)
	limits := getResourceList(podresources.LimitsAnnotation, node)
	if machine.Status.Requested == nil {
		machine.Status.Requested = corev1.ResourceList{}
	}
	if machine.Status.Limits == nil {
		machine.Status.Limits = corev1.ResourceList{}
	}

	for name, quantity := range requests {
		machine.Status.Requested[name] = quantity
	}
	for name, quantity := range limits {
		machine.Status.Limits[name] = quantity
	}
	machine.Status.NodeLabels = make(map[string]string, len(node.Labels))
	for k, v := range node.Labels {
		machine.Status.NodeLabels[k] = v
	}
	determineNodeRoles(machine)
	machine.Status.NodeAnnotations = make(map[string]string, len(node.Annotations))
	for k, v := range node.Annotations {
		machine.Status.NodeAnnotations[k] = v
	}
	machine.Status.NodeName = node.Name
	machine.APIVersion = "management.cattle.io/v3"
	machine.Kind = "Node"
	if machine.Labels == nil {
		machine.Labels = map[string]string{}
	}
	machine.Labels[nodehelper.LabelNodeName] = node.Name
	cleanStatus(machine)
	apimgmtv3.NodeConditionRegistered.True(machine)
	apimgmtv3.NodeConditionRegistered.Message(machine, "registered with kubernetes")
	return machine, nil
}

func isEqual(data1 map[corev1.ResourceName]resource.Quantity, data2 map[corev1.ResourceName]resource.Quantity) bool {
	if len(data1) == 0 && len(data2) == 0 {
		return true
	}
	if data1 == nil || data2 == nil {
		return false
	}
	for key, value := range data1 {
		if _, exists := data2[key]; !exists {
			return false
		}
		value2 := data2[key]
		if value.Value() != value2.Value() {
			return false
		}
	}
	return true
}

func (m *nodesSyncer) isClusterRestoring() (bool, error) {
	cluster, err := m.clusterLister.Get("", m.clusterNamespace)
	if err != nil {
		return false, err
	}
	if cluster.Spec.RancherKubernetesEngineConfig != nil &&
		cluster.Spec.RancherKubernetesEngineConfig.Restore.Restore {
		return true, nil
	}
	if cluster.Status.Driver == "imported" {
		return false, nil
	}
	if strings.HasPrefix(cluster.Name, "c-m-") {
		provCluster, err := m.provClusterCache.Get(cluster.Spec.FleetWorkspaceName, cluster.Spec.DisplayName)
		if err != nil {
			return false, err
		}
		capiCluster, err := m.capiClusterCache.Get(provCluster.Namespace, provCluster.Name)
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if capiCluster.Spec.ControlPlaneRef.Kind != "RKEControlPlane" || capiCluster.Spec.ControlPlaneRef.APIVersion != "rke.cattle.io/v1" {
			return false, nil
		}
		controlplane, err := m.rkeControlPlaneCache.Get(capiCluster.Spec.ControlPlaneRef.Namespace, capiCluster.Spec.ControlPlaneRef.Name)
		if err != nil {
			return false, err
		}
		phase := controlplane.Status.ETCDSnapshotRestorePhase
		return phase != "" && phase != rkev1.ETCDSnapshotPhaseFinished && phase != rkev1.ETCDSnapshotPhaseFailed, nil
	}

	return false, nil
}

func determineNodeRoles(machine *apimgmtv3.Node) {
	if machine.Status.NodeLabels != nil {
		_, etcd := machine.Status.NodeLabels["node-role.kubernetes.io/etcd"]
		_, control := machine.Status.NodeLabels["node-role.kubernetes.io/controlplane"]
		_, master := machine.Status.NodeLabels["node-role.kubernetes.io/master"]
		_, worker := machine.Status.NodeLabels["node-role.kubernetes.io/worker"]
		machine.Spec.Etcd = etcd
		machine.Spec.Worker = worker
		machine.Spec.ControlPlane = control || master
		if !machine.Spec.Worker && !machine.Spec.ControlPlane && !machine.Spec.Etcd {
			machine.Spec.Worker = true
		}
	}
}
