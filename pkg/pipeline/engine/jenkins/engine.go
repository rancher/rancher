package jenkins

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/pipeline/providers"
	"github.com/rancher/rancher/pkg/pipeline/remote"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	appsv1 "github.com/rancher/types/apis/apps/v1"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/project.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	stepNameRe = regexp.MustCompile(`step-\d+-\d+`)
)

type Engine struct {
	// UseCache affects resources that is not cached in follower instances of HA mode
	UseCache         bool
	JenkinsClient    *Client
	HTTPClient       *http.Client
	ServiceLister    v1.ServiceLister
	PodLister        v1.PodLister
	DeploymentLister appsv1.DeploymentLister

	Secrets                    v1.SecretInterface
	SecretLister               v1.SecretLister
	ManagementSecretLister     v1.SecretLister
	SourceCodeCredentials      v3.SourceCodeCredentialInterface
	SourceCodeCredentialLister v3.SourceCodeCredentialLister
	PipelineLister             v3.PipelineLister
	PipelineSettingLister      v3.PipelineSettingLister

	ClusterName string
	Dialer      dialer.Factory
}

func (j *Engine) getJenkinsURL(execution *v3.PipelineExecution) (string, error) {
	ns := utils.GetPipelineCommonName(execution.Spec.ProjectName)
	service, err := j.ServiceLister.Get(ns, utils.JenkinsName)
	if err != nil {
		return "", err
	}
	ip := service.Spec.ClusterIP
	return fmt.Sprintf("http://%s:%d", ip, utils.JenkinsPort), nil

}

func (j *Engine) PreCheck(execution *v3.PipelineExecution) (bool, error) {
	var pod *corev1.Pod
	var err error
	set := labels.Set(map[string]string{utils.LabelKeyApp: utils.JenkinsName})
	ns := utils.GetPipelineCommonName(execution.Spec.ProjectName)
	pods, err := j.PodLister.List(ns, set.AsSelector())
	if err != nil {
		return false, err
	}
	if len(pods) <= 0 {
		return false, errors.New("jenkins pod not found")
	}
	pod = pods[0]

	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true, nil
		}
	}

	return false, nil
}

func (j *Engine) getJenkinsClient(execution *v3.PipelineExecution) (*Client, error) {
	url, err := j.getJenkinsURL(execution)
	if err != nil {
		return nil, err
	}
	user := utils.PipelineSecretDefaultUser
	ns := utils.GetPipelineCommonName(execution.Spec.ProjectName)
	var secret *corev1.Secret
	if j.UseCache {
		secret, err = j.SecretLister.Get(ns, utils.PipelineSecretName)
	} else {
		secret, err = j.Secrets.GetNamespaced(ns, utils.PipelineSecretName, metav1.GetOptions{})
	}
	if err != nil || secret.Data == nil {
		return nil, fmt.Errorf("error get jenkins token - %v", err)
	}
	token := string(secret.Data[utils.PipelineSecretTokenKey])

	if j.HTTPClient == nil {
		dial, err := j.Dialer.ClusterDialer(j.ClusterName)
		if err != nil {
			return nil, err
		}
		j.HTTPClient = &http.Client{
			Transport: &http.Transport{
				Dial: dial,
			},
			Timeout: 15 * time.Second,
		}
	}
	return New(url, user, token, j.HTTPClient)
}

func (j *Engine) RunPipelineExecution(execution *v3.PipelineExecution) error {
	logrus.Debug("start RunPipelineExecution")
	jobName := getJobName(execution)
	client, err := j.getJenkinsClient(execution)
	if err != nil {
		return err
	}
	_, err = client.getJobInfo(jobName)
	if e, ok := err.(*httperror.APIError); ok && e.Code.Status == http.StatusNotFound {
		if err = j.createPipelineJob(client, execution); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := j.updatePipelineJob(client, execution); err != nil {
		return err
	}

	ns, name := ref.Parse(execution.Spec.PipelineName)
	pipeline, err := j.PipelineLister.Get(ns, name)
	if err != nil {
		return err
	}
	if err := j.setCredential(client, execution, pipeline.Spec.SourceCodeCredentialName); err != nil {
		return err
	}

	if err := j.preparePipeline(execution); err != nil {
		return err
	}
	if _, err := client.buildJob(jobName, map[string]string{}); err != nil {
		return err
	}
	return nil
}

func (j *Engine) preparePipeline(execution *v3.PipelineExecution) error {
	for _, stage := range execution.Spec.PipelineConfig.Stages {
		for _, step := range stage.Steps {
			if step.PublishImageConfig != nil {
				//prepare docker credential for publishimage step
				registry := utils.DefaultRegistry
				if step.PublishImageConfig.PushRemote && step.PublishImageConfig.Registry != "" {
					registry = step.PublishImageConfig.Registry
				} else {
					_, projectID := ref.Parse(execution.Spec.ProjectName)
					registry = fmt.Sprintf("%s.%s-pipeline", utils.LocalRegistry, projectID)
				}
				if err := j.prepareRegistryCredential(execution, registry); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (j *Engine) prepareRegistryCredential(execution *v3.PipelineExecution, registry string) error {
	secrets, err := j.ManagementSecretLister.List(execution.Namespace, labels.Everything())
	if err != nil {
		return err
	}
	username := ""
	password := ""
	for _, s := range secrets {
		if s.Type == "kubernetes.io/dockerconfigjson" {
			m := map[string]interface{}{}
			if err := json.Unmarshal(s.Data[".dockerconfigjson"], &m); err != nil {
				return err
			}
			auths := convert.ToMapInterface(m["auths"])
			for k, v := range auths {
				if registry != k {
					//find matching registry credential
					continue
				}
				cred := convert.ToMapInterface(v)
				username, _ = cred["username"].(string)
				password, _ = cred["password"].(string)
			}

		}

	}

	//store dockercredential in pipeline namespace
	//TODO key-key mapping instead of registry-key mapping
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	proceccedRegistry := strings.ToLower(reg.ReplaceAllString(registry, ""))

	secretName := fmt.Sprintf("%s-%s", execution.Namespace, proceccedRegistry)
	logrus.Debugf("preparing registry credential %s for %s", secretName, registry)
	ns := utils.GetPipelineCommonName(execution.Spec.ProjectName)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      secretName,
		},
		Data: map[string][]byte{
			utils.PublishSecretUserKey: []byte(username),
			utils.PublishSecretPwKey:   []byte(password),
		},
	}
	_, err = j.Secrets.Create(secret)
	if apierrors.IsAlreadyExists(err) {
		if _, err := j.Secrets.Update(secret); err != nil {
			return err
		}
		return nil
	}
	return err
}

func (j *Engine) createPipelineJob(client *Client, execution *v3.PipelineExecution) error {
	logrus.Debug("create jenkins job for pipeline")
	converter, err := initJenkinsPipelineConverter(execution, j.PipelineSettingLister, j.SecretLister)
	if err != nil {
		return err
	}
	jobconf, err := converter.convertPipelineExecutionToJenkinsPipeline()
	if err != nil {
		return err
	}
	jobName := getJobName(execution)
	bconf, _ := xml.MarshalIndent(jobconf, "  ", "    ")
	return client.createJob(jobName, bconf)
}

func (j *Engine) updatePipelineJob(client *Client, execution *v3.PipelineExecution) error {
	logrus.Debug("update jenkins job for pipeline")
	converter, err := initJenkinsPipelineConverter(execution, j.PipelineSettingLister, j.SecretLister)
	if err != nil {
		return err
	}
	jobconf, err := converter.convertPipelineExecutionToJenkinsPipeline()
	if err != nil {
		return err
	}
	jobName := getJobName(execution)
	bconf, _ := xml.MarshalIndent(jobconf, "  ", "    ")
	return client.updateJob(jobName, bconf)
}

func (j *Engine) RerunExecution(execution *v3.PipelineExecution) error {
	return j.RunPipelineExecution(execution)
}

func (j *Engine) StopExecution(execution *v3.PipelineExecution) error {
	jobName := getJobName(execution)
	client, err := j.getJenkinsClient(execution)
	if err != nil {
		return err
	}
	info, err := client.getJobInfo(jobName)
	if e, ok := err.(*httperror.APIError); ok && e.Code.Status == http.StatusNotFound {
		return nil
	} else if err != nil {
		return err
	}
	if info.InQueue {
		//delete in queue
		queueItem, ok := info.QueueItem.(map[string]interface{})
		if !ok {
			return fmt.Errorf("type assertion fail for queueitem")
		}
		queueID, ok := queueItem["id"].(float64)
		if !ok {
			return fmt.Errorf("type assertion fail for queueID")
		}
		if err := client.cancelQueueItem(int(queueID)); err != nil {
			return fmt.Errorf("cancel queueitem error:%v", err)
		}
	} else {
		buildInfo, err := client.getBuildInfo(jobName)
		if err != nil {
			return err
		}
		if buildInfo.Building {
			if err := client.stopJob(jobName, buildInfo.Number); err != nil {
				return err
			}
		}
	}
	return nil
}

func (j *Engine) SyncExecution(execution *v3.PipelineExecution) (bool, error) {
	updated := false

	jobName := getJobName(execution)
	client, err := j.getJenkinsClient(execution)
	if err != nil {
		return false, err
	}
	buildinfo, err := client.getBuildInfo(jobName)
	if e, ok := err.(*httperror.APIError); ok && e.Code.Status == http.StatusNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}

	if buildinfo.Result == "FAILURE" {
		//some errors are disclosed in buildinfo but not in wfbuildinfo
		execution.Status.ExecutionState = utils.StateFailed
		v3.PipelineExecutionConditionBuilt.False(execution)
		v3.PipelineExecutionConditionBuilt.Message(execution, "buildinfo result failure")
		updated = true
	}

	info, err := client.getWFBuildInfo(jobName)
	if e, ok := err.(*httperror.APIError); ok && e.Code.Status == http.StatusNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	for _, jenkinsStage := range info.Stages {
		//handle those in step-1-1 format
		if stepNameRe.MatchString(jenkinsStage.Name) {
			parts := strings.Split(jenkinsStage.Name, "-")
			stage, err := strconv.Atoi(parts[1])
			if err != nil {
				return false, err
			}
			step, err := strconv.Atoi(parts[2])
			if err != nil {
				return false, err
			}
			if len(execution.Status.Stages) <= stage || len(execution.Status.Stages[stage].Steps) <= step {
				return false, errors.New("error sync execution - index out of range")
			}
			status := jenkinsStage.Status
			if status == "SUCCESS" && execution.Status.Stages[stage].Steps[step].State != utils.StateSuccess {

				if jenkinsStage.DurationMillis < 100 {
					//For concurrent steps, there are cases Jenkins wfapi returns SUCCESS status while it it still IN_PROGRESS
					updated = true
					buildingStep(execution, stage, step, jenkinsStage)
					continue
				}
				updated = true
				if err := j.successStep(execution, stage, step, jenkinsStage); err != nil {
					return false, err
				}
			} else if (status == "FAILED" || status == "ABORTED") && execution.Status.Stages[stage].Steps[step].State != utils.StateFailed {
				updated = true
				if err := j.failStep(execution, stage, step, jenkinsStage); err != nil {
					return false, err
				}
			} else if status == "IN_PROGRESS" && execution.Status.Stages[stage].Steps[step].State != utils.StateBuilding {
				updated = true
				buildingStep(execution, stage, step, jenkinsStage)
			} else if status == "NOT_EXECUTED" && execution.Status.Stages[stage].Steps[step].State != utils.StateSkipped {
				updated = true
				skipStep(execution, stage, step, jenkinsStage)
			}
		}
	}

	if info.Status == "SUCCESS" && execution.Status.ExecutionState != utils.StateSuccess {
		updated = true
		execution.Labels[utils.PipelineFinishLabel] = "true"
		execution.Status.ExecutionState = utils.StateSuccess

		v3.PipelineExecutionConditionProvisioned.True(execution)
		v3.PipelineExecutionConditionBuilt.True(execution)
	} else if info.Status == "FAILED" && execution.Status.ExecutionState != utils.StateAborted &&
		execution.Status.ExecutionState != utils.StateFailed {
		updated = true
		execution.Labels[utils.PipelineFinishLabel] = "true"
		execution.Status.ExecutionState = utils.StateFailed
		if v3.PipelineExecutionConditionProvisioned.IsUnknown(execution) {
			v3.PipelineExecutionConditionProvisioned.True(execution)
		}
		v3.PipelineExecutionConditionBuilt.False(execution)
		v3.PipelineExecutionConditionBuilt.Message(execution, "Buildinfo got FAILED status")
	} else if info.Status == "IN_PROGRESS" && execution.Status.ExecutionState == utils.StateWaiting {
		updated = true
		execution.Status.ExecutionState = utils.StateBuilding
	}

	if execution.Status.ExecutionState == utils.StateBuilding {
		if len(execution.Status.Stages) > 0 &&
			len(execution.Status.Stages[0].Steps) > 0 &&
			execution.Status.Stages[0].Steps[0].State == utils.StateWaiting {
			//update ProvisionCondition
			prepareLog, err := client.getWFNodeLog(jobName, PrepareWFNodeID)
			if e, ok := err.(*httperror.APIError); ok && e.Code.Status == http.StatusNotFound {
				return false, nil
			} else if err != nil {
				return false, err
			}
			prevMessage := v3.PipelineExecutionConditionProvisioned.GetMessage(execution)
			curMessage := translatePreparingMessage(prepareLog.Text)
			if prevMessage != curMessage && curMessage != "" {
				v3.PipelineExecutionConditionProvisioned.Message(execution, curMessage)
				updated = true
			}
		} else {
			v3.PipelineExecutionConditionProvisioned.True(execution)
		}
	}

	return updated, nil
}

func (j *Engine) successStep(execution *v3.PipelineExecution, stage int, step int, jenkinsStage Stage) error {
	startTime := time.Unix(jenkinsStage.StartTimeMillis/1000, 0).Format(time.RFC3339)
	endTime := time.Unix((jenkinsStage.StartTimeMillis+jenkinsStage.DurationMillis)/1000, 0).Format(time.RFC3339)
	execution.Status.Stages[stage].Steps[step].State = utils.StateSuccess
	if execution.Status.Stages[stage].Steps[step].Started == "" {
		execution.Status.Stages[stage].Steps[step].Started = startTime
	}
	execution.Status.Stages[stage].Steps[step].Ended = endTime
	if execution.Status.Stages[stage].Started == "" {
		execution.Status.Stages[stage].Started = startTime
	}
	if execution.Status.Started == "" {
		execution.Status.Started = startTime
	}
	if utils.IsStageSuccess(execution.Status.Stages[stage]) {
		execution.Status.Stages[stage].State = utils.StateSuccess
		execution.Status.Stages[stage].Ended = endTime
		if stage == len(execution.Status.Stages)-1 {
			execution.Status.ExecutionState = utils.StateSuccess
			execution.Status.Ended = endTime
			execution.Labels[utils.PipelineFinishLabel] = "true"
			v3.PipelineExecutionConditionBuilt.True(execution)
		}
	}

	if err := j.saveStepLogToMinio(execution, stage, step); err != nil {
		return err
	}
	return nil
}

func (j *Engine) failStep(execution *v3.PipelineExecution, stage int, step int, jenkinsStage Stage) error {
	startTime := time.Unix(jenkinsStage.StartTimeMillis/1000, 0).Format(time.RFC3339)
	endTime := time.Unix((jenkinsStage.StartTimeMillis+jenkinsStage.DurationMillis)/1000, 0).Format(time.RFC3339)
	execution.Status.Stages[stage].Steps[step].State = utils.StateFailed
	execution.Status.Stages[stage].State = utils.StateFailed
	if execution.Status.ExecutionState != utils.StateAborted {
		execution.Status.ExecutionState = utils.StateFailed
		v3.PipelineExecutionConditionBuilt.False(execution)
		v3.PipelineExecutionConditionBuilt.Message(execution, fmt.Sprintf("Got FAILED status in '%s' stage", execution.Spec.PipelineConfig.Stages[stage].Name))
	}
	if execution.Status.Stages[stage].Steps[step].Started == "" {
		execution.Status.Stages[stage].Steps[step].Started = startTime
	}
	execution.Status.Stages[stage].Steps[step].Ended = endTime
	if execution.Status.Stages[stage].Started == "" {
		execution.Status.Stages[stage].Started = startTime
	}
	if execution.Status.Stages[stage].Ended == "" {
		execution.Status.Stages[stage].Ended = endTime
	}
	if execution.Status.Started == "" {
		execution.Status.Started = startTime
	}
	if execution.Status.Ended == "" {
		execution.Status.Ended = endTime
	}
	if err := j.saveStepLogToMinio(execution, stage, step); err != nil {
		return err
	}

	//clean waiting status of other stages/steps
	for i := 0; i < len(execution.Status.Stages); i++ {
		stage := &execution.Status.Stages[i]
		if stage.State == utils.StateWaiting {
			stage.State = ""
		}
		stepsize := len(execution.Status.Stages[i].Steps)
		for j := 0; j < stepsize; j++ {
			step := &stage.Steps[j]
			if step.State == utils.StateWaiting {
				step.State = ""
			}
		}
	}
	//abort concurrent building steps
	for i := 0; i < len(execution.Status.Stages[stage].Steps); i++ {
		if execution.Status.Stages[stage].Steps[i].State == utils.StateBuilding {
			execution.Status.Stages[stage].Steps[i].State = utils.StateAborted
			execution.Status.Stages[stage].Steps[i].Ended = endTime
			if err := j.saveStepLogToMinio(execution, stage, i); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildingStep(execution *v3.PipelineExecution, stage int, step int, jenkinsStage Stage) {
	startTime := time.Unix(jenkinsStage.StartTimeMillis/1000, 0).Format(time.RFC3339)
	execution.Status.Stages[stage].Steps[step].State = utils.StateBuilding
	if execution.Status.Stages[stage].Steps[step].Started == "" {
		execution.Status.Stages[stage].Steps[step].Started = startTime
	}
	if execution.Status.Stages[stage].State == utils.StateWaiting {
		execution.Status.Stages[stage].State = utils.StateBuilding
	}
	if execution.Status.Stages[stage].Started == "" {
		execution.Status.Stages[stage].Started = startTime
	}
	if execution.Status.ExecutionState == utils.StateWaiting {
		execution.Status.ExecutionState = utils.StateBuilding
	}
	if execution.Status.Started == "" {
		execution.Status.Started = startTime
	}

	stageName := execution.Spec.PipelineConfig.Stages[stage].Name
	message := fmt.Sprintf("Running '%s' stage", stageName)
	v3.PipelineExecutionConditionBuilt.CreateUnknownIfNotExists(execution)
	v3.PipelineExecutionConditionBuilt.Message(execution, message)
}

func skipStep(execution *v3.PipelineExecution, stage int, step int, jenkinsStage Stage) {
	endTime := time.Unix((jenkinsStage.StartTimeMillis+jenkinsStage.DurationMillis)/1000, 0).Format(time.RFC3339)
	execution.Status.Stages[stage].Steps[step].State = utils.StateSkipped

	curStage := execution.Status.Stages[stage]
	skipStage := true
	for _, curStep := range curStage.Steps {
		if curStep.State != utils.StateSkipped {
			skipStage = false
		}
	}

	if skipStage {
		execution.Status.Stages[stage].State = utils.StateSkipped
	} else if utils.IsStageSuccess(execution.Status.Stages[stage]) {
		execution.Status.Stages[stage].State = utils.StateSuccess
		execution.Status.Stages[stage].Ended = endTime

	}

	if stage == len(execution.Status.Stages)-1 &&
		(execution.Status.Stages[stage].State == utils.StateSkipped || execution.Status.Stages[stage].State == utils.StateSuccess) {
		execution.Status.ExecutionState = utils.StateSuccess
		execution.Status.Ended = endTime
		execution.Labels[utils.PipelineFinishLabel] = "true"
		v3.PipelineExecutionConditionBuilt.True(execution)
	}
}

func (j Engine) GetStepLog(execution *v3.PipelineExecution, stage int, step int) (string, error) {
	if len(execution.Status.Stages) <= stage || len(execution.Status.Stages[stage].Steps) <= step {
		return "", errors.New("invalid step index")
	}
	curStep := execution.Status.Stages[stage].Steps[step]
	if curStep.State == utils.StateWaiting {
		return "", nil
	} else if curStep.State != utils.StateBuilding {
		return j.getStepLogFromMinioStore(execution, stage, step)
	}
	return j.getStepLogFromJenkins(execution, stage, step)
}

func (j Engine) getStepLogFromJenkins(execution *v3.PipelineExecution, stage int, step int) (string, error) {
	if len(execution.Status.Stages) <= stage || len(execution.Status.Stages[stage].Steps) <= step {
		return "", errors.New("invalid step index")
	}
	jobName := getJobName(execution)
	client, err := j.getJenkinsClient(execution)
	if err != nil {
		return "", err
	}
	info, err := client.getWFBuildInfo(jobName)
	if err != nil {
		return "", err
	}
	WFnodeID := ""
	for _, jStage := range info.Stages {
		if jStage.Name == fmt.Sprintf("step-%d-%d", stage, step) {
			WFnodeID = jStage.ID
			break
		}
	}
	if WFnodeID == "" {
		return "", errors.New("Error WF Node for the step not found")
	}
	WFnodeInfo, err := client.getWFNodeInfo(jobName, WFnodeID)
	if err != nil {
		return "", err
	}
	if len(WFnodeInfo.StageFlowNodes) < 1 {
		return "", errors.New("Error step Node not found")
	}
	logNodeID := WFnodeInfo.StageFlowNodes[0].ID
	logrus.Debugf("trying getWFNodeLog, %v, %v", jobName, logNodeID)
	nodeLog, err := client.getWFNodeLog(jobName, logNodeID)
	if err != nil {
		return "", err
	}

	return nodeLog.Text, nil
}

func getJobName(execution *v3.PipelineExecution) string {
	if execution == nil {
		return ""
	}
	return fmt.Sprintf("%s%s", JenkinsJobPrefix, execution.Name)
}

func (j Engine) setCredential(client *Client, execution *v3.PipelineExecution, credentialID string) error {
	if credentialID == "" {
		return nil
	}
	ns, name := ref.Parse(credentialID)
	credential, err := j.SourceCodeCredentialLister.Get(ns, name)
	if err != nil {
		return err
	}
	err = client.getCredential(credentialID)
	if e, ok := err.(*httperror.APIError); !ok || e.Code.Status != http.StatusNotFound {
		return err
	}
	//set credential when it is not exist
	jenkinsCred := &Credential{}
	jenkinsCred.Class = "com.cloudbees.plugins.credentials.impl.UsernamePasswordCredentialsImpl"
	jenkinsCred.Scope = "GLOBAL"
	jenkinsCred.ID = execution.Name

	_, projID := ref.Parse(execution.Spec.ProjectName)
	scpConfig, err := providers.GetSourceCodeProviderConfig(credential.Spec.SourceCodeType, projID)
	if err != nil {
		return err
	}
	remote, err := remote.New(scpConfig)
	if err != nil {
		return err
	}

	jenkinsCred.Username = credential.Spec.GitLoginName
	jenkinsCred.Password = credential.Spec.AccessToken
	if credential.Spec.GitCloneToken != "" {
		jenkinsCred.Password = credential.Spec.GitCloneToken
	}
	if accessToken, err := utils.EnsureAccessToken(j.SourceCodeCredentials, remote, credential); err != nil {
		return err
	} else if accessToken != credential.Spec.AccessToken {
		jenkinsCred.Password = accessToken
	}

	bodyContent := map[string]interface{}{}
	bodyContent["credentials"] = jenkinsCred
	b, err := json.Marshal(bodyContent)
	if err != nil {
		return err
	}
	buff := bytes.NewBufferString("json=")
	buff.Write(b)
	return client.createCredential(buff.Bytes())
}

func translatePreparingMessage(log string) string {
	log = strings.TrimRight(log, "\n")
	lines := strings.Split(log, "\n")

	message := lines[len(lines)-1]
	if message == "" {
		message = "Setting up executors"
	} else if strings.Contains(message, " offline") {
		message = "Waiting for executors to be ready"
	} else if strings.Contains(message, "Running") {
		message = stripTags(message)
	} else {
		//unchanged
		message = ""
	}
	return message
}

func stripTags(content string) string {
	re, _ := regexp.Compile("\\<[\\S\\s]+?\\>")
	stripped := re.ReplaceAllString(content, "")
	return stripped
}
