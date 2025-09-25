package common

import (
	"fmt"
)

// Account defines GitHub account properties.
type GitHubAccount struct {
	ID        int    `json:"id,omitempty"`
	Login     string `json:"login,omitempty"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	HTMLURL   string `json:"html_url,omitempty"`
	Type      string `json:"type,omitempty"`
}

// GitHubTeam defines GitHub team properties.
type GitHubTeam struct {
	ID           int            `json:"id,omitempty"`
	Organization map[string]any `json:"organization,omitempty"`
	Name         string         `json:"name,omitempty"`
	Slug         string         `json:"slug,omitempty"`
}

func (t *GitHubTeam) ToGitHubAccount(url string) GitHubAccount {
	orgLogin := stringFromMap(t.Organization, "login")
	avatarURL := stringFromMap(t.Organization, "avatar_url")

	return GitHubAccount{
		ID:        t.ID,
		Name:      t.Name,
		AvatarURL: avatarURL,
		HTMLURL:   fmt.Sprintf(url, orgLogin, t.Slug),
		Login:     t.Slug,
	}
}

func stringFromMap(m map[string]any, key string) string {
	r, ok := m[key]
	if !ok {
		return ""
	}

	return r.(string)
}
