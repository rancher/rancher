package helm

import (
	"context"
	"os/exec"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var helmCmd = "helm_v3"

func InstallRancher(restConfig *rest.Config) error {
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
	err = InstallCertManager(restConfig)
	if err != nil {
		return err
	}

	// Add Rancher helm repo
	err = AddHelmRepo("rancher-stable", "https://releases.rancher.com/server-charts/stable")
	if err != nil {
		return err
	}

	// Install Rancher Chart
	err = InstallChart("rancher",
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

func InstallCertManager(restConfig *rest.Config) error {
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
	err = AddHelmRepo("jetstack", "https://charts.jetstack.io")
	if err != nil {
		return err
	}

	// Install cert-manager Chart
	err = InstallChart("cert-manager", "jetstack/cert-manager", "cert-manager", "", "--set", "installCRDs=true")
	if err != nil {
		return err
	}

	return nil
}

// Install a helm chart
func InstallChart(releaseName, helmRepo, namespace, version string, args ...string) error {
	// Default helm install command
	commandArgs := []string{
		"install",
		releaseName,
		helmRepo,
		"--namespace",
		namespace,
		"--wait",
	}

	commandArgs = append(commandArgs, args...)

	if version != "" {
		commandArgs = append(commandArgs, "--version", version)
	}

	msg, err := exec.Command(helmCmd, commandArgs...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "InstallChart: "+string(msg))
	}

	return nil
}

// Upgrade a helm chart
func UpgradeChart(releaseName, helmRepo, namespace, version string, args ...string) error {
	// Default helm install command
	commandArgs := []string{
		"upgrade",
		releaseName,
		helmRepo,
		"--namespace",
		namespace,
		"--wait",
	}

	commandArgs = append(commandArgs, args...)

	if version != "" {
		commandArgs = append(commandArgs, "--version", version)
	}

	msg, err := exec.Command(helmCmd, commandArgs...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "UpgradeChart: "+string(msg))
	}

	return nil
}

func AddHelmRepo(name, url string) error {
	msg, err := exec.Command(helmCmd, "repo", "add", name, url).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, "AddHelmRepo: "+string(msg))
	}

	return nil
}
