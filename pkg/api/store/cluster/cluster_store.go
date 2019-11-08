package cluster

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse/builder"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/norman/types/values"
	ccluster "github.com/rancher/rancher/pkg/api/customization/cluster"
	"github.com/rancher/rancher/pkg/api/customization/clustertemplate"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/controllers/management/clusterstatus"
	"github.com/rancher/rancher/pkg/controllers/management/etcdbackup"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	managementv3 "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	DefaultBackupIntervalHours = 12
	DefaultBackupRetention     = 6
	s3TransportTimeout         = 10
)

type Store struct {
	types.Store
	ShellHandler                  types.RequestHandler
	mu                            sync.Mutex
	KontainerDriverLister         v3.KontainerDriverLister
	ClusterTemplateLister         v3.ClusterTemplateLister
	ClusterTemplateRevisionLister v3.ClusterTemplateRevisionLister
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

	//check if template is passed. if yes, load template data
	if hasTemplate(data) {
		clusterTemplateRevision, clusterTemplate, err := r.validateTemplateInput(apiContext, data, false)
		if err != nil {
			return nil, err
		}
		clusterConfigSchema := apiContext.Schemas.Schema(&managementschema.Version, managementv3.ClusterSpecBaseType)
		data, err = loadDataFromTemplate(clusterTemplateRevision, clusterTemplate, data, clusterConfigSchema, nil)
		if err != nil {
			return nil, err
		}
	}

	err := setKubernetesVersion(data)
	if err != nil {
		return nil, err
	}
	// enable local backups for rke clusters by default
	enableLocalBackup(data)

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
	if err := validateS3Credentials(data); err != nil {
		return nil, err
	}

	return r.Store.Create(apiContext, schema, data)
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

func loadDataFromTemplate(clusterTemplateRevision *v3.ClusterTemplateRevision, clusterTemplate *v3.ClusterTemplate, data map[string]interface{}, clusterConfigSchema *types.Schema, existingCluster map[string]interface{}) (map[string]interface{}, error) {
	dataFromTemplate, err := convert.EncodeToMap(clusterTemplateRevision.Spec.ClusterConfig)
	if err != nil {
		return nil, err
	}
	dataFromTemplate["name"] = convert.ToString(data["name"])
	dataFromTemplate["description"] = convert.ToString(data[managementv3.ClusterSpecFieldDisplayName])
	dataFromTemplate[managementv3.ClusterSpecFieldClusterTemplateID] = ref.Ref(clusterTemplate)
	dataFromTemplate[managementv3.ClusterSpecFieldClusterTemplateRevisionID] = convert.ToString(data[managementv3.ClusterSpecFieldClusterTemplateRevisionID])

	dataFromTemplate = transposeNameFields(dataFromTemplate, clusterConfigSchema)
	var revisionQuestions []map[string]interface{}
	//Add in any answers to the clusterTemplateRevision's Questions[]
	allAnswers := convert.ToMapInterface(convert.ToMapInterface(data[managementv3.ClusterSpecFieldClusterTemplateAnswers])["values"])
	existingAnswers := convert.ToMapInterface(convert.ToMapInterface(existingCluster[managementv3.ClusterSpecFieldClusterTemplateAnswers])["values"])

	defaultedAnswers := make(map[string]string)

	for _, question := range clusterTemplateRevision.Spec.Questions {
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
			return nil, httperror.WrapAPIError(err, httperror.ServerError, "Error processing clusterTemplate answers")
		}
		keyParts := strings.Split(question.Variable, ".")
		values.PutValue(dataFromTemplate, val, keyParts...)

		questionMap, err := convert.EncodeToMap(question)
		if err != nil {
			return nil, httperror.WrapAPIError(err, httperror.ServerError, "Error reading clusterTemplate questions")
		}
		revisionQuestions = append(revisionQuestions, questionMap)
	}
	//save defaultAnswers to answer
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

	//validate that the data loaded is valid clusterSpec
	var spec v3.ClusterSpec
	if err := convert.ToObj(dataFromTemplate, &spec); err != nil {
		return nil, httperror.WrapAPIError(err, httperror.InvalidBodyContent, "Invalid clusterTemplate, cannot convert to cluster spec")
	}

	return dataFromTemplate, nil
}

func hasTemplate(data map[string]interface{}) bool {
	templateRevID := convert.ToString(data[managementv3.ClusterSpecFieldClusterTemplateRevisionID])
	if templateRevID != "" {
		return true
	}
	return false
}

func (r *Store) validateTemplateInput(apiContext *types.APIContext, data map[string]interface{}, isUpdate bool) (*v3.ClusterTemplateRevision, *v3.ClusterTemplate, error) {

	if !isUpdate {
		//if data also has rkeconfig, error out on create
		rkeConfig, ok := values.GetValue(data, "rancherKubernetesEngineConfig")
		if ok && rkeConfig != nil {
			return nil, nil, fmt.Errorf("cannot set rancherKubernetesEngineConfig and clusterTemplateRevision both")
		}
	}

	var templateID, templateRevID string

	templateRevIDStr := convert.ToString(data[managementv3.ClusterSpecFieldClusterTemplateRevisionID])
	var clusterTemplateRev managementv3.ClusterTemplateRevision

	//access check.
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

	//check if template is passed. if yes, load template data
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
		clusterUpdate, err := loadDataFromTemplate(clusterTemplateRevision, clusterTemplate, data, clusterConfigSchema, existingCluster)
		if err != nil {
			return nil, err
		}

		data = clusterUpdate
	} else if existingCluster[managementv3.ClusterSpecFieldClusterTemplateRevisionID] != nil {
		return nil, httperror.NewFieldAPIError(httperror.MissingRequired, "ClusterTemplateRevision", "this cluster is created from a clusterTemplateRevision, please pass the clusterTemplateRevision")
	}

	err = setKubernetesVersion(data)
	if err != nil {
		return nil, err
	}

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
	setWeavePasswordFieldsIfNotExists(existingCluster, data)
	if err := validateUpdatedS3Credentials(existingCluster, data); err != nil {
		return nil, err
	}

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

func setKubernetesVersion(data map[string]interface{}) error {
	rkeConfig, ok := values.GetValue(data, "rancherKubernetesEngineConfig")
	if ok && rkeConfig != nil {
		k8sVersion := values.GetValueN(data, "rancherKubernetesEngineConfig", "kubernetesVersion")
		if k8sVersion == nil || k8sVersion == "" {
			//set k8s version to system default on the spec
			defaultVersion := settings.KubernetesVersion.Get()
			values.PutValue(data, defaultVersion, "rancherKubernetesEngineConfig", "kubernetesVersion")
		} else {
			//if k8s version is already of rancher version form, noop
			//if k8s version is of form 1.14.x, figure out the latest
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

func setWeavePasswordFieldsIfNotExists(oldData, newData map[string]interface{}) {
	weaveConfig := values.GetValueN(newData, "rancherKubernetesEngineConfig", "network", "weaveNetworkProvider")
	if weaveConfig == nil {
		return
	}
	val := convert.ToMapInterface(weaveConfig)
	if val["password"] != nil {
		return
	}
	oldWeavePassword := convert.ToString(values.GetValueN(oldData, "rancherKubernetesEngineConfig", "network", "weaveNetworkProvider", "password"))
	if oldWeavePassword != "" {
		values.PutValue(newData, oldWeavePassword, "rancherKubernetesEngineConfig", "network", "weaveNetworkProvider", "password")
	}
}

func validateUpdatedS3Credentials(oldData, newData map[string]interface{}) error {
	newConfig := convert.ToMapInterface(values.GetValueN(newData, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig", "s3BackupConfig"))
	if newConfig == nil {
		return nil
	}

	oldConfig := convert.ToMapInterface(values.GetValueN(oldData, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig", "s3BackupConfig"))
	if oldConfig == nil {
		return validateS3Credentials(newData)
	}
	// remove "type" since it's added to the object by API, and it's not present in newConfig yet.
	delete(oldConfig, "type")
	if !reflect.DeepEqual(newConfig, oldConfig) {
		return validateS3Credentials(newData)
	}
	return nil
}

func validateS3Credentials(data map[string]interface{}) error {
	s3BackupConfig := values.GetValueN(data, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig", "s3BackupConfig")
	if s3BackupConfig == nil {
		return nil
	}
	configMap := convert.ToMapInterface(s3BackupConfig)
	sbc := &v3.S3BackupConfig{
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
	s3Client, err := etcdbackup.GetS3Client(sbc, s3TransportTimeout)
	if err != nil {
		return err
	}
	exists, err := s3Client.BucketExists(bucket)
	if err != nil {
		return fmt.Errorf("Unable to validate S3 backup target configration: %v", err)
	}
	if !exists {
		return fmt.Errorf("Unable to validate S3 backup target configration: bucket [%v] not found", bucket)
	}
	return nil
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
