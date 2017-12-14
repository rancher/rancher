package tokens

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
)

const (
	defaultSecret          = "secret"
	defaultTokenTTL        = "57600000"
	defaultRefreshTokenTTL = "7200000"
)

type tokenAPIServer struct {
	ctx          context.Context
	client       *config.ManagementContext
	tokensClient v3.TokenInterface
}

func newTokenAPIServer(ctx context.Context, mgmtCtx *config.ManagementContext) (*tokenAPIServer, error) {
	if mgmtCtx == nil {
		return nil, fmt.Errorf("Failed to build tokenAPIHandler, nil ManagementContext")
	}
	apiServer := &tokenAPIServer{
		ctx:          ctx,
		client:       mgmtCtx,
		tokensClient: mgmtCtx.Management.Tokens(""),
	}
	return apiServer, nil
}

//CreateLoginToken will authenticate with provider and create a jwt token
func (s *tokenAPIServer) createLoginToken(jsonInput v3.LoginInput) (v3.Token, int, error) {

	log.Info("Create Token Invoked %v", jsonInput)
	authenticated := true

	/* Authenticate User
		if provider != nil {
		token, status, err := provider.GenerateToken(json)
		if err != nil {
			return model.TokenOutput{}, status, err
		}
	}*/

	if authenticated {

		if s.client != nil {

			key, err := generateKey()
			if err != nil {
				log.Info("Failed to generate token key: %v", err)
				return v3.Token{}, 0, fmt.Errorf("Failed to generate token key")
			}

			//check that there is no token with this key.
			payload := make(map[string]interface{})
			tokenValue, err := createTokenWithPayload(payload, defaultSecret)
			if err != nil {
				log.Info("Failed to generate token value: %v", err)
				return v3.Token{}, 0, fmt.Errorf("Failed to generate token value")
			}

			ttl := jsonInput.TTLMillis
			refreshTTL := jsonInput.IdentityRefreshTTLMillis
			if ttl == "" {
				ttl = defaultTokenTTL               //16 hrs
				refreshTTL = defaultRefreshTokenTTL //2 hrs
			}

			k8sToken := &v3.Token{
				TokenID:                  key,
				TokenValue:               tokenValue, //signed jwt containing user details
				IsDerived:                false,
				TTLMillis:                ttl,
				IdentityRefreshTTLMillis: refreshTTL,
				User:         "dummy",
				ExternalID:   "github_12346",
				AuthProvider: "github",
			}
			rToken, err := s.createK8sTokenCR(k8sToken)
			return rToken, 0, err
		}
		log.Info("Client nil %v", s.client)
		return v3.Token{}, 500, fmt.Errorf("No k8s Client configured")
	}

	return v3.Token{}, 0, fmt.Errorf("No auth provider configured")
}

//CreateDerivedToken will create a jwt token for the authenticated user
func (s *tokenAPIServer) createDerivedToken(jsonInput v3.Token, tokenID string) (v3.Token, int, error) {

	log.Info("Create Derived Token Invoked")

	token, err := s.getK8sTokenCR(tokenID)

	if err != nil {
		return v3.Token{}, 401, err
	}

	if s.client != nil {
		key, err := generateKey()
		if err != nil {
			log.Info("Failed to generate token key: %v", err)
			return v3.Token{}, 0, fmt.Errorf("Failed to generate token key")
		}

		ttl := jsonInput.TTLMillis
		refreshTTL := jsonInput.IdentityRefreshTTLMillis
		if ttl == "" {
			ttl = defaultTokenTTL               //16 hrs
			refreshTTL = defaultRefreshTokenTTL //2 hrs
		}

		k8sToken := &v3.Token{
			TokenID:                  key,
			TokenValue:               token.TokenValue, //signed jwt containing user details
			IsDerived:                true,
			TTLMillis:                ttl,
			IdentityRefreshTTLMillis: refreshTTL,
			User:         token.User,
			ExternalID:   token.ExternalID,
			AuthProvider: token.AuthProvider,
		}
		rToken, err := s.createK8sTokenCR(k8sToken)
		return rToken, 0, err

	}
	log.Info("Client nil %v", s.client)
	return v3.Token{}, 500, fmt.Errorf("No k8s Client configured")

}

func (s *tokenAPIServer) createK8sTokenCR(k8sToken *v3.Token) (v3.Token, error) {
	if s.client != nil {

		labels := make(map[string]string)
		labels["io.cattle.token.field.externalID"] = k8sToken.ExternalID

		k8sToken.APIVersion = "management.cattle.io/v3"
		k8sToken.Kind = "Token"
		k8sToken.ObjectMeta = metav1.ObjectMeta{
			Name:   strings.ToLower(k8sToken.TokenID),
			Labels: labels,
		}
		createdToken, err := s.tokensClient.Create(k8sToken)

		if err != nil {
			log.Info("Failed to create token resource: %v", err)
			return v3.Token{}, err
		}
		log.Info("Created Token %v", createdToken)
		return *createdToken, nil
	}

	return v3.Token{}, fmt.Errorf("No k8s Client configured")
}

func (s *tokenAPIServer) getK8sTokenCR(tokenID string) (*v3.Token, error) {
	if s.client != nil {
		storedToken, err := s.tokensClient.Get(strings.ToLower(tokenID), metav1.GetOptions{})

		if err != nil {
			log.Info("Failed to get token resource: %v", err)
			return nil, fmt.Errorf("Failed to retrieve auth token")
		}

		log.Info("storedToken token resource: %v", storedToken)

		return storedToken, nil
	}
	return nil, fmt.Errorf("No k8s Client configured")
}

//GetTokens will list all tokens of the authenticated user - login and derived
func (s *tokenAPIServer) getTokens(tokenID string) ([]v3.Token, int, error) {
	log.Info("GET Token Invoked")
	var tokens []v3.Token

	if s.client != nil {
		storedToken, err := s.tokensClient.Get(strings.ToLower(tokenID), metav1.GetOptions{})

		if err != nil {
			log.Info("Failed to get token resource: %v", err)
			return tokens, 401, fmt.Errorf("Failed to retrieve auth token")
		}
		log.Info("storedToken token resource: %v", storedToken)
		externalID := storedToken.ExternalID
		set := labels.Set(map[string]string{"io.cattle.token.field.externalID": externalID})
		tokenList, err := s.tokensClient.List(metav1.ListOptions{LabelSelector: set.AsSelector().String()})
		if err != nil {
			return tokens, 0, fmt.Errorf("Error getting tokens for user: %v selector: %v  err: %v", externalID, set.AsSelector().String(), err)
		}

		for _, t := range tokenList.Items {
			log.Info("List token resource: %v", t)
			tokens = append(tokens, t)
		}
		return tokens, 0, nil

	}
	log.Info("Client nil %v", s.client)
	return tokens, 500, fmt.Errorf("No k8s Client configured")
}

func (s *tokenAPIServer) deleteToken(tokenKey string) (int, error) {
	log.Info("DELETE Token Invoked")

	if s.client != nil {
		err := s.tokensClient.Delete(strings.ToLower(tokenKey), &metav1.DeleteOptions{})

		if err != nil {
			return 500, fmt.Errorf("Failed to delete token")
		}
		log.Info("Deleted Token")
		return 0, nil

	}
	log.Info("Client nil %v", s.client)
	return 500, fmt.Errorf("No k8s Client configured")
}

func (s *tokenAPIServer) getIdentities(tokenKey string) ([]v3.Identity, int, error) {
	var identities []v3.Identity

	/*token, status, err := GetToken(tokenKey)

	if err != nil {
		return identities, 401, err
	} else {
		identities = append(identities, token.UserIdentity)
		identities = append(identities, token.GroupIdentities...)

		return identities, status, nil
	}*/

	identities = append(identities, getUserIdentity())
	identities = append(identities, getGroupIdentities()...)

	return identities, 0, nil

}

func getUserIdentity() v3.Identity {

	identity := v3.Identity{
		LoginName:      "dummy",
		DisplayName:    "Dummy User",
		ProfilePicture: "",
		ProfileURL:     "",
		Kind:           "user",
		Me:             true,
		MemberOf:       false,
	}
	identity.ObjectMeta = metav1.ObjectMeta{
		Name: "ldap_cn=dummy,dc=tad,dc=rancher,dc=io",
	}

	return identity
}

func getGroupIdentities() []v3.Identity {

	var identities []v3.Identity

	identity1 := v3.Identity{
		DisplayName:    "Admin group",
		LoginName:      "Administrators",
		ProfilePicture: "",
		ProfileURL:     "",
		Kind:           "group",
		Me:             false,
		MemberOf:       true,
	}
	identity1.ObjectMeta = metav1.ObjectMeta{
		Name: "ldap_cn=group1,dc=tad,dc=rancher,dc=io",
	}

	identity2 := v3.Identity{
		DisplayName:    "Dev group",
		LoginName:      "Developers",
		ProfilePicture: "",
		ProfileURL:     "",
		Kind:           "group",
		Me:             false,
		MemberOf:       true,
	}
	identity2.ObjectMeta = metav1.ObjectMeta{
		Name: "ldap_cn=group2,dc=tad,dc=rancher,dc=io",
	}

	identities = append(identities, identity1)
	identities = append(identities, identity2)

	return identities
}
