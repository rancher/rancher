package helmop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"path/filepath"
	"sort"
	"strings"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	"github.com/rancher/steve/pkg/podimpersonation"
	"github.com/rancher/steve/pkg/stores/proxy"
	"github.com/rancher/wrangler/pkg/data/convert"
	corev1controllers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"helm.sh/helm/v3/pkg/repo"
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
	helmDataPath = "/home/shell/.config/helm"
)

type Operations struct {
	namespace    string
	Impersonator *podimpersonation.PodImpersonation
	repos        catalogcontrollers.RepoClient
	secrets      corev1controllers.SecretClient
	clusterRepos catalogcontrollers.ClusterRepoClient
	ops          catalogcontrollers.OperationClient
	pods         corev1controllers.PodClient
	releases     catalogcontrollers.ReleaseClient
	cg           proxy.ClientGetter
}

func NewOperations(
	cg proxy.ClientGetter,
	catalog catalogcontrollers.Interface,
	pods corev1controllers.PodClient,
	secrets corev1controllers.SecretClient) *Operations {
	return &Operations{
		cg:           cg,
		namespace:    "dashboard-catalog",
		Impersonator: podimpersonation.New("helm-op", cg, time.Hour),
		repos:        catalog.Repo(),
		secrets:      secrets,
		pods:         pods,
		clusterRepos: catalog.ClusterRepo(),
		ops:          catalog.Operation(),
		releases:     catalog.Release(),
	}
}

func emptyData() map[string][]byte {
	return map[string][]byte{
		"repositories.yaml": []byte("{}"),
	}
}

func (s *Operations) Uninstall(ctx context.Context, user user.Info, namespace, name string, options io.Reader) (*catalog.Operation, error) {
	args, opNamespace, err := s.getUninstallArgs(namespace, name, options)
	if err != nil {
		return nil, err
	}

	return s.createOperation(ctx, user, opNamespace, args, emptyData())
}

func (s *Operations) Rollback(ctx context.Context, user user.Info, namespace, name string, options io.Reader) (*catalog.Operation, error) {
	args, status, err := s.getRollbackArgs(namespace, name, options)
	if err != nil {
		return nil, err
	}

	return s.createOperation(ctx, user, status, args, emptyData())
}

func (s *Operations) Upgrade(ctx context.Context, user user.Info, kind, namespace, name string, options io.Reader) (*catalog.Operation, error) {
	args, status, values, err := s.getUpgradeArgs(name, options)
	if err != nil {
		return nil, err
	}

	user, data, err := s.getSecretData(user, kind, namespace, name, values)
	if err != nil {
		return nil, err
	}

	return s.createOperation(ctx, user, status, args, data)
}

func (s *Operations) Install(ctx context.Context, user user.Info, kind, namespace, name string, options io.Reader) (*catalog.Operation, error) {
	installArgs, status, values, err := s.getInstallArgs(name, options)
	if err != nil {
		return nil, err
	}

	user, data, err := s.getSecretData(user, kind, namespace, name, values)
	if err != nil {
		return nil, err
	}

	return s.createOperation(ctx, user, status, installArgs, data)
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

func (s *Operations) getSpec(kind, namespace, name string) (*catalog.RepoSpec, error) {
	if kind == "ClusterRepo" {
		clusterRepo, err := s.clusterRepos.Get(name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &clusterRepo.Spec, nil
	}

	repo, err := s.repos.Get(namespace, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return &repo.Spec, nil
}

func (s *Operations) getSecretData(userInfo user.Info, kind, namespace, name string, values []byte) (user.Info, map[string][]byte, error) {
	repoSpec, err := s.getSpec(kind, namespace, name)
	if err != nil {
		return nil, nil, err
	}

	data := map[string][]byte{}
	entry := repo.Entry{
		Name:                  name,
		URL:                   repoSpec.URL,
		InsecureSkipTLSverify: repoSpec.InsecureSkipTLSverify,
	}

	if repoSpec.ClientSecret != nil {
		ns := repoSpec.ClientSecret.Namespace
		if namespace != "" {
			ns = namespace
		}

		secret, err := s.secrets.Get(ns, repoSpec.ClientSecret.Name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}

		switch secret.Type {
		case v1.SecretTypeTLS:
			for k, v := range secret.Data {
				data[k] = v
			}
			entry.CertFile = filepath.Join(helmDataPath, v1.TLSCertKey)
			entry.KeyFile = filepath.Join(helmDataPath, v1.TLSPrivateKeyKey)
		case v1.SecretTypeBasicAuth:
			entry.Username = string(secret.Data[v1.BasicAuthUsernameKey])
			entry.Password = string(secret.Data[v1.BasicAuthPasswordKey])
		}
	}

	if len(repoSpec.CABundle) > 0 {
		data["ca.pem"] = repoSpec.CABundle
		entry.CAFile = filepath.Join(helmDataPath, "ca.pem")
	}

	file := repo.NewFile()
	file.Repositories = append(file.Repositories, &entry)
	fileBytes, err := yaml.Marshal(file)
	if err != nil {
		return nil, nil, err
	}

	data["repositories.yaml"] = fileBytes
	data["values.yaml"] = values

	return getUser(namespace, repoSpec, userInfo), data, nil
}

func getUser(namespace string, repoSpec *catalog.RepoSpec, userInfo user.Info) user.Info {
	if repoSpec.ServiceAccount == "" {
		return userInfo
	}
	serviceAccountNS := repoSpec.ServiceAccountNamespace
	if namespace != "" {
		serviceAccountNS = namespace
	}
	if serviceAccountNS == "" || strings.Contains(repoSpec.ServiceAccountNamespace, ":") {
		return userInfo
	}
	return &user.DefaultInfo{
		Name: fmt.Sprintf("system:serviceaccount:%s:%s", repoSpec.ServiceAccount, serviceAccountNS),
		Groups: []string{
			"system:serviceaccounts",
			"system:serviceaccounts:" + serviceAccountNS,
		},
	}
}

func (s *Operations) getUninstallArgs(releaseNamespace, releaseName string, body io.Reader) ([]string, catalog.OperationStatus, error) {
	rel, err := s.releases.Get(releaseNamespace, releaseName, metav1.GetOptions{})
	if err != nil {
		return nil, catalog.OperationStatus{}, err
	}

	rollbackArgs := &catalog.ChartUninstallAction{}
	if err := json.NewDecoder(body).Decode(rollbackArgs); err != nil {
		return nil, catalog.OperationStatus{}, err
	}

	suffix := []string{
		rel.Spec.Name,
	}

	status := catalog.OperationStatus{
		Action:    "uninstall",
		Release:   rel.Spec.Name,
		Namespace: releaseNamespace,
	}

	args, _, err := toArgs(status.Action, nil, rollbackArgs, suffix)
	return args, status, err
}

func (s *Operations) getRollbackArgs(releaseNamespace, releaseName string, body io.Reader) ([]string, catalog.OperationStatus, error) {
	rel, err := s.releases.Get(releaseNamespace, releaseName, metav1.GetOptions{})
	if err != nil {
		return nil, catalog.OperationStatus{}, err
	}

	rollbackArgs := &catalog.ChartRollbackAction{}
	if err := json.NewDecoder(body).Decode(rollbackArgs); err != nil {
		return nil, catalog.OperationStatus{}, err
	}

	suffix := []string{
		rel.Spec.Name,
		fmt.Sprint(rel.Spec.Version),
	}

	status := catalog.OperationStatus{
		Action:    "rollback",
		Release:   rel.Spec.Name,
		Namespace: releaseNamespace,
	}

	args, _, err := toArgs(status.Action, nil, rollbackArgs, suffix)
	return args, status, err
}

func (s *Operations) getUpgradeArgs(repoName string, body io.Reader) ([]string, catalog.OperationStatus, []byte, error) {
	upgradeArgs := &catalog.ChartUpgradeAction{}
	err := json.NewDecoder(body).Decode(upgradeArgs)
	if err != nil {
		return nil, catalog.OperationStatus{}, nil, err
	}

	suffix := []string{
		upgradeArgs.ReleaseName,
		repoName + "/" + upgradeArgs.ChartName,
	}

	status := catalog.OperationStatus{
		Action:    "upgrade",
		Release:   upgradeArgs.ReleaseName,
		Namespace: namespace(upgradeArgs.Namespace),
	}

	args, values, err := toArgs("upgrade", upgradeArgs.Values, upgradeArgs, suffix)
	return args, status, values, err
}

func (s *Operations) getInstallArgs(repoName string, body io.Reader) ([]string, catalog.OperationStatus, []byte, error) {
	installArgs := &catalog.ChartInstallAction{}
	err := json.NewDecoder(body).Decode(installArgs)
	if err != nil {
		return nil, catalog.OperationStatus{}, nil, err
	}

	var suffix []string
	if installArgs.ReleaseName == "" {
		installArgs.GenerateName = true
	} else {
		suffix = append(suffix, installArgs.ReleaseName)
	}
	if installArgs.ChartName != "" {
		suffix = append(suffix, repoName+"/"+installArgs.ChartName)
	}

	status := catalog.OperationStatus{
		Action:    "install",
		Release:   installArgs.ReleaseName,
		Namespace: namespace(installArgs.Namespace),
	}

	args, values, err := toArgs(status.Action, installArgs.Values, installArgs, suffix)
	return args, status, values, err
}

func namespace(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}

func toArgs(operation string, values map[string]interface{}, input interface{}, suffix []string) (args []string, valueContent []byte, err error) {
	var prefix []string

	switch operation {
	case "uninstall":
		fallthrough
	case "rollback":
		prefix = []string{"helm-run", operation}
	default:
		prefix = []string{"helm-update", operation}
	}

	data, err := convert.EncodeToMap(input)
	if err != nil {
		return nil, nil, err
	}
	delete(data, "values")
	delete(data, "releaseName")
	delete(data, "chartName")
	if v, ok := data["disableOpenAPIValidation"]; ok {
		delete(data, "disableOpenAPIValidation")
		data["disableOpenapiValidation"] = v
	}
	if operation == "install" {
		data["createNamespace"] = true
	}

	for k, v := range data {
		s := convert.ToString(v)
		k = convert.ToArgKey(k)
		args = append(args, fmt.Sprintf("%s=%s", k, s))
	}

	if len(values) > 0 {
		args = append(args, "--values="+filepath.Join(helmDataPath, "values.yaml"))
		valueContent, err = json.Marshal(values)
		if err != nil {
			return nil, nil, err
		}
	}

	sort.Strings(args)
	return append(prefix, append(args, suffix...)...), valueContent, nil
}

func (s *Operations) createOperation(ctx context.Context, user user.Info, status catalog.OperationStatus, command []string, secretData map[string][]byte) (*catalog.Operation, error) {
	pod, podOptions := s.createPod(command, secretData)
	pod, err := s.Impersonator.CreatePod(ctx, user, pod, podOptions)
	if err != nil {
		return nil, err
	}

	command[0] = "helm"

	status.Token = pod.Labels[podimpersonation.TokenLabel]
	status.Command = strings.Join(command, " ")
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

func (s *Operations) createPod(command []string, secretData map[string][]byte) (*v1.Pod, *podimpersonation.PodOptions) {
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
					Image:           "ibuildthecloud/shell:v0.0.5",
					ImagePullPolicy: v1.PullIfNotPresent,
					Command:         command,
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
