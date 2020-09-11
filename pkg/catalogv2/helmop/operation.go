package helmop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rancher/apiserver/pkg/types"
	types2 "github.com/rancher/rancher/pkg/api/steve/catalog/types"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2/content"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/steve/pkg/podimpersonation"
	"github.com/rancher/steve/pkg/stores/proxy"
	"github.com/rancher/wrangler/pkg/data/convert"
	corev1controllers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

const (
	helmDataPath = "/home/shell/helm"
)

var (
	badChars = regexp.MustCompile("[^-.0-9a-zA-Z]")
)

type Operations struct {
	namespace      string
	contentManager *content.Manager
	Impersonator   *podimpersonation.PodImpersonation
	clusterRepos   catalogcontrollers.ClusterRepoClient
	ops            catalogcontrollers.OperationClient
	pods           corev1controllers.PodClient
	releases       catalogcontrollers.ReleaseClient
	cg             proxy.ClientGetter
}

func NewOperations(
	cg proxy.ClientGetter,
	catalog catalogcontrollers.Interface,
	contentManager *content.Manager,
	pods corev1controllers.PodClient) *Operations {
	return &Operations{
		cg:             cg,
		contentManager: contentManager,
		namespace:      namespaces.System,
		Impersonator:   podimpersonation.New("helm-op", cg, time.Hour, settings.FullShellImage),
		pods:           pods,
		clusterRepos:   catalog.ClusterRepo(),
		ops:            catalog.Operation(),
		releases:       catalog.Release(),
	}
}

func (s *Operations) Uninstall(ctx context.Context, user user.Info, namespace, name string, options io.Reader) (*catalog.Operation, error) {
	status, cmds, err := s.getUninstallArgs(namespace, name, options)
	if err != nil {
		return nil, err
	}

	return s.createOperation(ctx, user, status, cmds)
}

func (s *Operations) Rollback(ctx context.Context, user user.Info, namespace, name string, options io.Reader) (*catalog.Operation, error) {
	status, cmds, err := s.getRollbackArgs(namespace, name, options)
	if err != nil {
		return nil, err
	}

	return s.createOperation(ctx, user, status, cmds)
}

func (s *Operations) Upgrade(ctx context.Context, user user.Info, namespace, name string, options io.Reader) (*catalog.Operation, error) {
	status, cmds, err := s.getUpgradeCommand(namespace, name, options)
	if err != nil {
		return nil, err
	}

	user, err = s.getUser(user, namespace, name)
	if err != nil {
		return nil, err
	}

	return s.createOperation(ctx, user, status, cmds)
}

func (s *Operations) Install(ctx context.Context, user user.Info, namespace, name string, options io.Reader) (*catalog.Operation, error) {
	status, cmds, err := s.getInstallCommand(namespace, name, options)
	if err != nil {
		return nil, err
	}

	user, err = s.getUser(user, namespace, name)
	if err != nil {
		return nil, err
	}

	return s.createOperation(ctx, user, status, cmds)
}

func decodeParams(req *http.Request, target runtime.Object) error {
	return scheme.ParameterCodec.DecodeParameters(req.URL.Query(), corev1.SchemeGroupVersion, target)
}

func (s *Operations) proxyLogRequest(rw http.ResponseWriter, req *http.Request, pod *v1.Pod, client kubernetes.Interface) error {
	logOptions := &v1.PodLogOptions{}
	if err := decodeParams(req, logOptions); err != nil {
		return err
	}

	logOptions.Container = "helm"
	logURL := client.CoreV1().RESTClient().
		Get().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("log").
		VersionedParams(logOptions, scheme.ParameterCodec).URL()

	httpClient := client.CoreV1().RESTClient().(*rest.RESTClient).Client
	p := httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = logURL
			req.Host = logURL.Host
			delete(req.Header, "Authorization")
			delete(req.Header, "Cookie")
		},
		Transport:     httpClient.Transport,
		FlushInterval: time.Millisecond * 100,
	}

	p.ServeHTTP(rw, req)
	return nil
}

func (s *Operations) Log(rw http.ResponseWriter, req *http.Request, namespace, name string) error {
	op, err := s.ops.Get(namespace, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	pod, err := s.pods.Get(op.Status.PodNamespace, op.Status.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if len(pod.OwnerReferences) == 0 || len(op.OwnerReferences) == 0 || pod.OwnerReferences[0].UID != op.OwnerReferences[0].UID {
		return validation.NotFound
	}

	if pod.Labels[podimpersonation.TokenLabel] != op.Status.Token {
		return validation.NotFound
	}

	client, err := s.cg.AdminK8sInterface()
	if err != nil {
		return err
	}

	return s.proxyLogRequest(rw, req, pod, client)
}

func (s *Operations) getSpec(namespace, name string) (*catalog.RepoSpec, error) {
	if namespace == "" {
		clusterRepo, err := s.clusterRepos.Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &clusterRepo.Spec, nil
	}

	panic("namespace should not be empty")
}

func (s *Operations) getUser(userInfo user.Info, namespace, name string) (user.Info, error) {
	repoSpec, err := s.getSpec(namespace, name)
	if err != nil {
		return nil, err
	}

	if repoSpec.ServiceAccount == "" {
		return userInfo, nil
	}
	serviceAccountNS := repoSpec.ServiceAccountNamespace
	if namespace != "" {
		serviceAccountNS = namespace
	}
	if serviceAccountNS == "" || strings.Contains(repoSpec.ServiceAccountNamespace, ":") {
		return userInfo, nil
	}
	return &user.DefaultInfo{
		Name: fmt.Sprintf("system:serviceaccount:%s:%s", repoSpec.ServiceAccount, serviceAccountNS),
		Groups: []string{
			"system:serviceaccounts",
			"system:serviceaccounts:" + serviceAccountNS,
		},
	}, nil
}

func (s *Operations) getUninstallArgs(releaseNamespace, releaseName string, body io.Reader) (catalog.OperationStatus, Commands, error) {
	rel, err := s.releases.Get(releaseNamespace, releaseName, metav1.GetOptions{})
	if err != nil {
		return catalog.OperationStatus{}, nil, err
	}

	uninstallArgs := &types2.ChartUninstallAction{}
	if err := json.NewDecoder(body).Decode(uninstallArgs); err != nil {
		return catalog.OperationStatus{}, nil, err
	}

	cmd := Command{
		Operation: "uninstall",
		ArgObjects: []interface{}{
			uninstallArgs,
		},
		ReleaseName:      rel.Spec.Name,
		ReleaseNamespace: rel.Namespace,
	}

	status := catalog.OperationStatus{
		Action:    cmd.Operation,
		Release:   rel.Spec.Name,
		Namespace: releaseNamespace,
	}

	return status, Commands{cmd}, nil
}

func (s *Operations) getRollbackArgs(releaseNamespace, releaseName string, body io.Reader) (catalog.OperationStatus, Commands, error) {
	rel, err := s.releases.Get(releaseNamespace, releaseName, metav1.GetOptions{})
	if err != nil {
		return catalog.OperationStatus{}, nil, err
	}

	rollbackArgs := &types2.ChartRollbackAction{}
	if err := json.NewDecoder(body).Decode(rollbackArgs); err != nil {
		return catalog.OperationStatus{}, nil, err
	}

	cmd := Command{
		Operation: "rollback",
		ArgObjects: []interface{}{
			rollbackArgs,
		},
		ReleaseName:      rel.Spec.Name,
		ReleaseNamespace: rel.Namespace,
	}
	status := catalog.OperationStatus{
		Action:    "rollback",
		Release:   rel.Spec.Name,
		Namespace: releaseNamespace,
	}

	return status, Commands{cmd}, err
}

func (s *Operations) getUpgradeCommand(repoNamespace, repoName string, body io.Reader) (catalog.OperationStatus, Commands, error) {
	var (
		upgradeArgs = &types2.ChartUpgradeAction{}
		commands    Commands
	)
	err := json.NewDecoder(body).Decode(upgradeArgs)
	if err != nil {
		return catalog.OperationStatus{}, nil, err
	}

	status := catalog.OperationStatus{
		Action: "upgrade",
	}

	for _, chartUpgrade := range upgradeArgs.Charts {
		cmd, err := s.getChartCommand(repoNamespace, repoName, chartUpgrade.ChartName, chartUpgrade.Version, chartUpgrade.Values)
		if err != nil {
			return status, nil, err
		}
		cmd.ReleaseName = chartUpgrade.ReleaseName
		cmd.Operation = "upgrade"
		cmd.ArgObjects = []interface{}{
			chartUpgrade,
			upgradeArgs,
		}

		status.Release = chartUpgrade.ReleaseName
		status.Namespace = namespace(chartUpgrade.Namespace)

		commands = append(commands, cmd)
	}

	return status, commands, nil
}

type Command struct {
	Operation        string
	ArgObjects       []interface{}
	ValuesFile       string
	Values           []byte
	ChartFile        string
	Chart            []byte
	ReleaseName      string
	ReleaseNamespace string
}

type Commands []Command

func (c Commands) CommandArgs() ([]string, error) {
	var (
		result []string
	)
	for _, c := range c {
		args, err := c.renderArgs()
		if err != nil {
			return nil, err
		}
		if len(result) > 0 {
			result = append(result, ";")
		}
		result = append(result, "helm")
		result = append(result, args...)
	}
	return result, nil
}

func (c Commands) Render() (map[string][]byte, error) {
	data := map[string][]byte{}
	for i, cmd := range c {
		cmdData, err := cmd.Render(i)
		if err != nil {
			return nil, err
		}
		for k, v := range cmdData {
			data[k] = v
		}
	}

	return data, nil
}

func (c Command) Render(index int) (map[string][]byte, error) {
	args, err := c.renderArgs()
	if err != nil {
		return nil, err
	}

	data := map[string][]byte{
		fmt.Sprintf("operation%03d", index): []byte(strings.Join(args, "\x00")),
	}
	if len(c.ValuesFile) > 0 {
		data[c.ValuesFile] = c.Values
	}
	if len(c.ChartFile) > 0 {
		data[c.ChartFile] = c.Chart
	}
	return data, nil
}

func (c Command) renderArgs() ([]string, error) {
	var (
		args []string
	)

	dataMap := map[string]interface{}{}
	for _, argObject := range c.ArgObjects {
		data, err := convert.EncodeToMap(argObject)
		if err != nil {
			return nil, err
		}
		for k, v := range data {
			dataMap[k] = v
		}
	}

	delete(dataMap, "values")
	delete(dataMap, "charts")
	delete(dataMap, "releaseName")
	delete(dataMap, "chartName")
	if v, ok := dataMap["disableOpenAPIValidation"]; ok {
		delete(dataMap, "disableOpenAPIValidation")
		dataMap["disableOpenapiValidation"] = v
	}
	if c.Operation == "install" {
		dataMap["createNamespace"] = true
	}

	for k, v := range dataMap {
		s := convert.ToString(v)
		k = convert.ToArgKey(k)
		// This is a possibly unneeded check, but we want to ensure the strings have no null bytes so
		// running the xargs -0 works.
		if !utf8.ValidString(s) || !utf8.ValidString(k) {
			return nil, fmt.Errorf("invalid non-utf8 string")
		}
		args = append(args, fmt.Sprintf("%s=%s", k, s))
	}

	if len(c.Values) > 0 {
		args = append(args, "--values="+filepath.Join(helmDataPath, c.ValuesFile))
	}

	if c.ReleaseNamespace != "" {
		args = append(args, "--namespace="+c.ReleaseNamespace)
	}

	sort.Strings(args)
	if c.ReleaseName != "" {
		args = append(args, c.ReleaseName)
	}
	if len(c.Chart) > 0 {
		args = append(args, filepath.Join(helmDataPath, c.ChartFile))
	}

	return append([]string{"--debug", c.Operation}, args...), nil
}

func sanitizeVersion(chartVersion string) string {
	return badChars.ReplaceAllString(chartVersion, "-")
}

func (s *Operations) getChartCommand(namespace, name, chartName, chartVersion string, values map[string]interface{}) (Command, error) {
	chart, err := s.contentManager.Chart(namespace, name, chartName, chartVersion)
	if err != nil {
		return Command{}, err
	}
	chartData, err := ioutil.ReadAll(chart)
	chart.Close()
	if err != nil {
		return Command{}, err
	}

	c := Command{
		ValuesFile: fmt.Sprintf("values-%s-%s.yaml", chartName, sanitizeVersion(chartVersion)),
		ChartFile:  fmt.Sprintf("%s-%s.tgz", chartName, sanitizeVersion(chartVersion)),
		Chart:      chartData,
	}

	if len(values) > 0 {
		c.Values, err = json.Marshal(values)
		if err != nil {
			return Command{}, err
		}
	}

	return c, nil
}

func (s *Operations) getInstallCommand(repoNamespace, repoName string, body io.Reader) (catalog.OperationStatus, Commands, error) {
	installArgs := &types2.ChartInstallAction{}
	err := json.NewDecoder(body).Decode(installArgs)
	if err != nil {
		return catalog.OperationStatus{}, nil, err
	}

	var (
		cmds   []Command
		status = catalog.OperationStatus{
			Action: "install",
		}
	)

	for _, chartInstall := range installArgs.Charts {
		var suffix []string
		if chartInstall.ReleaseName == "" {
			chartInstall.GenerateName = true
		} else {
			suffix = append(suffix, chartInstall.ReleaseName)
		}

		cmd, err := s.getChartCommand(repoNamespace, repoName, chartInstall.ChartName, chartInstall.Version, chartInstall.Values)
		if err != nil {
			return status, nil, err
		}
		cmd.Operation = "install"
		cmd.ArgObjects = []interface{}{
			chartInstall,
			installArgs,
		}
		cmd.ReleaseName = chartInstall.ReleaseName

		status.Release = chartInstall.ReleaseName
		status.Namespace = namespace(chartInstall.Namespace)

		cmds = append(cmds, cmd)
	}

	return status, cmds, err
}

func namespace(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}

func (s *Operations) createOperation(ctx context.Context, user user.Info, status catalog.OperationStatus, cmds Commands) (*catalog.Operation, error) {
	if status.Action != "uninstall" {
		_, err := s.createNamespace(ctx, status.Namespace)
		if err != nil {
			return nil, err
		}
	}

	secretData, err := cmds.Render()
	if err != nil {
		return nil, err
	}

	pod, podOptions := s.createPod(secretData)
	pod, err = s.Impersonator.CreatePod(ctx, user, pod, podOptions)
	if err != nil {
		return nil, err
	}

	status.Command, err = cmds.CommandArgs()
	if err != nil {
		return nil, err
	}

	status.Token = pod.Labels[podimpersonation.TokenLabel]
	status.PodName = pod.Name
	status.PodNamespace = pod.Namespace

	op := &catalog.Operation{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pod.Name,
			Namespace:       status.Namespace,
			OwnerReferences: pod.OwnerReferences,
		},
	}
	op, err = s.ops.Create(op)
	if err != nil {
		return nil, err
	}

	op.Status = status
	return s.ops.UpdateStatus(op)
}

func (s *Operations) createNamespace(ctx context.Context, namespace string) (*v1.Namespace, error) {
	apiContext := types.GetAPIContext(ctx)
	client, err := s.cg.K8sInterface(apiContext)
	if err != nil {
		return nil, err
	}

	ns, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return client.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}, metav1.CreateOptions{})
	}
	return ns, err
}

func (s *Operations) createPod(secretData map[string][]byte) (*v1.Pod, *podimpersonation.PodOptions) {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "helm-operation-",
			Namespace:    s.namespace,
		},
		Data: secretData,
	}
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "helm-operation-",
			Namespace:    s.namespace,
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{
					Name: "data",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "helm-operation-",
						},
					},
				},
			},
			TerminationGracePeriodSeconds: new(int64),
			RestartPolicy:                 v1.RestartPolicyNever,
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
			},
			Tolerations: []v1.Toleration{
				{
					Key:      "cattle.io/os",
					Operator: "Equal",
					Value:    "linux",
					Effect:   "NoSchedule",
				},
			},
			Containers: []v1.Container{
				{
					Name: "helm",
					Env: []v1.EnvVar{
						{
							Name:  "KUBECONFIG",
							Value: "/home/shell/.kube/config",
						},
					},
					Stdin:           true,
					TTY:             true,
					StdinOnce:       true,
					Image:           settings.FullShellImage(),
					ImagePullPolicy: v1.PullIfNotPresent,
					Command:         []string{"helm-cmd"},
					WorkingDir:      helmDataPath,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "data",
							MountPath: helmDataPath,
						},
					},
				},
			},
		},
	}

	return pod, &podimpersonation.PodOptions{
		SecretsToCreate: []*v1.Secret{
			secret,
		},
	}
}
