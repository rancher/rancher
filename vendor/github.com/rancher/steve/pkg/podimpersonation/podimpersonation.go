package podimpersonation

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/wrangler/pkg/randomtoken"

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
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	roleLabel  = "pod-impersonation.cattle.io/cluster-role"
	keyLabel   = "pod-impersonation.cattle.io/key"
	TokenLabel = "pod-impersonation.cattle.io/token"
)

type PodImpersonation struct {
	roleTimeout time.Duration
	cg          proxy.ClientGetter
	key         string
}

func New(key string, cg proxy.ClientGetter, roleTimeout time.Duration) *PodImpersonation {
	return &PodImpersonation{
		roleTimeout: roleTimeout,
		cg:          cg,
		key:         key,
	}
}

func (s *PodImpersonation) PurgeOldRoles(gvr schema.GroupVersionResource, key string, obj runtime.Object) error {
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

	if meta.GetLabels()[roleLabel] != "true" && meta.GetLabels()[keyLabel] != s.key {
		return nil
	}

	if meta.GetCreationTimestamp().Add(s.roleTimeout).Before(time.Now()) {
		client, err := s.cg.AdminK8sInterface()
		if err != nil {
			return nil
		}
		name := meta.GetName()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_ = client.RbacV1().ClusterRoles().Delete(ctx, name, metav1.DeleteOptions{})
		}()
	}

	return nil
}

func (s *PodImpersonation) DeleteRole(ctx context.Context, pod v1.Pod) error {
	client, err := s.cg.AdminK8sInterface()
	if err != nil {
		return err
	}
	roleName := pod.Annotations[roleLabel]
	return client.RbacV1().ClusterRoles().Delete(ctx, roleName, metav1.DeleteOptions{})
}

type PodOptions struct {
	ConfigMapsToCreate []*v1.ConfigMap
	SecretsToCreate    []*v1.Secret
	Wait               bool
}

// CreatePod will create a pod with a service account that impersonates as user. Corresponding
// ClusterRoles, ClusterRoleBindings, and ServiceAccounts will be create.
// IMPORTANT NOTES:
//   1. To ensure this is used securely the namespace assigned to the pod must be a dedicated
//      namespace used only for the purpose of running impersonated pods. This is to ensure
//      proper protection for the service accounts created.
//   2. The pod must KUBECONFIG env var set to where you expect the kubeconfig to reside
func (s *PodImpersonation) CreatePod(ctx context.Context, user user.Info, pod *v1.Pod, podOptions *PodOptions) (*v1.Pod, error) {
	if podOptions == nil {
		podOptions = &PodOptions{}
	}

	client, err := s.cg.AdminK8sInterface()
	if err != nil {
		return nil, err
	}

	role, err := s.createRole(ctx, user, pod.Namespace, client)
	if err != nil {
		return nil, err
	}

	pod, err = s.createPod(ctx, user, role, pod, podOptions, client)
	if err != nil {
		_ = client.RbacV1().ClusterRoles().Delete(ctx, role.Name, metav1.DeleteOptions{})
		return nil, err
	}

	return pod, err
}

func (s *PodImpersonation) userAndClient(ctx context.Context) (user.Info, kubernetes.Interface, error) {
	client, err := s.cg.AdminK8sInterface()
	if err != nil {
		return nil, nil, err
	}

	user, ok := request.UserFrom(ctx)
	if !ok {
		return nil, nil, validation.Unauthorized
	}

	return user, client, nil
}

func (s *PodImpersonation) createNamespace(ctx context.Context, namespace string, client kubernetes.Interface) (*v1.Namespace, error) {
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

func (s *PodImpersonation) createRole(ctx context.Context, user user.Info, namespace string, client kubernetes.Interface) (*rbacv1.ClusterRole, error) {
	_, err := s.createNamespace(ctx, namespace, client)
	if err != nil {
		return nil, err
	}

	return client.RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pod-impersonation-" + s.key + "-",
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

func (s *PodImpersonation) createRoleBinding(ctx context.Context, role *rbacv1.ClusterRole, serviceAccount *v1.ServiceAccount, client kubernetes.Interface) error {
	_, err := client.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    "pod-impersonation-" + s.key + "-",
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

func (s *PodImpersonation) updateServiceAccount(ctx context.Context, pod *v1.Pod, serviceAccount *v1.ServiceAccount, client kubernetes.Interface) error {
	serviceAccount, err := client.CoreV1().ServiceAccounts(serviceAccount.Namespace).Get(ctx, serviceAccount.Name, metav1.GetOptions{})
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

	_, err = client.CoreV1().ServiceAccounts(serviceAccount.Namespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
	return err
}

func (s *PodImpersonation) createServiceAccount(ctx context.Context, role *rbacv1.ClusterRole, client kubernetes.Interface, namespace string) (*v1.ServiceAccount, error) {
	return client.CoreV1().ServiceAccounts(namespace).Create(ctx, &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    "pod-impersonation-" + s.key + "-",
			OwnerReferences: ref(role),
		},
	}, metav1.CreateOptions{})
}

func (s *PodImpersonation) waitForServiceAccount(ctx context.Context, client kubernetes.Interface, sa *v1.ServiceAccount) (*v1.ServiceAccount, error) {
	sa, err := client.CoreV1().ServiceAccounts(sa.Namespace).Get(ctx, sa.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if len(sa.Secrets) > 0 {
		return sa, nil
	}
	t := int64(30)
	w, err := client.CoreV1().ServiceAccounts(sa.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector:   "metadata.name=" + sa.Name,
		ResourceVersion: sa.ResourceVersion,
		TimeoutSeconds:  &t,
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		go func() {
			for range w.ResultChan() {
			}
		}()
		w.Stop()
	}()

	for event := range w.ResultChan() {
		sa, ok := event.Object.(*v1.ServiceAccount)
		if !ok {
			continue
		}
		if len(sa.Secrets) > 0 {
			return sa, nil
		}
	}

	return nil, fmt.Errorf("timeout trying to get secret for service account: %s/%s", sa.Namespace, sa.Name)
}

func (s *PodImpersonation) createPod(ctx context.Context, user user.Info, role *rbacv1.ClusterRole, pod *v1.Pod, podOptions *PodOptions, client kubernetes.Interface) (*v1.Pod, error) {
	sa, err := s.createServiceAccount(ctx, role, client, pod.Namespace)
	if err != nil {
		return nil, err
	}

	if err := s.createRoleBinding(ctx, role, sa, client); err != nil {
		return nil, err
	}

	sa, err = s.waitForServiceAccount(ctx, client, sa)
	if err != nil {
		return nil, err
	}

	pod = s.augmentPod(pod, sa)

	if err := s.createConfigMaps(ctx, user, role, pod, podOptions, client); err != nil {
		return nil, err
	}

	if err := s.createSecrets(ctx, role, pod, podOptions, client); err != nil {
		return nil, err
	}

	pod.OwnerReferences = ref(role)
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[roleLabel] = role.Name

	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}
	pod.Labels[TokenLabel], err = randomtoken.Generate()
	if err != nil {
		return nil, err
	}

	pod, err = client.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// ignore any error here
	err = s.updateServiceAccount(ctx, pod, sa, client)
	if err != nil {
		logrus.Warnf("failed to update service account %s/%s to be owned by pod %s/%s", sa.Namespace, sa.Name, pod.Namespace, pod.Name)
	}

	if !podOptions.Wait {
		return pod, nil
	}

	sec := int64(60)
	resp, err := client.CoreV1().Pods(pod.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector:   "metadata.name=" + pod.Name,
		ResourceVersion: pod.ResourceVersion,
		TimeoutSeconds:  &sec,
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		go func() {
			for range resp.ResultChan() {
			}
		}()
		resp.Stop()
	}()

	for event := range resp.ResultChan() {
		newPod, ok := event.Object.(*v1.Pod)
		if !ok {
			continue
		}
		if condition.Cond(v1.PodReady).IsTrue(newPod) {
			return newPod, nil
		}
	}

	return nil, fmt.Errorf("failed to create pod: %s/%s", pod.Namespace, pod.Name)
}

func (s *PodImpersonation) userKubeConfig(role *rbacv1.ClusterRole, namespace string) (*v1.ConfigMap, error) {
	cfg := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster": {
				Server: "http://127.0.0.1:8001",
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {
				Cluster: "cluster",
			},
		},
		CurrentContext: "default",
	}

	cfgData, err := clientcmd.Write(cfg)
	if err != nil {
		return nil, err
	}

	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    s.userConfigName(),
			Namespace:       namespace,
			OwnerReferences: ref(role),
		},
		Data: map[string]string{
			"config": string(cfgData),
		},
	}, nil
}

func (s *PodImpersonation) adminConfigName() string {
	return "impersonation-" + s.key + "-admin-kubeconfig-"
}

func (s *PodImpersonation) userConfigName() string {
	return "impersonation-" + s.key + "-user-kubeconfig-"
}

func (s *PodImpersonation) adminKubeConfig(user user.Info, role *rbacv1.ClusterRole, namespace string) (*v1.ConfigMap, error) {
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

	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    s.adminConfigName(),
			Namespace:       namespace,
			OwnerReferences: ref(role),
		},
		Data: map[string]string{
			"config": string(cfgData),
		},
	}, nil
}

func (s *PodImpersonation) augmentPod(pod *v1.Pod, sa *v1.ServiceAccount) *v1.Pod {
	var (
		zero = int64(0)
		t    = true
		f    = false
		m    = int32(420)
	)

	pod = pod.DeepCopy()

	pod.Spec.ServiceAccountName = ""
	pod.Spec.AutomountServiceAccountToken = &f
	pod.Spec.Volumes = append(pod.Spec.Volumes,
		v1.Volume{
			Name: "admin-kubeconfig",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: s.adminConfigName(),
					},
				},
			},
		},
		v1.Volume{
			Name: "user-kubeconfig",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: s.userConfigName(),
					},
				},
			},
		},
		v1.Volume{
			Name: sa.Secrets[0].Name,
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName:  sa.Secrets[0].Name,
					DefaultMode: &m,
				},
			},
		})

	for i, container := range pod.Spec.Containers {
		for _, envvar := range container.Env {
			if envvar.Name != "KUBECONFIG" {
				continue
			}

			pod.Spec.Containers[i].VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
				Name:      "user-kubeconfig",
				ReadOnly:  true,
				MountPath: envvar.Value,
				SubPath:   "config",
			})
			break
		}
	}

	pod.Spec.Containers = append(pod.Spec.Containers, v1.Container{
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
				Name:      "admin-kubeconfig",
				ReadOnly:  true,
				MountPath: "/root/.kube/config",
				SubPath:   "config",
			},
			{
				Name:      sa.Secrets[0].Name,
				MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
				ReadOnly:  true,
			},
		},
	})

	return pod
}

func (s *PodImpersonation) createSecrets(ctx context.Context, role *rbacv1.ClusterRole, pod *v1.Pod, podOptions *PodOptions, client kubernetes.Interface) error {
	translateNames := map[string]string{}
	for _, cm := range podOptions.SecretsToCreate {
		oldName := cm.GenerateName
		cm.OwnerReferences = ref(role)
		cm, err := client.CoreV1().Secrets(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		if oldName != "" {
			translateNames[oldName] = cm.Name
		}
	}

	for oldName, newName := range translateNames {
		for i, v := range pod.Spec.Volumes {
			if v.Secret != nil && v.Secret.SecretName == oldName {
				pod.Spec.Volumes[i].Secret.SecretName = newName
			}
		}
	}

	return nil
}

func (s *PodImpersonation) createConfigMaps(ctx context.Context, user user.Info, role *rbacv1.ClusterRole, pod *v1.Pod, podOptions *PodOptions, client kubernetes.Interface) error {
	userKubeConfig, err := s.userKubeConfig(role, pod.Namespace)
	if err != nil {
		return err
	}
	adminKubeConfig, err := s.adminKubeConfig(user, role, pod.Namespace)
	if err != nil {
		return err
	}

	translateNames := map[string]string{}
	for _, cm := range append(podOptions.ConfigMapsToCreate, userKubeConfig, adminKubeConfig) {
		oldName := cm.GenerateName
		cm.OwnerReferences = ref(role)
		cm, err := client.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		if oldName != "" {
			translateNames[oldName] = cm.Name
		}
	}

	for oldName, newName := range translateNames {
		for i, v := range pod.Spec.Volumes {
			if v.ConfigMap != nil && v.ConfigMap.Name == oldName {
				pod.Spec.Volumes[i].ConfigMap.Name = newName
			}
		}
	}

	return nil
}
