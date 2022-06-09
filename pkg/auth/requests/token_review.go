package requests

import (
	"net/http"

	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/steve/pkg/auth"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	authv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
)

type TokenReviewAuth struct {
	AuthClient authv1.AuthenticationV1Interface
}

func NewTokenReviewAuth(authClient authv1.AuthenticationV1Interface) auth.Authenticator {
	return &TokenReviewAuth{
		AuthClient: authClient,
	}
}

// Authenticate attempts to authenticate using given function. If authentication
// fails AND the endpoint is in allowedPaths, it will attempt to perform a token
// review to authenticate token and extract user info.
func (t *TokenReviewAuth) Authenticate(req *http.Request) (user.Info, bool, error) {
	info, hasAuth := request.UserFrom(req.Context())
	if info.GetName() != "system:cattle:error" {
		// auth has succeeded
		return info, hasAuth, nil
	}

	tokenReview := &v1.TokenReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "authentication.k8s.io/v1",
			Kind:       "TokenReview",
		},
		Spec: v1.TokenReviewSpec{
			Token: tokens.GetTokenAuthFromRequest(req),
		},
	}

	tokenReview, err := t.AuthClient.TokenReviews().Create(req.Context(), tokenReview, metav1.CreateOptions{})
	if err != nil {
		logrus.Debugf("tokenReview failed: %v", err)
		return info, false, nil
	}

	tokenReviewUserInfo := &user.DefaultInfo{
		Name:   tokenReview.Status.User.Username,
		UID:    tokenReview.Status.User.UID,
		Groups: tokenReview.Status.User.Groups,
	}

	return tokenReviewUserInfo, true, nil
}
