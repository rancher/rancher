package pipeline

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/pipeline/providers"
	"github.com/rancher/rancher/pkg/pipeline/remote"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/types/apis/project.cattle.io/v3"
	client "github.com/rancher/types/client/project/v3"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	actionRun        = "run"
	actionPushConfig = "pushconfig"
	linkConfigs      = "configs"
	linkYaml         = "yaml"
	linkBranches     = "branches"
	queryBranch      = "branch"
	queryConfigs     = "configs"
)

type Handler struct {
	PipelineLister             v3.PipelineLister
	PipelineExecutions         v3.PipelineExecutionInterface
	SourceCodeCredentialLister v3.SourceCodeCredentialLister
	SourceCodeCredentials      v3.SourceCodeCredentialInterface
}

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	if canCreatePipelineExecutionFromPipeline(apiContext, resource) {
		resource.AddAction(apiContext, actionRun)
	}
	if canUpdatePipeline(apiContext, resource) {
		resource.AddAction(apiContext, actionPushConfig)
	}
	resource.Links[linkConfigs] = apiContext.URLBuilder.Link(linkConfigs, resource)
	resource.Links[linkYaml] = apiContext.URLBuilder.Link(linkYaml, resource)
	resource.Links[linkBranches] = apiContext.URLBuilder.Link(linkBranches, resource)
}

func (h *Handler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	if apiContext.Link == linkYaml {
		if apiContext.Method == http.MethodPut {
			return h.updatePipelineConfigYaml(apiContext)
		}
		return h.getPipelineConfigYAML(apiContext)
	} else if apiContext.Link == linkConfigs {
		return h.getPipelineConfigJSON(apiContext)
	} else if apiContext.Link == linkBranches {
		return h.getValidBranches(apiContext)
	}

	return httperror.NewAPIError(httperror.NotFound, "Link not found")
}

func (h *Handler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case actionRun:
		if !canCreatePipelineExecutionFromPipeline(apiContext, nil) {
			return httperror.NewAPIError(httperror.NotFound, "not found")
		}
		return h.run(apiContext)
	case actionPushConfig:
		if !canUpdatePipeline(apiContext, nil) {
			return httperror.NewAPIError(httperror.NotFound, "not found")
		}
		return h.pushConfig(apiContext)
	}
	return httperror.NewAPIError(httperror.InvalidAction, "unsupported action")
}

func (h *Handler) run(apiContext *types.APIContext) error {
	ns, name := ref.Parse(apiContext.ID)
	pipeline, err := h.PipelineLister.Get(ns, name)
	if err != nil {
		return err
	}
	runPipelineInput := v3.RunPipelineInput{}
	requestBytes, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return err
	}
	if string(requestBytes) != "" {
		if err := json.Unmarshal(requestBytes, &runPipelineInput); err != nil {
			return err
		}
	}

	branch := runPipelineInput.Branch
	if branch == "" {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "Error branch is not specified for the pipeline to run")
	}

	userName := apiContext.Request.Header.Get("Impersonate-User")
	pipelineConfig, err := providers.GetPipelineConfigByBranch(h.SourceCodeCredentials, h.SourceCodeCredentialLister, pipeline, branch)
	if err != nil {
		return err
	}

	if pipelineConfig == nil {
		return fmt.Errorf("find no pipeline config to run in the branch")
	}

	info, err := h.getBuildInfoByBranch(pipeline, branch)
	if err != nil {
		return err
	}
	info.TriggerType = utils.TriggerTypeUser
	info.TriggerUserName = userName
	execution, err := utils.GenerateExecution(h.PipelineExecutions, pipeline, pipelineConfig, info)
	if err != nil {
		return err
	}

	if execution == nil {
		return errors.New("condition is not match, no build is triggered")
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, client.PipelineExecutionType, ref.Ref(execution), &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return err
}

func (h *Handler) pushConfig(apiContext *types.APIContext) error {
	ns, name := ref.Parse(apiContext.ID)
	pipeline, err := h.PipelineLister.Get(ns, name)
	if err != nil {
		return err
	}

	pushConfigInput := v3.PushPipelineConfigInput{}
	requestBytes, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return err
	}
	if string(requestBytes) != "" {
		if err := json.Unmarshal(requestBytes, &pushConfigInput); err != nil {
			return err
		}
	}

	//use current user's auth to do the push
	userName := apiContext.Request.Header.Get("Impersonate-User")
	creds, err := h.SourceCodeCredentialLister.List(userName, labels.Everything())
	if err != nil {
		return err
	}
	accessToken := ""
	sourceCodeType := model.GithubType
	var credential *v3.SourceCodeCredential
	for _, cred := range creds {
		if cred.Spec.ProjectName == pipeline.Spec.ProjectName && !cred.Status.Logout {
			accessToken = cred.Spec.AccessToken
			sourceCodeType = cred.Spec.SourceCodeType
			credential = cred
			break
		}
	}

	_, projID := ref.Parse(pipeline.Spec.ProjectName)
	scpConfig, err := providers.GetSourceCodeProviderConfig(sourceCodeType, projID)
	if err != nil {
		return err
	}
	remote, err := remote.New(scpConfig)
	if err != nil {
		return err
	}
	accessToken, err = utils.EnsureAccessToken(h.SourceCodeCredentials, remote, credential)
	if err != nil {
		return err
	}

	for branch, config := range pushConfigInput.Configs {
		content, err := utils.PipelineConfigToYaml(&config)
		if err != nil {
			return err
		}
		if err := remote.SetPipelineFileInRepo(pipeline.Spec.RepositoryURL, branch, accessToken, content); err != nil {
			if apierr, ok := err.(*httperror.APIError); ok && apierr.Code.Status == http.StatusNotFound {
				//github returns 404 for unauth request to prevent leakage of private repos
				return httperror.NewAPIError(httperror.Unauthorized, "current git account is unauthorized for the action")
			}
			return err
		}
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (h *Handler) getBuildInfoByBranch(pipeline *v3.Pipeline, branch string) (*model.BuildInfo, error) {
	credentialName := pipeline.Spec.SourceCodeCredentialName
	repoURL := pipeline.Spec.RepositoryURL
	accessToken := ""
	sourceCodeType := model.GithubType
	var scpConfig interface{}
	var credential *v3.SourceCodeCredential
	var err error
	if credentialName != "" {
		ns, name := ref.Parse(credentialName)
		credential, err = h.SourceCodeCredentialLister.Get(ns, name)
		if err != nil {
			return nil, err
		}
		sourceCodeType = credential.Spec.SourceCodeType
		accessToken = credential.Spec.AccessToken
		_, projID := ref.Parse(pipeline.Spec.ProjectName)
		scpConfig, err = providers.GetSourceCodeProviderConfig(sourceCodeType, projID)
		if err != nil {
			return nil, err
		}
	}
	remote, err := remote.New(scpConfig)
	if err != nil {
		return nil, err
	}
	accessToken, err = utils.EnsureAccessToken(h.SourceCodeCredentials, remote, credential)
	if err != nil {
		return nil, err
	}
	info, err := remote.GetHeadInfo(repoURL, branch, accessToken)
	if err != nil {
		return nil, err
	}
	return info, nil

}

func (h *Handler) getValidBranches(apiContext *types.APIContext) error {
	ns, name := ref.Parse(apiContext.ID)
	pipeline, err := h.PipelineLister.Get(ns, name)
	if err != nil {
		return err
	}

	accessKey := ""
	sourceCodeType := model.GithubType
	var scpConfig interface{}
	var cred *v3.SourceCodeCredential
	if pipeline.Spec.SourceCodeCredentialName != "" {
		ns, name = ref.Parse(pipeline.Spec.SourceCodeCredentialName)
		cred, err = h.SourceCodeCredentialLister.Get(ns, name)
		if err != nil {
			return err
		}
		accessKey = cred.Spec.AccessToken
		sourceCodeType = cred.Spec.SourceCodeType
		_, projID := ref.Parse(pipeline.Spec.ProjectName)
		scpConfig, err = providers.GetSourceCodeProviderConfig(sourceCodeType, projID)
		if err != nil {
			return err
		}
	}
	remote, err := remote.New(scpConfig)
	if err != nil {
		return err
	}
	accessKey, err = utils.EnsureAccessToken(h.SourceCodeCredentials, remote, cred)
	if err != nil {
		return err
	}

	validBranches := map[string]bool{}

	branches, err := remote.GetBranches(pipeline.Spec.RepositoryURL, accessKey)
	if err != nil {
		return err
	}
	for _, b := range branches {
		content, err := remote.GetPipelineFileInRepo(pipeline.Spec.RepositoryURL, b, accessKey)
		if err != nil {
			return err
		}
		logrus.Debugf("get content in branch %s:%v", b, string(content))
		if content != nil {
			validBranches[b] = true
		}
	}

	result := []string{}
	for b := range validBranches {
		result = append(result, b)
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		return err
	}
	apiContext.Response.Write(bytes)
	return nil
}

func (h *Handler) getPipelineConfigJSON(apiContext *types.APIContext) error {
	ns, name := ref.Parse(apiContext.ID)
	pipeline, err := h.PipelineLister.Get(ns, name)
	if err != nil {
		return err
	}
	branch := apiContext.Request.URL.Query().Get(queryBranch)

	m, err := h.getPipelineConfigs(pipeline, branch)
	if err != nil {
		return err
	}
	bytes, err := json.Marshal(m)
	if err != nil {
		return err
	}
	apiContext.Response.Write(bytes)
	return nil
}

func (h *Handler) getPipelineConfigYAML(apiContext *types.APIContext) error {
	yamlMap := map[string]interface{}{}
	m := map[string]*v3.PipelineConfig{}

	branch := apiContext.Request.URL.Query().Get(queryBranch)
	configs := apiContext.Request.URL.Query().Get(queryConfigs)
	if configs != "" {
		err := json.Unmarshal([]byte(configs), &m)
		if err != nil {
			return err
		}
		for b, config := range m {
			if config == nil {
				yamlMap[b] = nil
				continue
			}
			content, err := utils.PipelineConfigToYaml(config)
			if err != nil {
				return err
			}
			yamlMap[b] = string(content)
		}
	} else {
		ns, name := ref.Parse(apiContext.ID)
		pipeline, err := h.PipelineLister.Get(ns, name)
		if err != nil {
			return err
		}
		m, err = h.getPipelineConfigs(pipeline, branch)
		if err != nil {
			return err
		}
	}

	if branch != "" {
		config := m[branch]
		if config == nil {
			return nil
		}
		content, err := utils.PipelineConfigToYaml(config)
		if err != nil {
			return err
		}
		apiContext.Response.Write(content)
		return nil
	}

	for b, config := range m {
		if config == nil {
			yamlMap[b] = nil
			continue
		}
		content, err := utils.PipelineConfigToYaml(config)
		if err != nil {
			return err
		}
		yamlMap[b] = string(content)
	}

	bytes, err := json.Marshal(yamlMap)
	if err != nil {
		return err
	}
	apiContext.Response.Write(bytes)
	return nil
}
func (h *Handler) updatePipelineConfigYaml(apiContext *types.APIContext) error {
	branch := apiContext.Request.URL.Query().Get(queryBranch)
	if branch == "" {
		return httperror.NewAPIError(httperror.InvalidOption, "Branch is not specified")
	}

	ns, name := ref.Parse(apiContext.ID)
	pipeline, err := h.PipelineLister.Get(ns, name)
	if err != nil {
		return err
	}

	content, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return err
	}
	//check yaml
	config := &v3.PipelineConfig{}
	if err := yaml.Unmarshal(content, config); err != nil {
		return err
	}

	//use current user's auth to do the push
	userName := apiContext.Request.Header.Get("Impersonate-User")
	creds, err := h.SourceCodeCredentialLister.List(userName, labels.Everything())
	if err != nil {
		return err
	}
	accessToken := ""
	sourceCodeType := model.GithubType
	var credential *v3.SourceCodeCredential
	for _, cred := range creds {
		if cred.Spec.ProjectName == pipeline.Spec.ProjectName && !cred.Status.Logout {
			accessToken = cred.Spec.AccessToken
			sourceCodeType = cred.Spec.SourceCodeType
			credential = cred
		}
	}

	_, projID := ref.Parse(pipeline.Spec.ProjectName)
	scpConfig, err := providers.GetSourceCodeProviderConfig(sourceCodeType, projID)
	if err != nil {
		return err
	}
	remote, err := remote.New(scpConfig)
	if err != nil {
		return err
	}
	accessToken, err = utils.EnsureAccessToken(h.SourceCodeCredentials, remote, credential)
	if err != nil {
		return err
	}

	if err := remote.SetPipelineFileInRepo(pipeline.Spec.RepositoryURL, branch, accessToken, content); err != nil {
		if apierr, ok := err.(*httperror.APIError); ok && apierr.Code.Status == http.StatusNotFound {
			//github returns 404 for unauth request to prevent leakage of private repos
			return httperror.NewAPIError(httperror.Unauthorized, "current git account is unauthorized for the action")
		}
		return err
	}

	return nil
}

func (h *Handler) getPipelineConfigs(pipeline *v3.Pipeline, branch string) (map[string]*v3.PipelineConfig, error) {
	accessToken := ""
	sourceCodeType := model.GithubType
	var scpConfig interface{}
	var cred *v3.SourceCodeCredential
	var err error
	if pipeline.Spec.SourceCodeCredentialName != "" {
		ns, name := ref.Parse(pipeline.Spec.SourceCodeCredentialName)
		cred, err = h.SourceCodeCredentialLister.Get(ns, name)
		if err != nil {
			return nil, err
		}
		sourceCodeType = cred.Spec.SourceCodeType
		accessToken = cred.Spec.AccessToken
		_, projID := ref.Parse(pipeline.Spec.ProjectName)
		scpConfig, err = providers.GetSourceCodeProviderConfig(sourceCodeType, projID)
		if err != nil {
			return nil, err
		}
	}

	remote, err := remote.New(scpConfig)
	if err != nil {
		return nil, err
	}
	accessToken, err = utils.EnsureAccessToken(h.SourceCodeCredentials, remote, cred)
	if err != nil {
		return nil, err
	}

	m := map[string]*v3.PipelineConfig{}

	if branch != "" {
		content, err := remote.GetPipelineFileInRepo(pipeline.Spec.RepositoryURL, branch, accessToken)
		if err != nil {
			return nil, err
		}
		if content != nil {
			spec, err := utils.PipelineConfigFromYaml(content)
			if err != nil {
				return nil, errors.Wrapf(err, "Error fetching pipeline config in Branch '%s'", branch)
			}
			m[branch] = spec
		} else {
			m[branch] = nil
		}

	} else {
		branches, err := remote.GetBranches(pipeline.Spec.RepositoryURL, accessToken)
		if err != nil {
			return nil, err
		}
		for _, b := range branches {
			content, err := remote.GetPipelineFileInRepo(pipeline.Spec.RepositoryURL, b, accessToken)
			if err != nil {
				return nil, err
			}
			if content != nil {
				spec, err := utils.PipelineConfigFromYaml(content)
				if err != nil {
					return nil, errors.Wrapf(err, "Error fetching pipeline config in Branch '%s'", b)
				}
				m[b] = spec
			} else {
				m[b] = nil
			}
		}
	}

	return m, nil
}

func canUpdatePipeline(apiContext *types.APIContext, resource *types.RawResource) bool {
	obj := rbac.ObjFromContext(apiContext, resource)
	return apiContext.AccessControl.CanDo(
		v3.PipelineGroupVersionKind.Group, v3.PipelineResource.Name, "update", apiContext, obj, apiContext.Schema,
	) == nil
}

// This checks ability to execute pipeline from pipeline obj, not pipelineExecution obj
// Uses the project ID from the pipeline obj to check create permissions on executions in that NS
func canCreatePipelineExecutionFromPipeline(apiContext *types.APIContext, resource *types.RawResource) bool {
	obj := make(map[string]interface{})
	if resource != nil {
		obj[rbac.NamespaceID], _ = ref.Parse(resource.ID)
	} else {
		obj[rbac.NamespaceID], _ = ref.Parse(apiContext.ID)
	}
	if val, ok := obj[rbac.NamespaceID]; !ok || val == "" {
		return false
	}
	return apiContext.AccessControl.CanDo(
		v3.PipelineExecutionGroupVersionKind.Group, v3.PipelineExecutionResource.Name, "create", apiContext, obj, apiContext.Schema,
	) == nil
}
