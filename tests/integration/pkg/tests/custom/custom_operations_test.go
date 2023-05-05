//go:build operations
// +build operations

package custom

import (
	"context"
	"os"
	"strings"
	"testing"

	provisioningv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/dashboardapi/settings"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/rancher/tests/integration/pkg/clients"
	"github.com/rancher/rancher/tests/integration/pkg/cluster"
	"github.com/rancher/rancher/tests/integration/pkg/systemdnode"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

func TestCustomEtcdSnapshotOperations(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-etcd-snapshot-operations",
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

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker --controlplane", map[string]string{"custom-cluster-name": c.Name})
	if err != nil {
		t.Fatal(err)
	}

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd", map[string]string{"custom-cluster-name": c.Name})
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

	// Create a configmap called "myspecialconfigmap" that we will delete after taking a snapshot.
	_, err = clientset.CoreV1().ConfigMaps("default").Create(context.TODO(), &corev1.ConfigMap{
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

	// Create an etcd snapshot
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newC.Spec.RKEConfig.ETCDSnapshotCreate = &rkev1.ETCDSnapshotCreate{
			Generation: 1,
		}
		newC, err = clients.Provisioning.Cluster().Update(newC)
		if err != nil {
			return err
		}
		c = newC
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForControlPlane(clients, c, "etcd snapshot creation", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return rkeControlPlane.Status.ETCDSnapshotCreatePhase == rkev1.ETCDSnapshotPhaseFinished, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	var snapshot *rkev1.ETCDSnapshot
	// Get the etcd snapshot object
	if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		if apierrors.IsNotFound(err) || err == nil {
			return true
		}
		return false
	},
		func() error {
			snapshots, err := clients.RKE.ETCDSnapshot().List(c.Namespace, metav1.ListOptions{})
			if err != nil || len(snapshots.Items) == 0 {
				return err
			}
			snapshot = snapshots.Items[0].DeepCopy()
			return nil
		}); err != nil {
		t.Fatal(err)
	}

	assert.NotNil(t, snapshot)

	err = clientset.CoreV1().ConfigMaps(corev1.NamespaceDefault).Delete(context.TODO(), "myspecialconfigmap", metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the configmap no longer exists
	configMap, expectedErr := clientset.CoreV1().ConfigMaps(corev1.NamespaceDefault).Get(context.TODO(), "myspecialconfigmap", metav1.GetOptions{})
	if !apierrors.IsNotFound(expectedErr) {
		t.Fatal(expectedErr)
	}

	// The client will return a configmap object but it will not have anything populated.
	assert.Equal(t, "", configMap.Name)

	// Restore the snapshot!
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newC.Spec.RKEConfig.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{
			Name:             snapshot.Name,
			Generation:       1,
			RestoreRKEConfig: "none",
		}
		newC, err = clients.Provisioning.Cluster().Update(newC)
		if err != nil {
			return err
		}
		c = newC
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForControlPlane(clients, c, "etcd snapshot restore", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return rkeControlPlane.Status.ETCDSnapshotRestorePhase == rkev1.ETCDSnapshotPhaseFinished, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	// Check for the configmap!
	configMap, err = clientset.CoreV1().ConfigMaps(corev1.NamespaceDefault).Get(context.TODO(), "myspecialconfigmap", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	assert.NotNil(t, configMap)
}

func TestCustomCertificateRotationOperation(t *testing.T) {
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

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --controlplane", map[string]string{"custom-cluster-name": c.Name})
	if err != nil {
		t.Fatal(err)
	}

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd", map[string]string{"custom-cluster-name": c.Name})
	if err != nil {
		t.Fatal(err)
	}

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker", map[string]string{"custom-cluster-name": c.Name})
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	// Rotate certificates
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
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
	}); err != nil {
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

func TestCustomEncryptionKeyRotationOperation(t *testing.T) {
	// Encryption Key rotation is only possible with "stock configuration" on RKE2.
	if strings.ToLower(os.Getenv("DIST")) != "rke2" {
		t.Skip()
	}
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	c, err := cluster.New(clients, &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-custom-encryption-key-rotation-operations",
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

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --controlplane", map[string]string{"custom-cluster-name": c.Name})
	if err != nil {
		t.Fatal(err)
	}

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --etcd", map[string]string{"custom-cluster-name": c.Name})
	if err != nil {
		t.Fatal(err)
	}

	_, err = systemdnode.New(clients, c.Namespace, "#!/usr/bin/env sh\n"+command+" --worker", map[string]string{"custom-cluster-name": c.Name})
	if err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForCreate(clients, c)
	if err != nil {
		t.Fatal(err)
	}

	// Rotate encryption keys
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		newC, err := clients.Provisioning.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newC.Spec.RKEConfig.RotateEncryptionKeys = &rkev1.RotateEncryptionKeys{
			Generation: 1,
		}
		newC, err = clients.Provisioning.Cluster().Update(newC)
		if err != nil {
			return err
		}
		c = newC
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	_, err = cluster.WaitForControlPlane(clients, c, "rotate encryption keys", func(rkeControlPlane *rkev1.RKEControlPlane) (bool, error) {
		return rkeControlPlane.Status.RotateEncryptionKeys != nil && rkeControlPlane.Status.RotateEncryptionKeys.Generation == 1 && capr.Reconciled.IsTrue(rkeControlPlane.Status), nil
	})
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
