package operations

import (
	"context"
	"strings"
	"testing"

	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// RunCertificateRotationTest expects a "created" v2prov cluster and will run a certificate rotation on it and perform assertions.
func RunCertificateRotationTest(t *testing.T, clients *clients.Clients, c *v1.Cluster) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newC.Spec.RKEConfig.RotateCertificates = &rkev1.RotateCertificates{
			Generation: 1,
		}
		newC, err = clients.Provisioning.Cluster().Update(newC)
		if err != nil {
			return err
		}
		c = newC
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForControlPlane(clients, c, "rotate certificates", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return rkeControlPlane.Status.CertificateRotationGeneration == 1 && capr.Reconciled.IsTrue(rkeControlPlane.Status), nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	clientset, err := GetAndVerifyDownstreamClientset(clients, c)
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
