package githubapp

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLogOutAll(t *testing.T) {
	provider := Provider{
		githubClient: &githubAppClient{httpClient: http.DefaultClient},
		getConfig:    func() (*apiv3.GithubAppConfig, error) { return nil, nil },
	}

	// LogoutAll does nothing in this case and does not fail.
	require.NoError(t, provider.LogoutAll(nil, nil, nil))
}

func TestLogOut(t *testing.T) {
	provider := Provider{
		githubClient: &githubAppClient{httpClient: http.DefaultClient},
		getConfig:    func() (*apiv3.GithubAppConfig, error) { return nil, nil },
	}

	// Logout does nothing in this case and does not fail.
	require.NoError(t, provider.Logout(nil, nil, nil))
}

func TestGetName(t *testing.T) {
	provider := Provider{
		githubClient: &githubAppClient{httpClient: http.DefaultClient},
		getConfig:    func() (*apiv3.GithubAppConfig, error) { return nil, nil },
	}

	assert.Equal(t, Name, provider.GetName())
}

func TestCustomizeSchema(t *testing.T) {
	// This isn't currently tested
}

func TestTransformToAuthProvider(t *testing.T) {
	config := &apiv3.GithubAppConfig{}

	provider := Provider{
		githubClient: &githubAppClient{httpClient: http.DefaultClient},
		getConfig:    func() (*apiv3.GithubAppConfig, error) { return config, nil },
	}

	t.Run("when no alternative client_id is provided for hostname", func(t *testing.T) {
		rawAuthConfig := map[string]any{
			client.GithubAppConfigFieldHostname: "suse.com",
			client.GithubAppConfigFieldClientID: "test_client_id",
			client.GithubAppConfigFieldTLS:      true,
			".host":                             "example.com",
		}

		transformed, err := provider.TransformToAuthProvider(rawAuthConfig)
		require.NoError(t, err)

		want := map[string]any{
			"logoutAllEnabled":                          false,
			"logoutAllForced":                           false,
			"logoutAllSupported":                        false,
			publicclient.GithubProviderFieldRedirectURL: "https://suse.com/login/oauth/authorize?client_id=test_client_id",
		}
		assert.Equal(t, want, transformed)
	})

	t.Run("when alternative client_id is provided for hostname", func(t *testing.T) {
		rawAuthConfig := map[string]any{
			client.GithubAppConfigFieldHostname: "suse.com",
			client.GithubAppConfigFieldClientID: "test_client_id",
			client.GithubAppConfigFieldTLS:      true,
			".host":                             "example.com",
			"hostnameToClientId": map[string]any{
				"example.com": "other_client_id",
			},
		}

		transformed, err := provider.TransformToAuthProvider(rawAuthConfig)
		require.NoError(t, err)

		// The client_id is replaced with the correct client_id for the host.
		want := map[string]any{
			"logoutAllEnabled":   false,
			"logoutAllForced":    false,
			"logoutAllSupported": false,
			"redirectUrl":        "https://suse.com/login/oauth/authorize?client_id=other_client_id",
		}
		assert.Equal(t, want, transformed)
	})
}

func TestAuthenticateUser(t *testing.T) {
	authCode := "1234567"
	appID := "23456"
	privateKey := newTestCertificate(t)

	srv := httptest.NewServer(newFakeGitHubServer(t,
		withTestCode("test_client_id", authCode, "http://localhost:3000/callback", "testing"),
		withPrivateKey(appID, privateKey),
	))
	defer srv.Close()

	config := &apiv3.GithubAppConfig{
		Hostname:     stripScheme(t, srv),
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		AppID:        appID,
		PrivateKey:   string(privateKey),
	}
	provider := Provider{
		githubClient: &githubAppClient{httpClient: http.DefaultClient},
		getConfig:    func() (*apiv3.GithubAppConfig, error) { return config, nil },
		userManager:  &fakeUserManager{},
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	input := &apiv3.GithubLogin{
		Code: authCode,
	}
	userPrincipal, groupPrincipals, token, err := provider.AuthenticateUser(httptest.NewRecorder(), req, input)
	require.NoError(t, err)

	wantUser := apiv3.Principal{
		ObjectMeta: metav1.ObjectMeta{
			Name: "githubapp_user://1234",
		},
		DisplayName:    "monalisa octocat",
		LoginName:      "octocat",
		ProfilePicture: "https://github.com/images/error/octocat_happy.gif",
		PrincipalType:  "user",
		Me:             true,
		Provider:       "githubapp",
	}
	assert.Equal(t, wantUser, userPrincipal)

	wantGroups := []apiv3.Principal{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "githubapp_org://1",
			},
			DisplayName:    "Example Org 1",
			LoginName:      "example-org-1",
			ProfilePicture: "https://example.com/example-org-1-avatar.jpg",
			ProfileURL:     "",
			PrincipalType:  "group",
			MemberOf:       true,
			Provider:       "githubapp",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "githubapp_org://2",
			},
			DisplayName:    "Example Org 2",
			LoginName:      "example-org-2",
			ProfilePicture: "https://example.com/example-org-2-avatar.jpg",
			PrincipalType:  "group",
			MemberOf:       true,
			Provider:       "githubapp",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "githubapp_team://1215",
			},
			DisplayName: "dev-team", LoginName: "dev-team", ProfilePicture: "https://example.com/example-org-1-avatar.jpg",
			PrincipalType: "group",
			MemberOf:      true,
			Provider:      "githubapp",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "githubapp_team://1216",
			},
			DisplayName:    "dev-team",
			LoginName:      "dev-team",
			ProfilePicture: "https://example.com/example-org-2-avatar.jpg",
			PrincipalType:  "group",
			MemberOf:       true,
			Provider:       "githubapp",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "githubapp_team://1217",
			},
			DisplayName:    "test-team",
			LoginName:      "test-team",
			ProfilePicture: "https://example.com/example-org-1-avatar.jpg",
			PrincipalType:  "group",
			MemberOf:       true,
			Provider:       "githubapp",
		},
	}

	slices.SortFunc(groupPrincipals, func(a, b apiv3.Principal) int {
		return strings.Compare(a.ObjectMeta.Name, b.ObjectMeta.Name)
	})

	assert.Equal(t, wantGroups, groupPrincipals)
	assert.Empty(t, token)
}

func TestRefetchGroupPrincipals(t *testing.T) {
	authCode := "1234567"
	appID := "23456"
	privateKey := newTestCertificate(t)

	srv := httptest.NewServer(newFakeGitHubServer(t,
		withTestCode("test_client_id", authCode, "http://localhost:3000/callback", "testing"),
		withPrivateKey(appID, privateKey),
	))
	defer srv.Close()

	config := &apiv3.GithubAppConfig{
		Hostname:     stripScheme(t, srv),
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		AppID:        appID,
		PrivateKey:   string(privateKey),
	}
	provider := Provider{
		githubClient: &githubAppClient{httpClient: http.DefaultClient},
		getConfig:    func() (*apiv3.GithubAppConfig, error) { return config, nil },
		userManager:  &fakeUserManager{},
	}

	principals, err := provider.RefetchGroupPrincipals("githubapp_user://1234", "unused parameter")
	require.NoError(t, err)

	want := []apiv3.Principal{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "githubapp_org://1",
			},
			DisplayName:    "Example Org 1",
			LoginName:      "example-org-1",
			ProfilePicture: "https://example.com/example-org-1-avatar.jpg",
			PrincipalType:  "group",
			MemberOf:       true,
			Provider:       "githubapp",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "githubapp_org://2",
			},
			DisplayName:    "Example Org 2",
			LoginName:      "example-org-2",
			ProfilePicture: "https://example.com/example-org-2-avatar.jpg",
			PrincipalType:  "group",
			MemberOf:       true,
			Provider:       "githubapp",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "githubapp_team://1215",
			},
			DisplayName:    "dev-team",
			LoginName:      "dev-team",
			ProfilePicture: "https://example.com/example-org-1-avatar.jpg",
			PrincipalType:  "group",
			MemberOf:       true,
			Provider:       "githubapp",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "githubapp_team://1216",
			},
			DisplayName:    "dev-team",
			LoginName:      "dev-team",
			ProfilePicture: "https://example.com/example-org-2-avatar.jpg",
			PrincipalType:  "group",
			MemberOf:       true,
			Provider:       "githubapp",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "githubapp_team://1217",
			},
			DisplayName:    "test-team",
			LoginName:      "test-team",
			ProfilePicture: "https://example.com/example-org-1-avatar.jpg",
			PrincipalType:  "group",
			MemberOf:       true,
			Provider:       "githubapp",
		},
	}
	slices.SortFunc(principals, func(a, b apiv3.Principal) int {
		return strings.Compare(a.ObjectMeta.Name, b.ObjectMeta.Name)
	})
	assert.Equal(t, want, principals)
}

func TestSearchPrincipals(t *testing.T) {
	authCode := "1234567"
	appID := "23456"
	privateKey := newTestCertificate(t)

	srv := httptest.NewServer(newFakeGitHubServer(t,
		withTestCode("test_client_id", authCode, "http://localhost:3000/callback", "testing"),
		withPrivateKey(appID, privateKey),
	))
	defer srv.Close()

	config := &apiv3.GithubAppConfig{
		Hostname:     stripScheme(t, srv),
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		AppID:        appID,
		PrivateKey:   string(privateKey),
	}
	provider := Provider{
		githubClient: &githubAppClient{httpClient: http.DefaultClient},
		getConfig:    func() (*apiv3.GithubAppConfig, error) { return config, nil },
		userManager:  &fakeUserManager{},
	}

	searchTests := map[string]struct {
		key           string
		principalType string
		want          []apiv3.Principal
	}{
		"searching for users": {
			"octocat",
			"user",
			[]apiv3.Principal{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "githubapp_user://1234",
					},
					DisplayName:    "monalisa octocat",
					LoginName:      "octocat",
					ProfilePicture: "https://github.com/images/error/octocat_happy.gif",
					PrincipalType:  userType,
					Provider:       "githubapp",
				},
			},
		},
		"searching for groups includes orgs": {
			"example-org-2",
			"group",
			[]apiv3.Principal{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "githubapp_org://2",
					},
					DisplayName:    "Example Org 2",
					LoginName:      "example-org-2",
					ProfilePicture: "https://example.com/example-org-2-avatar.jpg",
					PrincipalType:  "group",
					Provider:       "githubapp",
				},
			},
		},
		"searching for groups finds teams": {
			"dev",
			"group",
			[]apiv3.Principal{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "githubapp_team://1215",
					},
					DisplayName:    "Dev Team",
					LoginName:      "dev-team",
					ProfilePicture: "https://example.com/example-org-1-avatar.jpg",
					PrincipalType:  "group", MemberOf: false,
					Provider: "githubapp",
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "githubapp_team://1216",
					},
					DisplayName:    "Dev Team",
					LoginName:      "dev-team",
					ProfilePicture: "https://example.com/example-org-2-avatar.jpg",
					PrincipalType:  "group",
					Provider:       "githubapp",
				},
			},
		},
		"searching includes org and user": {
			"example",
			"user",
			[]apiv3.Principal{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "githubapp_org://1",
					},
					DisplayName:    "Example Org 1",
					LoginName:      "example-org-1",
					ProfilePicture: "https://example.com/example-org-1-avatar.jpg",
					PrincipalType:  "group",
					Provider:       "githubapp",
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "githubapp_org://2",
					},
					DisplayName:    "Example Org 2",
					LoginName:      "example-org-2",
					ProfilePicture: "https://example.com/example-org-2-avatar.jpg",
					PrincipalType:  "group",
					Provider:       "githubapp",
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "githubapp_user://2",
					},
					DisplayName:    "example user",
					LoginName:      "example",
					ProfilePicture: "https://github.com/images/error/example_happy.gif",
					PrincipalType:  "user",
					Provider:       "githubapp",
				},
			},
		},
	}

	for name, tt := range searchTests {
		t.Run(name, func(t *testing.T) {
			accts, err := provider.SearchPrincipals(tt.key, tt.principalType, nil)
			require.NoError(t, err)

			slices.SortFunc(accts, func(a, b apiv3.Principal) int {
				return strings.Compare(a.ObjectMeta.Name, b.ObjectMeta.Name)
			})

			assert.Equal(t, tt.want, accts)
		})
	}

}

func TestGetPrincipal(t *testing.T) {
	authCode := "1234567"
	appID := "23456"
	privateKey := newTestCertificate(t)

	srv := httptest.NewServer(newFakeGitHubServer(t,
		withTestCode("test_client_id", authCode, "http://localhost:3000/callback", "testing"),
		withPrivateKey(appID, privateKey),
	))
	defer srv.Close()

	config := &apiv3.GithubAppConfig{
		Hostname:     stripScheme(t, srv),
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		AppID:        appID,
		PrivateKey:   string(privateKey),
	}
	provider := Provider{
		githubClient: &githubAppClient{httpClient: http.DefaultClient},
		getConfig:    func() (*apiv3.GithubAppConfig, error) { return config, nil },
		userManager:  &fakeUserManager{},
	}

	principalTests := map[string]struct {
		principalID string
		want        apiv3.Principal
	}{
		"existing user": {
			"githubapp_user://1234",
			apiv3.Principal{
				ObjectMeta: metav1.ObjectMeta{
					Name: "githubapp_user://1234",
				},
				DisplayName:    "octocat",
				LoginName:      "octocat",
				ProfilePicture: "https://github.com/images/error/octocat_happy.gif",
				PrincipalType:  "user",
				Provider:       "githubapp",
			},
		},
		"existing org": {
			"githubapp_org://1",
			apiv3.Principal{
				ObjectMeta: metav1.ObjectMeta{
					Name: "githubapp_org://1",
				},
				DisplayName:    "Example Org 1",
				LoginName:      "example-org-1",
				ProfilePicture: "https://example.com/example-org-1-avatar.jpg",
				PrincipalType:  "group",
				Provider:       "githubapp",
			},
		},
	}

	for name, tt := range principalTests {
		t.Run(name, func(t *testing.T) {
			principal, err := provider.GetPrincipal(tt.principalID, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, principal)
		})
	}

}

func TestParsePrincipalID(t *testing.T) {
	parseTests := []struct {
		principalID string
		wantKind    string
		wantID      int
	}{
		{
			"githubapp_user://867746",
			userType,
			867746,
		},
	}

	for _, tt := range parseTests {
		t.Run(tt.principalID, func(t *testing.T) {
			principalKind, id, err := parsePrincipalID(tt.principalID)
			require.NoError(t, err)

			assert.Equal(t, tt.wantKind, principalKind)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

func TestParsePrincipalIDErrors(t *testing.T) {
	parseTests := []string{
		"githubapp_user://testing",
		"github://",
	}

	for _, tt := range parseTests {
		_, _, err := parsePrincipalID(tt)
		assert.ErrorContains(t, err, "invalid id "+tt)
	}
}

type fakeUserManager struct {
}

func (f *fakeUserManager) CheckAccess(accessMode string, allowedPrincipalIDs []string, userPrincipalID string, groups []apiv3.Principal) (bool, error) {
	return true, nil
}

func (f *fakeUserManager) SetPrincipalOnCurrentUser(r *http.Request, principal apiv3.Principal) (*apiv3.User, error) {
	return nil, nil
}

func (f *fakeUserManager) IsMemberOf(token accessor.TokenAccessor, group apiv3.Principal) bool {
	return true
}

func (f *fakeUserManager) UserAttributeCreateOrUpdate(userID, provider string, groupPrincipals []apiv3.Principal, userExtraInfo map[string][]string, loginTime ...time.Time) error {
	return nil
}
