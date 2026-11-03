package objectstore

import (
	"encoding/base64"
	"fmt"
	"net"
	"sync"

	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/utils"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/randomtoken"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	objectStoreServiceNameTemplate = "%s.%s.svc.cluster.local"
	objectStoreLock                sync.Mutex
)

const (
	secretKeyCredAccessKey = "MINIO_ROOT_USER"
	secretKeyCredSecretKey = "MINIO_ROOT_PASSWORD"
	secretKeyTLSPublicCrt  = "public.crt"
	secretKeyTLSPrivateKey = "private.key"
)

const setupMinioBucket = `
#!/bin/sh

while ! curl -ksf https://127.0.0.1:9000/minio/health/live; do
    sleep 2
done

mc config host --insecure add myminio https://127.0.0.1:9000 $%s $%s

while true; do 
	mc ready --insecure myminio
	if [ $? = 0 ]; then 
		break; 
	fi; 
done; 

mc mb --insecure myminio/%s

sleep infinity
`

// createTLSSecret creates a TLS Secret with a self signed cert + key for the given service FQDN + IP,
// plus any extra IPs callers need in the SANs (see getObjectStore's external mode, where clients reach
// the store by node IP rather than through the Service).
func createTLSSecret(clients *clients.Clients, namespace, objectStore, serviceFQDN, serviceIP string, extraIPs []net.IP) (*corev1.Secret, error) {
	objectStoreTLSSecretName := objectStore + "-tls"
	secret, err := clients.Core.Secret().Get(namespace, objectStoreTLSSecretName, metav1.GetOptions{})
	if err == nil {
		return secret, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	cert, key, err := utils.GenerateSelfSignedCertKey(serviceFQDN, append([]net.IP{net.ParseIP(serviceIP)}, extraIPs...), nil)
	if err != nil {
		return nil, err
	}

	secret, err = clients.Core.Secret().Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectStoreTLSSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			secretKeyTLSPublicCrt:  cert,
			secretKeyTLSPrivateKey: key,
		},
	})
	if err == nil {
		clients.OnClose(func() {
			_ = clients.Core.Secret().Delete(secret.Namespace, secret.Name, &metav1.DeleteOptions{})
		})
		return secret, nil
	} else if apierrors.IsAlreadyExists(err) {
		return clients.Core.Secret().Get(namespace, objectStoreTLSSecretName, metav1.GetOptions{})
	}
	return nil, err
}

// createCredSecret creates a credential secret for the
func createCredSecret(clients *clients.Clients, namespace, objectStore string) (*corev1.Secret, error) {
	objectStoreCredSecretName := objectStore + "-cred"
	secret, err := clients.Core.Secret().Get(namespace, objectStoreCredSecretName, metav1.GetOptions{})
	if err == nil {
		return secret, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	accessKey, err := randomtoken.Generate()
	if err != nil {
		return nil, err
	}
	accessKey = accessKey[:16]

	secretKey, err := randomtoken.Generate()
	if err != nil {
		return nil, err
	}

	secret, err = clients.Core.Secret().Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectStoreCredSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			secretKeyCredAccessKey: []byte(accessKey),
			secretKeyCredSecretKey: []byte(secretKey),
		},
	})
	if err == nil {
		clients.OnClose(func() {
			_ = clients.Core.Secret().Delete(secret.Namespace, secret.Name, &metav1.DeleteOptions{})
		})
		return secret, nil
	} else if apierrors.IsAlreadyExists(err) {
		return clients.Core.Secret().Get(namespace, objectStoreCredSecretName, metav1.GetOptions{})
	}
	return nil, err
}

func createCloudCredentialSecret(clients *clients.Clients, namespace, name string, credentials *corev1.Secret) (*corev1.Secret, error) {
	cc, err := clients.Core.Secret().Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"accessKey": credentials.Data[secretKeyCredAccessKey],
			"secretKey": credentials.Data[secretKeyCredSecretKey],
		},
	})
	if apierrors.IsAlreadyExists(err) {
		return clients.Core.Secret().Get(namespace, name, metav1.GetOptions{})
	} else if err == nil {
		clients.OnClose(func() {
			_ = clients.Core.Secret().Delete(namespace, name, &metav1.DeleteOptions{})
		})
	}
	return cc, err
}

func createHelperConfigmap(clients *clients.Clients, namespace, objectStore, bucketName string) (*corev1.ConfigMap, error) {
	objectStoreHelperCMName := objectStore + "-helper"
	cm, err := clients.Core.ConfigMap().Get(namespace, objectStoreHelperCMName, metav1.GetOptions{})
	if err == nil {
		return cm, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	cm, err = clients.Core.ConfigMap().Create(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectStoreHelperCMName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"setup.sh": fmt.Sprintf(setupMinioBucket, secretKeyCredAccessKey, secretKeyCredSecretKey, bucketName),
		},
	})
	if apierrors.IsAlreadyExists(err) {
		return clients.Core.ConfigMap().Get(namespace, objectStoreHelperCMName, metav1.GetOptions{})
	} else if err == nil {
		clients.OnClose(func() {
			_ = clients.Core.ConfigMap().Delete(cm.Namespace, cm.Name, &metav1.DeleteOptions{})
		})
	}
	return cm, err
}

// createService exposes the object store. serviceType is NodePort when the store has to be reachable
// from outside the local cluster's network, and ClusterIP otherwise.
func createService(clients *clients.Clients, namespace, objectStore string, serviceType corev1.ServiceType) (*corev1.Service, error) {
	svc, err := clients.Core.Service().Create(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectStore,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: serviceType,
			Ports: []corev1.ServicePort{{
				Name:        "http",
				Protocol:    corev1.ProtocolTCP,
				AppProtocol: &[]string{"https"}[0],
				Port:        9000,
				TargetPort:  intstr.FromInt32(9000),
			}},
			Selector: map[string]string{
				"app": objectStore,
			},
		},
	})
	if apierrors.IsAlreadyExists(err) {
		svc, err = clients.Core.Service().Get(namespace, objectStore, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}

	clients.OnClose(func() {
		_ = clients.Core.Service().Delete(svc.Namespace, svc.Name, &metav1.DeleteOptions{})
	})

	err = wait.Object(clients.Ctx, clients.Core.Service().Watch, svc, func(obj runtime.Object) (bool, error) {
		latestSvc := obj.(*corev1.Service)
		if latestSvc.Spec.ClusterIP == "" {
			return false, nil
		}
		// A NodePort service is only usable once the port has actually been allocated.
		if serviceType == corev1.ServiceTypeNodePort && latestSvc.Spec.Ports[0].NodePort == 0 {
			return false, nil
		}
		svc = latestSvc
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	return svc, err
}

func getPod(clients *clients.Clients, namespace, objectStore string) (*corev1.Pod, error) {
	objectStoreCredSecretName := objectStore + "-cred"
	objectStoreTLSSecretName := objectStore + "-tls"
	objectStoreHelperCMName := objectStore + "-helper"
	pod, err := clients.Core.Pod().Get(namespace, objectStore, metav1.GetOptions{})
	if err == nil || !apierrors.IsNotFound(err) {
		return pod, err
	}

	pod, err = clients.Core.Pod().Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectStore,
			Namespace: namespace,
			Labels: map[string]string{
				"app": objectStore,
			},
		},
		Spec: corev1.PodSpec{
			EnableServiceLinks: new(bool),
			Volumes: []corev1.Volume{
				{
					Name: "tls",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: objectStoreTLSSecretName,
						},
					},
				},
				{
					Name: "cred",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: objectStoreCredSecretName,
						},
					},
				},
				{
					Name: "helper",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: objectStoreHelperCMName,
							},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "minio",
					Image: defaults.ObjectStoreServerImage,
					Env:   []corev1.EnvVar{},
					Command: []string{
						"/bin/bash",
						"-c",
					},
					Args: []string{
						"minio server /data --certs-dir /etc/minio/tls",
					},
					EnvFrom: []corev1.EnvFromSource{
						{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: objectStoreCredSecretName,
								},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "tls",
							MountPath: "/etc/minio/tls/" + secretKeyTLSPrivateKey,
							SubPath:   secretKeyTLSPrivateKey,
						},
						{
							Name:      "tls",
							MountPath: "/etc/minio/tls/" + secretKeyTLSPublicCrt,
							SubPath:   secretKeyTLSPublicCrt,
						},
					},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 9000,
						},
					},
				},
				{
					Name:  "setupbucket",
					Image: defaults.ObjectStoreUtilImage,
					Command: []string{
						"/bin/bash",
						"-x",
					},
					EnvFrom: []corev1.EnvFromSource{
						{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: objectStoreCredSecretName,
								},
							},
						},
					},
					Args: []string{
						"/helper/setup.sh",
					},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "helper",
						ReadOnly:  true,
						MountPath: "/helper",
					}},
				},
			},
		},
	})
	if apierrors.IsAlreadyExists(err) {
		return clients.Core.Pod().Get(namespace, objectStore, metav1.GetOptions{})
	} else if err == nil {
		clients.OnClose(func() {
			_ = clients.Core.Pod().Delete(pod.Namespace, pod.Name, &metav1.DeleteOptions{})
		})
	}
	return pod, err
}

type Info struct {
	AccessKey, SecretKey, Bucket, Endpoint, Cert, CloudCredentialName string
}

func GetObjectStore(clients *clients.Clients, namespace, identifier, bucket string) (Info, error) {
	return getObjectStore(clients, namespace, identifier, bucket, false)
}

// GetExternalObjectStore is GetObjectStore for machines that are not pods in the local cluster and so
// cannot reach a ClusterIP — CAPD machines, for instance, are Docker containers on the same bridge
// network as the local cluster's nodes but outside its pod network. The store is exposed on a
// NodePort and the returned Endpoint addresses a node directly; every node IP is added to the
// certificate's SANs so the returned Cert still verifies.
//
// Callers must not share an identifier with GetObjectStore: the two modes differ in Service type and
// certificate, and the per-identifier objects are created once and then reused.
func GetExternalObjectStore(clients *clients.Clients, namespace, identifier, bucket string) (Info, error) {
	return getObjectStore(clients, namespace, identifier, bucket, true)
}

// nodeIPs returns the InternalIP of every node in the local cluster, which is what an external client
// dials a NodePort on.
func nodeIPs(clients *clients.Clients) ([]net.IP, error) {
	nodes, err := clients.Core.Node().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var ips []net.IP
	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type != corev1.NodeInternalIP {
				continue
			}
			if ip := net.ParseIP(addr.Address); ip != nil {
				ips = append(ips, ip)
			}
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no node in the local cluster reports an InternalIP")
	}
	return ips, nil
}

func getObjectStore(clients *clients.Clients, namespace, identifier, bucket string, external bool) (Info, error) {
	objectStoreLock.Lock()
	defer objectStoreLock.Unlock()
	hid := name.Hex(identifier, 5)
	objectStore := name.SafeConcatName("objectstore", hid)

	cs, err := createCredSecret(clients, namespace, objectStore)
	if err != nil {
		return Info{}, err
	}

	ccName := name.SafeConcatName("cc", objectStore)
	cc, err := createCloudCredentialSecret(clients, namespace, ccName, cs)
	if err != nil {
		return Info{}, err
	}

	_, err = createHelperConfigmap(clients, namespace, objectStore, bucket)
	if err != nil {
		return Info{}, err
	}

	serviceType := corev1.ServiceTypeClusterIP
	var extraIPs []net.IP
	if external {
		serviceType = corev1.ServiceTypeNodePort
		if extraIPs, err = nodeIPs(clients); err != nil {
			return Info{}, err
		}
	}

	svc, err := createService(clients, namespace, objectStore, serviceType)
	if err != nil {
		return Info{}, err
	}

	tls, err := createTLSSecret(clients, namespace, objectStore, fmt.Sprintf(objectStoreServiceNameTemplate, objectStore, namespace), svc.Spec.ClusterIP, extraIPs)
	if err != nil {
		return Info{}, err
	}

	endpoint := fmt.Sprintf("%s:9000", svc.Spec.ClusterIP)
	if external {
		endpoint = fmt.Sprintf("%s:%d", extraIPs[0], svc.Spec.Ports[0].NodePort)
	}

	pod, err := getPod(clients, namespace, objectStore)
	if err != nil {
		return Info{}, err
	}

	err = wait.Object(clients.Ctx, clients.Core.Pod().Watch, pod, func(obj runtime.Object) (bool, error) {
		pod := obj.(*corev1.Pod)
		return pod.Status.PodIP != "" && pod.Status.Phase == corev1.PodRunning, nil
	})
	if err != nil {
		return Info{}, err
	}

	return Info{
		AccessKey:           string(cs.Data[secretKeyCredAccessKey]),
		SecretKey:           string(cs.Data[secretKeyCredSecretKey]),
		Bucket:              bucket,
		Endpoint:            endpoint,
		Cert:                base64.StdEncoding.EncodeToString(tls.Data[secretKeyTLSPublicCrt]),
		CloudCredentialName: cc.Name,
	}, nil
}
