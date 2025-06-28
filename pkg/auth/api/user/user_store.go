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
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

const (
	userByUsernameIndex = "auth.management.cattle.io/user-by-username"
)

type PasswordCreator interface {
	CreatePassword(user *v3.User, password string) error
}

type userStore struct {
	types.Store
	mu           sync.Mutex
	userIndexer  cache.Indexer
	userManager  user.Manager
	secretLister wranglerv1.SecretCache
	secretClient wranglerv1.SecretClient
	pwdCreator   PasswordCreator
}

func SetUserStore(schema *types.Schema, mgmt *config.ScaledContext) {
	userInformer := mgmt.Management.Users("").Controller().Informer()
	userIndexers := map[string]cache.IndexFunc{
		userByUsernameIndex: userByUsername,
	}
	userInformer.AddIndexers(userIndexers)

	store := &userStore{
		Store:        schema.Store,
		mu:           sync.Mutex{},
		userIndexer:  userInformer.GetIndexer(),
		userManager:  mgmt.UserManager,
		secretClient: mgmt.Wrangler.Core.Secret(),
		secretLister: mgmt.Wrangler.Core.Secret().Cache(),
		pwdCreator:   pbkdf2.New(mgmt.Wrangler.Core.Secret().Cache(), mgmt.Wrangler.Core.Secret()),
	}

	t := &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			// filter system users out of the api
			if princIDs, ok := data[client.UserFieldPrincipalIDs].([]interface{}); ok {
				for _, p := range princIDs {
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

func (s *userStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	username, ok := data[client.UserFieldUsername].(string)
	if !ok {
		return nil, errors.New("invalid username")
	}

	pwd, ok := data[client.UserFieldPassword].(string)
	if !ok {
		return nil, errors.New("invalid password")
	}

	if err := validatePassword(username, "", pwd, settings.PasswordMinLength.GetInt()); err != nil {
		return nil, httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}

	delete(data, client.UserFieldPassword)

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

	userId, ok := created[types.ResourceFieldID].(string)
	if !ok {
		return nil, errors.New("failed to get userId")
	}
	userUUID, ok := created[client.UserFieldUUID].(string)
	if !ok {
		return nil, errors.New("failed to get userId")
	}

	err = s.pwdCreator.CreatePassword(&v3.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: userId,
			UID:  apitypes.UID(userUUID),
		},
	}, pwd)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret password: %w", err)
	}

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

	return s.Store.Delete(apiContext, schema, id)
}

func getUser(apiContext *types.APIContext) (string, error) {
	user := apiContext.Request.Header.Get("Impersonate-User")
	if user == "" {
		return "", httperror.NewAPIError(httperror.ServerError, "There was an error authorizing the user")
	}

	return user, nil
}
