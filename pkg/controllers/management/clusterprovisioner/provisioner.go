package clusterprovisioner

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/imported"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	v1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/service"
	"github.com/rancher/rancher/pkg/kontainerdriver"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/flowcontrol"
)

const (
	KontainerEngineUpdate = "provisioner.cattle.io/ke-driver-update"
)

type Provisioner struct {
	ClusterController     v3.ClusterController
	Clusters              v3.ClusterInterface
	ConfigMaps            corev1.ConfigMapInterface
	NodeLister            v3.NodeLister
	Nodes                 v3.NodeInterface
	engineService         *service.EngineService
	backoff               *flowcontrol.Backoff
	KontainerDriverLister v3.KontainerDriverLister
	DynamicSchemasLister  v3.DynamicSchemaLister
	DaemonsetLister       v1.DaemonSetLister
	SecretLister          corev1.SecretLister
	Secrets               corev1.SecretInterface
}

func Register(ctx context.Context, management *config.ManagementContext) {
	p := &Provisioner{
		engineService:         service.NewEngineService(NewPersistentStore(management.Core.Namespaces(""), management.Core, management.Management.Clusters(""))),
		Clusters:              management.Management.Clusters(""),
		ConfigMaps:            management.Core.ConfigMaps(""),
		ClusterController:     management.Management.Clusters("").Controller(),
		NodeLister:            management.Management.Nodes("").Controller().Lister(),
		Nodes:                 management.Management.Nodes(""),
		backoff:               flowcontrol.NewBackOff(30*time.Second, 10*time.Minute),
		KontainerDriverLister: management.Management.KontainerDrivers("").Controller().Lister(),
		DynamicSchemasLister:  management.Management.DynamicSchemas("").Controller().Lister(),
		DaemonsetLister:       management.Apps.DaemonSets("").Controller().Lister(),
		SecretLister:          management.Core.Secrets("").Controller().Lister(),
		Secrets:               management.Core.Secrets(""),
	}
	// Add handlers
	p.Clusters.AddLifecycle(ctx, "cluster-provisioner-controller", p)
	management.Management.Nodes("").AddHandler(ctx, "cluster-provisioner-controller", p.machineChanged)
}

func skipOperatorCluster(action string, cluster *apimgmtv3.Cluster) bool {
	msgFmt := "%s cluster [%s] will be managed by %s-operator-controller, skipping %s"
	switch {
	case cluster.Spec.AKSConfig != nil:
		logrus.Debugf(msgFmt, "AKS", cluster.Name, "aks", action)
		return true
	case cluster.Spec.EKSConfig != nil:
		logrus.Debugf(msgFmt, "EKS", cluster.Name, "eks", action)
		return true
	case cluster.Spec.GKEConfig != nil:
		logrus.Debugf(msgFmt, "GKE", cluster.Name, "gke", action)
		return true
	case cluster.Spec.AliConfig != nil:
		logrus.Debugf(msgFmt, "Alibaba", cluster.Name, "ali", action)
		return true
	default:
		return false
	}
}

func (p *Provisioner) Remove(cluster *apimgmtv3.Cluster) (runtime.Object, error) {
	if skipOperatorCluster("remove", cluster) {
		return cluster, nil
	}

	logrus.Infof("Deleting cluster [%s]", cluster.Name)
	if skipLocalK3sImported(cluster) ||
		cluster.Status.Driver == "" {
		return nil, nil
	}

	for i := 0; i < 4; i++ {
		// cluster will be forcefully removed on last attempt
		err := p.driverRemove(cluster, i == 3)
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

func (p *Provisioner) Updated(cluster *apimgmtv3.Cluster) (runtime.Object, error) {
	if skipOperatorCluster("update", cluster) {
		return cluster, nil
	}

	if imported.IsAdministratedByProvisioningCluster(cluster) {
		reconcileACE(cluster)
		return p.Clusters.Update(cluster)
	}

	obj, err := apimgmtv3.ClusterConditionUpdated.Do(cluster, func() (runtime.Object, error) {
		anno, _ := cluster.Annotations[KontainerEngineUpdate]
		if anno == "updated" {
			// Cluster has already been updated proceed as usual
			setVersion(cluster)
			return p.update(cluster, false)

		} else if strings.HasPrefix(anno, "updating/") {
			// Check if it's been updating for more than 20 seconds, this lets
			// the controller take over attempting to update the cluster
			pieces := strings.Split(anno, "/")
			t, err := time.Parse(time.RFC3339, pieces[1])
			if err != nil || int(time.Since(t)/time.Second) > 20 {
				cluster.Annotations[KontainerEngineUpdate] = "updated"
				return p.Clusters.Update(cluster)
			}
			// Go routine is already running to update the cluster so wait
			return nil, nil
		}
		// Set the annotation and kickoff the update
		c, err := p.setKontainerEngineUpdate(cluster, "updating")
		if err != nil {
			return cluster, err
		}
		go p.waitForSchema(c)
		return nil, nil
	})

	return obj.(*apimgmtv3.Cluster), err
}

// waitForSchema waits for the driver and schema to be populated for the cluster
func (p *Provisioner) waitForSchema(cluster *apimgmtv3.Cluster) {
	var driver string
	if cluster.Spec.GenericEngineConfig == nil {
		if cluster.Spec.AmazonElasticContainerServiceConfig != nil {
			driver = "amazonelasticcontainerservice"
		}

		if cluster.Spec.AzureKubernetesServiceConfig != nil {
			driver = "azurekubernetesservice"
		}

		if cluster.Spec.GoogleKubernetesEngineConfig != nil {
			driver = "googlekubernetesengine"
		}
	} else {
		if d, ok := (*cluster.Spec.GenericEngineConfig)["driverName"]; ok {
			driver = d.(string)
		}
	}

	if driver != "" {
		var schemaName string
		backoff := wait.Backoff{
			Duration: 2 * time.Second,
			Factor:   1,
			Jitter:   0,
			Steps:    7,
		}
		err := wait.ExponentialBackoff(backoff, func() (bool, error) {
			driver, err := p.KontainerDriverLister.Get("", driver)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return false, err
				}
				return false, nil
			}

			if driver.Spec.BuiltIn {
				schemaName = driver.Status.DisplayName + "Config"
			} else {
				schemaName = driver.Status.DisplayName + "EngineConfig"
			}

			_, err = p.DynamicSchemasLister.Get("", strings.ToLower(schemaName))
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return false, err
				}
				return false, nil
			}

			return true, nil
		})
		if err != nil {
			logrus.Warnf("[cluster-provisioner-controller] Failed to find driver %v and schema %v for cluster %v on upgrade: %v",
				driver, schemaName, cluster.Name, err)
		}
	}

	_, err := p.setKontainerEngineUpdate(cluster, "updated")
	if err != nil {
		logrus.Warnf("[cluster-provisioner-controller] Failed to set annotation on cluster %v on upgrade: %v", cluster.Name, err)
	}
	p.ClusterController.Enqueue(cluster.Namespace, cluster.Name)
}

func (p *Provisioner) setKontainerEngineUpdate(cluster *apimgmtv3.Cluster, anno string) (*apimgmtv3.Cluster, error) {
	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   1,
		Jitter:   0,
		Steps:    6,
	}

	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		newCluster, err := p.Clusters.Get(cluster.Name, metav1.GetOptions{})
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

		newCluster.Annotations[KontainerEngineUpdate] = anno
		newCluster, err = p.Clusters.Update(newCluster)
		if err != nil {
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return false, err
		}
		cluster = newCluster
		return true, nil
	})
	if err != nil {
		return cluster, fmt.Errorf("[setKontainerEngineUpdate] Failed to update cluster [%s]: %v", cluster.Name, err)
	}
	return cluster, nil
}

func setVersion(cluster *apimgmtv3.Cluster) {
	if cluster.Spec.AmazonElasticContainerServiceConfig != nil {
		if cluster.Status.Version != nil {
			setConfigVersion := func(config *apimgmtv3.MapStringInterface) {
				v, found := values.GetValue(*config, "kubernetesVersion")
				if !found || convert.ToString(v) == "" && cluster.Status.Version != nil && cluster.Status.Version.Major != "" && len(cluster.Status.Version.Minor) > 1 {
					values.PutValue(*config, fmt.Sprintf("%s.%s", cluster.Status.Version.Major, cluster.Status.Version.Minor[:2]), "kubernetesVersion")
				}
			}

			// during upgrade it is possible genericEngineConfig has not been set
			if newConfig := cluster.Spec.AmazonElasticContainerServiceConfig; newConfig != nil {
				setConfigVersion(newConfig)
			}

			if oldConfig := cluster.Status.AppliedSpec.AmazonElasticContainerServiceConfig; oldConfig != nil {
				setConfigVersion(oldConfig)
			}
		}
	}
}

func (p *Provisioner) update(cluster *apimgmtv3.Cluster, create bool) (*apimgmtv3.Cluster, error) {
	cluster, err := p.reconcileCluster(cluster, create)
	if err != nil || imported.IsAdministratedByProvisioningCluster(cluster) {
		return cluster, err
	}

	apimgmtv3.ClusterConditionProvisioned.True(cluster)
	apimgmtv3.ClusterConditionProvisioned.Message(cluster, "")
	apimgmtv3.ClusterConditionProvisioned.Reason(cluster, "")
	apimgmtv3.ClusterConditionPending.True(cluster)

	if cluster.Spec.GenericEngineConfig != nil {
		return cluster, nil
	}
	nodes, err := p.NodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return cluster, err
	}
	err = p.k3sBasedClusterConfig(cluster, nodes)
	if err != nil {
		return cluster, err
	}

	return cluster, nil
}

func (p *Provisioner) machineChanged(key string, machine *apimgmtv3.Node) (runtime.Object, error) {
	parts := strings.SplitN(key, "/", 2)

	p.ClusterController.Enqueue("", parts[0])

	return machine, nil
}

func (p *Provisioner) Create(cluster *apimgmtv3.Cluster) (runtime.Object, error) {
	if skipOperatorCluster("create", cluster) || imported.IsAdministratedByProvisioningCluster(cluster) {
		return cluster, nil
	}

	var err error
	// Initialize conditions, be careful to not continually update them
	apimgmtv3.ClusterConditionPending.CreateUnknownIfNotExists(cluster)
	apimgmtv3.ClusterConditionProvisioned.CreateUnknownIfNotExists(cluster)

	if apimgmtv3.ClusterConditionWaiting.GetStatus(cluster) == "" {
		apimgmtv3.ClusterConditionWaiting.Unknown(cluster)
		if apimgmtv3.ClusterConditionWaiting.GetMessage(cluster) == "" {
			apimgmtv3.ClusterConditionWaiting.Message(cluster, "Waiting for API to be available")
		}
	}

	cluster, err = p.pending(cluster)
	if err != nil {
		return cluster, err
	}

	return p.provision(cluster)
}

func (p *Provisioner) provision(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	obj, err := apimgmtv3.ClusterConditionProvisioned.Do(cluster, func() (runtime.Object, error) {
		return p.update(cluster, true)
	})
	return obj.(*apimgmtv3.Cluster), err
}

func (p *Provisioner) pending(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if skipLocalK3sImported(cluster) {
		return cluster, nil
	}

	driver, err := p.validateDriver(cluster)
	if err != nil {
		return cluster, err
	}

	if driver == "" {
		return cluster, &controller.ForgetError{
			Err:    fmt.Errorf("waiting for full cluster configuration"),
			Reason: "Pending"}
	}

	if driver != cluster.Status.Driver {
		cluster.Status.Driver = driver
		return p.Clusters.Update(cluster)
	}

	return cluster, nil

}

func (p *Provisioner) backoffFailure(cluster *apimgmtv3.Cluster, spec *apimgmtv3.ClusterSpec) (bool, time.Duration) {
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

func (p *Provisioner) reconcileCluster(cluster *apimgmtv3.Cluster, create bool) (*apimgmtv3.Cluster, error) {
	if skipLocalK3sImported(cluster) {
		reconcileACE(cluster)
		return cluster, nil
	}

	var (
		apiEndpoint, serviceAccountToken, caCert string
		err                                      error
	)

	if cluster.Name != "local" && !apimgmtv3.ClusterConditionServiceAccountMigrated.IsTrue(cluster) &&
		apimgmtv3.ClusterConditionProvisioned.IsTrue(cluster) {
		spec, _, err := p.getConfig(cluster.Spec)
		if err != nil {
			return nil, err
		}

		serviceAccountToken, err = p.generateServiceAccount(cluster, *spec)
		if err != nil {
			return nil, err
		}

		secret, err := secretmigrator.NewMigrator(p.SecretLister, p.Secrets).CreateOrUpdateServiceAccountTokenSecret(cluster.Status.ServiceAccountTokenSecret, serviceAccountToken, cluster)
		if err != nil {
			return nil, err
		}
		cluster.Status.ServiceAccountTokenSecret = secret.Name
		cluster.Status.ServiceAccountToken = ""
		apimgmtv3.ClusterConditionServiceAccountMigrated.True(cluster)

		// Update the cluster in k8s
		cluster, err = p.Clusters.Update(cluster)
		if err != nil {
			return nil, err
		}

		err = p.removeLegacyServiceAccount(cluster, *spec)
		if err != nil {
			return nil, err
		}
	}

	p.setGenericConfigs(cluster)

	// Compute the new spec from the cluster resource.
	spec, configChanged, err := p.getSpec(cluster)
	if err != nil {
		return cluster, err
	}

	if !configChanged {
		return cluster, nil
	}

	if ok, delay := p.backoffFailure(cluster, spec); ok {
		return cluster, &controller.ForgetError{Err: fmt.Errorf("backing off failure, delay: %v", delay)}
	}

	logrus.Infof("Provisioning cluster [%s]", cluster.Name)
	if create {
		logrus.Infof("Creating cluster [%s]", cluster.Name)
		// setting updateTriggered to true since rke up will be called on cluster create
		apiEndpoint, serviceAccountToken, caCert, err = p.driverCreate(cluster, *spec)
		if err != nil && err.Error() == "cluster already exists" {
			logrus.Infof("Create done, Updating cluster [%s]", cluster.Name)
			apiEndpoint, serviceAccountToken, caCert, _, err = p.driverUpdate(cluster, *spec)
		}
	} else {
		logrus.Infof("Updating cluster [%s]", cluster.Name)

		// Attempt to manually trigger updating, otherwise it will not be triggered until after exiting reconcile
		apimgmtv3.ClusterConditionUpdated.Unknown(cluster)
		cluster, err = p.Clusters.Update(cluster)
		if err != nil {
			return cluster, fmt.Errorf("[reconcileCluster] Failed to update cluster [%s]: %v", cluster.Name, err)
		}

		apiEndpoint, serviceAccountToken, caCert, _, err = p.driverUpdate(cluster, *spec)
	}
	// at this point we know the cluster has been modified in driverCreate/Update so reload
	if newCluster, reloadErr := p.Clusters.Get(cluster.Name, metav1.GetOptions{}); reloadErr == nil {
		cluster = newCluster
	}

	cluster, recordErr := p.recordFailure(cluster, *spec, err)
	if recordErr != nil {
		return cluster, recordErr
	}

	// from here on we want to return the cluster, not just nil, so that the error can be properly recorded
	if err != nil {
		return cluster, err
	}

	err = p.removeLegacyServiceAccount(cluster, *spec)
	if err != nil {
		return nil, err
	}

	apimgmtv3.ClusterConditionServiceAccountMigrated.True(cluster)

	secret, err := secretmigrator.NewMigrator(p.SecretLister, p.Secrets).CreateOrUpdateServiceAccountTokenSecret(cluster.Status.ServiceAccountTokenSecret, serviceAccountToken, cluster)
	if err != nil {
		return nil, err
	}

	saved := false
	for i := 0; i < 20; i++ {
		cluster, err = p.Clusters.Get(cluster.Name, metav1.GetOptions{})
		if err != nil {
			return cluster, err
		}

		censoredSpec, err := p.censorGenericEngineConfig(*spec)
		if err != nil {
			return cluster, err
		}

		cluster.Status.AppliedSpec = censoredSpec
		cluster.Status.APIEndpoint = apiEndpoint
		cluster.Status.ServiceAccountTokenSecret = secret.Name
		cluster.Status.ServiceAccountToken = ""
		cluster.Status.CACert = caCert

		if cluster, err = p.Clusters.Update(cluster); err == nil {
			saved = true
			break
		}

		logrus.Errorf("failed to update cluster [%s]: %v", cluster.Name, err)
		time.Sleep(2)
	}

	if !saved {
		return cluster, fmt.Errorf("failed to update cluster")
	}

	logrus.Infof("Provisioned cluster [%s]", cluster.Name)
	return cluster, nil
}

func (p *Provisioner) setGenericConfigs(cluster *apimgmtv3.Cluster) {
	if cluster.Spec.GenericEngineConfig == nil || cluster.Status.AppliedSpec.GenericEngineConfig == nil {
		setGenericConfig := func(spec *apimgmtv3.ClusterSpec) {
			if spec.GenericEngineConfig == nil {
				if spec.AmazonElasticContainerServiceConfig != nil {
					spec.GenericEngineConfig = spec.AmazonElasticContainerServiceConfig
					(*spec.GenericEngineConfig)["driverName"] = "amazonelasticcontainerservice"
					spec.AmazonElasticContainerServiceConfig = nil
				}

				if spec.AzureKubernetesServiceConfig != nil {
					spec.GenericEngineConfig = spec.AzureKubernetesServiceConfig
					(*spec.GenericEngineConfig)["driverName"] = "azurekubernetesservice"
					spec.AzureKubernetesServiceConfig = nil
				}

				if spec.GoogleKubernetesEngineConfig != nil {
					spec.GenericEngineConfig = spec.GoogleKubernetesEngineConfig
					(*spec.GenericEngineConfig)["driverName"] = "googlekubernetesengine"
					spec.GoogleKubernetesEngineConfig = nil
				}
			}
		}

		setGenericConfig(&cluster.Spec)
		setGenericConfig(&cluster.Status.AppliedSpec)
	}
}

func copyMap(toCopy apimgmtv3.MapStringInterface) apimgmtv3.MapStringInterface {
	newMap := apimgmtv3.MapStringInterface{}

	for k, v := range toCopy {
		newMap[k] = v
	}

	return newMap
}

func (p *Provisioner) censorGenericEngineConfig(input apimgmtv3.ClusterSpec) (apimgmtv3.ClusterSpec, error) {
	if input.GenericEngineConfig == nil {
		// nothing to do
		return input, nil
	}

	config := copyMap(*input.GenericEngineConfig)
	driverName, ok := config[DriverNameField].(string)
	if !ok {
		// can't figure out driver type so blank out the whole thing
		logrus.Warnf("cluster %v has a generic engine config but no driver type field; can't hide password "+
			"fields so removing the entire config", input.DisplayName)
		input.GenericEngineConfig = nil
		return input, nil
	}

	driver, err := p.KontainerDriverLister.Get("", driverName)
	if err != nil {
		return apimgmtv3.ClusterSpec{}, err
	}

	var schemaName string
	if driver.Spec.BuiltIn {
		schemaName = driver.Status.DisplayName + "Config"
	} else {
		schemaName = driver.Status.DisplayName + "EngineConfig"
	}

	kontainerDriverSchema, err := p.DynamicSchemasLister.Get("", strings.ToLower(schemaName))
	if err != nil {
		return apimgmtv3.ClusterSpec{}, fmt.Errorf("error getting dynamic schema %v", err)
	}

	for key := range config {
		field := kontainerDriverSchema.Spec.ResourceFields[key]
		if field.Type == "password" {
			delete(config, key)
		}
	}

	input.GenericEngineConfig = &config
	return input, nil
}

func skipLocalK3sImported(cluster *apimgmtv3.Cluster) bool {
	return cluster.Status.Driver == apimgmtv3.ClusterDriverLocal ||
		cluster.Status.Driver == apimgmtv3.ClusterDriverImported ||
		cluster.Status.Driver == apimgmtv3.ClusterDriverK3s ||
		cluster.Status.Driver == apimgmtv3.ClusterDriverK3os ||
		cluster.Status.Driver == apimgmtv3.ClusterDriverRke2 ||
		cluster.Status.Driver == apimgmtv3.ClusterDriverRancherD
}

func (p *Provisioner) getConfig(spec apimgmtv3.ClusterSpec) (*apimgmtv3.ClusterSpec, interface{}, error) {
	var v interface{}
	if spec.GenericEngineConfig == nil {
		v = map[string]interface{}{}
	} else {
		v = *spec.GenericEngineConfig
	}

	return &spec, v, nil
}

func (p *Provisioner) validateDriver(cluster *apimgmtv3.Cluster) (string, error) {
	oldDriver := cluster.Status.Driver

	if oldDriver == apimgmtv3.ClusterDriverImported {
		return apimgmtv3.ClusterDriverImported, nil
	}

	newDriver, err := kontainerdriver.GetDriver(cluster, p.KontainerDriverLister)
	if err != nil {
		return "", err
	}

	if oldDriver == "" && newDriver == "" {
		return newDriver, nil
	}

	if oldDriver == "" {
		return newDriver, nil
	}

	if newDriver == "" {
		return "", &controller.ForgetError{
			Err:    fmt.Errorf("waiting for nodes"),
			Reason: "Pending",
		}
	}

	if oldDriver != newDriver {
		return newDriver, fmt.Errorf("driver change from %s to %s not allowed", oldDriver, newDriver)
	}

	return newDriver, nil
}

// getSpec computes the spec from the given cluster. Returns an error if the spec could not be computed. Otherwise,
// returns the spec and a boolean that will be true if the cluster config has changed and false otherwise.
func (p *Provisioner) getSpec(cluster *apimgmtv3.Cluster) (spec *apimgmtv3.ClusterSpec, configChanged bool, err error) {
	censoredOldSpec, err := p.censorGenericEngineConfig(cluster.Status.AppliedSpec)
	if err != nil {
		return nil, false, err
	}

	_, oldConfig, err := p.getConfig(censoredOldSpec)
	if err != nil {
		return nil, false, err
	}

	censoredSpec, err := p.censorGenericEngineConfig(cluster.Spec)
	if err != nil {
		return nil, false, err
	}

	_, newConfig, err := p.getConfig(censoredSpec)
	if err != nil {
		return nil, false, err
	}

	newSpec, _, err := p.getConfig(cluster.Spec)
	return newSpec, !reflect.DeepEqual(oldConfig, newConfig), err
}

func (p *Provisioner) recordFailure(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec, err error) (*apimgmtv3.Cluster, error) {
	if err == nil {
		p.backoff.DeleteEntry(cluster.Name)
		if cluster.Status.FailedSpec == nil {
			return cluster, nil
		}

		cluster.Status.FailedSpec = nil
		return p.Clusters.Update(cluster)
	}

	p.backoff.Next(cluster.Name, time.Now())
	cluster.Status.FailedSpec = &spec
	newCluster, _ := p.Clusters.Update(cluster)
	// mask the error
	return newCluster, nil
}

// transform an imported cluster into a k3s or k3os cluster using its discovered version
func (p *Provisioner) k3sBasedClusterConfig(cluster *apimgmtv3.Cluster, nodes []*apimgmtv3.Node) error {
	// version is not found until cluster is provisioned
	if cluster.Status.Driver == "" || cluster.Status.Version == nil || len(nodes) == 0 {
		return &controller.ForgetError{
			Err:    fmt.Errorf("waiting for full cluster configuration"),
			Reason: "Pending"}
	}
	if cluster.Status.Driver == apimgmtv3.ClusterDriverK3s ||
		cluster.Status.Driver == apimgmtv3.ClusterDriverK3os ||
		cluster.Status.Driver == apimgmtv3.ClusterDriverRke2 ||
		cluster.Status.Driver == apimgmtv3.ClusterDriverRancherD ||
		imported.IsAdministratedByProvisioningCluster(cluster) {
		return nil // no-op
	}
	isEmbedded := cluster.Status.Driver == apimgmtv3.ClusterDriverLocal

	if strings.Contains(cluster.Status.Version.String(), "k3s") {
		for _, node := range nodes {
			if _, ok := node.Status.NodeLabels["k3os.io/mode"]; ok {
				cluster.Status.Driver = apimgmtv3.ClusterDriverK3os
				break
			}
		}
		if cluster.Status.Driver != apimgmtv3.ClusterDriverK3os {
			cluster.Status.Driver = apimgmtv3.ClusterDriverK3s
		}
		// only set these values on init, and not for embedded clusters as those shouldn't be upgraded
		if cluster.Spec.K3sConfig == nil && !isEmbedded {
			cluster.Spec.K3sConfig = &apimgmtv3.K3sConfig{
				Version: cluster.Status.Version.String(),
			}
			cluster.Spec.K3sConfig.SetStrategy(1, 1)
		}
	} else if strings.Contains(cluster.Status.Version.String(), "rke2") {

		_, err := p.DaemonsetLister.Get("cattle-system", "rancher")
		if apierrors.IsNotFound(err) {
			cluster.Status.Driver = apimgmtv3.ClusterDriverRke2
		} else if err != nil {
			return err
		} else {
			cluster.Status.Driver = apimgmtv3.ClusterDriverRancherD
			return nil
		}
		if cluster.Spec.Rke2Config == nil {
			cluster.Spec.Rke2Config = &apimgmtv3.Rke2Config{
				Version: cluster.Status.Version.String(),
			}
			cluster.Spec.Rke2Config.SetStrategy(1, 1)
		}
	}
	return nil
}

func reconcileACE(cluster *apimgmtv3.Cluster) {
	if imported.IsAdministratedByProvisioningCluster(cluster) || cluster.Status.Driver == apimgmtv3.ClusterDriverRke2 || cluster.Status.Driver == apimgmtv3.ClusterDriverK3s {
		cluster.Status.AppliedSpec.LocalClusterAuthEndpoint = cluster.Spec.LocalClusterAuthEndpoint
	}
}
