package nodepool

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rke/services"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	nameRegexp       = regexp.MustCompile("^(.*?)([0-9]+)$")
	unReachableTaint = v1.Taint{
		Key:    "node.kubernetes.io/unreachable",
		Effect: "NoExecute",
	}
	falseValue = false
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

func (c *Controller) Updated(nodePool *v3.NodePool) (runtime.Object, error) {
	obj, err := v32.NodePoolConditionUpdated.Do(nodePool, func() (runtime.Object, error) {
		return nodePool, c.reconcile(nodePool)
	})
	return obj.(*v3.NodePool), err
}

func (c *Controller) Remove(nodePool *v3.NodePool) (runtime.Object, error) {
	logrus.Infof("Deleting nodePool [%s]", nodePool.Name)

	allNodes, err := c.nodes(nodePool, false)
	if err != nil {
		return nodePool, err
	}

	for _, node := range allNodes {
		_, nodePoolName := ref.Parse(node.Spec.NodePoolName)
		if nodePoolName != nodePool.Name {
			continue
		}

		err := c.deleteNode(node, time.Duration(0))
		if err != nil {
			return nil, err
		}
	}

	return nodePool, nil
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

	if simulate {
		return newNode, nil
	}

	return c.Nodes.Create(newNode)
}

func (c *Controller) deleteNode(node *v3.Node, duration time.Duration) error {
	f := metav1.DeletePropagationBackground

	if duration > time.Duration(0) {
		go func() {
			time.Sleep(duration)
			c.Nodes.DeleteNamespaced(node.Namespace, node.Name, &metav1.DeleteOptions{
				PropagationPolicy: &f,
			})
		}()
		return nil
	}

	return c.Nodes.DeleteNamespaced(node.Namespace, node.Name, &metav1.DeleteOptions{
		PropagationPolicy: &f,
	})
}

func (c *Controller) reconcile(nodePool *v3.NodePool) error {
	changed, err := c.createOrCheckNodes(nodePool, true)
	if err != nil {
		return err
	}

	if changed {
		_, err = c.createOrCheckNodes(nodePool, false)
	}

	return err
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

func (c *Controller) nodes(nodePool *v3.NodePool, simulate bool) ([]*v3.Node, error) {
	if simulate {
		return c.NodeLister.List(nodePool.Namespace, labels.Everything())
	}

	nodeList, err := c.Nodes.ListNamespaced(nodePool.Namespace, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var nodes []*v3.Node
	for i := range nodeList.Items {
		nodes = append(nodes, &nodeList.Items[i])
	}

	return nodes, nil
}

func (c *Controller) createOrCheckNodes(nodePool *v3.NodePool, simulate bool) (bool, error) {
	var (
		err                 error
		byName              = map[string]*v3.Node{}
		changed             = false
		nodes               []*v3.Node
		deleteNotReadyAfter = nodePool.Spec.DeleteNotReadyAfterSecs * time.Second
	)

	allNodes, err := c.nodes(nodePool, simulate)
	if err != nil {
		return false, err
	}

	for _, node := range allNodes {
		byName[node.Spec.RequestedHostname] = node

		_, nodePoolName := ref.Parse(node.Spec.NodePoolName)
		if nodePoolName != nodePool.Name {
			continue
		}

		if v32.NodeConditionProvisioned.IsFalse(node) || v32.NodeConditionInitialized.IsFalse(node) || v32.NodeConditionConfigSaved.IsFalse(node) {
			changed = true
			if !simulate {
				_ = c.deleteNode(node, 2*time.Minute)
			}
		}
		// remove unreachable node with the unreachable taint & status of Ready being Unknown
		q := getTaint(node.Spec.InternalNodeSpec.Taints, &unReachableTaint)
		if q != nil && deleteNotReadyAfter > 0 {
			changed = true
			if isNodeReadyUnknown(node) && !simulate {
				start := q.TimeAdded.Time
				if time.Since(start) > deleteNotReadyAfter {
					err = c.deleteNode(node, 0)
					if err != nil {
						return false, err
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

	quantity := nodePool.Spec.Quantity
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
			return false, err
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
				return false, err
			}
		}
	}

	return changed, nil
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
		logrus.Debugf("updating machine [%s] roles: nodepoolRoles: {%+v} node roles: {%+v}", node.Name, poolRolesMap, nodeRolesMap)
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

	toUpdate.Status.NodeConfig.Role = newRoles
	if simulate {
		return toUpdate, nil
	}
	return c.Nodes.Update(toUpdate)
}

// requeue checks every 5 seconds if the node is still unreachable with one goroutine per node
func (c *Controller) requeue(timeout time.Duration, np *v3.NodePool, node *v3.Node) {

	t := getTaint(node.Spec.InternalNodeSpec.Taints, &unReachableTaint)
	for t != nil {
		time.Sleep(5 * time.Second)
		exist, err := c.NodeLister.Get(node.Namespace, node.Name)
		if err != nil {
			break
		}
		t = getTaint(exist.Spec.InternalNodeSpec.Taints, &unReachableTaint)
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

// getTaint returns the taint that matches the given request
func getTaint(taints []v1.Taint, taintToFind *v1.Taint) *v1.Taint {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
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
