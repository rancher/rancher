package saml

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"github.com/crewjam/saml"
	"github.com/golang-jwt/jwt"
	log "github.com/sirupsen/logrus"
)

const rancherUserID = "rancherUserID"

// ServeHTTP is the handler for /saml/metadata and /saml/acs endpoints
func (s *Provider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serviceProvider := s.serviceProvider

	log.Debugf("SAML [ServeHTTP]: Received %q %q", r.Method, r.URL)

	r.ParseForm()

	if r.URL.Path == serviceProvider.MetadataURL.Path {
		buf, _ := xml.MarshalIndent(serviceProvider.Metadata(), "", "  ")
		w.Header().Set("Content-Type", "application/samlmetadata+xml")
		w.Write(buf)

		log.Debugf("SAML [ServeHTTP]: Returned meta data")
		return
	}

	if r.URL.Path == serviceProvider.AcsURL.Path {
		log.Debugf("SAML [ServeHTTP]: assertion processing started")

		r.ParseForm()
		assertion, err := serviceProvider.ParseResponse(r, s.getPossibleRequestIDs(r))

		if err != nil {
			log.Debugf("SAML [ServeHTTP]: assertion validation failed: %q", err)

			if parseErr, ok := err.(*saml.InvalidResponseError); ok {
				// Note: If access to the response itself is needed (debugging)
				// just add `parseErr.Response` to the log statement.

				log.Debugf("SAML NOW: %s\nSAML ERROR: %s",
					parseErr.Now, parseErr.PrivateErr)
			}

			redirectURL := r.URL.Host + "/login?errorCode=403"
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		log.Debugf("SAML [ServeHTTP]: assertions validated ok")

		s.HandleSamlAssertion(w, r, assertion)

		log.Debugf("SAML [ServeHTTP]: assertion processing completed")
		return
	}

	if r.URL.Path == serviceProvider.SloURL.Path {
		log.Debugf("SAML [ServeHTTP]: logout response processing started")

		s.FinalizeSamlLogout(w, r)

		log.Debugf("SAML [ServeHTTP]: logout response processing completed")
		return
	}

	log.Debugf("SAML [ServeHTTP]: Failed to handle %s %s", r.Method, r.URL)

	http.NotFoundHandler().ServeHTTP(w, r)
}

func (s *Provider) getPossibleRequestIDs(r *http.Request) []string {
	rv := []string{}
	serviceProvider := s.serviceProvider

	for name, value := range s.clientState.GetStates(r) {
		if strings.HasPrefix(name, "Rancher_") {
			continue
		}
		jwtParser := jwt.Parser{
			ValidMethods: []string{jwt.SigningMethodHS256.Name},
		}
		token, err := jwtParser.Parse(value, func(t *jwt.Token) (interface{}, error) {
			secretBlock := x509.MarshalPKCS1PrivateKey(serviceProvider.Key)
			return secretBlock, nil
		})
		if err != nil || !token.Valid {
			log.Debugf("... invalid token %s", err)
			continue
		}
		claims := token.Claims.(jwt.MapClaims)
		rv = append(rv, claims["id"].(string))
	}
	return rv
}

// HandleSamlLogin is the endpoint for /saml/login endpoint
func (s *Provider) HandleSamlLogin(w http.ResponseWriter, r *http.Request, userID string) (string, error) {
	serviceProvider := s.serviceProvider
	if r.URL.Path == serviceProvider.AcsURL.Path {
		return "", fmt.Errorf("don't wrap Middleware with RequireAccount")
	}

	binding := saml.HTTPRedirectBinding
	bindingLocation := serviceProvider.GetSSOBindingLocation(binding)

	req, err := serviceProvider.MakeAuthenticationRequest(bindingLocation, binding, saml.HTTPPostBinding)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return "", err
	}
	// relayState is limited to 80 bytes but also must be integrity protected.
	// this means that we cannot use a JWT because it is way too long. Instead
	// we set a cookie that corresponds to the state
	relayState := base64.URLEncoding.EncodeToString(randomBytes(42))

	secretBlock := x509.MarshalPKCS1PrivateKey(serviceProvider.Key)
	state := jwt.New(jwt.SigningMethodHS256)
	claims := state.Claims.(jwt.MapClaims)
	claims["id"] = req.ID
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

	redirectURL, err := req.Redirect(relayState, serviceProvider)
	if err != nil {
		return "", err
	}

	return redirectURL.String(), nil
}

// HandleSamlLogout is the final handler for the logoutAll action
func (s *Provider) HandleSamlLogout(name string, w http.ResponseWriter, r *http.Request) (string, error) {
	serviceProvider := s.serviceProvider
	if r.URL.Path == serviceProvider.AcsURL.Path {
		return "", fmt.Errorf("don't wrap Middleware with RequireAccount")
	}

	binding := saml.HTTPRedirectBinding
	bindingLocation := serviceProvider.GetSLOBindingLocation(binding)

	req, err := serviceProvider.MakeLogoutRequest(bindingLocation, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return "", err
	}
	// relayState is limited to 80 bytes but also must be integrity protected.
	// this means that we cannot use a JWT because it is way too long. Instead
	// we set a cookie that corresponds to the state
	relayState := base64.URLEncoding.EncodeToString(randomBytes(42))

	secretBlock := x509.MarshalPKCS1PrivateKey(serviceProvider.Key)
	state := jwt.New(jwt.SigningMethodHS256)
	claims := state.Claims.(jwt.MapClaims)
	claims["id"] = req.ID
	claims["uri"] = r.URL.String()

	signedState, err := state.SignedString(secretBlock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return "", err
	}

	s.clientState.SetState(w, r, relayState, signedState)

	redirectURL, err := req.Redirect(relayState, serviceProvider)
	if err != nil {
		return "", err
	}

	return redirectURL.String(), nil
}

func randomBytes(n int) []byte {
	rv := make([]byte, n)
	if _, err := saml.RandReader.Read(rv); err != nil {
		panic(err)
	}
	return rv
}
