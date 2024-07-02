package saml

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/crewjam/saml"
	"github.com/gorilla/mux"
	responsewriter "github.com/rancher/apiserver/pkg/middleware"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/settings"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IDPMetadata struct {
	XMLName           xml.Name                `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
	ValidUntil        time.Time               `xml:"validUntil,attr"`
	EntityID          string                  `xml:"entityID,attr"`
	IDPSSODescriptors []saml.IDPSSODescriptor `xml:"IDPSSODescriptor"`
	SPSSODescriptors  []saml.SPSSODescriptor  `xml:"SPSSODescriptor"`
}

var root *mux.Router
var appliedVersion string
var initMu sync.Mutex

const UITranslationKeyForErrorMessage = "invalidSamlAttrs"

func InitializeSamlServiceProvider(configToSet *v32.SamlConfig, name string) error {

	initMu.Lock()
	defer initMu.Unlock()

	if configToSet.ResourceVersion == appliedVersion {
		return nil
	}

	var privKey *rsa.PrivateKey
	var cert *x509.Certificate
	var err error
	var ok bool

	log.Debugf("SAML [InitializeSamlServiceProvider]: Validating input for provider %v", name)

	if configToSet.IDPMetadataContent == "" {
		return fmt.Errorf("SAML: Cannot initialize saml SP properly, missing IDP URL/metadata in the config %v", configToSet)
	}

	if configToSet.SpCert == "" {
		return fmt.Errorf("SAML: Cannot initialize saml SP properly, missing SpCert in the config %v", configToSet)
	}

	if configToSet.SpKey == "" {
		return fmt.Errorf("SAML: Cannot initialize saml SP properly, missing SpKey in the config %v", configToSet)
	}

	if configToSet.SpKey != "" {
		// used from ssh.ParseRawPrivateKey

		block, _ := pem.Decode([]byte(configToSet.SpKey))
		if block == nil {
			return fmt.Errorf("SAML: no key found")
		}

		if strings.Contains(block.Headers["Proc-Type"], "ENCRYPTED") {
			return fmt.Errorf("SAML: cannot decode encrypted private keys")
		}

		switch block.Type {
		case "RSA PRIVATE KEY":
			privKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return fmt.Errorf("SAML: error parsing PKCS1 RSA key: %v", err)
			}
		case "PRIVATE KEY":
			pk, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return fmt.Errorf("SAML: error parsing PKCS8 RSA key: %v", err)
			}
			privKey, ok = pk.(*rsa.PrivateKey)
			if !ok {
				return fmt.Errorf("SAML: unable to get rsa key")
			}
		default:
			return fmt.Errorf("SAML: unsupported key type %q", block.Type)
		}
	}

	if configToSet.SpCert != "" {
		block, _ := pem.Decode([]byte(configToSet.SpCert))
		if block == nil {
			return fmt.Errorf("SAML: failed to parse PEM block containing the private key")
		}

		cert, err = x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("SAML: failed to parse DER encoded public key: " + err.Error())
		}
	}

	provider, ok := SamlProviders[name]
	if !ok {
		return fmt.Errorf("SAML [InitializeSamlServiceProvider]: Provider %v not configured", name)
	}

	log.Debugf("SAML [InitializeSamlServiceProvider]: Initializing provider %v", name)

	rancherAPIHost := strings.TrimRight(configToSet.RancherAPIHost, "/")
	samlURL := rancherAPIHost + "/v1-saml/"
	samlURL += name
	actURL, err := url.Parse(samlURL)
	if err != nil {
		return fmt.Errorf("SAML: error in parsing URL")
	}

	metadataURL := *actURL
	metadataURL.Path = metadataURL.Path + "/saml/metadata"
	acsURL := *actURL
	acsURL.Path = acsURL.Path + "/saml/acs"

	sp := saml.ServiceProvider{
		Key:         privKey,
		Certificate: cert,
		MetadataURL: metadataURL,
		AcsURL:      acsURL,
		EntityID:    configToSet.EntityID,
	}

	// XML unmarshal throws an error for IdP Metadata cacheDuration field, as it's of type xml Duration. Using a separate struct for unmarshaling for now
	idm := &IDPMetadata{}
	if configToSet.IDPMetadataContent != "" {
		sp.IDPMetadata = &saml.EntityDescriptor{}
		if err := xml.NewDecoder(strings.NewReader(configToSet.IDPMetadataContent)).Decode(idm); err != nil {
			return fmt.Errorf("SAML: cannot initialize saml SP, cannot decode IDP Metadata content from the config %v, error %v", configToSet, err)
		}
	}

	sp.IDPMetadata.XMLName = idm.XMLName
	sp.IDPMetadata.ValidUntil = idm.ValidUntil
	sp.IDPMetadata.EntityID = idm.EntityID
	sp.IDPMetadata.SPSSODescriptors = idm.SPSSODescriptors
	sp.IDPMetadata.IDPSSODescriptors = idm.IDPSSODescriptors
	if name == ADFSName || name == OKTAName {
		sp.AuthnNameIDFormat = saml.UnspecifiedNameIDFormat
	}

	provider.serviceProvider = &sp

	cookieStore := ClientCookies{
		ServiceProvider: &sp,
		Name:            "token",
		Domain:          actURL.Host,
	}

	provider.clientState = &cookieStore

	root.Use(responsewriter.ContentTypeOptions)

	SamlProviders[name] = provider

	switch name {
	case PingName:
		root.Get("PingACS").HandlerFunc(provider.ServeHTTP)
		root.Get("PingMetadata").HandlerFunc(provider.ServeHTTP)
	case ADFSName:
		root.Get("AdfsACS").HandlerFunc(provider.ServeHTTP)
		root.Get("AdfsMetadata").HandlerFunc(provider.ServeHTTP)
	case KeyCloakName:
		root.Get("KeyCloakACS").HandlerFunc(provider.ServeHTTP)
		root.Get("KeyCloakMetadata").HandlerFunc(provider.ServeHTTP)
	case OKTAName:
		root.Get("OktaACS").HandlerFunc(provider.ServeHTTP)
		root.Get("OktaMetadata").HandlerFunc(provider.ServeHTTP)
	case ShibbolethName:
		root.Get("ShibbolethACS").HandlerFunc(provider.ServeHTTP)
		root.Get("ShibbolethMetadata").HandlerFunc(provider.ServeHTTP)
	}

	appliedVersion = configToSet.ResourceVersion

	return nil
}

func AuthHandler() http.Handler {
	root = mux.NewRouter()

	root.Methods("POST").Path("/v1-saml/ping/saml/acs").Name("PingACS")
	root.Methods("GET").Path("/v1-saml/ping/saml/metadata").Name("PingMetadata")

	root.Methods("POST").Path("/v1-saml/adfs/saml/acs").Name("AdfsACS")
	root.Methods("GET").Path("/v1-saml/adfs/saml/metadata").Name("AdfsMetadata")

	root.Methods("POST").Path("/v1-saml/keycloak/saml/acs").Name("KeyCloakACS")
	root.Methods("GET").Path("/v1-saml/keycloak/saml/metadata").Name("KeyCloakMetadata")

	root.Methods("POST").Path("/v1-saml/okta/saml/acs").Name("OktaACS")
	root.Methods("GET").Path("/v1-saml/okta/saml/metadata").Name("OktaMetadata")

	root.Methods("POST").Path("/v1-saml/shibboleth/saml/acs").Name("ShibbolethACS")
	root.Methods("GET").Path("/v1-saml/shibboleth/saml/metadata").Name("ShibbolethMetadata")

	return root
}

func (s *Provider) getSamlPrincipals(config *v32.SamlConfig, samlData map[string][]string) (v3.Principal, []v3.Principal, error) {
	var userPrincipal v3.Principal
	var groupPrincipals []v3.Principal
	uid, ok := samlData[config.UIDField]
	if !ok {
		// UID field provided by user is actually not there in SAMLResponse, without this we cannot differentiate between users and create separate principals
		return userPrincipal, groupPrincipals, fmt.Errorf("SAML: Unique ID field is not provided in SAML Response")
	}
	userPrincipal = v3.Principal{
		ObjectMeta:    metav1.ObjectMeta{Name: s.userType + "://" + uid[0]},
		Provider:      s.name,
		PrincipalType: "user",
		Me:            true,
	}

	displayName, ok := samlData[config.DisplayNameField]
	if ok {
		userPrincipal.DisplayName = displayName[0]
	}

	userName, ok := samlData[config.UserNameField]
	if ok {
		userPrincipal.LoginName = userName[0]
	}

	groups, ok := samlData[config.GroupsField]
	if ok {
		for _, group := range groups {
			group := v3.Principal{
				ObjectMeta:    metav1.ObjectMeta{Name: s.groupType + "://" + group},
				DisplayName:   group,
				Provider:      s.name,
				PrincipalType: "group",
				MemberOf:      true,
			}
			groupPrincipals = append(groupPrincipals, group)
		}
	}

	return userPrincipal, groupPrincipals, nil
}

// HandleSamlAssertion processes/handles the assertion obtained by the POST to /saml/acs from IdP
func (s *Provider) HandleSamlAssertion(w http.ResponseWriter, r *http.Request, assertion *saml.Assertion) {
	var groupPrincipals []v3.Principal
	var userPrincipal v3.Principal

	if relayState := r.Form.Get("RelayState"); relayState != "" {
		// delete the cookie
		s.clientState.DeleteState(w, r, relayState)
	}

	redirectURL := s.clientState.GetState(r, "Rancher_FinalRedirectURL")
	rancherAction := s.clientState.GetState(r, "Rancher_Action")
	if rancherAction == loginAction {
		redirectURL += "/login?"
	} else if rancherAction == testAndEnableAction {
		// the first query param is config=saml_provider_name set by UI
		redirectURL += "&"
	}

	samlData := make(map[string][]string)

	for _, attributeStatement := range assertion.AttributeStatements {
		for _, attr := range attributeStatement.Attributes {
			attrName := attr.FriendlyName
			if attrName == "" {
				attrName = attr.Name
			}
			for _, value := range attr.Values {
				samlData[attrName] = append(samlData[attrName], value.Value)
			}
		}
	}

	config, err := s.getSamlConfig()
	if err != nil {
		log.Errorf("SAML: Error getting saml config %v", err)
		http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
		return
	}

	userPrincipal, groupPrincipals, err = s.getSamlPrincipals(config, samlData)
	if err != nil {
		log.Error(err)
		// UI uses this translation key to get the error message
		http.Redirect(w, r, redirectURL+"errorCode=422&errorMsg="+UITranslationKeyForErrorMessage, http.StatusFound)
		return
	}
	allowedPrincipals := config.AllowedPrincipalIDs

	allowed, err := s.userMGR.CheckAccess(config.AccessMode, allowedPrincipals, userPrincipal.Name, groupPrincipals)
	if err != nil {
		log.Errorf("SAML: Error during login while checking access %v", err)
		http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
		return
	}
	if !allowed {
		log.Errorf("SAML: User [%s] is not an authorized user or is not a member of an authorized group", userPrincipal.Name)
		http.Redirect(w, r, redirectURL+"errorCode=403", http.StatusFound)
		return
	}

	userID := s.clientState.GetState(r, "Rancher_UserID")
	if userID != "" && rancherAction == testAndEnableAction {
		user, err := s.userMGR.SetPrincipalOnCurrentUserByUserID(userID, userPrincipal)
		if err != nil && user == nil {
			log.Errorf("SAML: Error setting principal on current user %v", err)
			http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
			return
		} else if err != nil && user != nil {
			http.Redirect(w, r, redirectURL+"errorCode=422&errorMsg="+err.Error(), http.StatusFound)
			return
		}

		config.Enabled = true
		err = s.saveSamlConfig(config)
		if err != nil {
			log.Errorf("SAML: Error saving saml config %v", err)
			http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
			return
		}

		isSecure := false
		if r.URL.Scheme == "https" {
			isSecure = true
		}
		err = s.setRancherToken(w, s.tokenMGR, user.Name, userPrincipal, groupPrincipals, isSecure)
		if err != nil {
			log.Errorf("SAML: Failed creating token with error: %v", err)
			http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
		}
		// delete the cookies
		s.clientState.DeleteState(w, r, "Rancher_UserID")
		s.clientState.DeleteState(w, r, "Rancher_Action")
		redirectURL := s.clientState.GetState(r, "Rancher_FinalRedirectURL")
		s.clientState.DeleteState(w, r, "Rancher_FinalRedirectURL")
		if redirectURL != "" {
			// delete the cookie
			s.clientState.DeleteState(w, r, "Rancher_FinalRedirectURL")
			http.Redirect(w, r, redirectURL, http.StatusFound)
		}
		return
	}

	displayName := userPrincipal.DisplayName
	if displayName == "" {
		displayName = userPrincipal.LoginName
	}
	user, err := s.userMGR.EnsureUser(userPrincipal.Name, displayName)
	if err != nil {
		log.Errorf("SAML: Failed getting user with error: %v", err)
		http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
		return
	}

	if user.Enabled != nil && !*user.Enabled {
		log.Errorf("SAML: User %v permission denied", user.Name)
		http.Redirect(w, r, redirectURL+"errorCode=403", http.StatusFound)
		return
	}

	userExtraInfo := s.GetUserExtraAttributes(userPrincipal)
	err = s.tokenMGR.UserAttributeCreateOrUpdate(user.Name, userPrincipal.Provider, groupPrincipals, userExtraInfo)
	if err != nil {
		log.Errorf("SAML: Failed creating or updating userAttribute with error: %v", err)
		http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
		return
	}

	err = s.setRancherToken(w, s.tokenMGR, user.Name, userPrincipal, groupPrincipals, true)
	if err != nil {
		log.Errorf("SAML: Failed creating token with error: %v", err)
		http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
	}
	redirectURL = s.clientState.GetState(r, "Rancher_FinalRedirectURL")

	if redirectURL != "" {
		// delete the cookie
		s.clientState.DeleteState(w, r, "Rancher_FinalRedirectURL")

		requestID := s.clientState.GetState(r, "Rancher_RequestID")
		log.Debugf("SAML: requestID: %s", requestID)
		if requestID != "" {
			// generate kubeconfig saml token
			responseType := s.clientState.GetState(r, "Rancher_ResponseType")
			publicKey := s.clientState.GetState(r, "Rancher_PublicKey")

			token, tokenValue, err := tokens.GetKubeConfigToken(user.Name, responseType, s.userMGR, userPrincipal)
			if err != nil {
				log.Errorf("SAML: getToken error %v", err)
				http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
				return
			}

			keyBytes, err := base64.StdEncoding.DecodeString(publicKey)
			if err != nil {
				log.Errorf("SAML: base64 DecodeString error %v", err)
				http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
				return
			}
			pubKey := &rsa.PublicKey{}
			err = json.Unmarshal(keyBytes, pubKey)
			if err != nil {
				log.Errorf("SAML: getPublicKey error %v", err)
				http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
				return
			}
			encryptedToken, err := rsa.EncryptOAEP(
				sha256.New(),
				rand.Reader,
				pubKey,
				[]byte(fmt.Sprintf("%s:%s", token.ObjectMeta.Name, tokenValue)),
				nil)
			if err != nil {
				log.Errorf("SAML: getEncryptedToken error %v", err)
				http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
				return
			}
			encoded := base64.StdEncoding.EncodeToString(encryptedToken)

			samlToken := &v3.SamlToken{
				Token:     encoded,
				ExpiresAt: token.ExpiresAt,
				ObjectMeta: v1.ObjectMeta{
					Name:      requestID,
					Namespace: namespace.GlobalNamespace,
				},
			}

			_, err = s.samlTokens.Create(samlToken)
			if err != nil {
				log.Errorf("SAML: createToken err %v", err)
				http.Redirect(w, r, redirectURL+"errorCode=500", http.StatusFound)
			}

			s.clientState.DeleteState(w, r, "Rancher_ConnToken")
			s.clientState.DeleteState(w, r, "Rancher_RequestUUID")
			s.clientState.DeleteState(w, r, "Rancher_ResponseType")
			s.clientState.DeleteState(w, r, "Rancher_PublicKey")

		}

		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}
}

func (s *Provider) setRancherToken(w http.ResponseWriter, tokenMGR *tokens.Manager, userID string, userPrincipal v3.Principal,
	groupPrincipals []v3.Principal, isSecure bool) error {
	authTimeout := settings.AuthUserSessionTTLMinutes.Get()
	var ttl int64
	if minutes, err := strconv.ParseInt(authTimeout, 10, 64); err == nil {
		ttl = minutes * 60 * 1000
	}

	rToken, unhashedTokenKey, err := tokenMGR.NewLoginToken(userID, userPrincipal, groupPrincipals, "", ttl, "")
	if err != nil {
		return err
	}

	tokenCookie := &http.Cookie{
		Name:     "R_SESS",
		Value:    rToken.ObjectMeta.Name + ":" + unhashedTokenKey,
		Secure:   isSecure,
		Path:     "/",
		HttpOnly: true,
	}
	http.SetCookie(w, tokenCookie)

	return nil
}
