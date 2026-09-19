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

	// principalIDRetries and defaultPrincipalIDRetryDelay bound the wait for the local principal
	// ID to be populated on a newly created user. The delay grows with every attempt.
	principalIDRetries           = 5
	defaultPrincipalIDRetryDelay = 100 * time.Millisecond
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
	// principalIDRetryDelay is the base delay between attempts to read the principal IDs
	// of a newly created user. Zero means defaultPrincipalIDRetryDelay.
	principalIDRetryDelay time.Duration
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

	return s.withPrincipalIDs(apiContext, schema, created, userId), nil
}

// withPrincipalIDs waits for the local principal ID to be populated on the created user and
// returns the refreshed user. A user is created without principal IDs and they are filled in
// asynchronously, but callers need them in the response to reference the user in role template
// bindings. The user as created is returned if the principal IDs don't show up in time.
func (s *userStore) withPrincipalIDs(apiContext *types.APIContext, schema *types.Schema, created map[string]interface{}, id string) map[string]interface{} {
	delay := s.principalIDRetryDelay
	if delay == 0 {
		delay = defaultPrincipalIDRetryDelay
	}

	for i := range principalIDRetries {
		time.Sleep(time.Duration(i+1) * delay)

		refreshed, err := s.ByID(apiContext, schema, id)
		if err != nil {
			logrus.Warnf("Failed to get user %s to read its principal IDs: %v", id, err)
			continue
		}

		if principalIDs, ok := refreshed[client.UserFieldPrincipalIDs].([]interface{}); ok && len(principalIDs) > 0 {
			return refreshed
		}
	}

	logrus.Warnf("Principal IDs of user %s were not populated in time, returning the user without them", id)

	return created
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

	delete(data, client.UserFieldPrincipalIDs)
	delete(data, client.UserFieldUsername)
	delete(data, client.UserFieldName)

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
