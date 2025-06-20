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
	ccluster "github.com/rancher/rancher/pkg/api/norman/customization/cluster"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/controllers/management/clusterstatus"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
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

type secrets struct {
	regSecret                        *corev1.Secret
	s3Secret                         *corev1.Secret
	weaveSecret                      *corev1.Secret
	vsphereSecret                    *corev1.Secret
	vcenterSecret                    *corev1.Secret
	openStackSecret                  *corev1.Secret
	aadClientSecret                  *corev1.Secret
	aadCertSecret                    *corev1.Secret
	aciAPICUserKeySecret             *corev1.Secret
	aciTokenSecret                   *corev1.Secret
	aciKafkaClientKeySecret          *corev1.Secret
	secretsEncryptionProvidersSecret *corev1.Secret
	bastionHostSSHKeySecret          *corev1.Secret
	kubeletExtraEnvSecret            *corev1.Secret
	privateRegistryECRSecret         *corev1.Secret
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

	data, err = r.Store.Create(apiContext, schema, data)
	if err != nil {
		return nil, err
	}

	return data, nil
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

	data, err = r.transposeDynamicFieldToGenericConfig(data)
	if err != nil {
		return nil, err
	}

	if err := validateNetworkFlag(data, false); err != nil {
		return nil, httperror.NewFieldAPIError(httperror.InvalidOption, "enableNetworkPolicy", err.Error())
	}

	data, err = r.Store.Update(apiContext, schema, data, id)
	if err != nil {
		return nil, err
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
