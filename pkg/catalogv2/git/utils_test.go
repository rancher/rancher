package git

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	assertlib "github.com/stretchr/testify/assert"
)

func Test_validateGitURL(t *testing.T) {
	testCases := []struct {
		gitURL        string
		expectedValid bool
		expectedError error
	}{
		// Valid URL's
		{
			gitURL:        "customusername@gitlab.com:user/repo.git",
			expectedValid: true,
			expectedError: nil,
		},
		{
			gitURL:        "customusername@gitlab.com:user/repo",
			expectedValid: true,
			expectedError: nil,
		},
		{
			gitURL:        "customusername@gitlab.com:user/repo-with-dashes.git",
			expectedValid: true,
			expectedError: nil,
		},
		{
			gitURL:        "git@github.com:user/repo.git",
			expectedValid: true,
			expectedError: nil,
		},
		{
			gitURL:        "git@gitlab.com:user/repo-with-dashes.git",
			expectedValid: true,
			expectedError: nil,
		},
		{
			gitURL:        "git@gitlab.com:user/repo",
			expectedValid: true,
			expectedError: nil,
		},
		// Invalid URL's
		{
			gitURL:        "ftp://admantium@gitlab.com:user/repo",
			expectedError: fmt.Errorf("only http(s) or ssh protocols supported"),
		},
		{
			gitURL:        "https://admantium@gitlab.com:user/repo",
			expectedError: fmt.Errorf("only http(s) or ssh protocols supported"),
		},
		{
			gitURL:        "https://admantium#gitlab.com:user/repo",
			expectedError: fmt.Errorf("only http(s) or ssh protocols supported"),
		},
	}
	assert := assertlib.New(t)
	for _, tc := range testCases {
		valid, err := validateGitURL(tc.gitURL)
		if err != nil {
			assert.EqualErrorf(tc.expectedError, err.Error(), "testcase: %v", tc)
			continue
		}
		assert.Equalf(tc.expectedValid, valid, "testcase: %v", tc)
	}
}

func Test_gitDir(t *testing.T) {
	assert := assertlib.New(t)
	testCases := []struct {
		namespace string
		name      string
		gitURL    string
		expected  string
	}{
		{
			"namespace", "name", "https://git.rancher.io/charts",
			"management-state/git-repo/namespace/name/4b40cac650031b74776e87c1a726b0484d0877c3ec137da0872547ff9b73a721",
		},
		// NOTE(manno): cannot test the other cases without poluting the filesystem
	}
	for _, tc := range testCases {
		actual := gitDir(tc.namespace, tc.name, tc.gitURL)
		assert.Equalf(tc.expected, actual, "testcase: %v", tc)
	}
}

func Test_parseUserFromSSHURL(t *testing.T) {

	testCases := []struct {
		test             string
		URL              string
		expectedUsername string
		expectedError    error
	}{
		{
			test:             "1.0 Valid compact SSH URL Success",
			URL:              "user@server:project.git",
			expectedUsername: "user",
			expectedError:    nil,
		},
		{
			test:             "1.1 Valid compact SSH URL Success",
			URL:              "ssh://user@mydomain.example:443/repository-name",
			expectedUsername: "user",
			expectedError:    nil,
		},
		{
			test:             "2.0 Invalid compact SSH URL",
			URL:              "user@mydomain.example@443/repository-name",
			expectedUsername: "user",
			expectedError:    fmt.Errorf("invalid ssh url: user@mydomain.example@443/repository-name"),
		},
		{
			test:             "2.1 Invalid compact SSH URL No @ character",
			URL:              "user#mydomain.example:443/repository-name",
			expectedUsername: "user",
			expectedError:    fmt.Errorf("invalid ssh url: user#mydomain.example:443/repository-name"),
		},
	}

	for _, tc := range testCases {
		user, err := parseUserFromSSHURL(tc.URL)
		if err != nil {
			assert.EqualError(t, tc.expectedError, err.Error())
			continue
		}
		assert.Equal(t, tc.expectedUsername, user)
	}
}
