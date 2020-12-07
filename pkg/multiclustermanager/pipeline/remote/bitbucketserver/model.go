package bitbucketserver

type Links struct {
	Self   []Link `json:"self"`
	HTML   Link   `json:"html"`
	Avatar Link   `json:"avatar"`
	Clone  []Link `json:"clone"`
}

type Link struct {
	Href string `json:"href"`
	Name string `json:"name"`
}

type Repository struct {
	Slug          string  `json:"slug"`
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	ScmID         string  `json:"scmId"`
	State         string  `json:"state"`
	StatusMessage string  `json:"statusMessage"`
	Forkable      bool    `json:"forkable"`
	Project       Project `json:"project"`
	Public        bool    `json:"public"`
	Links         Links   `json:"links"`
}

type Project struct {
	Key    string `json:"key"`
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Public bool   `json:"public"`
	Type   string `json:"type"`
	Links  Links  `json:"links"`
}

type paging struct {
	Size          int  `json:"size"`
	Limit         int  `json:"limit"`
	Start         int  `json:"start"`
	IsLastPage    bool `json:"isLastPage"`
	NextPageStart int  `json:"nextPageStart"`
}

type PaginatedRepositories struct {
	paging
	Values []Repository `json:"values"`
}

type User struct {
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	ID           int    `json:"id"`
	DisplayName  string `json:"displayName"`
	Active       bool   `json:"active"`
	Slug         string `json:"slug"`
	Type         string `json:"type"`
	Links        Links  `json:"links"`
}

type AccessToken struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	User        User     `json:"user"`
	Token       string   `json:"token"`
}

type PaginatedAccessToken struct {
	paging
	Values []AccessToken `json:"values"`
}

type Hook struct {
	ID            int               `json:"id"`
	Name          string            `json:"name"`
	Events        []string          `json:"events"`
	Configuration HookConfiguration `json:"configuration"`
	URL           string            `json:"url"`
	Active        bool              `json:"active"`
}

type HookConfiguration struct {
	Secret string `json:"secret"`
}

type PaginatedHooks struct {
	paging
	Values []Hook `json:"values"`
}

type Branch struct {
	ID              string `json:"id"`
	DisplayID       string `json:"displayId"`
	Type            string `json:"type"`
	LatestCommit    string `json:"latestCommit"`
	LatestChangeset string `json:"latestChangeset"`
	IsDefault       bool   `json:"isDefault"`
}

type PaginatedBranches struct {
	paging
	Values []Branch `json:"values"`
}

type Commit struct {
	ID        string `json:"id"`
	DisplayID string `json:"displayId"`
	Author    User   `json:"author"`
	Committer User   `json:"committer"`
	Message   string `json:"message"`
}

type PushEventPayload struct {
	EventKey   string     `json:"eventKey"`
	Date       string     `json:"date"`
	Actor      User       `json:"actor"`
	Repository Repository `json:"repository"`
	Changes    []Change   `json:"changes"`
}

type Change struct {
	Ref struct {
		ID        string `json:"id"`
		DisplayID string `json:"displayId"`
		Type      string `json:"type"`
	} `json:"ref"`
	RefID    string `json:"refId"`
	FromHash string `json:"fromHash"`
	ToHash   string `json:"toHash"`
	Type     string `json:"type"`
}

type PullRequestEventPayload struct {
	EventKey    string      `json:"eventKey"`
	Date        string      `json:"date"`
	Actor       User        `json:"actor"`
	PullRequest PullRequest `json:"pullRequest"`
}

type PullRequest struct {
	ID          int    `json:"id"`
	Version     int    `json:"version"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	Open        bool   `json:"open"`
	Closed      bool   `json:"closed"`
	FromRef     Ref    `json:"fromRef"`
	ToRef       Ref    `json:"toRef"`
	Locked      bool   `json:"locked"`
	Author      struct {
		User     User   `json:"user"`
		Role     string `json:"role"`
		Approved bool   `json:"approved"`
		Status   string `json:"status"`
	} `json:"author"`
	Links Links `json:"links"`
}

type Ref struct {
	ID           string     `json:"id"`
	DisplayID    string     `json:"displayId"`
	LatestCommit string     `json:"latestCommit"`
	Repository   Repository `json:"repository"`
}

type LastModified struct {
	Files        map[string]Commit `json:"files"`
	LatestCommit Commit            `json:"latestCommit"`
}
