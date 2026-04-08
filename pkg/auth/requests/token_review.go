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

const errorUserName string = "system:cattle:error"

type TokenReviewAuth struct {
	AuthClient authv1.AuthenticationV1Interface
}

func NewTokenReviewAuth(authClient authv1.AuthenticationV1Interface) auth.Authenticator {
	return &TokenReviewAuth{
		AuthClient: authClient,
	}
}

// Authenticate relies on a previous authentication step to have set the user in
// the request context.
//
// If the user is not the error user, it returns the user info from the context.
// If the user is the error user, it performs a token review using the token
// extracted from the request and returns the reviewed user info. If the token
// review fails, it returns no user and no error.
func (t *TokenReviewAuth) Authenticate(req *http.Request) (user.Info, bool, error) {
	info, hasAuth := request.UserFrom(req.Context())
	if info.GetName() != errorUserName {
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

	return tokenReviewUserInfo, tokenReview.Status.Authenticated, nil
}
