package client

const (
	RepoPermType       = "repoPerm"
	RepoPermFieldAdmin = "admin"
	RepoPermFieldPull  = "pull"
	RepoPermFieldPush  = "push"
)

type RepoPerm struct {
	Admin bool `json:"admin,omitempty"`
	Pull  bool `json:"pull,omitempty"`
	Push  bool `json:"push,omitempty"`
}
