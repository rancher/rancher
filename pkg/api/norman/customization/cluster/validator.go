package cluster

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2020-11-01/containerservice"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	gaccess "github.com/rancher/rancher/pkg/api/norman/customization/globalnamespaceaccess"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtclient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/controllers/managementuser/nodesyncer"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/service"
	"github.com/rancher/rancher/pkg/namespace"
	mgmtSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
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

	if err := v.validateGenericEngineConfig(request, &clusterSpec); err != nil {
		return err
	}

	if err := v.validateAKSConfig(request, data, &clusterSpec); err != nil {
		return err
	}

	if err := v.validateEKSConfig(request, data, &clusterSpec); err != nil {
		return err
	}

	return v.validateGKEConfig(request, data, &clusterSpec)
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
			cluster.Status.Driver == v32.ClusterDriverImported ||
			cluster.Status.Driver == v32.ClusterDriverK3s ||
			cluster.Status.Driver == v32.ClusterDriverRke2
	}
	if !isValidCluster {
		return httperror.NewFieldAPIError(httperror.InvalidState, "LocalClusterAuthEndpoint.Enabled", "Can only enable LocalClusterAuthEndpoint with RKE, RKE2, or K3s")
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

	if isK3s && cluster.Spec.K3sConfig == nil {
		// prevents embedded cluster from have k3sConfig set. Embedded cluster cannot be upgraded. Non-embedded
		// clusters' config will be set my controller.
		return httperror.NewAPIError(httperror.InvalidBodyContent, "k3sConfig cannot be changed from nil")
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

	isNewer, err := nodesyncer.IsNewerVersion(prevVersion, updateVersion)
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
	return access.ByID(request, &mgmtSchema.Version, mgmtclient.ClusterTemplateType, clusterTempRev.Spec.ClusterTemplateName, &ctMap)
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

func (v *Validator) validateAKSConfig(request *types.APIContext, cluster map[string]interface{}, clusterSpec *v32.ClusterSpec) error {
	aksConfig, ok := cluster["aksConfig"].(map[string]interface{})
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
	if azureCredential, ok := aksConfig["azureCredentialSecret"].(string); ok && (prevCluster == nil || azureCredential != prevCluster.Spec.AKSConfig.AzureCredentialSecret) {
		// Only check that the user has access to the credential if the credential is being changed.
		if err := validateCredentialAuth(request, azureCredential); err != nil {
			return err
		}
	}

	if err := v.validateAKSNetworkPolicy(clusterSpec, prevCluster); err != nil {
		return err
	}

	createFromImport := request.Method == http.MethodPost && aksConfig["imported"] == true

	if !createFromImport {
		if err := validateAKSKubernetesVersion(clusterSpec, prevCluster); err != nil {
			return err
		}
		if err := validateAKSNodePools(clusterSpec); err != nil {
			return err
		}
	}

	if request.Method != http.MethodPost {
		return nil
	}

	// validation for creates only
	if err := validateAKSClusterName(v.ClusterClient, clusterSpec); err != nil {
		return err
	}

	region, regionOk := aksConfig["resourceLocation"]
	if !regionOk || region == "" {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "must provide region")
	}

	return nil
}

// validateAKSKubernetesVersion checks whether a kubernetes version is provided
func validateAKSKubernetesVersion(spec *v32.ClusterSpec, prevCluster *v3.Cluster) error {
	clusterVersion := spec.AKSConfig.KubernetesVersion
	if clusterVersion == nil {
		return nil
	}

	if to.String(clusterVersion) == "" {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "cluster kubernetes version cannot be empty string")
	}

	return nil
}

// validateAKSNodePools checks whether a given NodePool version is empty or not supported.
// More involved validation is performed in the aks-operator.
func validateAKSNodePools(spec *v32.ClusterSpec) error {
	nodePools := spec.AKSConfig.NodePools
	if nodePools == nil {
		return nil
	}
	if len(nodePools) == 0 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "must have at least one nodepool")
	}

	for _, np := range nodePools {
		name := np.Name
		if to.String(name) == "" {
			return httperror.NewAPIError(httperror.InvalidBodyContent, "nodePool Name cannot be an empty string")
		}
		if np.OsType == "Windows" {
			return httperror.NewAPIError(httperror.InvalidBodyContent, "windows node pools are not supported")
		}
	}

	return nil
}

func validateAKSClusterName(client v3.ClusterInterface, spec *v32.ClusterSpec) error {
	// validate cluster does not reference an AKS cluster that is already backed by a Rancher cluster
	name := spec.AKSConfig.ClusterName
	region := spec.AKSConfig.ResourceLocation
	msgSuffix := fmt.Sprintf("in region [%s]", region)

	// cluster client is being used instead of lister to avoid the use of an outdated cache
	clusters, err := client.List(metav1.ListOptions{})
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, "failed to confirm clusterName is unique among Rancher AKS clusters "+msgSuffix)
	}

	for _, cluster := range clusters.Items {
		if cluster.Spec.AKSConfig == nil {
			continue
		}
		if name != cluster.Spec.AKSConfig.ClusterName {
			continue
		}
		if region != "" && region != cluster.Spec.AKSConfig.ResourceLocation {
			continue
		}

		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("cluster already exists for AKS cluster [%s] "+msgSuffix, name))
	}
	return nil
}

// validateAKSNetworkPolicy performs validation around setting enableNetworkPolicy on AKS clusters which turns on Project Network Isolation
func (v *Validator) validateAKSNetworkPolicy(clusterSpec *v32.ClusterSpec, prevCluster *v3.Cluster) error {
	// determine if network policy is enabled on the AKS cluster by checking the cluster spec and then the upstream spec if the field is nil (unmanaged)
	var networkPolicy string
	if clusterSpec.AKSConfig != nil && clusterSpec.AKSConfig.NetworkPolicy != nil {
		networkPolicy = *clusterSpec.AKSConfig.NetworkPolicy
	} else if prevCluster != nil && prevCluster.Status.AKSStatus.UpstreamSpec != nil && prevCluster.Status.AKSStatus.UpstreamSpec.NetworkPolicy != nil {
		networkPolicy = *prevCluster.Status.AKSStatus.UpstreamSpec.NetworkPolicy
	} else {
		return nil
	}

	// network policy enabled on the AKS cluster is a prerequisite for PNI
	if to.Bool(clusterSpec.EnableNetworkPolicy) && networkPolicy != string(containerservice.NetworkPolicyAzure) && networkPolicy != string(containerservice.NetworkPolicyCalico) {
		return httperror.NewAPIError(
			httperror.InvalidBodyContent,
			"Network Policy support must be enabled on AKS cluster in order to enable Project Network Isolation",
		)
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
	if amazonCredential, ok := eksConfig["amazonCredentialSecret"].(string); ok && (prevCluster == nil || amazonCredential != prevCluster.Spec.EKSConfig.AmazonCredentialSecret) {
		// Only check that the user has access to the credential if the credential is being changed.
		if err := validateCredentialAuth(request, amazonCredential); err != nil {
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
		// If security groups are provided, then subnets must also be provided
		securityGroups, _ := eksConfig["securityGroups"].([]interface{})
		subnets, _ := eksConfig["subnets"].([]interface{})

		if len(securityGroups) != 0 && len(subnets) == 0 {
			return httperror.NewAPIError(httperror.InvalidBodyContent,
				"subnets must be provided if security groups are provided")
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

// validateCredentialAuth validates that a user has access to the credential they are setting.
func validateCredentialAuth(request *types.APIContext, credential string) error {
	var accessCred mgmtclient.CloudCredential
	credentialErr := "error accessing cloud credential"
	if err := access.ByID(request, &mgmtSchema.Version, mgmtclient.CloudCredentialType, credential, &accessCred); err != nil {
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
	prev, _ := prevCluster["subnets"].([]interface{})
	new, _ := newCluster["subnets"].([]interface{})
	if len(prev) == 0 && len(new) == 0 {
		// should treat empty and nil as equal
		return nil
	}
	if !reflect.DeepEqual(prev, new) {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "cannot modify EKS subnets after creation")
	}
	return nil
}

func (v *Validator) validateGKEConfig(request *types.APIContext, cluster map[string]interface{}, clusterSpec *v32.ClusterSpec) error {
	gkeConfig, ok := cluster["gkeConfig"].(map[string]interface{})
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
	if googleCredential, ok := gkeConfig["googleCredentialSecret"].(string); ok && (prevCluster == nil || googleCredential != prevCluster.Spec.GKEConfig.GoogleCredentialSecret) {
		// Only check that the user has access to the credential if the credential is being changed.
		if err := validateCredentialAuth(request, googleCredential); err != nil {
			return err
		}
	}

	if err := v.validateGKENetworkPolicy(clusterSpec, prevCluster); err != nil {
		return err
	}

	createFromImport := request.Method == http.MethodPost && gkeConfig["imported"] == true
	if !createFromImport {
		if err := validateGKEKubernetesVersion(clusterSpec, prevCluster); err != nil {
			return err
		}
		if err := validateGKENodePools(clusterSpec); err != nil {
			return err
		}
	}

	if request.Method != http.MethodPost {
		return nil
	}

	// validation for creates only

	if err := validateGKEClusterName(v.ClusterClient, clusterSpec); err != nil {
		return err
	}

	if err := validateGKEPrivateClusterConfig(clusterSpec); err != nil {
		return err
	}

	region, regionOk := gkeConfig["region"]
	zone, zoneOk := gkeConfig["zone"]
	if (!regionOk || region == "") && (!zoneOk || zone == "") {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "must provide region or zone")
	}

	return nil
}

// validateGKENetworkPolicy performs validation around setting enableNetworkPolicy on GKE clusters which turns on Project Network Isolation
func (v *Validator) validateGKENetworkPolicy(clusterSpec *v32.ClusterSpec, prevCluster *v3.Cluster) error {
	// determine if network policy is enabled on the GKE cluster by checking the cluster spec and then the upstream spec if the field is nil (unmanaged)
	var netPolEnabled bool
	if clusterSpec.GKEConfig != nil && clusterSpec.GKEConfig.NetworkPolicyEnabled != nil {
		netPolEnabled = *clusterSpec.GKEConfig.NetworkPolicyEnabled
	} else if prevCluster != nil && prevCluster.Status.GKEStatus.UpstreamSpec != nil && prevCluster.Status.GKEStatus.UpstreamSpec.NetworkPolicyEnabled != nil {
		netPolEnabled = *prevCluster.Status.GKEStatus.UpstreamSpec.NetworkPolicyEnabled
	} else {
		return nil
	}

	// network policy enabled on the GKE cluster is a prerequisite for PNI
	if to.Bool(clusterSpec.EnableNetworkPolicy) && !netPolEnabled {
		return httperror.NewAPIError(
			httperror.InvalidBodyContent,
			"Network Policy support must be enabled on GKE cluster in order to enable Project Network Isolation",
		)
	}

	return nil
}

// validateGKEKubernetesVersion checks whether a kubernetes version is provided and if it is supported
func validateGKEKubernetesVersion(spec *v32.ClusterSpec, prevCluster *v3.Cluster) error {
	clusterVersion := spec.GKEConfig.KubernetesVersion
	if clusterVersion == nil {
		return nil
	}

	if *clusterVersion == "" {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "cluster kubernetes version cannot be empty string")
	}

	return nil
}

// validateGKENodePools checks whether a given node pool version is empty or not supported.
func validateGKENodePools(spec *v32.ClusterSpec) error {
	nodepools := spec.GKEConfig.NodePools
	if nodepools == nil {
		return nil
	}
	if len(nodepools) == 0 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("must have at least one node pool"))
	}

	var errors []string
	hasRequiredLinuxPool := false

	for _, np := range nodepools {
		if np.Name == nil || *np.Name == "" {
			return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("node pool name cannot be empty"))
		}

		version := np.Version
		if version == nil || *version == "" {
			errors = append(errors, fmt.Sprintf("node pool [%s] version cannot be empty", *np.Name))
			continue
		}

		// Windows images are WINDOWS_LTSC or WINDOWS_SAC. The cluster must have at least one non-Windows node pool.
		if !hasRequiredLinuxPool && !strings.Contains(strings.ToLower(np.Config.ImageType), "windows") {
			hasRequiredLinuxPool = true
		}
	}

	if !hasRequiredLinuxPool {
		errors = append(errors, fmt.Sprintf("at least 1 Linux node pool is required"))
	}

	if len(errors) != 0 {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf(strings.Join(errors, ";")))
	}
	return nil
}

func validateGKEClusterName(client v3.ClusterInterface, spec *v32.ClusterSpec) error {
	// validate cluster does not reference an GKE cluster that is already backed by a Rancher cluster
	name := spec.GKEConfig.ClusterName
	region := spec.GKEConfig.Region
	zone := spec.GKEConfig.Zone
	msgSuffix := fmt.Sprintf("in region [%s]", region)
	if region == "" {
		msgSuffix = fmt.Sprintf("in zone [%s]", spec.GKEConfig.Zone)
	}

	// cluster client is being used instead of lister to avoid the use of an outdated cache
	clusters, err := client.List(metav1.ListOptions{})
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, "failed to confirm clusterName is unique among Rancher GKE clusters "+msgSuffix)
	}

	for _, cluster := range clusters.Items {
		if cluster.Spec.GKEConfig == nil {
			continue
		}
		if name != cluster.Spec.GKEConfig.ClusterName {
			continue
		}
		if region != "" && region != cluster.Spec.GKEConfig.Region {
			continue
		}
		if zone != "" && zone != cluster.Spec.GKEConfig.Zone {
			continue
		}
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("cluster already exists for GKE cluster [%s] "+msgSuffix, name))
	}
	return nil
}

func validateGKEPrivateClusterConfig(spec *v32.ClusterSpec) error {
	if spec.GKEConfig.PrivateClusterConfig != nil && spec.GKEConfig.PrivateClusterConfig.EnablePrivateEndpoint && !spec.GKEConfig.PrivateClusterConfig.EnablePrivateNodes {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("private endpoint requires private nodes"))
	}
	return nil
}
