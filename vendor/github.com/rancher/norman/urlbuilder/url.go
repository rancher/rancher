package urlbuilder

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/wrangler/pkg/name"
)

const (
	PrefixHeader           = "X-API-URL-Prefix"
	ForwardedAPIHostHeader = "X-API-Host"
	ForwardedHostHeader    = "X-Forwarded-Host"
	ForwardedProtoHeader   = "X-Forwarded-Proto"
	ForwardedPortHeader    = "X-Forwarded-Port"
)

func New(r *http.Request, version types.APIVersion, schemas *types.Schemas) (types.URLBuilder, error) {
	requestURL := ParseRequestURL(r)
	responseURLBase, err := parseResponseURLBase(requestURL, r)
	if err != nil {
		return nil, err
	}

	builder := &urlBuilder{
		schemas:         schemas,
		requestURL:      requestURL,
		responseURLBase: responseURLBase,
		apiVersion:      version,
		query:           r.URL.Query(),
	}

	return builder, nil
}

func ParseRequestURL(r *http.Request) string {
	scheme := GetScheme(r)
	host := GetHost(r, scheme)
	return fmt.Sprintf("%s://%s%s%s", scheme, host, r.Header.Get(PrefixHeader), r.URL.Path)
}

func GetHost(r *http.Request, scheme string) string {
	host := r.Header.Get(ForwardedAPIHostHeader)
	if host != "" {
		return host
	}

	host = strings.Split(r.Header.Get(ForwardedHostHeader), ",")[0]
	if host == "" {
		host = r.Host
	}

	port := r.Header.Get(ForwardedPortHeader)
	if port == "" {
		return host
	}

	if port == "80" && scheme == "http" {
		return host
	}

	if port == "443" && scheme == "http" {
		return host
	}

	hostname, _, err := net.SplitHostPort(host)
	if err != nil {
		hostname = host
	}

	return strings.Join([]string{hostname, port}, ":")
}

func GetScheme(r *http.Request) string {
	scheme := r.Header.Get(ForwardedProtoHeader)
	if scheme != "" {
		switch scheme {
		case "ws":
			return "http"
		case "wss":
			return "https"
		default:
			return scheme
		}
	} else if r.TLS != nil {
		return "https"
	}
	return "http"
}

type urlBuilder struct {
	schemas         *types.Schemas
	requestURL      string
	responseURLBase string
	apiVersion      types.APIVersion
	subContext      string
	query           url.Values
}

func (u *urlBuilder) SetSubContext(subContext string) {
	u.subContext = subContext
}

func (u *urlBuilder) SchemaLink(schema *types.Schema) string {
	return u.constructBasicURL(schema.Version, "schemas", schema.ID)
}

func (u *urlBuilder) Link(linkName string, resource *types.RawResource) string {
	if resource.ID == "" || linkName == "" {
		return ""
	}

	if self, ok := resource.Links["self"]; ok {
		return self + "/" + strings.ToLower(linkName)
	}

	return u.constructBasicURL(resource.Schema.Version, resource.Schema.PluralName, resource.ID, strings.ToLower(linkName))
}

func (u *urlBuilder) ResourceLink(resource *types.RawResource) string {
	if resource.ID == "" {
		return ""
	}

	return u.constructBasicURL(resource.Schema.Version, resource.Schema.PluralName, resource.ID)
}

func (u *urlBuilder) Marker(marker string) string {
	newValues := url.Values{}
	for k, v := range u.query {
		newValues[k] = v
	}
	newValues.Set("marker", marker)
	return u.requestURL + "?" + newValues.Encode()
}

func (u *urlBuilder) ReverseSort(order types.SortOrder) string {
	newValues := url.Values{}
	for k, v := range u.query {
		newValues[k] = v
	}
	newValues.Del("order")
	newValues.Del("marker")
	if order == types.ASC {
		newValues.Add("order", string(types.DESC))
	} else {
		newValues.Add("order", string(types.ASC))
	}

	return u.requestURL + "?" + newValues.Encode()
}

func (u *urlBuilder) Current() string {
	return u.requestURL
}

func (u *urlBuilder) RelativeToRoot(path string) string {
	return u.responseURLBase + path
}

func (u *urlBuilder) Sort(field string) string {
	newValues := url.Values{}
	for k, v := range u.query {
		newValues[k] = v
	}
	newValues.Del("order")
	newValues.Del("marker")
	newValues.Set("sort", field)
	return u.requestURL + "?" + newValues.Encode()
}

func (u *urlBuilder) Collection(schema *types.Schema, versionOverride *types.APIVersion) string {
	plural := u.getPluralName(schema)
	if versionOverride == nil {
		return u.constructBasicURL(schema.Version, plural)
	}
	return u.constructBasicURL(*versionOverride, plural)
}

func (u *urlBuilder) SubContextCollection(subContext *types.Schema, contextName string, schema *types.Schema) string {
	return u.constructBasicURL(subContext.Version, subContext.PluralName, contextName, u.getPluralName(schema))
}

func (u *urlBuilder) Version(version types.APIVersion) string {
	return u.constructBasicURL(version)
}

func (u *urlBuilder) FilterLink(schema *types.Schema, fieldName string, value string) string {
	return u.constructBasicURL(schema.Version, schema.PluralName) + "?" +
		url.QueryEscape(fieldName) + "=" + url.QueryEscape(value)
}

func (u *urlBuilder) ResourceLinkByID(schema *types.Schema, id string) string {
	return u.constructBasicURL(schema.Version, schema.PluralName, id)
}

func (u *urlBuilder) constructBasicURL(version types.APIVersion, parts ...string) string {
	buffer := strings.Builder{}

	buffer.WriteString(u.responseURLBase)
	if version.Path == "" {
		buffer.WriteString(u.apiVersion.Path)
	} else {
		buffer.WriteString(version.Path)
	}
	buffer.WriteString(u.subContext)

	for _, part := range parts {
		if part == "" {
			return ""
		}
		buffer.WriteString("/")
		buffer.WriteString(part)
	}

	return buffer.String()
}

func (u *urlBuilder) getPluralName(schema *types.Schema) string {
	if schema.PluralName == "" {
		return strings.ToLower(name.GuessPluralName(schema.ID))
	}
	return strings.ToLower(schema.PluralName)
}

func parseResponseURLBase(requestURL string, r *http.Request) (string, error) {
	path := r.URL.Path

	index := strings.LastIndex(requestURL, path)
	if index == -1 {
		// Fallback, if we can't find path in requestURL, then we just assume the base is the root of the web request
		u, err := url.Parse(requestURL)
		if err != nil {
			return "", err
		}

		buffer := bytes.Buffer{}
		buffer.WriteString(u.Scheme)
		buffer.WriteString("://")
		buffer.WriteString(u.Host)
		return buffer.String(), nil
	}

	return requestURL[0:index], nil
}

func (u *urlBuilder) Action(action string, resource *types.RawResource) string {
	return u.constructBasicURL(resource.Schema.Version, resource.Schema.PluralName, resource.ID) + "?action=" + url.QueryEscape(action)
}

func (u *urlBuilder) CollectionAction(schema *types.Schema, versionOverride *types.APIVersion, action string) string {
	collectionURL := u.Collection(schema, versionOverride)
	return collectionURL + "?action=" + url.QueryEscape(action)
}

func (u *urlBuilder) ActionLinkByID(schema *types.Schema, id string, action string) string {
	return u.constructBasicURL(schema.Version, schema.PluralName, id) + "?action=" + url.QueryEscape(action)
}
