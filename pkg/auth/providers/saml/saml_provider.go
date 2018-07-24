package saml

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	publicclient "github.com/rancher/types/client/management/v3public"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"github.com/sirupsen/logrus"

	"github.com/rancher/types/apis/management.cattle.io/v3public"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const PingName = "ping"
const ADFSName = "adfs"

type Provider struct {
	ctx             context.Context
	authConfigs     v3.AuthConfigInterface
	userMGR         user.Manager
	tokenMGR        *tokens.Manager
	serviceProvider *saml.ServiceProvider
	name            string
	userType        string
	groupType       string
	clientState     samlsp.ClientState
}

var SamlProviders = make(map[string]*Provider)

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager, name string) common.AuthProvider {
	samlp := &Provider{
		ctx:         ctx,
		authConfigs: mgmtCtx.Management.AuthConfigs(""),
		userMGR:     userMGR,
		tokenMGR:    tokenMGR,
		name:        name,
		userType:    name + "_user",
		groupType:   name + "_group",
	}
	SamlProviders[name] = samlp
	return samlp
}

func (s *Provider) GetName() string {
	return s.name
}

func (s *Provider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = s.actionHandler
	schema.Formatter = s.formatter
}

func (s *Provider) TransformToAuthProvider(authConfig map[string]interface{}) map[string]interface{} {
	p := common.TransformToAuthProvider(authConfig)
	switch s.name {
	case PingName:
		p[publicclient.PingProviderFieldRedirectURL] = formSamlRedirectURLFromMap(authConfig, s.name)
	case ADFSName:
		p[publicclient.ADFSProviderFieldRedirectURL] = formSamlRedirectURLFromMap(authConfig, s.name)
	}
	return p
}

func (s *Provider) AuthenticateUser(input interface{}) (v3.Principal, []v3.Principal, string, error) {
	return v3.Principal{}, nil, "", fmt.Errorf("SAML providers do not implement Authenticate User API")
}

func PerformSamlLogin(name string, apiContext *types.APIContext, input interface{}) error {
	//input will contain the FINAL redirect URL
	login, ok := input.(*v3public.SamlLoginInput)
	if !ok {
		return errors.New("unexpected input type")
	}
	finalRedirectURL := login.FinalRedirectURL

	if provider, ok := SamlProviders[name]; ok {
		provider.clientState.SetState(apiContext.Response, apiContext.Request, "Rancher_FinalRedirectURL", finalRedirectURL)
		provider.clientState.SetState(apiContext.Response, apiContext.Request, "Rancher_Action", "login")
		idpRedirectURL, err := provider.HandleSamlLogin(apiContext.Response, apiContext.Request)
		if err != nil {
			return err
		}
		data := map[string]interface{}{
			"idpRedirectUrl": idpRedirectURL,
			"type":           "samlLoginOutput",
		}

		apiContext.WriteResponse(http.StatusOK, data)
		return nil
	}

	return nil
}

func (s *Provider) getSamlConfig() (*v3.SamlConfig, error) {
	authConfigObj, err := s.authConfigs.ObjectClient().UnstructuredClient().Get(s.name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("SAML: failed to retrieve SamlConfig, error: %v", err)
	}

	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("SAML: failed to retrieve SamlConfig, cannot read k8s Unstructured data")
	}
	storedSamlConfigMap := u.UnstructuredContent()

	storedSamlConfig := &v3.SamlConfig{}
	decode(storedSamlConfigMap, storedSamlConfig)

	if enabled, ok := storedSamlConfigMap["enabled"].(bool); ok {
		storedSamlConfig.Enabled = enabled
	}

	metadataMap, ok := storedSamlConfigMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("SAML: failed to retrieve SamlConfig metadata, cannot read k8s Unstructured data")
	}

	typemeta := &metav1.ObjectMeta{}
	mapstructure.Decode(metadataMap, typemeta)
	storedSamlConfig.ObjectMeta = *typemeta

	return storedSamlConfig, nil
}

func (s *Provider) saveSamlConfig(config *v3.SamlConfig) error {
	var configType string

	storedSamlConfig, err := s.getSamlConfig()
	if err != nil {
		return err
	}

	switch s.name {
	case PingName:
		configType = client.PingConfigType
	case ADFSName:
		configType = client.ADFSConfigType
	}

	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = configType
	storedSamlConfig.Annotations = config.Annotations
	config.ObjectMeta = storedSamlConfig.ObjectMeta

	logrus.Debugf("updating samlConfig")
	_, err = s.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil

}

func (s *Provider) toPrincipal(principalType string, princ v3.Principal, token *v3.Token) v3.Principal {
	if principalType == s.userType {
		if token != nil {
			princ.Me = s.isThisUserMe(token.UserPrincipal, princ)
			if princ.Me {
				princ.LoginName = token.UserPrincipal.LoginName
				princ.DisplayName = token.UserPrincipal.DisplayName
			}
		}
	} else {
		if token != nil {
			princ.MemberOf = s.tokenMGR.IsMemberOf(*token, princ)
		}
	}

	return princ
}

func (s *Provider) SearchPrincipals(searchKey, principalType string, token v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal

	if principalType == "" {
		principalType = s.userType
	}

	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: s.userType + "://" + searchKey},
		DisplayName:   searchKey,
		LoginName:     searchKey,
		PrincipalType: principalType,
		Provider:      s.name,
	}

	principals = append(principals, p)

	return principals, nil
}

func (s *Provider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	parts := strings.SplitN(principalID, ":", 2)
	if len(parts) != 2 {
		return v3.Principal{}, errors.Errorf("SAML: invalid id %v", principalID)
	}
	principalType := parts[0]
	externalID := strings.TrimPrefix(parts[1], "//")

	p := v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: principalType + "://" + externalID},
		DisplayName:   externalID,
		LoginName:     externalID,
		PrincipalType: principalType,
		Provider:      s.name,
	}

	p = s.toPrincipal(principalType, p, &token)

	return p, nil
}

func (s *Provider) isThisUserMe(me v3.Principal, other v3.Principal) bool {
	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}

func formSamlRedirectURLFromMap(config map[string]interface{}, name string) string {
	var hostname string
	switch name {
	case PingName:
		hostname, _ = config[client.PingConfigFieldRancherAPIHost].(string)
	case ADFSName:
		hostname, _ = config[client.ADFSConfigFieldRancherAPIHost].(string)
	}

	path := hostname + "/v1-saml/" + name + "/login"
	return path
}

func decode(m interface{}, rawVal interface{}) error {
	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   rawVal,
		TagName:  "json",
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(m)
}
