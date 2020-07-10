package bitbucketcloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/rancher/pkg/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const (
	apiEndpoint   = "https://api.bitbucket.org/2.0"
	authURL       = "https://bitbucket.org/site/oauth2/authorize"
	tokenURL      = "https://bitbucket.org/site/oauth2/access_token"
	maxPerPage    = "100"
	cloneUserName = "x-token-auth"
)

type client struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func New(config *v3.BitbucketCloudPipelineConfig) (model.Remote, error) {
	if config == nil {
		return nil, errors.New("empty gitlab config")
	}
	glClient := &client{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
	}
	return glClient, nil
}

func (c *client) Type() string {
	return model.BitbucketCloudType
}

func (c *client) Login(code string) (*v3.SourceCodeCredential, error) {
	oauthConfig := &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
	}

	token, err := oauthConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, err
	} else if token.TokenType != "bearer" || token.AccessToken == "" {
		return nil, fmt.Errorf("Fail to get accesstoken with oauth config")
	}

	user, err := c.getUser(token.AccessToken)
	if err != nil {
		return nil, err
	}
	cred := convertUser(user)
	cred.Spec.AccessToken = token.AccessToken
	cred.Spec.RefreshToken = token.RefreshToken
	cred.Spec.Expiry = token.Expiry.Format(time.RFC3339)
	return cred, nil
}

func (c *client) Repos(account *v3.SourceCodeCredential) ([]v3.SourceCodeRepository, error) {
	if account == nil {
		return nil, fmt.Errorf("empty account")
	}
	nexturl := apiEndpoint + "/repositories?role=admin"
	var repos []Repository
	for nexturl != "" {
		b, err := getFromBitbucket(nexturl, account.Spec.AccessToken)
		if err != nil {
			return nil, err
		}
		var pageRepos PaginatedRepositories
		if err := json.Unmarshal(b, &pageRepos); err != nil {
			return nil, err
		}
		nexturl = pageRepos.Next
		repos = append(repos, pageRepos.Values...)
	}

	return convertRepos(repos), nil
}

func (c *client) CreateHook(pipeline *v3.Pipeline, accessToken string) (string, error) {
	user, repo, err := getUserRepoFromURL(pipeline.Spec.RepositoryURL)
	if err != nil {
		return "", err
	}
	hookURL := fmt.Sprintf("%s/hooks?pipelineId=%s", settings.ServerURL.Get(), ref.Ref(pipeline))
	hook := Hook{
		Description:          "Webhook created by Rancher Pipeline",
		URL:                  hookURL,
		Active:               true,
		SkipCertVerification: true,
		Events: []string{
			"repo:push",
			"pullrequest:updated",
			"pullrequest:created",
		},
	}

	url := fmt.Sprintf("%s/repositories/%s/%s/hooks", apiEndpoint, user, repo)
	b, err := json.Marshal(hook)
	if err != nil {
		return "", err
	}
	reader := bytes.NewReader(b)

	resp, err := doRequestToBitbucket(http.MethodPost, url, accessToken, nil, reader)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(resp, &hook)
	if err != nil {
		return "", err
	}

	return hook.UUID, nil
}

func (c *client) DeleteHook(pipeline *v3.Pipeline, accessToken string) error {
	user, repo, err := getUserRepoFromURL(pipeline.Spec.RepositoryURL)
	if err != nil {
		return err
	}

	hook, err := c.getHook(pipeline, accessToken)
	if err != nil {
		return err
	}
	if hook != nil {
		url := fmt.Sprintf("%s/repositories/%s/%s/hooks/%v", apiEndpoint, user, repo, hook.UUID)
		_, err := doRequestToBitbucket(http.MethodDelete, url, accessToken, nil, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *client) getHook(pipeline *v3.Pipeline, accessToken string) (*Hook, error) {
	user, repo, err := getUserRepoFromURL(pipeline.Spec.RepositoryURL)
	if err != nil {
		return nil, err
	}

	var hooks PaginatedHooks
	var result *Hook
	url := fmt.Sprintf("%s/repositories/%s/%s/hooks", apiEndpoint, user, repo)

	b, err := getFromBitbucket(url, accessToken)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &hooks); err != nil {
		return nil, err
	}
	for _, hook := range hooks.Values {
		if strings.HasSuffix(hook.URL, fmt.Sprintf("hooks?pipelineId=%s", ref.Ref(pipeline))) {
			result = &hook
			break
		}
	}
	return result, nil
}

func (c *client) getFileFromRepo(filename string, owner string, repo string, branch string, accessToken string) ([]byte, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/src/%s/%s", apiEndpoint, owner, repo, branch, filename)
	return getFromBitbucket(url, accessToken)
}

func (c *client) GetPipelineFileInRepo(repoURL string, branch string, accessToken string) ([]byte, error) {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return nil, err
	}
	content, err := c.getFileFromRepo(utils.PipelineFileYaml, owner, repo, branch, accessToken)
	if err != nil {
		//look for both suffix
		content, err = c.getFileFromRepo(utils.PipelineFileYml, owner, repo, branch, accessToken)
	}
	if err != nil {
		logrus.Debugf("error GetPipelineFileInRepo - %v", err)
		return nil, nil
	}
	return content, nil
}

func (c *client) SetPipelineFileInRepo(repoURL string, branch string, accessToken string, content []byte) error {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return err
	}

	currentContent, err := c.getFileFromRepo(utils.PipelineFileYml, owner, repo, branch, accessToken)
	currentFileName := utils.PipelineFileYml
	if err != nil {
		if httpErr, ok := err.(*httperror.APIError); !ok || httpErr.Code.Status != http.StatusNotFound {
			return err
		}
		//look for both suffix
		currentContent, err = c.getFileFromRepo(utils.PipelineFileYaml, owner, repo, branch, accessToken)
		if err != nil {
			if httpErr, ok := err.(*httperror.APIError); !ok || httpErr.Code.Status != http.StatusNotFound {
				return err
			}
		} else {
			currentFileName = utils.PipelineFileYaml
		}
	}

	apiurl := fmt.Sprintf("%s/repositories/%s/%s/src", apiEndpoint, owner, repo)
	message := "Create .rancher-pipeline.yml file"
	if currentContent != nil {
		//update pipeline file
		message = fmt.Sprintf("Update %s file", currentFileName)
	}

	data := url.Values{}
	data.Set("message", message)
	data.Set("branch", branch)
	data.Set(currentFileName, string(content))
	data.Encode()
	reader := strings.NewReader(data.Encode())
	header := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	_, err = doRequestToBitbucket(http.MethodPost, apiurl, accessToken, header, reader)

	return err
}

func (c *client) GetBranches(repoURL string, accessToken string) ([]string, error) {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return nil, err
	}

	nexturl := fmt.Sprintf("%s/repositories/%s/%s/refs/branches", apiEndpoint, owner, repo)
	var result []string
	for nexturl != "" {
		b, err := getFromBitbucket(nexturl, accessToken)
		if err != nil {
			return nil, err
		}
		var pageBranches PaginatedBranches
		if err := json.Unmarshal(b, &pageBranches); err != nil {
			return nil, err
		}
		for _, branch := range pageBranches.Values {
			result = append(result, branch.Name)
		}
		nexturl = pageBranches.Next
	}

	return result, nil
}

func (c *client) GetHeadInfo(repoURL string, branch string, accessToken string) (*model.BuildInfo, error) {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/repositories/%s/%s/refs/branches/%s", apiEndpoint, owner, repo, branch)

	b, err := getFromBitbucket(url, accessToken)
	if err != nil {
		return nil, err
	}
	var branchObj Ref
	if err := json.Unmarshal(b, &branchObj); err != nil {
		return nil, err
	}
	info := &model.BuildInfo{}
	info.Commit = branchObj.Target.Hash
	info.Ref = branch
	info.Branch = branch
	info.Message = branchObj.Target.Message
	info.HTMLLink = branchObj.Links.HTML.Href
	info.AvatarURL = branchObj.Target.Author.User.Links.Avatar.Href
	info.Author = branchObj.Target.Author.User.UserName

	return info, nil
}

func convertUser(bitbucketUser *User) *v3.SourceCodeCredential {

	if bitbucketUser == nil {
		return nil
	}
	cred := &v3.SourceCodeCredential{}
	cred.Spec.SourceCodeType = model.BitbucketCloudType

	cred.Spec.AvatarURL = bitbucketUser.Links.Avatar.Href
	cred.Spec.HTMLURL = bitbucketUser.Links.HTML.Href
	cred.Spec.LoginName = bitbucketUser.UserName
	cred.Spec.GitLoginName = cloneUserName
	cred.Spec.DisplayName = bitbucketUser.DisplayName

	return cred

}

func (c *client) getUser(accessToken string) (*User, error) {
	url := apiEndpoint + "/user"
	b, err := getFromBitbucket(url, accessToken)
	if err != nil {
		return nil, err
	}
	user := &User{}
	if err := json.Unmarshal(b, user); err != nil {
		return nil, err
	}
	return user, nil
}

func convertRepos(repos []Repository) []v3.SourceCodeRepository {
	result := []v3.SourceCodeRepository{}
	for _, repo := range repos {
		if repo.Scm != "git" {
			//skip mercurial repos
			continue
		}
		r := v3.SourceCodeRepository{}
		for _, link := range repo.Links.Clone {
			if link.Name == "https" {
				u, _ := url.Parse(link.Href)
				if u != nil {
					u.User = nil
					r.Spec.URL = u.String()
				}
				break
			}
		}
		r.Spec.DefaultBranch = repo.MainBranch.Name
		r.Spec.Permissions.Admin = true
		r.Spec.Permissions.Pull = true
		r.Spec.Permissions.Push = true
		result = append(result, r)
	}
	return result
}

func (c *client) Refresh(cred *v3.SourceCodeCredential) (bool, error) {
	if cred == nil {
		return false, errors.New("cannot refresh empty credentials")
	}
	config := &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
	}
	source := config.TokenSource(
		oauth2.NoContext, &oauth2.Token{RefreshToken: cred.Spec.RefreshToken})

	token, err := source.Token()
	if err != nil || len(token.AccessToken) == 0 {
		return false, err
	}

	cred.Spec.AccessToken = token.AccessToken
	cred.Spec.RefreshToken = token.RefreshToken
	cred.Spec.Expiry = token.Expiry.Format(time.RFC3339)

	return true, nil

}

func getFromBitbucket(url string, accessToken string) ([]byte, error) {
	return doRequestToBitbucket(http.MethodGet, url, accessToken, nil, nil)
}

func doRequestToBitbucket(method string, url string, accessToken string, header map[string]string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	q := req.URL.Query()
	//set to max 100 per page to reduce query time
	if method == http.MethodGet {
		q.Set("pagelen", maxPerPage)
	}
	if accessToken != "" {
		q.Set("access_token", accessToken)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Add("Cache-control", "no-cache")
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Check the status code
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		var body bytes.Buffer
		io.Copy(&body, resp.Body)
		return nil, httperror.NewAPIErrorLong(resp.StatusCode, "", body.String())
	}
	r, err := ioutil.ReadAll(resp.Body)
	return r, err
}

func getUserRepoFromURL(repoURL string) (string, string, error) {
	reg := regexp.MustCompile(".*/([^/]*?)/([^/]*?).git")
	match := reg.FindStringSubmatch(repoURL)
	if len(match) != 3 {
		return "", "", fmt.Errorf("error getting user/repo from gitrepoUrl:%v", repoURL)
	}
	return match[1], match[2], nil
}
