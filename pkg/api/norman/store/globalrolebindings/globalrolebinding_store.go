package grbstore

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	GrbVersion    = "cleanup.cattle.io/grbUpgradeCluster"
	OldGrbVersion = "field.cattle.io/grbUpgrade"
)

func Wrap(store types.Store, grLister v3.GlobalRoleLister) types.Store {
	return &grbStore{
		Store:    store,
		grLister: grLister,
	}
}

type grbStore struct {
	types.Store

	grLister v3.GlobalRoleLister
}

func (s *grbStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	values.PutValue(data, "true", "annotations", GrbVersion)

	err := s.addOwnerReference(data)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, httperror.NewAPIError(httperror.NotFound, err.Error())
		}
		return nil, err
	}

	return s.Store.Create(apiContext, schema, data)
}

// addOwnerReference ensures that a globalRolebinding will be deleted when the role it references
// is deleted
func (s *grbStore) addOwnerReference(data map[string]interface{}) error {
	globalRoleName, _ := data["globalRoleId"].(string)

	globalRole, err := s.grLister.Get("", globalRoleName)
	if err != nil {
		return err
	}

	ownerReference := v1.OwnerReference{
		APIVersion: globalRole.APIVersion,
		Kind:       globalRole.Kind,
		Name:       globalRole.Name,
		UID:        globalRole.UID,
	}
	values.PutValue(data, []v1.OwnerReference{ownerReference}, "ownerReferences")

	return nil
}
