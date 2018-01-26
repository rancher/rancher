package clusterprovisioner

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"net/url"

	"github.com/pkg/errors"
	"github.com/rancher/kontainer-engine/drivers"
	"github.com/rancher/kontainer-engine/drivers/rke"
	"github.com/rancher/kontainer-engine/logstream"
	"github.com/rancher/kontainer-engine/service"
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/event"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/machine/store"
	machineconfig "github.com/rancher/rancher/pkg/machine/store/config"
	"github.com/rancher/rancher/pkg/management/dialer"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	RKEDriver    = "rancherKubernetesEngine"
	RKEDriverKey = "rancherKubernetesEngineConfig"
)

type Provisioner struct {
	ClusterController v3.ClusterController
	Clusters          v3.ClusterInterface
	Machines          v3.MachineLister
	MachineClient     v3.MachineInterface
	Driver            service.EngineService
	EventLogger       event.Logger
}

func Register(management *config.ManagementContext) {
	store, err := store.NewGenericEncrypedStore("c-", "", management.Core.Namespaces(""),
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
		Machines:          management.Management.Machines("").Controller().Lister(),
		MachineClient:     management.Management.Machines(""),
		EventLogger:       management.EventLogger,
	}

	// Add handlers
	p.Clusters.AddLifecycle("cluster-provisioner-controller", p)
	management.Management.Machines("").AddHandler("cluster-provisioner-controller", p.machineChanged)

	// Setup custom dialer to RKE
	secretStore, err := machineconfig.NewStore(management)
	if err != nil {
		logrus.Fatal(err)
	}

	d := &dialer.TLSDialerFactory{
		Store:           secretStore,
		MachineClient:   management.Management.Machines(""),
		ConfigMapGetter: management.K8sClient.CoreV1(),
	}

	driver := service.Drivers["rke"]
	rkeDriver := driver.(*rke.Driver)
	rkeDriver.DockerDialer = d.Build
}

func (p *Provisioner) Remove(cluster *v3.Cluster) (*v3.Cluster, error) {
	logrus.Infof("Deleting cluster [%s]", cluster.Name)
	if !needToProvision(cluster) {
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
		waiting, newObj, err := p.reconcileCluster(cluster, false)
		if err == nil && waiting {
			return newObj, &controller.ForgetError{Err: fmt.Errorf("waiting for nodes to provision or a valid configuration")}
		}
		v3.ClusterConditionProvisioned.True(cluster)
		return newObj, err
	})
	cluster, _ = obj.(*v3.Cluster)

	return cluster, err
}

func (p *Provisioner) machineChanged(key string, machine *v3.Machine) error {
	if machine == nil {
		return nil
	}

	if machine.Status.NodeConfig != nil {
		p.ClusterController.Enqueue("", machine.Namespace)
	}

	return nil
}

func (p *Provisioner) createMachines(cluster *v3.Cluster) (*v3.Cluster, error) {
	toCreate := map[string]v3.MachineConfig{}

	for _, machine := range cluster.Spec.Nodes {
		toCreate[machine.RequestedHostname] = machine
	}

	machines, err := p.Machines.List(cluster.Name, labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, machine := range machines {
		delete(toCreate, machine.Spec.RequestedHostname)
	}

	for _, machine := range toCreate {
		newMachine := &v3.Machine{}
		newMachine.GenerateName = "machine-"
		newMachine.Namespace = cluster.Name
		newMachine.Spec = machine.MachineSpec
		newMachine.Spec.ClusterName = cluster.Name
		newMachine.Labels = machine.Labels
		newMachine.Annotations = machine.Annotations

		_, err := p.MachineClient.Create(newMachine)
		if err != nil {
			return nil, err
		}
	}

	return cluster, nil
}

func (p *Provisioner) Create(cluster *v3.Cluster) (*v3.Cluster, error) {
	var err error

	if v3.ClusterConditionProvisioned.IsTrue(cluster) {
		return nil, nil
	}

	cluster.Status.ClusterName = cluster.Spec.DisplayName
	if cluster.Status.ClusterName == "" {
		cluster.Status.ClusterName = cluster.Name
	}

	// Initialize conditions, be careful to not continually update them
	v3.ClusterConditionProvisioned.CreateUnknownIfNotExists(cluster)
	if v3.ClusterConditionReady.GetStatus(cluster) == "" {
		v3.ClusterConditionReady.False(cluster)
	}
	if v3.ClusterConditionReady.GetMessage(cluster) == "" {
		v3.ClusterConditionReady.Message(cluster, "Waiting for API to be available")
	}

	obj, err := v3.ClusterConditionProvisioned.DoUntilTrue(cluster, func() (runtime.Object, error) {
		waiting, newCluster, err := p.reconcileCluster(cluster, true)
		if newCluster != nil {
			cluster = newCluster
		}
		if err != nil {
			return cluster, err
		}
		if waiting {
			return cluster, &controller.ForgetError{Err: fmt.Errorf("waiting for nodes to provision or a valid configuration")}
		}
		return cluster, err
	})

	return obj.(*v3.Cluster), err
}

// reconcileCluster returns true if waiting or false if ready to provision
func (p *Provisioner) reconcileCluster(cluster *v3.Cluster, create bool) (bool, *v3.Cluster, error) {
	if !needToProvision(cluster) {
		v3.ClusterConditionProvisioned.True(cluster)
		return false, cluster, nil
	}

	obj, err := v3.ClusterConditionMachinesCreated.DoUntilTrue(cluster, func() (runtime.Object, error) {
		return p.createMachines(cluster)
	})
	if err != nil {
		return false, obj.(*v3.Cluster), err
	}

	var apiEndpoint, serviceAccountToken, caCert string
	waiting, driver, spec, err := p.getSpec(cluster)
	if err != nil {
		return waiting, nil, errors.Wrapf(err, "failed to construct cluster [%s] spec", cluster.Name)
	}
	if spec == nil || waiting {
		return waiting, nil, nil
	}

	logrus.Infof("Provisioning cluster [%s]", cluster.Name)

	if create {
		logrus.Infof("Creating cluster [%s]", cluster.Name)
		apiEndpoint, serviceAccountToken, caCert, err = p.driverCreate(cluster, *spec)
		// valiate token
		if err == nil {
			err = validateClient(apiEndpoint, serviceAccountToken, caCert)
		}
	} else {
		logrus.Infof("Updating cluster [%s]", cluster.Name)
		apiEndpoint, serviceAccountToken, caCert, err = p.driverUpdate(cluster, *spec)
	}

	// at this point we know the cluster has been modified in driverCreate/Update so reload
	if newCluster, reloadErr := p.Clusters.Get(cluster.Name, metav1.GetOptions{}); reloadErr == nil {
		cluster = newCluster
	}

	// for here out we want to always return the cluster, not just nil, so that the error can be properly
	// recorded if needs be
	if err != nil {
		return false, cluster, err
	}

	saved := false
	for i := 0; i < 20; i++ {
		cluster, err = p.Clusters.Get(cluster.Name, metav1.GetOptions{})
		if err != nil {
			return false, cluster, err
		}

		cluster.Status.AppliedSpec = *spec
		cluster.Status.APIEndpoint = apiEndpoint
		cluster.Status.ServiceAccountToken = serviceAccountToken
		cluster.Status.CACert = caCert
		cluster.Status.Driver = driver

		if cluster, err = p.Clusters.Update(cluster); err == nil {
			saved = true
			break
		} else {
			logrus.Errorf("failed to update cluster [%s]: %v", cluster.Name, err)
			time.Sleep(2)
		}
	}

	if !saved {
		return false, cluster, fmt.Errorf("failed to update cluster")
	}

	logrus.Infof("Provisioned cluster [%s]", cluster.Name)
	return false, cluster, nil
}

func validateClient(api string, token string, cert string) error {
	u, err := url.Parse(api)
	if err != nil {
		return err
	}
	caBytes, err := base64.StdEncoding.DecodeString(cert)
	if err != nil {
		return err
	}
	config := &rest.Config{
		Host:        u.Host,
		Prefix:      u.Path,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caBytes,
		},
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	_, err = clientset.Discovery().ServerVersion()
	return err
}

func needToProvision(cluster *v3.Cluster) bool {
	return !cluster.Spec.Internal
}

func (p *Provisioner) getConfig(reconcileRKE bool, spec v3.ClusterSpec, clusterName string) (bool, string, *v3.ClusterSpec, interface{}, error) {
	var (
		ok    bool
		err   error
		nodes []v3.RKEConfigNode
	)

	// ignore error, not sure how this could ever fail
	data, _ := convert.EncodeToMap(spec)

	for k, v := range data {
		if !strings.HasSuffix(k, "Config") || convert.IsEmpty(v) {
			continue
		}

		driver := strings.TrimSuffix(k, "Config")

		if driver == RKEDriver && reconcileRKE {
			ok, nodes, err = p.reconcileRKENodes(clusterName)
			if err != nil {
				return true, "", nil, nil, err
			}
			if !ok {
				return true, "", nil, nil, nil
			}
			copy := *spec.RancherKubernetesEngineConfig
			spec.RancherKubernetesEngineConfig = &copy
			spec.RancherKubernetesEngineConfig.Nodes = nodes
			data, _ = convert.EncodeToMap(spec)
			v = data[RKEDriverKey]
		}

		return false, driver, &spec, v, nil
	}

	return false, "", nil, nil, nil
}

func (p *Provisioner) getSpec(cluster *v3.Cluster) (bool, string, *v3.ClusterSpec, error) {
	_, oldDriver, _, oldConfig, err := p.getConfig(false, cluster.Status.AppliedSpec, cluster.Name)
	if err != nil {
		return false, "", nil, err
	}
	waiting, newDriver, newSpec, newConfig, err := p.getConfig(true, cluster.Spec, cluster.Name)
	if err != nil {
		return false, "", nil, err
	}

	if waiting {
		return true, "", nil, nil
	}

	if oldDriver == "" && newDriver == "" {
		return false, "", nil, nil
	}

	if oldDriver == "" {
		return false, "", newSpec, nil
	}

	if oldDriver != newDriver {
		return false, "", nil, fmt.Errorf("driver change from %s to %s not allowed", oldDriver, newDriver)
	}

	if reflect.DeepEqual(oldConfig, newConfig) {
		return false, "", nil, nil
	}

	return false, newDriver, newSpec, nil
}

func (p *Provisioner) reconcileRKENodes(clusterName string) (bool, []v3.RKEConfigNode, error) {
	machines, err := p.Machines.List(clusterName, labels.Everything())
	if err != nil {
		return false, nil, err
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

		if !v3.MachineConditionProvisioned.IsTrue(machine) {
			continue
		}

		if slice.ContainsString(machine.Status.NodeConfig.Role, "etcd") {
			etcd = true
		}
		if slice.ContainsString(machine.Status.NodeConfig.Role, "controlplane") {
			controlplane = true
		}
		nodes = append(nodes, *machine.Status.NodeConfig)
	}

	if !etcd || !controlplane {
		return false, nil, nil
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].MachineName < nodes[j].MachineName
	})

	return true, nodes, nil
}

func isImported(cluster *v3.Cluster) bool {
	return cluster.Spec.ImportedConfig != nil && cluster.Spec.ImportedConfig.KubeConfig != ""
}

func (p *Provisioner) createImported(cluster *v3.Cluster) (string, string, string, error) {
	cfg, err := clientcmd.Load([]byte(cluster.Spec.ImportedConfig.KubeConfig))
	if err != nil {
		return "", "", "", err
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{})
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return "", "", "", err
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return "", "", "", err
	}

	token, err := drivers.GenerateServiceAccountToken(clientset)
	if err != nil {
		return "", "", "", err
	}

	return restConfig.Host, token, base64.StdEncoding.EncodeToString(restConfig.TLSClientConfig.CAData), nil
}

func (p *Provisioner) driverCreate(cluster *v3.Cluster, spec v3.ClusterSpec) (api string, token string, cert string, err error) {
	if isImported(cluster) {
		return p.createImported(cluster)
	}

	ctx, logger := p.getCtx(cluster, v3.ClusterConditionProvisioned)
	defer logger.Close()

	if newCluster, err := p.Clusters.Update(cluster); err == nil {
		cluster = newCluster
	}

	return p.Driver.Create(ctx, cluster.Status.ClusterName, spec)
}

func (p *Provisioner) driverUpdate(cluster *v3.Cluster, spec v3.ClusterSpec) (api string, token string, cert string, err error) {
	if isImported(cluster) {
		return p.createImported(cluster)
	}

	ctx, logger := p.getCtx(cluster, v3.ClusterConditionUpdated)
	defer logger.Close()

	if newCluster, err := p.Clusters.Update(cluster); err == nil {
		cluster = newCluster
	}

	return p.Driver.Update(ctx, cluster.Status.ClusterName, spec)
}

func (p *Provisioner) driverRemove(cluster *v3.Cluster) error {
	if isImported(cluster) {
		return nil
	}

	ctx, logger := p.getCtx(cluster, v3.ClusterConditionRemoved)
	defer logger.Close()

	_, err := v3.ClusterConditionUpdated.Do(cluster, func() (runtime.Object, error) {
		if newCluster, err := p.Clusters.Update(cluster); err == nil {
			cluster = newCluster
		}

		return cluster, p.Driver.Remove(ctx, cluster.Status.ClusterName, cluster.Spec)
	})

	return err
}

func (p *Provisioner) getCtx(cluster *v3.Cluster, cond condition.Cond) (context.Context, logstream.LoggerStream) {
	logger := logstream.NewLogStream()
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.New(map[string]string{
		"log-id": logger.ID(),
	}))
	go func() {
		for event := range logger.Stream() {
			if event.Error {
				p.EventLogger.Error(cluster, event.Message)
				logrus.Errorf("cluster [%s] provisioning: %s", cluster.Name, event.Message)
			} else {
				p.EventLogger.Info(cluster, event.Message)
				logrus.Infof("cluster [%s] provisioning: %s", cluster.Name, event.Message)
			}
			if cond.GetMessage(cluster) != event.Message {
				updated := false
				for i := 0; i < 2 && !updated; i++ {
					cond.Message(cluster, event.Message)
					if newCluster, err := p.Clusters.Update(cluster); err == nil {
						updated = true
						cluster = newCluster
					} else {
						newCluster, err = p.Clusters.Get(cluster.Name, metav1.GetOptions{})
						if err == nil {
							cluster = newCluster
						}
					}
				}
			}
		}
	}()
	return ctx, logger
}
