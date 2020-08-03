package model

import (
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
)

type Remote interface {
	Type() string

	//Login handle oauth login
	Login(code string) (*v3.SourceCodeCredential, error)

	Repos(account *v3.SourceCodeCredential) ([]v3.SourceCodeRepository, error)

	CreateHook(pipeline *v3.Pipeline, accessToken string) (string, error)

	DeleteHook(pipeline *v3.Pipeline, accessToken string) error

	GetPipelineFileInRepo(repoURL string, ref string, accessToken string) ([]byte, error)

	SetPipelineFileInRepo(repoURL string, ref string, accessToken string, content []byte) error

	GetBranches(repoURL string, accessToken string) ([]string, error)

	GetHeadInfo(repoURL string, branch string, accessToken string) (*BuildInfo, error)
}

type Refresher interface {
	Refresh(cred *v3.SourceCodeCredential) (bool, error)
}
