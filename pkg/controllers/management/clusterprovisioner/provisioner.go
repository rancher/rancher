package clusterprovisioner

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/pborman/uuid"
	"github.com/rancher/kontainer-engine/drivers/rke"
	"github.com/rancher/kontainer-engine/service"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/event"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/client"
	"github.com/rancher/rancher/pkg/configfield"
	"github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/encryptedstore"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/flowcontrol"
)

const (
	RKEDriver    = "rancherKubernetesEngine"
	RKEDriverKey = "rancherKubernetesEngineConfig"
)

type Provisioner struct {
	ClusterController v3.ClusterController
	Clusters          v3.ClusterInterface
	NodeLister        v3.NodeLister
	Nodes             v3.NodeInterface
	Driver            service.EngineService
	EventLogger       event.Logger
	backoff           *flowcontrol.Backoff
}

func Register(management *config.ManagementContext, dialerFactory dialer.Factory) {
	store, err := encryptedstore.NewGenericEncrypedStore("c-", "", management.Core.Namespaces(""),
		management.K8sClient.CoreV1())
	if err != nil {
		logrus.Fatal(err)
	}

	p := &Provisioner{
		Driver: service.NewEngineService(&engineStore{
			store: store,
		}),
		Clusters:          management.Management.Clusters(""),
		ClusterController: management.Management.Clusters("").Controller(),
		NodeLister:        management.Management.Nodes("").Controller().Lister(),
		Nodes:             management.Management.Nodes(""),
		EventLogger:       management.EventLogger,
		backoff:           flowcontrol.NewBackOff(time.Minute, 10*time.Minute),
	}

	// Add handlers
	p.Clusters.AddLifecycle("cluster-provisioner-controller", p)
	management.Management.Nodes("").AddHandler("cluster-provisioner-controller", p.machineChanged)

	local := &RKEDialerFactory{
		Factory: dialerFactory,
	}
	docker := &RKEDialerFactory{
		Factory: dialerFactory,
		Docker:  true,
	}

	driver := service.Drivers["rke"]
	rkeDriver := driver.(*rke.Driver)
	rkeDriver.DockerDialer = docker.Build
	rkeDriver.LocalDialer = local.Build
}

func (p *Provisioner) Remove(cluster *v3.Cluster) (*v3.Cluster, error) {
	logrus.Infof("Deleting cluster [%s]", cluster.Name)
	if !needToProvision(cluster) ||
		cluster.Status.Driver == "" ||
		cluster.Status.Driver == "imported" {
		return nil, nil
	}

	for i := 0; i < 4; i++ {
		err := p.driverRemove(cluster)
		if err == nil {
			break
		}
		if i == 3 {
			return cluster, fmt.Errorf("failed to remove the cluster [%s]: %v", cluster.Name, err)
		}
		time.Sleep(1 * time.Second)
	}
	logrus.Infof("Deleted cluster [%s]", cluster.Name)

	// cluster object will definitely have changed, reload
	return p.Clusters.Get(cluster.Name, metav1.GetOptions{})
}

func (p *Provisioner) Updated(cluster *v3.Cluster) (*v3.Cluster, error) {
	obj, err := v3.ClusterConditionUpdated.Do(cluster, func() (runtime.Object, error) {
		return p.update(cluster, false)
	})
	return obj.(*v3.Cluster), err
}

func (p *Provisioner) update(cluster *v3.Cluster, create bool) (*v3.Cluster, error) {
	if err := p.createNodes(cluster); err != nil {
		return cluster, err
	}

	cluster, err := p.reconcileCluster(cluster, create)
	if err != nil {
		return cluster, err
	}

	v3.ClusterConditionProvisioned.True(cluster)
	return cluster, nil
}

func (p *Provisioner) machineChanged(key string, machine *v3.Node) error {
	parts := strings.SplitN(key, "/", 2)
	if machine == nil || machine.Status.NodeConfig != nil {
		p.ClusterController.Enqueue("", parts[0])
	}

	return nil
}

func (p *Provisioner) createNode(name string, cluster *v3.Cluster, nodePool v3.NodePool) (*v3.Node, error) {
	newNode := &v3.Node{}
	newNode.GenerateName = "m-"
	newNode.Namespace = cluster.Name
	newNode.Spec = v3.NodeSpec{
		CommonNodeSpec: v3.CommonNodeSpec{
			Etcd:             nodePool.Etcd,
			ControlPlane:     nodePool.ControlPlane,
			Worker:           nodePool.Worker,
			NodeTemplateName: nodePool.NodeTemplateName,
		},
		NodePoolUUID:      nodePool.UUID,
		RequestedHostname: name,
		ClusterName:       cluster.Name,
	}
	newNode.Spec.ClusterName = cluster.Name
	newNode.Labels = nodePool.Labels
	newNode.Annotations = nodePool.Annotations

	if newNode.Spec.NodeTemplateName == "" || nodePool.HostnamePrefix == "" {
		logrus.Warnf("invalid node pool on cluster [%s], not creating node", cluster.Name)
		return newNode, nil
	}

	return p.Nodes.Create(newNode)
}

func (p *Provisioner) deleteNodeLater(node *v3.Node) {
	go func() {
		time.Sleep(2 * time.Minute)
		f := metav1.DeletePropagationForeground
		p.Nodes.DeleteNamespaced(node.Namespace, node.Name, &metav1.DeleteOptions{
			PropagationPolicy: &f,
		})
	}()
}

func (p *Provisioner) createNodes(cluster *v3.Cluster) error {
	if cluster.Status.Driver != RKEDriver {
		return nil
	}

	byUUID := map[string][]*v3.Node{}
	byName := map[string]bool{}

	changed := false
	for i, nodePool := range cluster.Spec.NodePools {
		if nodePool.UUID == "" {
			changed = true
			cluster.Spec.NodePools[i].UUID = uuid.New()
		}
	}

	if changed {
		_, err := p.Clusters.Update(cluster)
		if err != nil {
			return err
		}
	}

	nodes, err := p.NodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return err
	}

	waiting := false
	for _, node := range nodes {
		if v3.NodeConditionProvisioned.IsUnknown(node) {
			waiting = true
		}

		if v3.NodeConditionProvisioned.IsFalse(node) {
			p.deleteNodeLater(node)
		}
		byName[node.Spec.RequestedHostname] = true
		if node.Spec.NodePoolUUID != "" {
			byUUID[node.Spec.NodePoolUUID] = append(byUUID[node.Spec.NodePoolUUID], node)
		}
	}

	for _, nodePool := range cluster.Spec.NodePools {
		nodes := byUUID[nodePool.UUID]
		delete(byUUID, nodePool.UUID)

		if nodePool.Quantity <= 0 {
			nodePool.Quantity = 0
		}

		if len(nodes) == nodePool.Quantity {
			continue
		}

		for i := 1; len(nodes) < nodePool.Quantity; i++ {
			if waiting {
				// I do this so we don't get in a tight loop of creating nodes by accident
				return &controller.ForgetError{
					Err: fmt.Errorf("waiting on nodes to provision"),
				}
			}

			name := fmt.Sprintf("%s%02d", nodePool.HostnamePrefix, i)
			if byName[name] {
				continue
			}

			newNode, err := p.createNode(name, cluster, nodePool)
			if err != nil {
				return err
			}

			byName[newNode.Spec.RequestedHostname] = true
			nodes = append(nodes, newNode)
		}

		for len(nodes) > nodePool.Quantity {
			sort.Slice(nodes, func(i, j int) bool {
				return nodes[i].Spec.RequestedHostname < nodes[j].Spec.RequestedHostname
			})

			toDelete := nodes[len(nodes)-1]

			prop := metav1.DeletePropagationForeground
			err := p.Nodes.DeleteNamespaced(toDelete.Namespace, toDelete.Name, &metav1.DeleteOptions{
				PropagationPolicy: &prop,
			})
			if err != nil {
				return err
			}

			nodes = nodes[:len(nodes)-1]
			delete(byName, toDelete.Spec.RequestedHostname)
		}
	}

	for _, nodes := range byUUID {
		for _, node := range nodes {
			prop := metav1.DeletePropagationForeground
			err := p.Nodes.DeleteNamespaced(node.Namespace, node.Name, &metav1.DeleteOptions{
				PropagationPolicy: &prop,
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Provisioner) Create(cluster *v3.Cluster) (*v3.Cluster, error) {
	var err error

	cluster.Status.ClusterName = cluster.Spec.DisplayName
	if cluster.Status.ClusterName == "" {
		cluster.Status.ClusterName = cluster.Name
	}

	// Initialize conditions, be careful to not continually update them
	v3.ClusterConditionPending.CreateUnknownIfNotExists(cluster)
	v3.ClusterConditionProvisioned.CreateUnknownIfNotExists(cluster)

	if v3.ClusterConditionReady.GetStatus(cluster) == "" {
		v3.ClusterConditionReady.False(cluster)
	}
	if v3.ClusterConditionReady.GetMessage(cluster) == "" {
		v3.ClusterConditionReady.Message(cluster, "Waiting for API to be available")
	}

	cluster, err = p.pending(cluster)
	if err != nil {
		return cluster, err
	}

	return p.provision(cluster)
}

func (p *Provisioner) provision(cluster *v3.Cluster) (*v3.Cluster, error) {
	obj, err := v3.ClusterConditionProvisioned.Do(cluster, func() (runtime.Object, error) {
		return p.update(cluster, true)
	})
	return obj.(*v3.Cluster), err
}

func (p *Provisioner) pending(cluster *v3.Cluster) (*v3.Cluster, error) {
	obj, err := v3.ClusterConditionPending.DoUntilTrue(cluster, func() (runtime.Object, error) {
		if !needToProvision(cluster) {
			return cluster, nil
		}

		driver, err := p.validateDriver(cluster)
		if err != nil {
			return cluster, err
		}

		if driver == "" {
			return cluster, &controller.ForgetError{Err: fmt.Errorf("waiting for full cluster configuration")}
		}

		if driver != cluster.Status.Driver {
			cluster.Status.Driver = driver
			if driver == RKEDriver && cluster.Spec.RancherKubernetesEngineConfig == nil {
				cluster.Spec.RancherKubernetesEngineConfig = &v3.RancherKubernetesEngineConfig{}
			}
			return p.Clusters.Update(cluster)
		}

		return cluster, nil
	})

	return obj.(*v3.Cluster), err
}

func (p *Provisioner) backoffFailure(cluster *v3.Cluster, spec *v3.ClusterSpec) (bool, time.Duration) {
	if cluster.Status.FailedSpec == nil {
		return false, 0
	}

	if !reflect.DeepEqual(cluster.Status.FailedSpec, spec) {
		return false, 0
	}

	if p.backoff.IsInBackOffSinceUpdate(cluster.Name, time.Now()) {
		go func() {
			time.Sleep(p.backoff.Get(cluster.Name))
			p.ClusterController.Enqueue("", cluster.Name)
		}()
		return true, p.backoff.Get(cluster.Name)
	}

	return false, 0
}

// reconcileCluster returns true if waiting or false if ready to provision
func (p *Provisioner) reconcileCluster(cluster *v3.Cluster, create bool) (*v3.Cluster, error) {
	if !needToProvision(cluster) {
		return cluster, nil
	}

	var (
		apiEndpoint, serviceAccountToken, caCert string
		err                                      error
	)

	spec, err := p.getSpec(cluster)
	if err != nil || spec == nil {
		return cluster, err
	}

	if ok, delay := p.backoffFailure(cluster, spec); ok {
		return cluster, &controller.ForgetError{Err: fmt.Errorf("backing off failure, delay: %v", delay)}
	}

	logrus.Infof("Provisioning cluster [%s]", cluster.Name)

	if create {
		logrus.Infof("Creating cluster [%s]", cluster.Name)
		apiEndpoint, serviceAccountToken, caCert, err = p.driverCreate(cluster, *spec)
		// validate token
		if err == nil {
			err = client.Validate(apiEndpoint, serviceAccountToken, caCert)
		}
	} else {
		logrus.Infof("Updating cluster [%s]", cluster.Name)
		apiEndpoint, serviceAccountToken, caCert, err = p.driverUpdate(cluster, *spec)
	}

	// at this point we know the cluster has been modified in driverCreate/Update so reload
	if newCluster, reloadErr := p.Clusters.Get(cluster.Name, metav1.GetOptions{}); reloadErr == nil {
		cluster = newCluster
	}

	cluster, recordErr := p.recordFailure(cluster, *spec, err)
	if recordErr != nil {
		return cluster, recordErr
	}

	// for here out we want to always return the cluster, not just nil, so that the error can be properly
	// recorded if needs be
	if err != nil {
		return cluster, err
	}

	saved := false
	for i := 0; i < 20; i++ {
		cluster, err = p.Clusters.Get(cluster.Name, metav1.GetOptions{})
		if err != nil {
			return cluster, err
		}

		cluster.Status.AppliedSpec = *spec
		cluster.Status.APIEndpoint = apiEndpoint
		cluster.Status.ServiceAccountToken = serviceAccountToken
		cluster.Status.CACert = caCert

		if cluster, err = p.Clusters.Update(cluster); err == nil {
			saved = true
			break
		} else {
			logrus.Errorf("failed to update cluster [%s]: %v", cluster.Name, err)
			time.Sleep(2)
		}
	}

	if !saved {
		return cluster, fmt.Errorf("failed to update cluster")
	}

	logrus.Infof("Provisioned cluster [%s]", cluster.Name)
	return cluster, nil
}

func needToProvision(cluster *v3.Cluster) bool {
	return !cluster.Spec.Internal
}

func (p *Provisioner) getConfig(reconcileRKE bool, spec v3.ClusterSpec, driverName, clusterName string) (*v3.ClusterSpec, interface{}, error) {
	data, err := convert.EncodeToMap(spec)
	if err != nil {
		return nil, nil, err
	}

	v, ok := data[driverName+"Config"]
	if !ok || v == nil {
		v = map[string]interface{}{}
	}

	if driverName == RKEDriver && reconcileRKE {
		nodes, err := p.reconcileRKENodes(clusterName)
		if err != nil {
			return nil, nil, err
		}

		systemImages, err := getSystemImages(spec)
		if err != nil {
			return nil, nil, err
		}

		copy := *spec.RancherKubernetesEngineConfig
		spec.RancherKubernetesEngineConfig = &copy
		spec.RancherKubernetesEngineConfig.Nodes = nodes
		spec.RancherKubernetesEngineConfig.SystemImages = *systemImages

		data, _ = convert.EncodeToMap(spec)
		v = data[RKEDriverKey]
	}

	return &spec, v, nil
}

func (p *Provisioner) getDriver(cluster *v3.Cluster) string {
	driver := configfield.GetDriver(&cluster.Spec)

	if driver == "" {
		if len(cluster.Spec.NodePools) > 0 {
			return RKEDriver
		}

		nodes, err := p.reconcileRKENodes(cluster.Name)
		if err == nil && len(nodes) > 0 {
			return RKEDriver
		}
	}

	return driver
}

func (p *Provisioner) validateDriver(cluster *v3.Cluster) (string, error) {
	oldDriver := cluster.Status.Driver
	newDriver := p.getDriver(cluster)

	if oldDriver == "" && newDriver == "" {
		return newDriver, nil
	}

	if oldDriver == "" {
		return newDriver, nil
	}

	if newDriver == "" {
		return "", &controller.ForgetError{Err: fmt.Errorf("waiting for nodes")}
	}

	if oldDriver != newDriver {
		return newDriver, fmt.Errorf("driver change from %s to %s not allowed", oldDriver, newDriver)
	}

	return newDriver, nil
}

func getSystemImages(spec v3.ClusterSpec) (*v3.RKESystemImages, error) {
	// fetch system images from settings
	systemImagesStr := settings.KubernetesVersionToSystemImages.Get()
	if systemImagesStr == "" {
		return nil, fmt.Errorf("Failed to load setting %s", settings.KubernetesVersionToSystemImages.Name)
	}
	systemImagesMap := make(map[string]v3.RKESystemImages)
	if err := json.Unmarshal([]byte(systemImagesStr), &systemImagesMap); err != nil {
		return nil, err
	}

	version := spec.RancherKubernetesEngineConfig.Version
	if version == "" {
		version = settings.KubernetesVersion.Get()
	}

	systemImages, ok := systemImagesMap[version]
	if !ok {
		return nil, fmt.Errorf("Failed to find system images for version %v", version)
	}

	if len(spec.RancherKubernetesEngineConfig.PrivateRegistries) == 0 {
		return &systemImages, nil
	}

	// prepend private repo
	privateRegistry := spec.RancherKubernetesEngineConfig.PrivateRegistries[0]
	imagesMap, err := convert.EncodeToMap(systemImages)
	if err != nil {
		return nil, err
	}
	updatedMap := make(map[string]interface{})
	for key, value := range imagesMap {
		newValue := fmt.Sprintf("%s/%s", privateRegistry.URL, value)
		updatedMap[key] = newValue
	}
	if err := mapstructure.Decode(updatedMap, &systemImages); err != nil {
		return nil, err
	}
	return &systemImages, nil
}

func (p *Provisioner) getSpec(cluster *v3.Cluster) (*v3.ClusterSpec, error) {
	driverName, err := p.validateDriver(cluster)
	if err != nil {
		return nil, err
	}

	_, oldConfig, err := p.getConfig(false, cluster.Status.AppliedSpec, driverName, cluster.Name)
	if err != nil {
		return nil, err
	}

	newSpec, newConfig, err := p.getConfig(true, cluster.Spec, driverName, cluster.Name)
	if err != nil {
		return nil, err
	}

	if reflect.DeepEqual(oldConfig, newConfig) {
		newSpec = nil
	}

	return newSpec, nil
}

func (p *Provisioner) reconcileRKENodes(clusterName string) ([]v3.RKEConfigNode, error) {
	machines, err := p.NodeLister.List(clusterName, labels.Everything())
	if err != nil {
		return nil, err
	}

	etcd := false
	controlplane := false
	var nodes []v3.RKEConfigNode
	for _, machine := range machines {
		if machine.DeletionTimestamp != nil {
			continue
		}

		if machine.Status.NodeConfig == nil {
			continue
		}

		if len(machine.Status.NodeConfig.Role) == 0 {
			continue
		}

		if !v3.NodeConditionReady.IsTrue(machine) {
			continue
		}

		if slice.ContainsString(machine.Status.NodeConfig.Role, "etcd") {
			etcd = true
		}
		if slice.ContainsString(machine.Status.NodeConfig.Role, "controlplane") {
			controlplane = true
		}
		node := *machine.Status.NodeConfig
		if node.User == "" {
			node.User = "root"
		}
		if len(node.Role) == 0 {
			node.Role = []string{"worker"}
		}
		if node.NodeName == "" {
			node.NodeName = fmt.Sprintf("%s:%s", machine.Namespace, machine.Name)
		}
		nodes = append(nodes, node)
	}

	if !etcd || !controlplane {
		return nil, &controller.ForgetError{
			Err: fmt.Errorf("waiting for etcd and controlplane nodes to be registered"),
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].NodeName < nodes[j].NodeName
	})

	return nodes, nil
}
