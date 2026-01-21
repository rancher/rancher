package scim

import (
	"net/http"
	"net/url"

	authutil "github.com/rancher/rancher/pkg/auth/util"
	"github.com/sirupsen/logrus"
)

const (
	// URLPrefix is the base path for SCIM API endpoints.
	URLPrefix = "/v1-scim"
)

const (
	secretKindLabel   = "cattle.io/kind"
	authProviderLabel = "authn.management.cattle.io/provider"
	scimAuthToken     = "scim-auth-token"
)

// patchOp defines a single operation in a SCIM PATCH request.
type patchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

// first returns the first element of the slice s, or the zero value of E if s is empty.
func first[Slice ~[]E, E any](s Slice) E {
	if len(s) > 0 {
		return s[0]
	}

	var e E
	return e
}

// locationURL constructs the location URL for a SCIM resource.
func locationURL(r *http.Request, provider, resourceType, id string) string {
	host := "https://" + authutil.GetHost(r)
	location, err := url.JoinPath(host, URLPrefix, provider, resourceType, id)
	if err != nil {
		logrus.Errorf("scim::locationURL: failed to join URL path: %s", err)
		return "" // TODO: Revisit this.
	}
	return location
}
