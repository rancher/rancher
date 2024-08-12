package helm

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/rancher/shepherd/clients/helm"
	"github.com/rancher/shepherd/pkg/session"
)

// InstallRancher installs latest version of rancher including cert-manager
// using helm CLI with some predefined values set such as
// - BootstrapPassword : admin
// - Hostname          : Localhost
// - BundledMode       : True
// - Replicas          : 1
func InstallRancher(ts *session.Session, restConfig *rest.Config) error {
	//  ClientSet of kubernetes
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	// Create namespace cattle-system
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "cattle-system"}}
	_, err = clientset.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Install cert-manager chart
	err = InstallCertManager(ts, restConfig)
	if err != nil {
		return err
	}

	// Add Rancher helm repo
	err = helm.AddHelmRepo("rancher-stable", "https://releases.rancher.com/server-charts/stable")
	if err != nil {
		return err
	}

	// Install Rancher Chart
	err = helm.InstallChart(ts, "rancher",
		"rancher-stable/rancher",
		"cattle-system",
		"",
		"--set",
		"hostname=localhost",
		"--set",
		"bootstrapPassword=admin",
		"--set",
		"useBundledSystemChart=true",
		"--set",
		"replicas=1")
	if err != nil {
		return err
	}

	return nil
}

// InstallCertManager installs latest version cert manager available through helm
// CLI. It sets the installCRDs as true to install crds as well.
func InstallCertManager(ts *session.Session, restConfig *rest.Config) error {
	//  ClientSet of kubernetes
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	// Create namespace cert-manager
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "cert-manager"}}
	_, err = clientset.CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Add cert-manager Helm Repo
	err = helm.AddHelmRepo("jetstack", "https://charts.jetstack.io")
	if err != nil {
		return err
	}

	// Install cert-manager Chart
	err = helm.InstallChart(ts, "cert-manager", "jetstack/cert-manager", "cert-manager", "", "--set", "installCRDs=true")
	if err != nil {
		return err
	}

	return nil
}
