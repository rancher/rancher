package userscope

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/core/v1"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/client/management/v3"
	corev1 "k8s.io/api/core/v1"
	k8srbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	NamespaceID        = client.PreferenceFieldNamespaceId
	rolebindingPrefix  = "rolebinding-"
	userScopedRoleName = "cattle-user-scoped-resources"
)

type Store struct {
	Store             types.Store
	nsClient          v1.NamespaceInterface
	rbClient          rbacv1.RoleBindingInterface
	clusterRoleLister rbacv1.ClusterRoleLister
	crClient          rbacv1.ClusterRoleInterface
}

func NewStore(nsClient v1.NamespaceInterface,
	rbClient rbacv1.RoleBindingInterface,
	clusterRoleLister rbacv1.ClusterRoleLister,
	crClient rbacv1.ClusterRoleInterface,
	store types.Store) *Store {
	return &Store{
		Store:             store,
		nsClient:          nsClient,
		rbClient:          rbClient,
		clusterRoleLister: clusterRoleLister,
		crClient:          crClient,
	}
}

func (s *Store) Context() types.StorageContext {
	return s.Store.Context()
}

func (s *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	user, err := getUser(apiContext)
	if err != nil {
		return nil, err
	}

	return s.Store.ByID(apiContext, schema, addNamespace(user, id))
}

func (s *Store) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	user, err := getUser(apiContext)
	if err != nil {
		return nil, err
	}

	if opt == nil {
		return nil, nil
	}

	opt.Conditions = append(opt.Conditions, types.EQ(NamespaceID, getNamespace(user)))
	return s.Store.List(apiContext, schema, opt)
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	user, err := getUser(apiContext)
	if err != nil || data == nil {
		return nil, err
	}

	ns, err := s.ensureNamespace(user)
	if err != nil {
		return nil, err
	}

	if err = s.ensureRole(); err != nil {
		return nil, err
	}

	if err = s.ensureRoleBinding(user, ns); err != nil {
		return nil, err
	}

	data[NamespaceID] = ns
	return s.Store.Create(apiContext, schema, data)
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	user, err := getUser(apiContext)
	if err != nil {
		return nil, err
	}

	return s.Store.Update(apiContext, schema, data, addNamespace(user, id))
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	user, err := getUser(apiContext)
	if err != nil {
		return nil, err
	}

	return s.Store.Delete(apiContext, schema, addNamespace(user, id))
}

func (s *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	user, err := getUser(apiContext)
	if err != nil {
		return nil, err
	}

	if opt == nil {
		return nil, nil
	}

	opt.Conditions = append(opt.Conditions, types.EQ(NamespaceID, getNamespace(user)))
	return s.Store.Watch(apiContext, schema, opt)
}

func getUser(apiContext *types.APIContext) (string, error) {
	user := apiContext.Request.Header.Get("Impersonate-User")
	if user == "" {
		return "", httperror.NewAPIError(httperror.NotFound, "missing user")
	}
	return user, nil
}

func addNamespace(user, id string) string {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) == 1 {
		return fmt.Sprintf("%s:%s", getNamespace(user), parts[0])
	}
	return fmt.Sprintf("%s:%s", getNamespace(user), parts[1])
}

func getNamespace(user string) string {
	return user
}

func getRolebinding(user string) string {
	return rolebindingPrefix + user
}

func getClusterRole() *k8srbacv1.ClusterRole {
	return &k8srbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: userScopedRoleName,
		},
		Rules: []k8srbacv1.PolicyRule{
			k8srbacv1.PolicyRule{
				Verbs:     []string{"*"},
				APIGroups: []string{"management.cattle.io"},
				Resources: []string{"preferences", "nodetemplates", "sourcecodecredentials", "sourcecoderepositories"},
			},
		},
	}
}

func (s *Store) ensureNamespace(user string) (string, error) {
	ns := getNamespace(user)
	_, err := s.nsClient.Get(ns, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return "", err
		}
		if _, err = s.nsClient.Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"management.cattle.io/system-namespace": "true",
				},
				Name: ns,
			},
		}); err != nil {
			return "", err
		}
	}
	return ns, nil
}

func (s *Store) ensureRole() error {
	role := getClusterRole()
	if _, err := s.clusterRoleLister.Get("", role.Name); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		if _, err = s.crClient.Create(role); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureRoleBinding(user, namespace string) error {
	rb := getRolebinding(user)
	_, err := s.rbClient.Get(rb, metav1.GetOptions{})
	if err != nil {
		if _, err = s.rbClient.Create(&k8srbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"management.cattle.io/system-rolebinding": "true",
				},
				Name:      rb,
				Namespace: namespace,
			},
			RoleRef: k8srbacv1.RoleRef{
				APIGroup: rbacv1.ClusterRoleGroupVersionKind.Group,
				Kind:     rbacv1.ClusterRoleGroupVersionKind.Kind,
				Name:     userScopedRoleName,
			},
			Subjects: []k8srbacv1.Subject{
				k8srbacv1.Subject{
					Kind:     "User",
					APIGroup: rbacv1.GroupName,
					Name:     user,
				},
			},
		}); err != nil {
			return err
		}
	}
	return nil
}
