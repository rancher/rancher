package pipeline

import (
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/satori/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func refreshReposByCredential(sourceCodeRepositories v3.SourceCodeRepositoryInterface, sourceCodeRepositoryLister v3.SourceCodeRepositoryLister, credential *v3.SourceCodeCredential) ([]*v3.SourceCodeRepository, error) {

	remoteType := credential.Spec.SourceCodeType

	mockConfig := v3.ClusterPipeline{
		Spec: v3.ClusterPipelineSpec{
			GithubConfig: &v3.GithubClusterConfig{},
		},
	}
	remote, err := remote.New(mockConfig, remoteType)
	if err != nil {
		return nil, err
	}
	repos, err := remote.Repos(credential)
	if err != nil {
		return nil, err
	}

	//remove old repos
	repositories, err := sourceCodeRepositoryLister.List("", labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, repo := range repositories {
		if repo.Spec.SourceCodeCredentialName == credential.Name {
			if err := sourceCodeRepositories.Delete(repo.Name, &metav1.DeleteOptions{}); err != nil {
				return nil, err
			}
		}
	}

	//store new repos
	for _, repo := range repos {
		repo.Spec.SourceCodeCredentialName = credential.Name
		repo.Spec.ClusterName = credential.Spec.ClusterName
		repo.Spec.UserName = credential.Spec.UserName
		repo.Name = uuid.NewV4().String()
		if _, err := sourceCodeRepositories.Create(&repo); err != nil {
			return nil, err
		}
	}

	return repositories, nil
}
