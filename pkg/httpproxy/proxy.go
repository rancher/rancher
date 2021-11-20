package httpproxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"

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
		"impersonate-user":        true,
		"impersonate-group":       true,
	}
)

type Supplier func() []string

type proxy struct {
	prefix             string
	validHostsSupplier Supplier
	credentials        v1.SecretInterface
	mgmtClustersCache  mgmtv3.ClusterCache
	provClustersCache  provv1.ClusterCache
	authorizer         authorizer.Authorizer
}

func (p *proxy) isAllowed(host string) bool {
	for _, valid := range p.validHostsSupplier() {
		if valid == host {
			return true
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
	}

	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			if err := p.proxy(req); err != nil {
				logrus.Infof("Failed to proxy: %v", err)
			}
		},
		ModifyResponse: setModifiedHeaders,
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
	for name, value := range req.Header {
		if badHeaders[strings.ToLower(name)] {
			continue
		}

		copy := make([]string, len(value))
		for i := range value {
			copy[i] = strings.TrimPrefix(value[i], "rancher:")
		}
		headerCopy[name] = copy
	}

	req.Host = destURLHostname
	req.URL = destURL
	req.Header = headerCopy

	if auth != "" { // non-empty AuthHeader is noop
		req.Header.Set(AuthHeader, auth)
	} else if cAuth != "" {
		// setting CattleAuthHeader will replace credential id with secret data
		// and generate signature
		signer := newSigner(cAuth)
		if signer != nil {
			return signer.sign(req, p.secretGetter(req, cAuth), cAuth)
		}
		req.Header.Set(AuthHeader, cAuth)
	}

	replaceCookies(req)

	return nil
}

func (p *proxy) secretGetter(req *http.Request, cAuth string) SecretGetter {
	clusterID := getRequestParams(cAuth)["clusterID"]
	return func(namespace, name string) (*v1.Secret, error) {
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
		mgmtClusters []*v3.Cluster
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
			mgmtClusters = []*v3.Cluster{c}
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
