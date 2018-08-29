package gitlab

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/go-querystring/query"
	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/pipeline/remote/model"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"github.com/tomnomnom/linkheader"
	"github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGitlabAPI     = "https://gitlab.com/api/v4"
	defaultGitlabHost    = "gitlab.com"
	maxPerPage           = "100"
	gitlabAPI            = "%s%s/api/v4"
	gitlabLoginName      = "oauth2"
	accessLevelReporter  = 20
	accessLevelDeveloper = 30
	accessLevelMaster    = 40
)

type client struct {
	Scheme       string
	Host         string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	API          string
}

func New(config *v3.GitlabPipelineConfig) (model.Remote, error) {
	if config == nil {
		return nil, errors.New("empty gitlab config")
	}
	glClient := &client{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
	}
	if config.Hostname != "" && config.Hostname != defaultGitlabHost {
		glClient.Host = config.Hostname
		if config.TLS {
			glClient.Scheme = "https://"
		} else {
			glClient.Scheme = "http://"
		}
		glClient.API = fmt.Sprintf(gitlabAPI, glClient.Scheme, glClient.Host)
	} else {
		glClient.Scheme = "https://"
		glClient.Host = defaultGitlabHost
		glClient.API = defaultGitlabAPI
	}
	return glClient, nil
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

func (c *client) Login(code string) (*v3.SourceCodeCredential, error) {
	gitlabOauthConfig := &oauth2.Config{
		RedirectURL:  c.RedirectURL,
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
	user, repo, err := getUserRepoFromURL(pipeline.Spec.RepositoryURL)
	if err != nil {
		return "", err
	}
	project := url.QueryEscape(user + "/" + repo)
	hookURL := fmt.Sprintf("%s/hooks?pipelineId=%s", settings.ServerURL.Get(), ref.Ref(pipeline))
	opt := &gitlab.AddProjectHookOptions{
		PushEvents:          gitlab.Bool(true),
		MergeRequestsEvents: gitlab.Bool(true),
		TagPushEvents:       gitlab.Bool(true),
		URL:                 gitlab.String(hookURL),
		EnableSSLVerification: gitlab.Bool(false),
		Token: gitlab.String(pipeline.Status.Token),
	}

	url := fmt.Sprintf("%s/projects/%s/hooks", c.API, project)

	resp, err := doRequsetToGitlab(http.MethodPost, url, accessToken, opt)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	hook := &gitlab.ProjectHook{}
	err = json.Unmarshal(respData, hook)
	if err != nil {
		return "", err
	}

	return strconv.Itoa(hook.ID), nil
}

func (c *client) DeleteHook(pipeline *v3.Pipeline, accessToken string) error {
	user, repo, err := getUserRepoFromURL(pipeline.Spec.RepositoryURL)
	if err != nil {
		return err
	}
	project := url.QueryEscape(user + "/" + repo)

	hook, err := c.getHook(pipeline, accessToken)
	if err != nil {
		return err
	}
	if hook != nil {
		url := fmt.Sprintf("%s/projects/%s/hooks/%v", c.API, project, hook.ID)
		resp, err := doRequsetToGitlab(http.MethodDelete, url, accessToken, nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}
	return nil
}

func (c *client) getHook(pipeline *v3.Pipeline, accessToken string) (*gitlab.ProjectHook, error) {
	user, repo, err := getUserRepoFromURL(pipeline.Spec.RepositoryURL)
	if err != nil {
		return nil, err
	}
	project := url.QueryEscape(user + "/" + repo)

	var hooks []gitlab.ProjectHook
	var result *gitlab.ProjectHook
	url := fmt.Sprintf(c.API+"/projects/%s/hooks", project)
	resp, err := getFromGitlab(accessToken, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &hooks); err != nil {
		return nil, err
	}
	for _, hook := range hooks {
		if strings.HasSuffix(hook.URL, fmt.Sprintf("hooks?pipelineId=%s", ref.Ref(pipeline))) {
			result = &hook
		}
	}
	return result, nil
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
	file, err := c.getFileFromRepo(utils.PipelineFileYml, owner, repo, ref, accessToken)
	if err != nil {
		//look for both suffix
		file, err = c.getFileFromRepo(utils.PipelineFileYaml, owner, repo, ref, accessToken)
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

func (c *client) SetPipelineFileInRepo(repoURL string, branch string, accessToken string, content []byte) error {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return err
	}

	currentFile, err := c.getFileFromRepo(utils.PipelineFileYml, owner, repo, branch, accessToken)
	currentFileName := utils.PipelineFileYml
	if err != nil {
		if httpErr, ok := err.(*httperror.APIError); !ok || httpErr.Code.Status != http.StatusNotFound {
			return err
		}
		//look for both suffix
		currentFile, err = c.getFileFromRepo(utils.PipelineFileYaml, owner, repo, branch, accessToken)
		if err != nil {
			if httpErr, ok := err.(*httperror.APIError); !ok || httpErr.Code.Status != http.StatusNotFound {
				return err
			}
		} else {
			currentFileName = utils.PipelineFileYaml
		}
	}

	project := url.QueryEscape(owner + "/" + repo)
	url := fmt.Sprintf("%s/projects/%s/repository/files/%s?branch=%s", c.API, project, currentFileName, branch)
	message := "Create .rancher-pipeline.yml file"
	contentStr := string(content)
	method := http.MethodPost
	option := &gitlab.CreateFileOptions{
		Branch:        &branch,
		CommitMessage: &message,
		Content:       &contentStr,
	}

	if currentFile != nil {
		//update pipeline file
		method = http.MethodPut
		message = fmt.Sprintf("Update %s file", currentFileName)
		option.CommitMessage = &message
	}

	resp, err := doRequsetToGitlab(method, url, accessToken, option)
	defer resp.Body.Close()

	return nil
}

func (c *client) GetBranches(repoURL string, accessToken string) ([]string, error) {
	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return nil, err
	}

	project := url.QueryEscape(owner + "/" + repo)
	url := fmt.Sprintf(c.API+"/projects/%s/repository/branches", project)

	resp, err := getFromGitlab(accessToken, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	branches := []gitlab.Branch{}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &branches); err != nil {
		return nil, err
	}
	result := []string{}
	for _, branch := range branches {
		result = append(result, branch.Name)
	}

	return result, nil
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

func (c *client) GetHeadInfo(repoURL string, branch string, accessToken string) (*model.BuildInfo, error) {

	owner, repo, err := getUserRepoFromURL(repoURL)
	if err != nil {
		return nil, err
	}
	project := url.QueryEscape(owner + "/" + repo)
	url := fmt.Sprintf(c.API+"/projects/%s/repository/commits?with_stats=true&ref_name=%s", project, branch)

	resp, err := getFromGitlab(accessToken, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	commits := []gitlab.Commit{}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &commits); err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, errors.New("no commit found")
	}
	headCommit := commits[0]
	info := &model.BuildInfo{}
	info.Commit = headCommit.ID
	info.Ref = "refs/heads/" + branch
	info.Branch = branch
	info.Message = headCommit.Message
	info.Email = headCommit.AuthorEmail
	info.Author = headCommit.AuthorName
	info.HTMLLink = fmt.Sprintf("%s%s/%s/%s/commit/%s", c.Scheme, c.Host, owner, repo, headCommit.ID)
	userInfo, err := c.getGitlabUser(accessToken)
	if err != nil {
		return nil, err
	}
	info.AvatarURL = userInfo.AvatarURL

	return info, nil
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
	return doRequsetToGitlab(http.MethodGet, url, gitlabAccessToken, nil)
}

func doRequsetToGitlab(method string, url string, gitlabAccessToken string, opt interface{}) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	//set to max 100 per page to reduce query time
	if method == http.MethodGet {
		q := req.URL.Query()
		q.Set("per_page", maxPerPage)
		req.URL.RawQuery = q.Encode()
	}
	if opt != nil {
		q := req.URL.Query()
		optq, err := query.Values(opt)
		if err != nil {
			return nil, err
		}
		for k, v := range optq {
			q[k] = v
		}
		req.URL.RawQuery = q.Encode()
	}
	if gitlabAccessToken != "" {
		req.Header.Add("Authorization", "Bearer "+gitlabAccessToken)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/51.0.2704.103 Safari/537.36)")
	resp, err := client.Do(req)
	if err != nil {
		return resp, err
	}
	// Check the status code
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		var body bytes.Buffer
		io.Copy(&body, resp.Body)
		return resp, httperror.NewAPIErrorLong(resp.StatusCode, "", body.String())
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
	account.Spec.LoginName = gitlabAccount.Username
	account.Spec.GitLoginName = gitlabLoginName
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
		if accessLevel >= accessLevelReporter {
			// 20 for 'Reporter' level
			r.Spec.Permissions.Pull = true
		}
		if accessLevel >= accessLevelDeveloper {
			// 30 for 'Developer' level
			r.Spec.Permissions.Push = true
		}
		if accessLevel >= accessLevelMaster {
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

func getUserRepoFromURL(repoURL string) (string, string, error) {
	reg := regexp.MustCompile(".*/([^/]*?)/([^/]*?).git")
	match := reg.FindStringSubmatch(repoURL)
	if len(match) != 3 {
		return "", "", fmt.Errorf("error getting user/repo from gitrepoUrl:%v", repoURL)
	}
	return match[1], match[2], nil
}
