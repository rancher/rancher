package client

const (
	RepoPermType       = "repoPerm"
	RepoPermFieldAdmin = "admin"
	RepoPermFieldPull  = "pull"
	RepoPermFieldPush  = "push"
)

type RepoPerm struct {
	Admin bool `json:"admin,omitempty" yaml:"admin,omitempty"`
	Pull  bool `json:"pull,omitempty" yaml:"pull,omitempty"`
	Push  bool `json:"push,omitempty" yaml:"push,omitempty"`
}
