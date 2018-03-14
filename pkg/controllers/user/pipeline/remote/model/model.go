package model

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"net/http"
)

type Remote interface {
	Type() string

	CanLogin() bool

	CanRepos() bool

	CanHook() bool

	//Login handle oauth login
	Login(redirectURL string, code string) (*v3.SourceCodeCredential, error)

	Repos(account *v3.SourceCodeCredential) ([]v3.SourceCodeRepository, error)

	CreateHook(pipeline *v3.Pipeline, accessToken string) (string, error)

	DeleteHook(pipeline *v3.Pipeline, accessToken string) error

	ParseHook(r *http.Request)

	GetPipelineFileInRepo(repoURL string, ref string, accessToken string) ([]byte, error)

	GetDefaultBranch(repoURL string, accessToken string) (string, error)
}
