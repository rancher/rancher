/*
Package helmop implements handlers for managing helm operations.
*/
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
	data2 "github.com/rancher/wrangler/v2/pkg/data"
	"github.com/rancher/wrangler/v2/pkg/data/convert"
	corev1controllers "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	rbacv1controllers "github.com/rancher/wrangler/v2/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/v2/pkg/name"
	"github.com/rancher/wrangler/v2/pkg/schemas/validation"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	v1internal "k8s.io/kubernetes/pkg/apis/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	// helmDataPath contains the files such as values.yaml for a given chart and tar of the chart.
	helmDataPath = "/home/shell/helm"
	helmRunPath  = "/home/shell/helm-run"
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

var (
	podOptionsScheme = runtime.NewScheme()
	podOptionsCodec  = runtime.NewParameterCodec(podOptionsScheme)
)

var kustomization = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
transformers:
- /home/shell/helm-run/transform%s.yaml
resources:
- /home/shell/helm-run/all.yaml`

var transform = `apiVersion: builtin
kind: LabelTransformer
metadata:
  name: common-labels
labels:
  io.cattle.field/appId: %s
fieldSpecs:
- path: metadata/labels
  create: true
- path: spec/selector
  create: true
  version: v1
  kind: ReplicationController
- path: spec/template/metadata/labels
  create: true
  version: v1
  kind: ReplicationController
- path: spec/selector/matchLabels
  create: true
  kind: Deployment
- path: spec/template/metadata/labels
  create: true
  kind: Deployment
- path: spec/selector/matchLabels
  create: true
  kind: ReplicaSet
- path: spec/template/metadata/labels
  create: true
  kind: ReplicaSet
- path: spec/selector/matchLabels
  create: true
  kind: DaemonSet
- path: spec/template/metadata/labels
  create: true
  kind: DaemonSet
- path: spec/selector/matchLabels
  create: true
  group: apps
  kind: StatefulSet
- path: spec/template/metadata/labels
  create: true
  group: apps
  kind: StatefulSet
- path: spec/volumeClaimTemplates[]/metadata/labels
  create: true
  group: apps
  kind: StatefulSet
- path: spec/template/metadata/labels
  create: true
  group: batch
  kind: Job
- path: spec/jobTemplate/metadata/labels
  create: true
  group: batch
  kind: CronJob
- path: spec/jobTemplate/spec/template/metadata/labels
  create: true
  group: batch
  kind: CronJob`

func init() {
	v1internal.AddToScheme(podOptionsScheme)
}

// Operations describes a helm operation, containing its namespace, roles and such
type Operations struct {
	namespace      string                               // namespace the operation is going to be in
	contentManager *content.Manager                     // manager struct to retrieve information about helm repos and its charts
	Impersonator   *podimpersonation.PodImpersonation   // the impersonator used to manage pods created using the service account of the logged in user
	clusterRepos   catalogcontrollers.ClusterRepoClient // client for cluster repo custom resource
	ops            catalogcontrollers.OperationClient   // client for operation custom resource
	pods           corev1controllers.PodClient          // client for pod kubernetes resource
	apps           catalogcontrollers.AppClient         // client for apps custom resource
	roles          rbacv1controllers.RoleClient         // client for role kubernetes resource
	roleBindings   rbacv1controllers.RoleBindingClient  // client for rolebinding kubernetes resource
	cg             proxy.ClientGetter                   // dynamic kubernetes client factory
}

// NewOperations creates a new Operations struct with all fields initialized
func NewOperations(
	cg proxy.ClientGetter,
	catalog catalogcontrollers.Interface,
	rbac rbacv1controllers.Interface,
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
		roleBindings:   rbac.RoleBinding(),
		roles:          rbac.Role(),
	}
}

// Uninstall gets the uninstall commands using the given namespace, name and options and gets the user information using the isApp flag as true.
// Returns a catalog.Operation that represents the helm operation to be created
func (s *Operations) Uninstall(ctx context.Context, user user.Info, namespace, name string, options io.Reader, imageOverride string) (*catalog.Operation, error) {
	status, cmds, err := s.getUninstallArgs(namespace, name, options)
	if err != nil {
		return nil, err
	}

	user, err = s.getUser(user, namespace, name, true)
	if err != nil {
		return nil, err
	}

	return s.createOperation(ctx, user, status, cmds, imageOverride)
}

// Upgrade gets the upgrade commands using the given namespace, name and options and gets the user using the isApp flag as false.
// Returns a catalog.Operation that represents the helm operation to be created
func (s *Operations) Upgrade(ctx context.Context, user user.Info, namespace, name string, options io.Reader, imageOverride string) (*catalog.Operation, error) {
	status, cmds, err := s.getUpgradeCommand(namespace, name, options)
	if err != nil {
		return nil, err
	}

	user, err = s.getUser(user, namespace, name, false)
	if err != nil {
		return nil, err
	}

	return s.createOperation(ctx, user, status, cmds, imageOverride)
}

// Install gets the install commands using the given namespace, name and options and gets the user using the isApp flag as false.
// Returns a catalog.Operation that represents the helm operation to be created
func (s *Operations) Install(ctx context.Context, user user.Info, namespace, name string, options io.Reader, imageOverride string) (*catalog.Operation, error) {
	status, cmds, err := s.getInstallCommand(namespace, name, options)
	if err != nil {
		return nil, err
	}

	user, err = s.getUser(user, namespace, name, false)
	if err != nil {
		return nil, err
	}

	return s.createOperation(ctx, user, status, cmds, imageOverride)
}

// decodeParams decodes the request using it's url and v1 group version into the target object
func decodeParams(req *http.Request, target runtime.Object) error {
	return podOptionsCodec.DecodeParameters(req.URL.Query(), corev1.SchemeGroupVersion, target)
}

// proxyLogRequest proxies the given http.Request to get the logs of the given k8s pod and write it to the given http.ResponseWriter
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

	//Creates a reverse proxy and modifies the request url and headers
	p := httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = logURL
			req.Host = logURL.Host
			for key := range req.Header {
				if strings.HasPrefix(key, "Impersonate-Extra-") {
					delete(req.Header, key)
				}
			}
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

// Log receives a response writer, a http request, the namespace and name of an operation.
// Gets the pod of the operation and proxies the request to get logs of said pod
func (s *Operations) Log(rw http.ResponseWriter, req *http.Request, namespace, name string) error {
	op, err := s.ops.Get(namespace, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	pod, err := s.pods.Get(op.Status.PodNamespace, op.Status.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// check if the pod and op have objects depended by them and that they aren't the same
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

// getSpec receives the namespace and name of either an app or a repo according to the value of the isApp flag.
// If the isApp flag is true, gets the app and then check the annotations of the chart of the release to see if it's a cluster repo and to get it's name.
// Returns the found catalog.RepoSpec and doesn't return errors if the repo isn't found.
//
// If the isApp flag is false, gets the cluster repo directly using its name. Panics if the namespace isn't empty.
// Returns a catalog.RepoSec struct or an error if it's not found.
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

	panic("namespace should be empty")
}

// getUser receives the user info, the namespace of the repo and the name of either an app or a repo according to the value of the isApp flag.
// Gets the repoSpec and uses it to build a user.DefaultInfo struct with a default name and groups that will be used to create an operation in either the
// namespace of the repo or the one given.
// Returns a user.Info struct to create an operation.
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

// getUninstallArgs receives the app namespace, app name and body of the request.
// Returns an uninstall Command according to the input received and also returns the status of the operation that will be created
// to run the command
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

// getUpgradeCommand receives the repository namespace and name and body of the request.
// Returns the status of the operation that will be created and a list of Command to upgrade the charts received in the request
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
		cmd, err := s.getChartCommand(repoNamespace, repoName, chartUpgrade.ChartName, chartUpgrade.Version, true, chartUpgrade.Annotations, chartUpgrade.Values)
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

// Command represents a command that will be run inside a helm operation
type Command struct {
	Operation        string        // type of operation, eg upgrade, install, uninstall
	ArgObjects       []interface{} // the arguments that will be used in the command
	ValuesFile       string        // name of the values.yaml file
	Values           []byte        // content of the values.yaml file
	ChartFile        string        // name of the chart tar file
	Chart            []byte        // content of the chart file
	ReleaseName      string        // name of the release
	ReleaseNamespace string        // namespace of the release
	Kustomize        bool          // flag to inform if it should use kustomize.sh
}

type Commands []Command

// CommandArgs returns a list containing all the commands and their arguments
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

// Render calls the Command.Render method for each command and appends the
// result in a map and returns it
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

// Render returns a map containing the arguments and their values for the Command
func (c Command) Render(index int) (map[string][]byte, error) {
	args, err := c.renderArgs()
	if err != nil {
		return nil, err
	}

	fileNumID := fmt.Sprintf("%03d", index)
	data := map[string][]byte{
		fmt.Sprintf("operation%s", fileNumID): []byte(strings.Join(args, "\x00")),
	}
	if len(c.ValuesFile) > 0 {
		data[c.ValuesFile] = c.Values
	}
	if len(c.ChartFile) > 0 {
		data[c.ChartFile] = c.Chart
	}

	if c.Kustomize {
		data[fmt.Sprintf("kustomization%s.yaml", fileNumID)] = []byte(fmt.Sprintf(kustomization, fileNumID))
		data[fmt.Sprintf("transform%s.yaml", fileNumID)] = []byte(fmt.Sprintf(transform, c.ReleaseName))
	}

	return data, nil
}

// renderArgs returns a slice of string representing the Command.Operation and it's arguments.
// It uses the ArgObjects of the command struct to generate arguments for the command
// and removes the fields that aren't necessary in the command
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

	runPath := helmDataPath
	if c.Kustomize {
		// Run path when using kustomize.sh will be different. Original cannot be used
		// because write permissions are necessary and the helmDataPath cannot be
		// written to due to it having a SecretVolumeSource.
		runPath = helmRunPath
		args = append(args, "--post-renderer=/home/shell/kustomize.sh")
	}

	if len(c.Values) > 0 {
		args = append(args, "--values="+filepath.Join(runPath, c.ValuesFile))
	}

	if c.ReleaseNamespace != "" {
		args = append(args, "--namespace="+c.ReleaseNamespace)
	}

	sort.Strings(args)
	if c.ReleaseName != "" {
		args = append(args, c.ReleaseName)
	}
	if len(c.Chart) > 0 {
		args = append(args, filepath.Join(runPath, c.ChartFile))
	}

	return append([]string{c.Operation}, args...), nil
}

func sanitizeVersion(chartVersion string) string {
	return badChars.ReplaceAllString(chartVersion, "-")
}

// injectAnnotation receives the chart data from a tar file and injects the given annotations.
// Returns the modified chart data
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

		// checks if its chart.yaml
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

// addAnnotations receives that chart.yaml data and injects the given annotations in it
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

// enableKustomize returns whether kustomize should be used. If the helm operation is
// an upgrade and the migrated annotation is present, true will be returned.
func (s *Operations) enableKustomize(annotations map[string]string, upgrade bool) bool {
	if !upgrade {
		return false
	}

	if len(annotations) == 0 {
		return false
	}

	if val, _ := annotations["apps.cattle.io/migrated"]; val != "true" {
		return false
	}

	return true
}

// getChartCommand gets the chart based on the input, inject the annotations into it
// and then creates and return a Command containing the name of the values file, name of the chart file, the chart data
// and if the command should use kustomize.sh
func (s *Operations) getChartCommand(namespace, name, chartName, chartVersion string, upgrade bool, annotations map[string]string, values map[string]interface{}) (Command, error) {
	chart, err := s.contentManager.Chart(namespace, name, chartName, chartVersion, true)
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
		Kustomize:  s.enableKustomize(annotations, upgrade),
	}

	if len(values) > 0 {
		c.Values, err = json.Marshal(values)
		if err != nil {
			return Command{}, err
		}
	}

	return c, nil
}

// getInstallCommand receives the repository namespace, name, and body of the request.
// It decodes the request to get chart information for creating the `helm install` command
// along with args. It returns the catalog.OperationStatus struct and a slice of commands
// to install the charts received in the body of the request.
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
	// Sometimes there are two charts to be installed. First one being the CRD chart
	// and then the actual helm chart. So, we need a for loop and the last index of the array
	// would be the main chart.
	for _, chartInstall := range installArgs.Charts {
		cmd, err := s.getChartCommand(repoNamespace, repoName, chartInstall.ChartName, chartInstall.Version, false, chartInstall.Annotations, chartInstall.Values)
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

// namespace func sets the namespace as default if it is empty otherwise returns the same namespace it receives.
func namespace(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}

// createOperation creates an operation and its pod, along with its roles and roleBinding.
// Uses the Operations.Impersonator and Operations.ops to do it.
// Returns the created catalog.Operation struct
func (s *Operations) createOperation(ctx context.Context, user user.Info, status catalog.OperationStatus, cmds Commands, imageOverride string) (*catalog.Operation, error) {
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

	var kustomize bool
	for _, cmd := range cmds {
		if !cmd.Kustomize {
			continue
		}
		kustomize = true
		break
	}
	pod, podOptions := s.createPod(secretData, kustomize, imageOverride)
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

	if err := s.createRoleAndRoleBindings(op, user.GetName()); err != nil {
		return nil, err
	}

	op.Status = status
	return s.ops.UpdateStatus(op)
}

// createRoleAndRoleBindings creates a role that applies to the given catalog.Operation and
// creates a role binding that applies the rule to the given user
func (s *Operations) createRoleAndRoleBindings(op *catalog.Operation, user string) error {
	ownerRef := metav1.OwnerReference{
		APIVersion: op.APIVersion,
		Kind:       op.Kind,
		Name:       op.Name,
		UID:        op.UID,
	}
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name.SafeConcatName(op.GetName(), user, "role"),
			Namespace:       op.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"get"},
				Resources:     []string{"operations"},
				APIGroups:     []string{"catalog.cattle.io"},
				ResourceNames: []string{op.Name},
			},
		},
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name.SafeConcatName(op.GetName(), user, "rolebinding"),
			Namespace:       op.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.UserKind,
				Name:     user,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     role.Name,
		},
	}

	if _, err := s.roles.Create(role); err != nil {
		return err
	}

	if _, err := s.roleBindings.Create(roleBinding); err != nil {
		return err
	}

	return nil
}

// createNamespace creates a new k8s namespace and returns its object.
// It can also set the field.cattle.io/projectId annotation on the namespace if a non-empty projectID is provided.
// It creates a watch on the namespace and waits for the v3.ProjectConditionInitialRolesPopulated condition
// If the condition is not met within 30 seconds, it returns an error
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

// createPod creates the struct of the pod for the operation to run in. It also mounts a secret with the secretdata provided.
// If imageOverride is provided, it will override the default value of settings.FullShellImage.
// The created pod has default tolerations and node selectors.
// If the kustomize flag is true, the created pod is modified to be able to run the kustomize.sh script.
// Returns a pod object and a pod options object representing the helm operation pod and it's options
func (s *Operations) createPod(secretData map[string][]byte, kustomize bool, imageOverride string) (*v1.Pod, *podimpersonation.PodOptions) {
	image := imageOverride
	if image == "" {
		image = settings.FullShellImage()
	}
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
					Operator: corev1.TolerationOpEqual,
					Value:    "linux",
					Effect:   "NoSchedule",
				},
				{
					Key:      "node-role.kubernetes.io/controlplane",
					Operator: corev1.TolerationOpEqual,
					Value:    "true",
					Effect:   "NoSchedule",
				},
				{
					Key:      "node-role.kubernetes.io/etcd",
					Operator: corev1.TolerationOpEqual,
					Value:    "true",
					Effect:   "NoExecute",
				},
				{
					Key:      "node.cloudprovider.kubernetes.io/uninitialized",
					Operator: corev1.TolerationOpEqual,
					Value:    "true",
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
					Image:           image,
					ImagePullPolicy: v1.PullIfNotPresent,
					Command:         []string{"helm-cmd"},
					WorkingDir:      helmDataPath,
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "data",
							MountPath: helmDataPath,
							ReadOnly:  true,
						},
					},
				},
			},
		},
	}

	// if kustomize is false then helmDataPath is an acceptable path for helm to run. If it is true,
	// files are copied from helmDataPath to helmRunPath. This is because the kustomize.sh script
	// needs write permissions but volumes using a SecretVolumeSource are readOnly. This can not be
	// changed with the readOnly field or the defaultMode field.
	// See: https://github.com/kubernetes/kubernetes/issues/62099.
	if kustomize {
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			Name: "helm-run",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		})
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
			Name:      "helm-run",
			MountPath: helmRunPath,
		})
		pod.Spec.Containers[0].Lifecycle = &v1.Lifecycle{
			PostStart: &v1.LifecycleHandler{
				Exec: &v1.ExecAction{
					Command: []string{"/bin/sh", "-c", fmt.Sprintf("cp -r %s/. %s", helmDataPath, helmRunPath)},
				},
			},
		}
		pod.Spec.Containers[0].WorkingDir = helmRunPath
	}
	return pod, &podimpersonation.PodOptions{
		SecretsToCreate: []*v1.Secret{
			secret,
		},
		ImageOverride: imageOverride,
	}
}
