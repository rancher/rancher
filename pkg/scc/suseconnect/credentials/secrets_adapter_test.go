package credentials

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
)

const (
	Namespace = consts.DefaultSCCNamespace
	Finalizer = consts.FinalizerSccCredentials
)

var SecretName = consts.SCCCredentialsSecretName("testing-credentials")

func mockRegistration() v1.Registration {
	return v1.Registration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "registration-testing",
		},
		Spec: v1.RegistrationSpec{
			Mode: v1.RegistrationModeOnline,
		},
	}
}

func mockCredentials(
	testOwnerRef *metav1.OwnerReference,
	mockSecretsController *fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList],
	mockSecretsCache *fake.MockCacheInterface[*corev1.Secret],
) *CredentialSecretsAdapter {
	return New(
		Namespace,
		SecretName,
		Finalizer,
		testOwnerRef,
		mockSecretsController,
		mockSecretsCache,
		map[string]string{},
	)
}

func TestBasicNewSecretsAdapter(t *testing.T) {
	gomockCtrl := gomock.NewController(t)
	mockSecretsController := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](gomockCtrl)
	mockSecretsCache := fake.NewMockCacheInterface[*corev1.Secret](gomockCtrl)

	// Define expected calls to our mock controller using gomock.
	mockSecretsController.EXPECT().Get(Namespace, SecretName, metav1.GetOptions{}).MaxTimes(1)

	testReg := mockRegistration()
	testOwnerRef := testReg.ToOwnerRef()

	secretsBackedCredentials := mockCredentials(testOwnerRef, mockSecretsController, mockSecretsCache)
	assert.Equal(t, &CredentialSecretsAdapter{
		secretNamespace: Namespace,
		secretName:      SecretName,
		secrets:         mockSecretsController,
		secretCache:     mockSecretsCache,
		finalizerName:   Finalizer,
		ownerRef:        testOwnerRef,
		labels:          map[string]string{},
	}, secretsBackedCredentials)
}

func testSecret(name string, namespace string, dataOverride *map[string]string) corev1.Secret {
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

	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: secretData,
	}
}

func testSecretForUpdate(inSecret *corev1.Secret, inReg *v1.Registration) *corev1.Secret {
	inSecret.ObjectMeta.Finalizers = append(inSecret.ObjectMeta.Finalizers, Finalizer)
	inSecret.OwnerReferences = []metav1.OwnerReference{
		*inReg.ToOwnerRef(),
	}
	return inSecret
}

func preparedSecretsMocks(t *testing.T) (*fake.MockControllerInterface[*corev1.Secret, *corev1.SecretList], *fake.MockCacheInterface[*corev1.Secret]) {
	gomockCtrl := gomock.NewController(t)
	mockSecretsController := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](gomockCtrl)

	defaultTestSecret := testSecret("", "", nil)

	// Define expected calls to our mock controller using gomock.
	mockSecretsController.EXPECT().Get(Namespace, SecretName, metav1.GetOptions{}).Return(&defaultTestSecret, nil).AnyTimes()
	mockSecretsController.EXPECT().Create(defaultTestSecret).Return(&defaultTestSecret, nil).AnyTimes()

	mockSecretsCache := fake.NewMockCacheInterface[*corev1.Secret](gomockCtrl)
	mockSecretsCache.EXPECT().Get(Namespace, SecretName).Return(&defaultTestSecret, nil).AnyTimes()

	return mockSecretsController, mockSecretsCache
}

func TestNewSecretsAdapter(t *testing.T) {
	mockSecretsController, mockSecretsCache := preparedSecretsMocks(t)

	testReg := mockRegistration()
	testOwnerRef := testReg.ToOwnerRef()
	secretsBackedCredentials := mockCredentials(testOwnerRef, mockSecretsController, mockSecretsCache)
	_ = secretsBackedCredentials.Refresh()

	assert.Equal(t, &CredentialSecretsAdapter{
		secretNamespace: Namespace,
		secretName:      SecretName,
		finalizerName:   Finalizer,
		ownerRef:        testOwnerRef,
		secrets:         mockSecretsController,
		secretCache:     mockSecretsCache,
		credentials: SccCredentials{
			systemLogin: "system_testLoginUser",
			password:    "system_testLoginPassword",
			systemToken: "system_testLoginToken",
		},
		labels: map[string]string{},
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
}

func TestSecretsAdapterCredentials_Basic(t *testing.T) {
	mockSecretsController, mockSecretsCache := preparedSecretsMocks(t)

	testReg := mockRegistration()
	testOwnerRef := testReg.ToOwnerRef()
	secretsBackedCredentials := mockCredentials(testOwnerRef, mockSecretsController, mockSecretsCache)
	_ = secretsBackedCredentials.Refresh()

	assert.Equal(t, &CredentialSecretsAdapter{
		secretNamespace: Namespace,
		secretName:      SecretName,
		finalizerName:   Finalizer,
		ownerRef:        testOwnerRef,
		secrets:         mockSecretsController,
		secretCache:     mockSecretsCache,
		credentials: SccCredentials{
			systemLogin: "system_testLoginUser",
			password:    "system_testLoginPassword",
			systemToken: "system_testLoginToken",
		},
		labels: map[string]string{},
	}, secretsBackedCredentials)

	modifiedSecret := testSecret("", "", &map[string]string{
		TokenKey: "Hello-WORLD",
	})
	prepared := testSecretForUpdate(modifiedSecret.DeepCopy(), &testReg)
	// TODO fix skip
	t.Skip("something broke this")
	mockSecretsController.EXPECT().Update(prepared).Return(prepared, nil).MaxTimes(1)

	// Update token
	updateErr := secretsBackedCredentials.UpdateToken("Hello-WORLD")
	assert.NoError(t, updateErr)

	token, err := secretsBackedCredentials.Token()
	assert.NoError(t, err)
	assert.Equal(t, "Hello-WORLD", token)

	modifiedSecret = testSecret("", "", &map[string]string{
		TokenKey: "",
	})
	prepared = testSecretForUpdate(modifiedSecret.DeepCopy(), &testReg)
	mockSecretsController.EXPECT().Update(prepared).Return(prepared, nil).MaxTimes(1)

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
	prepared = testSecretForUpdate(modifiedSecret.DeepCopy(), &testReg)
	mockSecretsController.EXPECT().Update(prepared).Return(prepared, nil).MaxTimes(1)

	// Update login
	updateErr = secretsBackedCredentials.SetLogin("fred", "freds-system-PW")
	assert.NoError(t, updateErr)

	login, pass, err := secretsBackedCredentials.Login()
	assert.NoError(t, err)
	assert.Equal(t, "fred", login)
	assert.Equal(t, "freds-system-PW", pass)
}

func TestSecretsAdapterSccCredentials(t *testing.T) {
	mockSecretsController, mockSecretsCache := preparedSecretsMocks(t)

	testReg := mockRegistration()
	testOwnerRef := testReg.ToOwnerRef()
	secretsBackedCredentials := mockCredentials(testOwnerRef, mockSecretsController, mockSecretsCache)
	_ = secretsBackedCredentials.Refresh()

	assert.Equal(t, &CredentialSecretsAdapter{
		secretNamespace: Namespace,
		secretName:      SecretName,
		finalizerName:   Finalizer,
		ownerRef:        testOwnerRef,
		secrets:         mockSecretsController,
		secretCache:     mockSecretsCache,
		credentials: SccCredentials{
			systemLogin: "system_testLoginUser",
			password:    "system_testLoginPassword",
			systemToken: "system_testLoginToken",
		},
		labels: map[string]string{},
	}, secretsBackedCredentials)

	sccCrds := secretsBackedCredentials.SccCredentials()
	assert.NotNil(t, sccCrds)
	assert.Equal(t, secretsBackedCredentials.HasAuthentication(), sccCrds.HasAuthentication())
}

func TestSecretEmptyTokenUpdate(t *testing.T) {
	mockSecretsController, mockSecretsCache := preparedSecretsMocks(t)

	testReg := mockRegistration()
	testOwnerRef := testReg.ToOwnerRef()
	secretsBackedCredentials := mockCredentials(testOwnerRef, mockSecretsController, mockSecretsCache)
	_ = secretsBackedCredentials.Refresh()

	assert.Equal(t, &CredentialSecretsAdapter{
		secretNamespace: Namespace,
		secretName:      SecretName,
		finalizerName:   Finalizer,
		ownerRef:        testOwnerRef,
		secrets:         mockSecretsController,
		secretCache:     mockSecretsCache,
		credentials: SccCredentials{
			systemLogin: "system_testLoginUser",
			password:    "system_testLoginPassword",
			systemToken: "system_testLoginToken",
		},
		labels: map[string]string{},
	}, secretsBackedCredentials)

	modifiedSecret := testSecret("", "", &map[string]string{
		TokenKey: "",
	})
	prepared := testSecretForUpdate(modifiedSecret.DeepCopy(), &testReg)
	// TODO fix skip
	t.Skip("something broke this")
	mockSecretsController.EXPECT().Update(prepared).Return(prepared, nil).MaxTimes(1)

	updateErr := secretsBackedCredentials.UpdateToken("")
	assert.NoError(t, updateErr)
	updateErr = secretsBackedCredentials.UpdateToken("")
	assert.NoError(t, updateErr)
}

func TestSecretLoadErrors(t *testing.T) {
	t.Skip("TODO: Create a testing scenario that would trigger uncovered lines")
}
