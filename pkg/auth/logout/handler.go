package logout

import (
	"context"
	"net/http"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/sirupsen/logrus"
)

var cookieUnsetTimestamp = time.Date(1982, time.February, 10, 23, 0, 0, 0, time.UTC)

type tokenManager interface {
	GetToken(token string) (*v3.Token, int, error)
	DeleteTokenByName(name string) (int, error)
}

type handler struct {
	tokenMgr  tokenManager
	logout    func(http.ResponseWriter, *http.Request, accessor.TokenAccessor) error
	logoutAll func(http.ResponseWriter, *http.Request, accessor.TokenAccessor) error
}

func NewHandler(ctx context.Context, tokenMgr tokenManager) http.Handler {
	return &handler{
		tokenMgr:  tokenMgr,
		logoutAll: providers.ProviderLogoutAll,
		logout:    providers.ProviderLogout,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tokenAuthValue := tokens.GetTokenAuthFromRequest(r)
	if tokenAuthValue == "" {
		http.Error(w, "No valid token cookie or auth header", http.StatusUnauthorized)
		return
	}

	isSecure := r.URL.Scheme == "https"

	for _, cookieName := range []string{tokens.CookieName, tokens.CSRFCookie, tokens.IDTokenCookieName} {
		tokenCookie := &http.Cookie{
			Name:     cookieName,
			Value:    "",
			Secure:   isSecure,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
			Expires:  cookieUnsetTimestamp,
		}
		http.SetCookie(w, tokenCookie)
	}
	w.Header().Add("Content-type", "application/json")

	storedToken, status, err := h.tokenMgr.GetToken(tokenAuthValue)
	if err != nil {
		logrus.Errorf("logout: getting token: %v", err)

		if status == http.StatusNotFound {
			status = http.StatusInternalServerError
			http.Error(w, http.StatusText(status), status)
			return
		} else if status != http.StatusGone {
			http.Error(w, http.StatusText(status), status)
			return
		}
	}

	isLogoutAll := r.URL.Query().Has("all") || r.URL.Query().Get("action") == "logoutAll"

	if isLogoutAll {
		err = h.logoutAll(w, r, storedToken)
	} else {
		err = h.logout(w, r, storedToken)
	}
	if err != nil {
		logrus.Errorf("logout: provider logout: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if status == http.StatusGone {
		return
	}

	_, err = h.tokenMgr.DeleteTokenByName(storedToken.Name)
	if err != nil { // NotFound is already handled by DeleteTokenByName.
		logrus.Errorf("logout: deleting session token %s: %v", storedToken.Name, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
