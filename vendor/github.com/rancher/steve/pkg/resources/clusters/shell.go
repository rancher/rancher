package clusters

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/rancher/steve/pkg/stores/proxy"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	roleLabel = "shell.cattle.io/cluster-role"
)

type shell struct {
	namespace string
	cg        proxy.ClientGetter
}

func (s *shell) PurgeOldShell(gvr schema.GroupVersionResource, key string, obj runtime.Object) error {
	if obj == nil ||
		gvr.Version != "v1" ||
		gvr.Group != rbacv1.GroupName ||
		gvr.Resource != "clusterroles" {
		return nil
	}

	meta, err := meta.Accessor(obj)
	if err != nil {
		// ignore error
		logrus.Warnf("failed to find metadata for %v, %s", gvr, key)
		return nil
	}

	if meta.GetLabels()[roleLabel] != "true" {
		return nil
	}

	if meta.GetCreationTimestamp().Add(time.Hour).Before(time.Now()) {
		client, err := s.cg.AdminK8sInterface()
		if err != nil {
			return nil
		}
		name := meta.GetName()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			client.RbacV1().ClusterRoles().Delete(ctx, name, metav1.DeleteOptions{})
		}()
	}

	return nil
}

func (s *shell) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ctx, user, client, err := s.contextAndClient(req)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	role, err := s.createRole(ctx, user, client)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() {
		// Don't use request context as it already be canceled at this point
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		client.RbacV1().ClusterRoles().Delete(ctx, role.Name, metav1.DeleteOptions{})
	}()

	pod, err := s.createPod(ctx, user, role, client)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	s.proxyRequest(rw, req, pod, client)
}

func (s *shell) proxyRequest(rw http.ResponseWriter, req *http.Request, pod *v1.Pod, client kubernetes.Interface) {
	attachURL := client.CoreV1().RESTClient().
		Get().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("attach").
		VersionedParams(&v1.PodAttachOptions{
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
			Container: "shell",
		}, scheme.ParameterCodec).URL()

	httpClient := client.CoreV1().RESTClient().(*rest.RESTClient).Client
	p := httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL = attachURL
			req.Host = attachURL.Host
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

func (s *shell) createNamespace(ctx context.Context, client kubernetes.Interface) (*v1.Namespace, error) {
	ns, err := client.CoreV1().Namespaces().Get(ctx, s.namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return client.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: s.namespace,
			},
		}, metav1.CreateOptions{})
	}
	return ns, err
}

func (s *shell) createRole(ctx context.Context, user user.Info, client kubernetes.Interface) (*rbacv1.ClusterRole, error) {
	_, err := s.createNamespace(ctx, client)
	if err != nil {
		return nil, err
	}

	return client.RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "dashboard-shell-",
			Labels: map[string]string{
				roleLabel: "true",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"impersonate"},
				APIGroups:     []string{""},
				Resources:     []string{"users"},
				ResourceNames: []string{user.GetName()},
			},
			{
				Verbs:         []string{"impersonate"},
				APIGroups:     []string{""},
				Resources:     []string{"groups"},
				ResourceNames: user.GetGroups(),
			},
		},
		AggregationRule: nil,
	}, metav1.CreateOptions{})

}

func (s *shell) createRoleBinding(ctx context.Context, role *rbacv1.ClusterRole, serviceAccount *v1.ServiceAccount, client kubernetes.Interface) error {
	_, err := client.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    "dashboard-shell-",
			OwnerReferences: ref(role),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				APIGroup:  "",
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     role.Name,
		},
	}, metav1.CreateOptions{})
	return err
}

func ref(role *rbacv1.ClusterRole) []metav1.OwnerReference {
	ref := metav1.OwnerReference{
		Name: role.Name,
		UID:  role.UID,
	}
	ref.APIVersion, ref.Kind = rbacv1.SchemeGroupVersion.WithKind("ClusterRole").ToAPIVersionAndKind()
	return []metav1.OwnerReference{
		ref,
	}
}

func (s *shell) updateServiceAccount(ctx context.Context, pod *v1.Pod, serviceAccount *v1.ServiceAccount, client kubernetes.Interface) error {
	serviceAccount, err := client.CoreV1().ServiceAccounts(s.namespace).Get(ctx, serviceAccount.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	serviceAccount.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "v1",
			Kind:       "Pod",
			Name:       pod.Name,
			UID:        pod.UID,
		},
	}

	_, err = client.CoreV1().ServiceAccounts(s.namespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
	return err
}

func (s *shell) createServiceAccount(ctx context.Context, role *rbacv1.ClusterRole, client kubernetes.Interface) (*v1.ServiceAccount, error) {
	return client.CoreV1().ServiceAccounts(s.namespace).Create(ctx, &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    "dashboard-shell-",
			OwnerReferences: ref(role),
		},
	}, metav1.CreateOptions{})
}

func (s *shell) createConfigMap(ctx context.Context, user user.Info, role *rbacv1.ClusterRole, client kubernetes.Interface) (*v1.ConfigMap, error) {
	cfg := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster": {
				Server:               "https://kubernetes.default",
				CertificateAuthority: "/run/secrets/kubernetes.io/serviceaccount/ca.crt",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"user": {
				TokenFile:         "/run/secrets/kubernetes.io/serviceaccount/token",
				Impersonate:       user.GetName(),
				ImpersonateGroups: user.GetGroups(),
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {
				Cluster:  "cluster",
				AuthInfo: "user",
			},
		},
		CurrentContext: "default",
	}

	cfgData, err := clientcmd.Write(cfg)
	if err != nil {
		return nil, err
	}

	return client.CoreV1().ConfigMaps(s.namespace).Create(ctx, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    "dashboard-shell-",
			Namespace:       s.namespace,
			OwnerReferences: ref(role),
		},
		Data: map[string]string{
			"config": string(cfgData),
		},
	}, metav1.CreateOptions{})
}

func (s *shell) createPod(ctx context.Context, user user.Info, role *rbacv1.ClusterRole, client kubernetes.Interface) (*v1.Pod, error) {
	sa, err := s.createServiceAccount(ctx, role, client)
	if err != nil {
		return nil, err
	}

	if err := s.createRoleBinding(ctx, role, sa, client); err != nil {
		return nil, err
	}

	cm, err := s.createConfigMap(ctx, user, role, client)
	if err != nil {
		return nil, err
	}

	zero := int64(0)
	t := true
	pod, err := client.CoreV1().Pods(s.namespace).Create(ctx, &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    "dashboard-shell-",
			Namespace:       s.namespace,
			OwnerReferences: ref(role),
		},
		Spec: v1.PodSpec{
			TerminationGracePeriodSeconds: &zero,
			Volumes: []v1.Volume{
				{
					Name: "config",
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: cm.Name,
							},
						},
					},
				},
			},
			RestartPolicy:      v1.RestartPolicyNever,
			ServiceAccountName: sa.Name,
			Containers: []v1.Container{
				{
					Name:            "proxy",
					Image:           "ibuildthecloud/shell:v0.0.1",
					ImagePullPolicy: v1.PullIfNotPresent,
					Env: []v1.EnvVar{
						{
							Name:  "KUBECONFIG",
							Value: "/root/.kube/config",
						},
					},
					Command: []string{"kubectl", "proxy"},
					SecurityContext: &v1.SecurityContext{
						RunAsUser:              &zero,
						RunAsGroup:             &zero,
						ReadOnlyRootFilesystem: &t,
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "config",
							ReadOnly:  true,
							MountPath: "/root/.kube/config",
							SubPath:   "config",
						},
					},
				},
				{
					Name:            "shell",
					TTY:             true,
					Stdin:           true,
					StdinOnce:       true,
					Image:           "ibuildthecloud/shell:v0.0.1",
					ImagePullPolicy: v1.PullIfNotPresent,
					Command:         []string{"bash"},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// ignore any error here
	err = s.updateServiceAccount(ctx, pod, sa, client)
	if err != nil {
		logrus.Warnf("failed to update service account %s/%s to be owned by pod %s/%s", sa.Namespace, sa.Name, pod.Namespace, pod.Name)
	}

	sec := int64(60)
	resp, err := client.CoreV1().Pods(s.namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector:   "metadata.name=" + pod.Name,
		ResourceVersion: pod.ResourceVersion,
		TimeoutSeconds:  &sec,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Stop()

	for event := range resp.ResultChan() {
		newPod, ok := event.Object.(*v1.Pod)
		if !ok {
			continue
		}
		if condition.Cond(v1.PodReady).IsTrue(newPod) {
			return newPod, nil
		}
	}

	return nil, fmt.Errorf("failed to find shell")
}
