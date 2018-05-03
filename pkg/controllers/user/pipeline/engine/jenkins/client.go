package jenkins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

var (
	ErrNotFound = errors.New("Not Found")
)

type Client struct {
	API         string
	User        string
	Token       string
	CrumbHeader string
	CrumbBody   string

	HTTPClient *http.Client
}

func New(api string, user string, token string, httpClient *http.Client) (*Client, error) {
	c := &Client{
		API:        api,
		User:       user,
		Token:      token,
		HTTPClient: httpClient,
	}

	if err := c.getCSRF(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) getCSRF() error {
	getCrumbURL, err := url.Parse(c.API + GetCrumbURI)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(http.MethodGet, getCrumbURL.String(), nil)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := ioutil.ReadAll(resp.Body)
	Crumbs := strings.Split(string(data), ":")
	if len(Crumbs) != 2 {
		return errors.New("error get crumbs, Jenkins is probably not ready yet")
	}
	c.CrumbHeader = Crumbs[0]
	c.CrumbBody = Crumbs[1]
	return nil
}

//deleteBuild deletes the last build of a job
func (c *Client) deleteBuild(jobname string, buildNumber int) error {
	deleteBuildURI := fmt.Sprintf(DeleteBuildURI, jobname, buildNumber)
	var targetURL *url.URL
	var err error
	targetURL, err = url.Parse(c.API + deleteBuildURI)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(http.MethodPost, targetURL.String(), nil)

	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return checkHTTPError(resp, "delete build")

}

func (c *Client) execScript(script string) (string, error) {
	var targetURL *url.URL
	var err error
	targetURL, err = url.Parse(c.API + ScriptURI)
	if err != nil {
		return "", err
	}
	v := url.Values{}
	v.Add("script", script)
	req, _ := http.NewRequest(http.MethodPost, targetURL.String(), bytes.NewBufferString(v.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := checkHTTPError(resp, "exec script"); err != nil {
		return "", err
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (c *Client) createJob(jobname string, content []byte) error {
	createJobURL, err := url.Parse(c.API + CreateJobURI)
	if err != nil {
		return err
	}
	qry := createJobURL.Query()
	qry.Add("name", jobname)
	createJobURL.RawQuery = qry.Encode()
	//send request part
	req, _ := http.NewRequest(http.MethodPost, createJobURL.String(), bytes.NewReader(content))
	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.Header.Set("Content-Type", "application/xml")
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return checkHTTPError(resp, "create job")
}

func (c *Client) updateJob(jobname string, content []byte) error {
	updateJobURI := fmt.Sprintf(UpdateJobURI, jobname)
	updateJobURL, err := url.Parse(c.API + updateJobURI)
	if err != nil {
		return err
	}
	//send request part
	req, _ := http.NewRequest(http.MethodPost, updateJobURL.String(), bytes.NewReader(content))
	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.Header.Set("Content-Type", "application/xml")
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return checkHTTPError(resp, "update job")
}

func (c *Client) buildJob(jobname string, params map[string]string) (string, error) {
	buildURI := fmt.Sprintf(JenkinsJobBuildURI, jobname)

	var targetURL *url.URL
	targetURL, err := url.Parse(c.API + buildURI)

	if err != nil {
		return "", err
	}
	req, _ := http.NewRequest(http.MethodPost, targetURL.String(), nil)

	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return "", checkHTTPError(resp, "build job")
}

func (c *Client) getBuildInfo(jobname string) (*BuildInfo, error) {
	buildInfoURI := fmt.Sprintf(JenkinsBuildInfoURI, jobname)

	var targetURL *url.URL
	var err error
	targetURL, err = url.Parse(c.API + buildInfoURI)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest(http.MethodPost, targetURL.String(), nil)

	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkHTTPError(resp, "get build info"); err != nil {
		return nil, err
	}
	buildInfo := &BuildInfo{}
	respBytes, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(respBytes, buildInfo)
	if err != nil {
		return nil, err
	}

	return buildInfo, nil

}

func (c *Client) getJobInfo(jobname string) (*JobInfo, error) {
	jobInfoURI := fmt.Sprintf(JenkinsJobInfoURI, jobname)
	var targetURL *url.URL
	var err error
	targetURL, err = url.Parse(c.API + jobInfoURI)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest(http.MethodGet, targetURL.String(), nil)

	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkHTTPError(resp, "get job info"); err != nil {
		return nil, err
	}
	jobInfo := &JobInfo{}
	respBytes, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(respBytes, jobInfo)
	if err != nil {
		return nil, err
	}

	return jobInfo, nil

}

func (c *Client) getBuildRawOutput(jobname string, buildNumber int, startLine int) (string, error) {
	buildRawOutputURI := fmt.Sprintf(JenkinsBuildLogURI, jobname, buildNumber)
	if startLine > 0 {
		buildRawOutputURI += "&startLine=" + strconv.Itoa(startLine)
	}
	var targetURL *url.URL
	var err error
	targetURL, err = url.Parse(c.API + buildRawOutputURI)
	if err != nil {
		return "", err
	}
	req, _ := http.NewRequest(http.MethodGet, targetURL.String(), nil)

	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := checkHTTPError(resp, "get build output"); err != nil {
		return "", err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respBytes), nil

}

func (c *Client) stopJob(jobname string, buildNumber int) error {
	stopJobURI := fmt.Sprintf(StopJobURI, jobname, buildNumber)
	targetURL, err := url.Parse(c.API + stopJobURI)

	if err != nil {
		return err
	}
	req, _ := http.NewRequest(http.MethodPost, targetURL.String(), nil)

	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return checkHTTPError(resp, "stop job")
}

func (c *Client) cancelQueueItem(id int) error {
	cancelQueueItemURI := fmt.Sprintf(CancelQueueItemURI, id)
	targetURL, err := url.Parse(c.API + cancelQueueItemURI)

	if err != nil {
		return err
	}
	req, _ := http.NewRequest(http.MethodPost, targetURL.String(), nil)

	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.SetBasicAuth(c.User, c.Token)
	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			//no redirect
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return checkHTTPError(resp, "cancel queue")
}

func (c *Client) getWFBuildInfo(jobname string) (*WFBuildInfo, error) {
	buildInfoURI := fmt.Sprintf(JenkinsWFBuildInfoURI, jobname)

	var targetURL *url.URL
	var err error
	targetURL, err = url.Parse(c.API + buildInfoURI)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest(http.MethodGet, targetURL.String(), nil)

	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkHTTPError(resp, "get build info"); err != nil {
		return nil, err
	}
	buildInfo := &WFBuildInfo{}
	respBytes, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(respBytes, buildInfo)
	if err != nil {
		return nil, err
	}

	return buildInfo, nil

}

func (c *Client) getWFNodeInfo(jobname string, nodeID string) (*WFNodeInfo, error) {
	nodeInfoURI := fmt.Sprintf(JenkinsWFNodeInfoURI, jobname, nodeID)

	var targetURL *url.URL
	var err error
	targetURL, err = url.Parse(c.API + nodeInfoURI)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest(http.MethodGet, targetURL.String(), nil)

	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkHTTPError(resp, "get WFNode info"); err != nil {
		return nil, err
	}
	nodeInfo := &WFNodeInfo{}
	respBytes, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(respBytes, nodeInfo)
	if err != nil {
		return nil, err
	}

	return nodeInfo, nil

}

func (c *Client) getWFNodeLog(jobname string, nodeID string) (*WFNodeLog, error) {
	nodeLogURI := fmt.Sprintf(JenkinsWFNodeLogURI, jobname, nodeID)

	var targetURL *url.URL
	var err error
	targetURL, err = url.Parse(c.API + nodeLogURI)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest(http.MethodGet, targetURL.String(), nil)

	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := checkHTTPError(resp, "get WFNode log"); err != nil {
		return nil, err
	}
	nodeLog := &WFNodeLog{}
	respBytes, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(respBytes, nodeLog)
	if err != nil {
		return nil, err
	}

	return nodeLog, nil

}

func (c *Client) createCredential(content []byte) error {

	setCredURL, err := url.Parse(c.API + JenkinsSetCredURI)
	if err != nil {
		return err
	}

	req, _ := http.NewRequest(http.MethodPost, setCredURL.String(), bytes.NewReader(content))
	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return checkHTTPError(resp, "create credential")
}

func (c *Client) getCredential(credentialID string) error {

	getCredURI := fmt.Sprintf(JenkinsGetCredURI, credentialID)
	getCredURL, err := url.Parse(c.API + getCredURI)
	if err != nil {
		return err
	}

	req, _ := http.NewRequest(http.MethodGet, getCredURL.String(), nil)
	req.Header.Add(c.CrumbHeader, c.CrumbBody)
	req.SetBasicAuth(c.User, c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return checkHTTPError(resp, "get credential")
}

func checkHTTPError(resp *http.Response, event string) error {
	if resp == nil {
		return nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("%s error - status %d, result: %s", event, resp.StatusCode, string(data))
	}
	return nil
}
