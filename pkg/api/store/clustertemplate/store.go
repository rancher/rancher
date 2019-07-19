package clustertemplate

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/api/customization/clustertemplate"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	mgmtSchema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	managementv3 "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
)

const (
	clusterTemplateLabelName = "io.cattle.field/clusterTemplateId"
)

func WrapStore(store types.Store, mgmt *config.ScaledContext) types.Store {
	storeWrapped := &Store{
		Store:     store,
		users:     mgmt.Management.Users(""),
		grbLister: mgmt.Management.GlobalRoleBindings("").Controller().Lister(),
		grLister:  mgmt.Management.GlobalRoles("").Controller().Lister(),
		ctLister:  mgmt.Management.ClusterTemplates("").Controller().Lister(),
	}
	return storeWrapped
}

type Store struct {
	types.Store
	users     v3.UserInterface
	grbLister v3.GlobalRoleBindingLister
	grLister  v3.GlobalRoleLister
	ctLister  v3.ClusterTemplateLister
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {

	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
		if data[managementv3.ClusterTemplateRevisionFieldClusterConfig] == nil {
			return nil, httperror.NewAPIError(httperror.MissingRequired, "ClusterTemplateRevision field ClusterConfig is required")
		}
		err := p.checkPermissionToCreateRevision(apiContext, data)
		if err != nil {
			return nil, err
		}
		err = p.checkKubernetesVersionFormat(apiContext, data)
		if err != nil {
			return nil, err
		}
		if err := setLabelsAndOwnerRef(apiContext, data); err != nil {
			return nil, err
		}
	}

	return p.Store.Create(apiContext, schema, data)
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {

	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
		err := p.checkKubernetesVersionFormat(apiContext, data)
		if err != nil {
			return nil, err
		}

		isUsed, err := isTemplateInUse(apiContext, id)
		if err != nil {
			return nil, err
		}
		if isUsed {
			return nil, httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("Cannot update the %v until Clusters are referring it", apiContext.Type))
		}
	}

	return p.Store.Update(apiContext, schema, data, id)
}

func (p *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {

	isUsed, err := isTemplateInUse(apiContext, id)
	if err != nil {
		return nil, err
	}
	if isUsed {
		return nil, httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("Cannot delete the %v until Clusters referring it are removed", apiContext.Type))
	}

	//check if template.DefaultRevisionId is set, if yes error out if the revision is being deleted.
	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
		isDefault, err := isDefaultTemplateRevision(apiContext, id)
		if err != nil {
			return nil, err
		}
		if isDefault {
			return nil, httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("Cannot delete the %v since this is the default revision of the Template, Please change the default revision first", apiContext.Type))
		}
	}

	return p.Store.Delete(apiContext, schema, id)
}

func setLabelsAndOwnerRef(apiContext *types.APIContext, data map[string]interface{}) error {
	var template managementv3.ClusterTemplate

	templateID := convert.ToString(data[managementv3.ClusterTemplateRevisionFieldClusterTemplateID])
	if err := access.ByID(apiContext, apiContext.Version, managementv3.ClusterTemplateType, templateID, &template); err != nil {
		return err
	}

	split := strings.SplitN(template.ID, ":", 2)
	if len(split) != 2 {
		return fmt.Errorf("error in splitting clusterTemplate ID %v", template.ID)
	}
	templateName := split[1]

	labels := map[string]string{
		clusterTemplateLabelName: templateName,
	}
	data["labels"] = labels

	var ownerReferencesSlice []map[string]interface{}
	ownerReference := map[string]interface{}{
		managementv3.OwnerReferenceFieldKind:       "ClusterTemplate",
		managementv3.OwnerReferenceFieldAPIVersion: "management.cattle.io/v3",
		managementv3.OwnerReferenceFieldName:       templateName,
		managementv3.OwnerReferenceFieldUID:        template.UUID,
	}
	ownerReferencesSlice = append(ownerReferencesSlice, ownerReference)
	data["ownerReferences"] = ownerReferencesSlice

	return nil
}

func isTemplateInUse(apiContext *types.APIContext, id string) (bool, error) {

	/*check if there are any clusters referencing this template or templateRevision */

	var clusters []managementv3.Cluster
	var field string

	switch apiContext.Type {
	case managementv3.ClusterTemplateType:
		field = managementv3.ClusterSpecFieldClusterTemplateID
	case managementv3.ClusterTemplateRevisionType:
		field = managementv3.ClusterSpecFieldClusterTemplateRevisionID
	}

	conditions := []*types.QueryCondition{
		types.NewConditionFromString(field, types.ModifierEQ, []string{id}...),
	}

	if err := access.List(apiContext, apiContext.Version, managementv3.ClusterType, &types.QueryOptions{Conditions: conditions}, &clusters); err != nil {
		return false, err
	}

	if len(clusters) > 0 {
		return true, nil
	}

	return false, nil
}

func isDefaultTemplateRevision(apiContext *types.APIContext, id string) (bool, error) {

	var template managementv3.ClusterTemplate
	var templateRevision managementv3.ClusterTemplateRevision

	if err := access.ByID(apiContext, apiContext.Version, managementv3.ClusterTemplateRevisionType, id, &templateRevision); err != nil {
		return false, err
	}

	if err := access.ByID(apiContext, apiContext.Version, managementv3.ClusterTemplateType, templateRevision.ClusterTemplateID, &template); err != nil {
		return false, err
	}

	if template.DefaultRevisionID == id {
		return true, nil
	}

	return false, nil
}

func (p *Store) checkPermissionToCreateRevision(apiContext *types.APIContext, data map[string]interface{}) error {
	value, found := values.GetValue(data, managementv3.ClusterTemplateRevisionFieldClusterTemplateID)
	if !found {
		return httperror.NewAPIError(httperror.NotFound, "invalid request: clusterTemplateID not found")
	}

	clusterTemplateID := convert.ToString(value)
	_, clusterTemplateName := ref.Parse(clusterTemplateID)
	var ctMap map[string]interface{}
	if err := access.ByID(apiContext, &mgmtSchema.Version, managementv3.ClusterTemplateType, clusterTemplateID, &ctMap); err != nil {
		return httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("unable to access clusterTemplate by id: %v", err))
	}
	if err := apiContext.AccessControl.CanDo(v3.ClusterTemplateGroupVersionKind.Group, v3.ClusterTemplateResource.Name, "update", apiContext, ctMap, apiContext.Schema); err != nil {
		return httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("user does not have permission to update clusterTemplate %s by creating a revision for it", clusterTemplateName))
	}
	return nil
}

func (p *Store) checkKubernetesVersionFormat(apiContext *types.APIContext, data map[string]interface{}) error {
	clusterConfig, found := values.GetValue(data, managementv3.ClusterTemplateRevisionFieldClusterConfig)
	if !found || clusterConfig == nil {
		return httperror.NewAPIError(httperror.MissingRequired, "ClusterTemplateRevision field ClusterConfig is required")
	}
	k8sVersionReq := values.GetValueN(data, managementv3.ClusterTemplateRevisionFieldClusterConfig, "rancherKubernetesEngineConfig", "kubernetesVersion")
	if k8sVersionReq == nil {
		return nil
	}
	k8sVersion := convert.ToString(k8sVersionReq)
	genericPatch, err := clustertemplate.CheckKubernetesVersionFormat(k8sVersion)
	if err != nil {
		return err
	}
	if genericPatch {
		//ensure a question is added for "rancherKubernetesEngineConfig.kubernetesVersion"
		templateQuestions, ok := data[managementv3.ClusterTemplateRevisionFieldQuestions]
		if !ok {
			return httperror.NewAPIError(httperror.MissingRequired, fmt.Sprintf("ClusterTemplateRevision must have a Question set for %v", clustertemplate.RKEConfigK8sVersion))
		}
		templateQuestionsSlice := convert.ToMapSlice(templateQuestions)
		var foundQ bool
		for _, question := range templateQuestionsSlice {
			if question["variable"] == clustertemplate.RKEConfigK8sVersion {
				foundQ = true
				break
			}
		}
		if !foundQ {
			return httperror.NewAPIError(httperror.MissingRequired, fmt.Sprintf("ClusterTemplateRevision must have a Question set for %v", clustertemplate.RKEConfigK8sVersion))
		}
	}
	return nil
}
