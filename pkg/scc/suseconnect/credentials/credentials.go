package credentials

import (
	"github.com/sirupsen/logrus"

	"github.com/SUSE/connect-ng/pkg/connection"

	"github.com/pkg/errors"
)

type CredentialType int

const (
	CredentialTypeUnconfigured CredentialType = iota
	CredentialTypeToken
	CredentialTypeLogin
	CredentialTypeBoth
)

var credentialTypeName = map[CredentialType]string{
	CredentialTypeUnconfigured: "unconfigured",
	CredentialTypeToken:        "systemToken",
	CredentialTypeLogin:        "systemLogin",
	CredentialTypeBoth:         "systemToken and systemLogin",
}

func (ct CredentialType) String() string {
	return credentialTypeName[ct]
}

type SccCredentials struct {
	systemToken string
	systemLogin string
	password    string
}

func (c *SccCredentials) DeepCopy() *SccCredentials {
	if c == nil {
		return nil
	}

	out := &SccCredentials{
		systemToken: c.systemToken,
		systemLogin: c.systemLogin,
		password:    c.password,
	}
	return out
}

// CredentialsType Returns the mode (or modes) that are configured for authentication
func (c *SccCredentials) CredentialsType() CredentialType {
	if c.systemToken != "" && c.systemLogin != "" && c.password != "" {
		return CredentialTypeBoth
	}

	if c.systemToken != "" {
		return CredentialTypeToken
	}

	if c.systemLogin != "" && c.password != "" {
		return CredentialTypeLogin
	}

	return CredentialTypeUnconfigured
}

// HasAuthentication Returns true if we can authenticate at all, false otherwise.
func (c *SccCredentials) HasAuthentication() bool {
	configuredType := c.CredentialsType()
	return configuredType == CredentialTypeBoth
}

// Token returns the current system used to detect duplicated systems. This
// systemToken gets rotated on each non read operation.
func (c *SccCredentials) Token() (string, error) {
	return c.systemToken, nil
}

// UpdateToken is called when a systemToken has changed
func (c *SccCredentials) UpdateToken(newToken string) error {
	c.systemToken = newToken

	return nil
}

// Login returns the username and password
func (c *SccCredentials) Login() (string, string, error) {
	configuredType := c.CredentialsType()
	if configuredType == CredentialTypeUnconfigured || configuredType == CredentialTypeToken {
		return "", "", errors.New("cannot use systemLogin credentials when they are not properly configured")
	}

	return c.systemLogin, c.password, nil
}

// SetLogin updates the saved username and password
func (c *SccCredentials) SetLogin(newLogin string, newPassword string) error {
	if newLogin == "" || newPassword == "" {
		errorMessage := ""
		if newLogin == "" {
			errorMessage += "newLogin is empty"
		}
		if newPassword == "" {
			if newLogin == "" {
				errorMessage += " and "
			}
			errorMessage += "newPassword is empty"
		}
		logrus.Warnf("updating systemLogin to empty value(s); %v", errorMessage)
	}

	c.systemLogin = newLogin
	c.password = newPassword

	return nil
}

func (c *SccCredentials) SccCredentials() connection.Credentials {
	return c
}

func NewCredentials() *SccCredentials {
	return &SccCredentials{}
}
