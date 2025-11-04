package logout

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type fakeTokenManager struct {
	getTokenFunc          func(token string) (*v3.Token, int, error)
	deleteTokenByNameFunc func(name string) (int, error)
}

func (m *fakeTokenManager) GetToken(token string) (*v3.Token, int, error) {
	if m.getTokenFunc == nil {
		return nil, http.StatusNotFound, nil
	}
	return m.getTokenFunc(token)
}
func (m *fakeTokenManager) DeleteTokenByName(name string) (int, error) {
	if m.deleteTokenByNameFunc == nil {
		return http.StatusNotFound, nil
	}
	return m.deleteTokenByNameFunc(name)
}

func TestLogout(t *testing.T) {
	tokenID := "token-5lwps"
	bearerToken := tokenID + ":jslbp8qbkvpndjj4xmvl9crwh7w96pvxrg4xltsmcbcvvcrk9thphq"

	tokenManager := &fakeTokenManager{
		getTokenFunc: func(token string) (*v3.Token, int, error) {
			assert.Equal(t, bearerToken, token)
			return &v3.Token{
				ObjectMeta: metav1.ObjectMeta{
					Name: tokenID,
				},
			}, 0, nil
		},
		deleteTokenByNameFunc: func(name string) (int, error) {
			assert.Equal(t, tokenID, name)
			return http.StatusOK, nil
		},
	}

	var logoutCalled bool
	logoutFunc := func(w http.ResponseWriter, r *http.Request, accessor accessor.TokenAccessor) error {
		logoutCalled = true
		return nil
	}

	checkCookiesUnset := func(t *testing.T, w *httptest.ResponseRecorder) {
		require.Len(t, w.Result().Cookies(), 3)
		for _, cookie := range w.Result().Cookies() {
			switch cookie.Name {
			case tokens.CookieName, tokens.CSRFCookie, tokens.IDTokenCookieName:
				assert.Equal(t, "", cookie.Value)
				assert.Equal(t, -1, cookie.MaxAge)
				assert.True(t, cookie.HttpOnly)
				assert.Equal(t, "/", cookie.Path)
				assert.Equal(t, cookieUnsetTimestamp, cookie.Expires)
			default:
				require.FailNow(t, "unexpected cookie "+cookie.Name)
			}
		}
	}
	checkSuccessResponse := func(t *testing.T, w *httptest.ResponseRecorder) {
		assert.True(t, logoutCalled)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		checkCookiesUnset(t, w)
	}

	authRequest := func(path string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, path, nil)
		r.AddCookie(&http.Cookie{
			Name:  tokens.CookieName,
			Value: bearerToken,
		})
		return r
	}

	t.Run("standard logout", func(t *testing.T) {
		h := &handler{
			tokenMgr: tokenManager,
			logout:   logoutFunc,
		}
		r := authRequest("/v1/logout")
		w := httptest.NewRecorder()
		logoutCalled = false

		h.ServeHTTP(w, r)

		checkSuccessResponse(t, w)
	})

	t.Run("logout all", func(t *testing.T) {
		h := &handler{
			tokenMgr:  tokenManager,
			logoutAll: logoutFunc,
		}

		for _, url := range []string{"/v1/logout?all", "/v3/tokens?action=logoutAll"} {
			r := authRequest(url)
			w := httptest.NewRecorder()
			logoutCalled = false

			h.ServeHTTP(w, r)

			checkSuccessResponse(t, w)
		}
	})

	t.Run("no session cookie", func(t *testing.T) {
		h := &handler{
			tokenMgr: tokenManager,
			logout:   logoutFunc,
		}
		r := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
		w := httptest.NewRecorder()
		logoutCalled = false

		h.ServeHTTP(w, r)

		assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
		assert.False(t, logoutCalled)
	})

	t.Run("get token returns status that isn't ok", func(t *testing.T) {
		statusMap := map[int]int{
			http.StatusNotFound:            http.StatusInternalServerError,
			http.StatusUnauthorized:        http.StatusUnauthorized,
			http.StatusUnprocessableEntity: http.StatusUnprocessableEntity,
			http.StatusGone:                http.StatusOK,
		}
		for statusIn, statusOut := range statusMap {
			tokenManager := &fakeTokenManager{
				getTokenFunc: func(token string) (*v3.Token, int, error) {
					assert.Equal(t, bearerToken, token)
					return nil, statusIn, errors.New(http.StatusText(statusIn))
				},
				deleteTokenByNameFunc: func(name string) (int, error) {
					assert.Equal(t, tokenID, name)
					return http.StatusOK, nil
				},
			}
			h := &handler{
				tokenMgr: tokenManager,
				logout:   logoutFunc,
			}
			r := authRequest("/v1/logout")
			w := httptest.NewRecorder()
			logoutCalled = false

			h.ServeHTTP(w, r)

			assert.Equal(t, statusOut, w.Result().StatusCode)
			checkCookiesUnset(t, w)
			assert.Equal(t, statusOut == http.StatusOK, logoutCalled)
		}
	})

	t.Run("logout fails", func(t *testing.T) {
		h := &handler{
			tokenMgr: tokenManager,
			logout: func(w http.ResponseWriter, r *http.Request, accessor accessor.TokenAccessor) error {
				return errors.New("some error")
			},
		}
		r := authRequest("/v1/logout")
		w := httptest.NewRecorder()

		h.ServeHTTP(w, r)

		assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
	})

	t.Run("delete token fails", func(t *testing.T) {
		tokenManager := &fakeTokenManager{
			getTokenFunc: func(token string) (*v3.Token, int, error) {
				assert.Equal(t, bearerToken, token)
				return &v3.Token{
					ObjectMeta: metav1.ObjectMeta{
						Name: tokenID,
					},
				}, 0, nil
			},
			deleteTokenByNameFunc: func(name string) (int, error) {
				assert.Equal(t, tokenID, name)
				return http.StatusInternalServerError, errors.New("some error")
			},
		}

		h := &handler{
			tokenMgr: tokenManager,
			logout:   logoutFunc,
		}
		r := authRequest("/v1/logout")
		w := httptest.NewRecorder()
		logoutCalled = false

		h.ServeHTTP(w, r)

		assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
		checkCookiesUnset(t, w)
		assert.True(t, logoutCalled)
	})
}
