package nodesyncer

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/management/compose/common"
	"github.com/rancher/rancher/pkg/librke"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/systemaccount"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	AllNodeKey     = "_machine_all_"
	annotationName = "management.cattle.io/nodesyncer"
	apiUpdate      = "management.cattle.io/apiUpdate"
)

var apiUpdateMap = map[string]string{apiUpdate: "true"}

type nodeSyncer struct {
	machines         v3.NodeInterface
	clusterNamespace string
	nodesSyncer      *nodesSyncer
}

type nodesSyncer struct {
	machines         v3.NodeInterface
	machineLister    v3.NodeLister
	nodeLister       v1.NodeLister
	nodeClient       v1.NodeInterface
	podLister        v1.PodLister
	clusterNamespace string
	clusterLister    v3.ClusterLister
}

type nodeDrain struct {
	userManager          user.Manager
	tokenClient          v3.TokenInterface
	userClient           v3.UserInterface
	kubeConfigGetter     common.KubeConfigGetter
	clusterName          string
	systemAccountManager *systemaccount.Manager
	clusterLister        v3.ClusterLister
	machines             v3.NodeInterface
	ctx                  context.Context
	nodesToContext       map[string]context.CancelFunc
}

type canChangeValuePolicy func(key string) bool

func Register(ctx context.Context, cluster *config.UserContext, kubeConfigGetter common.KubeConfigGetter) {
	m := &nodesSyncer{
		clusterNamespace: cluster.ClusterName,
		machines:         cluster.Management.Management.Nodes(cluster.ClusterName),
		machineLister:    cluster.Management.Management.Nodes(cluster.ClusterName).Controller().Lister(),
		nodeLister:       cluster.Core.Nodes("").Controller().Lister(),
		nodeClient:       cluster.Core.Nodes(""),
		podLister:        cluster.Core.Pods("").Controller().Lister(),
		clusterLister:    cluster.Management.Management.Clusters("").Controller().Lister(),
	}

	n := &nodeSyncer{
		clusterNamespace: cluster.ClusterName,
		machines:         cluster.Management.Management.Nodes(cluster.ClusterName),
		nodesSyncer:      m,
	}

	d := &nodeDrain{
		userManager:          cluster.Management.UserManager,
		tokenClient:          cluster.Management.Management.Tokens(""),
		userClient:           cluster.Management.Management.Users(""),
		kubeConfigGetter:     kubeConfigGetter,
		clusterName:          cluster.ClusterName,
		systemAccountManager: systemaccount.NewManager(cluster.Management),
		clusterLister:        cluster.Management.Management.Clusters("").Controller().Lister(),
		machines:             cluster.Management.Management.Nodes(cluster.ClusterName),
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

func (n *nodeSyncer) needUpdate(key string, node *corev1.Node) (bool, error) {
	if node == nil || node.DeletionTimestamp != nil {
		return true, nil
	}
	existing, err := n.nodesSyncer.getMachineForNode(node, true)
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
	nodeToPodMap, err := n.nodesSyncer.getNonTerminatedPods()
	if err != nil {
		return false, err
	}
	toUpdate, err := n.nodesSyncer.convertNodeToMachine(node, existing, nodeToPodMap)
	if err != nil {
		return false, err
	}

	// update only when nothing changed
	if objectsAreEqual(existing, toUpdate) {
		return false, nil
	}
	return true, nil
}

func (m *nodesSyncer) sync(key string, machine *v3.Node) (runtime.Object, error) {
	if key == fmt.Sprintf("%s/%s", m.clusterNamespace, AllNodeKey) {
		return nil, m.reconcileAll()
	}
	return nil, nil
}

func (m *nodesSyncer) updateNodeAndNode(node *corev1.Node, obj *v3.Node) (*corev1.Node, *v3.Node, error) {
	node, err := m.nodeClient.Update(node)
	if err != nil {
		// Return v3.Node is nil because it hasn't been persisted and therefor out of sync with cache
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

func (m *nodesSyncer) updateLabels(node *corev1.Node, obj *v3.Node, nodePlan v3.RKEConfigNodePlan) (*corev1.Node, *v3.Node, error) {
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

	obj.Spec.MetadataUpdate.Labels = v3.MapDelta{}

	return m.updateNodeAndNode(node, obj)
}

// For any key that already exist in the plan, we should update or delete. For others, do not touch the plan.
func computePlanDelta(planValues map[string]string, delta v3.MapDelta) (map[string]string, bool) {
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

func (m *nodesSyncer) updateAnnotations(node *corev1.Node, obj *v3.Node, nodePlan v3.RKEConfigNodePlan) (*corev1.Node, *v3.Node, error) {
	finalMap, changed := computeDelta(node.Annotations, nodePlan.Annotations, obj.Spec.MetadataUpdate.Annotations, nil)
	if !changed {
		return node, obj, nil
	}

	node, obj = node.DeepCopy(), obj.DeepCopy()
	node.Annotations = finalMap
	obj.Spec.MetadataUpdate.Annotations = v3.MapDelta{}

	return m.updateNodeAndNode(node, obj)
}

func (m *nodesSyncer) syncLabels(key string, obj *v3.Node) (runtime.Object, error) {
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

// computeDelta will return the final updated map to apply and a boolean indicating whether there are changes to be applied.
// If the boolean is false, the caller need not take any action as the data is already in sync.
func computeDelta(currentState map[string]string, planValues map[string]string, delta v3.MapDelta, canChangeValue canChangeValuePolicy) (map[string]string, bool) {
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

func (m *nodesSyncer) getNodePlan(node *v3.Node) (v3.RKEConfigNodePlan, error) {
	cluster, err := m.clusterLister.Get("", node.Namespace)
	if err != nil {
		return v3.RKEConfigNodePlan{}, err
	}

	if cluster.Status.Driver != v3.ClusterDriverRKE || cluster.Status.AppliedSpec.RancherKubernetesEngineConfig == nil {
		return v3.RKEConfigNodePlan{}, nil
	}

	if node.Status.NodeConfig == nil {
		return v3.RKEConfigNodePlan{}, nil
	}

	dockerInfo, err := librke.GetDockerInfo(node)
	if err != nil {
		return v3.RKEConfigNodePlan{}, err
	}

	plan, err := librke.New().GeneratePlan(context.Background(), cluster.Status.AppliedSpec.RancherKubernetesEngineConfig, dockerInfo, map[string]interface{}{})
	if err != nil {
		return v3.RKEConfigNodePlan{}, err
	}

	for _, nodePlan := range plan.Nodes {
		if nodePlan.Address == node.Status.NodeConfig.Address {
			return nodePlan, nil
		}
	}

	return v3.RKEConfigNodePlan{}, nil
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

	nodeMap := make(map[string]*corev1.Node)
	for _, node := range nodes {
		nodeMap[node.Name] = node
	}

	machines, err := m.machineLister.List(m.clusterNamespace, labels.NewSelector())
	if err != nil {
		return err
	}
	machineMap := make(map[string]*v3.Node)
	toDelete := make(map[string]*v3.Node)
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
	nodeToPodMap, err := m.getNonTerminatedPods()
	if err != nil {
		return err
	}

	// reconcile machines for existing nodes
	for name, node := range nodeMap {
		machine := machineMap[name]
		err = m.reconcileNodeForNode(machine, node, nodeToPodMap)
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

func (m *nodesSyncer) reconcileNodeForNode(machine *v3.Node, node *corev1.Node, pods map[string][]*corev1.Pod) error {
	if machine == nil {
		return m.createNode(node, pods)
	}
	return m.updateNode(machine, node, pods)
}

func (m *nodesSyncer) removeNode(machine *v3.Node) error {
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

func (m *nodesSyncer) updateNode(existing *v3.Node, node *corev1.Node, pods map[string][]*corev1.Pod) error {
	toUpdate, err := m.convertNodeToMachine(node, existing, pods)
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

func (m *nodesSyncer) createNode(node *corev1.Node, pods map[string][]*corev1.Pod) error {
	// respect user defined name or label
	if nodehelper.IgnoreNode(node.Name, node.Labels) {
		logrus.Debugf("Skipping v3.node creation for [%v] node", node.Name)
		return nil
	}

	// try to get machine from api, in case cache didn't get the update
	existing, err := m.getMachineForNode(node, false)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	machine, err := m.convertNodeToMachine(node, existing, pods)
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

func (m *nodesSyncer) getMachineForNode(node *corev1.Node, cache bool) (*v3.Node, error) {
	if cache {
		return nodehelper.GetMachineForNode(node, m.clusterNamespace, m.machineLister)
	}

	labelsSearchSet := labels.Set{nodehelper.LabelNodeName: node.Name}
	machines, err := m.machines.List(metav1.ListOptions{LabelSelector: labelsSearchSet.AsSelector().String()})
	if err != nil {
		return nil, err
	}
	if len(machines.Items) == 0 {
		machines, err = m.machines.List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
	}

	for _, machine := range machines.Items {
		if machine.Namespace == m.clusterNamespace {
			if nodehelper.IsNodeForNode(node, &machine) {
				return &machine, nil
			}
		}
	}

	return nil, nil
}

func resetConditions(machine *v3.Node) *v3.Node {
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

func objectsAreEqual(existing *v3.Node, toUpdate *v3.Node) bool {
	// we are updating spec and status only, so compare them
	toUpdateToCompare := resetConditions(toUpdate)
	existingToCompare := resetConditions(existing)
	statusEqual := statusEqualTest(toUpdateToCompare.Status.InternalNodeStatus, existingToCompare.Status.InternalNodeStatus)
	labelsEqual := reflect.DeepEqual(toUpdateToCompare.Status.NodeLabels, existing.Status.NodeLabels) && reflect.DeepEqual(toUpdateToCompare.Labels, existing.Labels)
	annotationsEqual := reflect.DeepEqual(toUpdateToCompare.Status.NodeAnnotations, existing.Status.NodeAnnotations)
	specEqual := reflect.DeepEqual(toUpdateToCompare.Spec.InternalNodeSpec, existingToCompare.Spec.InternalNodeSpec)
	nodeNameEqual := toUpdateToCompare.Status.NodeName == existingToCompare.Status.NodeName
	requestsEqual := isEqual(toUpdateToCompare.Status.Requested, existingToCompare.Status.Requested)
	limitsEqual := isEqual(toUpdateToCompare.Status.Limits, existingToCompare.Status.Limits)
	rolesEqual := toUpdateToCompare.Spec.Worker == existingToCompare.Spec.Worker && toUpdateToCompare.Spec.Etcd == existingToCompare.Spec.Etcd &&
		toUpdateToCompare.Spec.ControlPlane == existingToCompare.Spec.ControlPlane

	retVal := statusEqual && specEqual && nodeNameEqual && labelsEqual && annotationsEqual && requestsEqual && limitsEqual && rolesEqual
	if !retVal {
		logrus.Debugf("ObjectsAreEqualResults for %s: statusEqual: %t specEqual: %t"+
			" nodeNameEqual: %t labelsEqual: %t annotationsEqual: %t requestsEqual: %t limitsEqual: %t rolesEqual: %t",
			toUpdate.Name, statusEqual, specEqual, nodeNameEqual, labelsEqual, annotationsEqual, requestsEqual, limitsEqual, rolesEqual)
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

func (m *nodesSyncer) convertNodeToMachine(node *corev1.Node, existing *v3.Node, pods map[string][]*corev1.Pod) (*v3.Node, error) {
	var machine *v3.Node
	if existing == nil {
		machine = &v3.Node{
			Spec:   v3.NodeSpec{},
			Status: v3.NodeStatus{},
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

	requests, limits := aggregateRequestAndLimitsForNode(pods[node.Name])
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

	if node.Labels != nil {
		_, etcd := node.Labels["node-role.kubernetes.io/etcd"]
		machine.Spec.Etcd = etcd
		_, control := node.Labels["node-role.kubernetes.io/controlplane"]
		_, master := node.Labels["node-role.kubernetes.io/master"]
		if control || master {
			machine.Spec.ControlPlane = true
		}
		_, worker := node.Labels["node-role.kubernetes.io/worker"]
		machine.Spec.Worker = worker
		if !machine.Spec.Worker && !machine.Spec.ControlPlane && !machine.Spec.Etcd {
			machine.Spec.Worker = true
		}
	}

	machine.Status.NodeAnnotations = node.Annotations
	machine.Status.NodeLabels = node.Labels
	machine.Status.NodeName = node.Name
	machine.APIVersion = "management.cattle.io/v3"
	machine.Kind = "Node"
	if machine.Labels == nil {
		machine.Labels = map[string]string{}
	}
	machine.Labels[nodehelper.LabelNodeName] = node.Name
	v3.NodeConditionRegistered.True(machine)
	v3.NodeConditionRegistered.Message(machine, "registered with kubernetes")
	return machine, nil
}

func (m *nodesSyncer) getNonTerminatedPods() (map[string][]*corev1.Pod, error) {
	pods := make(map[string][]*corev1.Pod)
	fromCache, err := m.podLister.List("", labels.NewSelector())
	if err != nil {
		return pods, err
	}

	for _, pod := range fromCache {
		if pod.Spec.NodeName == "" {
			continue
		}
		// kubectl uses this cache to filter out the pods
		if pod.Status.Phase == "Succeeded" || pod.Status.Phase == "Failed" {
			continue
		}
		var nodePods []*corev1.Pod
		if fromMap, ok := pods[pod.Spec.NodeName]; ok {
			nodePods = fromMap
		}
		nodePods = append(nodePods, pod)
		pods[pod.Spec.NodeName] = nodePods
	}
	return pods, nil
}

func aggregateRequestAndLimitsForNode(pods []*corev1.Pod) (map[corev1.ResourceName]resource.Quantity, map[corev1.ResourceName]resource.Quantity) {
	requests, limits := map[corev1.ResourceName]resource.Quantity{}, map[corev1.ResourceName]resource.Quantity{}
	podsData := make(map[string]map[string]map[corev1.ResourceName]resource.Quantity)
	if pods != nil {
		//podName -> req/limit -> data
		for _, pod := range pods {
			podsData[pod.Name] = make(map[string]map[corev1.ResourceName]resource.Quantity)
			requests, limits := getPodData(pod)
			podsData[pod.Name]["requests"] = requests
			podsData[pod.Name]["limits"] = limits
		}
		requests[corev1.ResourcePods] = *resource.NewQuantity(int64(len(pods)), resource.DecimalSI)
	}
	for _, podData := range podsData {
		podRequests, podLimits := podData["requests"], podData["limits"]
		addMap(podRequests, requests)
		addMap(podLimits, limits)
	}
	return requests, limits
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

func getPodData(pod *corev1.Pod) (map[corev1.ResourceName]resource.Quantity, map[corev1.ResourceName]resource.Quantity) {
	requests, limits := map[corev1.ResourceName]resource.Quantity{}, map[corev1.ResourceName]resource.Quantity{}
	for _, container := range pod.Spec.Containers {
		addMap(container.Resources.Requests, requests)
		addMap(container.Resources.Limits, limits)
	}

	for _, container := range pod.Spec.InitContainers {
		addMapForInit(container.Resources.Requests, requests)
		addMapForInit(container.Resources.Limits, limits)
	}
	return requests, limits
}

func addMap(data1 map[corev1.ResourceName]resource.Quantity, data2 map[corev1.ResourceName]resource.Quantity) {
	for name, quantity := range data1 {
		if value, ok := data2[name]; !ok {
			data2[name] = quantity.DeepCopy()
		} else {
			value.Add(quantity)
			data2[name] = value
		}
	}
}

func addMapForInit(data1 map[corev1.ResourceName]resource.Quantity, data2 map[corev1.ResourceName]resource.Quantity) {
	for name, quantity := range data1 {
		value, ok := data2[name]
		if !ok {
			data2[name] = quantity.DeepCopy()
			continue
		}
		if quantity.Cmp(value) > 0 {
			data2[name] = quantity.DeepCopy()
		}
	}
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
	return false, nil
}
