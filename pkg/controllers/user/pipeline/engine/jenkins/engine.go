package jenkins

import (
	"encoding/xml"
	"fmt"

	"bytes"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Engine struct {
	Client *Client

	NodeLister    v1.NodeLister
	ServiceLister v1.ServiceLister

	Secrets                    v1.SecretInterface
	SecretLister               v1.SecretLister
	SourceCodeCredentialLister v3.SourceCodeCredentialLister
}

func (j *Engine) getJenkinsURL() (string, error) {
	//FIXME proper way to connect to Jenkins in cluster
	nodes, err := j.NodeLister.List("", labels.NewSelector())
	if err != nil {
		return "", err
	}
	if len(nodes) < 1 {
		return "", errors.New("no available nodes")
	}
	if len(nodes[0].Status.Addresses) < 1 {
		return "", errors.New("no available address")
	}
	host := nodes[0].Status.Addresses[0].Address

	svcport := 0
	service, err := j.ServiceLister.Get(utils.PipelineNamespace, "jenkins")
	if err != nil {
		return "", err
	}

	ports := service.Spec.Ports
	for _, port := range ports {
		if port.NodePort != 0 && port.Name == "http" {
			svcport = int(port.NodePort)
			break
		}
	}
	return fmt.Sprintf("http://%s:%d", host, svcport), nil
}

func (j *Engine) PreCheck() error {
	url, err := j.getJenkinsURL()
	if err != nil {
		return err
	}
	user := JenkinsDefaultUser
	secret, err := j.SecretLister.Get(utils.PipelineNamespace, "jenkins")
	if err != nil || secret.Data == nil {
		return fmt.Errorf("error get jenkins token - %v", err)
	}
	token := string(secret.Data["jenkins-admin-password"])

	client, err := New(url, user, token)
	if err != nil {
		return err
	}
	j.Client = client
	return nil
}

func (j *Engine) RunPipeline(pipeline *v3.Pipeline, triggerType string) error {

	jobName := getJobName(pipeline)

	if _, err := j.Client.getJobInfo(jobName); err == ErrNotFound {
		if err = j.createPipelineJob(pipeline); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := j.updatePipelineJob(pipeline); err != nil {
		return err
	}

	if err := j.setCredential(pipeline); err != nil {
		return err
	}

	if err := j.preparePipeline(pipeline); err != nil {
		return err
	}

	if _, err := j.Client.buildJob(jobName, map[string]string{}); err != nil {
		return err
	}
	return nil
}

func (j *Engine) preparePipeline(pipeline *v3.Pipeline) error {
	for n, stage := range pipeline.Spec.Stages {
		for m, step := range stage.Steps {
			if step.PublishImageConfig != nil {
				//prepare docker credential for publishimage step
				if err := j.prepareRegistryCredential(pipeline, n, m); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (j *Engine) prepareRegistryCredential(pipeline *v3.Pipeline, stage int, step int) error {

	publishImageStep := pipeline.Spec.Stages[stage].Steps[step]
	registry, _, _ := utils.SplitImageTag(publishImageStep.PublishImageConfig.Tag)

	secrets, err := j.SecretLister.List(pipeline.Namespace, labels.Everything())
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
				username = cred["username"].(string)
				password = cred["password"].(string)
			}

		}

	}

	//store dockercredential in pipeline namespace
	//TODO key-key mapping instead of registry-key mapping
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	proceccedRegistry := strings.ToLower(reg.ReplaceAllString(registry, ""))

	secretName := fmt.Sprintf("%s-%s", pipeline.Namespace, proceccedRegistry)
	logrus.Debugf("preparing registry credential %s for %s", secretName, registry)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.PipelineNamespace,
			Name:      secretName,
		},
		Data: map[string][]byte{
			"username": []byte(username),
			"password": []byte(password),
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

func (j *Engine) createPipelineJob(pipeline *v3.Pipeline) error {
	logrus.Debug("create jenkins job for pipeline")
	jobconf := ConvertPipelineToJenkinsPipeline(pipeline)

	jobName := getJobName(pipeline)
	bconf, _ := xml.MarshalIndent(jobconf, "  ", "    ")
	return j.Client.createJob(jobName, bconf)
}

func (j *Engine) updatePipelineJob(pipeline *v3.Pipeline) error {
	logrus.Debug("update jenkins job for pipeline")
	jobconf := ConvertPipelineToJenkinsPipeline(pipeline)

	jobName := getJobName(pipeline)
	bconf, _ := xml.MarshalIndent(jobconf, "  ", "    ")
	return j.Client.updateJob(jobName, bconf)
}

func (j *Engine) RerunExecution(execution *v3.PipelineExecution) error {

	return j.RunPipeline(&execution.Spec.Pipeline, utils.TriggerTypeUser)
}

func (j *Engine) StopExecution(execution *v3.PipelineExecution) error {

	jobName := getJobName(&execution.Spec.Pipeline)
	buildNumber := execution.Spec.Run
	info, err := j.Client.getJobInfo(jobName)
	if err == ErrNotFound {
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
		if err := j.Client.cancelQueueItem(int(queueID)); err != nil {
			return fmt.Errorf("cancel queueitem error:%v", err)
		}
	} else {
		buildInfo, err := j.Client.getBuildInfo(jobName)
		if err != nil {
			return err
		}
		if buildInfo.Building {
			if err := j.Client.stopJob(jobName, buildNumber); err != nil {
				return err
			}
		}
	}
	return nil
}

func (j *Engine) SyncExecution(execution *v3.PipelineExecution) (bool, error) {

	updated := false

	jobName := getJobName(&execution.Spec.Pipeline)
	if execution.Status.Commit == "" {
		buildinfo, err := j.Client.getBuildInfo(jobName)
		if err != nil {
			return false, err
		}
		for _, action := range buildinfo.Actions {
			if action.LastBuiltRevision.SHA1 != "" {
				execution.Status.Commit = action.LastBuiltRevision.SHA1
				updated = true
			}
		}
	}

	info, err := j.Client.getWFBuildInfo(jobName)
	if err != nil {
		return false, err
	}
	for _, jenkinsStage := range info.Stages {
		//handle those in step-1-1 format
		parts := strings.Split(jenkinsStage.Name, "-")
		if len(parts) == 3 {
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
				updated = true
				successStep(execution, stage, step, jenkinsStage)
			} else if status == "FAILED" && execution.Status.Stages[stage].Steps[step].State != utils.StateFail {
				updated = true
				failStep(execution, stage, step, jenkinsStage)
			} else if status == "IN_PROGRESS" && execution.Status.Stages[stage].Steps[step].State != utils.StateBuilding {
				updated = true
				buildingStep(execution, stage, step, jenkinsStage)
			}
		}
	}

	if info.Status == "SUCCESS" && execution.Status.ExecutionState != utils.StateSuccess {
		updated = true
		execution.Labels["pipeline.management.cattle.io/finish"] = "true"
		execution.Status.ExecutionState = utils.StateSuccess
	} else if info.Status == "FAILED" && execution.Status.ExecutionState != utils.StateFail {
		updated = true
		execution.Labels["pipeline.management.cattle.io/finish"] = "true"
		execution.Status.ExecutionState = utils.StateFail
	} else if info.Status == "IN_PROGRESS" && execution.Status.ExecutionState != utils.StateBuilding {
		updated = true
		execution.Status.ExecutionState = utils.StateBuilding
	}

	return updated, nil
}

func successStep(execution *v3.PipelineExecution, stage int, step int, jenkinsStage Stage) {

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
		}
	}
}

func failStep(execution *v3.PipelineExecution, stage int, step int, jenkinsStage Stage) {

	startTime := time.Unix(jenkinsStage.StartTimeMillis/1000, 0).Format(time.RFC3339)
	endTime := time.Unix((jenkinsStage.StartTimeMillis+jenkinsStage.DurationMillis)/1000, 0).Format(time.RFC3339)
	execution.Status.Stages[stage].Steps[step].State = utils.StateFail
	execution.Status.Stages[stage].State = utils.StateFail
	execution.Status.ExecutionState = utils.StateFail
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
}

func buildingStep(execution *v3.PipelineExecution, stage int, step int, jenkinsStage Stage) {
	startTime := time.Unix(jenkinsStage.StartTimeMillis/1000, 0).Format(time.RFC3339)
	execution.Status.Stages[stage].Steps[step].State = utils.StateBuilding
	if execution.Status.Stages[stage].Steps[step].Started == "" {
		execution.Status.Stages[stage].Steps[step].Started = startTime
	}
	if execution.Status.Stages[stage].Started == "" {
		execution.Status.Stages[stage].Started = startTime
	}
	if execution.Status.Started == "" {
		execution.Status.Started = startTime
	}
}

func (j Engine) GetStepLog(execution *v3.PipelineExecution, stage int, step int) (string, error) {

	jobName := getJobName(&execution.Spec.Pipeline)
	info, err := j.Client.getWFBuildInfo(jobName)
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
	WFnodeInfo, err := j.Client.getWFNodeInfo(jobName, WFnodeID)
	if err != nil {
		return "", err
	}
	if len(WFnodeInfo.StageFlowNodes) < 1 {
		return "", errors.New("Error step Node not found")
	}
	logNodeID := WFnodeInfo.StageFlowNodes[0].ID
	logrus.Debugf("trying getWFNodeLog, %v, %v", jobName, logNodeID)
	nodeLog, err := j.Client.getWFNodeLog(jobName, logNodeID)
	if err != nil {
		return "", err
	}

	return nodeLog.Text, nil
}

func getJobName(pipeline *v3.Pipeline) string {
	return fmt.Sprintf("%s%s-%d", JenkinsJobPrefix, pipeline.Name, pipeline.Status.NextRun)
}

func (j Engine) setCredential(pipeline *v3.Pipeline) error {
	if len(pipeline.Spec.Stages) < 1 || len(pipeline.Spec.Stages[0].Steps) < 1 || pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig == nil {
		return errors.New("Invalid pipeline definition")
	}
	credentialID := pipeline.Spec.Stages[0].Steps[0].SourceCodeConfig.SourceCodeCredentialName
	souceCodeCredential, err := j.SourceCodeCredentialLister.Get("", credentialID)
	if err != nil {
		return err
	}

	if err := j.Client.getCredential(credentialID); err != ErrNotFound {
		return err
	}
	//set credential when it is not exist
	jenkinsCred := &Credential{}
	jenkinsCred.Class = "com.cloudbees.plugins.credentials.impl.UsernamePasswordCredentialsImpl"
	jenkinsCred.Scope = "GLOBAL"
	jenkinsCred.ID = credentialID

	jenkinsCred.Username = souceCodeCredential.Spec.LoginName
	jenkinsCred.Password = souceCodeCredential.Spec.AccessToken

	bodyContent := map[string]interface{}{}
	bodyContent["credentials"] = jenkinsCred
	b, err := json.Marshal(bodyContent)
	if err != nil {
		return err
	}
	buff := bytes.NewBufferString("json=")
	buff.Write(b)
	return j.Client.createCredential(buff.Bytes())
}
