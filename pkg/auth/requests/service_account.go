package requests

import (
	"fmt"
	"net/http"
	"strings"

	jwtv4 "github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	authcontext "github.com/rancher/rancher/pkg/auth/context"
	"github.com/rancher/rancher/pkg/auth/tokens"
	controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/steve/pkg/auth"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type (
	restConfigGetter  func(cluster *mgmtv3.Cluster, context *config.ScaledContext, secretLister corev1.SecretLister) (*rest.Config, error)
	authClientCreator func(clusterID string) (kubernetes.Interface, error)
)

// ServiceAccountAuth is an authenticator that authenticates requests using the downstream service account's JWT.
type ServiceAccountAuth struct {
	scaledContext             *config.ScaledContext
	clusterLister             mgmtv3.ClusterLister
	secretLister              corev1.SecretLister
	restConfigGetter          restConfigGetter
	authClientCreator         authClientCreator
	clusterProxyConfigsGetter controllers.ClusterProxyConfigCache
}

// NewServiceAccountAuth creates a new instance of ServiceAccountAuth.
func NewServiceAccountAuth(
	scaledContext *config.ScaledContext,
	restConfigGetter restConfigGetter,
) auth.Authenticator {
	return &ServiceAccountAuth{
		scaledContext:    scaledContext,
		clusterLister:    scaledContext.Management.Clusters("").Controller().Lister(),
		secretLister:     scaledContext.Core.Secrets("").Controller().Lister(),
		restConfigGetter: restConfigGetter,
		authClientCreator: func(clusterID string) (kubernetes.Interface, error) {
			return scaledContext.Wrangler.MultiClusterManager.K8sClient(clusterID)
		},
		clusterProxyConfigsGetter: scaledContext.Wrangler.Mgmt.ClusterProxyConfig().Cache(),
	}
}

// Authenticate the request using the downstream service account's JWT.
func (t *ServiceAccountAuth) Authenticate(req *http.Request) (user.Info, bool, error) {
	// Basic checks to see if we even need to proceed
	info, hasAuth := request.UserFrom(req.Context())
	if info.GetName() != "system:cattle:error" {
		return info, hasAuth, nil
	}
	clusterID := mux.Vars(req)["clusterID"]
	if clusterID == "" {
		return info, hasAuth, fmt.Errorf("no clusterID found in request")
	}
	// Check the cluster setting value to determine whether we will continue the auth process
	settings, err := t.clusterProxyConfigsGetter.List(clusterID, labels.NewSelector())
	if err != nil {
		logrus.Debugf("rejecting downstream proxy request for %s, unable to fetch ClusterProxySettings object for cluster", req.URL.Path)
		return info, false, nil
	}
	if settings == nil {
		logrus.Debugf("rejecting downstream proxy request for %s, no ClusterProxySettings object exists for cluster", req.URL.Path)
		return info, false, nil
	}
	if len(settings) > 1 {
		logrus.Errorf("multiple clusterproxyconfigs found for cluster, which is a misconfiguration, feature is disabled")
		return info, false, nil
	}

	if !settings[0].Enabled {
		logrus.Debugf("rejecting downstream proxy request for %s, current setting is enabled: %v", req.URL.Path, settings[0].Enabled)
		return info, false, nil
	}

	// See if the token is a JWT.
	rawToken := tokens.GetTokenAuthFromRequest(req)

	jwtParser := jwtv4.NewParser(jwtv4.WithoutClaimsValidation())
	claims := jwtv4.RegisteredClaims{}
	// Using ParseUnverified is deliberate here to look at the basic info in the token.
	// Later on, we do a real TokenReview against the downstream cluster to actually verify the JWT.
	_, _, err = jwtParser.ParseUnverified(rawToken, &claims)
	if err != nil {
		logrus.Debug("saauth: error parsing JWT")
		return info, false, err
	}

	if !strings.HasPrefix(claims.Subject, serviceaccount.ServiceAccountUsernamePrefix) {
		logrus.Debugf("saauth: JWT sub is not a service account: %v", err)
		return info, false, nil
	}

	if isTokenExpired(claims.ExpiresAt) {
		logrus.Debugf("saauth: Service Account JWT is expired. Expiration time was: %v", claims.ExpiresAt)
		return info, false, nil
	}

	// Get a client for the downstream cluster.
	downstreamAuthClient, err := t.authClientCreator(clusterID)
	if err != nil {
		logrus.Errorf("saauth: failed to fetch downstream kubeconfig: %v", err)
		return info, false, fmt.Errorf("failed to get downstream auth client when validating token for downstream cluster: %s", clusterID)
	}

	tokenReview := &v1.TokenReview{
		Spec: v1.TokenReviewSpec{
			Token: rawToken,
		},
	}

	// Make the token review request to the downstream cluster.
	tokenReview, err = downstreamAuthClient.AuthenticationV1().TokenReviews().Create(req.Context(), tokenReview, metav1.CreateOptions{})
	if err != nil {
		logrus.Debugf("saauth: error creating a tokenreview request: %v", err)
		return info, false, nil
	}

	// Let others know that this request is authenticated using a service account.
	*req = *req.WithContext(authcontext.SetSAAuthenticated(req.Context()))

	extraMap := convertExtra(tokenReview.Status.User.Extra)

	return &user.DefaultInfo{
		Name:   tokenReview.Status.User.Username,
		UID:    tokenReview.Status.User.UID,
		Groups: tokenReview.Status.User.Groups,
		Extra:  extraMap,
	}, tokenReview.Status.Authenticated, nil
}

// isTokenExpired takes the expiration time from a JWT and returns true if it is expired, otherwise it returns false.
func isTokenExpired(expirationTime *jwtv4.NumericDate) bool {
	if expirationTime == nil {
		// Token does not have an expiration time, so it is not expired.
		return false
	}

	currentTime := jwtv4.TimeFunc()
	return currentTime.After(expirationTime.Time)
}

// convertExtra converts the Extra value from a tokenResponse to map[string][]string
func convertExtra(extra map[string]v1.ExtraValue) map[string][]string {
	result := make(map[string][]string)
	for key, values := range extra {
		result[key] = values
	}
	return result
}
