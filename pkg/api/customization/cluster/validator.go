package cluster

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/rancher/kontainer-engine/service"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	gaccess "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	"github.com/rancher/rancher/pkg/controllers/management/k3supgrade"
	"github.com/rancher/rancher/pkg/controllers/user/cis"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	mgmtSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	mgmtclient "github.com/rancher/types/client/management/v3"
	"github.com/robfig/cron"
)

type Validator struct {
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
	var clusterSpec v3.ClusterSpec
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

	if err := v.validateK3sVersionUpgrade(request, &clusterSpec); err != nil {
		return err
	}

	if err := v.validateScheduledClusterScan(&clientClusterSpec); err != nil {
		return err
	}

	if err := v.validateGenericEngineConfig(request, &clusterSpec); err != nil {
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
		if profile != string(v3.CisScanProfileTypePermissive) &&
			profile != string(v3.CisScanProfileTypeHardened) {
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

func (v *Validator) validateLocalClusterAuthEndpoint(request *types.APIContext, spec *v3.ClusterSpec) error {
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
			cluster.Status.Driver == v3.ClusterDriverRKE ||
			cluster.Status.Driver == v3.ClusterDriverImported
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
func (v *Validator) validateK3sVersionUpgrade(request *types.APIContext, spec *v3.ClusterSpec) error {
	upgradeNotReadyErr := httperror.NewAPIError(httperror.Conflict, "k3s version upgrade is not ready, try again later")

	if request.Method == http.MethodPost {
		return nil
	}

	if spec.K3sConfig == nil {
		// only applies to k3s clusters
		return nil
	}

	// must wait for original spec version to be set
	if spec.K3sConfig.Version == "" {
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

	prevVersion := cluster.Status.Version.GitVersion
	updateVersion := spec.K3sConfig.Version

	if prevVersion == updateVersion {
		// no op
		return nil
	}

	isNewer, err := k3supgrade.IsNewerVersion(prevVersion, updateVersion)
	if err != nil {
		errMsg := fmt.Sprintf("unable to compare k3s version [%s]", spec.K3sConfig.Version)
		return httperror.NewAPIError(httperror.InvalidBodyContent, errMsg)
	}

	if !isNewer {
		// update version must be higher than previous version, downgrades are not supported
		errMsg := fmt.Sprintf("cannot upgrade k3s cluster version from [%s] to [%s]. New version must be higher.", prevVersion, updateVersion)
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
func (v *Validator) validateGenericEngineConfig(request *types.APIContext, spec *v3.ClusterSpec) error {

	if request.Method == http.MethodPost {
		return nil
	}

	if spec.AmazonElasticContainerServiceConfig != nil {
		// compare with current cluster
		clusterName := request.ID
		prevCluster, err := v.ClusterLister.Get("", clusterName)
		if err != nil {
			return httperror.WrapAPIError(err, httperror.InvalidBodyContent, err.Error())
		}

		err = validateEKS(*prevCluster.Spec.GenericEngineConfig, *spec.AmazonElasticContainerServiceConfig)
		if err != nil {
			return err
		}
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
