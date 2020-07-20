package app

import (
	"fmt"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	pv3app "github.com/rancher/rancher/pkg/api/customization/app"
	catUtil "github.com/rancher/rancher/pkg/catalog/utils"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	hcommon "github.com/rancher/rancher/pkg/controllers/managementuser/helm/common"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	mgmtschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/namespace"
	"k8s.io/apimachinery/pkg/api/errors"
)

type Store struct {
	types.Store
	Apps                  pv3.AppLister
	TemplateVersionLister v3.CatalogTemplateVersionLister
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if err := s.checkAccessToTemplateVersion(apiContext, data); err != nil {
		return nil, err
	}

	if err := s.verifyAppExternalIDMatchesProject(data, ""); err != nil {
		return nil, err
	}

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
	if err := s.checkAccessToTemplateVersion(apiContext, data); err != nil {
		return nil, err
	}

	if err := s.verifyAppExternalIDMatchesProject(data, id); err != nil {
		return nil, err
	}

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

	return catUtil.ValidateRancherVersion(template)
}

func (s *Store) checkAccessToTemplateVersion(apiContext *types.APIContext, data map[string]interface{}) error {
	templateVersionID, ns, err := s.parseAppExternalID(data)
	if err != nil {
		return err
	}
	if templateVersionID == "" && ns == "" {
		// all users can use a local template to create apps
		return nil
	}
	if ns == namespace.GlobalNamespace {
		// all users have read access to global catalogs, and can use their template versions to create apps
		return nil
	}
	templateVersionID = ns + ":" + templateVersionID

	var templateVersion client.CatalogTemplateVersion
	if err := access.ByID(apiContext, &mgmtschema.Version, client.CatalogTemplateVersionType, templateVersionID, &templateVersion); err != nil {
		if apiError, ok := err.(*httperror.APIError); ok {
			if apiError.Code.Status == httperror.PermissionDenied.Status {
				return httperror.NewAPIError(httperror.NotFound, "Cannot find template version")
			}
		}
		return err
	}
	return nil
}

func (s *Store) verifyAppExternalIDMatchesProject(data map[string]interface{}, id string) error {
	_, catalogNs, err := s.parseAppExternalID(data)
	if err != nil {
		return err
	}
	if catalogNs == namespace.GlobalNamespace || catalogNs == "" {
		// apps from global catalog or local template can be launched in any cluster
		return nil
	}

	// check if target project is either same as the catalogNs (project scoped catalog), or belongs in the ns (cluster scoped catalog)
	projectID := convert.ToString(data["projectId"])
	if projectID == "" {
		// this can happen only during app edit, get projectID from app
		ns, name := ref.Parse(id)
		if ns == "" || name == "" {
			return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("app id [%s] passed during edit is invalid", id))
		}
		app, err := s.Apps.Get(ns, name)
		if err != nil {
			return fmt.Errorf("error getting app %s: %v", id, err)
		}
		projectID = app.Spec.ProjectName
	}
	clusterName, projectName := ref.Parse(projectID)
	if catalogNs == clusterName || catalogNs == projectName {
		return nil
	}
	return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("Cannot use catalog from %v to launch app in %v", catalogNs, projectID))
}

func (s *Store) parseAppExternalID(data map[string]interface{}) (string, string, error) {
	externalID := convert.ToString(data["externalId"])
	if externalID == "" {
		return "", "", nil
	}

	templateVersionID, ns, err := hcommon.ParseExternalID(externalID)
	if err != nil {
		return "", "", err
	}
	return templateVersionID, ns, nil
}
