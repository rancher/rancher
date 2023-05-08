package googleoauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/tokens"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	Name                 = "googleoauth"
	userType             = "user"
	groupType            = "group"
	domainPublicViewType = "domain_public"
)

var scopes = []string{"openid", "profile", "email", admin.AdminDirectoryUserReadonlyScope, admin.AdminDirectoryGroupReadonlyScope}

type googleOauthProvider struct {
	authConfigs  v3.AuthConfigInterface
	secrets      corev1.SecretInterface
	goauthClient *GClient
	userMGR      user.Manager
	tokenMGR     *tokens.Manager
	ctx          context.Context
}

func Configure(ctx context.Context, mgmtCtx *config.ScaledContext, userMGR user.Manager, tokenMGR *tokens.Manager) common.AuthProvider {
	gClient := GClient{
		httpClient: &http.Client{
			Timeout: time.Second * 30,
		},
	}
	return &googleOauthProvider{
		ctx:          ctx,
		authConfigs:  mgmtCtx.Management.AuthConfigs(""),
		secrets:      mgmtCtx.Core.Secrets(""),
		goauthClient: &gClient,
		userMGR:      userMGR,
		tokenMGR:     tokenMGR,
	}
}

func (g *googleOauthProvider) AuthenticateUser(ctx context.Context, input interface{}) (v3.Principal, []v3.Principal, string, error) {
	login, ok := input.(*v32.GoogleOauthLogin)
	if !ok {
		return v3.Principal{}, nil, "", fmt.Errorf("unexpected input type")
	}
	return g.loginUser(ctx, login, nil, false)
}

// loginUser takes as input the code; gets access_token and refresh_token in exhange; uses access_token to get user info
// and groups (if allowed); and returns the user and group principals and oauth token
func (g *googleOauthProvider) loginUser(c context.Context, googleOAuthCredential *v32.GoogleOauthLogin, config *v32.GoogleOauthConfig, testAndEnableAction bool) (v3.Principal, []v3.Principal, string, error) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal
	var err error

	if config == nil {
		config, err = g.getGoogleOAuthConfigCR()
		if err != nil {
			return userPrincipal, groupPrincipals, "", err
		}
	}

	logrus.Debugf("[Google OAuth] loginuser: Using code to get oauth token")
	securityCode := googleOAuthCredential.Code
	oauth2Config, err := google.ConfigFromJSON([]byte(config.OauthCredential), scopes...)
	if err != nil {
		return userPrincipal, groupPrincipals, "", err
	}
	// Exchange the code for oauthToken
	gOAuthToken, err := oauth2Config.Exchange(c, securityCode)
	if err != nil {
		return userPrincipal, groupPrincipals, "", err
	}
	logrus.Debugf("[Google OAuth] loginuser: Exchanged code for oauth token")

	// init the admin directory service
	adminSvc, err := g.getDirectoryService(c, config.AdminEmail, []byte(config.ServiceAccountCredential), oauth2Config.TokenSource(c, gOAuthToken))
	if err != nil {
		return userPrincipal, groupPrincipals, "", err
	}
	userPrincipal, groupPrincipals, err = g.getUserInfoAndGroups(adminSvc, gOAuthToken, config, testAndEnableAction)
	if err != nil {
		return userPrincipal, groupPrincipals, "", err
	}

	logrus.Debugf("[Google OAuth] loginuser: Checking user's access to Rancher")
	allowed, err := g.userMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipal.Name, groupPrincipals)
	if err != nil {
		return userPrincipal, groupPrincipals, "", err
	}
	if !allowed {
		return userPrincipal, groupPrincipals, "", httperror.NewAPIError(httperror.Unauthorized, "unauthorized")
	}

	// save entire oauthToken because it contains refresh_token and token expiry time
	// oauth2.TokenSource used in getDirectoryService uses all these fields to auto renew the access token (Ref: https://github.com/golang/oauth2/blob/aaccbc9213b0974828f81aaac109d194880e3014/oauth2.go#L235)
	oauthToken, err := json.Marshal(gOAuthToken)
	if err != nil {
		return userPrincipal, groupPrincipals, "", err
	}

	logrus.Debugf("[Google OAuth] loginuser: Returning principals and marshaled oauth token")
	return userPrincipal, groupPrincipals, string(oauthToken), nil
}

func (g *googleOauthProvider) SearchPrincipals(searchKey, principalType string, token v3.Token) ([]v3.Principal, error) {
	var principals []v3.Principal
	var err error

	config, err := g.getGoogleOAuthConfigCR()
	if err != nil {
		return principals, err
	}

	storedOauthToken, err := g.tokenMGR.GetSecret(token.UserID, token.AuthProvider, []*v3.Token{&token})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	logrus.Debugf("[Google OAuth] SearchPrincipals: Retrieved stored oauth token")
	adminSvc, err := g.getdirectoryServiceFromStoredToken(storedOauthToken, config)
	if err != nil {
		return principals, err
	}

	logrus.Debugf("[Google OAuth] SearchPrincipals: Initialized dir svc with stored oauth token")
	accounts, err := g.searchPrincipals(adminSvc, searchKey, principalType, config)
	if err != nil {
		return principals, err
	}
	for _, acc := range accounts {
		principals = append(principals, g.toPrincipal(acc.Type, acc, &token))
	}
	logrus.Debugf("[Google OAuth] SearchPrincipals: Returning principals")
	return principals, nil
}

func (g *googleOauthProvider) GetPrincipal(principalID string, token v3.Token) (v3.Principal, error) {
	var principal v3.Principal
	config, err := g.getGoogleOAuthConfigCR()
	if err != nil {
		return principal, err
	}
	storedOauthToken, err := g.tokenMGR.GetSecret(token.UserID, token.AuthProvider, []*v3.Token{&token})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return principal, err
		}
	}
	logrus.Debugf("[Google OAuth] GetPrincipal: Retrieved stored oauth token")
	adminSvc, err := g.getdirectoryServiceFromStoredToken(storedOauthToken, config)
	if err != nil {
		return principal, err
	}

	logrus.Debugf("[Google OAuth] GetPrincipal: Initialized dir svc with stored oauth token")
	externalID, principalType, err := getUIDFromPrincipalID(principalID)
	if err != nil {
		return principal, err
	}
	logrus.Debugf("[Google OAuth] GetPrincipal: Parsed principalID")
	switch principalType {
	case userType:
		user, err := adminSvc.Users.Get(externalID).Do()
		if err != nil {
			if config.ServiceAccountCredential == "" {
				// used client creds, try get again with viewType=domain_public
				if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == http.StatusForbidden {
					user, err = adminSvc.Users.Get(externalID).ViewType(domainPublicViewType).Do()
					if err != nil {
						return principal, err
					}
				} else {
					return principal, err
				}
			} else {
				return principal, err
			}
		}
		acc := Account{
			SubjectUniqueID: user.Id,
			Email:           user.PrimaryEmail,
			PictureURL:      user.ThumbnailPhotoUrl,
			Type:            userType,
		}
		if user.Name != nil {
			acc.Name = user.Name.FullName
			acc.GivenName = user.Name.GivenName
			acc.FamilyName = user.Name.FamilyName
		}
		return g.toPrincipal(userType, acc, &token), nil
	case groupType:
		group, err := adminSvc.Groups.Get(externalID).Do()
		if err != nil {
			if config.ServiceAccountCredential == "" {
				// used client creds, getting group for non-admin might fail with forbidden, if that's the case don't throw
				// error
				if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == http.StatusForbidden {
					return principal, nil
				}
			}
			return principal, err
		}
		return g.toPrincipal(groupType, Account{SubjectUniqueID: group.Id, Email: group.Email, Name: group.Name}, &token), nil
	default:
		return principal, fmt.Errorf("cannot get the google account due to invalid externalIDType %v", principalType)
	}
}

func (g *googleOauthProvider) GetName() string {
	return Name
}

func (g *googleOauthProvider) CustomizeSchema(schema *types.Schema) {
	schema.ActionHandler = g.actionHandler
	schema.Formatter = g.formatter
}

func (g *googleOauthProvider) TransformToAuthProvider(authConfig map[string]interface{}) (map[string]interface{}, error) {
	p := common.TransformToAuthProvider(authConfig)
	val, err := g.formGoogleOAuthRedirectURLFromMap(authConfig)
	if err != nil {
		return nil, err
	}
	p[publicclient.GoogleOAuthProviderFieldRedirectURL] = val
	return p, nil
}

func (g *googleOauthProvider) RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error) {
	var principals []v3.Principal
	config, err := g.getGoogleOAuthConfigCR()
	if err != nil {
		return principals, err
	}
	adminSvc, err := g.getdirectoryServiceFromStoredToken(secret, config)
	if err != nil {
		return principals, err
	}
	logrus.Debugf("[Google OAuth] RefetchGroupPrincipals: Initialized dir svc with stored oauth token")
	externalID, _, err := getUIDFromPrincipalID(principalID)
	if err != nil {
		return principals, err
	}
	logrus.Debugf("[Google OAuth] GetPrincipal: Parsed principalID")
	groupPrincipals, err := g.getGroupsUserBelongsTo(adminSvc, externalID, config.Hostname, config)
	if err != nil {
		return principals, err
	}
	if !config.NestedGroupMembershipEnabled {
		return groupPrincipals, nil
	}
	return g.fetchParentGroups(config, groupPrincipals, adminSvc, config.Hostname)
}

func (g *googleOauthProvider) CanAccessWithGroupProviders(userPrincipalID string, groupPrincipals []v3.Principal) (bool, error) {
	config, err := g.getGoogleOAuthConfigCR()
	if err != nil {
		logrus.Errorf("Error fetching google OAuth config: %v", err)
		return false, err
	}
	allowed, err := g.userMGR.CheckAccess(config.AccessMode, config.AllowedPrincipalIDs, userPrincipalID, groupPrincipals)
	if err != nil {
		return false, err
	}
	return allowed, nil
}

func (g *googleOauthProvider) getGoogleOAuthConfigCR() (*v32.GoogleOauthConfig, error) {
	authConfigObj, err := g.authConfigs.ObjectClient().UnstructuredClient().Get(Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve GoogleOAuthConfig, error: %v", err)
	}
	u, ok := authConfigObj.(runtime.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to retrieve GoogleOAuthConfig, cannot read k8s Unstructured data")
	}
	storedGoogleOAuthConfigMap := u.UnstructuredContent()

	storedGoogleOAuthConfig := &v32.GoogleOauthConfig{}
	err = common.Decode(storedGoogleOAuthConfigMap, storedGoogleOAuthConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode Google Oauth Config: %w", err)
	}

	if storedGoogleOAuthConfig.OauthCredential != "" {
		value, err := common.ReadFromSecret(g.secrets, storedGoogleOAuthConfig.OauthCredential, strings.ToLower(client.GoogleOauthConfigFieldOauthCredential))
		if err != nil {
			return nil, err
		}
		storedGoogleOAuthConfig.OauthCredential = value
	}

	if storedGoogleOAuthConfig.ServiceAccountCredential != "" {
		value, err := common.ReadFromSecret(g.secrets, storedGoogleOAuthConfig.ServiceAccountCredential, strings.ToLower(client.GoogleOauthConfigFieldServiceAccountCredential))
		if err != nil {
			return nil, err
		}
		storedGoogleOAuthConfig.ServiceAccountCredential = value
	}
	return storedGoogleOAuthConfig, nil
}

func (g *googleOauthProvider) saveGoogleOAuthConfigCR(config *v32.GoogleOauthConfig) error {
	storedGoogleOAuthConfig, err := g.getGoogleOAuthConfigCR()
	if err != nil {
		return err
	}
	config.APIVersion = "management.cattle.io/v3"
	config.Kind = v3.AuthConfigGroupVersionKind.Kind
	config.Type = client.GoogleOauthConfigType
	config.ObjectMeta = storedGoogleOAuthConfig.ObjectMeta

	secretInfo := convert.ToString(config.OauthCredential)
	field := strings.ToLower(client.GoogleOauthConfigFieldOauthCredential)
	if err := common.CreateOrUpdateSecrets(g.secrets, secretInfo, field, strings.ToLower(config.Type)); err != nil {
		return err
	}
	config.OauthCredential = common.GetFullSecretName(config.Type, field)

	if config.ServiceAccountCredential != "" {
		secretInfo = convert.ToString(config.ServiceAccountCredential)
		field = strings.ToLower(client.GoogleOauthConfigFieldServiceAccountCredential)
		if err := common.CreateOrUpdateSecrets(g.secrets, secretInfo, field, strings.ToLower(config.Type)); err != nil {
			return err
		}
		config.ServiceAccountCredential = common.GetFullSecretName(config.Type, field)
	}

	_, err = g.authConfigs.ObjectClient().Update(config.ObjectMeta.Name, config)
	if err != nil {
		return err
	}
	return nil
}

func (g *googleOauthProvider) toPrincipal(principalType string, acct Account, token *v3.Token) v3.Principal {
	displayName := acct.Name
	if displayName == "" {
		displayName = acct.Email
	}

	princ := v3.Principal{
		ObjectMeta:     metav1.ObjectMeta{Name: Name + "_" + principalType + "://" + acct.SubjectUniqueID},
		DisplayName:    displayName,
		LoginName:      acct.Email,
		Provider:       Name,
		Me:             false,
		ProfilePicture: acct.PictureURL,
	}

	if principalType == userType {
		princ.PrincipalType = "user"
		if token != nil {
			princ.Me = g.isThisUserMe(token.UserPrincipal, princ)
		}
	} else {
		princ.PrincipalType = "group"
		if token != nil {
			princ.MemberOf = g.tokenMGR.IsMemberOf(*token, princ)
		}
	}
	return princ
}

func (g *googleOauthProvider) isThisUserMe(me v3.Principal, other v3.Principal) bool {
	if me.ObjectMeta.Name == other.ObjectMeta.Name && me.LoginName == other.LoginName && me.PrincipalType == other.PrincipalType {
		return true
	}
	return false
}

func (g *googleOauthProvider) getDirectoryService(ctx context.Context, userEmail string, jsonCredentials []byte, accessTokenSource oauth2.TokenSource) (*admin.Service, error) {
	if userEmail != "" && len(jsonCredentials) > 0 {
		// oauth golang library performs all these steps: https://developers.google.com/identity/protocols/OAuth2ServiceAccount#jwt-auth
		// using JWTConfigFromJSON method
		config, err := google.JWTConfigFromJSON(jsonCredentials, admin.AdminDirectoryUserReadonlyScope, admin.AdminDirectoryGroupReadonlyScope)
		if err != nil {
			logrus.Errorf("[Google OAuth] error unmarshaling service account creds: %v", err)
			return nil, fmt.Errorf("invalid Service Account Credentials provided")
		}
		config.Subject = userEmail
		srv, err := admin.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx)))
		if err != nil {
			logrus.Errorf("[Google OAuth] error generating tokenSource for service account creds: %v", err)
			return nil, fmt.Errorf("invalid Service Account Credentials provided")
		}
		return srv, nil
	}
	// client oauth creds are used
	srv, err := admin.NewService(ctx, option.WithTokenSource(accessTokenSource))
	if err != nil {
		return nil, err
	}
	return srv, nil
}

func (g *googleOauthProvider) GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string {
	extras := make(map[string][]string)
	if userPrincipal.Name != "" {
		extras[common.UserAttributePrincipalID] = []string{userPrincipal.Name}
	}
	if userPrincipal.LoginName != "" {
		extras[common.UserAttributeUserName] = []string{userPrincipal.LoginName}
	}
	return extras
}

// IsDisabledProvider checks if the Google auth provider is currently disabled in Rancher.
func (g *googleOauthProvider) IsDisabledProvider() (bool, error) {
	googleOauthConfig, err := g.getGoogleOAuthConfigCR()
	if err != nil {
		return false, err
	}
	return !googleOauthConfig.Enabled, nil
}
