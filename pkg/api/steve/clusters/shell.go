package clusters

import (
	"context"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/steve/pkg/podimpersonation"
	"github.com/rancher/steve/pkg/stores/proxy"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type shell struct {
	namespace       string
	impersonator    *podimpersonation.PodImpersonation
	cg              proxy.ClientGetter
	clusterRegistry string
}

func (s *shell) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ctx, user, client, err := s.contextAndClient(req)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	var imageOverride string
	if s.clusterRegistry != "" {
		imageOverride = s.clusterRegistry + "/" + settings.ShellImage.Get()
	}

	pod, err := s.impersonator.CreatePod(ctx, user, s.createPod(imageOverride), &podimpersonation.PodOptions{
		Wait:          true,
		ImageOverride: imageOverride,
	})
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		_ = client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	}()
	s.proxyRequest(rw, req, pod, client)
}

func (s *shell) proxyRequest(rw http.ResponseWriter, req *http.Request, pod *v1.Pod, client kubernetes.Interface) {
	attachURL := client.CoreV1().RESTClient().
		Get().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
			Container: "shell",
			Command:   []string{"welcome"},
		}, scheme.ParameterCodec).URL()

	httpClient := client.CoreV1().RESTClient().(*rest.RESTClient).Client
	p := httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = attachURL
			req.Host = attachURL.Host
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
}

func (s *shell) contextAndClient(req *http.Request) (context.Context, user.Info, kubernetes.Interface, error) {
	ctx := req.Context()
	client, err := s.cg.AdminK8sInterface()
	if err != nil {
		return ctx, nil, nil, err
	}

	user, ok := request.UserFrom(ctx)
	if !ok {
		return ctx, nil, nil, validation.Unauthorized
	}

	return ctx, user, client, nil
}

func (s *shell) createPod(imageOverride string) *v1.Pod {
	imageName := imageOverride
	if imageName == "" {
		imageName = settings.FullShellImage()
	}
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "dashboard-shell-",
			Namespace:    s.namespace,
		},
		Spec: v1.PodSpec{
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
				{
					Key:      "node-role.kubernetes.io/controlplane",
					Operator: "Equal",
					Value:    "true",
					Effect:   "NoSchedule",
				},
				{
					Key:      "node-role.kubernetes.io/control-plane",
					Operator: "Equal",
					Value:    "true",
					Effect:   "NoSchedule",
				},
				{
					Key:      "node-role.kubernetes.io/etcd",
					Operator: "Equal",
					Value:    "true",
					Effect:   "NoExecute",
				},
			},
			Containers: []v1.Container{
				{
					Name:      "shell",
					TTY:       true,
					Stdin:     true,
					StdinOnce: true,
					Env: []v1.EnvVar{
						{
							Name:  "KUBECONFIG",
							Value: "/home/shell/.kube/config",
						},
					},
					Image:           imageName,
					ImagePullPolicy: v1.PullIfNotPresent,
				},
			},
		},
	}
}
