package nodepool

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rke/services"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	nameRegexp = regexp.MustCompile("^(.*?)([0-9]+)$")
)

const (
	ReconcileAnnotation  = "nodepool.cattle.io/reconcile"
	DeleteNodeAnnotation = "nodepool.cattle.io/delete-node"
)

type Controller struct {
	NodePoolController v3.NodePoolController
	NodePoolLister     v3.NodePoolLister
	NodePools          v3.NodePoolInterface
	NodeLister         v3.NodeLister
	Nodes              v3.NodeInterface
	mutex              sync.RWMutex
	syncmap            map[string]bool
}

func Register(ctx context.Context, management *config.ManagementContext) {
	p := &Controller{
		NodePoolController: management.Management.NodePools("").Controller(),
		NodePoolLister:     management.Management.NodePools("").Controller().Lister(),
		NodePools:          management.Management.NodePools(""),
		NodeLister:         management.Management.Nodes("").Controller().Lister(),
		Nodes:              management.Management.Nodes(""),
		syncmap:            make(map[string]bool),
	}

	// Add handlers
	p.NodePools.AddLifecycle(ctx, "nodepool-provisioner", p)
	management.Management.Nodes("").AddHandler(ctx, "nodepool-provisioner", p.machineChanged)
}

func (c *Controller) Create(nodePool *v3.NodePool) (runtime.Object, error) {
	return nodePool, nil
}

func (c *Controller) needsReconcile(nodePool *v3.NodePool, nodes []*v3.Node) bool {
	changed, _, err := c.createOrCheckNodes(nodePool, nodes, true)
	if err != nil {
		logrus.Debugf("[nodepoool] error checking pool for reconciliation: %s", err)
	}

	return changed
}

func (c *Controller) reconcile(nodePool *v3.NodePool, nodes []*v3.Node) {
	_, qty, err := c.createOrCheckNodes(nodePool, nodes, false)
	if err != nil {
		logrus.Errorf("[nodepool] reconcile error, create or check nodes: %s", err)
	}

	if qty != nodePool.Spec.Quantity {
		nodePool.Spec.Quantity = qty
	}

	_, err = c.setReconcileAnnotation(nodePool, "")
	if err != nil {
		logrus.Errorf("[nodepool] error updating reconcile annotation to updated: %s", err)
	}
}

func (c *Controller) Updated(nodePool *v3.NodePool) (runtime.Object, error) {
	obj, err := v32.NodePoolConditionUpdated.Do(nodePool, func() (runtime.Object, error) {
		anno, _ := nodePool.Annotations[ReconcileAnnotation]
		if anno == "" {
			nodes, err := c.NodeLister.List(nodePool.Namespace, labels.Everything())
			if err != nil {
				return nodePool, err
			}
			go c.deleteBadNodes(nodes)
			if c.needsReconcile(nodePool, nodes) {
				logrus.Debugf("[nodepool] reconcile needed for %s", nodePool.Name)
				np, err := c.setReconcileAnnotation(nodePool, "updating")
				if err != nil {
					return nodePool, err
				}
				go c.reconcile(np, nodes)
				return nil, nil
			}
		} else if strings.HasPrefix(anno, "updating/") {
			// gate updating the node pool to every 20s
			pieces := strings.Split(anno, "/")
			t, err := time.Parse(time.RFC3339, pieces[1])
			if err != nil || int(time.Since(t)/time.Second) > 20 {
				nodePool.Annotations[ReconcileAnnotation] = ""
				return c.NodePools.Update(nodePool)
			}
			// go routine is already running to update the cluster so wait
			return nil, nil
		}

		// pool doesn't need to reconcile, nothing to do
		return nil, nil
	})

	return obj.(*v3.NodePool), err
}

func (c *Controller) Remove(nodePool *v3.NodePool) (runtime.Object, error) {
	logrus.Infof("[nodepool] deleting %s", nodePool.Name)

	logrus.Debugf("[nodepool] listing nodes for pool %s", nodePool.Name)
	nodeList, err := c.Nodes.ListNamespaced(nodePool.Namespace, metav1.ListOptions{})
	if err != nil {
		return nodePool, err
	}

	for _, node := range nodeList.Items {
		_, nodePoolName := ref.Parse(node.Spec.NodePoolName)
		if nodePoolName != nodePool.Name {
			continue
		}

		if err := c.deleteNode(&node, 0); err != nil {
			return nodePool, err
		}
	}

	return nodePool, nil
}

func (c *Controller) setReconcileAnnotation(nodePool *v3.NodePool, anno string) (*v3.NodePool, error) {
	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   1,
		Jitter:   0,
		Steps:    6,
	}

	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		newPool, err := c.NodePools.GetNamespaced(nodePool.Namespace, nodePool.Name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return false, err
			}
			return false, nil
		}

		if anno == "updating" {
			// Add a timestamp for comparison since this anno was added
			anno = anno + "/" + time.Now().Format(time.RFC3339)
		}

		newPool.Annotations[ReconcileAnnotation] = anno
		newPool.Spec.Quantity = nodePool.Spec.Quantity // incase the pool size changed during reconcile
		newPool, err = c.NodePools.Update(newPool)
		if err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		nodePool = newPool
		return true, nil
	})
	if err != nil {
		return nodePool, fmt.Errorf("[nodepool] Failed to update nodePool annotation [%s]: %v", nodePool.Name, err)
	}
	return nodePool, nil
}

func (c *Controller) setDeleteNodeAnnotation(node *v3.Node) (*v3.Node, error) {
	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   1,
		Jitter:   0,
		Steps:    6,
	}

	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		newNode, err := c.Nodes.GetNamespaced(node.Namespace, node.Name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return false, err
			}
			return false, nil
		}

		newNode.Annotations[DeleteNodeAnnotation] = "true"
		newNode, err = c.Nodes.Update(newNode)
		if err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		node = newNode
		return true, nil
	})
	if err != nil {
		return node, fmt.Errorf("[nodeprool] Failed to update node delete annotation [%s]: %v", node.Name, err)
	}
	return node, nil
}

func (c *Controller) machineChanged(key string, machine *v3.Node) (runtime.Object, error) {
	if machine == nil {
		nps, err := c.NodePoolLister.List("", labels.Everything())
		if err != nil {
			return nil, err
		}
		for _, np := range nps {
			c.NodePoolController.Enqueue(np.Namespace, np.Name)
		}
	} else if machine.Spec.NodePoolName != "" {
		ns, name := ref.Parse(machine.Spec.NodePoolName)
		c.NodePoolController.Enqueue(ns, name)
	}

	return nil, nil
}

func (c *Controller) createNode(name string, nodePool *v3.NodePool, simulate bool) (*v3.Node, error) {
	newNode := &v3.Node{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "m-",
			Namespace:    nodePool.Namespace,
			Labels:       nodePool.Labels,
			Annotations:  nodePool.Annotations,
		},
		Spec: v32.NodeSpec{
			Etcd:              nodePool.Spec.Etcd,
			ControlPlane:      nodePool.Spec.ControlPlane,
			Worker:            nodePool.Spec.Worker,
			NodeTemplateName:  nodePool.Spec.NodeTemplateName,
			NodePoolName:      ref.Ref(nodePool),
			RequestedHostname: name,
		},
	}

	delete(newNode.Annotations, ReconcileAnnotation)
	if simulate {
		return newNode, nil
	}

	n, err := c.Nodes.Create(newNode)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("[nodepool] node created %s", n.Name)
	return n, nil
}

func (c *Controller) deleteNode(node *v3.Node, duration time.Duration) error {
	newNode, err := c.setDeleteNodeAnnotation(node)
	if err != nil {
		return err
	}

	if duration > time.Duration(0) {
		go func() {
			time.Sleep(duration)
			c.deleteNodeBackoffAndRetry(newNode)
		}()
		return nil
	}

	return c.deleteNodeBackoffAndRetry(newNode)
}

func (c *Controller) deleteNodeBackoffAndRetry(node *v3.Node) error {
	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   1,
		Jitter:   0,
		Steps:    6,
	}

	logrus.Debugf("[nodepool] attempting to delete node %s", node.Name)
	f := metav1.DeletePropagationBackground
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		err := c.Nodes.DeleteNamespaced(node.Namespace, node.Name, &metav1.DeleteOptions{
			PropagationPolicy: &f,
		})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		}
		return true, nil
	})
}

func parsePrefix(fullPrefix string) (prefix string, minLength, start int) {
	m := nameRegexp.FindStringSubmatch(fullPrefix)
	if len(m) == 0 {
		return fullPrefix, 1, 1
	}
	prefix = m[1]
	start, _ = strconv.Atoi(m[2])
	return prefix, len(m[2]), start
}

func (c *Controller) deleteBadNodes(nodes []*v3.Node) {
	for _, node := range nodes {
		if anno, ok := node.Annotations[DeleteNodeAnnotation]; ok && anno == "true" {
			return
		}
		if v32.NodeConditionProvisioned.IsFalse(node) || v32.NodeConditionInitialized.IsFalse(node) || v32.NodeConditionConfigSaved.IsFalse(node) {
			logrus.Debugf("[nodepool] bad node found: %s", node.Name)
			_ = c.deleteNode(node, 2*time.Minute)
		}
	}
}

func (c *Controller) createOrCheckNodes(nodePool *v3.NodePool, allNodes []*v3.Node, simulate bool) (bool, int, error) {
	var (
		err                 error
		byName              = map[string]*v3.Node{}
		changed             = false
		nodes               []*v3.Node
		deleteNotReadyAfter = nodePool.Spec.DeleteNotReadyAfterSecs * time.Second
	)

	quantity := nodePool.Spec.Quantity
	for _, node := range allNodes {
		byName[node.Spec.RequestedHostname] = node

		_, nodePoolName := ref.Parse(node.Spec.NodePoolName)
		if nodePoolName != nodePool.Name || node.DeletionTimestamp != nil {
			continue
		}

		if node.Spec.ScaledownTime != "" {
			logrus.Debugf("[nodepool] scaledown time detected for %s: %s and now it is %s",
				node.Name, node.Spec.ScaledownTime, time.Now().Format(time.RFC3339))
			scaledown, err := time.Parse(time.RFC3339, node.Spec.ScaledownTime)
			if err != nil {
				logrus.Errorf("[nodepool] failed to parse scaledown time, is it in RFC3339? %s: %s", node.Spec.ScaledownTime, err)
			} else {
				if scaledown.Before(time.Now()) {
					changed = true
					if !simulate {
						logrus.Debugf("[nodepool] scaling down, removing node %s", node.Name)
						if err = c.deleteNode(node, 0); err != nil {
							return false, quantity, err
						}
					}
					quantity--
					continue
				}

				// scaledown happening in the future, enqueue after to check again later
				c.NodePoolController.EnqueueAfter(nodePool.Namespace, nodePool.Name, scaledown.Sub(time.Now()))
			}
		}

		// remove unreachable node with the unreachable taint & status of Ready being Unknown
		q := getUnreachableTaint(node.Spec.InternalNodeSpec.Taints)
		if q != nil && deleteNotReadyAfter > 0 {
			changed = true
			if isNodeReadyUnknown(node) && !simulate {
				start := q.TimeAdded.Time
				if time.Since(start) > deleteNotReadyAfter {
					err = c.deleteNode(node, 0)
					if err != nil {
						return false, quantity, err
					}
				} else {
					c.mutex.Lock()
					nodeid := node.Namespace + ":" + node.Name
					if _, ok := c.syncmap[nodeid]; !ok {
						c.syncmap[nodeid] = true
						go c.requeue(deleteNotReadyAfter, nodePool, node)
					}
					c.mutex.Unlock()
				}
			}
		}
		nodes = append(nodes, node)
	}

	if quantity < 0 {
		quantity = 0
	}

	prefix, minLength, start := parsePrefix(nodePool.Spec.HostnamePrefix)
	for i := start; len(nodes) < quantity; i++ {
		ia := strconv.Itoa(i)
		name := prefix + ia
		if len(ia) < minLength {
			name = fmt.Sprintf("%s%0"+strconv.Itoa(minLength)+"d", prefix, i)
		}

		if byName[name] != nil {
			continue
		}

		changed = true
		newNode, err := c.createNode(name, nodePool, simulate)
		if err != nil {
			return false, quantity, err
		}

		byName[newNode.Spec.RequestedHostname] = newNode
		nodes = append(nodes, newNode)
	}

	for len(nodes) > quantity {
		sort.Sort(byHostname(nodes))

		toDelete := nodes[len(nodes)-1]

		changed = true
		if !simulate {
			c.deleteNode(toDelete, 0)
		}

		nodes = nodes[:len(nodes)-1]
		delete(byName, toDelete.Spec.RequestedHostname)
	}

	for _, n := range nodes {
		if needRoleUpdate(n, nodePool) {
			changed = true
			_, err := c.updateNodeRoles(n, nodePool, simulate)
			if err != nil {
				return false, quantity, err
			}
		}
	}

	return changed, quantity, nil
}

func needRoleUpdate(node *v3.Node, nodePool *v3.NodePool) bool {
	if node.Status.NodeConfig == nil {
		return false
	}
	if len(node.Status.NodeConfig.Role) == 0 && !nodePool.Spec.Worker {
		return true
	}

	nodeRolesMap := map[string]bool{}
	nodeRolesMap[services.ETCDRole] = false
	nodeRolesMap[services.ControlRole] = false
	nodeRolesMap[services.WorkerRole] = false

	for _, role := range node.Status.NodeConfig.Role {
		switch r := role; r {
		case services.ETCDRole:
			nodeRolesMap[services.ETCDRole] = true
		case services.ControlRole:
			nodeRolesMap[services.ControlRole] = true
		case services.WorkerRole:
			nodeRolesMap[services.WorkerRole] = true
		}
	}
	poolRolesMap := map[string]bool{}
	poolRolesMap[services.ETCDRole] = nodePool.Spec.Etcd
	poolRolesMap[services.ControlRole] = nodePool.Spec.ControlPlane
	poolRolesMap[services.WorkerRole] = nodePool.Spec.Worker

	r := !reflect.DeepEqual(nodeRolesMap, poolRolesMap)
	if r {
		logrus.Debugf("[nodepool] updating machine [%s] roles: nodepoolRoles: {%+v} node roles: {%+v}", node.Name, poolRolesMap, nodeRolesMap)
	}
	return r
}

func (c *Controller) updateNodeRoles(existing *v3.Node, nodePool *v3.NodePool, simulate bool) (*v3.Node, error) {
	toUpdate := existing.DeepCopy()
	var newRoles []string

	if nodePool.Spec.ControlPlane {
		newRoles = append(newRoles, "controlplane")
	}
	if nodePool.Spec.Etcd {
		newRoles = append(newRoles, "etcd")
	}
	if nodePool.Spec.Worker {
		newRoles = append(newRoles, "worker")
	}

	if len(newRoles) == 0 {
		newRoles = []string{"worker"}
	}

	toUpdate.Status.NodeConfig.Role = newRoles
	if simulate {
		return toUpdate, nil
	}
	return c.Nodes.Update(toUpdate)
}

// requeue checks every 5 seconds if the node is still unreachable with one goroutine per node
func (c *Controller) requeue(timeout time.Duration, np *v3.NodePool, node *v3.Node) {

	t := getUnreachableTaint(node.Spec.InternalNodeSpec.Taints)
	for t != nil {
		time.Sleep(5 * time.Second)
		exist, err := c.NodeLister.Get(node.Namespace, node.Name)
		if err != nil {
			break
		}
		t = getUnreachableTaint(exist.Spec.InternalNodeSpec.Taints)
		if t != nil && time.Since(t.TimeAdded.Time) > timeout {
			logrus.Debugf("Enqueue nodepool controller: %s %s", np.Namespace, np.Name)
			c.NodePoolController.Enqueue(np.Namespace, np.Name)
			break
		}
	}
	c.mutex.Lock()
	delete(c.syncmap, node.Namespace+":"+node.Name)
	c.mutex.Unlock()
}

func getUnreachableTaint(taints []v1.Taint) *v1.Taint {
	for _, taint := range taints {
		if taint.Key == v1.TaintNodeUnreachable {
			return &taint
		}
	}
	return nil
}

// IsNodeReady returns true if a node Ready condition is Unknown; false otherwise.
func isNodeReadyUnknown(node *v3.Node) bool {
	for _, c := range node.Status.InternalNodeStatus.Conditions {
		if c.Type == v1.NodeReady {
			return c.Status == v1.ConditionUnknown
		}
	}
	return false
}
