package publicapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"encoding/base32"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/apis/management.cattle.io/v3public"
	"github.com/rancher/types/apis/management.cattle.io/v3public/schema"
	"github.com/rancher/types/client/management/v3public"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const (
	CookieName           = "R_SESS"
	userByPrincipalIndex = "auth.management.cattle.io/userByPrincipal"
)

func newLoginHandler(mgmt *config.ManagementContext) *loginHandler {
	userInformer := mgmt.Management.Users("").Controller().Informer()
	userIndexers := map[string]cache.IndexFunc{
		userByPrincipalIndex: userByPrincipal,
	}
	userInformer.AddIndexers(userIndexers)

	return &loginHandler{
		userIndexer: userInformer.GetIndexer(),
		mgmt:        mgmt,
	}
}

type loginHandler struct {
	mgmt        *config.ManagementContext
	userIndexer cache.Indexer
}

func (h *loginHandler) login(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName != "login" {
		return httperror.NewAPIError(httperror.ActionNotAvailable, "")
	}

	w := request.Response

	token, responseType, status, err := h.createLoginToken(request)
	if err != nil {
		logrus.Errorf("Login failed with error: %v", err)
		if status == 0 {
			status = http.StatusInternalServerError
		}
		return httperror.NewAPIErrorLong(status, util.GetHTTPErrorCode(status), fmt.Sprintf("%v", err))
	}

	if responseType == "cookie" {
		tokenCookie := &http.Cookie{
			Name:     CookieName,
			Value:    token.ObjectMeta.Name + ":" + token.Token,
			Secure:   true,
			Path:     "/",
			HttpOnly: true,
		}
		http.SetCookie(w, tokenCookie)
	} else {
		tokenData, err := tokens.ConvertTokenResource(request.Schemas.Schema(&schema.PublicVersion, client.TokenType), token)
		if err != nil {
			return err
		}
		tokenData["token"] = token.ObjectMeta.Name + ":" + token.Token
		request.WriteResponse(http.StatusCreated, tokenData)
	}

	return nil
}

func (h *loginHandler) createLoginToken(request *types.APIContext) (v3.Token, string, int, error) {
	logrus.Debugf("Create Token Invoked")

	bytes, err := ioutil.ReadAll(request.Request.Body)
	if err != nil {
		logrus.Errorf("login failed with error: %v", err)
		return v3.Token{}, "", httperror.InvalidBodyContent.Status, httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	generic := &v3public.GenericLogin{}
	err = json.Unmarshal(bytes, generic)
	if err != nil {
		logrus.Errorf("unmarshal failed with error: %v", err)
		return v3.Token{}, "", httperror.InvalidBodyContent.Status, httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}
	responseType := generic.ResponseType
	description := generic.Description
	ttl := generic.TTLMillis

	var input interface{}
	var providerName string
	switch request.Type {
	case client.GithubProviderType:
		gInput := &v3public.GithubLogin{}
		input = gInput
		providerName = "github"
	case client.LocalProviderType:
		lInput := &v3public.LocalLogin{}
		input = lInput
		providerName = "local"
	default:
		return v3.Token{}, "", httperror.ServerError.Status, httperror.NewAPIError(httperror.ServerError, "Unknown login type")
	}

	err = json.Unmarshal(bytes, input)
	if err != nil {
		logrus.Errorf("unmarshal failed with error: %v", err)
		return v3.Token{}, "", httperror.InvalidBodyContent.Status, httperror.NewAPIError(httperror.InvalidBodyContent, "")
	}

	// Authenticate User
	userPrincipal, groupPrincipals, providerInfo, status, err := providers.AuthenticateUser(input, providerName)
	if status != 0 || err != nil {
		return v3.Token{}, "", status, err
	}

	logrus.Debug("User Authenticated")

	user, err := h.ensureUser(userPrincipal)
	if err != nil {
		return v3.Token{}, "", 500, err
	}

	k8sToken := tokens.GenerateNewLoginToken(user.Name, userPrincipal, groupPrincipals, providerInfo, ttl, description)
	rToken, err := tokens.CreateTokenCR(&k8sToken)
	return rToken, responseType, 0, err
}

func (h *loginHandler) ensureUser(principal v3.Principal) (*v3.User, error) {
	// First check the local cache
	u, err := h.checkCache(principal)
	if err != nil {
		return nil, err
	}
	if u != nil {
		return u, nil
	}

	// Not in cache, query API by label
	u, labelSet, err := h.checkLabels(principal)
	if err != nil {
		return nil, err
	}
	if u != nil {
		return u, nil
	}

	// Doesn't exist, create user
	user := &v3.User{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "user-",
			Labels:       labelSet,
		},
		DisplayName:  principal.DisplayName,
		PrincipalIDs: []string{principal.Name},
	}

	created, err := h.mgmt.Management.Users("").Create(user)
	if err != nil {
		return nil, err
	}

	_, err = h.mgmt.Management.GlobalRoleBindings("").Create(&v3.GlobalRoleBinding{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "globalrolebinding-",
		},
		UserName:       created.Name,
		GlobalRoleName: "user",
	})
	if err != nil {
		return nil, err
	}

	localPrincipal := "local://" + created.Name
	if !slice.ContainsString(created.PrincipalIDs, localPrincipal) {
		created.PrincipalIDs = append(created.PrincipalIDs, localPrincipal)
		return h.mgmt.Management.Users("").Update(created)
	}

	return created, nil
}

func (h *loginHandler) checkCache(userPrincipal v3.Principal) (*v3.User, error) {
	users, err := h.userIndexer.ByIndex(userByPrincipalIndex, userPrincipal.Name)
	if err != nil {
		return nil, err
	}
	if len(users) > 1 {
		return nil, errors.Errorf("can't find unique user for principal %v", userPrincipal.Name)
	}
	if len(users) == 1 {
		u := users[0].(*v3.User)
		return u.DeepCopy(), nil
	}
	return nil, nil
}

func (h *loginHandler) checkLabels(principal v3.Principal) (*v3.User, labels.Set, error) {
	encodedPrincipalID := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(principal.Name))
	if len(encodedPrincipalID) > 63 {
		encodedPrincipalID = encodedPrincipalID[:63]
	}
	labelKey := fmt.Sprintf("authn.managment.cattle.io/%v-principalId", principal.Provider)
	set := labels.Set(map[string]string{labelKey: encodedPrincipalID})
	users, err := h.mgmt.Management.Users("").List(v1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return nil, nil, err
	}

	if len(users.Items) == 0 {
		return nil, set, nil
	}

	var match *v3.User
	for _, u := range users.Items {
		if slice.ContainsString(u.PrincipalIDs, principal.Name) {
			if match != nil {
				// error out on duplicates
				return nil, nil, errors.Errorf("can't find unique user for principal %v", principal.Name)
			}
			match = &u
		}
	}

	return match, set, nil
}

func userByPrincipal(obj interface{}) ([]string, error) {
	u, ok := obj.(*v3.User)
	if !ok {
		return []string{}, nil
	}

	return u.PrincipalIDs, nil
}
