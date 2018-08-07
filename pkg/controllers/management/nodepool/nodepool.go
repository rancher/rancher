package nodepool

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"

	"reflect"

	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rke/services"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	nameRegexp = regexp.MustCompile("^(.*?)([0-9]+)$")
)

type Controller struct {
	NodePoolController v3.NodePoolController
	NodePoolLister     v3.NodePoolLister
	NodePools          v3.NodePoolInterface
	NodeLister         v3.NodeLister
	Nodes              v3.NodeInterface
}

func Register(management *config.ManagementContext) {
	p := &Controller{
		NodePoolController: management.Management.NodePools("").Controller(),
		NodePoolLister:     management.Management.NodePools("").Controller().Lister(),
		NodePools:          management.Management.NodePools(""),
		NodeLister:         management.Management.Nodes("").Controller().Lister(),
		Nodes:              management.Management.Nodes(""),
	}

	// Add handlers
	p.NodePools.AddLifecycle("nodepool-provisioner", p)
	management.Management.Nodes("").AddHandler("nodepool-provisioner", p.machineChanged)
}

func (c *Controller) Create(nodePool *v3.NodePool) (*v3.NodePool, error) {
	return nodePool, nil
}

func (c *Controller) Updated(nodePool *v3.NodePool) (*v3.NodePool, error) {
	obj, err := v3.NodePoolConditionUpdated.Do(nodePool, func() (runtime.Object, error) {
		return nodePool, c.createNodes(nodePool)
	})
	return obj.(*v3.NodePool), err
}

func (c *Controller) Remove(nodePool *v3.NodePool) (*v3.NodePool, error) {
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

func (c *Controller) machineChanged(key string, machine *v3.Node) error {
	if machine == nil {
		nps, err := c.NodePoolLister.List("", labels.Everything())
		if err != nil {
			return err
		}
		for _, np := range nps {
			c.NodePoolController.Enqueue(np.Namespace, np.Name)
		}
	} else if machine.Spec.NodePoolName != "" {
		ns, name := ref.Parse(machine.Spec.NodePoolName)
		c.NodePoolController.Enqueue(ns, name)
	}

	return nil
}

func (c *Controller) createNode(name string, nodePool *v3.NodePool, simulate bool) (*v3.Node, error) {
	newNode := &v3.Node{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "m-",
			Namespace:    nodePool.Namespace,
			Labels:       nodePool.Labels,
			Annotations:  nodePool.Annotations,
		},
		Spec: v3.NodeSpec{
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

func (c *Controller) createNodes(nodePool *v3.NodePool) error {
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

	nodeList, err := c.Nodes.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var nodes []*v3.Node
	for i := range nodeList.Items {
		if nodeList.Items[i].Namespace == nodePool.Namespace {
			nodes = append(nodes, &nodeList.Items[i])
		}
	}

	return nodes, nil
}

func (c *Controller) createOrCheckNodes(nodePool *v3.NodePool, simulate bool) (bool, error) {
	var (
		err     error
		byName  = map[string]*v3.Node{}
		changed = false
		nodes   []*v3.Node
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

		if v3.NodeConditionProvisioned.IsFalse(node) || v3.NodeConditionInitialized.IsFalse(node) || v3.NodeConditionConfigSaved.IsFalse(node) {
			changed = true
			if !simulate {
				c.deleteNode(node, 2*time.Minute)
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
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].Spec.RequestedHostname < nodes[j].Spec.RequestedHostname
		})

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
	return !reflect.DeepEqual(nodeRolesMap, poolRolesMap)
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
