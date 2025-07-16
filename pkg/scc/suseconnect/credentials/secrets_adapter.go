package credentials

import (
	"fmt"
	"github.com/SUSE/connect-ng/pkg/connection"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/controllers/common"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"maps"
)

const (
	UsernameKey = "systemLogin"
	PasswordKey = "password"
	TokenKey    = "systemToken"
)

type CredentialSecretsAdapter struct {
	secretNamespace string
	secretName      string
	ownerRef        *metav1.OwnerReference
	// TODO (dan) : let's make sure the lookups are handled via a secret cache
	secrets     v1core.SecretController
	secretCache v1core.SecretCache
	credentials SccCredentials
	labels      map[string]string
}

func New(
	namespace, name string,
	ownerRef *metav1.OwnerReference,
	secrets v1core.SecretController,
	secretCache v1core.SecretCache,
	labels map[string]string,
) *CredentialSecretsAdapter {
	labels[consts.LabelSccSecretRole] = string(consts.SCCCredentialsRole)
	return &CredentialSecretsAdapter{
		secretNamespace: namespace,
		secretName:      name,
		ownerRef:        ownerRef,
		secrets:         secrets,
		secretCache:     secretCache,
		labels:          labels,
	}
}

func (c *CredentialSecretsAdapter) InitSecret() error {
	return c.saveCredentials()
}

func (c *CredentialSecretsAdapter) Refresh() error {
	return c.loadCredentials()
}

func (c *CredentialSecretsAdapter) loadCredentials() error {
	// TODO gather errors
	sccCreds, err := c.secretCache.Get(c.secretNamespace, c.secretName)
	if err == nil && sccCreds != nil {
		if len(sccCreds.Data) == 0 {
			return fmt.Errorf("secret %s/%s has no data fields; but should always have them", c.secretNamespace, c.secretName)
		}
		username, _ := sccCreds.Data[UsernameKey]
		password, _ := sccCreds.Data[PasswordKey]
		token, _ := sccCreds.Data[TokenKey]
		_ = c.credentials.SetLogin(string(username), string(password))
		_ = c.credentials.UpdateToken(string(token))
	}

	return err
}

func (c *CredentialSecretsAdapter) saveCredentials() error {
	create := false
	// TODO gather errors
	sccCreds, err := c.secrets.Get(c.secretNamespace, c.secretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		create = true
		sccCreds = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      c.secretName,
				Namespace: c.secretNamespace,
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

	sccCreds = common.SecretAddCredentialsFinalizer(sccCreds)

	if sccCreds.Labels == nil {
		sccCreds.Labels = c.labels
	} else {
		maps.Copy(sccCreds.Labels, c.labels)
	}

	if c.ownerRef != nil {
		sccCreds.OwnerReferences = []metav1.OwnerReference{*c.ownerRef}
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
	currentSecret, err := c.secretCache.Get(c.secretNamespace, c.secretName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if currentSecret == nil {
		return nil
	}

	if common.SecretHasCredentialsFinalizer(currentSecret) {
		updatedSecret := currentSecret.DeepCopy()
		updatedSecret = common.SecretRemoveCredentialsFinalizer(updatedSecret)
		if _, updateErr := c.secrets.Update(updatedSecret); updateErr != nil {
			if apierrors.IsNotFound(updateErr) {
				return nil
			}

			return updateErr
		}
	}

	return c.secrets.Delete(c.secretNamespace, c.secretName, &metav1.DeleteOptions{})
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

func (c *CredentialSecretsAdapter) SetRegistrationCredentialsSecretRef(registrationObj *v1.Registration) *v1.Registration {
	registrationObj.Status.SystemCredentialsSecretRef = &corev1.SecretReference{
		Namespace: c.secretNamespace,
		Name:      c.secretName,
	}
	return registrationObj
}

var _ connection.Credentials = &CredentialSecretsAdapter{}
