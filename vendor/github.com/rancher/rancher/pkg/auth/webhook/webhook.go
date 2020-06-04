package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/rancher/pkg/auth/requests"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TokenReviewer struct {
	Authenticator requests.Authenticator
	ExternalIDs   bool
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

	if strings.HasPrefix(tr.Spec.Token, "cookie://") {
		req.Header.Set("Cookie", fmt.Sprintf("R_SESS=%s", strings.TrimPrefix(tr.Spec.Token, "cookie://")))
	} else {
		req.Header.Set("Authorization", "Bearer "+tr.Spec.Token)
	}

	ok, user, groups, err := t.Authenticator.Authenticate(req)
	if err != nil {
		handleErr(rw, err)
		return
	}

	trResp := &v1.TokenReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "authentication.k8s.io/v1",
			Kind:       "TokenReview",
		},
		Status: v1.TokenReviewStatus{
			Authenticated: ok,
			User: v1.UserInfo{
				UID:      user,
				Username: user,
				Groups:   groups,
			},
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
