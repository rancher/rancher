package common

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/pipeline/remote"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	v1 "github.com/rancher/rancher/pkg/types/apis/core/v1"
	v3 "github.com/rancher/rancher/pkg/types/apis/project.cattle.io/v3"
	client "github.com/rancher/rancher/pkg/types/client/project/v3"
	uuid "github.com/satori/go.uuid"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	projectNameField = "projectName"
	hyphenChar       = "-"
	dashChar         = "_"
)

type BaseProvider struct {
	SourceCodeProviderConfigs  v3.SourceCodeProviderConfigInterface
	SourceCodeCredentials      v3.SourceCodeCredentialInterface
	SourceCodeCredentialLister v3.SourceCodeCredentialLister
	SourceCodeRepositories     v3.SourceCodeRepositoryInterface
	Pipelines                  v3.PipelineInterface
	PipelineExecutions         v3.PipelineExecutionInterface
	NamespaceLister            v1.NamespaceLister
	Namespaces                 v1.NamespaceInterface

	PipelineIndexer             cache.Indexer
	PipelineExecutionIndexer    cache.Indexer
	SourceCodeCredentialIndexer cache.Indexer
	SourceCodeRepositoryIndexer cache.Indexer
}

func (b BaseProvider) TransformToSourceCodeProvider(config map[string]interface{}, providerType string) map[string]interface{} {
	result := map[string]interface{}{}
	if m, ok := config["metadata"].(map[string]interface{}); ok {
		result["id"] = fmt.Sprintf("%v:%v", m[client.ObjectMetaFieldNamespace], m[client.ObjectMetaFieldName])
	}
	if t := convert.ToString(config[client.SourceCodeProviderFieldType]); t != "" {
		result[client.SourceCodeProviderFieldType] = providerType
	}
	if t := convert.ToString(config[projectNameField]); t != "" {
		result["projectId"] = t
	}

	return result
}

func (b BaseProvider) Cleanup(projectID string, sourceCodeType string) error {
	pipelines, err := b.PipelineIndexer.ByIndex(utils.PipelineByProjectIndex, projectID)
	if err != nil {
		return err
	}
	for _, p := range pipelines {

		pipeline, _ := p.(*v3.Pipeline)
		if err := b.Pipelines.DeleteNamespaced(pipeline.Namespace, pipeline.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	pipelineExecutions, err := b.PipelineExecutionIndexer.ByIndex(utils.PipelineExecutionByProjectIndex, projectID)
	if err != nil {
		return err
	}
	for _, e := range pipelineExecutions {
		execution, _ := e.(*v3.PipelineExecution)
		if err := b.PipelineExecutions.DeleteNamespaced(execution.Namespace, execution.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	crdKey := utils.ProjectNameAndSourceCodeTypeKey(projectID, sourceCodeType)
	credentials, err := b.SourceCodeCredentialIndexer.ByIndex(utils.SourceCodeCredentialByProjectAndTypeIndex, crdKey)
	if err != nil {
		return err
	}
	for _, c := range credentials {
		credential, _ := c.(*v3.SourceCodeCredential)
		if err := b.SourceCodeCredentials.DeleteNamespaced(credential.Namespace, credential.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	repoKey := utils.ProjectNameAndSourceCodeTypeKey(projectID, sourceCodeType)
	repositories, err := b.SourceCodeRepositoryIndexer.ByIndex(utils.SourceCodeRepositoryByProjectAndTypeIndex, repoKey)
	if err != nil {
		return err
	}
	for _, r := range repositories {
		repo, _ := r.(*v3.SourceCodeRepository)
		if err := b.SourceCodeRepositories.DeleteNamespaced(repo.Namespace, repo.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (b BaseProvider) DisableAction(request *types.APIContext, sourceCodeType string) error {
	ns, _ := ref.Parse(request.ID)
	o, err := b.SourceCodeProviderConfigs.ObjectClient().UnstructuredClient().GetNamespaced(ns, sourceCodeType, metav1.GetOptions{})
	if err != nil {
		return err
	}
	u, _ := o.(runtime.Unstructured)
	config := u.UnstructuredContent()
	if convert.ToBool(config[client.SourceCodeProviderConfigFieldEnabled]) {
		config[client.SourceCodeProviderConfigFieldEnabled] = false
		if _, err := b.SourceCodeProviderConfigs.ObjectClient().Update(sourceCodeType, o); err != nil {
			return err
		}
		if t := convert.ToString(config[projectNameField]); t != "" {
			return b.Cleanup(t, sourceCodeType)
		}
	}

	return nil
}

func (b BaseProvider) AuthAddAccount(userID string, code string, config interface{}, projectID string, sourceCodeType string) (*v3.SourceCodeCredential, error) {
	if userID == "" {
		return nil, errors.New("unauth")
	}

	remote, err := remote.New(config)
	if err != nil {
		return nil, err
	}
	account, err := remote.Login(code)
	if err != nil {
		return nil, err
	}
	_, projectName := ref.Parse(projectID)
	account.Name = normalizeName(fmt.Sprintf("%s-%s-%s", projectName, sourceCodeType, account.Spec.LoginName))
	account.Namespace = userID
	account.Spec.UserName = userID
	account.Spec.ProjectName = projectID

	if _, err := b.NamespaceLister.Get("", userID); err != nil {
		if !apierror.IsNotFound(err) {
			return nil, err
		}
		_, err := b.Namespaces.Create(
			&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: userID,
				},
			})
		if err != nil {
			return nil, err
		}
	}
	_, err = b.SourceCodeCredentials.Create(account)
	if apierror.IsAlreadyExists(err) {
		exist, err := b.SourceCodeCredentialLister.Get(userID, account.Name)
		if err != nil {
			return nil, err
		}
		account.ResourceVersion = exist.ResourceVersion
		return b.SourceCodeCredentials.Update(account)
	} else if err != nil {
		return nil, err
	}
	return account, nil
}

func (b BaseProvider) RefreshReposByCredentialAndConfig(credential *v3.SourceCodeCredential, config interface{}) ([]v3.SourceCodeRepository, error) {
	namespace := credential.Namespace
	credentialID := ref.Ref(credential)

	remote, err := remote.New(config)
	if err != nil {
		return nil, err
	}
	repos, err := remote.Repos(credential)
	if err != nil {
		return nil, err
	}

	//remove old repos
	repositories, err := b.SourceCodeRepositoryIndexer.ByIndex(utils.SourceCodeRepositoryByCredentialIndex, credentialID)
	if err != nil {
		return nil, err
	}
	for _, r := range repositories {
		repo, _ := r.(*v3.SourceCodeRepository)
		if err := b.SourceCodeRepositories.DeleteNamespaced(namespace, repo.Name, &metav1.DeleteOptions{}); err != nil {
			return nil, err
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
		if _, err := b.SourceCodeRepositories.Create(&repo); err != nil {
			return nil, err
		}
	}

	return repos, nil
}

func normalizeName(name string) string {
	result := strings.ToLower(name)
	result = strings.Replace(result, dashChar, hyphenChar, -1)
	result = strings.TrimLeft(result, hyphenChar)
	result = strings.TrimRight(result, hyphenChar)
	return result
}
