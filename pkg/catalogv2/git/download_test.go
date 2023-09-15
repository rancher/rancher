package git

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

const chartsSmallForkURL = "https://github.com/rancher/charts-small-fork"
const mainBranch = "main"
const lastBranch = "test-1"

func TestMain(m *testing.M) {
	// Run all the tests
	exitCode := m.Run()

	// Cleanup after tests
	cleanup()

	// Exit with the proper code
	os.Exit(exitCode)
}

func cleanup() {
	// Delete the management-state directory
	os.RemoveAll("management-state")
}

func Test_Ensure(t *testing.T) {
	testCases := []struct {
		test            string
		secret          *corev1.Secret
		namespace       string
		name            string
		gitURL          string
		insecureSkipTLS bool
		caBundle        []byte
		branch          string
		expectedError   error
	}{
		{
			test:            "#1 TestCase: Success - Clone, Reset And Exit",
			secret:          nil,
			namespace:       "cattle-test",
			name:            "small-fork-test",
			gitURL:          chartsSmallForkURL,
			insecureSkipTLS: false,
			caBundle:        []byte{},
			branch:          mainBranch,
			expectedError:   nil,
		},
		{
			test:            "#2 TestCase: Success - Clone, Reset And Fetch Last Branch",
			secret:          nil,
			namespace:       "cattle-test",
			name:            "small-fork-test",
			gitURL:          chartsSmallForkURL,
			insecureSkipTLS: false,
			caBundle:        []byte{},
			branch:          lastBranch,
			expectedError:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, err := BuildRepoConfig(tc.secret, tc.namespace, tc.name, tc.gitURL, tc.insecureSkipTLS, tc.caBundle)
			// Check the error
			if tc.expectedError == nil && tc.expectedError != err {
				t.Errorf("Expected error: %v |But got: %v", tc.expectedError, err)
			}

			err = repo.Ensure(tc.branch)
			// Check the error
			if tc.expectedError == nil && tc.expectedError != err {
				t.Errorf("Expected error: %v |But got: %v", tc.expectedError, err)
			}
			// Only testing error in some cases
			if err != nil {
				assert.EqualError(t, tc.expectedError, err.Error())
			}
		})
	}
}

func Test_Head(t *testing.T) {
	testCases := []struct {
		test            string
		secret          *corev1.Secret
		namespace       string
		name            string
		gitURL          string
		insecureSkipTLS bool
		caBundle        []byte
		branch          string
		expectedCommit  string
		expectedError   error
	}{
		{
			test:            "#1 TestCase: Success - Clone, Reset And Return Commit",
			secret:          nil,
			namespace:       "cattle-test",
			name:            "small-fork-test",
			gitURL:          chartsSmallForkURL,
			insecureSkipTLS: false,
			caBundle:        []byte{},
			branch:          lastBranch,
			expectedCommit:  "226d544def39de56db210e96d2b0b535badf9bdd",
			expectedError:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, err := BuildRepoConfig(tc.secret, tc.namespace, tc.name, tc.gitURL, tc.insecureSkipTLS, tc.caBundle)
			// Check the error
			if tc.expectedError == nil && tc.expectedError != err {
				t.Errorf("Expected error: %v |But got: %v", tc.expectedError, err)
			}

			commit, err := repo.Head(tc.branch)
			// Check the error
			if tc.expectedError == nil && tc.expectedError != err {
				t.Errorf("Expected error: %v |But got: %v", tc.expectedError, err)
			}
			// Only testing error in some cases
			if err != nil {
				assert.EqualError(t, tc.expectedError, err.Error())
			}

			assert.Equal(t, len(commit), len(tc.expectedCommit))
		})
	}
}

func Test_CheckUpdate(t *testing.T) {
	testCases := []struct {
		test              string
		secret            *corev1.Secret
		namespace         string
		name              string
		gitURL            string
		insecureSkipTLS   bool
		caBundle          []byte
		branch            string
		systemCatalogMode string
		expectedCommit    string
		expectedError     error
	}{
		{
			test:              "#1 TestCase: Success ",
			secret:            nil,
			namespace:         "cattle-test",
			name:              "small-fork-test",
			gitURL:            chartsSmallForkURL,
			insecureSkipTLS:   false,
			caBundle:          []byte{},
			branch:            lastBranch,
			systemCatalogMode: "",
			expectedCommit:    "226d544def39de56db210e96d2b0b535badf9bdd",
			expectedError:     nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, err := BuildRepoConfig(tc.secret, tc.namespace, tc.name, tc.gitURL, tc.insecureSkipTLS, tc.caBundle)
			// Check the error
			if tc.expectedError == nil && tc.expectedError != err {
				t.Errorf("Expected error: %v |But got: %v", tc.expectedError, err)
			}

			commit, err := repo.CheckUpdate(tc.branch, tc.systemCatalogMode)
			// Check the error
			if tc.expectedError == nil && tc.expectedError != err {
				t.Errorf("Expected error: %v |But got: %v", tc.expectedError, err)
			}
			// Only testing error in some cases
			if err != nil {
				assert.EqualError(t, tc.expectedError, err.Error())
			}

			assert.Equal(t, len(commit), len(tc.expectedCommit))
		})
	}
}
