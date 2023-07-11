package registry

import (
	"fmt"
	"sync"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/rancher/rke/pki/cert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	registryNamespace = "default"
	cacheLock         sync.Mutex
)

func newRegistryServiceName(baseName string) string {
	return fmt.Sprintf("%s.default.svc.cluster.local", baseName)
}

func createPasswordSecret(clients *clients.Clients, namespace string) (*corev1.Secret, error) {
	secret, err := clients.Core.Secret().Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "registry-secret-",
			Namespace:    namespace,
		},
		StringData: map[string]string{
			"username": "admin",
			"password": "admin",
		},
		Type: "rke.cattle.io/auth-config",
	})
	if err != nil {
		return nil, err
	}
	clients.OnClose(func() {
		_ = clients.Core.Secret().Delete(namespace, secret.Name, nil)
	})
	return secret, nil
}

func createTLSSecret(clients *clients.Clients, namespace string, registryTLSSecret *corev1.Secret) (*corev1.Secret, error) {
	secret, err := clients.Core.Secret().Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "registry-secret-",
			Namespace:    namespace,
		},
		Data: registryTLSSecret.Data,
		Type: corev1.SecretTypeTLS,
	})
	if err != nil {
		return nil, err
	}
	clients.OnClose(func() {
		_ = clients.Core.Secret().Delete(namespace, secret.Name, nil)
	})
	return secret, nil
}

func createRegistrySecret(clients *clients.Clients, podName string) (*corev1.Secret, error) {
	secret, err := clients.Core.Secret().Get(registryNamespace, podName, metav1.GetOptions{})
	if err == nil {
		return secret, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	cert, key, err := cert.GenerateSelfSignedCertKey(newRegistryServiceName(podName), nil, nil)
	if err != nil {
		return nil, err
	}

	secret, err = clients.Core.Secret().Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: registryNamespace,
		},
		Data: map[string][]byte{
			// admin:admin
			"htpasswd":              []byte("admin:$2y$05$vhD/ZtPIUBFrSR4BRyWaDeWAj7NOa5/xZRCjijO6XRRBaiRTLQ/76"),
			corev1.TLSCertKey:       cert,
			corev1.TLSPrivateKeyKey: key,
		},
	})
	if err == nil {
		return secret, nil
	} else if apierrors.IsAlreadyExists(err) {
		return clients.Core.Secret().Get(registryNamespace, podName, metav1.GetOptions{})
	}
	return nil, err
}

func createService(clients *clients.Clients, name string) error {
	_, err := clients.Core.Service().Create(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: registryNamespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name:        "http",
				Protocol:    corev1.ProtocolTCP,
				AppProtocol: &[]string{"https"}[0],
				Port:        5000,
				TargetPort:  intstr.FromInt(5000),
			}},
			Selector: map[string]string{
				"app": name,
			},
		},
	})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func createOrGetPod(clients *clients.Clients, podName string, pullThrough bool) (*corev1.Pod, error) {
	pod, err := clients.Core.Pod().Get(registryNamespace, podName, metav1.GetOptions{})
	if err == nil || !apierrors.IsNotFound(err) {
		return pod, err
	}

	podEnv := []corev1.EnvVar{
		{
			Name:  "REGISTRY_AUTH",
			Value: "htpasswd",
		},
		{
			Name:  "REGISTRY_AUTH_HTPASSWD_REALM",
			Value: "Registry Realm",
		},
		{
			Name:  "REGISTRY_AUTH_HTPASSWD_PATH",
			Value: "/etc/auth/htpasswd",
		},
		{
			Name:  "REGISTRY_HTTP_TLS_CERTIFICATE",
			Value: "/etc/auth/tls.crt",
		},
		{
			Name:  "REGISTRY_HTTP_TLS_KEY",
			Value: "/etc/auth/tls.key",
		},
	}

	// Configure the pod as a pull-through cache, if necessary. When proxy.remoteUrl on the registry config, it will
	// act as a proxy for the upstream registry and cache images it downloads, but you will not be able to push images
	// to it! For that, you'd need another registry.
	if pullThrough {
		podEnv = append(podEnv, corev1.EnvVar{
			Name:  "REGISTRY_PROXY_REMOTEURL",
			Value: "https://registry-1.docker.io",
		})
	}

	pod, err = clients.Core.Pod().Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: registryNamespace,
			Labels: map[string]string{
				"app": podName,
			},
		},
		Spec: corev1.PodSpec{
			EnableServiceLinks: new(bool),
			Volumes: []corev1.Volume{{
				Name: "htpasswd",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: podName,
					},
				},
			}},
			Containers: []corev1.Container{
				{
					Name:  "cache",
					Image: "registry",
					Env:   podEnv,
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "htpasswd",
						ReadOnly:  true,
						MountPath: "/etc/auth",
					}},
				},
			},
		},
	})
	if apierrors.IsAlreadyExists(err) {
		return clients.Core.Pod().Get(registryNamespace, podName, metav1.GetOptions{})
	}
	return pod, err
}

func createSharedObjects(clients *clients.Clients, podName string, pullThrough bool) (*corev1.Secret, error) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	registrySecret, err := createRegistrySecret(clients, podName)
	if err != nil {
		return nil, err
	}

	pod, err := createOrGetPod(clients, podName, pullThrough)
	if err != nil {
		return nil, err
	}

	err = createService(clients, podName)
	if err != nil {
		return nil, err
	}

	err = wait.Object(clients.Ctx, clients.Core.Pod().Watch, pod, func(obj runtime.Object) (bool, error) {
		pod := obj.(*corev1.Pod)
		return pod.Status.PodIP != "" && pod.Status.Phase == corev1.PodRunning, nil
	})
	return registrySecret, err
}

// CreateOrGetRegistry gets existing registry config, or creates a new one and returns the config. The registry will
// be created with the given name in the "default" namespace, and its secrets will be created in the given namespace so
// that pods created in that namespace can rely on images sourced from the new registry. If pullThrough is set to true,
// the registry will be configured as a proxy (a.k.a pull-through cache) for docker.io.
func CreateOrGetRegistry(clients *clients.Clients, namespace, name string, pullThrough bool) (rkev1.Registry, error) {
	registrySecret, err := createSharedObjects(clients, name, pullThrough)
	if err != nil {
		return rkev1.Registry{}, err
	}

	passwordSecret, err := createPasswordSecret(clients, namespace)
	if err != nil {
		return rkev1.Registry{}, err
	}

	tlsSecret, err := createTLSSecret(clients, namespace, registrySecret)
	if err != nil {
		return rkev1.Registry{}, err
	}

	serviceName := newRegistryServiceName(name)

	// Specify dummy.io registries to ensure we can deliver the same data twice without thrashing.
	return rkev1.Registry{
		Mirrors: map[string]rkev1.Mirror{
			"docker.io": {
				Endpoints: []string{
					fmt.Sprintf("https://%s:5000", serviceName),
				},
			},
			"dummy.cattle.io": {
				Endpoints: []string{
					"https://dummy.cattle.io",
				},
			},
		},
		Configs: map[string]rkev1.RegistryConfig{
			serviceName + ":5000": {
				AuthConfigSecretName: passwordSecret.Name,
				TLSSecretName:        tlsSecret.Name,
				CABundle:             tlsSecret.Data[corev1.TLSCertKey],
			},
			"dummy.cattle.io": {
				AuthConfigSecretName: passwordSecret.Name,
				TLSSecretName:        tlsSecret.Name,
				CABundle:             tlsSecret.Data[corev1.TLSCertKey],
			},
		},
	}, nil
}
