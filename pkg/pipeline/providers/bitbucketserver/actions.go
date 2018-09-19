package bitbucket

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/project.cattle.io/v3"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/pipeline/remote"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/client/project/v3"
	"github.com/satori/go.uuid"
	"io/ioutil"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"
)

const (
	bitbucketDefaultHostName = "https://bitbucket.org"
	actionDisable            = "disable"
	actionTestAndApply       = "testAndApply"
	actionLogin              = "login"
	projectNameField         = "projectName"
)

func (b *BitbucketProvider) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	if convert.ToBool(resource.Values["enabled"]) {
		resource.AddAction(apiContext, actionDisable)
	}

	resource.AddAction(apiContext, actionTestAndApply)
}

func (b *BitbucketProvider) ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == actionTestAndApply {
		return b.testAndApply(actionName, action, request)
	} else if actionName == actionDisable {
		return b.disableAction(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (b *BitbucketProvider) providerFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, actionLogin)
}

func (b *BitbucketProvider) providerActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == actionLogin {
		return b.authuser(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func formBitbucketRedirectURLFromMap(config map[string]interface{}) string {
	clientID := convert.ToString(config[client.BitbucketCloudPipelineConfigFieldClientID])
	return fmt.Sprintf("%s/site/oauth2/authorize?client_id=%s&response_type=code", bitbucketDefaultHostName, clientID)
}

func (b *BitbucketProvider) testAndApply(actionName string, action *types.Action, apiContext *types.APIContext) error {
	applyInput := &v3.BitbucketCloudApplyInput{}

	if err := json.NewDecoder(apiContext.Request.Body).Decode(applyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	ns, _ := ref.Parse(apiContext.ID)
	pConfig, err := b.GetProviderConfig(ns)
	if err != nil {
		return err
	}
	storedBitbucketPipelineConfig, ok := pConfig.(*v3.BitbucketCloudPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get github provider config")
	}
	toUpdate := storedBitbucketPipelineConfig.DeepCopy()
	toUpdate.ClientID = applyInput.BitbucketConfig.ClientID
	toUpdate.ClientSecret = applyInput.BitbucketConfig.ClientSecret
	toUpdate.RedirectURL = applyInput.BitbucketConfig.RedirectURL

	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	sourceCodeCredential, err := b.authAddAccount(userName, applyInput.Code, toUpdate)
	if err != nil {
		return err
	}
	if _, err = b.refreshReposByCredentialAndConfig(sourceCodeCredential, toUpdate); err != nil {
		return err
	}

	toUpdate.Enabled = true
	//update bitbucket pipeline config
	if _, err = b.SourceCodeProviderConfigs.ObjectClient().Update(toUpdate.Name, toUpdate); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, nil)
	return nil
}

func (b *BitbucketProvider) authuser(apiContext *types.APIContext) error {
	authUserInput := v3.AuthUserInput{}
	requestBytes, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(requestBytes, &authUserInput); err != nil {
		return err
	}

	ns, _ := ref.Parse(apiContext.ID)
	pConfig, err := b.GetProviderConfig(ns)
	if err != nil {
		return err
	}
	config, ok := pConfig.(*v3.BitbucketCloudPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get bitbucket provider config")
	}
	if !config.Enabled {
		return errors.New("bitbucket oauth app is not configured")
	}

	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	account, err := b.authAddAccount(userName, authUserInput.Code, config)
	if err != nil {
		return err
	}
	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, client.SourceCodeCredentialType, account.Name, &data); err != nil {
		return err
	}

	if _, err := b.refreshReposByCredentialAndConfig(account, config); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (b *BitbucketProvider) authAddAccount(userID string, code string, config *v3.BitbucketCloudPipelineConfig) (*v3.SourceCodeCredential, error) {
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
	account.Name = strings.ToLower(fmt.Sprintf("%s-%s-%s", config.Namespace, model.BitbucketCloudType, account.Spec.LoginName))
	account.Namespace = userID
	account.Spec.UserName = userID
	account.Spec.ProjectName = config.ProjectName
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

func (b *BitbucketProvider) disableAction(request *types.APIContext) error {
	ns, _ := ref.Parse(request.ID)
	o, err := b.SourceCodeProviderConfigs.ObjectClient().UnstructuredClient().GetNamespaced(ns, model.BitbucketCloudType, metav1.GetOptions{})
	if err != nil {
		return err
	}
	u, _ := o.(runtime.Unstructured)
	config := u.UnstructuredContent()
	if convert.ToBool(config[client.SourceCodeProviderConfigFieldEnabled]) {
		config[client.SourceCodeProviderConfigFieldEnabled] = false
		if _, err := b.SourceCodeProviderConfigs.ObjectClient().Update(model.BitbucketCloudType, o); err != nil {
			return err
		}
		if t := convert.ToString(config[projectNameField]); t != "" {
			return b.cleanup(t)
		}
	}

	return nil
}

func (b *BitbucketProvider) cleanup(projectID string) error {
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

	crdKey := utils.ProjectNameAndSourceCodeTypeKey(projectID, model.BitbucketCloudType)
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

	repoKey := utils.ProjectNameAndSourceCodeTypeKey(projectID, model.BitbucketCloudType)
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

func (b *BitbucketProvider) refreshReposByCredentialAndConfig(credential *v3.SourceCodeCredential, config *v3.BitbucketCloudPipelineConfig) ([]v3.SourceCodeRepository, error) {
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
