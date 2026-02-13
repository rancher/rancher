package scim

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

// AuthenticationType defines the type of authentication scheme.
type AuthenticationType string

const (
	// AuthenticationTypeOauth indicates that the authentication type is OAuth.
	AuthenticationTypeOauth AuthenticationType = "oauth"
	// AuthenticationTypeOauth2 indicates that the authentication type is OAuth2.
	AuthenticationTypeOauth2 AuthenticationType = "oauth2"
	// AuthenticationTypeOauthBearerToken indicates that the authentication type is OAuth2 Bearer Token.
	AuthenticationTypeOauthBearerToken AuthenticationType = "oauthbearertoken"
)

// AuthenticationScheme represents an authentication scheme supported by the service provider.
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

// ServiceProviderConfig represents the SCIM Service Provider Configuration.
type ServiceProviderConfig struct {
	// AuthenticationSchemes is a multi-valued complex type that specifies supported authentication scheme properties.
	AuthenticationSchemes []AuthenticationScheme
	// MaxResults denotes the the integer value specifying the maximum number of resources returned in a response. It defaults to 100.
	MaxResults int
	// SupportFiltering indicates whether or not the SCIM implementation supports filtering.
	SupportFiltering bool
	// SupportPatch indicates whether or not the SCIM implementation supports patch requests.
	SupportPatch bool
}

// getRaw returns the raw representation of the ServiceProviderConfig.
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

// getRawAuthenticationSchemes returns the raw representation of the authentication schemes.
func (c ServiceProviderConfig) getRawAuthenticationSchemes() []map[string]any {
	schemes := make([]map[string]any, 0, len(c.AuthenticationSchemes))
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

// GetServiceProviderConfig returns the SCIM Service Provider Configuration.
func (s *SCIMServer) GetServiceProviderConfig(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::GetServiceProviderConfig: url %s", r.URL)

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
		MaxResults:       MaxPageSize,
		SupportFiltering: true,
		SupportPatch:     true,
	}

	writeResponse(w, config.getRaw())
}
