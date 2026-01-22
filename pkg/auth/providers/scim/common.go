package scim

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

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

const (
	// DefaultPageSize is the default number of results per page when count is not specified.
	DefaultPageSize = 100
	// MaxPageSize is the maximum allowed page size to prevent excessive memory usage.
	MaxPageSize = 1000
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

// PaginationParams holds parsed and validated pagination parameters.
type PaginationParams struct {
	StartIndex int // 1-based, validated to be >= 1.
	Count      int // Number of items per page, clamped to [0, MaxPageSize].
}

// ParsePaginationParams extracts and validates startIndex and count from the request.
// Returns defaults if parameters are not provided.
func ParsePaginationParams(r *http.Request) (PaginationParams, error) {
	params := PaginationParams{
		StartIndex: 1,
		Count:      DefaultPageSize,
	}

	if value := r.URL.Query().Get("startIndex"); value != "" {
		idx, err := strconv.Atoi(value)
		if err != nil {
			return params, fmt.Errorf("invalid startIndex: must be an integer")
		}
		if idx < 1 {
			return params, fmt.Errorf("invalid startIndex: must be >= 1")
		}
		params.StartIndex = idx
	}

	if value := r.URL.Query().Get("count"); value != "" {
		cnt, err := strconv.Atoi(value)
		if err != nil {
			return params, fmt.Errorf("invalid count: must be an integer")
		}
		if cnt < 0 {
			return params, fmt.Errorf("invalid count: must be >= 0")
		}
		if cnt > MaxPageSize {
			cnt = MaxPageSize
		}
		params.Count = cnt
	}

	return params, nil
}

// Paginate applies pagination to a slice and returns the paginated subset along with
// the actual start index used. The startIndex in params is 1-based per SCIM protocol.
func Paginate[T any](items []T, params PaginationParams) (result []T, startIndex int) {
	total := len(items)
	startIndex = params.StartIndex

	// Convert 1-based startIndex to 0-based offset.
	offset := params.StartIndex - 1

	// If offset is beyond the total, return empty slice.
	if offset >= total || offset < 0 {
		return []T{}, startIndex
	}

	// Calculate end index.
	end := min(offset+params.Count, total)

	return items[offset:end], startIndex
}
