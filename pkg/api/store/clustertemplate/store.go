package clustertemplate

import (
	"strings"

	"fmt"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	gaccess "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementv3 "github.com/rancher/types/client/management/v3"
)

const (
	clusterTemplateLabelName = "io.cattle.field/clusterTemplateId"
)

func WrapStore(store types.Store, users v3.UserInterface, grbLister v3.GlobalRoleBindingLister, grLister v3.GlobalRoleLister) types.Store {
	storeWrapped := &Store{
		Store:     store,
		users:     users,
		grbLister: grbLister,
		grLister:  grLister,
	}
	return storeWrapped
}

type Store struct {
	types.Store
	users     v3.UserInterface
	grbLister v3.GlobalRoleBindingLister
	grLister  v3.GlobalRoleLister
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {

	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateType) {
		if err := p.canSetEnforce(apiContext, data, ""); err != nil {
			return nil, err
		}
	}

	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
		if err := setLabelsAndOwnerRef(apiContext, data); err != nil {
			return nil, err
		}
	}

	return p.Store.Create(apiContext, schema, data)
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {

	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateType) {
		if err := p.canSetEnforce(apiContext, data, id); err != nil {
			return nil, err
		}
	}

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

func (p *Store) canSetEnforce(apiContext *types.APIContext, data map[string]interface{}, templateID string) error {
	//check if turning on enforced flag

	enforcedFlagInData := convert.ToBool(data[managementv3.ClusterTemplateFieldEnforced])
	enforcedFlagChanged := enforcedFlagInData

	if templateID != "" {
		var template managementv3.ClusterTemplate
		if err := access.ByID(apiContext, apiContext.Version, managementv3.ClusterTemplateType, templateID, &template); err != nil {
			return err
		}
		if template.Enforced != enforcedFlagInData {
			enforcedFlagChanged = true
		}
	}
	if enforcedFlagChanged {
		//only admin can set the flag
		ma := gaccess.MemberAccess{
			Users:     p.users,
			GrLister:  p.grLister,
			GrbLister: p.grbLister,
		}
		callerID := apiContext.Request.Header.Get(gaccess.ImpersonateUserHeader)
		isAdmin, err := ma.IsAdmin(callerID)
		if err != nil {
			return err
		}
		if !isAdmin {
			return httperror.NewAPIError(httperror.PermissionDenied, fmt.Sprintf("ClusterTemplate's %v field cannot be changed", managementv3.ClusterTemplateFieldEnforced))
		}
	}

	return nil
}
