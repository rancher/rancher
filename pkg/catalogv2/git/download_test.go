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
		commit          string
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
			commit:          "0e2b9da9ddde5c1e502bba6474119856496e5026",
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
			commit:          "0e2b9da9ddde5c1e502bba6474119856496e5026",
			insecureSkipTLS: false,
			caBundle:        []byte{},
			branch:          lastBranch,
			expectedError:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Ensure(tc.secret, tc.namespace, tc.name, tc.gitURL, tc.commit, tc.insecureSkipTLS, tc.caBundle)
			// Check the error
			if tc.expectedError == nil && tc.expectedError != err {
				t.Errorf("Expected error: %v |But got: %v", tc.expectedError, err)
			}

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
			branch:          mainBranch,
			expectedCommit:  "226d544def39de56db210e96d2b0b535badf9bdd",
			expectedError:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			commit, err := Head(tc.secret, tc.namespace, tc.name, tc.gitURL, tc.branch, tc.insecureSkipTLS, tc.caBundle)
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

func Test_Update(t *testing.T) {
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
			commit, err := Update(tc.secret, tc.namespace, tc.name, tc.gitURL, tc.branch, tc.insecureSkipTLS, tc.caBundle)
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
