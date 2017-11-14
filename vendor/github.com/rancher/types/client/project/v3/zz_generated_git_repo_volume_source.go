package client

const (
	GitRepoVolumeSourceType            = "gitRepoVolumeSource"
	GitRepoVolumeSourceFieldDirectory  = "directory"
	GitRepoVolumeSourceFieldRepository = "repository"
	GitRepoVolumeSourceFieldRevision   = "revision"
)

type GitRepoVolumeSource struct {
	Directory  string `json:"directory,omitempty"`
	Repository string `json:"repository,omitempty"`
	Revision   string `json:"revision,omitempty"`
}
