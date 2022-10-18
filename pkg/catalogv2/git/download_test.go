package git

import (
	"testing"

	assertlib "github.com/stretchr/testify/assert"
)

func Test_isGitSSH(t *testing.T) {
	testCases := []struct {
		gitURL   string
		expected bool
	}{
		// True cases
		{"customusername@github.com:user/repo.git", true},
		{"customusername@gitlab.com:user/repo.git", true},
		{"customusername@gitlab.com:user/repo", true},
		{"customusername@gitlab.com:user/repo-with-dashes.git", true},
		{"git@github.com:user/repo.git", true},
		{"git@gitlab.com:user/repo-with-dashes.git", true},
		{"git@gitlab.com:user/repo", true},
		// False cases
		{"https://github.com/user/repo.git", false},
		{"http://gitlab.com/user/repo.git", false},
		{"http://gitlab.com/user/repo", false},
		{"http://gitlab.com", false},
		{"git@gitlab.com", false},
	}
	assert := assertlib.New(t)
	for _, tc := range testCases {
		actual, err := isGitSSH(tc.gitURL)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		assert.Equalf(tc.expected, actual, "testcase: %v", tc)
	}
}
