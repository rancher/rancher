package custom

import (
	"context"
	"strings"
	"testing"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/dashboardapi/settings"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/systemdnode"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

func Test_Operation_Custom_CertificateRotation(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-certificate-rotation-operations",
		},
		Spec: provisioningv1.ClusterSpec{
			RKEConfig: &provisioningv1.RKEConfig{},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	command, err := cluster.CustomCommand(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, command)

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --controlplane", map[string]string{"custom-cluster-name": c.Name}, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd", map[string]string{"custom-cluster-name": c.Name}, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker", map[string]string{"custom-cluster-name": c.Name}, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	wContext, err := wrangler.NewContext(context.TODO(), clients.ClientConfig, clients.RESTConfig)
	if err != nil {
		t.Fatal(err)
	}

	// Register settings so that the provider is set and we can retrieve the internal server URL + CA for the kubeconfig manager below.
	if err := settings.Register(wContext.Mgmt.Setting()); err != nil {
		t.Fatal(err)
	}
	kcManager := kubeconfig.New(wContext)

	// Get kubeconfig for the downstream cluster to create test resources
	restConfig, err := kcManager.GetRESTConfig(c, c.Status)
	if err != nil {
		t.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatal(err)
	}

	// Try to continuously get the kubernetes default service because the API server will be flapping at this point.
	err = retry.OnError(retry.DefaultRetry, func(err error) bool {
		if strings.Contains(err.Error(), "connection refused") || apierrors.IsServiceUnavailable(err) {
			return true
		}
		return false
	}, func() error {
		_, err = clientset.CoreV1().Services(corev1.NamespaceDefault).Get(context.TODO(), "kubernetes", metav1.GetOptions{})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = clientset.CoreV1().ConfigMaps(corev1.NamespaceDefault).Create(context.TODO(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "myspecialconfigmap",
		},
		Data: map[string]string{
			"test": "wow",
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	configMap, err := clientset.CoreV1().ConfigMaps(corev1.NamespaceDefault).Get(context.TODO(), "myspecialconfigmap", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	assert.NotNil(t, configMap)
}
