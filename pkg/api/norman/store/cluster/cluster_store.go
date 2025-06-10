package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/blang/semver"
	mVersion "github.com/mcuadros/go-version"
	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/norman/types/values"
	ccluster "github.com/rancher/rancher/pkg/api/norman/customization/cluster"
	"github.com/rancher/rancher/pkg/api/norman/customization/clustertemplate"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/controllers/management/clusterstatus"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	rkedefaults "github.com/rancher/rke/cluster"
	"github.com/rancher/rke/k8s"
	rketypes "github.com/rancher/rke/types"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
)

const (
	DefaultBackupIntervalHours          = 12
	DefaultBackupRetention              = 6
	s3TransportTimeout                  = 10
	clusterSecrets                      = "clusterSecrets"
	registrySecretKey                   = "privateRegistrySecret"
	s3SecretKey                         = "s3CredentialSecret"
	weaveSecretKey                      = "weavePasswordSecret"
	vsphereSecretKey                    = "vsphereSecret"
	virtualCenterSecretKey              = "virtualCenterSecret"
	openStackSecretKey                  = "openStackSecret"
	aadClientSecretKey                  = "aadClientSecret"
	aadClientCertSecretKey              = "aadClientCertSecret"
	aciAPICUserKeySecretKey             = "aciAPICUserKeySecret"
	aciTokenSecretKey                   = "aciTokenSecret"
	aciKafkaClientKeySecretKey          = "aciKafkaClientKeySecret"
	secretsEncryptionProvidersSecretKey = "secretsEncryptionProvidersSecret"
	bastionHostSSHKeySecretKey          = "bastionHostSSHKeySecret"
	kubeletExtraEnvSecretKey            = "kubeletExtraEnvSecret"
	privateRegistryECRSecretKey         = "privateRegistryECRSecret"
	clusterCRIDockerdAnn                = "io.cattle.cluster.cridockerd.enable"
)

type Store struct {
	types.Store
	ShellHandler                  types.RequestHandler
	mu                            sync.Mutex
	KontainerDriverLister         v3.KontainerDriverLister
	ClusterTemplateLister         v3.ClusterTemplateLister
	ClusterTemplateRevisionLister v3.ClusterTemplateRevisionLister
	NodeLister                    v3.NodeLister
	ClusterLister                 v3.ClusterLister
	DialerFactory                 dialer.Factory
	ClusterClient                 dynamic.ResourceInterface
	SecretClient                  v1.SecretInterface
	SecretLister                  v1.SecretLister
	secretMigrator                *secretmigrator.Migrator
}

type transformer struct {
	KontainerDriverLister v3.KontainerDriverLister
}

func (t *transformer) TransformerFunc(_ *types.APIContext, _ *types.Schema, data map[string]interface{}, _ *types.QueryOptions) (map[string]interface{}, error) {
	data = transformSetNilSnapshotFalse(data)
	return t.transposeGenericConfigToDynamicField(data)
}

// transposeGenericConfigToDynamicField converts a genericConfig to one usable by rancher and maps a kontainer id to a kontainer name
func (t *transformer) transposeGenericConfigToDynamicField(data map[string]interface{}) (map[string]interface{}, error) {
	if data["genericEngineConfig"] != nil {
		drivers, err := t.KontainerDriverLister.List("", labels.Everything())
		if err != nil {
			return nil, err
		}

		var driver *apimgmtv3.KontainerDriver
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

func GetClusterStore(schema *types.Schema, mgmt *config.ScaledContext, clusterManager *clustermanager.Manager, k8sProxy http.Handler) *Store {

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
		Store:                         t,
		KontainerDriverLister:         mgmt.Management.KontainerDrivers("").Controller().Lister(),
		ShellHandler:                  linkHandler.LinkHandler,
		ClusterTemplateLister:         mgmt.Management.ClusterTemplates("").Controller().Lister(),
		ClusterTemplateRevisionLister: mgmt.Management.ClusterTemplateRevisions("").Controller().Lister(),
		ClusterLister:                 mgmt.Management.Clusters("").Controller().Lister(),
		NodeLister:                    mgmt.Management.Nodes("").Controller().Lister(),
		DialerFactory:                 mgmt.Dialer,
		SecretLister:                  mgmt.Core.Secrets("").Controller().Lister(),
		secretMigrator: secretmigrator.NewMigrator(
			mgmt.Core.Secrets("").Controller().Lister(),
			mgmt.Core.Secrets(""),
		),
	}

	dynamicClient, err := dynamic.NewForConfig(&mgmt.RESTConfig)
	if err != nil {
		logrus.Warnf("GetClusterStore error creating K8s dynamic client: %v", err)
	} else {
		s.ClusterClient = dynamicClient.Resource(v3.ClusterGroupVersionResource)
	}

	schema.Store = s
	return s
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

	err := setKubernetesVersion(data, true)
	if err != nil {
		return nil, err
	}
	enableCRIDockerd(data)
	setInstanceMetadataHostname(data)
	// enable local backups for rke clusters by default
	enableLocalBackup(data)
	if err := setNodeUpgradeStrategy(data, nil); err != nil {
		return nil, err
	}

	data, err = r.transposeDynamicFieldToGenericConfig(data)
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
	if err = validateKeyRotation(data); err != nil {
		return nil, err
	}
	cleanPrivateRegistry(data)

	return data, nil
}

func transposeNameFields(data map[string]interface{}, clusterConfigSchema *types.Schema) map[string]interface{} {

	if clusterConfigSchema != nil {
		for fieldName, field := range clusterConfigSchema.ResourceFields {

			if definition.IsReferenceType(field.Type) && strings.HasSuffix(fieldName, "Id") {
				dataKeyName := strings.TrimSuffix(fieldName, "Id") + "Name"
				data[fieldName] = data[dataKeyName]
				delete(data, dataKeyName)
			}
		}
	}
	return data
}

func convertRegistryMapToSliceInOrder(registryMap map[int]map[string]interface{}) ([]map[string]interface{}, error) {
	size := len(registryMap)
	registries := make([]map[string]interface{}, size)
	for k, v := range registryMap {
		if k >= size {
			return nil, fmt.Errorf("the index %d is out of the bound of the registry list (size of %d)", k, size)
		}
		registries[k] = v
		registries[k][managementv3.PrivateRegistryFieldIsDefault] = false
	}
	// set the first registry in the list as the default registry if it exists
	if len(registries) > 0 {
		registries[0][managementv3.PrivateRegistryFieldIsDefault] = true
	}
	return registries, nil
}

func getIndexFromQuestion(question string) (int, error) {
	re, err := regexp.Compile(`\[\d+\]`)
	if err != nil {
		return 0, err
	}
	target := re.FindString(question)
	if target == "" {
		return 0, fmt.Errorf("cannot get index from the question: %s", question)
	}
	return strconv.Atoi(target[1 : len(target)-1])
}

func getKeyFromQuestion(question string) (string, error) {
	re, err := regexp.Compile(`\[.+\]`)
	if err != nil {
		return "", err
	}
	target := re.FindString(question)
	if target == "" {
		return "", fmt.Errorf("cannot get key from the question: %s", question)
	}
	return target[1 : len(target)-1], nil
}

func hasTemplate(data map[string]interface{}) bool {
	templateRevID := convert.ToString(data[managementv3.ClusterSpecFieldClusterTemplateRevisionID])
	if templateRevID != "" {
		return true
	}
	return false
}

func (r *Store) validateTemplateInput(apiContext *types.APIContext, data map[string]interface{}, isUpdate bool) (*apimgmtv3.ClusterTemplateRevision, *apimgmtv3.ClusterTemplate, error) {
	if !isUpdate {
		// if data also has rkeconfig, error out on create
		rkeConfig, ok := values.GetValue(data, "rancherKubernetesEngineConfig")
		if ok && rkeConfig != nil {
			return nil, nil, fmt.Errorf("cannot set rancherKubernetesEngineConfig and clusterTemplateRevision both")
		}
	}

	var templateID, templateRevID string

	templateRevIDStr := convert.ToString(data[managementv3.ClusterSpecFieldClusterTemplateRevisionID])
	var clusterTemplateRev managementv3.ClusterTemplateRevision

	// access check.
	if err := access.ByID(apiContext, apiContext.Version, managementv3.ClusterTemplateRevisionType, templateRevIDStr, &clusterTemplateRev); err != nil {
		if apiError, ok := err.(*httperror.APIError); ok {
			if apiError.Code.Status == httperror.PermissionDenied.Status {
				return nil, nil, httperror.NewAPIError(httperror.NotFound, "The clusterTemplateRevision is not found")
			}
		}
		return nil, nil, err
	}

	splitID := strings.Split(templateRevIDStr, ":")
	if len(splitID) == 2 {
		templateRevID = splitID[1]
	}

	clusterTemplateRevision, err := r.ClusterTemplateRevisionLister.Get(namespace.GlobalNamespace, templateRevID)
	if err != nil {
		return nil, nil, err
	}

	if clusterTemplateRevision.Spec.Enabled != nil && !(*clusterTemplateRevision.Spec.Enabled) {
		return nil, nil, fmt.Errorf("cannot create cluster, clusterTemplateRevision is disabled")
	}

	templateIDStr := clusterTemplateRevision.Spec.ClusterTemplateName
	splitID = strings.Split(templateIDStr, ":")
	if len(splitID) == 2 {
		templateID = splitID[1]
	}

	clusterTemplate, err := r.ClusterTemplateLister.Get(namespace.GlobalNamespace, templateID)
	if err != nil {
		return nil, nil, err
	}

	return clusterTemplateRevision, clusterTemplate, nil

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
							"type":   string(apimgmtv3.ClusterConditionPending),
						},
						{
							"status": "Unknown",
							"type":   string(apimgmtv3.ClusterConditionProvisioned),
						},
						{
							"status": "Unknown",
							"type":   string(apimgmtv3.ClusterConditionWaiting),
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

	err = setKubernetesVersion(data, false)
	if err != nil {
		return nil, err
	}
	enableCRIDockerd(data)
	setInstanceMetadataHostname(data)
	if err := setNodeUpgradeStrategy(data, existingCluster); err != nil {
		return nil, err
	}
	data, err = r.transposeDynamicFieldToGenericConfig(data)
	if err != nil {
		return nil, err
	}

	if err := validateNetworkFlag(data, false); err != nil {
		return nil, httperror.NewFieldAPIError(httperror.InvalidOption, "enableNetworkPolicy", err.Error())
	}

	cleanPrivateRegistry(data)
	if err := validateKeyRotation(data); err != nil {
		return nil, err
	}

	// When any rancherKubernetesEngineConfig.cloudProvider.vsphereCloudProvider.virtualCenter has been removed, updating cluster using k8s dynamic client to properly
	// replace cluster spec. This is required due to r.Store.Update is merging this data instead of replacing it, https://github.com/rancher/rancher/issues/27306
	if newVCenter, ok := values.GetValue(data, "rancherKubernetesEngineConfig", "cloudProvider", "vsphereCloudProvider", "virtualCenter"); ok && newVCenter != nil {
		if oldVCenter, ok := values.GetValue(existingCluster, "rancherKubernetesEngineConfig", "cloudProvider", "vsphereCloudProvider", "virtualCenter"); ok && oldVCenter != nil {
			if oldVCenterMap, oldOk := oldVCenter.(map[string]interface{}); oldOk && oldVCenterMap != nil {
				if newVCenterMap, newOk := newVCenter.(map[string]interface{}); newOk && newVCenterMap != nil {
					for k := range oldVCenterMap {
						if _, ok := newVCenterMap[k]; !ok && oldVCenterMap[k] != nil {
							return r.updateClusterByK8sclient(apiContext.Request.Context(), id, updatedName, data)
						}
					}
				}
			}
		}
	}

	return data, err
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

func (r *Store) updateClusterByK8sclient(ctx context.Context, id, name string, data map[string]interface{}) (map[string]interface{}, error) {
	logrus.Tracef("Updating cluster [%s] using K8s dynamic client", id)
	if r.ClusterClient == nil {
		return nil, fmt.Errorf("Error updating the cluster: k8s client is nil")
	}

	object, err := r.ClusterClient.Get(ctx, id, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error updating the cluster: %v", err)
	}

	// Replacing name by displayName to properly update cluster using k8s format
	values.PutValue(data, name, "displayName")
	values.RemoveValue(data, "name")

	// Setting data as cluster spec before update
	object.Object["spec"] = data
	object, err = r.ClusterClient.Update(ctx, object, metav1.UpdateOptions{})
	if err != nil || object == nil {
		return nil, fmt.Errorf("Error updating the cluster: %v", err)
	}

	// Replacing displayName by name to properly return updated data using API format
	values.PutValue(object.Object, name, "spec", "name")
	values.RemoveValue(object.Object, "spec", "displayName")

	return object.Object["spec"].(map[string]interface{}), nil
}

func canUseClusterName(apiContext *types.APIContext, requestedName string) error {
	var clusters []managementv3.Cluster

	if err := access.List(apiContext, apiContext.Version, managementv3.ClusterType, &types.QueryOptions{}, &clusters); err != nil {
		return err
	}

	for _, c := range clusters {
		if c.Removed == "" && strings.EqualFold(c.Name, requestedName) {
			// cluster exists by this name
			return httperror.NewFieldAPIError(httperror.NotUnique, "Cluster name", "")
		}
	}

	return nil
}

func setKubernetesVersion(data map[string]interface{}, create bool) error {
	rkeConfig, ok := values.GetValue(data, "rancherKubernetesEngineConfig")
	if ok && rkeConfig != nil {
		k8sVersion := values.GetValueN(data, "rancherKubernetesEngineConfig", "kubernetesVersion")
		if k8sVersion == nil || k8sVersion == "" {
			// Only set when its a new cluster
			if create {
				// set k8s version to system default on the spec
				defaultVersion := settings.KubernetesVersion.Get()
				values.PutValue(data, defaultVersion, "rancherKubernetesEngineConfig", "kubernetesVersion")
			}
		} else {
			// if k8s version is already of rancher version form, noop
			// if k8s version is of form 1.14.x, figure out the latest
			k8sVersionRequested := convert.ToString(k8sVersion)
			if strings.Contains(k8sVersionRequested, "-rancher") {
				deprecated, err := isDeprecated(k8sVersionRequested)
				if err != nil {
					return err
				}
				if deprecated {
					return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Requested kubernetesVersion %v is deprecated", k8sVersionRequested))
				}
				return nil
			}
			translatedVersion, err := getSupportedK8sVersion(k8sVersionRequested)
			if err != nil {
				return err
			}
			if translatedVersion == "" {
				return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Requested kubernetesVersion %v is not supported currently", k8sVersionRequested))
			}
			values.PutValue(data, translatedVersion, "rancherKubernetesEngineConfig", "kubernetesVersion")
		}
	}
	return nil
}

func isDeprecated(version string) (bool, error) {
	deprecatedVersions := make(map[string]bool)
	deprecatedVersionSetting := settings.KubernetesVersionsDeprecated.Get()
	if deprecatedVersionSetting != "" {
		if err := json.Unmarshal([]byte(deprecatedVersionSetting), &deprecatedVersions); err != nil {
			return false, errors.Wrapf(err, "Error reading the setting %v", settings.KubernetesVersionsDeprecated.Name)
		}
	}
	return convert.ToBool(deprecatedVersions[version]), nil
}

func getSupportedK8sVersion(k8sVersionRequest string) (string, error) {
	_, err := clustertemplate.CheckKubernetesVersionFormat(k8sVersionRequest)
	if err != nil {
		return "", err
	}

	supportedVersions := strings.Split(settings.KubernetesVersionsCurrent.Get(), ",")
	range1, err := semver.ParseRange("=" + k8sVersionRequest)
	if err != nil {
		return "", httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Requested kubernetesVersion %v is not of valid semver [major.minor.patch] format", k8sVersionRequest))
	}

	for _, v := range supportedVersions {
		semv, err := semver.ParseTolerant(strings.Split(v, "-rancher")[0])
		if err != nil {
			return "", httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Semver translation failed for the current K8bernetes Version %v, err: %v", v, err))
		}
		if range1(semv) {
			return v, nil
		}
	}
	return "", nil
}

func validateNetworkFlag(data map[string]interface{}, create bool) error {
	enableNetworkPolicy := values.GetValueN(data, "enableNetworkPolicy")
	if enableNetworkPolicy == nil && create {
		// setting default values for new clusters if value not passed
		values.PutValue(data, false, "enableNetworkPolicy")
	} else if value := convert.ToBool(enableNetworkPolicy); value {
		rke2Config := values.GetValueN(data, "rke2Config")
		k3sConfig := values.GetValueN(data, "k3sConfig")
		if rke2Config != nil || k3sConfig != nil {
			if create {
				values.PutValue(data, false, "enableNetworkPolicy")
				return nil
			}
			return fmt.Errorf("enableNetworkPolicy should be false for k3s or rke2 clusters")
		}
	}
	return nil
}

func setNodeUpgradeStrategy(newData, oldData map[string]interface{}) error {
	rkeConfig := values.GetValueN(newData, "rancherKubernetesEngineConfig")
	if rkeConfig == nil {
		return nil
	}
	rkeConfigMap := convert.ToMapInterface(rkeConfig)
	upgradeStrategy := rkeConfigMap["upgradeStrategy"]
	oldUpgradeStrategy := values.GetValueN(oldData, "rancherKubernetesEngineConfig", "upgradeStrategy")
	if upgradeStrategy == nil {
		if oldUpgradeStrategy != nil {
			upgradeStrategy = oldUpgradeStrategy
		} else {
			upgradeStrategy = &rketypes.NodeUpgradeStrategy{
				MaxUnavailableWorker:       rkedefaults.DefaultMaxUnavailableWorker,
				MaxUnavailableControlplane: rkedefaults.DefaultMaxUnavailableControlplane,
				Drain:                      func() *bool { b := false; return &b }(),
			}
		}
		values.PutValue(newData, upgradeStrategy, "rancherKubernetesEngineConfig", "upgradeStrategy")
		return nil
	}
	upgradeStrategyMap := convert.ToMapInterface(upgradeStrategy)
	if control, ok := upgradeStrategyMap["maxUnavailableControlplane"]; ok && control != "" {
		if err := validateUnavailable(convert.ToString(control)); err != nil {
			return fmt.Errorf("maxUnavailableControlplane is invalid: %v", err)
		}
	} else {
		values.PutValue(newData, rkedefaults.DefaultMaxUnavailableControlplane, "rancherKubernetesEngineConfig", "upgradeStrategy", "maxUnavailableControlplane")
	}
	if worker, ok := upgradeStrategyMap["maxUnavailableWorker"]; ok && worker != "" {
		if err := validateUnavailable(convert.ToString(worker)); err != nil {
			return fmt.Errorf("maxUnavailableWorker is invalid: %v", err)
		}
	} else {
		values.PutValue(newData, rkedefaults.DefaultMaxUnavailableWorker, "rancherKubernetesEngineConfig", "upgradeStrategy", "maxUnavailableWorker")
	}

	nodeDrainInput := upgradeStrategyMap["nodeDrainInput"]
	if nodeDrainInput == nil {
		oldDrainInput := convert.ToMapInterface(oldUpgradeStrategy)["nodeDrainInput"]
		if oldDrainInput != nil {
			nodeDrainInput = oldDrainInput
		} else {
			ignoreDaemonSets := true
			nodeDrainInput = &apimgmtv3.NodeDrainInput{
				IgnoreDaemonSets: &ignoreDaemonSets,
				GracePeriod:      -1,
				Timeout:          120,
			}
		}
		values.PutValue(newData, nodeDrainInput, "rancherKubernetesEngineConfig", "upgradeStrategy", "nodeDrainInput")
	}
	return nil
}

func validateUnavailable(input string) error {
	parsed := intstr.Parse(input)
	if parsed.Type == intstr.Int && parsed.IntVal < 1 {
		return fmt.Errorf("value must be greater than 0: %s", input)
	} else if parsed.Type == intstr.String {
		if strings.HasPrefix(parsed.StrVal, "-") || strings.HasPrefix(parsed.StrVal, "0") {
			return fmt.Errorf("value must be greater than 0: %s", input)
		}
		s := strings.Replace(parsed.StrVal, "%", "", -1)
		if _, err := strconv.Atoi(s); err != nil {
			return fmt.Errorf("value must be valid int %s: %v", parsed.StrVal, err)
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
			backupConfig = &rketypes.BackupConfig{
				Enabled:       &enabled,
				IntervalHours: DefaultBackupIntervalHours,
				Retention:     DefaultBackupRetention,
			}
			// enable rancher etcd backup
			values.PutValue(data, backupConfig, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig")
		}
	}
}

func setInstanceMetadataHostname(data map[string]interface{}) {
	rkeConfig, ok := values.GetValue(data, "rancherKubernetesEngineConfig")
	if ok && rkeConfig != nil {
		cloudProviderName := convert.ToString(values.GetValueN(data, "rancherKubernetesEngineConfig", "cloudProvider", "name"))
		_, ok := values.GetValue(data, "rancherKubernetesEngineConfig", "cloudProvider", "useInstanceMetadataHostname")
		if !ok {
			if cloudProviderName == k8s.AWSCloudProvider {
				// set default false for in-tree aws cloud provider
				values.PutValue(data, false, "rancherKubernetesEngineConfig", "cloudProvider", "useInstanceMetadataHostname")
			} else if cloudProviderName == k8s.ExternalAWSCloudProviderName {
				// set default true for external-aws cloud provider
				values.PutValue(data, true, "rancherKubernetesEngineConfig", "cloudProvider", "useInstanceMetadataHostname")
			}
		}
	}
}

func enableCRIDockerd(data map[string]interface{}) {
	rkeConfig, ok := values.GetValue(data, "rancherKubernetesEngineConfig")
	if ok && rkeConfig != nil {
		k8sVersion := convert.ToString(values.GetValueN(data, "rancherKubernetesEngineConfig", "kubernetesVersion"))
		if mVersion.Compare(k8sVersion, "v1.24.0-rancher1-1", ">=") {
			annotation, _ := values.GetValue(data, managementv3.ClusterFieldAnnotations)
			m := toMap(annotation)
			var enableCRIDockerd124 bool
			if enable, ok := m[clusterCRIDockerdAnn]; ok && convert.ToString(enable) == "false" {
				values.PutValue(data, enableCRIDockerd124, "rancherKubernetesEngineConfig", "enableCriDockerd")
				return
			}
			enableCRIDockerd124 = true
			values.PutValue(data, enableCRIDockerd124, "rancherKubernetesEngineConfig", "enableCriDockerd")
		}
	}
}

func cleanPrivateRegistry(data map[string]interface{}) {
	registries, ok := values.GetSlice(data, "rancherKubernetesEngineConfig", "privateRegistries")
	if !ok || registries == nil {
		return
	}
	var updatedRegistries []map[string]interface{}
	for _, registry := range registries {
		if registry["ecrCredentialPlugin"] != nil {
			awsAccessKeyID, _ := values.GetValue(registry, "ecrCredentialPlugin", "awsAccessKeyId")
			awsSecretAccessKey, _ := values.GetValue(registry, "ecrCredentialPlugin", "awsSecretAccessKey")
			awsAccessToken, _ := values.GetValue(registry, "ecrCredentialPlugin", "awsAccessToken")
			if awsAccessKeyID == nil && awsSecretAccessKey == nil && awsAccessToken == nil {
				delete(registry, "ecrCredentialPlugin")
			}
		}
		updatedRegistries = append(updatedRegistries, registry)
	}
	values.PutValue(data, updatedRegistries, "rancherKubernetesEngineConfig", "privateRegistries")
}

func validateKeyRotation(data map[string]interface{}) error {
	secretsEncryptionEnabled, _ := values.GetValue(data, "rancherKubernetesEngineConfig", "services", "kubeApi", "secretsEncryptionConfig", "enabled")
	rotateEncryptionKeyEnabled, _ := values.GetValue(data, "rancherKubernetesEngineConfig", "rotateEncryptionKey")
	if rotateEncryptionKeyEnabled != nil && rotateEncryptionKeyEnabled == true {
		if secretsEncryptionEnabled != nil && secretsEncryptionEnabled == false {
			return fmt.Errorf("unable to rotate encryption key when encryption configuration is disabled")
		}
	}
	return nil
}
