package credentials

import (
	"github.com/SUSE/connect-ng/pkg/connection"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCredentialTypeStrings(t *testing.T) {
	assert.Equal(t, "unconfigured", CredentialTypeUnconfigured.String())
	assert.Equal(t, "systemToken", CredentialTypeToken.String())
	assert.Equal(t, "systemLogin", CredentialTypeLogin.String())
	assert.Equal(t, "systemToken and systemLogin", CredentialTypeBoth.String())
}

func TestNewCredentials(t *testing.T) {
	credential := NewCredentials()
	err := credential.SetLogin("systemLogin", "password")
	assert.NoError(t, err)
	login, password, err := credential.Login()
	assert.NoError(t, err)
	assert.Equal(t, "systemLogin", login)
	assert.Equal(t, "password", password)
}

func TestSetLogin(t *testing.T) {
	credentials := SccCredentials{
		systemLogin: "",
		password:    "",
	}
	err := credentials.SetLogin("newLogin", "newPassword")
	assert.NoError(t, err)

	credsCopy := credentials.DeepCopy()
	assert.Equal(t, &credentials, credsCopy)
	err = credsCopy.SetLogin("", "newPassword2")
	assert.NoError(t, err)
	assert.NotEqual(t, &credentials, credsCopy)
}

func TestSetLoginEmptyFields(t *testing.T) {
	credential := SccCredentials{
		systemLogin: "123",
		password:    "321",
	}
	assert.Equal(t, "123", credential.systemLogin)
	assert.Equal(t, "321", credential.password)
	err := credential.SetLogin("", "")
	assert.NoError(t, err)
	assert.Equal(t, "", credential.systemLogin)
	assert.Equal(t, "", credential.password)
}

func TestSetLoginMultipleErrors(t *testing.T) {
	credential := SccCredentials{
		systemLogin: "",
		password:    "",
	}
	err := credential.SetLogin("", "newPassword")
	assert.NoError(t, err)
	assert.Equal(t, "", credential.systemLogin)
	assert.Equal(t, "newPassword", credential.password)

	err = credential.SetLogin("newLogin", "newPassword")
	assert.NoError(t, err)
	assert.Equal(t, "newLogin", credential.systemLogin)
	assert.Equal(t, "newPassword", credential.password)

	err = credential.SetLogin("newLogin", "")
	assert.NoError(t, err)
	assert.Equal(t, "newLogin", credential.systemLogin)
	assert.Equal(t, "", credential.password)
}

func TestHasAuthentication(t *testing.T) {
	credential := SccCredentials{
		systemToken: "systemToken",
		systemLogin: "systemLogin",
		password:    "password",
	}
	assert.True(t, credential.HasAuthentication())

	credential = SccCredentials{
		systemToken: "",
		systemLogin: "systemLogin",
		password:    "password",
	}
	assert.False(t, credential.HasAuthentication())

	credential = SccCredentials{
		systemToken: "",
		systemLogin: "",
		password:    "",
	}
	assert.False(t, credential.HasAuthentication())
}

func TestToken(t *testing.T) {
	credential := SccCredentials{
		systemToken: "systemToken",
		systemLogin: "",
		password:    "",
	}
	token, err := credential.Token()
	assert.Equal(t, "systemToken", token)
	assert.NoError(t, err)
	err = credential.UpdateToken("newTokenTest")
	assert.NoError(t, err)

	token, err = credential.Token()
	assert.NotEqual(t, "systemToken", token)
	assert.Equal(t, "newTokenTest", token)
	assert.NoError(t, err)
}

func TestTokenErrors(t *testing.T) {
	credential := SccCredentials{
		systemToken: "",
		systemLogin: "",
		password:    "",
	}
	_, err := credential.Token()
	assert.NoError(t, err)

	err = credential.UpdateToken("testToken")
	assert.NoError(t, err)
}

func TestCredentialsType(t *testing.T) {
	credential := SccCredentials{
		systemToken: "",
		systemLogin: "systemLogin",
		password:    "password",
	}
	assert.Equal(t, CredentialTypeLogin, credential.CredentialsType())

	credential = SccCredentials{
		systemToken: "",
		systemLogin: "systemLogin",
		password:    "",
	}
	assert.Equal(t, CredentialTypeUnconfigured, credential.CredentialsType())

	credential = SccCredentials{
		systemToken: "systemToken",
		systemLogin: "",
		password:    "",
	}
	assert.Equal(t, CredentialTypeToken, credential.CredentialsType())

	credential = SccCredentials{
		systemToken: "systemToken",
		systemLogin: "systemLogin",
		password:    "",
	}
	assert.Equal(t, CredentialTypeToken, credential.CredentialsType())

	credential = SccCredentials{
		systemToken: "",
		systemLogin: "",
		password:    "",
	}
	assert.Equal(t, CredentialTypeUnconfigured, credential.CredentialsType())
}

func TestCredentialsEmpty(t *testing.T) {
	credential := NewCredentials()
	assert.Equal(t, CredentialTypeUnconfigured, credential.CredentialsType())

	emptyCreds := SccCredentials{
		systemToken: "",
		systemLogin: "",
		password:    "",
	}
	assert.Equal(t, &emptyCreds, credential)

}

func TestLogin(t *testing.T) {
	credential := SccCredentials{
		systemToken: "",
		systemLogin: "user1",
		password:    "pass1",
	}
	login, pass, err := credential.Login()
	assert.Equal(t, "user1", login)
	assert.Equal(t, "pass1", pass)
	assert.NoError(t, err)

	err = credential.SetLogin("user2", "pass2")
	assert.NoError(t, err)

	login, pass, err = credential.Login()
	assert.NotEqual(t, "user1", login)
	assert.Equal(t, "user2", login)
	assert.NotEqual(t, "pass1", pass)
	assert.Equal(t, "pass2", pass)
	assert.NoError(t, err)
}

func TestLoginErrors(t *testing.T) {
	credential := SccCredentials{
		systemToken: "",
		systemLogin: "",
		password:    "",
	}
	_, _, err := credential.Login()
	assert.Error(t, err)

	err = credential.SetLogin("user3", "")
	assert.NoError(t, err)

	err = credential.SetLogin("", "pass3")
	assert.NoError(t, err)

	err = credential.SetLogin("user3", "pass3")
	assert.NoError(t, err)

	err = credential.SetLogin("", "")
	err = credential.UpdateToken("token")

	_, _, err = credential.Login()
	assert.Error(t, err)
}

func TestSccCredentialsInterface(t *testing.T) {
	creds := SccCredentials{
		systemToken: "",
		systemLogin: "test",
		password:    "password123",
	}

	newTest, ok := creds.SccCredentials().(connection.Credentials)
	assert.True(t, ok)
	assert.Equal(t, &creds, newTest)
}
