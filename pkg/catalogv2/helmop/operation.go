package helmop

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalogv2/content"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/steve/pkg/podimpersonation"
	"github.com/rancher/steve/pkg/stores/proxy"
	data2 "github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	corev1controllers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

const (
	helmDataPath = "/home/shell/helm"
)

var (
	badChars  = regexp.MustCompile("[^-.0-9a-zA-Z]")
	thirty    = int64(30)
	chartYAML = map[string]bool{
		"chart.yaml": true,
		"Chart.yaml": true,
		"chart.yml":  true,
		"Chart.yml":  true,
	}
)

type Operations struct {
	namespace      string
	contentManager *content.Manager
	Impersonator   *podimpersonation.PodImpersonation
	clusterRepos   catalogcontrollers.ClusterRepoClient
	ops            catalogcontrollers.OperationClient
	pods           corev1controllers.PodClient
	apps           catalogcontrollers.AppClient
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
		apps:           catalog.App(),
	}
}

func (s *Operations) Uninstall(ctx context.Context, user user.Info, namespace, name string, options io.Reader) (*catalog.Operation, error) {
	status, cmds, err := s.getUninstallArgs(namespace, name, options)
	if err != nil {
		return nil, err
	}

	user, err = s.getUser(user, namespace, name, true)
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

	user, err = s.getUser(user, namespace, name, false)
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

	user, err = s.getUser(user, namespace, name, false)
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
			delete(req.Header, "Impersonate-Group")
			delete(req.Header, "Impersonate-User")
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

func (s *Operations) getSpec(namespace, name string, isApp bool) (*catalog.RepoSpec, error) {
	if isApp {
		rel, err := s.apps.Get(namespace, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		if rel.Spec.Chart != nil && rel.Spec.Chart.Metadata != nil {
			isClusterRepo := rel.Spec.Chart.Metadata.Annotations["catalog.cattle.io/ui-source-repo-type"]
			if isClusterRepo != "cluster" {
				return &catalog.RepoSpec{}, nil
			}
			clusterRepoName := rel.Spec.Chart.Metadata.Annotations["catalog.cattle.io/ui-source-repo"]
			clusterRepo, err := s.clusterRepos.Get(clusterRepoName, metav1.GetOptions{})
			if err != nil {
				// don't report error if annotation doesn't exist
				return &catalog.RepoSpec{}, nil
			}
			return &clusterRepo.Spec, nil
		}
		return &catalog.RepoSpec{}, nil
	}
	if namespace == "" {
		clusterRepo, err := s.clusterRepos.Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &clusterRepo.Spec, nil
	}

	panic("namespace should not be empty")
}

func (s *Operations) getUser(userInfo user.Info, namespace, name string, isApp bool) (user.Info, error) {
	repoSpec, err := s.getSpec(namespace, name, isApp)
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
		Name: fmt.Sprintf("system:serviceaccount:%s:%s", serviceAccountNS, repoSpec.ServiceAccount),
		Groups: []string{
			"system:serviceaccounts",
			"system:serviceaccounts:" + serviceAccountNS,
		},
	}, nil
}

func (s *Operations) getUninstallArgs(appNamespace, appName string, body io.Reader) (catalog.OperationStatus, Commands, error) {
	rel, err := s.apps.Get(appNamespace, appName, metav1.GetOptions{})
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
		Namespace: appNamespace,
	}

	return status, Commands{cmd}, nil
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

	if upgradeArgs.MaxHistory == 0 {
		upgradeArgs.MaxHistory = 5
	}
	upgradeArgs.Install = true

	status := catalog.OperationStatus{
		Action:    "upgrade",
		Namespace: namespace(upgradeArgs.Namespace),
	}

	for _, chartUpgrade := range upgradeArgs.Charts {
		cmd, err := s.getChartCommand(repoNamespace, repoName, chartUpgrade.ChartName, chartUpgrade.Version, chartUpgrade.Annotations, chartUpgrade.Values)
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

	delete(dataMap, "annotations")
	delete(dataMap, "values")
	delete(dataMap, "charts")
	delete(dataMap, "releaseName")
	delete(dataMap, "chartName")
	delete(dataMap, "projectId")
	if v, ok := dataMap["disableOpenAPIValidation"]; ok {
		delete(dataMap, "disableOpenAPIValidation")
		dataMap["disableOpenapiValidation"] = v
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

	return append([]string{c.Operation}, args...), nil
}

func sanitizeVersion(chartVersion string) string {
	return badChars.ReplaceAllString(chartVersion, "-")
}

func injectAnnotation(data []byte, annotations map[string]string) ([]byte, error) {
	if len(annotations) == 0 {
		return data, nil
	}

	tgz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	var (
		dest    = &bytes.Buffer{}
		destGz  = gzip.NewWriter(dest)
		destTar = tar.NewWriter(destGz)
		tar     = tar.NewReader(tgz)
	)

	for {
		header, err := tar.Next()
		if err == io.EOF {
			break
		}

		data, err := ioutil.ReadAll(tar)
		if err != nil {
			return nil, err
		}

		parts := strings.Split(header.Name, "/")
		if len(parts) == 2 && chartYAML[parts[1]] {
			data, err = addAnnotations(data, annotations)
			if err != nil {
				return nil, err
			}
			header.Size = int64(len(data))
		}

		if err := destTar.WriteHeader(header); err != nil {
			return nil, err
		}

		_, err = destTar.Write(data)
		if err != nil {
			return nil, err
		}
	}

	if err = destTar.Close(); err != nil {
		return nil, err
	}

	if err = destGz.Close(); err != nil {
		return nil, err
	}

	return dest.Bytes(), nil
}

func addAnnotations(data []byte, annotations map[string]string) ([]byte, error) {
	chartData := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &chartData); err != nil {
		return nil, err
	}

	chartAnnotations := data2.Object(chartData).Map("annotations")
	if chartAnnotations == nil {
		chartAnnotations = map[string]interface{}{}
	}
	for k, v := range annotations {
		chartAnnotations[k] = v
	}

	chartData["annotations"] = chartAnnotations
	return yaml.Marshal(chartData)
}

func (s *Operations) getChartCommand(namespace, name, chartName, chartVersion string, annotations map[string]string, values map[string]interface{}) (Command, error) {
	chart, err := s.contentManager.Chart(namespace, name, chartName, chartVersion)
	if err != nil {
		return Command{}, err
	}
	chartData, err := ioutil.ReadAll(chart)
	chart.Close()
	if err != nil {
		return Command{}, err
	}

	chartData, err = injectAnnotation(chartData, annotations)
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
		cmd, err := s.getChartCommand(repoNamespace, repoName, chartInstall.ChartName, chartInstall.Version, chartInstall.Annotations, chartInstall.Values)
		if err != nil {
			return status, nil, err
		}
		cmd.Operation = "install"
		cmd.ArgObjects = []interface{}{
			chartInstall,
			installArgs,
		}
		cmd.ReleaseName = chartInstall.ReleaseName

		if len(installArgs.Charts) == 1 {
			if cmd.ReleaseName == "" {
				cmd.ArgObjects = append(cmd.ArgObjects, map[string]interface{}{
					"generateName": "true",
				})
			}
		} else {
			cmd.Operation = "upgrade"
			cmd.ArgObjects = append(cmd.ArgObjects, map[string]interface{}{
				"install": "true",
			})
		}

		status.Release = chartInstall.ReleaseName

		cmds = append(cmds, cmd)
	}

	status.Namespace = namespace(installArgs.Namespace)
	status.ProjectID = installArgs.ProjectID

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
		_, err := s.createNamespace(ctx, status.Namespace, status.ProjectID)
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

func (s *Operations) createNamespace(ctx context.Context, namespace, projectID string) (*v1.Namespace, error) {
	apiContext := types.GetAPIContext(ctx)
	client, err := s.cg.K8sInterface(apiContext)
	if err != nil {
		return nil, err
	}

	annotations := map[string]string{}
	if projectID != "" {
		annotations["field.cattle.io/projectId"] = strings.ReplaceAll(projectID, "/", ":")
	}
	// We just always try to create an ignore the error. This is because you might have create but not get privileges
	_, _ = client.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        namespace,
			Annotations: annotations,
		},
	}, metav1.CreateOptions{})

	if projectID == "" {
		return client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	}

	adminClient, err := s.cg.AdminK8sInterface()
	if err != nil {
		return nil, err
	}

	w, err := adminClient.CoreV1().Namespaces().Watch(ctx, metav1.ListOptions{
		FieldSelector:  "metadata.name=" + namespace,
		TimeoutSeconds: &thirty,
	})
	if err != nil {
		return nil, err
	}

	defer func() {
		w.Stop()
		// no clue if this needed, but I'm afraid there will be a stuck producer
		for range w.ResultChan() {
		}
	}()

	for event := range w.ResultChan() {
		if ns, ok := event.Object.(*v1.Namespace); ok {
			if ok, err := namespaces.IsNamespaceConditionSet(ns, string(v3.ProjectConditionInitialRolesPopulated), true); err != nil {
				return ns, err
			} else if ok {
				return ns, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to wait for roles to be populated")
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
