package app

import (
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	pv3app "github.com/rancher/rancher/pkg/api/customization/app"
	"github.com/rancher/rancher/pkg/catalog/catutils"
	hcommon "github.com/rancher/rancher/pkg/controllers/user/helm/common"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	pv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
)

type Store struct {
	types.Store
	Apps                  pv3.AppLister
	TemplateVersionLister v3.CatalogTemplateVersionLister
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if err := s.validateRancherVersion(data); err != nil {
		return nil, err
	}

	return s.Store.Create(apiContext, schema, data)
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	if err := s.validateForMultiClusterApp(id, "delete"); err != nil {
		return nil, err
	}
	return s.Store.Delete(apiContext, schema, id)
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if err := s.validateRancherVersion(data); err != nil {
		return nil, err
	}

	if err := s.validateForMultiClusterApp(id, "update"); err != nil {
		return nil, err
	}

	return s.Store.Update(apiContext, schema, data, id)
}

func (s *Store) validateForMultiClusterApp(id string, msg string) error {
	ns, name := ref.Parse(id)
	if ns == "" || name == "" {
		return fmt.Errorf("invalid app id %s", id)
	}
	app, err := s.Apps.Get(ns, name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error getting app %s: %v", id, err)
		}
		return nil
	}
	if label, ok := app.Labels[pv3app.MCappLabel]; ok && label != "" {
		return fmt.Errorf("app %s is controlled by mcapp %s : cannot be %sd", id, label, msg)
	}
	return nil
}

func (s *Store) validateRancherVersion(data map[string]interface{}) error {
	externalID := convert.ToString(data["externalId"])
	if externalID == "" {
		return nil
	}

	templateVersionID, namespace, err := hcommon.ParseExternalID(externalID)
	if err != nil {
		return err
	}

	template, err := s.TemplateVersionLister.Get(namespace, templateVersionID)
	if err != nil {
		return err
	}

	return catutils.ValidateRancherVersion(template)
}
