package catalog

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	c "github.com/rancher/rancher/pkg/api/norman/customization/catalog"
	gaccess "github.com/rancher/rancher/pkg/api/norman/customization/globalnamespaceaccess"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
)

const catalogSecretKey = "credentialSecret"

type Store struct {
	types.Store
	Users          v3.UserInterface
	GrbLister      v3.GlobalRoleBindingLister
	GrLister       v3.GlobalRoleLister
	secretMigrator *secretmigrator.Migrator
	clusterLister  v3.ClusterLister
}

func Wrap(store types.Store, users v3.UserInterface, grbLister v3.GlobalRoleBindingLister, grLister v3.GlobalRoleLister, secretLister v1.SecretLister, secrets v1.SecretInterface, clusterLister v3.ClusterLister) types.Store {
	return &Store{
		Store:          store,
		Users:          users,
		GrbLister:      grbLister,
		GrLister:       grLister,
		secretMigrator: secretmigrator.NewMigrator(secretLister, secrets),
		clusterLister:  clusterLister,
	}
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	password, _ := data["password"].(string)
	secret, err := s.secretMigrator.CreateOrUpdateCatalogSecret("", password, nil)
	if err != nil {
		return nil, err
	}
	if secret != nil {
		data[catalogSecretKey] = secret.Name
		data["password"] = ""
	}
	data, err = s.Store.Create(apiContext, schema, data)
	if err != nil {
		if secret != nil {
			if cleanupErr := s.secretMigrator.Cleanup(secret.Name); cleanupErr != nil {
				logrus.Errorf("catalog store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
			}
		}
		return nil, err
	}
	if secret == nil {
		return data, nil
	}
	var cluster *v3.Cluster
	if clusterID, ok := data["clusterId"]; ok {
		cluster, err = s.clusterLister.Get("", clusterID.(string))
	} else if projectID, ok := data["projectId"]; ok {
		clusterID, _ := ref.Parse(projectID.(string))
		cluster, err = s.clusterLister.Get("", clusterID)
	}
	if err != nil {
		return nil, err
	}
	var owner metav1.OwnerReference
	if cluster != nil {
		owner = metav1.OwnerReference{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Cluster",
			Name:       cluster.Name,
			UID:        cluster.UID,
		}
	} else {
		owner = metav1.OwnerReference{
			APIVersion: "management.cattle.io/v3",
			Kind:       "Catalog",
			Name:       data["id"].(string),
			UID:        k8sTypes.UID(data["uuid"].(string)),
		}
	}
	err = s.secretMigrator.UpdateSecretOwnerReference(secret, owner)
	if err != nil {
		logrus.Errorf("catalog store: failed to set %s %s as secret owner", owner.Kind, owner.Name)
	}
	return data, nil
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	isSystemCatalog, err := s.isSystemCatalog(apiContext, schema, id)
	if err != nil {
		return nil, err
	}
	if isSystemCatalog {
		return nil, httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprint("not allowed to delete system-library catalog"))
	}
	existing, err := s.ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}
	_, isCluster := existing["clusterId"]
	_, isProject := existing["projectId"]
	// global catalogs are owners of the secret so it will automatically be cleaned up
	if isCluster || isProject {
		if secretName, ok := existing[catalogSecretKey]; ok {
			err := s.secretMigrator.Cleanup(secretName.(string))
			if err != nil {
				return nil, err
			}
		}
	}
	return s.Store.Delete(apiContext, schema, id)
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	isSystemCatalog, err := s.isSystemCatalog(apiContext, schema, id)
	if err != nil {
		return nil, err
	}
	if isSystemCatalog {
		isRestrictedAdmin, err := s.isRestrictedAdmin(apiContext)
		if err != nil {
			return nil, err
		}
		if strings.ToLower(settings.SystemCatalog.Get()) == "bundled" || isRestrictedAdmin {
			return nil, httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprint("not allowed to edit system-library catalog"))
		}
	}
	existing, err := s.ByID(apiContext, schema, id)
	if err != nil {
		return nil, err
	}
	currentSecret, _ := existing[catalogSecretKey].(string)
	password, _ := data["password"].(string)
	secret, err := s.secretMigrator.CreateOrUpdateCatalogSecret(currentSecret, password, nil)
	if err != nil {
		return nil, err
	}
	if secret != nil {
		data[catalogSecretKey] = secret.Name
		data["password"] = ""
	}
	data, err = s.Store.Update(apiContext, schema, data, id)
	if err != nil {
		if secret != nil && currentSecret == "" {
			if cleanupErr := s.secretMigrator.Cleanup(secret.Name); cleanupErr != nil {
				logrus.Errorf("catalog store: encountered error while handling migration error: %v, original error: %v", cleanupErr, err)
			}
		}
	}
	return data, err
}

func (s *Store) isRestrictedAdmin(apiContext *types.APIContext) (bool, error) {
	ma := gaccess.MemberAccess{
		Users:     s.Users,
		GrLister:  s.GrLister,
		GrbLister: s.GrbLister,
	}
	callerID := apiContext.Request.Header.Get(gaccess.ImpersonateUserHeader)

	return ma.IsRestrictedAdmin(callerID)
}

// isSystemCatalog checks whether the catalog is the the system catalog maintained by rancher
func (s *Store) isSystemCatalog(apiContext *types.APIContext, schema *types.Schema, id string) (bool, error) {
	catalog, err := s.ByID(apiContext, schema, id)
	if err != nil {
		return false, err
	}
	if catalog["url"] == c.SystemLibraryURL && catalog["name"] == c.SystemCatalogName {
		return true, nil
	}
	return false, nil
}
