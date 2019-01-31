package app

import (
	"fmt"
	"github.com/rancher/norman/types"
	pv3app "github.com/rancher/rancher/pkg/api/customization/app"
	"github.com/rancher/rancher/pkg/ref"
	pv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"k8s.io/apimachinery/pkg/api/errors"
)

type Store struct {
	types.Store
	Apps pv3.AppLister
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	if err := s.validateForMultiClusterApp(id, "delete"); err != nil {
		return nil, err
	}
	return s.Store.Delete(apiContext, schema, id)
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
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
