package clustertemplate

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	gaccess "github.com/rancher/rancher/pkg/api/customization/globalnamespaceaccess"
	rrbac "github.com/rancher/rancher/pkg/controllers/management/globalnamespacerbac"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	managementv3 "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		rbLister:  mgmt.RBAC.RoleBindings("").Controller().Lister(),
	}
	return storeWrapped
}

type Store struct {
	types.Store
	users     v3.UserInterface
	grbLister v3.GlobalRoleBindingLister
	grLister  v3.GlobalRoleLister
	rbLister  rbacv1.RoleBindingLister
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {

	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateType) {
		if err := p.canSetEnforce(apiContext, data, ""); err != nil {
			return nil, err
		}
	}

	if strings.EqualFold(apiContext.Type, managementv3.ClusterTemplateRevisionType) {
		err := p.checkPermissionToCreateRevision(apiContext, data)
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

func (p *Store) checkPermissionToCreateRevision(apiContext *types.APIContext, data map[string]interface{}) error {
	userID := apiContext.Request.Header.Get("Impersonate-User")
	if userID == "" {
		return httperror.NewAPIError(httperror.NotFound, "invalid request: userID not found")
	}

	value, found := values.GetValue(data, managementv3.ClusterTemplateRevisionFieldClusterTemplateID)
	if !found {
		return httperror.NewAPIError(httperror.NotFound, "invalid request: clusterTemplateID not found")
	}

	clusterTemplateID := convert.ToString(value)
	_, clusterTemplateName := ref.Parse(clusterTemplateID)

	// check if rolebindings of type owner or member exist for this user for this clustertemplate
	// if yes, then user is allowed to create revision for this template
	// else not allowed since, either user has no access or only read-only access by either:
	// 1. being added explicitly as read-only member OR
	// 2. template is public, which gives everyone read-only access
	canUpdate := false
	for _, accessType := range []string{rrbac.OwnerAccess, rrbac.MemberAccess} {
		rbName, _ := rrbac.GetRoleNameAndVerbs(accessType, clusterTemplateName, rrbac.ClusterTemplateResource)
		// check if rolebinding with this name exists and has the user (caller of this request) as a subject
		rb, err := p.rbLister.Get(namespace.GlobalNamespace, rbName)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if rb == nil {
			continue
		}
		for _, sub := range rb.Subjects {
			if sub.Name == userID {
				// user is either owner or member
				canUpdate = true
				break
			}
		}
		if canUpdate {
			break
		}
	}

	if !canUpdate {
		return httperror.NewAPIError(httperror.PermissionDenied, "read-only member of clustertemplate cannot create revisions for it")
	}
	return nil
}
