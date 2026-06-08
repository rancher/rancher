package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToGitHubAccountNonStringOrgField(t *testing.T) {
	team := &GitHubTeam{
		ID:   1,
		Name: "my-team",
		Slug: "my-team-slug",
		Organization: map[string]any{
			"login":      "my-org",
			"avatar_url": 12345, // non-string value: would panic before the fix
		},
	}
	account := team.ToGitHubAccount("https://github.com/orgs/%s/teams/%s")
	// avatar_url was a non-string so stringFromMap should return ""
	assert.Equal(t, "", account.AvatarURL)
	// Login is the team slug, unaffected by the bad avatar_url value
	assert.Equal(t, "my-team-slug", account.Login)
}
