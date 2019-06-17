package clustertemplate

import (
	"strings"

	"fmt"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	managementv3 "github.com/rancher/types/client/management/v3"
)

const (
	clusterTemplateLabelName = "io.cattle.field/clusterTemplateId"
)

func WrapStore(store types.Store) types.Store {
	storeWrapped := &Store{
		Store: store,
	}
	return storeWrapped
}

type Store struct {
	types.Store
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {

	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
		if err := setLabelsAndOwnerRef(apiContext, data); err != nil {
			return nil, err
		}
	}

	return p.Store.Create(apiContext, schema, data)
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {

	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
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

	labels := map[string]string{
		clusterTemplateLabelName: template.Name,
	}
	data["labels"] = labels

	var ownerReferencesSlice []map[string]interface{}
	ownerReference := map[string]interface{}{
		managementv3.OwnerReferenceFieldKind:       "ClusterTemplate",
		managementv3.OwnerReferenceFieldAPIVersion: "management.cattle.io/v3",
		managementv3.OwnerReferenceFieldName:       template.Name,
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
