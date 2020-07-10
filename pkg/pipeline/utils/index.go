package utils

import (
	"fmt"

	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/rancher/pkg/types/apis/project.cattle.io/v3"
)

const (
	PipelineByProjectIndex                    = "pipeline.project.cattle.io/pipeline-by-project"
	PipelineExecutionByClusterIndex           = "pipeline.project.cattle.io/execution-by-cluster"
	PipelineExecutionByProjectIndex           = "pipeline.project.cattle.io/execution-by-project"
	SourceCodeCredentialByProjectAndTypeIndex = "pipeline.project.cattle.io/credential-by-project-and-type"
	SourceCodeRepositoryByCredentialIndex     = "pipeline.project.cattle.io/repository-by-credential"
	SourceCodeRepositoryByProjectAndTypeIndex = "pipeline.project.cattle.io/repository-by-project-and-type"
)

func PipelineByProjectName(obj interface{}) ([]string, error) {
	pipeline, ok := obj.(*v3.Pipeline)
	if !ok {
		return []string{}, nil
	}

	return []string{pipeline.Spec.ProjectName}, nil
}

func PipelineExecutionByProjectName(obj interface{}) ([]string, error) {
	execution, ok := obj.(*v3.PipelineExecution)
	if !ok {
		return []string{}, nil
	}

	return []string{execution.Spec.ProjectName}, nil
}

func PipelineExecutionByClusterName(obj interface{}) ([]string, error) {
	execution, ok := obj.(*v3.PipelineExecution)
	if !ok {
		return []string{}, nil
	}
	cluster, _ := ref.Parse(execution.Spec.ProjectName)
	return []string{cluster}, nil
}

func SourceCodeCredentialByProjectNameAndType(obj interface{}) ([]string, error) {
	credential, ok := obj.(*v3.SourceCodeCredential)
	if !ok {
		return []string{}, nil
	}
	key := ProjectNameAndSourceCodeTypeKey(credential.Spec.ProjectName, credential.Spec.SourceCodeType)

	return []string{key}, nil
}

func SourceCodeRepositoryByProjectNameAndType(obj interface{}) ([]string, error) {
	repository, ok := obj.(*v3.SourceCodeRepository)
	if !ok {
		return []string{}, nil
	}
	key := ProjectNameAndSourceCodeTypeKey(repository.Spec.ProjectName, repository.Spec.SourceCodeType)

	return []string{key}, nil
}

func SourceCodeRepositoryByCredentialName(obj interface{}) ([]string, error) {
	repository, ok := obj.(*v3.SourceCodeRepository)
	if !ok {
		return []string{}, nil
	}
	return []string{repository.Spec.SourceCodeCredentialName}, nil
}

func ProjectNameAndSourceCodeTypeKey(projectName, souceCodeType string) string {
	return fmt.Sprintf("%s.%s", projectName, souceCodeType)
}
