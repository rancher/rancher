package imported

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

// curlPeerFailedVerification is curl's exit code when the server's certificate does not verify
// (CURLE_PEER_FAILED_VERIFICATION) — which includes a target the certificate's SANs do not cover.
// Unlike curl's message, it is stable across curl versions.
const curlPeerFailedVerification = 60

// Test_Imported_Operation_SetD_ImportedServerJoinOutsideCertificateSANs is the negative control for
// assertRestoreServerArgsTrusted: it shows that a server cannot join through an address its peer's
// certificate does not list, and that probeServerTLS reports exactly that rather than passing.
//
// The init server runs IPv6-only inside a dual-stack pod, so its pod also has an IPv4 address that
// is not one of its node-ips, and the second server is told to join through that IPv4 address. That
// is the situation a restore creates if it points rejoining nodes at an address the leader never
// certified.
//
// # Timing
//
// Whether such an address is rejected depends on when it is first used. The distro's serving
// certificate is managed by dynamiclistener, which adds the local address of every incoming
// connection to the certificate until the server's SAN filter is running (K3s/RKE2
// tls-san-security); the filter starts some seconds after the server, when it also persists the
// certificate as the <runtime>-serving secret. An address first connected to inside that window is
// certified for good, and the join through it succeeds. So the test holds the joiner back, via
// BeforeJoin, until the serving secret exists, and checks the certificate before and after the
// joiner tries — without that, the result is a race between the joiner's install and the filter.
//
// The same window reopens on a restore, since the restore removes the leader's TLS directory and
// the leader starts afresh: rejoining nodes that reach it early can get an uncertified address added
// rather than rejected.
//
// It needs no Rancher operation, just two downstream servers. Needs a dual-stack host cluster, and
// skips without one: see dev-scripts/k3d-dual-stack.
func Test_Imported_Operation_SetD_ImportedServerJoinOutsideCertificateSANs(t *testing.T) {
	cs, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	requireHostIPFamilies(t, cs, []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol})

	ns, err := namespace.Random(cs)
	require.NoError(t, err)
	registryCACert, err := registry.EnsureRegistryCache(cs)
	require.NoError(t, err)

	version := defaults.SomeK8sVersion
	runtime := capr.GetRuntime(version)
	dataDir := fmt.Sprintf("/var/lib/rancher/%s", runtime)
	port := strconv.Itoa(capr.GetRuntimeSupervisorPort(version))

	var certifiedURL, uncertifiedURL, uncertifiedIP string
	pods, err := cluster.NewImportedClusterPods(cs, ns.Name, version,
		[]cluster.ImportedNodePool{{ETCD: true, ControlPlane: true, Quantity: 2}},
		nil, registryCACert,
		cluster.ImportedClusterOptions{
			ExtractServerToken: true,
			Network: &cluster.ImportedNetwork{
				IPFamilies: []corev1.IPFamily{corev1.IPv6Protocol},
				JoinFamily: corev1.IPv4Protocol,
			},
			BeforeJoin: func(initPod *corev1.Pod) error {
				certifiedURL = "https://" + net.JoinHostPort(cluster.PodIPForFamily(initPod, corev1.IPv6Protocol), port)
				uncertifiedIP = cluster.PodIPForFamily(initPod, corev1.IPv4Protocol)
				uncertifiedURL = "https://" + net.JoinHostPort(uncertifiedIP, port)

				waitForServingCertFiltered(t, cs, ns.Name, initPod.Name, runtime)

				// Positive control: the init server is trusted on its node-ip. Without this, the
				// failure below could just as well mean the server is not serving.
				require.NoError(t, probeServerTLS(cs, ns.Name, initPod.Name, dataDir, certifiedURL),
					"init server should be trusted on its node-ip %s", certifiedURL)

				// The same server, through the address its certificate omits, does not verify. This
				// is the first connection to that address, made after the SAN filter is running, so
				// it must not have been added.
				assertProbeRejectedBySAN(t, cs, ns.Name, initPod.Name, dataDir, uncertifiedURL)
				assertServingCertOmits(t, cs, ns.Name, initPod.Name, runtime, uncertifiedIP)
				return nil
			},
		})
	require.NoError(t, err)

	fx := &importedClusterFixture{ns: ns, pods: pods}
	nodes := importedNodes(t, cs, fx)
	require.Len(t, nodes, 2)
	initNode, joiner := nodes[0], nodes[1]

	// The joiner, told to use that address, is rejected the same way by the distro itself.
	unit := capr.GetRuntimeServerUnit(version)
	var journal string
	err = utilwait.PollUntilContextTimeout(context.TODO(), 10*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
		out, err := cluster.ExecOnPod(cs, ns.Name, joiner.pod.Name, "sh", "-c", fmt.Sprintf(
			"journalctl -u %s --no-pager -o cat 2>/dev/null | grep -F 'x509: certificate is valid for' | tail -n1", unit))
		journal = strings.TrimSpace(out)
		return err == nil && strings.Contains(journal, uncertifiedIP), nil
	})
	require.NoError(t, err, "joiner %s never reported a certificate rejection for %s (last matching %s log line: %q)",
		joiner.name, uncertifiedURL, unit, journal)
	t.Logf("joiner %s rejected as expected: %s", joiner.name, journal)

	// Its attempts did not get the address added, the probe still fails, and it never made it into
	// the cluster.
	assertServingCertOmits(t, cs, ns.Name, initNode.pod.Name, runtime, uncertifiedIP)
	assertProbeRejectedBySAN(t, cs, ns.Name, initNode.pod.Name, dataDir, uncertifiedURL)

	members, err := readETCDMembers(t, cs, fx, initNode.pod, dataDir, capr.GetRuntimeSupervisorPort(version))
	require.NoError(t, err)
	require.Len(t, members, 1, "the joiner should not have become an etcd member: %+v", members)
	assert.True(t, strings.HasPrefix(members[0].Name, initNode.name+"-"),
		"the only etcd member should be the init node %s, got %s", initNode.name, members[0].Name)
}

// distroKubectl returns a shell prefix that runs kubectl against the server's own API on a server
// node of runtime.
func distroKubectl(runtime string) string {
	binDir := "/usr/local/bin"
	if runtime == capr.RuntimeRKE2 {
		binDir = "/var/lib/rancher/rke2/bin"
	}
	return fmt.Sprintf("KUBECONFIG=/etc/rancher/%[1]s/%[1]s.yaml %[2]s/kubectl", runtime, binDir)
}

// waitForServingCertFiltered waits until the server on pod persists its serving certificate as the
// <runtime>-serving secret. The distro starts its SAN filter immediately before creating that secret,
// so from then on the certificate only gains addresses the filter allows.
func waitForServingCertFiltered(t *testing.T, cs *clients.Clients, namespace, pod, runtime string) {
	t.Helper()

	var lastOut string
	err := utilwait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 10*time.Minute, true, func(context.Context) (bool, error) {
		out, err := cluster.ExecOnPod(cs, namespace, pod, "sh", "-c", fmt.Sprintf(
			"%s get secret -n kube-system %s-serving -o name", distroKubectl(runtime), runtime))
		lastOut = strings.TrimSpace(out)
		return err == nil && lastOut != "", nil
	})
	require.NoError(t, err, "the %s-serving secret never appeared on %s (last output %q)", runtime, pod, lastOut)
}

// assertServingCertOmits fails if the server on pod has certified ip, according to the
// listener.cattle.io/cn-* annotations dynamiclistener records on the <runtime>-serving secret.
func assertServingCertOmits(t *testing.T, cs *clients.Clients, namespace, pod, runtime, ip string) {
	t.Helper()

	out, err := cluster.ExecOnPod(cs, namespace, pod, "sh", "-c", fmt.Sprintf(
		"%s get secret -n kube-system %s-serving -o jsonpath='{.metadata.annotations}'", distroKubectl(runtime), runtime))
	require.NoError(t, err, "reading the %s-serving secret on %s: %s", runtime, pod, out)
	assert.NotContains(t, out, `":"`+ip+`"`, "the %s serving certificate should not list %s: %s", pod, ip, out)
}

// assertProbeRejectedBySAN fails unless probeServerTLS of serverURL from pod fails on certificate
// verification specifically.
func assertProbeRejectedBySAN(t *testing.T, cs *clients.Clients, namespace, pod, dataDir, serverURL string) {
	t.Helper()

	err := probeServerTLS(cs, namespace, pod, dataDir, serverURL)
	require.Error(t, err, "probe of %s should fail certificate verification", serverURL)
	assert.Contains(t, err.Error(), fmt.Sprintf("exit code %d", curlPeerFailedVerification),
		"probe of %s should fail on certificate verification, not on something else", serverURL)
}
