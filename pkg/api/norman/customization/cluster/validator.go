package cluster

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	gaccess "github.com/rancher/rancher/pkg/api/norman/customization/globalnamespaceaccess"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtclient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/controllers/management/k3sbasedupgrade"
	"github.com/rancher/rancher/pkg/controllers/managementuser/cis"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/service"
	"github.com/rancher/rancher/pkg/namespace"
	mgmtSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/robfig/cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Validator struct {
	ClusterClient                 v3.ClusterInterface
	ClusterLister                 v3.ClusterLister
	ClusterTemplateLister         v3.ClusterTemplateLister
	ClusterTemplateRevisionLister v3.ClusterTemplateRevisionLister
	Users                         v3.UserInterface
	GrbLister                     v3.GlobalRoleBindingLister
	GrLister                      v3.GlobalRoleLister
	CisConfigClient               v3.CisConfigInterface
	CisConfigLister               v3.CisConfigLister
	CisBenchmarkVersionClient     v3.CisBenchmarkVersionInterface
	CisBenchmarkVersionLister     v3.CisBenchmarkVersionLister
}

func (v *Validator) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var clusterSpec v32.ClusterSpec
	var clientClusterSpec mgmtclient.Cluster
	if err := convert.ToObj(data, &clusterSpec); err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "Cluster spec conversion error")
	}

	if err := convert.ToObj(data, &clientClusterSpec); err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "Client cluster spec conversion error")
	}

	if err := v.validateEnforcement(request, data); err != nil {
		return err
	}

	if err := v.validateLocalClusterAuthEndpoint(request, &clusterSpec); err != nil {
		return err
	}

	if err := v.validateK3sBasedVersionUpgrade(request, &clusterSpec); err != nil {
		return err
	}

	if err := v.validateScheduledClusterScan(&clientClusterSpec); err != nil {
		return err
	}

	if err := v.validateGenericEngineConfig(request, &clusterSpec); err != nil {
		return err
	}

	if err := v.validateEKSConfig(request, data, &clusterSpec); err != nil {
		return err
	}

	return nil
}

func (v *Validator) validateScheduledClusterScan(spec *mgmtclient.Cluster) error {
	// If this cluster is created using a template, we dont have the version in the provided data, skip
	if spec.ClusterTemplateRevisionID != "" {
		return nil
	}

	// If CIS scan is not present/enabled, skip
	if spec.ScheduledClusterScan == nil ||
		(spec.ScheduledClusterScan != nil && !spec.ScheduledClusterScan.Enabled) {
		return nil
	}
	currentK8sVersion := spec.RancherKubernetesEngineConfig.Version
	overrideBenchmarkVersion := ""
	if spec.ScheduledClusterScan.ScanConfig.CisScanConfig != nil {
		overrideBenchmarkVersion = spec.ScheduledClusterScan.ScanConfig.CisScanConfig.OverrideBenchmarkVersion
	}
	_, _, err := cis.GetBenchmarkVersionToUse(overrideBenchmarkVersion, currentK8sVersion,
		v.CisConfigLister, v.CisConfigClient,
		v.CisBenchmarkVersionLister, v.CisBenchmarkVersionClient,
	)
	if err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}
	return validateScheduledClusterScan(spec)
}

func validateScheduledClusterScan(spec *mgmtclient.Cluster) error {
	// If this cluster is created using a template, we dont have the version in the provided data, skip
	if spec.ClusterTemplateRevisionID != "" {
		return nil
	}

	if spec.ScheduledClusterScan.ScanConfig != nil &&
		spec.ScheduledClusterScan.ScanConfig.CisScanConfig != nil {
		profile := spec.ScheduledClusterScan.ScanConfig.CisScanConfig.Profile
		if profile != string(v32.CisScanProfileTypePermissive) &&
			profile != string(v32.CisScanProfileTypeHardened) {
			return httperror.NewFieldAPIError(httperror.InvalidOption, "ScheduledClusterScan.ScanConfig.CisScanConfig.Profile", "profile can be either permissive or hardened")
		}
	}

	if spec.ScheduledClusterScan.ScheduleConfig != nil {
		if spec.ScheduledClusterScan.ScheduleConfig.Retention < 0 {
			return httperror.NewFieldAPIError(httperror.MinLimitExceeded, "ScheduledClusterScan.ScheduleConfig.Retention", "Retention count cannot be negative")
		}
		schedule, err := cron.ParseStandard(spec.ScheduledClusterScan.ScheduleConfig.CronSchedule)
		if err != nil {
			return httperror.NewFieldAPIError(httperror.InvalidFormat, "ScheduledClusterScan.ScheduleConfig.CronSchedule", fmt.Sprintf("error parsing cron schedule: %v", err))
		}
		now := time.Now().Round(time.Second)
		next1 := schedule.Next(now).Round(time.Second)
		next2 := schedule.Next(next1).Round(time.Second)
		timeAfter := next2.Sub(next1).Round(time.Second)

		if timeAfter < (1 * time.Hour) {
			if spec.ScheduledClusterScan.ScanConfig.CisScanConfig.DebugMaster ||
				spec.ScheduledClusterScan.ScanConfig.CisScanConfig.DebugWorker {
				return nil
			}
			return httperror.NewFieldAPIError(httperror.MinLimitExceeded, "ScheduledClusterScan.ScheduleConfig.CronSchedule", "minimum interval is one hour")
		}
	}
	return nil
}

func (v *Validator) validateLocalClusterAuthEndpoint(request *types.APIContext, spec *v32.ClusterSpec) error {
	if !spec.LocalClusterAuthEndpoint.Enabled {
		return nil
	}

	var isValidCluster bool
	if request.ID == "" {
		isValidCluster = spec.RancherKubernetesEngineConfig != nil
	} else {
		cluster, err := v.ClusterLister.Get("", request.ID)
		if err != nil {
			return err
		}
		isValidCluster = cluster.Status.Driver == "" ||
			cluster.Status.Driver == v32.ClusterDriverRKE ||
			cluster.Status.Driver == v32.ClusterDriverImported
	}
	if !isValidCluster {
		return httperror.NewFieldAPIError(httperror.InvalidState, "LocalClusterAuthEndpoint.Enabled", "Can only enable LocalClusterAuthEndpoint with RKE")
	}

	if spec.LocalClusterAuthEndpoint.CACerts != "" && spec.LocalClusterAuthEndpoint.FQDN == "" {
		return httperror.NewFieldAPIError(httperror.MissingRequired, "LocalClusterAuthEndpoint.FQDN", "CACerts defined but FQDN is not defined")
	}

	return nil
}

func (v *Validator) validateEnforcement(request *types.APIContext, data map[string]interface{}) error {

	if !strings.EqualFold(settings.ClusterTemplateEnforcement.Get(), "true") {
		return nil
	}

	var spec mgmtclient.Cluster
	if err := convert.ToObj(data, &spec); err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "Cluster spec conversion error")
	}

	if !v.checkClusterForEnforcement(&spec) {
		return nil
	}

	ma := gaccess.MemberAccess{
		Users:     v.Users,
		GrLister:  v.GrLister,
		GrbLister: v.GrbLister,
	}

	//if user is admin, no checks needed
	callerID := request.Request.Header.Get(gaccess.ImpersonateUserHeader)

	isAdmin, err := ma.IsAdmin(callerID)
	if err != nil {
		return err
	}
	if isAdmin {
		return nil
	}

	//enforcement is true, template is a must
	if spec.ClusterTemplateRevisionID == "" {
		return httperror.NewFieldAPIError(httperror.MissingRequired, "", "A clusterTemplateRevision to create a cluster")
	}

	err = v.accessTemplate(request, &spec)
	if err != nil {
		if httperror.IsForbidden(err) || httperror.IsNotFound(err) {
			return httperror.NewAPIError(httperror.NotFound, "The clusterTemplateRevision is not found")
		}
		return err
	}

	return nil
}

// TODO: test validator
// prevents downgrades, no-ops, and upgrading before versions have been set
func (v *Validator) validateK3sBasedVersionUpgrade(request *types.APIContext, spec *v32.ClusterSpec) error {
	upgradeNotReadyErr := httperror.NewAPIError(httperror.Conflict, "k3s version upgrade is not ready, try again later")

	if request.Method == http.MethodPost {
		return nil
	}
	isK3s := spec.K3sConfig != nil
	isrke2 := spec.Rke2Config != nil
	if !isK3s && !isrke2 {
		// only applies to k3s clusters
		return nil
	}

	// must wait for original spec version to be set
	if (isK3s && spec.K3sConfig.Version == "") || (isrke2 && spec.Rke2Config.Version == "") {
		return upgradeNotReadyErr
	}

	cluster, err := v.ClusterLister.Get("", request.ID)
	if err != nil {
		return err
	}

	// must wait for original status version to be set
	if cluster.Status.Version == nil {
		return upgradeNotReadyErr
	}

	var updateVersion string
	if cluster.Status.Driver == v32.ClusterDriverRke2 {
		updateVersion = spec.Rke2Config.Version
	} else {
		updateVersion = spec.K3sConfig.Version
	}

	prevVersion := cluster.Status.Version.GitVersion
	if prevVersion == updateVersion {
		// no op
		return nil
	}

	isNewer, err := k3sbasedupgrade.IsNewerVersion(prevVersion, updateVersion)
	if err != nil {
		errMsg := fmt.Sprintf("unable to compare cluster version [%s]", updateVersion)
		return httperror.NewAPIError(httperror.InvalidBodyContent, errMsg)
	}

	if !isNewer {
		// update version must be higher than previous version, downgrades are not supported
		errMsg := fmt.Sprintf("cannot upgrade cluster version from [%s] to [%s]. New version must be higher.", prevVersion, updateVersion)
		return httperror.NewAPIError(httperror.InvalidBodyContent, errMsg)
	}

	return nil
}

func (v *Validator) checkClusterForEnforcement(spec *mgmtclient.Cluster) bool {
	if spec.RancherKubernetesEngineConfig != nil {
		return true
	}

	if spec.ClusterTemplateRevisionID != "" {
		return true
	}
	return false
}

func (v *Validator) accessTemplate(request *types.APIContext, spec *mgmtclient.Cluster) error {
	split := strings.SplitN(spec.ClusterTemplateRevisionID, ":", 2)
	if len(split) != 2 {
		return fmt.Errorf("error in splitting clusterTemplateRevision name %v", spec.ClusterTemplateRevisionID)
	}
	revName := split[1]
	clusterTempRev, err := v.ClusterTemplateRevisionLister.Get(namespace.GlobalNamespace, revName)
	if err != nil {
		return err
	}

	var ctMap map[string]interface{}
	if err := access.ByID(request, &mgmtSchema.Version, mgmtclient.ClusterTemplateType, clusterTempRev.Spec.ClusterTemplateName, &ctMap); err != nil {
		return err
	}

	return nil
}

// validateGenericEngineConfig allows for additional validation of clusters that depend on Kontainer Engine or Rancher Machine driver
func (v *Validator) validateGenericEngineConfig(request *types.APIContext, spec *v32.ClusterSpec) error {

	if request.Method == http.MethodPost {
		return nil
	}

	if spec.AmazonElasticContainerServiceConfig != nil {
		// compare with current cluster
		clusterName := request.ID
		prevCluster, err := v.ClusterLister.Get("", clusterName)
		if err != nil {
			return err
		}

		err = validateEKS(*prevCluster.Spec.GenericEngineConfig, *spec.AmazonElasticContainerServiceConfig)
		if err != nil {
			return err
		}
	}

	return nil

}

func (v *Validator) validateEKSConfig(request *types.APIContext, cluster map[string]interface{}, clusterSpec *v32.ClusterSpec) error {
	eksConfig, ok := cluster["eksConfig"].(map[string]interface{})
	if !ok {
		return nil
	}

	var prevCluster *v3.Cluster

	if request.Method == http.MethodPut {
		var err error
		prevCluster, err = v.ClusterLister.Get("", request.ID)
		if err != nil {
			return err
		}
	}

	// check user's access to cloud credential
	if amazonCredential, ok := eksConfig["amazonCredentialSecret"].(string); ok {
		if err := validateEKSCredentialAuth(request, amazonCredential, prevCluster); err != nil {
			return err
		}
	}

	createFromImport := request.Method == http.MethodPost && eksConfig["imported"] == true

	if !createFromImport {
		if err := validateEKSKubernetesVersion(clusterSpec, prevCluster); err != nil {
			return err
		}
		if err := validateEKSNodegroups(clusterSpec); err != nil {
			return err
		}
		if err := validateEKSAccess(request, eksConfig, prevCluster); err != nil {
			return err
		}
	}

	if request.Method != http.MethodPost {
		return nil
	}

	// validation for creates only

	// validate cluster does not reference an EKS cluster that is already backed by a Rancher cluster
	name, _ := eksConfig["displayName"]
	region, _ := eksConfig["region"]

	// cluster client is being used instead of lister to avoid the use of an outdated cache
	clusters, err := v.ClusterClient.List(metav1.ListOptions{})
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("failed to confirm displayName is unique among Rancher EKS clusters for region %s", region))
	}

	for _, cluster := range clusters.Items {
		if cluster.Spec.EKSConfig == nil {
			continue
		}
		if name != cluster.Spec.EKSConfig.DisplayName {
			continue
		}
		if region != cluster.Spec.EKSConfig.Region {
			continue
		}
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("cluster already exists for EKS cluster [%s] in region [%s]", name, region))
	}

	if !createFromImport {
		// validate either all networking fields are provided or no networking fields are provided
		securityGroups, _ := eksConfig["securityGroups"].([]interface{})
		subnets, _ := eksConfig["subnets"].([]interface{})

		allNetworkingFieldsProvided := len(subnets) != 0 && len(securityGroups) != 0
		noNetworkingFieldsProvided := len(subnets) == 0 && len(securityGroups) == 0

		if !(allNetworkingFieldsProvided || noNetworkingFieldsProvided) {
			if !createFromImport {
				return httperror.NewAPIError(httperror.InvalidBodyContent,
					"must provide both networking fields (subnets, securityGroups) or neither")
			}
		}
	}

	return nil
}

func validateEKSAccess(request *types.APIContext, eksConfig map[string]interface{}, prevCluster *v3.Cluster) error {
	publicAccess, _ := eksConfig["publicAccess"]
	privateAccess, _ := eksConfig["privateAccess"]
	if request.Method != http.MethodPost {
		if publicAccess == nil {
			publicAccess = prevCluster.Spec.EKSConfig.PublicAccess
		}
		if privateAccess == nil {
			privateAccess = prevCluster.Spec.EKSConfig.PrivateAccess
		}
	}

	// can only perform comparisons on interfaces, cannot use as bool
	if publicAccess == false && privateAccess == false {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			"public access, private access, or both must be enabled")
	}
	return nil
}

// validateEKSKubernetesVersion checks whether a kubernetes version is provided and if it is supported
func validateEKSKubernetesVersion(spec *v32.ClusterSpec, prevCluster *v3.Cluster) error {
	clusterVersion := spec.EKSConfig.KubernetesVersion
	if clusterVersion == nil {
		return nil
	}

	if aws.StringValue(clusterVersion) == "" {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "cluster kubernetes version cannot be empty string")
	}

	return nil
}

// validateEKSCredentialAuth validates that a user has access to the credential they are setting and the credential
// they are overwriting. If there is no previous credential such as during a create or the old credential cannot
// be found, the auth check will succeed as long as the user can access the new credential.
func validateEKSCredentialAuth(request *types.APIContext, credential string, prevCluster *v3.Cluster) error {
	var accessCred mgmtclient.CloudCredential
	credentialErr := "error accessing cloud credential"
	if err := access.ByID(request, &mgmtSchema.Version, mgmtclient.CloudCredentialType, credential, &accessCred); err != nil {
		return httperror.NewAPIError(httperror.NotFound, credentialErr)
	}

	if prevCluster == nil {
		return nil
	}

	if prevCluster.Spec.EKSConfig == nil {
		return nil
	}

	// validate the user has access to the old cloud credential before allowing them to change it
	credential = prevCluster.Spec.EKSConfig.AmazonCredentialSecret
	if err := access.ByID(request, &mgmtSchema.Version, mgmtclient.CloudCredentialType, credential, &accessCred); err != nil {
		if apiError, ok := err.(*httperror.APIError); ok {
			if apiError.Code.Status == httperror.NotFound.Status {
				// old cloud credential doesn't exist anymore, anyone can change it
				return nil
			}
		}
		return httperror.NewAPIError(httperror.NotFound, credentialErr)
	}

	return nil
}

// validateEKSNodegroups checks whether a given nodegroup version is empty or not supported.
// More involved validation is performed in the EKS-operator.
func validateEKSNodegroups(spec *v32.ClusterSpec) error {
	nodegroups := spec.EKSConfig.NodeGroups
	if nodegroups == nil {
		return nil
	}
	if len(nodegroups) == 0 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("must have at least one nodegroup"))
	}

	var errors []string

	for _, ng := range nodegroups {
		name := aws.StringValue(ng.NodegroupName)
		if name == "" {
			return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("nodegroupName cannot be an empty"))
		}

		version := ng.Version
		if version == nil {
			continue
		}
		if aws.StringValue(version) == "" {
			errors = append(errors, fmt.Sprintf("nodegroup [%s] version cannot be empty string", name))
			continue
		}
	}

	if len(errors) != 0 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf(strings.Join(errors, ";")))
	}
	return nil
}

func validateEKS(prevCluster, newCluster map[string]interface{}) error {
	// check config is for EKS clusters
	if driver, ok := prevCluster["driverName"]; ok {
		if driver != service.AmazonElasticContainerServiceDriverName {
			return nil
		}
	}

	// don't allow for updating subnets
	if prev, ok := prevCluster["subnets"]; ok {
		if new, ok := newCluster["subnets"]; ok {
			if !reflect.DeepEqual(prev, new) {
				return httperror.NewAPIError(httperror.InvalidBodyContent, "cannot modify EKS subnets after creation")
			}
		}
	}
	return nil
}
