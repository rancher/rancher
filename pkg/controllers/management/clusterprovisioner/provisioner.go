package clusterprovisioner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/norman/types/values"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	util "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/controllers/management/imported"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	v1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/rke"
	"github.com/rancher/rancher/pkg/kontainer-engine/service"
	"github.com/rancher/rancher/pkg/kontainerdriver"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/rkedialerfactory"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rke/services"
	rketypes "github.com/rancher/rke/types"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/flowcontrol"
)

const (
	RKEDriverKey          = "rancherKubernetesEngineConfig"
	KontainerEngineUpdate = "provisioner.cattle.io/ke-driver-update"
	RkeRestoreAnnotation  = "rke.cattle.io/restore"
	RKEForceUpdate        = "rke.cattle.io/force-update"
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
	Backups               v3.EtcdBackupLister
	RKESystemImages       v3.RkeK8sSystemImageInterface
	RKESystemImagesLister v3.RkeK8sSystemImageLister
	SecretLister          corev1.SecretLister
	Secrets               corev1.SecretInterface
}

func Register(ctx context.Context, management *config.ManagementContext) {
	p := &Provisioner{
		engineService:         service.NewEngineService(NewPersistentStore(management.Core.Namespaces(""), management.Core)),
		Clusters:              management.Management.Clusters(""),
		ConfigMaps:            management.Core.ConfigMaps(""),
		ClusterController:     management.Management.Clusters("").Controller(),
		NodeLister:            management.Management.Nodes("").Controller().Lister(),
		Nodes:                 management.Management.Nodes(""),
		backoff:               flowcontrol.NewBackOff(30*time.Second, 10*time.Minute),
		KontainerDriverLister: management.Management.KontainerDrivers("").Controller().Lister(),
		DynamicSchemasLister:  management.Management.DynamicSchemas("").Controller().Lister(),
		Backups:               management.Management.EtcdBackups("").Controller().Lister(),
		RKESystemImagesLister: management.Management.RkeK8sSystemImages("").Controller().Lister(),
		RKESystemImages:       management.Management.RkeK8sSystemImages(""),
		DaemonsetLister:       management.Apps.DaemonSets("").Controller().Lister(),
		SecretLister:          management.Core.Secrets("").Controller().Lister(),
		Secrets:               management.Core.Secrets(""),
	}
	// Add handlers
	p.Clusters.AddLifecycle(ctx, "cluster-provisioner-controller", p)
	management.Management.Nodes("").AddHandler(ctx, "cluster-provisioner-controller", p.machineChanged)

	local := &rkedialerfactory.RKEDialerFactory{
		Factory: management.Dialer,
		Ctx:     ctx,
	}
	docker := &rkedialerfactory.RKEDialerFactory{
		Factory: management.Dialer,
		Docker:  true,
		Ctx:     ctx,
	}

	driver := service.Drivers[service.RancherKubernetesEngineDriverName]
	rkeDriver := driver.(*rke.Driver)
	rkeDriver.DockerDialer = docker.Build
	rkeDriver.LocalDialer = local.Build
	rkeDriver.WrapTransportFactory = docker.WrapTransport
	mgmt := management.Management
	rkeDriver.DataStore = NewDataStore(mgmt.RkeAddons("").Controller().Lister(),
		mgmt.RkeAddons(""),
		mgmt.RkeK8sServiceOptions("").Controller().Lister(),
		mgmt.RkeK8sServiceOptions(""),
		mgmt.RkeK8sSystemImages("").Controller().Lister(),
		mgmt.RkeK8sSystemImages(""))
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
	default:
		return false
	}
}

func isRke1CustomCluster(cluster *apimgmtv3.Cluster, nodes []*apimgmtv3.Node) bool {
	if cluster.Status.Driver == apimgmtv3.ClusterDriverRKE {
		for _, n := range nodes {
			if n.Status.NodeTemplateSpec == nil {
				return true
			}
		}
	}
	return false
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

	nodes, err := p.NodeLister.List(cluster.Name, labels.Everything())
	if err != nil {
		return cluster, err
	}

	if isRke1CustomCluster(cluster, nodes) {
		logrus.Debugf("Skipping RKE1 Custom Cluster in favor of node-cleanup logic [%s] ", cluster.Name)
		return cluster, nil
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
	if skipOperatorCluster("update", cluster) || imported.IsAdministratedByProvisioningCluster(cluster) {
		return cluster, nil
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
	if cluster.Spec.RancherKubernetesEngineConfig != nil {
		if cluster.Spec.RancherKubernetesEngineConfig.Version == "" {
			// set version from the applied spec
			if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig != nil {
				if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.Version != "" {
					cluster.Spec.RancherKubernetesEngineConfig.Version = cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.Version
				} else {
					cluster.Spec.RancherKubernetesEngineConfig.Version = settings.KubernetesVersion.Get()
				}
			}
		}
	} else if cluster.Spec.AmazonElasticContainerServiceConfig != nil {
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

	if cluster.Spec.RancherKubernetesEngineConfig != nil || cluster.Spec.GenericEngineConfig != nil {
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
		if driver == apimgmtv3.ClusterDriverRKE && cluster.Spec.RancherKubernetesEngineConfig == nil {
			cluster.Spec.RancherKubernetesEngineConfig = &rketypes.RancherKubernetesEngineConfig{}
		}
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

var errKeyRotationFailed = errors.New("encryption key rotation failed, please restore your cluster from backup")

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
		driverName, err := p.validateDriver(cluster)
		if err != nil {
			return nil, err
		}

		spec, _, err := p.getConfig(true, cluster.Spec, driverName, cluster.Name)
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

	// If there is no change to cluster config and the force update annotation is not set to true, a reconcile is
	// not necessary, so we can just return early.
	forceUpdate := cluster.Annotations != nil && cluster.Annotations[RKEForceUpdate] == "true"
	if !configChanged && !forceUpdate {
		return cluster, nil
	}

	if ok, delay := p.backoffFailure(cluster, spec); ok {
		return cluster, &controller.ForgetError{Err: fmt.Errorf("backing off failure, delay: %v", delay)}
	}

	logrus.Infof("Provisioning cluster [%s]", cluster.Name)
	var updateTriggered bool
	if create {
		logrus.Infof("Creating cluster [%s]", cluster.Name)
		// setting updateTriggered to true since rke up will be called on cluster create
		updateTriggered = true
		apiEndpoint, serviceAccountToken, caCert, err = p.driverCreate(cluster, *spec)
		if err != nil && err.Error() == "cluster already exists" {
			logrus.Infof("Create done, Updating cluster [%s]", cluster.Name)
			apiEndpoint, serviceAccountToken, caCert, updateTriggered, err = p.driverUpdate(cluster, *spec, forceUpdate)
		}
	} else if spec.RancherKubernetesEngineConfig != nil && spec.RancherKubernetesEngineConfig.Restore.Restore {
		logrus.Infof("Restoring cluster [%s] from backup", cluster.Name)
		// cluster may need to be restored if key rotation fails
		// ensure restore does not get short-circuited by key rotation since RKE checks for key rotation before restore
		spec.RancherKubernetesEngineConfig.RotateEncryptionKey = false
		apiEndpoint, serviceAccountToken, caCert, err = p.restoreClusterBackup(cluster, *spec)
	} else if strings.Contains(apimgmtv3.ClusterConditionUpdated.GetMessage(cluster), errKeyRotationFailed.Error()) {
		logrus.Infof("Key rotation failed, cluster needs to be restored. Skipping driver updates.")
		return cluster, nil // prevent driver updates if the cluster needs to be restored
	} else if spec.RancherKubernetesEngineConfig != nil && spec.RancherKubernetesEngineConfig.RotateCertificates != nil {
		logrus.Infof("Rotating certificates for cluster [%s]", cluster.Name)
		apiEndpoint, serviceAccountToken, caCert, updateTriggered, err = p.driverUpdate(cluster, *spec, forceUpdate)
	} else if spec.RancherKubernetesEngineConfig != nil && spec.RancherKubernetesEngineConfig.RotateEncryptionKey {
		logrus.Infof("Rotating encryption key for cluster [%s]", cluster.Name)
		apiEndpoint, serviceAccountToken, caCert, updateTriggered, err = p.driverUpdate(cluster, *spec, forceUpdate)
		if err != nil {
			logrus.Errorf("[reconcileCluster] Encryption key rotation error: %v", err)
			// an error during key rotation means the user has to restore their cluster
			err = errKeyRotationFailed
		}
	} else {
		logrus.Infof("Updating cluster [%s]", cluster.Name)

		// Attempt to manually trigger updating, otherwise it will not be triggered until after exiting reconcile
		apimgmtv3.ClusterConditionUpdated.Unknown(cluster)
		cluster, err = p.Clusters.Update(cluster)
		if err != nil {
			return cluster, fmt.Errorf("[reconcileCluster] Failed to update cluster [%s]: %v", cluster.Name, err)
		}

		apiEndpoint, serviceAccountToken, caCert, updateTriggered, err = p.driverUpdate(cluster, *spec, forceUpdate)
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
		resetRkeConfigFlags(cluster, updateTriggered)

		// initialize on first rke up
		if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig != nil && cluster.Status.NodeVersion == 0 {
			cluster.Status.NodeVersion++
		}

		// Remove the force reconcile annotation from the cluster since it has been reconciled successfully.
		delete(cluster.Annotations, RKEForceUpdate)

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

	if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig != nil {
		logrus.Infof("Updated cluster [%s] with node version [%v]", cluster.Name, cluster.Status.NodeVersion)
		p.reconcileForUpgrade(cluster.Name)
	}

	logrus.Infof("Provisioned cluster [%s]", cluster.Name)
	return cluster, nil
}

func (p *Provisioner) reconcileForUpgrade(clusterName string) {
	p.Nodes.Controller().Enqueue(clusterName, "upgrade_")
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

func resetRkeConfigFlags(cluster *apimgmtv3.Cluster, updateTriggered bool) {
	if cluster.Spec.RancherKubernetesEngineConfig != nil {
		cluster.Spec.RancherKubernetesEngineConfig.RotateEncryptionKey = false
		cluster.Spec.RancherKubernetesEngineConfig.RotateCertificates = nil
		if cluster.Spec.RancherKubernetesEngineConfig.Restore.Restore {
			cluster.Annotations[RkeRestoreAnnotation] = "true"
			cluster.Status.NodeVersion++
		}
		cluster.Spec.RancherKubernetesEngineConfig.Restore = rketypes.RestoreConfig{}
		if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig != nil {
			cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.RotateCertificates = nil
			cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.Restore = rketypes.RestoreConfig{}
		}
		if !updateTriggered {
			return
		}
		if cluster.Status.Capabilities.TaintSupport == nil || !*cluster.Status.Capabilities.TaintSupport {
			supportsTaints := true
			cluster.Status.Capabilities.TaintSupport = &supportsTaints
		}
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

func (p *Provisioner) getConfig(reconcileRKE bool, spec apimgmtv3.ClusterSpec, driverName, clusterName string) (*apimgmtv3.ClusterSpec, interface{}, error) {
	var v interface{}
	if spec.GenericEngineConfig == nil {
		if spec.RancherKubernetesEngineConfig != nil {
			var err error
			v, err = convert.EncodeToMap(spec.RancherKubernetesEngineConfig)
			if err != nil {
				return nil, nil, err
			}
		} else {
			v = map[string]interface{}{}
		}
	} else {
		v = *spec.GenericEngineConfig
	}

	if driverName == apimgmtv3.ClusterDriverRKE && spec.RancherKubernetesEngineConfig != nil {
		spec.RancherKubernetesEngineConfig = spec.RancherKubernetesEngineConfig.DeepCopy()

		if reconcileRKE {
			nodes, err := p.reconcileRKENodes(clusterName)
			if err != nil {
				return nil, nil, err
			}
			spec.RancherKubernetesEngineConfig.Nodes = nodes
		}

		systemImages, err := p.getSystemImages(spec)
		if err != nil {
			return nil, nil, err
		}

		spec.RancherKubernetesEngineConfig.SystemImages = *systemImages
		data, _ := convert.EncodeToMap(spec)
		v, _ = data[RKEDriverKey]
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

func (p *Provisioner) getSystemImages(spec apimgmtv3.ClusterSpec) (*rketypes.RKESystemImages, error) {
	version := spec.RancherKubernetesEngineConfig.Version
	if version == "" {
		return nil, fmt.Errorf("kubernetes version (spec.rancherKubernetesEngineConfig.kubernetesVersion) is unset")
	}
	// fetch system images from settings
	systemImages, err := kd.GetRKESystemImages(version, p.RKESystemImagesLister, p.RKESystemImages)
	if err != nil {
		return nil, fmt.Errorf("failed to find system images for version %s: %v", version, err)
	}

	privateRegistry := util.GetPrivateRegistryURL(&apimgmtv3.Cluster{Spec: spec})
	if privateRegistry == "" {
		return &systemImages, nil
	}

	// prepend private repo
	imagesMap, err := convert.EncodeToMap(systemImages)
	if err != nil {
		return nil, err
	}
	updatedMap := make(map[string]interface{})
	for key, value := range imagesMap {
		newValue := fmt.Sprintf("%s/%s", privateRegistry, value)
		updatedMap[key] = newValue
	}
	// Decoding updateMap to systemImages using json marshal/unmarshal to honor field names
	updatedByte, err := json.Marshal(updatedMap)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(updatedByte, &systemImages)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("Updated system images to use private registry [%s]: %#v", privateRegistry, systemImages)
	return &systemImages, nil
}

// getSpec computes the spec from the given cluster. Returns an error if the spec could not be computed. Otherwise,
// returns the spec and a boolean that will be true if the cluster config has changed and false otherwise.
func (p *Provisioner) getSpec(cluster *apimgmtv3.Cluster) (spec *apimgmtv3.ClusterSpec, configChanged bool, err error) {
	driverName, err := p.validateDriver(cluster)
	if err != nil {
		return nil, false, err
	}

	censoredOldSpec, err := p.censorGenericEngineConfig(cluster.Status.AppliedSpec)
	if err != nil {
		return nil, false, err
	}

	_, oldConfig, err := p.getConfig(false, censoredOldSpec, driverName, cluster.Name)
	if err != nil {
		return nil, false, err
	}

	censoredSpec, err := p.censorGenericEngineConfig(cluster.Spec)
	if err != nil {
		return nil, false, err
	}

	_, newConfig, err := p.getConfig(true, censoredSpec, driverName, cluster.Name)
	if err != nil {
		return nil, false, err
	}

	newSpec, _, err := p.getConfig(true, cluster.Spec, driverName, cluster.Name)
	return newSpec, !reflect.DeepEqual(oldConfig, newConfig), err
}

func (p *Provisioner) reconcileRKENodes(clusterName string) ([]rketypes.RKEConfigNode, error) {
	machines, err := p.NodeLister.List(clusterName, labels.Everything())
	if err != nil {
		return nil, err
	}

	var etcd, controlplane, worker bool
	var nodes []rketypes.RKEConfigNode
	for _, machine := range machines {
		if machine.DeletionTimestamp != nil {
			continue
		}

		if apimgmtv3.NodeConditionProvisioned.IsUnknown(machine) && (machine.Spec.Etcd || machine.Spec.ControlPlane) {
			return nil, &controller.ForgetError{
				Err:    fmt.Errorf("waiting for %s to finish provisioning", machine.Spec.RequestedHostname),
				Reason: "Provisioning",
			}
		}

		if machine.Status.NodeConfig == nil {
			continue
		}

		if len(machine.Status.NodeConfig.Role) == 0 {
			continue
		}

		if !apimgmtv3.NodeConditionProvisioned.IsTrue(machine) {
			continue
		}

		if slice.ContainsString(machine.Status.NodeConfig.Role, services.ETCDRole) {
			etcd = true
		}
		if slice.ContainsString(machine.Status.NodeConfig.Role, services.ControlRole) {
			controlplane = true
		}
		if slice.ContainsString(machine.Status.NodeConfig.Role, services.WorkerRole) {
			worker = true
		}

		node := *machine.Status.NodeConfig
		if node.User == "" {
			node.User = "root"
		}
		if node.Port == "" {
			node.Port = "22"
		}
		if node.NodeName == "" {
			node.NodeName = ref.FromStrings(machine.Namespace, machine.Name)
		}
		nodes = append(nodes, node)
	}

	if !etcd || !controlplane || !worker {
		return nil, &controller.ForgetError{
			Err:    fmt.Errorf("waiting for etcd, controlplane and worker nodes to be registered"),
			Reason: "Provisioning",
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].NodeName < nodes[j].NodeName
	})

	return nodes, nil
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

func (p *Provisioner) restoreClusterBackup(cluster *apimgmtv3.Cluster, spec apimgmtv3.ClusterSpec) (api string, token string, cert string, err error) {
	snapshot := strings.Split(spec.RancherKubernetesEngineConfig.Restore.SnapshotName, ":")[1]
	backup, err := p.Backups.Get(cluster.Name, snapshot)
	if err != nil {
		return "", "", "", err
	}
	if backup.Spec.ClusterID != cluster.Name {
		return "", "", "", fmt.Errorf("snapshot [%s] is not a backup of cluster [%s]", backup.Name, cluster.Name)
	}

	api, token, cert, err = p.driverRestore(cluster, spec, GetBackupFilename(backup))
	if err != nil {
		return "", "", "", err
	}
	// checking if we have s3 config and that it's not inconsistent. This happens
	// when restore is performed with invalid credentials and then the cluster is updated to fix it.
	if spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig != nil {
		s3Config := cluster.Spec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig
		appliedS3Conf := cluster.Status.AppliedSpec.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig

		if !reflect.DeepEqual(s3Config, appliedS3Conf) {
			logrus.Infof("updated spec during restore detected for cluster [%s], update is required", cluster.Name)
			api, token, cert, _, err = p.driverUpdate(cluster, spec, false)
		}
	}
	return api, token, cert, err
}

func GetBackupFilenameFromURL(URL string) (string, error) {
	if !isValidURL(URL) {
		return "", fmt.Errorf("URL is not valid: [%s]", URL)
	}
	parsedURL, err := url.Parse(URL)
	if err != nil {
		return "", err
	}
	if parsedURL.Path == "" {
		return "", fmt.Errorf("No path found in URL: [%s]", URL)
	}
	extractedPath := path.Base(parsedURL.Path)
	return extractedPath, nil
}

// isValidURL tests a string to determine if it is a url or not.
// https://golangcode.com/how-to-check-if-a-string-is-a-url/
func isValidURL(URL string) bool {
	_, err := url.ParseRequestURI(URL)
	if err != nil {
		return false
	}
	return true
}

func GetBackupFilename(backup *apimgmtv3.EtcdBackup) string {
	snapshot := backup.Name
	if filename, err := GetBackupFilenameFromURL(backup.Spec.Filename); err == nil { // s3 file
		// need to remove extension
		snapshot = strings.TrimSuffix(filename, path.Ext(filename))
	} else if len(backup.Spec.Filename) != 0 { // not s3 url
		snapshot = strings.TrimSuffix(backup.Spec.Filename, path.Ext(backup.Spec.Filename))
	}
	return snapshot
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
