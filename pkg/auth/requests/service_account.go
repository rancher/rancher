package requests

import (
	"fmt"
	"net/http"
	"strings"

	jwtv4 "github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	authcontext "github.com/rancher/rancher/pkg/auth/context"
	"github.com/rancher/rancher/pkg/auth/tokens"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/steve/pkg/auth"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	authv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	"k8s.io/client-go/rest"
)

const serviceAccountSubjectPrefix = "system:serviceaccount:"

type (
	restConfigGetter  func(cluster *mgmtv3.Cluster, context *config.ScaledContext, secretLister corev1.SecretLister) (*rest.Config, error)
	authClientCreator func(config *rest.Config) (authv1.AuthenticationV1Interface, error)
)

// ServiceAccountAuth is an authenticator that authenticates requests using the downstream service account's JWT.
type ServiceAccountAuth struct {
	scaledContext     *config.ScaledContext
	clusterLister     mgmtv3.ClusterLister
	secretLister      corev1.SecretLister
	restConfigGetter  restConfigGetter
	authClientCreator authClientCreator
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
		authClientCreator: func(config *rest.Config) (authv1.AuthenticationV1Interface, error) {
			return authv1.NewForConfig(config)
		},
	}
}

// Authenticate the request using the downstream service account's JWT.
func (t *ServiceAccountAuth) Authenticate(req *http.Request) (user.Info, bool, error) {
	info, hasAuth := request.UserFrom(req.Context())
	if info.GetName() != "system:cattle:error" {
		return info, hasAuth, nil
	}

	// See if the token is a JWT.
	rawToken := tokens.GetTokenAuthFromRequest(req)

	jwtParser := jwtv4.Parser{}
	claims := jwtv4.RegisteredClaims{}
	_, _, err := jwtParser.ParseUnverified(rawToken, &claims)
	if err != nil {
		logrus.Debugf("saauth: error parsing JWT: %v", err)
		return info, false, err
	}

	if !strings.HasPrefix(claims.Subject, serviceAccountSubjectPrefix) {
		logrus.Debugf("saauth: JWT sub is not a service account: %v", err)
		return info, false, nil
	}

	if isTokenExpired(claims.ExpiresAt) {
		logrus.Debugf("saauth: Service Account JWT is expired. Expiration time was: %v", claims.ExpiresAt)
		return info, false, nil
	}

	// Make sure the cluster exists.
	clusterID := mux.Vars(req)["clusterID"]
	if clusterID == "" {
		return info, hasAuth, fmt.Errorf("no clusterID found in request")
	}

	cluster, err := t.clusterLister.Get("", clusterID)
	if err != nil {
		logrus.Debugf("saauth: error getting cluster: %v", err)
		return info, false, err
	}

	// Get rest config for the cluster and instantiate an authentication client.
	kubeConfig, err := t.restConfigGetter(cluster, t.scaledContext, t.secretLister)
	if kubeConfig == nil || err != nil {
		return info, false, err
	}

	authClient, err := t.authClientCreator(kubeConfig)
	if err != nil {
		logrus.Debugf("saauth: error creating authentication client: %v", err)
		return info, false, err
	}

	tokenReview := &v1.TokenReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "authentication.k8s.io/v1",
			Kind:       "TokenReview",
		},
		Spec: v1.TokenReviewSpec{
			Token: rawToken,
		},
	}

	// Make the token review request to the downstream cluster.
	tokenReview, err = authClient.TokenReviews().Create(req.Context(), tokenReview, metav1.CreateOptions{})
	if err != nil {
		logrus.Debugf("saauth: error creating a tokenreview request: %v", err)
		return info, false, nil
	}

	// Let others know that this request is authenticated using a service account.
	*req = *req.WithContext(authcontext.SetSAAuthenticated(req.Context()))

	return &user.DefaultInfo{
		Name:   tokenReview.Status.User.Username,
		UID:    tokenReview.Status.User.UID,
		Groups: tokenReview.Status.User.Groups,
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
