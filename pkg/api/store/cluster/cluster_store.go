package cluster

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	ccluster "github.com/rancher/rancher/pkg/api/customization/cluster"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/controllers/management/clusterstatus"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementv3 "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	DefaultBackupIntervalHours = 12
	DefaultBackupRetention     = 6
)

type Store struct {
	types.Store
	ShellHandler          types.RequestHandler
	mu                    sync.Mutex
	KontainerDriverLister v3.KontainerDriverLister
}

type transformer struct {
	KontainerDriverLister v3.KontainerDriverLister
}

func (t *transformer) TransformerFunc(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
	data = transformSetNilSnapshotFalse(data)
	return t.transposeGenericConfigToDynamicField(data)
}

//transposeGenericConfigToDynamicField converts a genericConfig to one usable by rancher and maps a kontainer id to a kontainer name
func (t *transformer) transposeGenericConfigToDynamicField(data map[string]interface{}) (map[string]interface{}, error) {
	if data["genericEngineConfig"] != nil {
		drivers, err := t.KontainerDriverLister.List("", labels.Everything())
		if err != nil {
			return nil, err
		}

		var driver *v3.KontainerDriver
		driverName := data["genericEngineConfig"].(map[string]interface{})[clusterprovisioner.DriverNameField].(string)
		// iterate over kontainer drivers to find the one that maps to the genericEngineConfig DriverName ("kd-**") -> "example"
		for _, candidate := range drivers {
			if driverName == candidate.Name {
				driver = candidate
				break
			}
		}
		if driver == nil {
			logrus.Warnf("unable to find the kontainer driver %v that maps to %v", driverName, data[clusterprovisioner.DriverNameField])
			return data, nil
		}

		var driverTypeName string
		if driver.Spec.BuiltIn {
			driverTypeName = driver.Status.DisplayName + "Config"
		} else {
			driverTypeName = driver.Status.DisplayName + "EngineConfig"
		}

		data[driverTypeName] = data["genericEngineConfig"]
		delete(data, "genericEngineConfig")
	}

	return data, nil
}

func SetClusterStore(schema *types.Schema, mgmt *config.ScaledContext, clusterManager *clustermanager.Manager, k8sProxy http.Handler) {
	transformer := transformer{
		KontainerDriverLister: mgmt.Management.KontainerDrivers("").Controller().Lister(),
	}
	t := &transform.Store{
		Store:       schema.Store,
		Transformer: transformer.TransformerFunc,
	}

	linkHandler := &ccluster.ShellLinkHandler{
		Proxy:          k8sProxy,
		ClusterManager: clusterManager,
	}

	s := &Store{
		Store:                 t,
		KontainerDriverLister: mgmt.Management.KontainerDrivers("").Controller().Lister(),
		ShellHandler:          linkHandler.LinkHandler,
	}

	schema.Store = s
}

func transformSetNilSnapshotFalse(data map[string]interface{}) map[string]interface{} {
	var (
		etcd  interface{}
		found bool
	)

	etcd, found = values.GetValue(data, "appliedSpec", "rancherKubernetesEngineConfig", "services", "etcd")
	if found {
		etcd := convert.ToMapInterface(etcd)
		val, found := values.GetValue(etcd, "snapshot")
		if !found || val == nil {
			values.PutValue(data, false, "appliedSpec", "rancherKubernetesEngineConfig", "services", "etcd", "snapshot")
		}
	}

	etcd, found = values.GetValue(data, "rancherKubernetesEngineConfig", "services", "etcd")
	if found {
		etcd := convert.ToMapInterface(etcd)
		val, found := values.GetValue(etcd, "snapshot")
		if !found || val == nil {
			values.PutValue(data, false, "rancherKubernetesEngineConfig", "services", "etcd", "snapshot")
		}
	}

	return data
}

func (r *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	// Really we want a link handler but the URL parse makes it impossible to add links to clusters for now.  So this
	// is basically a hack
	if apiContext.Query.Get("shell") == "true" {
		return nil, r.ShellHandler(apiContext, nil)
	}

	return r.Store.ByID(apiContext, schema, id)
}

func (r *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	name := convert.ToString(data["name"])
	if name == "" {
		return nil, httperror.NewFieldAPIError(httperror.MissingRequired, "Cluster name", "")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := canUseClusterName(apiContext, name); err != nil {
		return nil, err
	}

	setKubernetesVersion(data)
	// enable local backups for rke clusters by default
	enableLocalBackup(data)

	data, err := r.transposeDynamicFieldToGenericConfig(data)
	if err != nil {
		return nil, err
	}

	if err := validateNetworkFlag(data, true); err != nil {
		return nil, httperror.NewFieldAPIError(httperror.InvalidOption, "enableNetworkPolicy", err.Error())
	}

	if driverName, _ := values.GetValue(data, "genericEngineConfig", "driverName"); driverName == "amazonelasticcontainerservice" {
		sessionToken, _ := values.GetValue(data, "genericEngineConfig", "sessionToken")
		annotation, _ := values.GetValue(data, managementv3.ClusterFieldAnnotations)
		m := toMap(annotation)
		m[clusterstatus.TemporaryCredentialsAnnotationKey] = strconv.FormatBool(
			sessionToken != "" && sessionToken != nil)
		values.PutValue(data, m, managementv3.ClusterFieldAnnotations)
	}

	if err = setInitialConditions(data); err != nil {
		return nil, err
	}

	return r.Store.Create(apiContext, schema, data)
}

func setInitialConditions(data map[string]interface{}) error {
	if data[managementv3.ClusterStatusFieldConditions] == nil {
		data[managementv3.ClusterStatusFieldConditions] = []map[string]interface{}{}
	}

	conditions, ok := data[managementv3.ClusterStatusFieldConditions].([]map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to parse field \"%v\" type \"%v\" as \"[]map[string]interface{}\"",
			managementv3.ClusterStatusFieldConditions, reflect.TypeOf(data[managementv3.ClusterStatusFieldConditions]))
	}
	for key := range data {
		if strings.Index(key, "Config") == len(key)-6 {
			data[managementv3.ClusterStatusFieldConditions] =
				append(
					conditions,
					[]map[string]interface{}{
						{
							"status": "True",
							"type":   string(v3.ClusterConditionPending),
						},
						{
							"status": "Unknown",
							"type":   string(v3.ClusterConditionProvisioned),
						},
						{
							"status": "Unknown",
							"type":   string(v3.ClusterConditionWaiting),
						},
					}...,
				)
		}
	}

	return nil
}

func toMap(rawMap interface{}) map[string]interface{} {
	if theMap, ok := rawMap.(map[string]interface{}); ok {
		return theMap
	}

	return make(map[string]interface{})
}

func (r *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	updatedName := convert.ToString(data["name"])
	if updatedName == "" {
		return nil, httperror.NewFieldAPIError(httperror.MissingRequired, "Cluster name", "")
	}

	existingCluster, err := r.ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}

	clusterName, ok := existingCluster["name"].(string)
	if !ok {
		clusterName = ""
	}

	if !strings.EqualFold(updatedName, clusterName) {
		r.mu.Lock()
		defer r.mu.Unlock()

		if err := canUseClusterName(apiContext, updatedName); err != nil {
			return nil, err
		}
	}

	setKubernetesVersion(data)

	data, err = r.transposeDynamicFieldToGenericConfig(data)
	if err != nil {
		return nil, err
	}

	if err := validateNetworkFlag(data, false); err != nil {
		return nil, httperror.NewFieldAPIError(httperror.InvalidOption, "enableNetworkPolicy", err.Error())
	}

	setBackupConfigSecretKeyIfNotExists(existingCluster, data)
	setPrivateRegistryPasswordIfNotExists(existingCluster, data)
	setCloudProviderPasswordFieldsIfNotExists(existingCluster, data)

	return r.Store.Update(apiContext, schema, data, id)
}

// this method moves the cluster config to and from the genericEngineConfig field so that
// the kontainer drivers behave similarly to the existing machine drivers
func (r *Store) transposeDynamicFieldToGenericConfig(data map[string]interface{}) (map[string]interface{}, error) {
	dynamicField, err := r.getDynamicField(data)
	if err != nil {
		return nil, fmt.Errorf("error getting kontainer drivers: %v", err)
	}

	// No dynamic schema field exists on this cluster so return immediately
	if dynamicField == "" {
		return data, nil
	}

	// overwrite generic engine config so it gets saved
	data["genericEngineConfig"] = data[dynamicField]
	delete(data, dynamicField)

	return data, nil
}

func (r *Store) getDynamicField(data map[string]interface{}) (string, error) {
	drivers, err := r.KontainerDriverLister.List("", labels.Everything())
	if err != nil {
		return "", err
	}

	for _, driver := range drivers {
		var driverName string
		if driver.Spec.BuiltIn {
			driverName = driver.Status.DisplayName + "Config"
		} else {
			driverName = driver.Status.DisplayName + "EngineConfig"
		}

		if data[driverName] != nil {
			if !(driver.Status.DisplayName == "rancherKubernetesEngine" || driver.Status.DisplayName == "import") {
				return driverName, nil
			}
		}
	}

	return "", nil
}

func canUseClusterName(apiContext *types.APIContext, requestedName string) error {
	var clusters []managementv3.Cluster

	if err := access.List(apiContext, apiContext.Version, managementv3.ClusterType, &types.QueryOptions{}, &clusters); err != nil {
		return err
	}

	for _, c := range clusters {
		if c.Removed == "" && strings.EqualFold(c.Name, requestedName) {
			//cluster exists by this name
			return httperror.NewFieldAPIError(httperror.NotUnique, "Cluster name", "")
		}
	}

	return nil
}

func setKubernetesVersion(data map[string]interface{}) {
	rkeConfig, ok := values.GetValue(data, "rancherKubernetesEngineConfig")

	if ok && rkeConfig != nil {
		k8sVersion := values.GetValueN(data, "rancherKubernetesEngineConfig", "kubernetesVersion")
		if k8sVersion == nil || k8sVersion == "" {
			//set k8s version to system default on the spec
			defaultVersion := settings.KubernetesVersion.Get()
			values.PutValue(data, defaultVersion, "rancherKubernetesEngineConfig", "kubernetesVersion")
		}
	}
}

func validateNetworkFlag(data map[string]interface{}, create bool) error {
	enableNetworkPolicy := values.GetValueN(data, "enableNetworkPolicy")
	rkeConfig := values.GetValueN(data, "rancherKubernetesEngineConfig")
	plugin := convert.ToString(values.GetValueN(convert.ToMapInterface(rkeConfig), "network", "plugin"))

	if enableNetworkPolicy == nil {
		// setting default values for new clusters if value not passed
		values.PutValue(data, false, "enableNetworkPolicy")
	} else if value := convert.ToBool(enableNetworkPolicy); value {
		if rkeConfig == nil {
			if create {
				values.PutValue(data, false, "enableNetworkPolicy")
				return nil
			}
			return fmt.Errorf("enableNetworkPolicy should be false for non-RKE clusters")
		}
		if plugin != "canal" {
			return fmt.Errorf("plugin %s should have enableNetworkPolicy %v", plugin, !value)
		}
	}

	return nil
}

func enableLocalBackup(data map[string]interface{}) {
	rkeConfig, ok := values.GetValue(data, "rancherKubernetesEngineConfig")

	if ok && rkeConfig != nil {
		legacyConfig := values.GetValueN(data, "rancherKubernetesEngineConfig", "services", "etcd", "snapshot")
		if legacyConfig != nil && legacyConfig.(bool) { //  don't enable rancher backup if legacy is enabled.
			return
		}
		backupConfig := values.GetValueN(data, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig")

		if backupConfig == nil {
			enabled := true
			backupConfig = &v3.BackupConfig{
				Enabled:       &enabled,
				IntervalHours: DefaultBackupIntervalHours,
				Retention:     DefaultBackupRetention,
			}
			// enable rancher etcd backup
			values.PutValue(data, backupConfig, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig")
		}
	}
}

func setBackupConfigSecretKeyIfNotExists(oldData, newData map[string]interface{}) {
	s3BackupConfig := values.GetValueN(newData, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig", "s3BackupConfig")
	if s3BackupConfig == nil {
		return
	}
	val := convert.ToMapInterface(s3BackupConfig)
	if val["secretKey"] != nil {
		return
	}
	oldSecretKey := convert.ToString(values.GetValueN(oldData, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig", "s3BackupConfig", "secretKey"))
	if oldSecretKey != "" {
		values.PutValue(newData, oldSecretKey, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig", "s3BackupConfig", "secretKey")
	}
}

func setPrivateRegistryPasswordIfNotExists(oldData, newData map[string]interface{}) {
	newSlice, ok := values.GetSlice(newData, "rancherKubernetesEngineConfig", "privateRegistries")
	if !ok || newSlice == nil {
		return
	}
	oldSlice, ok := values.GetSlice(oldData, "rancherKubernetesEngineConfig", "privateRegistries")
	if !ok || oldSlice == nil {
		return
	}

	var updatedConfig []map[string]interface{}
	for _, newConfig := range newSlice {
		if newConfig["password"] != nil {
			updatedConfig = append(updatedConfig, newConfig)
			continue
		}
		for _, oldConfig := range oldSlice {
			if newConfig["url"] == oldConfig["url"] && newConfig["user"] == oldConfig["user"] &&
				oldConfig["password"] != nil {
				newConfig["password"] = oldConfig["password"]
				break
			}
		}
		updatedConfig = append(updatedConfig, newConfig)
	}
	values.PutValue(newData, updatedConfig, "rancherKubernetesEngineConfig", "privateRegistries")
}

func setCloudProviderPasswordFieldsIfNotExists(oldData, newData map[string]interface{}) {
	replaceWithOldSecretIfNotExists(oldData, newData, "openstackCloudProvider", "rancherKubernetesEngineConfig", "cloudProvider", "openstackCloudProvider", "global", "password")
	replaceWithOldSecretIfNotExists(oldData, newData, "vsphereCloudProvider", "rancherKubernetesEngineConfig", "cloudProvider", "vsphereCloudProvider", "global", "password")
	azureCloudProviderPasswordFields := []string{"aadClientSecret", "aadClientCertPassword"}
	for _, secretField := range azureCloudProviderPasswordFields {
		replaceWithOldSecretIfNotExists(oldData, newData, "azureCloudProvider", "rancherKubernetesEngineConfig", "cloudProvider", "azureCloudProvider", secretField)
	}
	vSphereCloudProviderConfig := values.GetValueN(newData, "rancherKubernetesEngineConfig", "cloudProvider", "vsphereCloudProvider")
	if vSphereCloudProviderConfig == nil {
		return
	}
	newVCenter := values.GetValueN(newData, "rancherKubernetesEngineConfig", "cloudProvider", "vsphereCloudProvider", "virtualCenter")
	if newVCenter != nil {
		oldVCenter := values.GetValueN(oldData, "rancherKubernetesEngineConfig", "cloudProvider", "vsphereCloudProvider", "virtualCenter")
		newVCenterMap := convert.ToMapInterface(newVCenter)
		oldVCenterMap := convert.ToMapInterface(oldVCenter)
		for vCenterName, vCenterConfigInterface := range newVCenterMap {
			vCenterConfig := convert.ToMapInterface(vCenterConfigInterface)
			if vCenterConfig["password"] != nil {
				continue
			}
			// new vcenter has no password provided
			// see if this vcenter exists in oldData
			if oldVCenterConfigInterface, ok := oldVCenterMap[vCenterName]; ok {
				oldVCenterConfig := convert.ToMapInterface(oldVCenterConfigInterface)
				if oldVCenterConfig["password"] != nil {
					vCenterConfig["password"] = oldVCenterConfig["password"]
					newVCenterMap[vCenterName] = vCenterConfig
				}
			}
		}
	}
}

func replaceWithOldSecretIfNotExists(oldData, newData map[string]interface{}, cloudProviderName string, keys ...string) {
	cloudProviderConfig := values.GetValueN(newData, "rancherKubernetesEngineConfig", "cloudProvider", cloudProviderName)
	if cloudProviderConfig == nil {
		return
	}
	newSecret := convert.ToString(values.GetValueN(newData, keys...))
	if newSecret != "" {
		return
	}
	oldSecret := convert.ToString(values.GetValueN(oldData, keys...))
	if oldSecret != "" {
		values.PutValue(newData, oldSecret, keys...)
	}
}
