//go:build (validation || infra.rke1 || cluster.any || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !sanity && !extended

package charts

import (
	"os"
	"strings"

	settings "github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/tests/v2/actions/charts"
	namespaces "github.com/rancher/rancher/tests/v2/actions/namespaces"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extencharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/pkg/environmentflag"
	"github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (n *GateKeeperTestSuite) TestGateKeeperAllowedNamespaces() {
	subSession := n.session.NewSession()
	defer subSession.Cleanup()

	client, err := n.client.WithSession(subSession)
	require.NoError(n.T(), err)

	if !client.Flags.GetValue(environmentflag.GatekeeperAllowedNamespaces) {
		n.T().Skip("skipping TestGateKeeperAllowedNamespaces because GatekeeperAllowedNamespaces flag not set in cattle config")
	}

	n.T().Log("Installing latest version of gatekeeper chart")
	err = charts.InstallRancherGatekeeperChart(client, n.gatekeeperChartInstallOptions)
	require.NoError(n.T(), err)

	n.T().Log("Waiting for gatekeeper chart deployments to have expected number of available replicas")
	err = extencharts.WatchAndWaitDeployments(client, n.project.ClusterID, charts.RancherGatekeeperNamespace, metav1.ListOptions{})
	require.NoError(n.T(), err)

	n.T().Log("Waiting for gatekeeper chart DaemonSets to have expected number of available nodes")
	err = extencharts.WatchAndWaitDaemonSets(client, n.project.ClusterID, charts.RancherGatekeeperNamespace, metav1.ListOptions{})
	require.NoError(n.T(), err)

	n.T().Log("creating constraint template")
	readTemplateYamlFile, err := os.ReadFile("./resources/opa-allowednamespacestemplate.yaml")
	require.NoError(n.T(), err)
	yamlTemplateInput := &management.ImportClusterYamlInput{
		DefaultNamespace: charts.RancherGatekeeperNamespace,
		YAML:             string(readTemplateYamlFile),
	}

	n.T().Log("getting list of all namespaces")
	sysNamespaces := settings.SystemNamespaces.Get()
	sysNamespacesSlice := strings.Split(sysNamespaces, ",")

	// constraint must exclude cattle-gatekeeper-system and all namespaces that are dynamically generated during gatekeeper installation and upgrade
	// for example: pod-impersonation-helm-op-f9pwc and cattle-impersonation-user-vhkst-token
	n.T().Log("creating constraint")
	yamlString, err := extencharts.GenerateGatekeeperConstraintYaml([]string{""},
		[]string{"cattle-gatekeeper-system", "ingress-nginx-controller-admission", "kube-dns", "cattle-controllers", "rke-network-plugin", "extension-apiserver-authentication", "fleet-agent-lock", "udp-services", "cattle-impersonation-user*", "pod-impersonation-helm-op*", "local*", "default*", "test*", "canal*", "nginx-ingress-controller*", "rke*", "coredns*", "gatekeeper*", "calico-kube-controllers*", "kube-root*", "cattle*", "kube-root-ca*", "ingress-nginx-admission*", "metrics-server*"},
		[]string{"Namespace"},
		"ns-must-be-allowed", sysNamespacesSlice, "deny", "constraints.gatekeeper.sh/v1beta1", "K8sAllowedNamespaces")
	require.NoError(n.T(), err)
	n.T().Log(yamlString)

	yamlConstraintInput := &management.ImportClusterYamlInput{
		DefaultNamespace: charts.RancherGatekeeperNamespace,
		YAML:             yamlString,
	}

	// get the cluster
	cluster, err := client.Management.Cluster.ByID(n.project.ClusterID)
	require.NoError(n.T(), err)

	n.T().Log("applying constraint template")
	_, err = client.Management.Cluster.ActionImportYaml(cluster, yamlTemplateInput)
	require.NoError(n.T(), err)

	n.T().Log("applying constraint")
	// Use ActionImportYaml to the apply the constraint yaml file
	_, err = client.Management.Cluster.ActionImportYaml(cluster, yamlConstraintInput)
	require.NoError(n.T(), err)

	n.T().Log("Create a namespace that doesn't have an allowed name and assert that creation fails with the expected error")
	_, err = namespaces.CreateNamespace(client, RancherDisallowedNamespace, "{}", map[string]string{}, map[string]string{}, n.project)
	assert.ErrorContains(n.T(), err, "admission webhook \"validation.gatekeeper.sh\" denied the request: [ns-must-be-allowed] Namespace not allowed")

}
