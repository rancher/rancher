package git

import (
	"testing"

	"github.com/gliderlabs/ssh"
	plumbing "github.com/go-git/go-git/v5/plumbing"
	plumbingHTTP "github.com/go-git/go-git/v5/plumbing/transport/http"
	plumbingSSH "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	gomock "github.com/golang/mock/gomock"
	"github.com/pkg/errors"
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
func TestSetRepoCredentials(t *testing.T) {

	// Prepare the test cases
	testCases := []struct {
		name       string
		secret     *corev1.Secret
		username   string
		password   string
		sshKey     string
		knownHosts string
		expectErr  bool
	}{
		{"#1 No Auth: Success", nil, "", "", "", "", false},
		{"#2.1 Basic Auth: Success", &corev1.Secret{Type: corev1.SecretTypeBasicAuth}, "random_user", "random_password", "", "", false},
		{"#2.2 Basic Auth: Failure", &corev1.Secret{Type: corev1.SecretTypeBasicAuth}, "", "", "", "", true},
		{"#3.1 SSH Auth: Success", &corev1.Secret{Type: corev1.SecretTypeSSHAuth}, "", "", id_rsa_test_random, known_hosts_dummy_test, false},
		{"#3.2 SSH Auth: Failure", &corev1.Secret{Type: corev1.SecretTypeSSHAuth}, "", "", "", known_hosts_dummy_test, true},
	}

	// Run the testCases
	for _, tc := range testCases {
		if tc.secret != nil {
			tc.secret.Data = map[string][]byte{
				corev1.BasicAuthUsernameKey: []byte(tc.username),
				corev1.BasicAuthPasswordKey: []byte(tc.password),
				corev1.SSHAuthPrivateKey:    []byte(tc.sshKey),
				"known_hosts":               []byte(tc.knownHosts),
			}
		}

		config := &git{
			secret: tc.secret,
		}

		// Create new repoOperation
		ro := &repoOperation{
			config: config,
		}

		// Run the function
		err := ro.setRepoCredentials()

		// Check the error
		if tc.expectErr && err == nil {
			t.Errorf("Expected an error for case %+v, but got nil", tc)
		} else if !tc.expectErr && err != nil {
			t.Errorf("Did not expect an error for case %+v, but got: %v", tc, err)
		}

		auth := ro.GetAuth()
		switch authType := auth.(type) {
		case *plumbingHTTP.BasicAuth:
			if tc.secret.Type == corev1.SecretTypeBasicAuth {
				if authType.Username != tc.username || authType.Password != tc.password {
					t.Errorf("Expected username/password: %s/%s, but got: %s/%s",
						tc.username, tc.password, authType.Username, authType.Password)
				}

			}
		case *plumbingSSH.PublicKeys:
			if tc.secret.Type == corev1.SecretTypeSSHAuth {
				if authType.User != "git" {
					t.Errorf("Expected user: %s, but got: %s", "git", authType.User)
				}
				// Check if Signer is non-nil and of type ssh.Signer
				_, ok := authType.Signer.(ssh.Signer)
				if !ok || authType.Signer == nil {
					t.Errorf("Failed to parse SSH private key")
				}
				// Check that HostKeyCallback is not nil
				if authType.HostKeyCallbackHelper.HostKeyCallback == nil {
					t.Errorf("HostKeyCallback is nil")
				}
			}
		}

	} // switch end
} // test end

func Test_EnsureClonedRepo(t *testing.T) {
	testCases := []struct {
		name               string
		call_openOrClone   bool
		call_hardReset     bool
		call_fetchAndReset bool
		openOrClone_err    error
		hardReset_err      error
		fetchAndReset_err  error
		expected_err       error
	}{
		{
			name:               "#1 TestCase: Success - Open, Reset And Exit",
			call_openOrClone:   true,
			call_hardReset:     true,
			call_fetchAndReset: false,
			openOrClone_err:    nil,
			hardReset_err:      nil,
			// fetchAndReset_err:,
			expected_err: nil,
		},
		{
			name:               "#2 TestCase: Success - Open, Reset And FetchReset",
			call_openOrClone:   true,
			call_hardReset:     true,
			call_fetchAndReset: true,
			openOrClone_err:    nil,
			hardReset_err:      errTestCustom,
			fetchAndReset_err:  nil,
			expected_err:       nil,
		},
		{
			name:               "#3 TestCase: Failure - Clone And Fail",
			call_openOrClone:   true,
			call_hardReset:     false,
			call_fetchAndReset: false,
			openOrClone_err:    errTestCustom,
			// hardReset_err:          nil,
			// fetchAndReset_err:  nil,
			expected_err: errTestCustom,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Set up Mock controller
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := NewMockrepository(ctrl)

			// Set Expectations according to calls
			if tc.call_openOrClone {
				mockRepo.EXPECT().cloneOrOpen(gomock.Any()).Return(tc.openOrClone_err)
			}
			if tc.call_hardReset {
				mockRepo.EXPECT().hardReset(gomock.Any()).Return(tc.hardReset_err)
			}
			if tc.call_fetchAndReset {
				mockRepo.EXPECT().fetchAndReset(gomock.Any()).Return(tc.fetchAndReset_err)
			}

			// config, _ := newGitForRepoTest()
			// ro, _ := newRepoOperation(config)
			git := newRepository(&extendedRepo{mockRepo})
			commit := "imagine_some_crazy_sha_here"
			err := git.EnsureClonedRepo(commit)
			if (err != nil && err.Error() != tc.expected_err.Error()) || (err == nil && tc.expected_err != nil) {
				t.Errorf("got error: %v, want %v", err, tc.expected_err)
			}
		})
	}
}

func Test_CloneHead(t *testing.T) {
	someCommitHash := plumbing.NewHash("9c5a25675676b680898f933681abde88f53dba95")

	testCases := []struct {
		name                  string
		call_cloneOrOpen      bool
		call_hardReset        bool
		call_getCurrentCommit bool
		openOrClone_err       error
		hardReset_err         error
		getCurrentCommit_resp plumbing.Hash
		getCurrentCommit_err  error
		expected_commit       string
		expected_err          error
	}{
		{
			name:                  "#1 TestCase: Success - First Clone || Dont Clone",
			call_cloneOrOpen:      true,
			call_hardReset:        true,
			call_getCurrentCommit: true,
			openOrClone_err:       nil,
			hardReset_err:         nil,
			getCurrentCommit_resp: someCommitHash,
			getCurrentCommit_err:  nil,
			expected_commit:       someCommitHash.String(),
			expected_err:          nil,
		},
		{
			name:                  "#2 TestCase: Failure - hardReset(!clone)",
			call_cloneOrOpen:      true,
			call_hardReset:        true,
			call_getCurrentCommit: false,
			openOrClone_err:       nil,
			hardReset_err:         errTestCustom,
			// getCurrentCommit_resp: commitHash,
			// getCurrentCommit_err:  nil,
			expected_commit: "",
			expected_err:    errTestCustom,
		},
		{
			name:                  "#3 TestCase: Failure - cloneOrOpen(err)",
			call_cloneOrOpen:      true,
			call_hardReset:        false,
			call_getCurrentCommit: false,
			openOrClone_err:       errTestCustom,
			// hardReset_err:         errTestCustom,
			// getCurrentCommit_resp: commitHash,
			// getCurrentCommit_err:  nil,
			expected_commit: "",
			expected_err:    errTestCustom,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Set up Mock controller
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := NewMockrepository(ctrl)

			// Set Expectations according to calls
			if tc.call_cloneOrOpen {
				mockRepo.EXPECT().cloneOrOpen(gomock.Any()).Return(tc.openOrClone_err)
			}
			if tc.call_hardReset {
				mockRepo.EXPECT().hardReset(gomock.Any()).Return(tc.hardReset_err)
			}
			if tc.call_getCurrentCommit {
				mockRepo.EXPECT().getCurrentCommit().Return(tc.getCurrentCommit_resp, tc.getCurrentCommit_err)
			}

			git := newRepository(&extendedRepo{mockRepo})
			branch := "main"
			commit, err := git.CloneHead(branch)
			if (err != nil && err.Error() != tc.expected_err.Error()) || (err == nil && tc.expected_err != nil) {
				t.Errorf("got error: %v, want %v", err, tc.expected_err)
			}

			if commit != tc.expected_commit {
				t.Errorf("CloneHead() = %v; want '%v'", commit, tc.expected_commit)
			}
		})
	}
}

func Test_UpdateToLatestRef(t *testing.T) {
	currentCommitHash := plumbing.NewHash("9c5a25675676b680898f933681abde88f53dba95")
	newCommitHash := plumbing.NewHash("9c5a25675676b680898f933681abde88f53dba99")

	testCases := []struct {
		name                   string
		call_cloneOrOpen       bool
		call_hardReset         bool
		call_getCurrentCommit  bool
		call_getLastCommitHash bool
		call_fetchAndReset     bool
		call_getCurrentCommit2 bool
		openOrClone_err        error
		hardReset_err          error
		getCurrentCommit_resp  plumbing.Hash
		getCurrentCommit_err   error
		getLastCommitHash_resp plumbing.Hash
		getLastCommitHash_err  error
		fetchAndReset_err      error
		getCurrentCommit2_resp plumbing.Hash
		getCurrentCommit2_err  error
		expected_commit        string
		expected_err           error
	}{
		{
			name:                   "#1 TestCase: Success - remote commit HASH not changed",
			call_cloneOrOpen:       true,
			call_hardReset:         true,
			call_getCurrentCommit:  true,
			call_getLastCommitHash: true,
			call_fetchAndReset:     false,
			call_getCurrentCommit2: false,
			openOrClone_err:        nil,
			hardReset_err:          nil,
			getCurrentCommit_resp:  currentCommitHash,
			getCurrentCommit_err:   nil,
			getLastCommitHash_resp: currentCommitHash,
			getLastCommitHash_err:  nil,
			// fetchAndReset_err:  nil,
			// getCurrentCommit2_resp: currentCommitHash,
			// getCurrentCommit2_err:  nil,
			expected_commit: "9c5a25675676b680898f933681abde88f53dba95",
			expected_err:    nil,
		},
		{
			name:                   "#2 TestCase: Success - remoteSHA changed",
			call_cloneOrOpen:       true,
			call_hardReset:         true,
			call_getCurrentCommit:  true,
			call_getLastCommitHash: true,
			call_fetchAndReset:     true,
			call_getCurrentCommit2: true,
			openOrClone_err:        nil,
			hardReset_err:          nil,
			getCurrentCommit_resp:  currentCommitHash,
			getCurrentCommit_err:   nil,
			getLastCommitHash_resp: newCommitHash,
			getLastCommitHash_err:  nil,
			fetchAndReset_err:      nil,
			getCurrentCommit2_resp: newCommitHash,
			getCurrentCommit2_err:  nil,
			expected_commit:        newCommitHash.String(),
			expected_err:           nil,
		},
		{
			name:                   "#3 TestCase: Success - fetchAndResetHead error",
			call_cloneOrOpen:       true,
			call_hardReset:         true,
			call_getCurrentCommit:  true,
			call_getLastCommitHash: true,
			call_fetchAndReset:     true,
			call_getCurrentCommit2: false,
			openOrClone_err:        nil,
			hardReset_err:          nil,
			getCurrentCommit_resp:  currentCommitHash,
			getCurrentCommit_err:   nil,
			getLastCommitHash_resp: newCommitHash,
			getLastCommitHash_err:  nil,
			fetchAndReset_err:      errTestCustom,
			// getCurrentCommit2_resp: "some-random-sha",
			// getCurrentCommit2_err:  nil,
			expected_commit: currentCommitHash.String(),
			expected_err:    errTestCustom,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Set up Mock controller
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := NewMockrepository(ctrl)

			// Set Expectations according to calls
			if tc.call_cloneOrOpen {
				mockRepo.EXPECT().cloneOrOpen(gomock.Any()).Return(tc.openOrClone_err)
			}
			if tc.call_hardReset {
				mockRepo.EXPECT().hardReset(gomock.Any()).Return(tc.hardReset_err)
			}
			if tc.call_getCurrentCommit {
				mockRepo.EXPECT().getCurrentCommit().Return(tc.getCurrentCommit_resp, tc.getCurrentCommit_err)
			}
			if tc.call_getLastCommitHash {
				mockRepo.EXPECT().getLastCommitHash(gomock.Any(), gomock.Any()).Return(tc.getLastCommitHash_resp, tc.getLastCommitHash_err)
			}
			if tc.call_fetchAndReset {
				mockRepo.EXPECT().fetchAndReset(gomock.Any()).Return(tc.fetchAndReset_err)
			}
			if tc.call_getCurrentCommit2 {
				mockRepo.EXPECT().getCurrentCommit().Return(tc.getCurrentCommit2_resp, tc.getCurrentCommit2_err)
			}

			git := newRepository(&extendedRepo{mockRepo})
			branch := "main"
			commit, err := git.UpdateToLatestRef(branch)
			if (err != nil && err.Error() != tc.expected_err.Error()) || (err == nil && tc.expected_err != nil) {
				t.Errorf("got error: %v, want %v", err, tc.expected_err)
			}

			if commit != tc.expected_commit {
				t.Errorf("UpdateToLatestRef() = %v; want '%v'", commit, tc.expected_commit)
			}
		})
	}
}
