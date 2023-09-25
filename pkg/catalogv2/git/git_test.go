package git

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

var errTestCustom error = errors.New("some error")

// dummy certs and keys randomly generate and not attached to anything
// do not ident this or change in any way
const id_rsa_test_random string = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAABFwAAAAdzc2gtcn
NhAAAAAwEAAQAAAQEA1joQLF9WVMWpFL1WOf/DDHiA1xAe7J5fLCfdbzvZdcT8dYibKOB/
BVyUT/0ML92fC8Tvw+7VuRxppQxIfPVlsSRl0mzdrOnckmQDzr9Uc1G0tPhIbOK3v+ZBU9
PISH3eEQTxrzeqXgGrHn163H0npD9TAOWg7TRvcT07uHPRPR2/b8xGPt80UIzSQSKLA9br
LrPh6Xk2RrC+hnwXyscMMhhq2dxr9xOZt5ad2EWhPbw/rzIJZYUwcj/idZiwuraDdD8XAO
VydBVb6f+KmI3IDkzDE3M/T0ZzYxWBCJ2l+UmY6Ry6Aev1zZOEUwa+cgNNeNK4bMkvk3jh
urxKdpWdAwAAA9CWMljkljJY5AAAAAdzc2gtcnNhAAABAQDWOhAsX1ZUxakUvVY5/8MMeI
DXEB7snl8sJ91vO9l1xPx1iJso4H8FXJRP/Qwv3Z8LxO/D7tW5HGmlDEh89WWxJGXSbN2s
6dySZAPOv1RzUbS0+Ehs4re/5kFT08hIfd4RBPGvN6peAasefXrcfSekP1MA5aDtNG9xPT
u4c9E9Hb9vzEY+3zRQjNJBIosD1usus+HpeTZGsL6GfBfKxwwyGGrZ3Gv3E5m3lp3YRaE9
vD+vMgllhTByP+J1mLC6toN0PxcA5XJ0FVvp/4qYjcgOTMMTcz9PRnNjFYEInaX5SZjpHL
oB6/XNk4RTBr5yA0140rhsyS+TeOG6vEp2lZ0DAAAAAwEAAQAAAQBQ9AkXk4FesIEH7tKO
QUm2RTU+z/83oFNLrtbdSWsQN3vFeHVVuZwWbRk4ruGzltaaznVif7bw6D092wnreppOmf
gGUBBm3sr04OmVb7TcjSQx/N71kqkoUb0fDdlSF4pauRkRgwIU2yGMeJN8jaj0xt85aAzr
hlSUoLSYK+AGbS3abEoaITFw5ef5q2EHwCL4tzoYjxJR912Lnp2se/27x6CEvKiT/ZwiwW
ET0XpXitHKK7VuzPDTFJtREHX0lj1/Stk5GbCOwn+YkK+hfTf8CJGhSxgnLZ587Vf3Tp/E
nuzwuhFY+ZukvV7MtHH1OMLwwRVur9fJSsYcZUbDgnmFAAAAgQDnkV5ePaOrBKjHZVyZXR
C6r0OoAjpnFS42O2EgEECbUi80k1EKaF9mJ0b7JJeurFdRmFjnvyTPgCJfbWliDlH0fdZZ
6X8//75lPl9mjGvWplzWL38AlVdbTfx7SJZBMttGb0P6tsj/zobaLtGSLu/UNClC1gxwYt
BSur0XjkY6GQAAAIEA7wmaPA3lo4fT39bMcOH2ZY53qdlUdHpOGOIlMkVx+XghgAofbA7G
7dT5QVGxfeUlhSTZ77GWn4rlS2I+2iUdSOz5PPrnuzi81XmHbtA5LKpaufocBKnPxl/M5k
NK5ZIBscZGRcMEvS3gP/FYX8lqEZxjdZ3AVtH/uWmgvm1ZpoUAAACBAOVtvsGwTsMWg8ur
8k/qCPkvMCxX2W7OxXFCcl6CPWeIgOxD5eZVDX7ru1MoivJlKXqkgHhnotdhgvj4UnPHLY
GcNPCys7DulTxRR5cIkjM2QBfpzHoDprHIaUPrCBObt+CbUfBLQZ8xoygUBFsoZbZ2Yfne
S1Lv7p9e+yDv4V/nAAAAGm5pY2tAbG9jYWxob3N0LmxvY2FsZG9tYWlu
-----END OPENSSH PRIVATE KEY-----`
const known_hosts_dummy_test string = `github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==
	`

// TestSetRepoCredentials
// The goal here is to test the critical parts.
// What happens if the wrong type of credentials are provided.
// Ensure that the configuration is setted when valid credentials are provided
func TestBuildRepoConfig(t *testing.T) {

	// # Used on test #2.1 adn #2.2
	const randomUser = "random_user"
	const randomPassword = "random_password"
	secretBasicHTTPSAuth := map[string][]byte{
		corev1.BasicAuthUsernameKey: []byte(randomUser),
		corev1.BasicAuthPasswordKey: []byte(randomPassword),
	}
	secretBasicHTTPSNoPasswordAuth := map[string][]byte{
		corev1.BasicAuthUsernameKey: []byte(randomUser),
	}

	// Used on test #3.1 and #3.2
	secretSSHAuth := map[string][]byte{
		corev1.SSHAuthPrivateKey: []byte(id_rsa_test_random),
	}
	secretSSHKnowHostsAuth := map[string][]byte{
		corev1.SSHAuthPrivateKey: []byte(id_rsa_test_random),
		"known_hosts":            []byte(known_hosts_dummy_test),
	}

	// Prepare the test cases
	testCases := []struct {
		test            string
		secret          *corev1.Secret
		namespace       string
		name            string
		gitURL          string
		insecureSkipTLS bool
		caBundle        []byte
		expectErr       error
	}{
		{"#1 No Secret: Success", nil, "cattle-test-namespace", "charts-test", "https://somerandom.git", false, []byte{}, nil},

		{"#2.1 HTTPS Secret: Success", &corev1.Secret{
			Type: corev1.SecretTypeBasicAuth,
			Data: secretBasicHTTPSAuth,
		}, "cattle-test-namespace", "charts-test", "https://somerandom.git", false, []byte{}, nil},

		{"#2.2 HTTPS Secret: Failure", &corev1.Secret{
			Type: corev1.SecretTypeBasicAuth,
			Data: secretBasicHTTPSNoPasswordAuth,
		}, "cattle-test-namespace", "charts-test", "https://somerandom.git", false, []byte{}, fmt.Errorf("username or password not provided")},

		{"#3.1 SSH Secret: Success", &corev1.Secret{
			Type: corev1.SecretTypeSSHAuth,
			Data: secretSSHAuth,
		}, "cattle-test-namespace", "charts-test", "https://somerandom.git", false, []byte{}, nil},

		{"#3.2 SSH && Known Hosts Secret: Success", &corev1.Secret{
			Type: corev1.SecretTypeSSHAuth,
			Data: secretSSHKnowHostsAuth,
		}, "cattle-test-namespace", "charts-test", "https://somerandom.git", false, []byte{}, nil},
	}

	// Run the testCases
	for _, tc := range testCases {
		repo, err := BuildRepoConfig(tc.secret, tc.namespace, tc.name, tc.gitURL, tc.insecureSkipTLS, tc.caBundle)
		// Check the error
		if tc.expectErr == nil && tc.expectErr != err {
			t.Errorf("Expected error: %v |But got: %v", tc.expectErr, err)
		}
		// Only testing error in some cases
		if err != nil {
			assert.EqualError(t, tc.expectErr, err.Error())
			continue
		}

		// testing authentication methods
		if tc.secret == nil {
			assert.Nil(t, repo.auth, "Auth object should be nil")
		} else {
			assert.NotNil(t, repo.auth, "Auth object should not be nil")
			switch tc.secret.Type {
			case corev1.SecretTypeBasicAuth:
				storedAuth := repo.auth.String()
				assert.Equal(t, storedAuth, fmt.Sprintf("http-basic-auth - %s:*******", randomUser))
			case corev1.SecretTypeSSHAuth:
				storedAuth := repo.auth.String()
				assert.Equal(t, storedAuth, "user: git, name: ssh-public-keys")
			}
		}
		// testing local repository configurations
		assert.Equal(t, repo.fetchOpts.Depth, 1)
		assert.Equal(t, repo.cloneOpts.Depth, 1)
		assert.Contains(t, repo.Directory, tc.namespace, "Directory %s should contain the namespace %s", repo.Directory, tc.namespace)
		assert.Contains(t, repo.Directory, tc.name, "Directory %s should contain the chart name %s", repo.Directory, tc.name)
		assert.Equal(t, repo.URL, tc.gitURL)
		assert.Equal(t, repo.insecureTLSVerify, tc.insecureSkipTLS)
		assert.Equal(t, repo.caBundle, tc.caBundle)

	}
} // test end
