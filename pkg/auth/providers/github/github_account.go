package github

import (
	"fmt"
)

type searchResult struct {
	Items []Account `json:"items"`
}

// Account defines properties an account on github has
type Account struct {
	ID        int    `json:"id,omitempty"`
	Login     string `json:"login,omitempty"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	HTMLURL   string `json:"html_url,omitempty"`
	Type      string `json:"type,omitempty"`
}

// Team defines properties a team on github has
type Team struct {
	ID           int                    `json:"id,omitempty"`
	Organization map[string]interface{} `json:"organization,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Slug         string                 `json:"slug,omitempty"`
}

func (t *Team) toGithubAccount(url string, account *Account) {
	account.ID = t.ID
	account.Name = t.Name
	orgLogin := (t.Organization["login"]).(string)
	account.AvatarURL = t.Organization["avatar_url"].(string)
	account.HTMLURL = fmt.Sprintf(url, orgLogin, t.Slug)
	account.Login = t.Slug
}
