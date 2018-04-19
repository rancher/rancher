package pipeline

import (
	"fmt"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/mapper"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/satori/uuid"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func refreshReposByCredential(sourceCodeRepositories v3.SourceCodeRepositoryInterface, sourceCodeRepositoryLister v3.SourceCodeRepositoryLister, credential *v3.SourceCodeCredential, clusterPipeline *v3.ClusterPipeline) ([]*v3.SourceCodeRepository, error) {

	remoteType := credential.Spec.SourceCodeType
	namespace := credential.Namespace
	credentialID := ref.Ref(credential)

	remote, err := remote.New(*clusterPipeline, remoteType)
	if err != nil {
		return nil, err
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
		repo.Spec.SourceCodeCredentialName = credentialID
		repo.Spec.ClusterName = credential.Spec.ClusterName
		repo.Spec.UserName = credential.Spec.UserName
		repo.Spec.SourceCodeType = credential.Spec.SourceCodeType
		repo.Name = uuid.NewV4().String()
		repo.Namespace = namespace
		if _, err := sourceCodeRepositories.Create(&repo); err != nil {
			return nil, err
		}
	}

	return repositories, nil
}

func toYaml(pipeline *v3.Pipeline) ([]byte, error) {
	m, err := convert.EncodeToMap(pipeline.Spec)
	if err != nil {
		return nil, err
	}
	//keep consistent with API naming
	mapper.Drop{Field: "projectName"}.FromInternal(m)
	mapper.DisplayName{}.FromInternal(m)
	stages, _ := values.GetSlice(m, "stages")
	for _, stage := range stages {
		steps, _ := values.GetSlice(stage, "steps")
		for _, step := range steps {
			mapper.Move{From: "sourceCodeConfig/sourceCodeCredentialName", To: "sourceCodeConfig/sourceCodeCredentialId"}.FromInternal(step)
		}
	}

	content, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}

	return content, nil
}

func fromYaml(content []byte) (map[string]interface{}, error) {
	m := map[interface{}]interface{}{}
	if err := yaml.Unmarshal(content, &m); err != nil {
		return nil, err
	}
	result := cleanupInterfaceMap(m)
	return result, nil
}

//cleanupInterfaceMap convert map[interface{}]interface{} to map[string]interface{}.
func cleanupInterfaceMap(in map[interface{}]interface{}) map[string]interface{} {
	res := make(map[string]interface{})
	for k, v := range in {
		res[fmt.Sprintf("%v", k)] = cleanupMapValue(v)
	}
	return res
}

func cleanupInterfaceArray(in []interface{}) []interface{} {
	res := make([]interface{}, len(in))
	for i, v := range in {
		res[i] = cleanupMapValue(v)
	}
	return res
}

func cleanupMapValue(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		return cleanupInterfaceArray(v)
	case map[interface{}]interface{}:
		return cleanupInterfaceMap(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
