package gitlab

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/catalog/git"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"github.com/tomnomnom/linkheader"
	"github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultGitlabAPI = "https://gitlab.com/api/v3"
	maxPerPage       = "100"
	gitlabAPI        = "%s%s/api/v3"
	gitlabLoginName  = "oauth2"
)

type client struct {
	Scheme       string
	Host         string
	ClientID     string
	ClientSecret string
	API          string
}

func New(pipeline v3.ClusterPipeline) (model.Remote, error) {
	if pipeline.Spec.GitlabConfig == nil {
		return nil, errors.New("gitlab is not configured")
	}
	remote := &client{
		ClientID:     pipeline.Spec.GitlabConfig.ClientID,
		ClientSecret: pipeline.Spec.GitlabConfig.ClientSecret,
	}
	if pipeline.Spec.GitlabConfig.Host != "" && pipeline.Spec.GitlabConfig.Host != "gitlab.com" {
		remote.Host = pipeline.Spec.GitlabConfig.Host
		if pipeline.Spec.GitlabConfig.TLS {
			remote.Scheme = "https://"
		} else {
			remote.Scheme = "http://"
		}
		remote.API = fmt.Sprintf(gitlabAPI, remote.Scheme, remote.Host)
	} else {
		remote.Scheme = "https://"
		remote.Host = "gitlab.com"
		remote.API = defaultGitlabAPI
	}
	return remote, nil
}

func (c *client) Type() string {
	return model.GitlabType
}

func (c *client) CanLogin() bool {
	return true
}

func (c *client) CanRepos() bool {
	return true
}

func (c *client) CanHook() bool {
	return true
}

func (c *client) Login(redirectURL string, code string) (*v3.SourceCodeCredential, error) {
	gitlabOauthConfig := &oauth2.Config{
		RedirectURL:  redirectURL,
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Scopes:       []string{"api"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("%s%s/oauth/authorize", c.Scheme, c.Host),
			TokenURL: fmt.Sprintf("%s%s/oauth/token", c.Scheme, c.Host),
		},
	}
	token, err := gitlabOauthConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, err
	} else if token.TokenType != "bearer" || token.AccessToken == "" {
		return nil, fmt.Errorf("Fail to get accesstoken with oauth config")
	}
	return c.GetAccount(token.AccessToken)
}

func (c *client) Repos(account *v3.SourceCodeCredential) ([]v3.SourceCodeRepository, error) {
	if account == nil {
		return nil, fmt.Errorf("empty account")
	}
	accessToken := account.Spec.AccessToken
	return c.getGitlabRepos(accessToken)
}

func (c *client) CreateHook(pipeline *v3.Pipeline, accessToken string) (string, error) {
	if err := utils.ValidPipelineSpec(pipeline.Spec); err != nil {
		return "", err
	}
	sourceCodeConfig := pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig
	user, repo, err := getUserRepoFromURL(sourceCodeConfig.URL)
	if err != nil {
		return "", err
	}

	hookURL := fmt.Sprintf("%s?pipelineId=%s:%s", utils.CIEndpoint, pipeline.Namespace, pipeline.Name)
	id, err := c.createGitlabWebhook(user, repo, accessToken, hookURL, pipeline.Status.Token)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (c *client) DeleteHook(pipeline *v3.Pipeline, accessToken string) error {
	if err := utils.ValidPipelineSpec(pipeline.Spec); err != nil {
		return err
	}
	sourceCodeConfig := pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig
	user, repo, err := getUserRepoFromURL(sourceCodeConfig.URL)
	if err != nil {
		return err
	}

	return c.deleteGitlabWebhook(user, repo, accessToken, pipeline.Status.WebHookID)
}

func (c *client) getFileFromRepo(filename string, owner string, repo string, ref string, accessToken string) (*gitlab.File, error) {

	project := url.QueryEscape(owner + "/" + repo)
	url := fmt.Sprintf("%s/projects/%s/repository/files/%s?ref=%s", c.API, project, filename, ref)
	resp, err := getFromGitlab(accessToken, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	file := &gitlab.File{}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, file); err != nil {
		return nil, err
	}
	return file, nil
}

func (c *client) GetPipelineFileInRepo(repoURL string, ref string, accessToken string) ([]byte, error) {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return nil, err
	}
	if ref == "" {
		defaultBranch, err := c.GetDefaultBranch(repoURL, accessToken)
		if err != nil {
			return nil, err
		}
		ref = defaultBranch
	}
	file, err := c.getFileFromRepo(".pipeline.yaml", owner, repo, ref, accessToken)
	if err != nil {
		//look for both suffix
		file, err = c.getFileFromRepo(".pipeline.yml", owner, repo, ref, accessToken)
	}
	if err != nil {
		logrus.Debugf("error GetPipelineFileInRepo - %v", err)
		return nil, nil
	}
	if file.Content != "" {
		b, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return nil, err
		}

		return b, nil
	}
	return nil, nil
}

func (c *client) GetDefaultBranch(repoURL string, accessToken string) (string, error) {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return "", err
	}

	project := url.QueryEscape(owner + "/" + repo)
	url := fmt.Sprintf(c.API+"/projects/%s", project)

	resp, err := getFromGitlab(accessToken, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	p := &gitlab.Project{}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(b, p); err != nil {
		return "", err
	}
	if p.DefaultBranch != "" {
		return p.DefaultBranch, nil
	}
	return "", nil
}

func (c *client) GetHeadCommit(repoURL string, branch string, credential *v3.SourceCodeCredential) (string, error) {
	if credential != nil {
		userName := credential.Spec.LoginName
		token := credential.Spec.AccessToken
		repoURL = strings.Replace(repoURL, "://", "://"+userName+":"+token+"@", 1)
	}

	return git.RemoteBranchHeadCommit(repoURL, branch)
}

func (c *client) GetAccount(accessToken string) (*v3.SourceCodeCredential, error) {
	account, err := c.getGitlabUser(accessToken)
	if err != nil {
		return nil, err
	}
	remoteAccount := convertAccount(account)
	remoteAccount.Spec.AccessToken = accessToken
	return remoteAccount, nil
}

func (c *client) getGitlabUser(gitlabAccessToken string) (*gitlab.User, error) {

	url := c.API + "/user"
	resp, err := getFromGitlab(gitlabAccessToken, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	gitlabAcct := &gitlab.User{}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, gitlabAcct); err != nil {
		return nil, err
	}
	return gitlabAcct, nil
}

func getFromGitlab(gitlabAccessToken string, url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logrus.Error(err)
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	q := req.URL.Query()
	q.Set("per_page", maxPerPage)
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Authorization", "Bearer "+gitlabAccessToken)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36)")
	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("Received error from gitlab: %v", err)
		return resp, err
	}
	// Check the status code
	switch resp.StatusCode {
	case 200:
	case 201:
	default:
		var body bytes.Buffer
		io.Copy(&body, resp.Body)
		return resp, fmt.Errorf("Request failed, got status code: %d. Response: %s",
			resp.StatusCode, body.Bytes())
	}
	return resp, nil
}

func paginateGitlab(gitlabAccessToken string, url string) ([]*http.Response, error) {
	var responses []*http.Response

	response, err := getFromGitlab(gitlabAccessToken, url)
	if err != nil {
		return responses, err
	}
	responses = append(responses, response)
	nextURL := nextGitlabPage(response)
	for nextURL != "" {
		response, err = getFromGitlab(gitlabAccessToken, nextURL)
		if err != nil {
			return responses, err
		}
		responses = append(responses, response)
		nextURL = nextGitlabPage(response)
	}

	return responses, nil
}

func nextGitlabPage(response *http.Response) string {
	header := response.Header.Get("link")

	if header != "" {
		links := linkheader.Parse(header)
		for _, link := range links {
			if link.Rel == "next" {
				return link.URL
			}
		}
	}

	return ""
}

func convertAccount(gitlabAccount *gitlab.User) *v3.SourceCodeCredential {

	if gitlabAccount == nil {
		return nil
	}
	account := &v3.SourceCodeCredential{}
	account.Spec.SourceCodeType = model.GitlabType

	account.Spec.AvatarURL = gitlabAccount.AvatarURL
	account.Spec.HTMLURL = gitlabAccount.WebsiteURL
	account.Spec.LoginName = gitlabLoginName
	account.Spec.DisplayName = gitlabAccount.Name

	return account

}

func (c *client) getGitlabRepos(gitlabAccessToken string) ([]v3.SourceCodeRepository, error) {
	url := c.API + "/projects?membership=true"
	var repos []gitlab.Project
	responses, err := paginateGitlab(gitlabAccessToken, url)
	if err != nil {
		return nil, err
	}
	for _, response := range responses {
		defer response.Body.Close()
		b, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		var reposObj []gitlab.Project
		if err := json.Unmarshal(b, &reposObj); err != nil {
			return nil, err
		}
		repos = append(repos, reposObj...)
	}

	return convertRepos(repos), nil
}

func convertRepos(repos []gitlab.Project) []v3.SourceCodeRepository {
	result := []v3.SourceCodeRepository{}
	for _, repo := range repos {
		r := v3.SourceCodeRepository{}

		r.Spec.URL = repo.HTTPURLToRepo
		//r.Spec.Language = No language info in gitlab API
		r.Spec.DefaultBranch = repo.DefaultBranch

		accessLevel := getAccessLevel(repo)
		if accessLevel >= 20 {
			// 20 for 'Reporter' level
			r.Spec.Permissions.Pull = true
		}
		if accessLevel >= 30 {
			// 30 for 'Developer' level
			r.Spec.Permissions.Push = true
		}
		if accessLevel >= 40 {
			// 40 for 'Master' level and 50 for 'Owner' level
			r.Spec.Permissions.Admin = true
		}
		result = append(result, r)
	}
	return result
}

func getAccessLevel(repo gitlab.Project) int {
	accessLevel := 0
	if repo.Permissions == nil {
		return accessLevel
	}
	if repo.Permissions.ProjectAccess != nil && int(repo.Permissions.ProjectAccess.AccessLevel) > accessLevel {
		accessLevel = int(repo.Permissions.ProjectAccess.AccessLevel)
	}
	if repo.Permissions.GroupAccess != nil && int(repo.Permissions.GroupAccess.AccessLevel) > accessLevel {
		accessLevel = int(repo.Permissions.GroupAccess.AccessLevel)
	}
	return accessLevel
}
