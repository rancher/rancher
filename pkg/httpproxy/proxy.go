package httpproxy

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"

	mgmt "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	prov "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/management/cluster"
	provcluster "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/steve/pkg/auth"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/publicsuffix"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/apiserver/pkg/endpoints/request"
)

const (
	ForwardProto = "X-Forwarded-Proto"
	APIAuth      = "X-API-Auth-Header"
	CattleAuth   = "X-API-CattleAuth-Header"
	AuthHeader   = "Authorization"
	SetCookie    = "Set-Cookie"
	Cookie       = "Cookie"
	APISetCookie = "X-Api-Set-Cookie-Header"
	APICookie    = "X-Api-Cookie-Header"
	hostRegex    = "[A-Za-z0-9-]+"
	CSP          = "Content-Security-Policy"
	XContentType = "X-Content-Type-Options"
)

var (
	httpStart  = regexp.MustCompile("^http:/([^/])")
	httpsStart = regexp.MustCompile("^https:/([^/])")
	badHeaders = map[string]bool{
		"host":                    true,
		"transfer-encoding":       true,
		"content-length":          true,
		"x-api-auth-header":       true,
		"x-api-cattleauth-header": true,
		"cf-connecting-ip":        true,
		"cf-ray":                  true,
	}
	badHeaderPrefixes = []string{
		"impersonate-",
	}
)

func isBadHeader(header string) bool {
	header = strings.ToLower(header)

	if badHeaders[header] {
		return true
	}

	for _, prefix := range badHeaderPrefixes {
		if strings.HasPrefix(header, prefix) {
			return true
		}
	}

	return false
}

type Supplier func() []string

type proxy struct {
	prefix             string
	validHostsSupplier Supplier
	credentials        v1.SecretInterface
	mgmtClustersCache  mgmtv3.ClusterCache
	provClustersCache  provv1.ClusterCache
	proxyEndpointCache mgmtv3.ProxyEndpointCache
	authorizer         authorizer.Authorizer
	insecureTransport  http.RoundTripper
}

func (p *proxy) isAllowed(host string) bool {
	for _, valid := range p.validHostsSupplier() {
		if valid == host {
			return true
		}

		// Ideally the rancher webhook would prevent resources from specifying an overly
		// broad domain from the get-go, but due to rancher/rancher/issues/50631,
		// this may not always be the case. To prevent potential security issues,
		// we also check for overly broad domains here and skip them if found.
		if isOverlyBroad(valid) {
			logrus.Debugf("Skipping overly broad wildcard match for proxy request: %s", valid)
			continue
		}

		if strings.HasPrefix(valid, "*") && strings.HasSuffix(host, valid[1:]) {
			return true
		}

		if strings.Contains(valid, ".%.") || strings.HasPrefix(valid, "%.") {
			r := constructRegex(valid)
			if match := r.MatchString(host); match {
				return true
			}
		}
	}

	return false
}

func NewProxy(prefix string, validHosts Supplier, scaledContext *config.ScaledContext) (http.Handler, error) {
	cfg := authorizerfactory.DelegatingAuthorizerConfig{
		SubjectAccessReviewClient: scaledContext.K8sClient.AuthorizationV1(),
		AllowCacheTTL:             time.Second * time.Duration(settings.AuthorizationCacheTTLSeconds.GetInt()),
		DenyCacheTTL:              time.Second * time.Duration(settings.AuthorizationDenyCacheTTLSeconds.GetInt()),
		WebhookRetryBackoff:       &auth.WebhookBackoff,
	}

	authorizer, err := cfg.New()
	if err != nil {
		return nil, err
	}

	p := proxy{
		authorizer:         authorizer,
		prefix:             prefix,
		validHostsSupplier: validHosts,
		credentials:        scaledContext.Core.Secrets(""),
		mgmtClustersCache:  scaledContext.Wrangler.Mgmt.Cluster().Cache(),
		provClustersCache:  scaledContext.Wrangler.Provisioning.Cluster().Cache(),
		proxyEndpointCache: scaledContext.Wrangler.Mgmt.ProxyEndpoint().Cache(),
		insecureTransport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // intentional, opt-in per route
		},
	}

	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			if err := p.proxy(req); err != nil {
				logrus.Infof("Failed to proxy request: %v", err)
			}
		},
		ModifyResponse: setModifiedHeaders,
		Transport:      &perRouteTLSTransport{proxy: &p},
	}, nil
}

func setModifiedHeaders(res *http.Response) error {
	// replace set cookies
	res.Header.Del(APISetCookie)
	// There may be multiple set cookies
	for _, setCookie := range res.Header[SetCookie] {
		res.Header.Add(APISetCookie, setCookie)
	}
	res.Header.Del(SetCookie)
	// add security headers (similar to raw.githubusercontent)
	res.Header.Set(CSP, "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	res.Header.Set(XContentType, "nosniff")
	return nil
}

func (p *proxy) proxy(req *http.Request) error {
	path := req.URL.String()
	index := strings.Index(path, p.prefix)
	destPath := path[index+len(p.prefix):]

	if httpsStart.MatchString(destPath) {
		destPath = httpsStart.ReplaceAllString(destPath, "https://$1")
	} else if httpStart.MatchString(destPath) {
		destPath = httpStart.ReplaceAllString(destPath, "http://$1")
	} else {
		destPath = "https://" + destPath
	}

	destURL, err := url.Parse(destPath)
	if err != nil {
		return err
	}

	destURL.RawQuery = req.URL.RawQuery
	destURLHostname := destURL.Hostname()

	if !p.isAllowed(destURLHostname) {
		return fmt.Errorf("invalid host: %v", destURLHostname)
	}

	headerCopy := http.Header{}

	if req.TLS != nil {
		headerCopy.Set(ForwardProto, "https")
	}

	auth := req.Header.Get(APIAuth)
	cAuth := req.Header.Get(CattleAuth)

	for key, value := range req.Header {
		if isBadHeader(key) {
			continue
		}

		copy := make([]string, len(value))
		for i := range value {
			copy[i] = strings.TrimPrefix(value[i], "rancher:")
		}
		headerCopy[key] = copy
	}

	req.Host = destURLHostname
	req.URL = destURL
	req.Header = headerCopy

	if auth != "" { // non-empty AuthHeader is noop
		req.Header.Set(AuthHeader, auth)
	} else if cAuth != "" {
		// If a known signer mode is specified by the client, use it directly.
		signer := newSigner(cAuth)
		if signer != nil {
			return signer.sign(req, p.secretGetter(req, cAuth), cAuth)
		}
		// No client-specified mode: check whether the matching ProxyEndpoint route
		// defines a server-side injection pattern. This allows extension authors to
		// control how credentials are applied without requiring the client to know
		// the injection details.
		if route := p.findMatchingRoute(destURLHostname); route != nil && route.CredentialInjection != nil {
			return p.applyRouteInjection(req, cAuth, route)
		}
		req.Header.Set(AuthHeader, cAuth)
	}

	replaceCookies(req)

	return nil
}

func (p *proxy) secretGetter(req *http.Request, cAuth string) SecretGetter {
	clusterID := getRequestParams(cAuth)["clusterID"]
	return func(namespace, name string) (*corev1.Secret, error) {
		user, ok := request.UserFrom(req.Context())
		if !ok {
			return nil, fmt.Errorf("failed to find user")
		}
		decision, reason, err := p.authorizer.Authorize(req.Context(), authorizer.AttributesRecord{
			User:            user,
			Verb:            "get",
			Namespace:       namespace,
			APIVersion:      "v1",
			Resource:        "secrets",
			Name:            name,
			ResourceRequest: true,
		})
		if err != nil {
			return nil, err
		}
		unauthorizedErr := fmt.Errorf("unauthorized %s to %s/%s: %s", user.GetName(), namespace, name, reason)
		if decision != authorizer.DecisionAllow {
			decision, err = p.checkIndirectAccessViaCluster(req, user, clusterID, fmt.Sprintf("%s:%s", namespace, name))
			if err != nil {
				return nil, err
			}
			if decision != authorizer.DecisionAllow {
				return nil, unauthorizedErr
			}
		}
		return p.credentials.Controller().Lister().Get(namespace, name)
	}
}

// checkIndirectAccessViaCluster checks if the user has access to the cloud credential via being owner of a cluster associated to the cloud credential.
// Currently, only EKS and provisioningv2 clusters are supported because those clusters have a cloud credential associated to them.
// GKE and AKS clusters also have cloud credential associated to them, but those are checked via specific proxies (not the meta proxy).
func (p *proxy) checkIndirectAccessViaCluster(req *http.Request, user user.Info, clusterID, credID string) (authorizer.Decision, error) {
	var (
		mgmtClusters []*mgmt.Cluster
		provClusters []*prov.Cluster
		err          error
	)
	if clusterID == "" {
		// If no clusterID is passed, then we check all clusters that the user has access to and are associated to the cloud credential.
		// Both management and provisioning clusters should be checked.
		mgmtClusters, err = p.mgmtClustersCache.GetByIndex(cluster.ByCloudCredential, credID)
		if err != nil {
			return authorizer.DecisionDeny, err
		}

		provClusters, err = p.provClustersCache.GetByIndex(provcluster.ByCloudCred, credID)
		if err != nil {
			return authorizer.DecisionDeny, err
		}
	} else {
		if c, err := p.mgmtClustersCache.Get(clusterID); err == nil {
			mgmtClusters = []*mgmt.Cluster{c}
		} else {
			return authorizer.DecisionDeny, err
		}
		provClusters, err = p.provClustersCache.GetByIndex(provcluster.ByCluster, clusterID)
		if err != nil {
			return authorizer.DecisionDeny, err
		}
	}
	if len(mgmtClusters)+len(provClusters) == 0 {
		return authorizer.DecisionDeny, err
	}

	for _, c := range mgmtClusters {
		if c.Spec.EKSConfig == nil || c.Spec.EKSConfig.AmazonCredentialSecret != credID {
			continue
		}

		decision, err := p.checkAccessToV3ClusterWithID(req, user, c.Name)
		if err == nil && decision == authorizer.DecisionAllow {
			return decision, nil
		}
	}

	for _, c := range provClusters {
		if c.Spec.CloudCredentialSecretName != credID {
			continue
		}

		// Check that the user has access to the management cluster associated to the provisioning cluster.
		// If a user has access to the management cluster, then the user has access to the provisioning cluster.
		decision, err := p.checkAccessToV3ClusterWithID(req, user, c.Status.ClusterName)
		if err == nil && decision == authorizer.DecisionAllow {
			return decision, nil
		}
	}
	return authorizer.DecisionDeny, nil
}

func (p *proxy) checkAccessToV3ClusterWithID(req *http.Request, user user.Info, clusterID string) (authorizer.Decision, error) {
	decision, _, err := p.authorizer.Authorize(req.Context(), authorizer.AttributesRecord{
		User:            user,
		Verb:            "update",
		APIGroup:        v3.GroupName,
		APIVersion:      v3.Version,
		Resource:        "clusters",
		Name:            clusterID,
		ResourceRequest: true,
	})

	return decision, err
}

func replaceCookies(req *http.Request) {
	// Do not forward rancher cookies to third parties
	req.Header.Del(Cookie)
	// Allow client to use their own cookies with Cookie header
	if cookie := req.Header.Get(APICookie); cookie != "" {
		req.Header.Set(Cookie, cookie)
		req.Header.Del(APICookie)
	}
}

func constructRegex(host string) *regexp.Regexp {
	// incoming host "ec2.%.amazonaws.com"
	// Converted to regex "^ec2\.[A-Za-z0-9-]+\.amazonaws\.com$"
	parts := strings.Split(host, ".")
	for i, part := range parts {
		if part == "%" {
			parts[i] = hostRegex
		} else {
			parts[i] = regexp.QuoteMeta(part)
		}
	}

	str := "^" + strings.Join(parts, "\\.") + "$"

	return regexp.MustCompile(str)
}

// isOverlyBroad checks if the given domain is an overly broad wildcard
// that would allow proxying to essentially any domain. It does this by determining the
// eTLD and ensuring that the segment preceding that is not a wildcard.
func isOverlyBroad(pattern string) bool {
	if !strings.ContainsAny(pattern, "*%") {
		return false
	}

	// replace wildcards with a valid character so publicsuffix can parse it
	normalized := strings.ReplaceAll(pattern, "*", "z")
	normalized = strings.ReplaceAll(normalized, "%", "z")

	// get the suffix, .com, .co.uk, etc.
	suffix, _ := publicsuffix.PublicSuffix(normalized)

	// identify the label right before the eTLD
	suffixDotCount := strings.Count(suffix, ".")
	labels := strings.Split(pattern, ".")

	// Find the character for that label
	idx := len(labels) - suffixDotCount - 2

	if idx < 0 {
		return true // Pattern is just a suffix, treat as broad/invalid
	}
	targetLabel := labels[idx]

	// check if that label is a plain wildcard.
	return targetLabel == "*" || targetLabel == "%"
}

// findMatchingRoute returns the first ProxyEndpointRoute whose domain pattern matches host,
// or nil if no match is found. It uses the same wildcard/regex logic as isAllowed.
func (p *proxy) findMatchingRoute(host string) *mgmt.ProxyEndpointRoute {
	endpoints, err := p.proxyEndpointCache.List(nil)
	if err != nil {
		logrus.Debugf("httpproxy: failed to list ProxyEndpoints for route lookup: %v", err)
		return nil
	}
	for _, ep := range endpoints {
		for i := range ep.Spec.Routes {
			route := &ep.Spec.Routes[i]
			if routeMatchesHost(route.Domain, host) {
				return route
			}
		}
	}
	return nil
}

// routeMatchesHost reports whether the domain pattern from a ProxyEndpointRoute matches host,
// using the same rules as proxy.isAllowed.
func routeMatchesHost(pattern, host string) bool {
	if pattern == host {
		return true
	}
	if isOverlyBroad(pattern) {
		return false
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(host, pattern[1:]) {
		return true
	}
	if strings.Contains(pattern, ".%.") || strings.HasPrefix(pattern, "%.") {
		return constructRegex(pattern).MatchString(host)
	}
	return false
}

// applyRouteInjection fetches the credential identified by credID in cAuth, then applies the
// injection pattern defined on the matching ProxyEndpoint route to the outgoing request.
func (p *proxy) applyRouteInjection(req *http.Request, cAuth string, route *mgmt.ProxyEndpointRoute) error {
	params := getRequestParams(cAuth)
	credID := params["credID"]
	if credID == "" {
		return fmt.Errorf("server-defined injection requires credID in %s header", CattleAuth)
	}
	secretData, err := getCredential(credID, p.secretGetter(req, cAuth))
	if err != nil {
		return fmt.Errorf("failed to retrieve credential for route injection: %w", err)
	}
	return applyInjectionSpec(req, route.CredentialInjection, secretData)
}

// perRouteTLSTransport selects the appropriate HTTP transport based on whether the destination
// ProxyEndpointRoute has InsecureSkipTLSVerify enabled.
type perRouteTLSTransport struct {
	proxy *proxy
}

func (t *perRouteTLSTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	route := t.proxy.findMatchingRoute(req.URL.Hostname())
	if route != nil && route.InsecureSkipTLSVerify {
		return t.proxy.insecureTransport.RoundTrip(req)
	}
	return http.DefaultTransport.RoundTrip(req)
}
