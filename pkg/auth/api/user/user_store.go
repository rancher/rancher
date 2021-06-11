package user

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/slice"
	gaccess "github.com/rancher/rancher/pkg/api/norman/customization/globalnamespaceaccess"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const userByUsernameIndex = "auth.management.cattle.io/user-by-username"

type userStore struct {
	types.Store
	mu                       sync.Mutex
	userIndexer              cache.Indexer
	userManager              user.Manager
	users                    v3.UserInterface
	globalRoleBindingsClient v3.GlobalRoleBindingInterface
	globalRolesClient        v3.GlobalRoleInterface
}

func SetUserStore(schema *types.Schema, mgmt *config.ScaledContext) {
	userInformer := mgmt.Management.Users("").Controller().Informer()
	userIndexers := map[string]cache.IndexFunc{
		userByUsernameIndex: userByUsername,
	}
	userInformer.AddIndexers(userIndexers)
	users := mgmt.Management.Users("")
	grbClient := mgmt.Management.GlobalRoleBindings("")
	grClient := mgmt.Management.GlobalRoles("")

	store := &userStore{
		Store:                    schema.Store,
		mu:                       sync.Mutex{},
		userIndexer:              userInformer.GetIndexer(),
		userManager:              mgmt.UserManager,
		users:                    users,
		globalRoleBindingsClient: grbClient,
		globalRolesClient:        grClient,
	}

	t := &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			// filter system users out of the api
			if princIds, ok := data[client.UserFieldPrincipalIDs].([]interface{}); ok {
				for _, p := range princIds {
					pid, _ := p.(string)
					if strings.HasPrefix(pid, "system://") {
						if opt != nil && opt.Options["ByID"] == "true" {
							return nil, httperror.NewAPIError(httperror.NotFound, "resource not found")
						}
						return nil, nil
					}
				}
			}

			// set "me" field on user
			userID := apiContext.Request.Header.Get("Impersonate-User")
			if userID != "" {
				id, ok := data[types.ResourceFieldID].(string)
				if ok {
					if id == userID {
						data["me"] = "true"
					}
				}
			}

			return data, nil
		},
	}

	schema.Store = t
}

func userByUsername(obj interface{}) ([]string, error) {
	u, ok := obj.(*v3.User)
	if !ok {
		return []string{}, nil
	}

	return []string{u.Username}, nil
}

func hashPassword(data map[string]interface{}) error {
	pass, ok := data[client.UserFieldPassword].(string)
	if !ok {
		return errors.New("password not a string")
	}
	hashed, err := HashPasswordString(pass)
	if err != nil {
		return err
	}
	data[client.UserFieldPassword] = string(hashed)

	return nil
}

func HashPasswordString(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.Wrap(err, "problem encrypting password")
	}
	return string(hash), nil
}

func (s *userStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if err := hashPassword(data); err != nil {
		return nil, err
	}

	created, err := s.create(apiContext, schema, data)
	if err != nil {
		return nil, err
	}

Tries:
	for x := 0; x < 3; x++ {
		if id, ok := created[types.ResourceFieldID].(string); ok {
			time.Sleep(time.Duration((x+1)*100) * time.Millisecond)

			created, err = s.ByID(apiContext, schema, id)
			if err != nil {
				logrus.Warnf("error while getting user: %v", err)
				continue
			}

			var principalIDs []interface{}
			if pids, ok := created[client.UserFieldPrincipalIDs].([]interface{}); ok {
				principalIDs = pids
			}

			for _, pid := range principalIDs {
				if pidString, ok := pid.(string); ok {
					if strings.HasPrefix(pidString, "local://") {
						break Tries
					}
				}
			}

			created[client.UserFieldPrincipalIDs] = append(principalIDs, "local://"+id)
			created, err = s.Update(apiContext, schema, created, id)
			if err != nil {
				if httperror.IsConflict(err) {
					continue
				}

				logrus.Warnf("error while updating user: %v", err)
				break
			}
			break
		}
	}

	delete(created, client.UserFieldPassword)

	return created, nil
}

func (s *userStore) create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	username, ok := data[client.UserFieldUsername].(string)
	if !ok {
		return nil, errors.New("invalid username")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	users, err := s.userIndexer.ByIndex(userByUsernameIndex, username)
	if err != nil {
		return nil, err
	}
	if len(users) > 0 {
		return nil, httperror.NewFieldAPIError(httperror.NotUnique, "username", "Username is already in use.")
	}

	return s.Store.Create(apiContext, schema, data)
}

func (s *userStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	currentUser, err := getUser(apiContext)
	if err != nil {
		return nil, err
	}

	willBeInactive := false
	if val, ok := data[client.UserFieldEnabled].(bool); ok {
		willBeInactive = !val
	}

	if currentUser == id && willBeInactive {
		return nil, httperror.NewAPIError(httperror.InvalidAction, "You cannot deactivate yourself")
	}

	isAdminResource, err := isAdminResource(id, s.users, s.globalRoleBindingsClient, s.globalRolesClient)
	if err != nil {
		return nil, err
	}
	if isAdminResource {
		callerID := apiContext.Request.Header.Get(gaccess.ImpersonateUserHeader)
		isRestrictedAdmin, err := isRestrictedAdmin(callerID, s.users, s.globalRoleBindingsClient)
		if err != nil {
			return nil, err
		}
		if isRestrictedAdmin {
			return nil, httperror.NewAPIError(httperror.InvalidAction, "you cannot edit this user")
		}
	}
	return s.Store.Update(apiContext, schema, data, id)
}

func (s *userStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	currentUser, err := getUser(apiContext)
	if err != nil {
		return nil, err
	}

	if currentUser == id {
		return nil, httperror.NewAPIError(httperror.InvalidAction, "You cannot delete yourself")
	}

	isAdminResource, err := isAdminResource(id, s.users, s.globalRoleBindingsClient, s.globalRolesClient)
	if err != nil {
		return nil, err
	}
	if isAdminResource {
		callerID := apiContext.Request.Header.Get(gaccess.ImpersonateUserHeader)
		isRestrictedAdmin, err := isRestrictedAdmin(callerID, s.users, s.globalRoleBindingsClient)
		if err != nil {
			return nil, err
		}
		if isRestrictedAdmin {
			return nil, httperror.NewAPIError(httperror.InvalidAction, "you cannot delete this user")
		}
	}
	return s.Store.Delete(apiContext, schema, id)
}

func getUser(apiContext *types.APIContext) (string, error) {
	user := apiContext.Request.Header.Get("Impersonate-User")
	if user == "" {
		return "", httperror.NewAPIError(httperror.ServerError, "There was an error authorizing the user")
	}

	return user, nil
}

func isRestrictedAdmin(callerID string, users v3.UserInterface, grbClient v3.GlobalRoleBindingInterface) (bool, error) {

	u, err := users.Get(callerID, v1.GetOptions{})
	if err != nil {
		return false, err
	}
	if u == nil {
		return false, fmt.Errorf("No user found with ID %v", callerID)
	}

	// Get globalRoleBinding for this user
	grbs, err := grbClient.List(v1.ListOptions{})
	if err != nil {
		return false, err
	}
	for _, grb := range grbs.Items {
		if grb.UserName == callerID {
			if grb.GlobalRoleName == rbac.GlobalRestrictedAdmin {
				return true, nil
			}
		}
	}
	return false, nil
}

func isAdminResource(resourceID string, users v3.UserInterface, grbClient v3.GlobalRoleBindingInterface, grClient v3.GlobalRoleInterface) (bool, error) {
	u, err := users.Get(resourceID, v1.GetOptions{})
	if err != nil {
		return false, err
	}
	if u == nil {
		return false, fmt.Errorf("No user found with ID %v", resourceID)
	}
	// Get globalRoleBinding for this user
	grbs, err := grbClient.List(v1.ListOptions{})
	if err != nil {
		return false, err
	}
	for _, grb := range grbs.Items {
		if grb.UserName == resourceID {
			gr, err := grClient.Get(grb.GlobalRoleName, v1.GetOptions{})
			if apierrors.IsNotFound(err) {
				continue
			} else if err != nil {
				return false, err
			}
			for _, rule := range gr.Rules {
				// admin roles have all resources and all verbs allowed
				if slice.ContainsString(rule.Resources, "*") && slice.ContainsString(rule.APIGroups, "*") && slice.ContainsString(rule.Verbs, "*") {
					// caller is global admin
					return true, nil
				}
			}
		}
	}
	return false, nil
}
