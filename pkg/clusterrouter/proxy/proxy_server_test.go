package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	auth "github.com/rancher/rancher/pkg/auth/context"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestServeHTTP(t *testing.T) {
	tests := map[string]struct {
		tokenCreator                   tokenCreator
		impersonatorAccountTokenGetter impersonatorAccountTokenGetter
		header                         http.Header
		ctx                            func() context.Context
		wantHeader                     http.Header
		wantErr                        string
	}{
		"get impersonation token": {
			header: map[string][]string{},
			ctx: func() context.Context {
				return request.WithUser(context.TODO(), &user.DefaultInfo{
					Name: "user",
					UID:  "user",
				})
			},
			impersonatorAccountTokenGetter: func(_ user.Info, _ ClusterContextGetter, _ string) (string, error) {
				return "fake-token", nil
			},
			wantHeader: map[string][]string{
				"Authorization": {
					"Bearer fake-token",
				},
			},
			wantErr: "",
		},
		"impersonate sa": {
			tokenCreator: func(_ context.Context, _ ClusterContextGetter, _ string, _ string) (string, error) {
				return "fake-token", nil
			},
			header: map[string][]string{
				"Impersonate-User":  {"user"},
				"Impersonate-Group": {"group"},
			},
			ctx: func() context.Context {
				ctx := context.TODO()
				ctx = request.WithUser(ctx, &user.DefaultInfo{
					Name: "user",
					UID:  "user",
				})

				return auth.SetSAImpersonation(ctx, "system:serviceaccount:ns-test:sa")
			},
			wantHeader: map[string][]string{
				"Authorization": {
					"Bearer fake-token",
				},
			},
			wantErr: "",
		},
		"get impersonation token - error": {
			header: map[string][]string{},
			ctx: func() context.Context {
				ctx := context.TODO()
				return request.WithUser(ctx, &user.DefaultInfo{
					Name: "user",
					UID:  "user",
				})
			},
			impersonatorAccountTokenGetter: func(_ user.Info, _ ClusterContextGetter, _ string) (string, error) {
				return "", errors.New("can't create token")
			},
			wantHeader: map[string][]string{},
			wantErr:    "can't create token",
		},
		"impersonate sa - create token error": {
			tokenCreator: func(ctx context.Context, getter ClusterContextGetter, s string, s2 string) (string, error) {
				return "", errors.New("can't create token")
			},
			header: map[string][]string{},
			ctx: func() context.Context {
				ctx := context.TODO()
				ctx = request.WithUser(ctx, &user.DefaultInfo{
					Name: "user",
					UID:  "user",
				})

				return auth.SetSAImpersonation(ctx, "system:serviceaccount:ns-test:sa")
			},
			wantHeader: map[string][]string{},
			wantErr:    "can't create token",
		},
		"already authenticated with a serviceaccount token": {
			header: map[string][]string{},
			ctx: func() context.Context {
				ctx := context.TODO()
				ctx = request.WithUser(ctx, &user.DefaultInfo{
					Name: "user",
					UID:  "user",
				})

				return auth.SetSAAuthenticated(ctx)
			},
			wantHeader: map[string][]string{},
			wantErr:    "",
		},
		"user not authenticated": {
			header: map[string][]string{},
			ctx: func() context.Context {
				return context.TODO()
			},
			wantHeader: map[string][]string{},
			wantErr:    "Unauthorized 401",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := RemoteService{
				transport: func() (http.RoundTripper, error) {
					return http.DefaultTransport, nil
				},
				url: func() (url.URL, error) {
					return url.URL{}, nil
				},
				cluster: &v3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				tokenCreator:                   test.tokenCreator,
				impersonatorAccountTokenGetter: test.impersonatorAccountTokenGetter,
			}
			req := &http.Request{
				URL: &url.URL{
					Path: "http://localhost",
				},
				Header: test.header,
			}
			req = req.WithContext(test.ctx())
			rw := httptest.NewRecorder()

			r.ServeHTTP(rw, req)

			assert.Equal(t, test.wantHeader, req.Header)
			if test.wantErr != "" {
				bodyBytes, err := io.ReadAll(rw.Body)
				assert.NoError(t, err)
				assert.Contains(t, string(bodyBytes), test.wantErr)
			}
		})
	}

}
