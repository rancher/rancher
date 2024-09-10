//go:build (validation || infra.rke1 || cluster.any || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !sanity && !extended

package charts

import (
	"strings"

	settings "github.com/rancher/rancher/pkg/settings"
	namespaces "github.com/rancher/rancher/tests/v2/actions/namespaces"
	"github.com/rancher/shepherd/pkg/environmentflag"
	"github.com/stretchr/testify/require"
)

func (n *GateKeeperTestSuite) TestGateKeeperAllowedNamespacesPostUpgrade() {

	subSession := n.session.NewSession()
	defer subSession.Cleanup()

	client, err := n.client.WithSession(subSession)
	require.NoError(n.T(), err)

	if !client.Flags.GetValue(environmentflag.GatekeeperAllowedNamespaces) {
		n.T().Skip("skipping TestGateKeeperAllowedNamespacesPostUpgrade because GatekeeperAllowedNamespaces flag not set in cattle config")
	}

	sysNamespaces := settings.SystemNamespaces.Get()

	sysNamespacesSlice := strings.Split(sysNamespaces, ",")
	for _, namespace := range sysNamespacesSlice {
		_, err = namespaces.CreateNamespace(client, namespace, "{}", map[string]string{}, map[string]string{}, n.project)
		if err != nil {
			errString := "namespaces \"" + namespace + "\" already exists"
			require.ErrorContains(n.T(), err, errString)
		}
	}
}
