package saml

import (
	"net/http"
	"strings"
	"time"

	"github.com/crewjam/saml"
)

// ClientState implements client side storage for state.
type ClientState interface {
	SetState(w http.ResponseWriter, r *http.Request, id string, value string)
	GetStates(r *http.Request) map[string]string
	GetState(r *http.Request, id string) string
	DeleteState(w http.ResponseWriter, r *http.Request, id string) error
}

// ClientToken implements client side storage for signed authorization tokens.
type ClientToken interface {
	GetToken(r *http.Request) string
	SetToken(w http.ResponseWriter, r *http.Request, value string, maxAge time.Duration)
}

const stateCookiePrefix = "saml_"

type ClientCookies struct {
	ServiceProvider *saml.ServiceProvider
	Name            string
	Domain          string
	Secure          bool
}

// SetState stores the named state value by setting a cookie.
func (c ClientCookies) SetState(w http.ResponseWriter, r *http.Request, id string, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookiePrefix + id,
		Value:    value,
		MaxAge:   int(saml.MaxIssueDelay.Seconds()),
		HttpOnly: true,
		Secure:   c.Secure || r.URL.Scheme == "https",
		Path:     c.ServiceProvider.AcsURL.Path,
	})
}

// GetStates returns the currently stored states by reading cookies.
func (c ClientCookies) GetStates(r *http.Request) map[string]string {
	rv := map[string]string{}
	for _, cookie := range r.Cookies() {
		if !strings.HasPrefix(cookie.Name, stateCookiePrefix) {
			continue
		}
		name := strings.TrimPrefix(cookie.Name, stateCookiePrefix)
		rv[name] = cookie.Value
	}
	return rv
}

// GetState returns a single stored state by reading the cookies
func (c ClientCookies) GetState(r *http.Request, id string) string {
	stateCookie, err := r.Cookie(stateCookiePrefix + id)
	if err != nil {
		return ""
	}
	return stateCookie.Value
}

// DeleteState removes the named stored state by clearing the corresponding cookie.
func (c ClientCookies) DeleteState(w http.ResponseWriter, r *http.Request, id string) error {
	cookie, err := r.Cookie(stateCookiePrefix + id)
	if err != nil {
		return err
	}
	cookie.Value = ""
	cookie.Expires = time.Unix(1, 0) // past time as close to epoch as possible, but not zero time.Time{}
	http.SetCookie(w, cookie)
	return nil
}

// SetToken assigns the specified token by setting a cookie.
func (c ClientCookies) SetToken(w http.ResponseWriter, r *http.Request, value string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     c.Name,
		Domain:   c.Domain,
		Value:    value,
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   c.Secure || r.URL.Scheme == "https",
		Path:     "/",
	})
}

// GetToken returns the token by reading the cookie.
func (c ClientCookies) GetToken(r *http.Request) string {
	cookie, err := r.Cookie(c.Name)
	if err != nil {
		return ""
	}
	return cookie.Value
}
