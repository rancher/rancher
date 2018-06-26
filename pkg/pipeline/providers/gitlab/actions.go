package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/pipeline/remote"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/client/project/v3"
	"github.com/satori/go.uuid"
	"io/ioutil"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"net/url"
	"strings"
)

const (
	gitlabDefaultHostName = "https://gitlab.com"
	actionDisable         = "disable"
	actionTestAndApply    = "testAndApply"
	actionLogin           = "login"
	projectNameField      = "projectName"
	hyphenChar            = "-"
	dashChar              = "_"
)

func (g *GlProvider) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	if convert.ToBool(resource.Values["enabled"]) {
		resource.AddAction(apiContext, actionDisable)
	}

	resource.AddAction(apiContext, actionTestAndApply)
}

func (g *GlProvider) ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == actionTestAndApply {
		return g.testAndApply(actionName, action, request)
	} else if actionName == actionDisable {
		return g.disableAction(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (g *GlProvider) providerFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, actionLogin)
}

func (g *GlProvider) providerActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == actionLogin {
		return g.authuser(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (g *GlProvider) testAndApply(actionName string, action *types.Action, apiContext *types.APIContext) error {
	applyInput := &v3.GitlabPipelineConfigApplyInput{}

	if err := json.NewDecoder(apiContext.Request.Body).Decode(applyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	ns, _ := ref.Parse(apiContext.ID)
	pConfig, err := g.GetProviderConfig(ns)
	if err != nil {
		return err
	}
	storedGitlabPipelineConfig, ok := pConfig.(*v3.GitlabPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get gitlab provider config")
	}
	toUpdate := storedGitlabPipelineConfig.DeepCopy()

	toUpdate.ClientID = applyInput.GitlabConfig.ClientID
	toUpdate.ClientSecret = applyInput.GitlabConfig.ClientSecret
	toUpdate.Hostname = applyInput.GitlabConfig.Hostname
	toUpdate.TLS = applyInput.GitlabConfig.TLS
	currentURL := apiContext.URLBuilder.Current()
	u, err := url.Parse(currentURL)
	if err != nil {
		return err
	}
	toUpdate.RedirectURL = fmt.Sprintf("%s://%s/verify-auth", u.Scheme, u.Host)
	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	sourceCodeCredential, err := g.authAddAccount(userName, applyInput.Code, toUpdate)
	if err != nil {
		return err
	}
	if _, err = g.refreshReposByCredentialAndConfig(sourceCodeCredential, toUpdate); err != nil {
		return err
	}
	toUpdate.Enabled = true
	//update gitlab pipeline config
	if _, err = g.SourceCodeProviderConfigs.ObjectClient().Update(toUpdate.Name, toUpdate); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, nil)
	return nil
}

func (g *GlProvider) authuser(apiContext *types.APIContext) error {
	authUserInput := v3.AuthUserInput{}
	requestBytes, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(requestBytes, &authUserInput); err != nil {
		return err
	}

	ns, _ := ref.Parse(apiContext.ID)
	pConfig, err := g.GetProviderConfig(ns)
	if err != nil {
		return err
	}
	config, ok := pConfig.(*v3.GitlabPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get gitlab provider config")
	}
	if !config.Enabled {
		return errors.New("gitlab oauth app is not configured")
	}

	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	account, err := g.authAddAccount(userName, authUserInput.Code, config)
	if err != nil {
		return err
	}
	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, client.SourceCodeCredentialType, account.Name, &data); err != nil {
		return err
	}

	if _, err := g.refreshReposByCredentialAndConfig(account, config); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (g *GlProvider) refreshReposByCredentialAndConfig(credential *v3.SourceCodeCredential, config *v3.GitlabPipelineConfig) ([]v3.SourceCodeRepository, error) {
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
	repositories, err := g.SourceCodeRepositoryIndexer.ByIndex(utils.SourceCodeRepositoryByCredentialIndex, credentialID)
	if err != nil {
		return nil, err
	}
	for _, r := range repositories {
		repo, _ := r.(*v3.SourceCodeRepository)
		if err := g.SourceCodeRepositories.DeleteNamespaced(namespace, repo.Name, &metav1.DeleteOptions{}); err != nil {
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
		if _, err := g.SourceCodeRepositories.Create(&repo); err != nil {
			return nil, err
		}
	}

	return repos, nil
}

func (g *GlProvider) authAddAccount(userID string, code string, config *v3.GitlabPipelineConfig) (*v3.SourceCodeCredential, error) {
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

	account.Name = normalizeName(fmt.Sprintf("%s-%s-%s", config.Namespace, model.GitlabType, account.Spec.LoginName))
	account.Namespace = userID
	account.Spec.UserName = userID
	account.Spec.ProjectName = config.ProjectName
	_, err = g.SourceCodeCredentials.Create(account)
	if apierror.IsAlreadyExists(err) {
		exist, err := g.SourceCodeCredentialLister.Get(userID, account.Name)
		if err != nil {
			return nil, err
		}
		account.ResourceVersion = exist.ResourceVersion
		return g.SourceCodeCredentials.Update(account)
	} else if err != nil {
		return nil, err
	}
	return account, nil
}

func (g *GlProvider) disableAction(request *types.APIContext) error {
	ns, _ := ref.Parse(request.ID)
	o, err := g.SourceCodeProviderConfigs.ObjectClient().UnstructuredClient().GetNamespaced(ns, model.GitlabType, metav1.GetOptions{})
	if err != nil {
		return err
	}
	u, _ := o.(runtime.Unstructured)
	config := u.UnstructuredContent()
	if convert.ToBool(config[client.SourceCodeProviderConfigFieldEnabled]) {
		config[client.SourceCodeProviderConfigFieldEnabled] = false
		if _, err := g.SourceCodeProviderConfigs.ObjectClient().Update(model.GitlabType, o); err != nil {
			return err
		}
		if t := convert.ToString(config[projectNameField]); t != "" {
			return g.cleanup(t)
		}
	}

	return nil
}

func (g *GlProvider) cleanup(projectID string) error {
	pipelines, err := g.PipelineIndexer.ByIndex(utils.PipelineByProjectIndex, projectID)
	if err != nil {
		return err
	}
	for _, p := range pipelines {
		pipeline, _ := p.(*v3.Pipeline)
		if err := g.Pipelines.DeleteNamespaced(pipeline.Namespace, pipeline.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	pipelineExecutions, err := g.PipelineExecutionIndexer.ByIndex(utils.PipelineExecutionByProjectIndex, projectID)
	if err != nil {
		return err
	}
	for _, e := range pipelineExecutions {
		execution, _ := e.(*v3.PipelineExecution)
		if err := g.PipelineExecutions.DeleteNamespaced(execution.Namespace, execution.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	crdKey := utils.ProjectNameAndSourceCodeTypeKey(projectID, model.GitlabType)
	credentials, err := g.SourceCodeCredentialIndexer.ByIndex(utils.SourceCodeCredentialByProjectAndTypeIndex, crdKey)
	if err != nil {
		return err
	}
	for _, c := range credentials {
		credential, _ := c.(*v3.SourceCodeCredential)
		if err := g.SourceCodeCredentials.DeleteNamespaced(credential.Namespace, credential.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	repoKey := utils.ProjectNameAndSourceCodeTypeKey(projectID, model.GitlabType)
	repositories, err := g.SourceCodeRepositoryIndexer.ByIndex(utils.SourceCodeRepositoryByProjectAndTypeIndex, repoKey)
	if err != nil {
		return err
	}
	for _, r := range repositories {
		repo, _ := r.(*v3.SourceCodeRepository)
		if err := g.SourceCodeRepositories.DeleteNamespaced(repo.Namespace, repo.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func formGitlabRedirectURLFromMap(config map[string]interface{}) string {
	hostname := convert.ToString(config[client.GitlabPipelineConfigFieldHostname])
	clientID := convert.ToString(config[client.GitlabPipelineConfigFieldClientID])
	tls := convert.ToBool(config[client.GitlabPipelineConfigFieldTLS])
	return gitlabRedirectURL(hostname, clientID, tls)
}

func gitlabRedirectURL(hostname, clientID string, tls bool) string {
	redirect := ""
	if hostname != "" {
		scheme := "http://"
		if tls {
			scheme = "https://"
		}
		redirect = scheme + hostname
	} else {
		redirect = gitlabDefaultHostName
	}
	redirect = fmt.Sprintf("%s/oauth/authorize?client_id=%s&response_type=code", redirect, clientID)
	return redirect
}

func normalizeName(name string) string {
	result := strings.ToLower(name)
	result = strings.Replace(result, dashChar, hyphenChar, -1)
	result = strings.TrimLeft(result, hyphenChar)
	result = strings.TrimRight(result, hyphenChar)
	return result
}
