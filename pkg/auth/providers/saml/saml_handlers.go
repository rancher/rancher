package saml

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"
)

const rancherUserID = "rancherUserID"

// ServeHTTP is the handler for /saml/metadata and /saml/acs endpoints
func (s *Provider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serviceProvider := s.serviceProvider

	log.Debugf("SAML [ServeHTTP]: Received %q %q", r.Method, r.URL)

	r.ParseForm()

	if r.URL.Path == s.metadataURL.Path {
		metadata, err := serviceProvider.MetadataWithSLO(0)
		if err != nil {
			log.Errorf("SAML [ServeHTTP]: error generating metadata: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		buf, _ := xml.MarshalIndent(metadata, "", "  ")
		w.Header().Set("Content-Type", "application/samlmetadata+xml")
		w.Write(buf)

		log.Debugf("SAML [ServeHTTP]: Returned meta data")
		return
	}

	if r.URL.Path == s.acsURL.Path {
		log.Debugf("SAML [ServeHTTP]: assertion processing started")

		r.ParseForm()
		assertionInfo, err := serviceProvider.RetrieveAssertionInfo(r.FormValue("SAMLResponse"))

		if err != nil {
			log.Debugf("SAML [ServeHTTP]: assertion validation failed: %q", err)

			redirectURL := r.URL.Host + "/login?errorCode=403"
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		log.Debugf("SAML [ServeHTTP]: assertions validated ok")

		s.HandleSamlAssertion(w, r, assertionInfo)

		log.Debugf("SAML [ServeHTTP]: assertion processing completed")
		return
	}

	if r.URL.Path == s.sloURL.Path {
		log.Debugf("SAML [ServeHTTP]: logout response processing started")

		s.FinalizeSamlLogout(w, r)

		log.Debugf("SAML [ServeHTTP]: logout response processing completed")
		return
	}

	log.Debugf("SAML [ServeHTTP]: Failed to handle %s %s", r.Method, r.URL)

	http.NotFoundHandler().ServeHTTP(w, r)
}

// HandleSamlLogin is the endpoint for /saml/login endpoint
func (s *Provider) HandleSamlLogin(w http.ResponseWriter, r *http.Request, userID string) (string, error) {
	serviceProvider := s.serviceProvider
	if r.URL.Path == s.acsURL.Path {
		return "", fmt.Errorf("don't wrap Middleware with RequireAccount")
	}

	// relayState is limited to 80 bytes but also must be integrity protected.
	// this means that we cannot use a JWT because it is way too long. Instead
	// we set a cookie that corresponds to the state
	relayState := base64.URLEncoding.EncodeToString(randomBytes(42))

	secretBlock := x509.MarshalPKCS1PrivateKey(s.spKey)
	state := jwt.New(jwt.SigningMethodHS256)
	claims := state.Claims.(jwt.MapClaims)
	claims["uri"] = r.URL.String()
	if userID != "" {
		claims[rancherUserID] = userID
	}

	signedState, err := state.SignedString(secretBlock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return "", err
	}

	s.clientState.SetState(w, r, relayState, signedState)

	// Use redirect binding: signature goes in the URL query string.
	doc, err := serviceProvider.BuildAuthRequestDocumentNoSig()
	if err != nil {
		return "", err
	}
	redirectURL, err := serviceProvider.BuildAuthURLRedirect(relayState, doc)
	if err != nil {
		return "", err
	}

	return redirectURL, nil
}

// HandleSamlLogout is the final handler for the logoutAll action
func (s *Provider) HandleSamlLogout(name string, w http.ResponseWriter, r *http.Request) (string, error) {
	serviceProvider := s.serviceProvider
	if r.URL.Path == s.acsURL.Path {
		return "", fmt.Errorf("don't wrap Middleware with RequireAccount")
	}

	// relayState is limited to 80 bytes but also must be integrity protected.
	// this means that we cannot use a JWT because it is way too long. Instead
	// we set a cookie that corresponds to the state
	relayState := base64.URLEncoding.EncodeToString(randomBytes(42))

	secretBlock := x509.MarshalPKCS1PrivateKey(s.spKey)
	state := jwt.New(jwt.SigningMethodHS256)
	claims := state.Claims.(jwt.MapClaims)
	claims["uri"] = r.URL.String()

	signedState, err := state.SignedString(secretBlock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return "", err
	}

	s.clientState.SetState(w, r, relayState, signedState)

	// name is the NameID of the user at the IdP. Session index is not tracked
	// so an empty string is passed.
	doc, err := serviceProvider.BuildLogoutRequestDocumentNoSig(name, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return "", err
	}
	redirectURL, err := serviceProvider.BuildLogoutURLRedirect(relayState, doc)
	if err != nil {
		return "", err
	}

	return redirectURL, nil
}

func randomBytes(n int) []byte {
	rv := make([]byte, n)
	if _, err := rand.Read(rv); err != nil {
		panic(err)
	}
	return rv
}
