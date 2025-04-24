package credentials

import (
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
)

func TestBasicNewSecretsAdapter(t *testing.T) {
	gomockCtrl := gomock.NewController(t)
	mockSecretsController := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](gomockCtrl)

	// Define expected calls to our mock controller using gomock.
	mockSecretsController.EXPECT().Get(Namespace, SecretName, metav1.GetOptions{}).MaxTimes(1)

	secretsBackedCredentials := New(mockSecretsController)
	assert.Equal(t, &CredentialSecretsAdapter{
		secrets: mockSecretsController,
	}, secretsBackedCredentials)
}

func testSecret(name string, namespace string, dataOverride *map[string]string) v1.Secret {
	if name == "" {
		name = SecretName
	}
	if namespace == "" {
		namespace = Namespace
	}

	secretData := map[string][]byte{
		UsernameKey: []byte("system_testLoginUser"),
		PasswordKey: []byte("system_testLoginPassword"),
		TokenKey:    []byte("system_testLoginToken"),
	}

	if dataOverride != nil {
		dataOverrideMap := *dataOverride
		username, ok := dataOverrideMap[UsernameKey]
		if ok {
			secretData[UsernameKey] = []byte(username)
		}
		password, ok := dataOverrideMap[PasswordKey]
		if ok {
			secretData[PasswordKey] = []byte(password)
		}
		token, ok := dataOverrideMap[TokenKey]
		if ok {
			secretData[TokenKey] = []byte(token)
		}
	}

	return v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: secretData,
	}
}

func preparedSecretsMock(t *testing.T) *fake.MockControllerInterface[*v1.Secret, *v1.SecretList] {
	gomockCtrl := gomock.NewController(t)
	mockSecretsController := fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](gomockCtrl)

	defaultTestSecret := testSecret("", "", nil)

	// Define expected calls to our mock controller using gomock.
	mockSecretsController.EXPECT().Get(Namespace, SecretName, metav1.GetOptions{}).Return(&defaultTestSecret, nil).AnyTimes()
	mockSecretsController.EXPECT().Create(defaultTestSecret).Return(&defaultTestSecret, nil).AnyTimes()

	return mockSecretsController
}

func TestNewSecretsAdapter(t *testing.T) {
	mockSecretsController := preparedSecretsMock(t)

	secretsBackedCredentials := New(mockSecretsController)
	assert.Equal(t, &CredentialSecretsAdapter{
		secrets: mockSecretsController,
		credentials: SccCredentials{
			systemLogin: "system_testLoginUser",
			password:    "system_testLoginPassword",
			systemToken: "system_testLoginToken",
		},
	}, secretsBackedCredentials)
}

func TestSecretsAdapterCredentials(t *testing.T) {
	mockSecretsController := preparedSecretsMock(t)

	secretsBackedCredentials := New(mockSecretsController)
	assert.Equal(t, &CredentialSecretsAdapter{
		secrets: mockSecretsController,
		credentials: SccCredentials{
			systemLogin: "system_testLoginUser",
			password:    "system_testLoginPassword",
			systemToken: "system_testLoginToken",
		},
	}, secretsBackedCredentials)

	hasAuth := secretsBackedCredentials.HasAuthentication()
	assert.True(t, hasAuth)

	token, err := secretsBackedCredentials.Token()
	assert.NoError(t, err)
	assert.Equal(t, "system_testLoginToken", token)

	login, pass, err := secretsBackedCredentials.Login()
	assert.NoError(t, err)
	assert.Equal(t, "system_testLoginUser", login)
	assert.Equal(t, "system_testLoginPassword", pass)

	modifiedSecret := testSecret("", "", &map[string]string{
		TokenKey: "Hello-WORLD",
	})
	mockSecretsController.EXPECT().Update(&modifiedSecret).Return(&modifiedSecret, nil).AnyTimes()

	// Update token
	updateErr := secretsBackedCredentials.UpdateToken("Hello-WORLD")
	assert.NoError(t, updateErr)

	token, err = secretsBackedCredentials.Token()
	assert.NoError(t, err)
	assert.Equal(t, "Hello-WORLD", token)

	modifiedSecret = testSecret("", "", &map[string]string{
		TokenKey: "",
	})
	mockSecretsController.EXPECT().Update(&modifiedSecret).Return(&modifiedSecret, nil).AnyTimes()

	// Update token ERROR
	updateErr = secretsBackedCredentials.UpdateToken("")
	assert.NoError(t, updateErr)

	token, err = secretsBackedCredentials.Token()
	assert.NoError(t, err)
	assert.Equal(t, "", token)

	modifiedSecret = testSecret("", "", &map[string]string{
		UsernameKey: "fred",
		PasswordKey: "freds-system-PW",
		TokenKey:    "",
	})
	mockSecretsController.EXPECT().Update(&modifiedSecret).Return(&modifiedSecret, nil).AnyTimes()

	// Update login
	updateErr = secretsBackedCredentials.SetLogin("fred", "freds-system-PW")
	assert.NoError(t, updateErr)

	login, pass, err = secretsBackedCredentials.Login()
	assert.NoError(t, err)
	assert.Equal(t, "fred", login)
	assert.Equal(t, "freds-system-PW", pass)

	// Update token ERROR
	updateErr = secretsBackedCredentials.SetLogin("", "1")
	assert.Error(t, updateErr)

	login, pass, err = secretsBackedCredentials.Login()
	assert.NoError(t, err)
	assert.Equal(t, "fred", login)
	assert.Equal(t, "freds-system-PW", pass)
}

func TestSecretsAdapterSccCredentials(t *testing.T) {
	mockSecretsController := preparedSecretsMock(t)

	secretsBackedCredentials := New(mockSecretsController)
	assert.Equal(t, &CredentialSecretsAdapter{
		secrets: mockSecretsController,
		credentials: SccCredentials{
			systemLogin: "system_testLoginUser",
			password:    "system_testLoginPassword",
			systemToken: "system_testLoginToken",
		},
	}, secretsBackedCredentials)

	sccCrds := secretsBackedCredentials.SccCredentials()
	assert.NotNil(t, sccCrds)
	assert.Equal(t, secretsBackedCredentials.HasAuthentication(), sccCrds.HasAuthentication())
}

func TestSecretEmptyTokenUpdate(t *testing.T) {
	mockSecretsController := preparedSecretsMock(t)

	secretsBackedCredentials := New(mockSecretsController)
	assert.Equal(t, &CredentialSecretsAdapter{
		secrets: mockSecretsController,
		credentials: SccCredentials{
			systemLogin: "system_testLoginUser",
			password:    "system_testLoginPassword",
			systemToken: "system_testLoginToken",
		},
	}, secretsBackedCredentials)

	modifiedSecret := testSecret("", "", &map[string]string{
		TokenKey: "",
	})
	mockSecretsController.EXPECT().Update(&modifiedSecret).Return(&modifiedSecret, nil).AnyTimes()

	updateErr := secretsBackedCredentials.UpdateToken("")
	assert.NoError(t, updateErr)
	updateErr = secretsBackedCredentials.UpdateToken("")
	assert.NoError(t, updateErr)
}

func TestSecretLoadErrors(t *testing.T) {
	t.Skip("TODO: Create a testing scenario that would trigger uncovered lines")
}
