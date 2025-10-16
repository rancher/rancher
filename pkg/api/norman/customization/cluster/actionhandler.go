package cluster

import (
	"fmt"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/rancher/rancher/pkg/auth/tokens"
	mgmtclient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/user"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type userManager interface {
	GetUser(r *http.Request) string
}

type tokenManager interface {
	EnsureToken(input user.TokenInput) (string, runtime.Object, error)
	EnsureClusterToken(clusterName string, input user.TokenInput) (string, runtime.Object, error)
}

// ActionHandler used for performing various cluster actions.
type ActionHandler struct {
	NodeLister     v3.NodeLister
	UserMgr        userManager
	TokenMgr       tokenManager
	ClusterManager *clustermanager.Manager
	AuthToken      requests.AuthTokenGetter
}

// ClusterActionHandler runs the handler for the provided cluster action in the given context.
func (a ActionHandler) ClusterActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case v32.ClusterActionGenerateKubeconfig:
		return a.GenerateKubeconfigActionHandler(actionName, action, apiContext)
	case v32.ClusterActionImportYaml:
		return a.ImportYamlHandler(actionName, action, apiContext)
	}
	return httperror.NewAPIError(httperror.NotFound, "not found")
}

// ensureClusterToken will create a new kubeconfig token for the user in the provided context with the default TTL.
func (a ActionHandler) ensureClusterToken(clusterID string, apiContext *types.APIContext) (string, error) {
	input, err := a.createTokenInput(apiContext)
	if err != nil {
		return "", err
	}

	tokenKey, _, err := a.TokenMgr.EnsureClusterToken(clusterID, input)
	if err != nil {
		return "", err
	}

	return tokenKey, nil
}

// ensureToken will create a new kubeconfig token for the user in the provided context with the default TTL.
func (a ActionHandler) ensureToken(apiContext *types.APIContext) (string, error) {
	input, err := a.createTokenInput(apiContext)
	if err != nil {
		return "", err
	}

	tokenKey, _, err := a.TokenMgr.EnsureToken(input)
	if err != nil {
		return "", err
	}

	return tokenKey, nil
}

// createTokenInput will create the input for a new kubeconfig token with the default TTL.
func (a ActionHandler) createTokenInput(apiContext *types.APIContext) (user.TokenInput, error) {
	userName := a.UserMgr.GetUser(apiContext.Request)
	tokenNamePrefix := fmt.Sprintf("kubeconfig-%s", userName)

	authToken, err := a.AuthToken.TokenFromRequest(apiContext.Request)
	if err != nil {
		return user.TokenInput{}, err
	}

	defaultTokenTTL, err := tokens.GetKubeconfigDefaultTokenTTLInMilliSeconds()
	if err != nil {
		return user.TokenInput{}, fmt.Errorf("failed to get default token TTL: %w", err)
	}

	return user.TokenInput{
		TokenName:     tokenNamePrefix,
		Description:   "Kubeconfig token",
		Kind:          "kubeconfig",
		UserName:      userName,
		AuthProvider:  authToken.GetAuthProvider(),
		TTL:           defaultTokenTTL,
		Randomize:     true,
		UserPrincipal: authToken.GetUserPrincipal(),
	}, nil
}

func (a ActionHandler) generateKubeConfig(apiContext *types.APIContext, cluster *mgmtclient.Cluster) (*clientcmdapi.Config, error) {
	token, err := a.ensureToken(apiContext)
	if err != nil {
		return nil, err
	}

	return a.ClusterManager.KubeConfig(cluster.ID, token), nil
}
