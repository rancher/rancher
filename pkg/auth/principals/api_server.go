package principals

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"

	"github.com/rancher/rancher/pkg/auth/providers"
)

type principalAPIServer struct {
	ctx              context.Context
	client           *config.ManagementContext
	principalsClient v3.PrincipalInterface
	tokensClient     v3.TokenInterface
}

func newPrincipalAPIServer(ctx context.Context, mgmtCtx *config.ManagementContext) (*principalAPIServer, error) {
	if mgmtCtx == nil {
		return nil, fmt.Errorf("Failed to build tokenAPIHandler, nil ManagementContext")
	}
	providers.Configure(ctx, mgmtCtx)

	apiServer := &principalAPIServer{
		ctx:              ctx,
		client:           mgmtCtx,
		principalsClient: mgmtCtx.Management.Principals(""),
		tokensClient:     mgmtCtx.Management.Tokens(""),
	}
	return apiServer, nil
}

func (s *principalAPIServer) getPrincipals(tokenKey string) ([]v3.Principal, int, error) {
	principals := make([]v3.Principal, 0)

	token, err := s.getTokenCR(tokenKey)

	if err != nil {
		return principals, 401, err
	}

	//add code to make sure token is valid
	principals = append(principals, token.UserPrincipal)
	principals = append(principals, token.GroupPrincipals...)

	return principals, 0, nil

}

func (s *principalAPIServer) findPrincipals(tokenKey string, name string) ([]v3.Principal, int, error) {
	var principals []v3.Principal
	var status int
	logrus.Debugf("searchPrincipals: search for name: %v", name)

	token, err := s.getTokenCR(tokenKey)
	if err != nil {
		return principals, 401, err
	}
	principals, status, err = providers.SearchPrincipals(name, *token)

	return principals, status, err
}

func (s *principalAPIServer) getTokenCR(tokenID string) (*v3.Token, error) {

	storedToken, err := s.tokensClient.Get(strings.ToLower(tokenID), metav1.GetOptions{})

	if err != nil {
		logrus.Info("Failed to get token resource: %v", err)
		return nil, fmt.Errorf("Failed to retrieve auth token")
	}
	return storedToken, nil

}
