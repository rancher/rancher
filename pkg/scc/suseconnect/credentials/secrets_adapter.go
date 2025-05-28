package credentials

import (
	"github.com/SUSE/connect-ng/pkg/connection"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SecretName  = "rancher-scc-system-credentials"
	Namespace   = "cattle-system"
	UsernameKey = "systemLogin"
	PasswordKey = "password"
	TokenKey    = "systemToken"
)

type CredentialSecretsAdapter struct {
	secrets     v1core.SecretController
	credentials SccCredentials
	// TODO: implement secret reconcile logic
	currentSHA string
}

func New(secrets v1core.SecretController) *CredentialSecretsAdapter {
	newAdapterCreds := NewCredentials()

	// Load initial creds from secret
	sccCreds, err := secrets.Get(Namespace, SecretName, metav1.GetOptions{})
	if err == nil && sccCreds != nil && len(sccCreds.Data) != 0 {
		username, _ := sccCreds.Data[UsernameKey]
		password, _ := sccCreds.Data[PasswordKey]
		token, _ := sccCreds.Data[TokenKey]
		_ = newAdapterCreds.SetLogin(string(username), string(password))
		_ = newAdapterCreds.UpdateToken(string(token))
	}

	return &CredentialSecretsAdapter{
		secrets:     secrets,
		credentials: *newAdapterCreds,
	}
}

func (c *CredentialSecretsAdapter) Refresh() error {
	return c.loadCredentials()
}

func (c *CredentialSecretsAdapter) loadCredentials() error {
	// TODO gather errors
	sccCreds, err := c.secrets.Get(Namespace, SecretName, metav1.GetOptions{})
	if err == nil && sccCreds != nil && len(sccCreds.Data) != 0 {
		username, _ := sccCreds.Data[UsernameKey]
		password, _ := sccCreds.Data[PasswordKey]
		token, _ := sccCreds.Data[TokenKey]
		_ = c.credentials.SetLogin(string(username), string(password))
		_ = c.credentials.UpdateToken(string(token))
	}

	return nil
}

func (c *CredentialSecretsAdapter) saveCredentials() error {
	create := false
	// TODO gather errors
	sccCreds, err := c.secrets.Get(Namespace, SecretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		create = true
		sccCreds = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SecretName,
				Namespace: Namespace,
			},
			Data: map[string][]byte{},
		}
	}

	if sccCreds.Data == nil {
		sccCreds.Data = make(map[string][]byte)
	}

	login, pw, err := c.credentials.Login()
	if err == nil {
		sccCreds.Data[UsernameKey] = []byte(login)
		sccCreds.Data[PasswordKey] = []byte(pw)
	}

	token, err := c.credentials.Token()
	if err == nil {
		sccCreds.Data[TokenKey] = []byte(token)
	}

	var createOrUpdateErr error
	if create {
		_, createOrUpdateErr = c.secrets.Create(sccCreds)
	} else {
		_, createOrUpdateErr = c.secrets.Update(sccCreds)
	}

	return createOrUpdateErr
}

func (c *CredentialSecretsAdapter) Remove() error {
	return c.secrets.Delete(Namespace, SecretName, &metav1.DeleteOptions{})
}

func (c *CredentialSecretsAdapter) HasAuthentication() bool {
	if err := c.loadCredentials(); err != nil {
		return false
	}
	return c.credentials.HasAuthentication()
}

func (c *CredentialSecretsAdapter) Token() (string, error) {
	if err := c.loadCredentials(); err != nil {
		return "", err
	}
	return c.credentials.Token()
}

func (c *CredentialSecretsAdapter) UpdateToken(newToken string) error {
	if newToken == "" && c.credentials.systemToken == "" {
		return nil
	}
	updateErr := c.credentials.UpdateToken(newToken)
	if updateErr != nil {
		return updateErr
	}
	return c.saveCredentials()
}

func (c *CredentialSecretsAdapter) Login() (string, string, error) {
	return c.credentials.Login()
}

func (c *CredentialSecretsAdapter) SetLogin(newUser string, newPass string) error {
	updateErr := c.credentials.SetLogin(newUser, newPass)
	if updateErr != nil {
		return updateErr
	}
	return c.saveCredentials()
}

func (c *CredentialSecretsAdapter) SccCredentials() connection.Credentials {
	return c
}
