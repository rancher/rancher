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
	"github.com/rancher/norman/parse/builder"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/norman/types/values"
	ccluster "github.com/rancher/rancher/pkg/api/norman/customization/cluster"
	"github.com/rancher/rancher/pkg/api/norman/customization/clustertemplate"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/controllers/management/clusterstatus"
	"github.com/rancher/rancher/pkg/controllers/management/etcdbackup"
	"github.com/rancher/rancher/pkg/controllers/management/rkeworkerupgrader"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator/assemblers"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	nodehelper "github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/ref"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	rkedefaults "github.com/rancher/rke/cluster"
	"github.com/rancher/rke/k8s"
	rkeservices "github.com/rancher/rke/services"
	rketypes "github.com/rancher/rke/types"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sTypes "k8s.io/apimachinery/pkg/types"
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

	// check if template is passed. if yes, load template data
	if hasTemplate(data) {
		clusterTemplateRevision, clusterTemplate, err := r.validateTemplateInput(apiContext, data, false)
		if err != nil {
			return nil, err
		}
		clusterConfigSchema := apiContext.Schemas.Schema(&managementschema.Version, managementv3.ClusterSpecBaseType)
		data, err = loadDataFromTemplate(clusterTemplateRevision, clusterTemplate, data, clusterConfigSchema, nil, r.SecretLister)
		if err != nil {
			return nil, err
		}
		data = cleanQuestions(data)
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
	if err = validateS3Credentials(data, nil); err != nil {
		return nil, err
	}
	if err = validateKeyRotation(data); err != nil {
		return nil, err
	}
	cleanPrivateRegistry(data)

	allSecrets, err := r.migrateSecrets(apiContext.Request.Context(), data, nil, "", "", "", "", "", "", "", "", "", "", "", "", "", "", "")

	if err != nil {
		return nil, err
	}

	data, err = r.Store.Create(apiContext, schema, data)
	if err != nil {
		cleanup := func(secret *corev1.Secret) {
			if secret != nil {
				if cleanupErr := r.secretMigrator.Cleanup(secret.Name); cleanupErr != nil {
					logrus.Errorf("cluster store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
		}

		cleanup(allSecrets.regSecret)
		cleanup(allSecrets.s3Secret)
		cleanup(allSecrets.weaveSecret)
		cleanup(allSecrets.vsphereSecret)
		cleanup(allSecrets.vcenterSecret)
		cleanup(allSecrets.openStackSecret)
		cleanup(allSecrets.aadClientSecret)
		cleanup(allSecrets.aadCertSecret)
		cleanup(allSecrets.aciAPICUserKeySecret)
		cleanup(allSecrets.aciTokenSecret)
		cleanup(allSecrets.aciKafkaClientKeySecret)
		cleanup(allSecrets.secretsEncryptionProvidersSecret)
		cleanup(allSecrets.bastionHostSSHKeySecret)
		cleanup(allSecrets.kubeletExtraEnvSecret)
		cleanup(allSecrets.privateRegistryECRSecret)

		return nil, err
	}
	owner := metav1.OwnerReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       "Cluster",
		Name:       data["id"].(string),
		UID:        k8sTypes.UID(data["uuid"].(string)),
	}
	errMsg := fmt.Sprintf("cluster store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
	updateOwner := func(secret *corev1.Secret) {
		if secret != nil {
			err = r.secretMigrator.UpdateSecretOwnerReference(secret, owner)
			if err != nil {
				logrus.Errorf(errMsg)
			}
		}
	}

	updateOwner(allSecrets.regSecret)
	updateOwner(allSecrets.s3Secret)
	updateOwner(allSecrets.weaveSecret)
	updateOwner(allSecrets.vsphereSecret)
	updateOwner(allSecrets.vcenterSecret)
	updateOwner(allSecrets.openStackSecret)
	updateOwner(allSecrets.aadClientSecret)
	updateOwner(allSecrets.aadCertSecret)
	updateOwner(allSecrets.aciAPICUserKeySecret)
	updateOwner(allSecrets.aciTokenSecret)
	updateOwner(allSecrets.aciKafkaClientKeySecret)
	updateOwner(allSecrets.secretsEncryptionProvidersSecret)
	updateOwner(allSecrets.bastionHostSSHKeySecret)
	updateOwner(allSecrets.kubeletExtraEnvSecret)
	updateOwner(allSecrets.privateRegistryECRSecret)

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

func loadDataFromTemplate(clusterTemplateRevision *apimgmtv3.ClusterTemplateRevision, clusterTemplate *apimgmtv3.ClusterTemplate, data map[string]interface{}, clusterConfigSchema *types.Schema, existingCluster map[string]interface{}, secretLister v1.SecretLister) (map[string]interface{}, error) {
	clusterConfig := *clusterTemplateRevision.Spec.ClusterConfig
	clusterConfigSpec, err := assemblers.AssembleRKEConfigTemplateSpec(clusterTemplateRevision, apimgmtv3.ClusterSpec{ClusterSpecBase: clusterConfig}, secretLister)
	if err != nil {
		return nil, err
	}
	dataFromTemplate, err := convert.EncodeToMap(clusterConfigSpec)
	if err != nil {
		return nil, err
	}
	dataFromTemplate["name"] = convert.ToString(data["name"])
	dataFromTemplate["description"] = convert.ToString(data[managementv3.ClusterSpecFieldDisplayName])
	dataFromTemplate[managementv3.ClusterSpecFieldClusterTemplateID] = ref.Ref(clusterTemplate)
	dataFromTemplate[managementv3.ClusterSpecFieldClusterTemplateRevisionID] = convert.ToString(data[managementv3.ClusterSpecFieldClusterTemplateRevisionID])

	dataFromTemplate = transposeNameFields(dataFromTemplate, clusterConfigSchema)
	var revisionQuestions []map[string]interface{}
	// Add in any answers to the clusterTemplateRevision's Questions[]
	allAnswers := convert.ToMapInterface(convert.ToMapInterface(data[managementv3.ClusterSpecFieldClusterTemplateAnswers])["values"])
	existingAnswers := convert.ToMapInterface(convert.ToMapInterface(existingCluster[managementv3.ClusterSpecFieldClusterTemplateAnswers])["values"])

	defaultedAnswers := make(map[string]string)

	// The key in the map is used to preserve the order of registries
	registryMap := make(map[int]map[string]interface{})
	existingRegistries := convert.ToMapSlice(convert.ToMapInterface(existingCluster[managementv3.ClusterSpecFieldRancherKubernetesEngineConfig])[managementv3.RancherKubernetesEngineConfigFieldPrivateRegistries])
	privateRegistryOverride := false
	for i, registry := range existingRegistries {
		registryMap[i] = registry
	}
	processingError := "Error processing clusterTemplate answers"
	for _, question := range clusterTemplateRevision.Spec.Questions {
		if question.Default == "" {
			if secretmigrator.MatchesQuestionPath(question.Variable) {
				if strings.HasPrefix(question.Variable, "rancherKubernetesEngineConfig.privateRegistries") {

					registries, ok := values.GetSlice(dataFromTemplate, "rancherKubernetesEngineConfig", "privateRegistries")
					if !ok {
						return nil, httperror.WrapAPIError(err, httperror.ServerError, processingError)
					}
					index, err := getIndexFromQuestion(question.Variable)
					if err != nil {
						return nil, httperror.WrapAPIError(err, httperror.ServerError, processingError)
					}
					question.Default = registries[index]["password"].(string)
				} else if strings.HasPrefix(question.Variable, "rancherKubernetesEngineConfig.cloudProvider.vsphereCloudProvider.virtualCenter") {
					vcenters, ok := values.GetValue(dataFromTemplate, "rancherKubernetesEngineConfig", "cloudProvider", "vsphereCloudProvider", "virtualCenter")
					if !ok {
						return nil, httperror.WrapAPIError(err, httperror.ServerError, processingError)
					}
					key, err := getKeyFromQuestion(question.Variable)
					if err != nil {
						return nil, httperror.WrapAPIError(err, httperror.ServerError, processingError)
					}
					question.Default = vcenters.(map[string]interface{})[key].(map[string]interface{})["password"].(string)

				} else {
					keyParts := strings.Split(question.Variable, ".")
					qDefault, ok := values.GetValue(dataFromTemplate, keyParts...)
					if ok {
						question.Default = qDefault.(string)
					}
				}
			}
		}
		answer, ok := allAnswers[question.Variable]
		if !ok {
			if question.Required && question.Default == "" {
				return nil, httperror.WrapAPIError(err, httperror.MissingRequired, fmt.Sprintf("Missing answer for a required clusterTemplate question: %v", question.Variable))
			}
			answer = question.Default
			defaultedAnswers[question.Variable] = question.Default
		}
		if existingCluster != nil && strings.EqualFold(question.Variable, "rancherKubernetesEngineConfig.kubernetesVersion") {
			if convert.ToString(answer) == convert.ToString(existingAnswers[question.Variable]) {
				answer = values.GetValueN(existingCluster, "rancherKubernetesEngineConfig", "kubernetesVersion")
			}
		}
		val, err := builder.ConvertSimple(question.Type, answer, builder.Create)
		if err != nil {
			return nil, httperror.WrapAPIError(err, httperror.ServerError, processingError)
		}
		keyParts := strings.Split(question.Variable, ".")
		if strings.HasPrefix(question.Variable, "rancherKubernetesEngineConfig.privateRegistries") {
			privateRegistryOverride = true
			// for example: question.Variable = rancherKubernetesEngineConfig.privateRegistries[0].url
			index, err := getIndexFromQuestion(question.Variable)
			if err != nil {
				return nil, httperror.WrapAPIError(err, httperror.ServerError, "Error processing clusterTemplate answers to private registry")
			}
			key := keyParts[len(keyParts)-1]
			if _, ok := registryMap[index]; ok {
				registryMap[index][key] = val
			} else {
				registryMap[index] = map[string]interface{}{
					key: val,
				}
			}
		} else {
			values.PutValue(dataFromTemplate, val, keyParts...)
		}

		questionMap, err := convert.EncodeToMap(question)
		if err != nil {
			return nil, httperror.WrapAPIError(err, httperror.ServerError, "Error reading clusterTemplate questions")
		}
		revisionQuestions = append(revisionQuestions, questionMap)
	}
	if len(registryMap) > 0 && privateRegistryOverride {
		registries, err := convertRegistryMapToSliceInOrder(registryMap)
		if err != nil {
			return nil, httperror.WrapAPIError(err, httperror.ServerError, "Error processing clusterTemplate answers to private registry")
		}
		// save privateRegistries back to rancherKubernetesEngineConfig
		values.PutValue(dataFromTemplate, registries, managementv3.ClusterSpecFieldRancherKubernetesEngineConfig, managementv3.RancherKubernetesEngineConfigFieldPrivateRegistries)
	}
	// save defaultAnswers to answer
	if allAnswers == nil {
		allAnswers = make(map[string]interface{})
	}
	for key, val := range defaultedAnswers {
		allAnswers[key] = val
	}

	finalAnswerMap := make(map[string]interface{})
	finalAnswerMap["values"] = allAnswers
	dataFromTemplate[managementv3.ClusterSpecFieldClusterTemplateAnswers] = finalAnswerMap
	dataFromTemplate[managementv3.ClusterSpecFieldClusterTemplateQuestions] = revisionQuestions

	dataFromTemplate[managementv3.ClusterSpecFieldDescription] = convert.ToString(data[managementv3.ClusterSpecFieldDescription])

	annotations, ok := data[managementv3.MetadataUpdateFieldAnnotations]
	if ok {
		dataFromTemplate[managementv3.MetadataUpdateFieldAnnotations] = convert.ToMapInterface(annotations)
	}

	labels, ok := data[managementv3.MetadataUpdateFieldLabels]
	if ok {
		dataFromTemplate[managementv3.MetadataUpdateFieldLabels] = convert.ToMapInterface(labels)
	}

	// make sure fleetworkspace is copied over
	fleetworkspace, ok := data[managementv3.ClusterFieldFleetWorkspaceName]
	if ok {
		dataFromTemplate[managementv3.ClusterFieldFleetWorkspaceName] = fleetworkspace
	}

	//validate that the data loaded is valid clusterSpec
	var spec apimgmtv3.ClusterSpec
	if err := convert.ToObj(dataFromTemplate, &spec); err != nil {
		return nil, httperror.WrapAPIError(err, httperror.InvalidBodyContent, "Invalid clusterTemplate, cannot convert to cluster spec")
	}

	return dataFromTemplate, nil
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

	// check if template is passed. if yes, load template data
	if hasTemplate(data) {
		if existingCluster[managementv3.ClusterSpecFieldClusterTemplateRevisionID] == "" {
			return nil, httperror.NewAPIError(httperror.InvalidOption, fmt.Sprintf("this cluster is not created using a clusterTemplate, cannot update it to use a clusterTemplate now"))
		}

		clusterTemplateRevision, clusterTemplate, err := r.validateTemplateInput(apiContext, data, true)
		if err != nil {
			return nil, err
		}

		updatedTemplateID := clusterTemplateRevision.Spec.ClusterTemplateName
		templateID := convert.ToString(existingCluster[managementv3.ClusterSpecFieldClusterTemplateID])

		if !strings.EqualFold(updatedTemplateID, templateID) {
			return nil, httperror.NewAPIError(httperror.InvalidOption, fmt.Sprintf("cannot update cluster, cluster cannot be changed to a new clusterTemplate"))
		}

		clusterConfigSchema := apiContext.Schemas.Schema(&managementschema.Version, managementv3.ClusterSpecBaseType)
		clusterUpdate, err := loadDataFromTemplate(clusterTemplateRevision, clusterTemplate, data, clusterConfigSchema, existingCluster, r.SecretLister)
		if err != nil {
			return nil, err
		}
		clusterUpdate = cleanQuestions(clusterUpdate)

		data = clusterUpdate

		// keep monitoring and alerting flags on the cluster as is, no turning off these flags from templaterevision.
		if !clusterTemplateRevision.Spec.ClusterConfig.EnableClusterAlerting {
			data[managementv3.ClusterSpecFieldEnableClusterAlerting] = existingCluster[managementv3.ClusterSpecFieldEnableClusterAlerting]
		}

	} else if existingCluster[managementv3.ClusterSpecFieldClusterTemplateRevisionID] != nil {
		return nil, httperror.NewFieldAPIError(httperror.MissingRequired, "ClusterTemplateRevision", "this cluster is created from a clusterTemplateRevision, please pass the clusterTemplateRevision")
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
	dialer, err := r.DialerFactory.ClusterDialer(id)
	if err != nil {
		return nil, errors.Wrap(err, "error getting dialer")
	}
	if err := validateUpdatedS3Credentials(existingCluster, data, dialer); err != nil {
		return nil, err
	}
	if err := validateKeyRotation(data); err != nil {
		return nil, err
	}
	if err := r.validateUnavailableNodes(data, existingCluster, id); err != nil {
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

	getSecretByKey := func(key string) string {
		if v, ok := values.GetValue(existingCluster, clusterSecrets, key); ok && v.(string) != "" {
			return v.(string)
		}
		if v, ok := values.GetValue(existingCluster, key); ok {
			return v.(string)
		}
		return ""
	}
	currentRegSecret := getSecretByKey(registrySecretKey)
	currentS3Secret := getSecretByKey(s3SecretKey)
	currentWeaveSecret := getSecretByKey(weaveSecretKey)
	currentVsphereSecret := getSecretByKey(vsphereSecretKey)
	currentVcenterSecret := getSecretByKey(virtualCenterSecretKey)
	currentOpenStackSecret := getSecretByKey(openStackSecretKey)
	currentAADClientSecret := getSecretByKey(aadClientSecretKey)
	currentAADCertSecret := getSecretByKey(aadClientCertSecretKey)
	currentACIAPICUserKeySecret := getSecretByKey(aciAPICUserKeySecretKey)
	currentACITokenSecret := getSecretByKey(aciTokenSecretKey)
	currentACIKafkaClientKeySecret := getSecretByKey(aciKafkaClientKeySecretKey)
	currentSecretsEncryptionProvidersSecret := getSecretByKey(secretsEncryptionProvidersSecretKey)
	currentBastionHostSSHSecret := getSecretByKey(bastionHostSSHKeySecretKey)
	currentKubeletExtraEnvSecret := getSecretByKey(kubeletExtraEnvSecretKey)
	currentPrivateRegistryECRSecret := getSecretByKey(privateRegistryECRSecretKey)
	allSecrets, err := r.migrateSecrets(apiContext.Request.Context(), data, existingCluster,
		currentRegSecret,
		currentS3Secret,
		currentWeaveSecret,
		currentVsphereSecret,
		currentVcenterSecret,
		currentOpenStackSecret,
		currentAADClientSecret,
		currentAADCertSecret,
		currentACIAPICUserKeySecret,
		currentACITokenSecret,
		currentACIKafkaClientKeySecret,
		currentSecretsEncryptionProvidersSecret,
		currentBastionHostSSHSecret,
		currentKubeletExtraEnvSecret,
		currentPrivateRegistryECRSecret)

	if err != nil {
		return nil, err
	}
	data, err = r.Store.Update(apiContext, schema, data, id)
	if err != nil {
		cleanup := func(secret *corev1.Secret, current string) {
			if secret != nil && current == "" {
				if cleanupErr := r.secretMigrator.Cleanup(secret.Name); cleanupErr != nil {
					logrus.Errorf("cluster store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
				}
			}
		}

		cleanup(allSecrets.regSecret, currentRegSecret)
		cleanup(allSecrets.s3Secret, currentS3Secret)
		cleanup(allSecrets.weaveSecret, currentWeaveSecret)
		cleanup(allSecrets.vsphereSecret, currentVsphereSecret)
		cleanup(allSecrets.vcenterSecret, currentVcenterSecret)
		cleanup(allSecrets.openStackSecret, currentOpenStackSecret)
		cleanup(allSecrets.aadClientSecret, currentAADClientSecret)
		cleanup(allSecrets.aadCertSecret, currentAADCertSecret)
		cleanup(allSecrets.aciAPICUserKeySecret, currentACIAPICUserKeySecret)
		cleanup(allSecrets.aciTokenSecret, currentACITokenSecret)
		cleanup(allSecrets.aciKafkaClientKeySecret, currentACIKafkaClientKeySecret)
		cleanup(allSecrets.secretsEncryptionProvidersSecret, currentSecretsEncryptionProvidersSecret)
		cleanup(allSecrets.bastionHostSSHKeySecret, currentBastionHostSSHSecret)
		cleanup(allSecrets.kubeletExtraEnvSecret, currentKubeletExtraEnvSecret)
		cleanup(allSecrets.privateRegistryECRSecret, currentPrivateRegistryECRSecret)

		return nil, err
	}
	if allSecrets.regSecret != nil || allSecrets.s3Secret != nil || allSecrets.weaveSecret != nil || allSecrets.vsphereSecret != nil || allSecrets.vcenterSecret != nil || allSecrets.openStackSecret != nil || allSecrets.aadClientSecret != nil || allSecrets.aadCertSecret != nil || allSecrets.secretsEncryptionProvidersSecret != nil {
		if r.ClusterClient == nil {
			return nil, fmt.Errorf("Error updating the cluster: k8s client is nil")
		}
		cluster, err := r.ClusterClient.Get(apiContext.Request.Context(), existingCluster["id"].(string), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		removeFromStatus := func(secret *corev1.Secret, key string) {
			if secret != nil {
				if _, ok := existingCluster[key]; ok {
					values.RemoveValue(cluster.Object, "status", key)
				}
			}
		}
		removeFromStatus(allSecrets.regSecret, registrySecretKey)
		removeFromStatus(allSecrets.s3Secret, s3SecretKey)
		removeFromStatus(allSecrets.weaveSecret, weaveSecretKey)
		removeFromStatus(allSecrets.vsphereSecret, vsphereSecretKey)
		removeFromStatus(allSecrets.vcenterSecret, virtualCenterSecretKey)
		removeFromStatus(allSecrets.openStackSecret, openStackSecretKey)
		removeFromStatus(allSecrets.aadClientSecret, aadClientSecretKey)
		removeFromStatus(allSecrets.aadCertSecret, aadClientCertSecretKey)

		_, err = r.ClusterClient.Update(apiContext.Request.Context(), cluster, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
	}
	owner := metav1.OwnerReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       "Cluster",
		Name:       data["id"].(string),
		UID:        k8sTypes.UID(data["uuid"].(string)),
	}
	errMsg := fmt.Sprintf("cluster store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
	updateOwner := func(secret *corev1.Secret) {
		if secret != nil {
			err = r.secretMigrator.UpdateSecretOwnerReference(secret, owner)
			if err != nil {
				logrus.Errorf(errMsg)
			}
		}
	}

	updateOwner(allSecrets.regSecret)
	updateOwner(allSecrets.s3Secret)
	updateOwner(allSecrets.weaveSecret)
	updateOwner(allSecrets.vsphereSecret)
	updateOwner(allSecrets.vcenterSecret)
	updateOwner(allSecrets.openStackSecret)
	updateOwner(allSecrets.aadClientSecret)
	updateOwner(allSecrets.aadCertSecret)
	updateOwner(allSecrets.aciAPICUserKeySecret)
	updateOwner(allSecrets.aciTokenSecret)
	updateOwner(allSecrets.aciKafkaClientKeySecret)
	updateOwner(allSecrets.secretsEncryptionProvidersSecret)
	updateOwner(allSecrets.bastionHostSSHKeySecret)
	updateOwner(allSecrets.kubeletExtraEnvSecret)
	updateOwner(allSecrets.privateRegistryECRSecret)
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

func (r *Store) migrateSecrets(ctx context.Context, data, existingCluster map[string]interface{}, currentReg, currentS3, currentWeave, currentVsphere, currentVCenter, currentOpenStack, currentAADClientSecret, currentAADCert, currentACIAPICUserKey, currentACIToken, currentACIKafkaClientKey, currentSecretsEncryptionProviders, currentbastionHostSSHKeySecret, currentKubeletExtraEnvSecret, currentPrivateRegistryECRSecret string) (secrets, error) {
	rkeConfig, err := getRkeConfig(data)
	if err != nil || rkeConfig == nil {
		return secrets{}, err
	}
	var s secrets
	s.regSecret, err = r.secretMigrator.CreateOrUpdatePrivateRegistrySecret(currentReg, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.regSecret != nil {
		values.PutValue(data, s.regSecret.Name, clusterSecrets, registrySecretKey)
		rkeConfig.PrivateRegistries = secretmigrator.CleanRegistries(rkeConfig.PrivateRegistries)
	}
	s.s3Secret, err = r.secretMigrator.CreateOrUpdateS3Secret(currentS3, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.s3Secret != nil {
		values.PutValue(data, s.s3Secret.Name, clusterSecrets, s3SecretKey)
		rkeConfig.Services.Etcd.BackupConfig.S3BackupConfig.SecretKey = ""
	}
	s.weaveSecret, err = r.secretMigrator.CreateOrUpdateWeaveSecret(currentWeave, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.weaveSecret != nil {
		values.PutValue(data, s.weaveSecret.Name, clusterSecrets, weaveSecretKey)
		rkeConfig.Network.WeaveNetworkProvider.Password = ""
	}
	s.vsphereSecret, err = r.secretMigrator.CreateOrUpdateVsphereGlobalSecret(currentVsphere, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.vsphereSecret != nil {
		values.PutValue(data, s.vsphereSecret.Name, clusterSecrets, vsphereSecretKey)
		rkeConfig.CloudProvider.VsphereCloudProvider.Global.Password = ""
	}
	s.vcenterSecret, err = r.secretMigrator.CreateOrUpdateVsphereVirtualCenterSecret(currentVCenter, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.vcenterSecret != nil {
		values.PutValue(data, s.vcenterSecret.Name, clusterSecrets, virtualCenterSecretKey)
		for k, v := range rkeConfig.CloudProvider.VsphereCloudProvider.VirtualCenter {
			v.Password = ""
			rkeConfig.CloudProvider.VsphereCloudProvider.VirtualCenter[k] = v
		}
	}
	s.openStackSecret, err = r.secretMigrator.CreateOrUpdateOpenStackSecret(currentOpenStack, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.openStackSecret != nil {
		values.PutValue(data, s.openStackSecret.Name, clusterSecrets, openStackSecretKey)
		rkeConfig.CloudProvider.OpenstackCloudProvider.Global.Password = ""
	}
	s.aadClientSecret, err = r.secretMigrator.CreateOrUpdateAADClientSecret(currentAADClientSecret, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.aadClientSecret != nil {
		values.PutValue(data, s.aadClientSecret.Name, clusterSecrets, aadClientSecretKey)
		rkeConfig.CloudProvider.AzureCloudProvider.AADClientSecret = ""
	}
	s.aadCertSecret, err = r.secretMigrator.CreateOrUpdateAADCertSecret(currentAADCert, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.aadCertSecret != nil {
		values.PutValue(data, s.aadCertSecret.Name, clusterSecrets, aadClientCertSecretKey)
		rkeConfig.CloudProvider.AzureCloudProvider.AADClientCertPassword = ""
	}
	s.aciAPICUserKeySecret, err = r.secretMigrator.CreateOrUpdateACIAPICUserKeySecret(currentACIAPICUserKey, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.aciAPICUserKeySecret != nil {
		values.PutValue(data, s.aciAPICUserKeySecret.Name, clusterSecrets, aciAPICUserKeySecretKey)
		rkeConfig.Network.AciNetworkProvider.ApicUserKey = ""
	}
	s.aciTokenSecret, err = r.secretMigrator.CreateOrUpdateACITokenSecret(currentACIToken, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.aciTokenSecret != nil {
		values.PutValue(data, s.aciTokenSecret.Name, clusterSecrets, aciTokenSecretKey)
		rkeConfig.Network.AciNetworkProvider.Token = ""
	}
	s.aciKafkaClientKeySecret, err = r.secretMigrator.CreateOrUpdateACIKafkaClientKeySecret(currentACIKafkaClientKey, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.aciKafkaClientKeySecret != nil {
		values.PutValue(data, s.aciKafkaClientKeySecret.Name, clusterSecrets, aciKafkaClientKeySecretKey)
		rkeConfig.Network.AciNetworkProvider.KafkaClientKey = ""
	}
	s.secretsEncryptionProvidersSecret, err = r.secretMigrator.CreateOrUpdateSecretsEncryptionProvidersSecret(currentSecretsEncryptionProviders, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.secretsEncryptionProvidersSecret != nil {
		values.PutValue(data, s.secretsEncryptionProvidersSecret.Name, clusterSecrets, secretsEncryptionProvidersSecretKey)
		rkeConfig.Services.KubeAPI.SecretsEncryptionConfig.CustomConfig.Resources = nil
	}
	s.bastionHostSSHKeySecret, err = r.secretMigrator.CreateOrUpdateBastionHostSSHKeySecret(currentbastionHostSSHKeySecret, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.bastionHostSSHKeySecret != nil {
		values.PutValue(data, s.bastionHostSSHKeySecret.Name, clusterSecrets, bastionHostSSHKeySecretKey)
		rkeConfig.BastionHost.SSHKey = ""
	}
	s.kubeletExtraEnvSecret, err = r.secretMigrator.CreateOrUpdateKubeletExtraEnvSecret(currentKubeletExtraEnvSecret, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.kubeletExtraEnvSecret != nil {
		values.PutValue(data, s.kubeletExtraEnvSecret.Name, clusterSecrets, kubeletExtraEnvSecretKey)
		env := make([]string, 0, len(rkeConfig.Services.Kubelet.ExtraEnv))
		for _, e := range rkeConfig.Services.Kubelet.ExtraEnv {
			if !strings.Contains(e, "AWS_SECRET_ACCESS_KEY") {
				env = append(env, e)
			}
		}
		rkeConfig.Services.Kubelet.ExtraEnv = env
	}
	s.privateRegistryECRSecret, err = r.secretMigrator.CreateOrUpdatePrivateRegistryECRSecret(currentPrivateRegistryECRSecret, rkeConfig, nil)
	if err != nil {
		return secrets{}, err
	}
	if s.privateRegistryECRSecret != nil {
		values.PutValue(data, s.privateRegistryECRSecret.Name, clusterSecrets, privateRegistryECRSecretKey)
		for _, reg := range rkeConfig.PrivateRegistries {
			if ecr := reg.ECRCredentialPlugin; ecr != nil {
				ecr.AwsSecretAccessKey = ""
				ecr.AwsSessionToken = ""
			}
		}
	}

	data["rancherKubernetesEngineConfig"], err = convert.EncodeToMap(rkeConfig)
	if err != nil {
		return secrets{}, err
	}
	return s, nil
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

func validateUpdatedS3Credentials(oldData, newData map[string]interface{}, dialer dialer.Dialer) error {
	newConfig := convert.ToMapInterface(values.GetValueN(newData, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig", "s3BackupConfig"))
	if newConfig == nil {
		return nil
	}

	oldConfig := convert.ToMapInterface(values.GetValueN(oldData, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig", "s3BackupConfig"))
	if oldConfig == nil {
		return validateS3Credentials(newData, dialer)
	}
	// remove "type" since it's added to the object by API, and it's not present in newConfig yet.
	delete(oldConfig, "type")
	if !reflect.DeepEqual(newConfig, oldConfig) {
		return validateS3Credentials(newData, dialer)
	}
	return nil
}

func validateS3Credentials(data map[string]interface{}, dialer dialer.Dialer) error {
	s3BackupConfig := values.GetValueN(data, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig", "s3BackupConfig")
	if s3BackupConfig == nil {
		return nil
	}
	configMap := convert.ToMapInterface(s3BackupConfig)
	sbc := &rketypes.S3BackupConfig{
		AccessKey: convert.ToString(configMap["accessKey"]),
		SecretKey: convert.ToString(configMap["secretKey"]),
		Endpoint:  convert.ToString(configMap["endpoint"]),
		Region:    convert.ToString(configMap["region"]),
		CustomCA:  convert.ToString(configMap["customCa"]),
	}
	// skip if we don't have credentials defined.
	if sbc.AccessKey == "" && sbc.SecretKey == "" {
		return nil
	}
	bucket := convert.ToString(configMap["bucketName"])
	if bucket == "" {
		return fmt.Errorf("Empty bucket name")
	}
	s3Client, err := etcdbackup.GetS3Client(sbc, s3TransportTimeout, dialer)
	if err != nil {
		return err
	}
	exists, err := s3Client.BucketExists(context.TODO(), bucket)
	if err != nil {
		return fmt.Errorf("Unable to validate S3 backup target configuration: %v", err)
	}
	if !exists {
		return fmt.Errorf("Unable to validate S3 backup target configuration: bucket [%v] not found", bucket)
	}
	return nil
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

func (r *Store) validateUnavailableNodes(data, existingData map[string]interface{}, id string) error {
	cluster, err := r.ClusterLister.Get("", id)
	if err != nil {
		return fmt.Errorf("error getting cluster, try again %v", err)
	}
	// no need to validate if cluster's already provisioning or upgrading
	if !apimgmtv3.ClusterConditionProvisioned.IsTrue(cluster) ||
		!apimgmtv3.ClusterConditionUpdated.IsTrue(cluster) ||
		apimgmtv3.ClusterConditionUpgraded.IsUnknown(cluster) {
		return nil
	}
	spec, err := getRkeConfig(data)
	if err != nil || spec == nil {
		return err
	}
	status, err := getRkeConfig(existingData)
	if err != nil || status == nil {
		return err
	}
	if reflect.DeepEqual(status, spec) {
		return nil
	}
	nodes, err := r.NodeLister.List(id, labels.Everything())
	if err != nil {
		return fmt.Errorf("error fetching nodes, try again %v", err)
	}
	return canUpgrade(nodes, spec.UpgradeStrategy)
}

func getRkeConfig(data map[string]interface{}) (*rketypes.RancherKubernetesEngineConfig, error) {
	rkeConfig := values.GetValueN(data, "rancherKubernetesEngineConfig")
	if rkeConfig == nil {
		return nil, nil
	}
	config, err := json.Marshal(rkeConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "error marshaling rkeConfig")
	}
	var spec *rketypes.RancherKubernetesEngineConfig
	if err = json.Unmarshal(config, &spec); err != nil {
		return nil, errors.Wrapf(err, "error reading rkeConfig")
	}
	return spec, nil
}

func canUpgrade(nodes []*apimgmtv3.Node, upgradeStrategy *rketypes.NodeUpgradeStrategy) error {
	var (
		controlReady, controlNotReady, workerOnlyReady, workerOnlyNotReady int
	)
	for _, node := range nodes {
		if node.Status.NodeConfig == nil {
			continue
		}
		if slice.ContainsString(node.Status.NodeConfig.Role, rkeservices.ControlRole) {
			// any node having control role
			if nodehelper.IsMachineReady(node) {
				controlReady++
			} else {
				controlNotReady++
			}
			continue
		}
		if slice.ContainsString(node.Status.NodeConfig.Role, rkeservices.ETCDRole) {
			continue
		}
		if nodehelper.IsMachineReady(node) {
			workerOnlyReady++
		} else {
			workerOnlyNotReady++
		}
	}
	maxUnavailableControl, err := rkeworkerupgrader.CalculateMaxUnavailable(upgradeStrategy.MaxUnavailableControlplane, controlReady+controlNotReady)
	if err != nil {
		return err
	}
	if controlNotReady >= maxUnavailableControl {
		return fmt.Errorf("not enough control plane nodes ready to upgrade, maxUnavailable: %v, notReady: %v, ready: %v",
			maxUnavailableControl, controlNotReady, controlReady)
	}
	maxUnavailableWorker, err := rkeworkerupgrader.CalculateMaxUnavailable(upgradeStrategy.MaxUnavailableWorker, workerOnlyReady+workerOnlyNotReady)
	if err != nil {
		return err
	}
	if workerOnlyNotReady >= maxUnavailableWorker {
		return fmt.Errorf("not enough worker nodes ready to upgrade, maxUnavailable: %v, notReady: %v, ready: %v",
			maxUnavailableWorker, workerOnlyNotReady, workerOnlyReady)
	}
	return nil
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

func cleanQuestions(data map[string]interface{}) map[string]interface{} {
	if _, ok := data["questions"]; ok {
		questions := data["questions"].([]map[string]interface{})
		for i, q := range questions {
			if secretmigrator.MatchesQuestionPath(q["variable"].(string)) {
				delete(q, "default")
			}
			questions[i] = q
		}
		values.PutValue(data, questions, "questions")
	}
	if _, ok := values.GetValue(data, "answers", "values"); ok {
		values.RemoveValue(data, "answers", "values", secretmigrator.S3BackupAnswersPath)
		values.RemoveValue(data, "answers", "values", secretmigrator.WeavePasswordAnswersPath)
		values.RemoveValue(data, "answers", "values", secretmigrator.VsphereGlobalAnswersPath)
		values.RemoveValue(data, "answers", "values", secretmigrator.OpenStackAnswersPath)
		values.RemoveValue(data, "answers", "values", secretmigrator.AADClientAnswersPath)
		values.RemoveValue(data, "answers", "values", secretmigrator.AADCertAnswersPath)
		for i := 0; ; i++ {
			key := fmt.Sprintf(secretmigrator.RegistryPasswordAnswersPath, i)
			if _, ok := values.GetValue(data, "answers", "values", key); !ok {
				break
			}
			values.RemoveValue(data, "answers", "values", key)
		}
		vcenters, ok := values.GetValue(data, "rancherKubernetesEngineConfig", "cloudProvider", "vsphereCloudProvider", "virtualCenter")
		if ok {
			for k := range vcenters.(map[string]interface{}) {
				key := fmt.Sprintf(secretmigrator.VcenterAnswersPath, k)
				values.RemoveValue(data, "answers", "values", key)
			}
		}
	}
	return data
}
