package logging

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/clustermanager"
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/configsyncer"
	"github.com/rancher/rancher/pkg/controllers/user/logging/deployer"
	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/core/v1"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	projectv3 "github.com/rancher/types/apis/project.cattle.io/v3"
	mgmtv3client "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config/dialer"

	"github.com/pkg/errors"
	k8scorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var (
	fluentdCommand = "fluentd -q --dry-run -i "
)

type Handler struct {
	clusterManager       *clustermanager.Manager
	dialerFactory        dialer.Factory
	k8sProxy             http.Handler
	testerDeployer       *deployer.TesterDeployer
	configGenerator      *configsyncer.ConfigGenerator
	projectLoggingLister mgmtv3.ProjectLoggingLister
}

func NewHandler(dialer dialer.Factory, appsGetter projectv3.AppsGetter, projectLister mgmtv3.ProjectLister, pods v1.PodInterface, projectLoggingLister mgmtv3.ProjectLoggingLister, namespaces v1.NamespaceInterface, templateLister mgmtv3.TemplateLister, clusterManager *clustermanager.Manager, k8sProxy http.Handler) *Handler {
	testerDeployer := deployer.NewTesterDeployer(appsGetter, projectLister, pods, projectLoggingLister, namespaces, templateLister)
	configGenerator := configsyncer.NewConfigGenerator(metav1.NamespaceAll, nil, projectLoggingLister, namespaces.Controller().Lister())

	return &Handler{
		clusterManager:       clusterManager,
		dialerFactory:        dialer,
		k8sProxy:             k8sProxy,
		testerDeployer:       testerDeployer,
		configGenerator:      configGenerator,
		projectLoggingLister: projectLoggingLister,
	}
}

func CollectionFormatter(apiContext *types.APIContext, resource *types.GenericCollection) {
	resource.AddAction(apiContext, "test")
	resource.AddAction(apiContext, "dryRun")
}

func (h *Handler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	var target mgmtv3.LoggingTargets
	var clusterName, projectName, level string

	switch apiContext.Type {
	case mgmtv3client.ClusterLoggingType:
		var input mgmtv3.ClusterTestInput
		actionInput, err := parse.ReadBody(apiContext.Request)
		if err != nil {
			return err
		}

		if err = convert.ToObj(actionInput, &input); err != nil {
			return err
		}

		target = input.LoggingTargets
		clusterName = input.ClusterName
		level = loggingconfig.ClusterLevel
	case mgmtv3client.ProjectLoggingType:
		var input mgmtv3.ProjectTestInput
		actionInput, err := parse.ReadBody(apiContext.Request)
		if err != nil {
			return err
		}

		if err = convert.ToObj(actionInput, &input); err != nil {
			return err
		}

		target = input.LoggingTargets
		projectName = input.ProjectName
		clusterName, _ = ref.Parse(input.ProjectName)
		level = loggingconfig.ProjectLevel
	}

	switch actionName {
	case "test":
		if err := h.testLoggingTarget(clusterName, target); err != nil {
			return httperror.NewAPIError(httperror.ServerError, err.Error())
		}

		apiContext.WriteResponse(http.StatusNoContent, nil)
		return nil

	case "dryRun":

		if err := h.dryRunLoggingTarget(apiContext, level, clusterName, projectName, target); err != nil {
			return httperror.NewAPIError(httperror.ServerError, err.Error())
		}

		apiContext.WriteResponse(http.StatusNoContent, nil)
		return nil

	}

	return httperror.NewAPIError(httperror.InvalidAction, "invalid action: "+actionName)

}

func (h *Handler) testLoggingTarget(clusterName string, target mgmtv3.LoggingTargets) error {
	clusterDialer, err := h.dialerFactory.ClusterDialer(clusterName)
	if err != nil {
		return errors.Wrap(err, "get cluster dialer failed")
	}

	wp := utils.NewLoggingTargetTestWrap(target)
	if wp == nil {
		return nil
	}

	return wp.TestReachable(clusterDialer)
}

func (h *Handler) dryRunLoggingTarget(apiContext *types.APIContext, level, clusterName, projectID string, target mgmtv3.LoggingTargets) error {
	var err error
	var dryRunConfigBuf []byte

	if level == loggingconfig.ClusterLevel {
		clusterLogging := &mgmtv3.ClusterLogging{
			Spec: mgmtv3.ClusterLoggingSpec{
				LoggingTargets: target,
				ClusterName:    clusterName,
			},
		}
		dryRunConfigBuf, err = h.configGenerator.GenerateClusterLoggingConfig(clusterLogging, "testSystemProjectID")
		if err != nil {
			return err
		}
	} else {
		curProjectLoggings, err := h.projectLoggingLister.List(metav1.NamespaceAll, labels.NewSelector())
		if err != nil {
			return errors.Wrap(err, "list project logging failed")
		}

		new := &mgmtv3.ProjectLogging{
			Spec: mgmtv3.ProjectLoggingSpec{
				LoggingTargets: target,
				ProjectName:    projectID,
			},
		}

		newProjectLoggings := append(curProjectLoggings, new)
		dryRunConfigBuf, err = h.configGenerator.GenerateProjectLoggingConfig(newProjectLoggings, "testSystemProjectID")
		if err != nil {
			return err
		}
	}

	if err := h.testerDeployer.Deploy(level, clusterName, projectID, target); err != nil {
		return err
	}

	context, err := h.clusterManager.UserContext(clusterName)
	if err != nil {
		return err
	}

	pods, err := context.K8sClient.CoreV1().Pods(loggingconfig.LoggingNamespace).List(metav1.ListOptions{
		LabelSelector: labels.Set(loggingconfig.FluentdTesterSelector).String(),
	})
	if err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return errors.New("could not find fluentd tester pod")
	}

	pod := pods.Items[0]
	if !condition.Cond(k8scorev1.PodReady).IsTrue(&pod) {
		return errors.New("pod fluentd tester not ready")
	}

	req := context.K8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&k8scorev1.PodExecOptions{
			Container: loggingconfig.FluentdTesterContainerName,
			Command:   []string{"bash"},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(&context.RESTConfig, "POST", req.URL())
	if err != nil {
		return errors.Wrap(err, "create executor failed")
	}
	var stdout, stderr, stdin bytes.Buffer

	dryRunConfig := strings.Replace(string(dryRunConfigBuf), `$`, `\$`, -1)
	stdin.WriteString(fluentdCommand)
	stdin.WriteString(fmt.Sprintf(`"%s"`, dryRunConfig))

	handler := &streamHandler{resizeEvent: make(chan remotecommand.TerminalSize)}
	if err = executor.Stream(remotecommand.StreamOptions{
		Stdin:             &stdin,
		Stdout:            &stdout,
		Stderr:            &stderr,
		TerminalSizeQueue: handler,
		Tty:               true,
	}); err != nil {
		err = errors.Wrapf(err, "stream error")
		if stderr.String() != "" {
			err = errors.WithMessage(err, "stderr: "+stderr.String())
		}

		if stdout.String() != "" {
			err = errors.WithMessage(err, "stdout: "+stdout.String())
		}
		return err
	}

	if stderr.String() != "" {
		return errors.New("dry run fluentd failed, stderr: " + stderr.String())
	}

	if stdout.String() != "" {
		return errors.New("dry run fluentd failed, stdout: " + stdout.String())
	}

	return nil
}

func (handler *streamHandler) Next() (size *remotecommand.TerminalSize) {
	ret := <-handler.resizeEvent
	size = &ret
	return
}

type streamHandler struct {
	resizeEvent chan remotecommand.TerminalSize
}
