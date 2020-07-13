package providers

import (
	"github.com/rancher/rancher/pkg/pipeline/remote"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/rancher/pkg/types/apis/project.cattle.io/v3"
	uuid "github.com/satori/go.uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func GetPipelineConfigByBranch(sourceCodeCredentials v3.SourceCodeCredentialInterface, sourceCodeCredentialLister v3.SourceCodeCredentialLister, pipeline *v3.Pipeline, branch string) (*v3.PipelineConfig, error) {
	credentialName := pipeline.Spec.SourceCodeCredentialName
	repoURL := pipeline.Spec.RepositoryURL
	_, projID := ref.Parse(pipeline.Spec.ProjectName)
	var scpConfig interface{}
	var credential *v3.SourceCodeCredential
	var err error
	if credentialName != "" {
		ns, name := ref.Parse(credentialName)
		credential, err = sourceCodeCredentialLister.Get(ns, name)
		if err != nil {
			return nil, err
		}
		sourceCodeType := credential.Spec.SourceCodeType
		scpConfig, err = GetSourceCodeProviderConfig(sourceCodeType, projID)
		if err != nil {
			return nil, err
		}
	}
	remote, err := remote.New(scpConfig)
	if err != nil {
		return nil, err
	}
	accessToken, err := utils.EnsureAccessToken(sourceCodeCredentials, remote, credential)
	if err != nil {
		return nil, err
	}
	content, err := remote.GetPipelineFileInRepo(repoURL, branch, accessToken)
	if err != nil {
		return nil, err
	}
	if content != nil {
		pipelineConfig, err := utils.PipelineConfigFromYaml(content)
		if err != nil {
			return nil, err
		}
		return pipelineConfig, nil
	}
	return nil, nil

}

func RefreshReposByCredential(sourceCodeRepositories v3.SourceCodeRepositoryInterface, sourceCodeRepositoryLister v3.SourceCodeRepositoryLister, sourceCodeCredentials v3.SourceCodeCredentialInterface, credential *v3.SourceCodeCredential, sourceCodeProviderConfig interface{}) ([]*v3.SourceCodeRepository, error) {
	namespace := credential.Namespace
	credentialID := ref.Ref(credential)

	remote, err := remote.New(sourceCodeProviderConfig)
	if err != nil {
		return nil, err
	}
	accessToken, err := utils.EnsureAccessToken(sourceCodeCredentials, remote, credential)
	if err != nil {
		return nil, err
	} else if accessToken != credential.Spec.AccessToken {
		credential.Spec.AccessToken = accessToken
	}
	repos, err := remote.Repos(credential)
	if err != nil {
		return nil, err
	}

	//remove old repos
	repositories, err := sourceCodeRepositoryLister.List(namespace, labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, repo := range repositories {
		if repo.Spec.SourceCodeCredentialName == credentialID {
			if err := sourceCodeRepositories.DeleteNamespaced(namespace, repo.Name, &metav1.DeleteOptions{}); err != nil {
				return nil, err
			}
		}
	}

	//store new repos
	for _, repo := range repos {
		if !repo.Spec.Permissions.Admin {
			//store only admin repos
			continue
		}
		repo.Spec.SourceCodeCredentialName = credentialID
		repo.Spec.UserName = credential.Spec.UserName
		repo.Spec.SourceCodeType = credential.Spec.SourceCodeType
		repo.Name = uuid.NewV4().String()
		repo.Namespace = namespace
		repo.Spec.ProjectName = credential.Spec.ProjectName
		if _, err := sourceCodeRepositories.Create(&repo); err != nil {
			return nil, err
		}
	}

	return repositories, nil
}
