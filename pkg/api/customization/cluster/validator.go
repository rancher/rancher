package cluster

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/rancher/kontainer-engine/service"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	gaccess "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	mgmtSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	mgmtclient "github.com/rancher/types/client/management/v3"
)

type Validator struct {
	ClusterLister                 v3.ClusterLister
	ClusterTemplateLister         v3.ClusterTemplateLister
	ClusterTemplateRevisionLister v3.ClusterTemplateRevisionLister
	Users                         v3.UserInterface
	GrbLister                     v3.GlobalRoleBindingLister
	GrLister                      v3.GlobalRoleLister
}

func (v *Validator) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var spec v3.ClusterSpec

	if err := convert.ToObj(data, &spec); err != nil {
		return httperror.WrapAPIError(err, httperror.InvalidBodyContent, "Cluster spec conversion error")
	}

	if err := v.validateEnforcement(request, data); err != nil {
		return err
	}

	if err := v.validateLocalClusterAuthEndpoint(request, &spec); err != nil {
		return err
	}
	if err := v.validateGenericEngineConfig(request, &spec); err != nil {
		return err
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

	canAccess, err := v.isTemplateAccessible(request, &spec)
	if err != nil {
		return err
	}

	if !canAccess {
		return httperror.NewFieldAPIError(httperror.PermissionDenied, "", "No permission to access clusterTemplateRevision")
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

func (v *Validator) isTemplateAccessible(request *types.APIContext, spec *mgmtclient.Cluster) (bool, error) {
	split := strings.SplitN(spec.ClusterTemplateRevisionID, ":", 2)
	if len(split) != 2 {
		return false, fmt.Errorf("error in splitting clusterTemplateRevision name %v", spec.ClusterTemplateRevisionID)
	}
	revName := split[1]
	clusterTempRev, err := v.ClusterTemplateRevisionLister.Get(namespace.GlobalNamespace, revName)
	if err != nil {
		return false, err
	}

	var ctMap map[string]interface{}
	if err := access.ByID(request, &mgmtSchema.Version, mgmtclient.ClusterTemplateType, clusterTempRev.Spec.ClusterTemplateName, &ctMap); err != nil {
		return false, httperror.WrapAPIError(err, httperror.PermissionDenied, fmt.Sprintf("unable to access clusterTemplate by id: %v", err))
	}

	return true, nil
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
