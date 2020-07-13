package bitbucketserver

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/mrjones/oauth"
	"github.com/pkg/errors"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/ref"
	v3 "github.com/rancher/rancher/pkg/types/apis/project.cattle.io/v3"
	client "github.com/rancher/rancher/pkg/types/client/project/v3"
	"github.com/rancher/rke/pki/cert"
	"github.com/rancher/wrangler/pkg/randomtoken"
)

const (
	actionDisable      = "disable"
	actionTestAndApply = "testAndApply"
	actionGenerateKeys = "generateKeys"
	actionRequestLogin = "requestLogin"
	actionLogin        = "login"
)

func (b *BsProvider) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	if convert.ToBool(resource.Values["enabled"]) {
		resource.AddAction(apiContext, actionDisable)
	}

	resource.AddAction(apiContext, actionGenerateKeys)
	resource.AddAction(apiContext, actionRequestLogin)
	resource.AddAction(apiContext, actionTestAndApply)
}

func (b *BsProvider) ActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == actionTestAndApply {
		return b.testAndApply(request)
	} else if actionName == actionDisable {
		return b.DisableAction(request, b.GetName())
	} else if actionName == actionGenerateKeys {
		return b.generateKeys(request)
	} else if actionName == actionRequestLogin {
		return b.requestLogin(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (b *BsProvider) providerFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, actionRequestLogin)
	resource.AddAction(apiContext, actionLogin)
}

func (b *BsProvider) providerActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == actionRequestLogin {
		return b.requestLogin(request)
	} else if actionName == actionLogin {
		return b.login(request)
	}

	return httperror.NewAPIError(httperror.ActionNotAvailable, "")
}

func (b *BsProvider) generateKeys(apiContext *types.APIContext) error {
	ns, _ := ref.Parse(apiContext.ID)
	pConfig, err := b.GetProviderConfig(ns)
	if err != nil {
		return err
	}
	storedBitbucketPipelineConfig, ok := pConfig.(*v3.BitbucketServerPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get bitbucket server provider config")
	}
	toUpdate := storedBitbucketPipelineConfig.DeepCopy()
	token, err := randomtoken.Generate()
	if err != nil {
		return err
	}
	toUpdate.ConsumerKey = token
	pub, private, err := generateKeyPair()
	if err != nil {
		return err
	}
	toUpdate.PrivateKey = private
	toUpdate.PublicKey = pub
	if _, err = b.SourceCodeProviderConfigs.ObjectClient().Update(toUpdate.Name, toUpdate); err != nil {
		return err
	}

	return nil
}

func generateKeyPair() (string, string, error) {
	key, err := cert.NewPrivateKey()
	if err != nil {
		return "", "", err
	}
	privateKey := string(cert.EncodePrivateKeyPEM(key))
	publicKeyByte, err := cert.EncodePublicKeyPEM(&key.PublicKey)
	if err != nil {
		return "", "", err
	}
	publicKey := string(publicKeyByte)
	return publicKey, privateKey, nil
}

func (b *BsProvider) requestLogin(apiContext *types.APIContext) error {
	input := &v3.BitbucketServerRequestLoginInput{}
	if err := json.NewDecoder(apiContext.Request.Body).Decode(input); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	ns, _ := ref.Parse(apiContext.ID)
	pConfig, err := b.GetProviderConfig(ns)
	if err != nil {
		return err
	}
	storedBitbucketPipelineConfig, ok := pConfig.(*v3.BitbucketServerPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get bitbucket server provider config")
	}
	consumerKey := storedBitbucketPipelineConfig.ConsumerKey
	rsaKey := storedBitbucketPipelineConfig.PrivateKey
	host := input.Hostname
	tls := input.TLS
	if host == "" {
		host = storedBitbucketPipelineConfig.Hostname
		tls = storedBitbucketPipelineConfig.TLS
	}
	if tls {
		host = "https://" + host
	} else {
		host = "http://" + host
	}
	consumer, err := getOauthConsumer(consumerKey, rsaKey, host)
	if err != nil {
		return err
	}
	_, loginURL, err := consumer.GetRequestTokenAndUrl(input.RedirectURL)
	if err != nil {
		return err
	}
	data := map[string]interface{}{
		"loginUrl": loginURL,
		"type":     "bitbucketServerRequestLoginOutput",
	}
	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (b *BsProvider) testAndApply(apiContext *types.APIContext) error {
	applyInput := &v3.BitbucketServerApplyInput{}

	if err := json.NewDecoder(apiContext.Request.Body).Decode(applyInput); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}

	projectID, _ := ref.Parse(apiContext.ID)
	pConfig, err := b.GetProviderConfig(projectID)
	if err != nil {
		return err
	}
	storedBitbucketPipelineConfig, ok := pConfig.(*v3.BitbucketServerPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get bitbucket server provider config")
	}
	toUpdate := storedBitbucketPipelineConfig.DeepCopy()
	toUpdate.Hostname = applyInput.Hostname
	toUpdate.TLS = applyInput.TLS
	toUpdate.RedirectURL = applyInput.RedirectURL

	code := fmt.Sprintf("%s:%s:%s", projectID, applyInput.OAuthToken, applyInput.OAuthVerifier)
	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	sourceCodeCredential, err := b.AuthAddAccount(userName, code, toUpdate, toUpdate.ProjectName, model.BitbucketServerType)
	if err != nil {
		return err
	}
	if _, err = b.RefreshReposByCredentialAndConfig(sourceCodeCredential, toUpdate); err != nil {
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

func (b *BsProvider) login(apiContext *types.APIContext) error {
	loginInput := v3.AuthUserInput{}
	requestBytes, err := ioutil.ReadAll(apiContext.Request.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(requestBytes, &loginInput); err != nil {
		return err
	}

	projectID, _ := ref.Parse(apiContext.ID)
	pConfig, err := b.GetProviderConfig(projectID)
	if err != nil {
		return err
	}
	config, ok := pConfig.(*v3.BitbucketServerPipelineConfig)
	if !ok {
		return fmt.Errorf("Failed to get bitbucket provider config")
	}
	if !config.Enabled {
		return errors.New("bitbucket oauth app is not configured")
	}

	code := fmt.Sprintf("%s:%s", projectID, loginInput.Code)
	//oauth and add user
	userName := apiContext.Request.Header.Get("Impersonate-User")
	account, err := b.AuthAddAccount(userName, code, config, config.ProjectName, model.BitbucketServerType)
	if err != nil {
		return err
	}
	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, client.SourceCodeCredentialType, account.Name, &data); err != nil {
		return err
	}

	if _, err := b.RefreshReposByCredentialAndConfig(account, config); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func getOauthConsumer(consumerKey, rsaKey, hostURL string) (*oauth.Consumer, error) {
	keyBytes := []byte(rsaKey)
	block, _ := pem.Decode(keyBytes)
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	bitbucketOauthConsumer := oauth.NewRSAConsumer(
		consumerKey,
		privateKey,
		oauth.ServiceProvider{
			RequestTokenUrl:   fmt.Sprintf("%s/plugins/servlet/oauth/request-token", hostURL),
			AuthorizeTokenUrl: fmt.Sprintf("%s/plugins/servlet/oauth/authorize", hostURL),
			AccessTokenUrl:    fmt.Sprintf("%s/plugins/servlet/oauth/access-token", hostURL),
			HttpMethod:        http.MethodPost,
		})
	return bitbucketOauthConsumer, nil
}
