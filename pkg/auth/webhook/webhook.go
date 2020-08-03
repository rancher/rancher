package webhook

import (
	"encoding/json"
	"net/http"

	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/endpoints/request"
)

type TokenReviewer struct {
	ExternalIDs bool
}

func (t *TokenReviewer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	dec := json.NewDecoder(req.Body)
	tr := &v1.TokenReview{}
	err := dec.Decode(tr)
	if err != nil {
		handleErr(rw, err)
		return
	}

	req = &http.Request{
		Header: map[string][]string{},
	}

	userInfo, ok := request.UserFrom(req.Context())
	if !ok {
		handleErr(rw, requests.ErrMustAuthenticate)
		return
	}

	user := v1.UserInfo{
		UID:      userInfo.GetUID(),
		Username: userInfo.GetName(),
		Groups:   userInfo.GetGroups(),
	}
	for k, v := range userInfo.GetExtra() {
		if user.Extra == nil {
			user.Extra = map[string]v1.ExtraValue{}
		}
		user.Extra[k] = v
	}

	trResp := &v1.TokenReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "authentication.k8s.io/v1",
			Kind:       "TokenReview",
		},
		Status: v1.TokenReviewStatus{
			Authenticated: ok,
			User:          user,
		},
	}

	writeResp(rw, trResp)
}

func writeResp(rw http.ResponseWriter, tr *v1.TokenReview) {
	rw.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(rw)
	err := enc.Encode(tr)
	if err != nil {
		logrus.Infof("Failed to encode token review response")
	}
}

func handleErr(rw http.ResponseWriter, err error) {
	writeResp(rw, &v1.TokenReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "authentication.k8s.io/v1",
			Kind:       "TokenReview",
		},
		Status: v1.TokenReviewStatus{
			Error: err.Error(),
		},
	})
}
