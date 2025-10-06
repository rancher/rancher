package logout

import (
	"context"
	"net/http"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/sirupsen/logrus"
)

func NewHandler(ctx context.Context, tokenMgr tokenManager) http.Handler {
	return &handler{
		tokenMgr: tokenMgr,
	}
}

type tokenManager interface {
	GetToken(token string) (*v3.Token, int, error)
	DeleteTokenByName(name string) (int, error)
}

type handler struct {
	tokenMgr tokenManager
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tokenAuthValue := tokens.GetTokenAuthFromRequest(r)
	if tokenAuthValue == "" {
		http.Error(w, "No valid token cookie or auth header", http.StatusUnauthorized)
		return
	}

	isSecure := false
	if r.URL.Scheme == "https" {
		isSecure = true
	}

	for _, cookieName := range []string{tokens.CookieName, tokens.CSRFCookie} {
		tokenCookie := &http.Cookie{
			Name:     cookieName,
			Value:    "",
			Secure:   isSecure,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
			Expires:  time.Date(1982, time.February, 10, 23, 0, 0, 0, time.UTC),
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

	isLogoutAll := r.URL.Query().Has("all") || r.URL.Query().Has("logoutAll")

	if isLogoutAll {
		err = providers.ProviderLogoutAll(w, r, storedToken)
	} else {
		err = providers.ProviderLogout(w, r, storedToken)
	}
	if err != nil {
		logrus.Errorf("logout: provider logout: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	status, err = h.tokenMgr.DeleteTokenByName(storedToken.Name)
	if err != nil {
		logrus.Errorf("logout: deleting session token %s: %v", storedToken.Name, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
