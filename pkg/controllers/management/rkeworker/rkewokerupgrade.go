package rkeworker

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/docker/docker/api/types"
	hash "github.com/mitchellh/hashstructure"
	"github.com/rancher/norman/types/convert"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/librke"
	"github.com/rancher/rancher/pkg/randomtoken"
	"github.com/rancher/rancher/pkg/ticker"
	rkeservices "github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type upgradeHandler struct {
	nodesLister          v3.NodeLister
	nodes                v3.NodeInterface
	clusters             v3.ClusterInterface
	clusterLister        v3.ClusterLister
	serviceOptionsLister v3.RKEK8sServiceOptionLister
	serviceOptions       v3.RKEK8sServiceOptionInterface
	sysImagesLister      v3.RKEK8sSystemImageLister
	sysImages            v3.RKEK8sSystemImageInterface
	ctx                  context.Context
	mutex                sync.RWMutex
}

var (
	interval       = 30 * time.Second
	clusterMapData = map[string]context.CancelFunc{}
	clusterLock    = sync.Mutex{}
)

func Register(ctx context.Context, mgmt *config.ManagementContext) {
	logrus.Infof("registering management upgrade handler for rke worker nodes")
	uh := upgradeHandler{
		nodesLister:          mgmt.Management.Nodes("").Controller().Lister(),
		nodes:                mgmt.Management.Nodes(""),
		clusters:             mgmt.Management.Clusters(""),
		clusterLister:        mgmt.Management.Clusters("").Controller().Lister(),
		serviceOptionsLister: mgmt.Management.RKEK8sServiceOptions("").Controller().Lister(),
		serviceOptions:       mgmt.Management.RKEK8sServiceOptions(""),
		sysImagesLister:      mgmt.Management.RKEK8sSystemImages("").Controller().Lister(),
		sysImages:            mgmt.Management.RKEK8sSystemImages(""),
		ctx:                  ctx,
	}

	mgmt.Management.Clusters("").Controller().AddHandler(ctx, "rkeworkerupgradehandler", uh.Sync)
	//
	//	clusterMapData = map[string]context.CancelFunc{}
}

func (uh *upgradeHandler) Sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		// what if currToken is set for server?
		return cluster, nil
	}

	if cluster.Name == "local" || cluster.Status.AppliedSpec.RancherKubernetesEngineConfig == nil {
		return cluster, nil
	}

	logrus.Infof("received sync for cluster [%s]", cluster.Name)

	nodes, err := uh.nodesLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return nil, err
	}

	nodes, notReady := filterNodes(nodes)

	logrus.Infof("nodes %v notReady %v", len(nodes), notReady)

	nodeUpgradeStatus := cluster.Status.NodeUpgradeStatus

	if nodeUpgradeStatus == nil {
		// set LastAppliedToken for the first time when requiredNum nodes (total - maxUnavailable) are ready
		return uh.initToken(cluster)
	}

	oldHash := cluster.Status.NodeUpgradeStatus.LastAppliedToken
	currHash, err := uh.getClusterHash(cluster)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster hash for [%s]: %v", cluster.Name, err)
	}
	if oldHash == currHash {
		return cluster, nil
	}

	if currHash != cluster.Status.NodeUpgradeStatus.CurrentToken {
		clusterObj := cluster.DeepCopy()
		clusterObj.Status.NodeUpgradeStatus.CurrentToken = currHash
		clusterObj.Status.NodeUpgradeStatus.Nodes = map[string]map[string]string{}
		clusterObj.Status.NodeUpgradeStatus.Nodes[currHash] = map[string]string{}

		cluster, err = uh.clusters.Update(clusterObj)
		if err != nil {
			return nil, err
		}
		logrus.Infof("token changed for cluster [%s] old: %s new: %s", cluster.Name, oldHash, currHash)
	}

	nodes, err = uh.nodesLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return nil, err
	}

	nodes, notReady = filterNodes(nodes)

	logrus.Infof("nodes %v notReady %v", len(nodes), notReady)

	maxAllowed, err := getNum(&cluster.Spec.RancherKubernetesEngineConfig.NodeUpgradeStrategy.RollingUpdate.MaxUnavailable,
		len(nodes), false)
	if err != nil {
		return nil, err
	}

	if notReady >= maxAllowed {
		return nil, fmt.Errorf("not enough nodes not upgrade for cluster [%s]: notReady %v maxUnavailable %v", cluster.Name, notReady, maxAllowed)
	}

	var (
		//errgrp          errgroup.Group
		upgrading, done int
	)
	toProcessMap, toPrepareMap, doneMap := map[string]bool{}, map[string]bool{}, map[string]bool{}

	//nodesQueue := util.GetObjectQueue(nodes)
	//
	//for w := 0; w < WorkerThreads; w++ {
	//	errgrp.Go(func() error {
	for _, node := range nodes {
		if v3.NodeConditionUpdated.IsTrue(node) && currHash == v3.NodeConditionUpdated.GetReason(node) {
			done += 1
			doneMap[node.Name] = true
			continue
		}
		if (v3.NodeConditionUpdated.IsUnknown(node) && currHash == v3.NodeConditionUpdated.GetReason(node)) ||
			node.Spec.DesiredNodeUnschedulable == "drain" {
			upgrading += 1
			continue
		}
		if v3.NodeConditionDrained.IsTrue(node) {
			upgrading += 1
			toProcessMap[node.Name] = true
			continue
		}
		toPrepareMap[node.Name] = true
	}
	//	return nil
	//})
	//}

	//if err := errgrp.Wait(); err != nil {
	//	logrus.Errorf("error trying error group %v", err)
	//}

	logrus.Infof("workerNodeInfo for cluster [%s]: maxAllowed %v upgrading %v toProcess %v", cluster.Name, maxAllowed, upgrading, toProcessMap)

	/*
		toPrepareMap: nodes to drain/cordon
		toProcessMap: nodes to update with currHash after drained
		doneMap: nodes successfully updated with currHash

		clusterMap: current nodes status stored on cluster.Status
	*/
	cluster, clusterMap, err := uh.syncNodeStatus(cluster, currHash, maxAllowed, doneMap, toPrepareMap, toProcessMap)
	if err != nil {
		return nil, err
	}

	if clusterMap == nil {
		return cluster, nil
	}

	upgradeStrategy := cluster.Spec.RancherKubernetesEngineConfig.NodeUpgradeStrategy.RollingUpdate
	toDrain := upgradeStrategy.Drain
	nodeDrainInput := upgradeStrategy.DrainInput
	if toDrain && nodeDrainInput == nil {
		nodeDrainInput = &v3.NodeDrainInput{
			Force:            true,
			IgnoreDaemonSets: true,
			DeleteLocalData:  true,
		}
	}

	logrus.Infof("clusterMap %v", clusterMap)

	go uh.updateNodes(clusterMap, toDrain, nodeDrainInput, currHash, cluster.Name)

	if done == len(nodes) && cluster.Status.NodeUpgradeStatus.LastAppliedToken != currHash {
		logrus.Infof("setting lastAppliedToken %s", currHash)
		clusterCopy := cluster.DeepCopy()
		clusterCopy.Status.NodeUpgradeStatus.LastAppliedToken = currHash
		clusterCopy.Status.NodeUpgradeStatus.CurrentToken = ""
		clusterCopy.Status.NodeUpgradeStatus.Nodes[currHash] = map[string]string{}
		cluster, err = uh.clusters.Update(clusterCopy)
		if err != nil {
			return nil, err
		}
	}

	logrus.Infof("returning from sync [%s]", cluster.Name)
	return cluster, nil
}

func (uh *upgradeHandler) updateNodes(clusterMap map[string]string, toDrain bool, nodeDrainInput *v3.NodeDrainInput, currHash string, cName string) {
	for name, state := range clusterMap {
		node, err := uh.nodesLister.Get(cName, name)
		if err != nil {
			logrus.Errorf("error finding node [%s] for cluster [%s]: %v", name, cName, err)
			continue
		}
		if state == "draining" {
			if err := uh.prepareNode(node, toDrain, nodeDrainInput, currHash); err != nil {
				logrus.Infof("error updating node %s", node.Name) //uh.clusters.Controller().Enqueue("", cName)
				break
			}
		} else if state == "updating" {
			if err := uh.processNode(node, toDrain, nodeDrainInput, currHash); err != nil {
				logrus.Infof("error updating node %s", node.Name) //uh.clusters.Controller().Enqueue("", cName)
				break
			}
		}
	}
}

func (uh *upgradeHandler) prepareNode(node *v3.Node, toDrain bool, nodeDrainInput *v3.NodeDrainInput, currHash string) error {
	var nodeCopy *v3.Node

	if !toDrain && node.Spec.DesiredNodeUnschedulable != "true" {
		nodeCopy = node.DeepCopy()
		nodeCopy.Spec.DesiredNodeUnschedulable = "true"
		v3.NodeConditionUpdated.Unknown(nodeCopy)
		v3.NodeConditionUpdated.Reason(nodeCopy, currHash)
		v3.NodeConditionUpdated.Message(nodeCopy, "updating!")
	}

	if toDrain && node.Spec.DesiredNodeUnschedulable != "drain" {
		nodeCopy = node.DeepCopy()
		nodeCopy.Spec.DesiredNodeUnschedulable = "drain"
		nodeCopy.Spec.NodeDrainInput = nodeDrainInput
	}

	if nodeCopy == nil {
		return nil
	}

	if nodeCopy != nil {
		logrus.Infof("updating with nodeCopy param %s %s", node.Name, nodeCopy.Spec.DesiredNodeUnschedulable)

		_, err := uh.nodes.Update(nodeCopy)
		if err != nil {
			return err
		}
	}
	return nil
}

func (uh *upgradeHandler) processNode(node *v3.Node, toDrain bool, nodeDrainInput *v3.NodeDrainInput, currHash string) error {
	logrus.Infof("processNode %s", node.Name)

	if !toDrain && !node.Spec.InternalNodeSpec.Unschedulable {
		logrus.Errorf("why node is in processing %s %v", node.Name, node.Spec.InternalNodeSpec.Unschedulable)
		return nil
	}

	if toDrain && !v3.NodeConditionDrained.IsTrue(node) {
		logrus.Errorf("why node is in processing %s for drain %v", node.Name, v3.NodeConditionDrained.GetStatus(node))
		return nil
	}

	nodeEqual := v3.NodeConditionUpdated.IsUnknown(node) && v3.NodeConditionUpdated.GetReason(node) == currHash
	logrus.Infof("nodeEqual %s %v", node.Name, nodeEqual)

	if !nodeEqual {
		logrus.Infof("updating node %s for upgrade", node.Name)
		nodeCopy := node.DeepCopy()
		v3.NodeConditionUpdated.Unknown(nodeCopy)
		v3.NodeConditionUpdated.Reason(nodeCopy, currHash)
		v3.NodeConditionUpdated.Message(nodeCopy, "updating!")

		_, err := uh.nodes.Update(nodeCopy)
		if err != nil {
			return err

		}
	}
	return nil
}

func workerOnly(roles []string) bool {
	worker := false
	for _, role := range roles {
		if role == rkeservices.ETCDRole {
			return false
		}
		if role == rkeservices.ControlRole {
			return false
		}
		if role == rkeservices.WorkerRole {
			worker = true
		}
	}
	return worker
}

func (uh *upgradeHandler) initToken(cluster *v3.Cluster) (*v3.Cluster, error) {
	nodes, err := uh.nodesLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return nil, err
	}

	nodes, _ = filterNodes(nodes)

	requiredForInit, err := getNum(&cluster.Spec.RancherKubernetesEngineConfig.NodeUpgradeStrategy.RollingUpdate.MaxUnavailable,
		len(nodes), true)
	if err != nil {
		return nil, err
	}

	uh.mutex.Lock()
	defer uh.mutex.Unlock()
	if requiredForInit <= len(nodes) {
		logrus.Infof("%v worker nodes ready for cluster [%s], setting token", requiredForInit, cluster.Name)
		clusterHash, err := uh.getClusterHash(cluster)
		if err != nil {
			return nil, fmt.Errorf("error getting cluster hash for [%s]: %v", cluster.Name, err)
		}
		clusterObj := cluster.DeepCopy()
		clusterObj.Status.NodeUpgradeStatus = &v3.NodeUpgradeStatus{
			LastAppliedToken: clusterHash,
		}
		cluster, err = uh.clusters.Update(clusterObj)
		if err != nil {
			return nil, err
		}
	}
	return cluster, nil
}

func (uh *upgradeHandler) syncNodeStatus(cluster *v3.Cluster, currHash string, maxAllowed int,
	doneMap, toPrepareMap, toProcessMap map[string]bool) (*v3.Cluster, map[string]string, error) {

	uh.mutex.Lock()
	defer uh.mutex.Unlock()

	cluster, err := uh.clusterLister.Get("", cluster.Name)
	if err != nil {
		return nil, nil, err
	}

	nodeUpgradeStatus := cluster.Status.NodeUpgradeStatus.DeepCopy()
	clusterMap := nodeUpgradeStatus.Nodes[currHash]

	logrus.Infof("original clusterMap %v %v", clusterMap, clusterMap == nil)

	if clusterMap == nil {
		return cluster, nil, nil
	}

	for name := range clusterMap {
		if doneMap[name] {
			delete(clusterMap, name)
		}
	}

	for name := range toProcessMap {
		clusterMap[name] = "updating"
	}

	upgrading := len(clusterMap)

	for name := range toPrepareMap {
		if upgrading == maxAllowed {
			break
		}
		clusterMap[name] = "draining"
		upgrading += 1
	}

	logrus.Infof("sync worker node status for [%s]: %v %v", cluster.Name, clusterMap, cluster.Status.NodeUpgradeStatus.Nodes[currHash])

	if !reflect.DeepEqual(clusterMap, cluster.Status.NodeUpgradeStatus.Nodes[currHash]) {
		clusterObj := cluster.DeepCopy()
		clusterObj.Status.NodeUpgradeStatus.Nodes[currHash] = clusterMap
		logrus.Debugf("sync worker node status on cluster [%s]: old %v new %v", cluster.Status.NodeUpgradeStatus.Nodes[currHash], clusterMap)

		cluster, err = uh.clusters.Update(clusterObj)
		if err != nil {
			return nil, nil, err
		}
	}

	return cluster, clusterMap, nil
}

func (uh *upgradeHandler) getClusterHash(cluster *v3.Cluster) (string, error) {
	// check for node OS, if windowsPreferredCluster, we should also check for windows, else just empty would get default values

	return "upgr102", nil
	osType := ""
	svcOptions, err := uh.getServiceOptions(cluster.Spec.RancherKubernetesEngineConfig.Version, osType)
	if err != nil {
		return "", err
	}

	hostDockerInfo := types.Info{OSType: osType}
	clusterPlan, err := librke.New().GenerateClusterPlan(uh.ctx, cluster.Status.AppliedSpec.RancherKubernetesEngineConfig, hostDockerInfo, svcOptions)
	if err != nil {
		return "", err
	}

	clusterHash, err := ClusterHash(clusterPlan.Processes)
	if err != nil {
		return "", err
	}

	return clusterHash, nil
}

func (uh *upgradeHandler) getServiceOptions(k8sVersion, osType string) (map[string]interface{}, error) {
	data := map[string]interface{}{}
	svcOptions, err := kd.GetRKEK8sServiceOptions(k8sVersion, uh.serviceOptionsLister, uh.serviceOptions, uh.sysImagesLister, uh.sysImages, kd.Linux)
	if err != nil {
		logrus.Errorf("getK8sServiceOptions: k8sVersion %s [%v]", k8sVersion, err)
		return data, err
	}
	if svcOptions != nil {
		data["k8s-service-options"] = svcOptions
	}
	if osType == "windows" {
		svcOptionsWindows, err := kd.GetRKEK8sServiceOptions(k8sVersion, uh.serviceOptionsLister, uh.serviceOptions, uh.sysImagesLister, uh.sysImages, kd.Windows)
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

func ClusterHash(processes map[string]v3.RKEProcess) (string, error) {
	cHash, err := hash.Hash(processes, nil)
	if err != nil {
		return "", err
	}
	return convert.ToString(cHash), nil
}

func getNum(maxUnavailable *intstr.IntOrString, nodes int, init bool) (int, error) {
	maxAllowed, err := intstr.GetValueFromIntOrPercent(maxUnavailable, nodes, false)
	if err != nil {
		logrus.Infof("getMaxAllowed err %v", err)
		return 0, err
	}
	if init {
		if nodes > maxAllowed {
			return nodes - maxAllowed, nil
		}
	}
	if maxAllowed >= 1 {
		return maxAllowed, nil
	}
	return 1, nil
}

func filterNodes(nodes []*v3.Node) ([]*v3.Node, int) {
	var filtered []*v3.Node
	//var errgrp errgroup.Group
	//
	notReady := 0
	//
	//nodesQueue := util.GetObjectQueue(nodes)
	//
	//for w := 0; w < WorkerThreads; w++ {
	//	errgrp.Go(func() error {
	for _, node := range nodes {
		//node := queueNode.(*v3.Node)
		if node.Status.NodeConfig == nil || !workerOnly(node.Status.NodeConfig.Role) || node.DeletionTimestamp != nil {
			continue
		}

		nodeGood := v3.NodeConditionRegistered.IsTrue(node) && v3.NodeConditionProvisioned.IsTrue(node) &&
			!v3.NodeConditionReady.IsUnknown(node)

		//logrus.Infof("nodeConditionReady %v", node.Status.InternalNodeStatus.Conditions)
		if !nodeGood {
			notReady += 1
			logrus.Infof("node is not ready %s", node.Name)
			continue
		}

		filtered = append(filtered, node)
	}
	//return nil
	//	})
	//}
	//
	//if err := errgrp.Wait(); err != nil {
	//	logrus.Errorf("error trying error group %v", err)
	//}

	return filtered, notReady
}

func (uh *upgradeHandler) startTicker(ctx context.Context, cName string) {
	defer clusterLock.Unlock()
	clusterLock.Lock()

	_, ok := clusterMapData[cName]
	if ok {
		return
	}

	cctx, cancel := context.WithCancel(ctx)
	clusterMapData[cName] = cancel

	go uh.start(cctx, cName)
}

func (uh *upgradeHandler) start(ctx context.Context, cName string) {
	for range ticker.Context(ctx, interval) {
		logrus.Infof("upgradeHandler ticker enqueue [%s]", cName)

		cluster, err := uh.clusterLister.Get("", cName)
		if err != nil {
			logrus.Errorf("cluster errror %s", cName)
		}

		clusterCopy := cluster.DeepCopy()

		tok, err := randomtoken.Generate()
		if err != nil {
			logrus.Errorf("TOken err %s", tok)
		}
		clusterCopy.Annotations["KINARA"] = tok

		if _, err := uh.clusters.Update(clusterCopy); err != nil {
			logrus.Errorf("updatess err %v", err)
		}

		//uh.clusters.Controller().Enqueue("", cName)
	}
}

func stopTicker(cName string) {
	defer clusterLock.Unlock()
	clusterLock.Lock()

	cancelFunc, ok := clusterMapData[cName]
	if !ok {
		return
	}

	logrus.Infof("upgradeHandler stopping ticker for cluster [%s]", cName)

	if cancelFunc != nil {
		cancelFunc()
	}

	delete(clusterMapData, cName)
	return
}
