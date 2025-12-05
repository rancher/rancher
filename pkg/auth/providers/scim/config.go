package scim

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

type AuthenticationType string

const (
	// AuthenticationTypeOauth indicates that the authentication type is OAuth.
	AuthenticationTypeOauth AuthenticationType = "oauth"
	// AuthenticationTypeOauth2 indicates that the authentication type is OAuth2.
	AuthenticationTypeOauth2 AuthenticationType = "oauth2"
	// AuthenticationTypeOauthBearerToken indicates that the authentication type is OAuth2 Bearer Token.
	AuthenticationTypeOauthBearerToken AuthenticationType = "oauthbearertoken"
	// AuthenticationTypeHTTPBasic indicated that the authentication type is Basic Access Authentication.
	AuthenticationTypeHTTPBasic AuthenticationType = "httpbasic"
	// AuthenticationTypeHTTPDigest indicated that the authentication type is Digest Access Authentication.
	AuthenticationTypeHTTPDigest AuthenticationType = "httpdigest"
)

type AuthenticationScheme struct {
	// Type is the authentication scheme. This specification defines the values "oauth", "oauth2", "oauthbearertoken",
	// "httpbasic", and "httpdigest".
	Type AuthenticationType
	// Name is the common authentication scheme name, e.g., HTTP Basic.
	Name string
	// Description of the authentication scheme.
	Description string
	// SpecURI is an HTTP-addressable URL pointing to the authentication scheme's specification.
	SpecURI string
	// DocumentationURI is an HTTP-addressable URL pointing to the authentication scheme's usage documentation.
	DocumentationURI string
	// Primary is a boolean value indicating the 'primary' or preferred authentication scheme.
	Primary bool
}
type ServiceProviderConfig struct {
	// AuthenticationSchemes is a multi-valued complex type that specifies supported authentication scheme properties.
	AuthenticationSchemes []AuthenticationScheme
	// MaxResults denotes the the integer value specifying the maximum number of resources returned in a response. It defaults to 100.
	MaxResults int
	// SupportFiltering whether you SCIM implementation will support filtering.
	SupportFiltering bool
	// SupportPatch whether your SCIM implementation will support patch requests.
	SupportPatch bool
}

func (c ServiceProviderConfig) getRaw() map[string]any {
	return map[string]any{
		"schemas":          []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"documentationUri": "https://ranchermanager.docs.rancher.com",
		"patch": map[string]bool{
			"supported": c.SupportPatch,
		},
		"bulk": map[string]any{
			"supported":      false,
			"maxOperations":  1000,
			"maxPayloadSize": 1048576,
		},
		"filter": map[string]any{
			"supported":  c.SupportFiltering,
			"maxResults": c.MaxResults,
		},
		"changePassword": map[string]bool{
			"supported": false,
		},
		"sort": map[string]bool{
			"supported": false,
		},
		"etag": map[string]bool{
			"supported": false,
		},
		"authenticationSchemes": c.getRawAuthenticationSchemes(),
	}
}

func (c ServiceProviderConfig) getRawAuthenticationSchemes() []map[string]any {
	schemes := make([]map[string]any, 0)
	for _, s := range c.AuthenticationSchemes {
		schemes = append(schemes, map[string]any{
			"description":      s.Description,
			"documentationUri": s.DocumentationURI,
			"name":             s.Name,
			"primary":          s.Primary,
			"specUri":          s.SpecURI,
			"type":             s.Type,
		})
	}
	return schemes
}

func (s *SCIMServer) GetServiceProviderConfig(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("scim::GetServiceProviderConfig: url %s", r.URL.String())

	config := &ServiceProviderConfig{
		AuthenticationSchemes: []AuthenticationScheme{
			{
				Type:        AuthenticationTypeOauthBearerToken,
				Name:        "OAuth Bearer Token",
				Description: "Authentication scheme using the OAuth Bearer Token",
				SpecURI:     "http://tools.ietf.org/html/draft-ietf-scim-core-protocol-10#section-3.1",
				Primary:     true,
			},
		},
		MaxResults:       100,
		SupportFiltering: false,
		SupportPatch:     false,
	}

	writeResponse(w, config.getRaw())
}
