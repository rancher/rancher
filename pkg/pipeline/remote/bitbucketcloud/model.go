package bitbucketcloud

type User struct {
	UserName    string `json:"username"`
	Website     string `json:"website"`
	DisplayName string `json:"display_name"`
	AccountID   string `json:"account_id"`
	Links       *Links `json:"links"`
}

type Links struct {
	HTML   Link   `json:"html"`
	Avatar Link   `json:"avatar"`
	Clone  []Link `json:"clone"`
}

type Link struct {
	Href string `json:"href"`
	Name string `json:"name"`
}

type Repository struct {
	Scm         string `json:"scm"`
	Website     string `json:"website"`
	HasWiki     bool   `json:"has_wiki"`
	Name        string `json:"name"`
	Links       *Links `json:"links"`
	ForkPolicy  string `json:"fork_policy"`
	Language    string `json:"language"`
	MainBranch  Object `json:"mainbranch"`
	FullName    string `json:"full_name"`
	HasIssues   bool   `json:"has_issues"`
	Owner       *User  `json:"owner"`
	IsPrivate   bool   `json:"is_private"`
	Description string `json:"description"`
}

type PaginatedRepositories struct {
	Paging
	Values []Repository `json:"values"`
}

type Paging struct {
	Size       int    `json:"size"`
	Page       int    `json:"page"`
	PageLength int    `json:"pagelen"`
	Next       string `json:"next"`
	Previous   string `json:"previous"`
}

type Hook struct {
	UUID                 string   `json:"uuid"`
	Description          string   `json:"description"`
	Links                *Links   `json:"links"`
	URL                  string   `json:"url"`
	SkipCertVerification bool     `json:"skip_cert_verification"`
	Active               bool     `json:"active"`
	Events               []string `json:"events"`
}

type PaginatedHooks struct {
	Paging
	Values []Hook `json:"values"`
}

type Ref struct {
	Type   string  `json:"type"`
	Name   string  `json:"name"`
	Links  *Links  `json:"links"`
	Target *Target `json:"target"`
}

type PaginatedBranches struct {
	Paging
	Values []Ref `json:"values"`
}

type Target struct {
	Hash       string      `json:"hash"`
	Repository *Repository `json:"repository"`
	Links      *Links      `json:"links"`
	Author     *Author     `json:"author"`
	Date       string      `json:"date"`
	Message    string      `json:"message"`
}

type Author struct {
	Raw  string `json:"raw"`
	User *User  `json:"user"`
}

type Object struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type PushEventPayload struct {
	Actor      User       `json:"actor"`
	Repository Repository `json:"repository"`
	Push       struct {
		Changes []Change `json:"changes"`
	} `json:"push"`
}

type Change struct {
	New Ref `json:"new"`
}

type PullRequestEventPayload struct {
	Actor       User        `json:"actor"`
	PullRequest PullRequest `json:"pullrequest"`
	Repository  Repository  `json:"repository"`
}

type PullRequest struct {
	ID          int                 `json:"id"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	State       string              `json:"state"`
	Author      User                `json:"author"`
	Source      PullRequestEndpoint `json:"source"`
	Destination PullRequestEndpoint `json:"destination"`
	Links       Links               `json:"links"`
	Created     string              `json:"created_on"`
	Updated     string              `json:"updated_on"`
}

type PullRequestEndpoint struct {
	Branch struct {
		Name string `json:"name"`
	} `json:"branch"`
	Commit struct {
		Hash string `json:"hash"`
	} `json:"commit"`
	Repository Repository `json:"repository"`
}
