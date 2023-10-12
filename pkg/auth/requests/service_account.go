package requests

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/clustermanager"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/steve/pkg/auth"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	authv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
)

type DownstreamTokenReviewAuth struct {
	scaledContext *config.ScaledContext
	clusterLister v3.ClusterLister
	secretLister  corev1.SecretLister
}

func NewDownstreamTokenReviewAuth(scaledContext *config.ScaledContext) auth.Authenticator {
	return &DownstreamTokenReviewAuth{
		scaledContext: scaledContext,
		clusterLister: scaledContext.Management.Clusters("").Controller().Lister(),
		secretLister:  scaledContext.Core.Secrets("").Controller().Lister(),
	}
}

// Authenticate ...
func (t *DownstreamTokenReviewAuth) Authenticate(req *http.Request) (user.Info, bool, error) {
	info, hasAuth := request.UserFrom(req.Context())
	if info.GetName() != "system:cattle:error" {
		return info, hasAuth, nil
	}

	rawToken := tokens.GetTokenAuthFromRequest(req)

	jwtParser := jwt.Parser{}
	claims := jwt.StandardClaims{}
	_, _, err := jwtParser.ParseUnverified(rawToken, &claims)
	if err != nil {
		return info, false, err
	}

	if !strings.HasPrefix(claims.Subject, "system:serviceaccount:") {
		return info, false, nil
	}

	clusterID := mux.Vars(req)["clusterID"]
	if clusterID == "" {
		return info, hasAuth, nil
	}

	cluster, err := t.clusterLister.Get("", clusterID)
	if err != nil {
		return info, false, err
	}

	kubeConfig, err := clustermanager.ToRESTConfig(cluster, t.scaledContext, t.secretLister)
	if kubeConfig == nil || err != nil {
		return info, false, err
	}

	authClient, err := authv1.NewForConfig(kubeConfig)
	if err != nil {
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

	tokenReview, err = authClient.TokenReviews().Create(req.Context(), tokenReview, metav1.CreateOptions{})
	if err != nil {
		logrus.Debugf("tokenReview failed: %v", err)
		return info, false, nil
	}

	return &user.DefaultInfo{
		Name:   tokenReview.Status.User.Username,
		UID:    tokenReview.Status.User.UID,
		Groups: tokenReview.Status.User.Groups,
		Extra: map[string][]string{
			"sa-auth": nil,
		},
	}, true, nil
}
