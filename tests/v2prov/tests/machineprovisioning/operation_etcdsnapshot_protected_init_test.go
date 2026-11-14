package machineprovisioning

import (
	"fmt"
	"strings"
	"testing"
	"time"

	provisioningv1api "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/objectstore"
	"github.com/rancher/rancher/tests/v2prov/operations"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/external"
)

const (
	initProtectedPreTerminateHook = capi.PreTerminateDeleteHookAnnotationPrefix + "/rke-bootstrap-cleanup"
	initProtectedEtcdPeerPort     = "2380"
	initProtectedVoterCondition   = "EtcdIsVoter"
)

// Test_Provisioning_SetB_MP_EtcdRestoreWhileInitProtected deletes the last etcd Machine D while its replacement E
// cannot reach D's etcd peer port. D must stay protected and E must never become the init node. A legacy S3 restore
// must then force-remove D, restore onto E, and bring back the snapshot data.
func Test_Provisioning_SetB_MP_EtcdRestoreWhileInitProtected(t *testing.T) {
	clients, err := clients.New()
	require.NoError(t, err)
	defer clients.Close()

	newNs, err := namespace.Random(clients)
	require.NoError(t, err)

	osInfo, err := objectstore.GetObjectStore(clients, newNs.Name, "store0", "s3snapshots")
	require.NoError(t, err)

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mp-etcd-restore-init-protected",
			Namespace: newNs.Name,
		},
		Spec: provisioningv1api.ClusterSpec{
			RKEConfig: &provisioningv1api.RKEConfig{
				ClusterConfiguration: rkev1.ClusterConfiguration{
					ETCD: &rkev1.ETCD{
						DisableSnapshots: true,
						S3: &rkev1.ETCDSnapshotS3{
							Endpoint:            osInfo.Endpoint,
							EndpointCA:          osInfo.Cert,
							Bucket:              osInfo.Bucket,
							CloudCredentialName: osInfo.CloudCredentialName,
							Folder:              "testfolder",
						},
					},
				},
				MachinePools: []provisioningv1api.RKEMachinePool{{
					EtcdRole:         true,
					ControlPlaneRole: true,
					WorkerRole:       true,
					Quantity:         &defaults.One,
				}},
			},
		},
	})
	require.NoError(t, err)

	c, err = cluster.WaitForCreate(clients, c)
	require.NoError(t, err)

	defer func() {
		if t.Failed() {
			data, newErr := cluster.GatherDebugData(clients, c)
			if newErr != nil {
				logrus.Error(newErr)
			}
			fmt.Printf("cluster %s protected init restore test failed\ncluster %s test data bundle: \n%s\n", c.Name, c.Name, data)
		}
	}()

	// Record the only etcd Machine D, its plan Secret join URL and its init-node mark.
	machines, err := cluster.Machines(clients, c)
	require.NoError(t, err)
	require.Len(t, machines.Items, 1, "expected exactly one etcd machine initially")
	d := machines.Items[0].DeepCopy()
	require.Equal(t, "PodMachine", d.Spec.InfrastructureRef.Kind, "test requires the pod machine driver")
	require.True(t, d.Spec.Bootstrap.ConfigRef.IsDefined(), "D has no bootstrap reference")
	require.Eventually(t, func() bool {
		m, err := clients.CAPI.Machine().Get(d.Namespace, d.Name, metav1.GetOptions{})
		return err == nil && m.Annotations[initProtectedPreTerminateHook] != ""
	}, 2*time.Minute, 2*time.Second, "D never received the pre-terminate hook")

	var dJoinURL string
	require.Eventually(t, func() bool {
		s, err := initProtectedPlanSecret(clients, d)
		if err != nil {
			return false
		}
		dJoinURL = s.Annotations[capr.JoinURLAnnotation]
		return dJoinURL != "" && s.Labels[capr.InitNodeLabel] == "true"
	}, 5*time.Minute, 2*time.Second, "D never became the init node with a join URL")
	dNodeName := ""
	require.Eventually(t, func() bool {
		m, err := clients.CAPI.Machine().Get(d.Namespace, d.Name, metav1.GetOptions{})
		if err != nil || !m.Status.NodeRef.IsDefined() {
			return false
		}
		dNodeName = m.Status.NodeRef.Name
		return true
	}, 5*time.Minute, 2*time.Second, "D has no NodeRef")

	// Snapshot with a canary ConfigMap, then delete the canary so the restore must bring it back.
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-configmap-" + name.Hex(time.Now().String(), 10),
		},
		Data: map[string]string{
			"test": "wow",
		},
	}
	snapshot := operations.RunSnapshotCreateTest(t, clients, c, cm, "s3")
	require.NotNil(t, snapshot)
	operations.DeleteConfigMapBeforeRestore(t, clients, c, cm)

	// Block only D's etcd peer port. Loopback stays open so D's own single-node etcd keeps running.
	dPodNamespace, dPodName := initProtectedPodFor(t, clients, d)
	peerRules := []struct {
		chain string
		iface []string
	}{
		{chain: "INPUT", iface: []string{"!", "-i", "lo"}},
		{chain: "OUTPUT", iface: []string{"!", "-o", "lo"}},
	}
	for _, rule := range peerRules {
		insert := initProtectedPeerRejectCmd("-I", rule.chain, rule.iface)
		out, err := cluster.ExecOnPod(clients, dPodNamespace, dPodName, insert...)
		require.NoError(t, err, "failed to insert %s peer-port block: %s", rule.chain, out)
		// Registered before the rest of the test so a failed run still removes the rule while clients are open.
		defer func() {
			_, _ = cluster.ExecOnPod(clients, dPodNamespace, dPodName, initProtectedPeerRejectCmd("-D", rule.chain, rule.iface)...)
		}()
		check := initProtectedPeerRejectCmd("-C", rule.chain, rule.iface)
		out, err = cluster.ExecOnPod(clients, dPodNamespace, dPodName, check...)
		require.NoError(t, err, "%s peer-port block was not present after insertion: %s", rule.chain, out)
	}

	// A broken D should fail here, not later with a confusing restore error.
	clusterClients, err := clients.ForCluster(c.Namespace, c.Name)
	require.NoError(t, err)
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(2 * time.Second) {
		node, err := clusterClients.Core.Node().Get(dNodeName, metav1.GetOptions{})
		require.NoError(t, err, "D's downstream node disappeared after the peer-port block")
		require.True(t, nodeReady(node), "D's downstream node is not Ready after the peer-port block")
	}

	// Delete D directly. It must stay protected by its hook and must still be the only Machine CAPI counts.
	require.NoError(t, clients.CAPI.Machine().Delete(d.Namespace, d.Name, &metav1.DeleteOptions{}))
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); time.Sleep(2 * time.Second) {
		held, err := clients.CAPI.Machine().Get(d.Namespace, d.Name, metav1.GetOptions{})
		require.NoError(t, err, "D was removed while its etcd peer port was blocked")
		require.Equal(t, d.UID, held.UID, "D was replaced while held")
		require.False(t, held.DeletionTimestamp.IsZero(), "D is not deleting")
		require.NotEmpty(t, held.Annotations[initProtectedPreTerminateHook], "D's pre-terminate hook was removed")

		bootstrap, err := clients.RKE.RKEBootstrap().Get(d.Namespace, d.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotContains(t, bootstrap.Annotations, capr.ForceRemoveEtcdAnnotation, "D was force-removed without a restore")

		ml, err := cluster.Machines(clients, c)
		require.NoError(t, err)
		require.Len(t, ml.Items, 1, "CAPI should count only the held D")
		require.Equal(t, d.UID, ml.Items[0].UID)
	}

	// Add capacity so E can join D. Scale back to one so no extra replacement is created.
	c, err = operations.Scale(clients, c, 0, 2, false)
	require.NoError(t, err)

	var e *capi.Machine
	require.Eventually(t, func() bool {
		ml, err := cluster.Machines(clients, c)
		if err != nil {
			return false
		}
		for i := range ml.Items {
			if ml.Items[i].UID != d.UID && ml.Items[i].DeletionTimestamp.IsZero() {
				e = ml.Items[i].DeepCopy()
				return true
			}
		}
		return false
	}, 10*time.Minute, 2*time.Second, "replacement E never appeared")

	c, err = operations.Scale(clients, c, 0, 1, false)
	require.NoError(t, err)
	require.Equal(t, "PodMachine", e.Spec.InfrastructureRef.Kind, "test requires the pod machine driver")

	// E must join D but must not be promoted to init while the peer port is blocked.
	require.Eventually(t, func() bool {
		s, err := initProtectedPlanSecret(clients, e)
		return err == nil && s.Annotations[capr.JoinedToAnnotation] == dJoinURL
	}, 10*time.Minute, 2*time.Second, "E never joined D's join URL")

	const replacementJoinMessage = "waiting for the replacement to join the existing etcd cluster through"
	require.Eventually(t, func() bool {
		cp, err := clients.RKE.RKEControlPlane().Get(c.Namespace, c.Name, metav1.GetOptions{})
		return err == nil && strings.Contains(capr.Ready.GetMessage(&cp.Status), replacementJoinMessage)
	}, 5*time.Minute, 2*time.Second, "control plane never reported the replacement join wait")

	for deadline := time.Now().Add(3 * time.Minute); time.Now().Before(deadline); time.Sleep(2 * time.Second) {
		cp, err := clients.RKE.RKEControlPlane().Get(c.Namespace, c.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.Contains(t, capr.Ready.GetMessage(&cp.Status), replacementJoinMessage, "control plane lost the replacement join wait")

		eSecret, err := initProtectedPlanSecret(clients, e)
		require.NoError(t, err)
		require.NotEqual(t, "true", eSecret.Labels[capr.InitNodeLabel], "E was promoted to init node without a restore")

		dSecret, err := initProtectedPlanSecret(clients, d)
		require.NoError(t, err)
		require.Equal(t, "true", dSecret.Labels[capr.InitNodeLabel], "D lost its init-node mark")

		held, err := clients.CAPI.Machine().Get(d.Namespace, d.Name, metav1.GetOptions{})
		require.NoError(t, err, "D was removed during the hold")
		require.Equal(t, d.UID, held.UID)
		require.NotEmpty(t, held.Annotations[initProtectedPreTerminateHook], "D's pre-terminate hook was removed")

		bootstrap, err := clients.RKE.RKEBootstrap().Get(d.Namespace, d.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotContains(t, bootstrap.Annotations, capr.ForceRemoveEtcdAnnotation, "D was force-removed before the restore")

		currentE, err := clients.CAPI.Machine().Get(e.Namespace, e.Name, metav1.GetOptions{})
		require.NoError(t, err, "E was removed during the hold")
		require.Equal(t, e.UID, currentE.UID)
		require.True(t, currentE.DeletionTimestamp.IsZero(), "E is deleting during the hold")
	}

	// The legacy S3 restore is the only path allowed to elect E and force-remove D.
	restoreStart := time.Now()
	operations.RunSnapshotRestoreTest(t, clients, c, snapshot.Name, cm, 1)
	t.Logf("restore completed in %s", time.Since(restoreStart).Round(time.Second))

	require.Eventually(t, func() bool {
		m, err := clients.CAPI.Machine().Get(d.Namespace, d.Name, metav1.GetOptions{})
		if apierror.IsNotFound(err) {
			return true
		}
		return err == nil && m.UID != d.UID
	}, 5*time.Minute, 2*time.Second, "D was not removed after the restore")

	ml, err := cluster.Machines(clients, c)
	require.NoError(t, err)
	require.Len(t, ml.Items, 1, "expected only E after the restore")
	require.Equal(t, e.UID, ml.Items[0].UID)

	require.Eventually(t, func() bool {
		s, err := initProtectedPlanSecret(clients, e)
		return err == nil && s.Labels[capr.InitNodeLabel] == "true"
	}, 5*time.Minute, 2*time.Second, "E is not the init node after the restore")

	capiCluster, err := clients.CAPI.Cluster().Get(c.Namespace, c.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.False(t, ptr.Deref(capiCluster.Spec.Paused, false), "CAPI cluster is still paused after the restore")

	currentE, err := clients.CAPI.Machine().Get(e.Namespace, e.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.True(t, currentE.Status.NodeRef.IsDefined(), "E has no NodeRef after the restore")
	eNodeName := currentE.Status.NodeRef.Name

	clusterClients, err = clients.ForCluster(c.Namespace, c.Name)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		nodes, err := clusterClients.Core.Node().List(metav1.ListOptions{})
		if err != nil {
			return false
		}
		var liveName string
		live := 0
		for i := range nodes.Items {
			if nodes.Items[i].DeletionTimestamp.IsZero() {
				live++
				liveName = nodes.Items[i].Name
			}
		}
		return live == 1 && liveName == eNodeName
	}, 5*time.Minute, 2*time.Second, "downstream nodes did not converge to E only")

	// nodesyncer publishes EtcdIsVoter asynchronously, so poll the v3 Node.
	require.Eventually(t, func() bool {
		nodes, err := clients.Mgmt.Node().List(c.Status.ClusterName, metav1.ListOptions{})
		if err != nil {
			return false
		}
		for i := range nodes.Items {
			n := &nodes.Items[i]
			if n.Status.NodeLabels[capr.MachineUIDLabel] != string(e.UID) {
				continue
			}
			for _, condition := range n.Status.InternalNodeStatus.Conditions {
				if condition.Type == corev1.NodeConditionType(initProtectedVoterCondition) && condition.Status == corev1.ConditionTrue {
					return true
				}
			}
		}
		return false
	}, 5*time.Minute, 2*time.Second, "E's management Node never reported EtcdIsVoter=True")

	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	require.NoError(t, err)
}

// Test_Provisioning_SetA_MP_SingleNodeDirectDeleteHandoff directly deletes the only etcd Machine D of a healthy
// single-node cluster. D must stay protected while E joins through it. The hand-off must then complete without a
// restore, and a canary ConfigMap written before the delete must survive.
func Test_Provisioning_SetA_MP_SingleNodeDirectDeleteHandoff(t *testing.T) {
	clients, err := clients.New()
	require.NoError(t, err)
	defer clients.Close()

	newNs, err := namespace.Random(clients)
	require.NoError(t, err)

	c, err := cluster.New(clients, &provisioningv1api.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mp-single-node-handoff",
			Namespace: newNs.Name,
		},
		Spec: provisioningv1api.ClusterSpec{
			RKEConfig: &provisioningv1api.RKEConfig{
				MachinePools: []provisioningv1api.RKEMachinePool{{
					EtcdRole:         true,
					ControlPlaneRole: true,
					WorkerRole:       true,
					Quantity:         &defaults.One,
				}},
			},
		},
	})
	require.NoError(t, err)

	c, err = cluster.WaitForCreate(clients, c)
	require.NoError(t, err)

	defer func() {
		if t.Failed() {
			data, newErr := cluster.GatherDebugData(clients, c)
			if newErr != nil {
				logrus.Error(newErr)
			}
			fmt.Printf("cluster %s single-node direct delete handoff test failed\ncluster %s test data bundle: \n%s\n", c.Name, c.Name, data)
		}
	}()

	// Record the only etcd Machine D with its hook, join URL and init-node mark.
	machines, err := cluster.Machines(clients, c)
	require.NoError(t, err)
	require.Len(t, machines.Items, 1, "expected exactly one machine initially")
	d := machines.Items[0].DeepCopy()
	require.Equal(t, "PodMachine", d.Spec.InfrastructureRef.Kind, "test requires the pod machine driver")
	require.Eventually(t, func() bool {
		m, err := clients.CAPI.Machine().Get(d.Namespace, d.Name, metav1.GetOptions{})
		return err == nil && m.Annotations[initProtectedPreTerminateHook] != ""
	}, 2*time.Minute, 2*time.Second, "D never received the pre-terminate hook")

	var dJoinURL string
	require.Eventually(t, func() bool {
		s, err := initProtectedPlanSecret(clients, d)
		if err != nil {
			return false
		}
		dJoinURL = s.Annotations[capr.JoinURLAnnotation]
		return dJoinURL != "" && s.Labels[capr.InitNodeLabel] == "true"
	}, 5*time.Minute, 2*time.Second, "D never became the init node with a join URL")

	// The canary must still exist with the same UID and data after the hand-off.
	downstream, err := operations.GetAndVerifyDownstreamClientset(clients, c)
	require.NoError(t, err)
	canary, err := downstream.CoreV1().ConfigMaps("default").Create(clients.Ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "single-node-handoff-" + name.Hex(time.Now().String(), 10),
			Namespace: "default",
		},
		Data: map[string]string{"test": "wow"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	// Delete D directly. It must stay protected and must still be the only Machine CAPI counts.
	require.NoError(t, clients.CAPI.Machine().Delete(d.Namespace, d.Name, &metav1.DeleteOptions{}))
	require.Eventually(t, func() bool {
		cp, err := clients.RKE.RKEControlPlane().Get(c.Namespace, c.Name, metav1.GetOptions{})
		return err == nil && strings.Contains(capr.Ready.GetMessage(&cp.Status), "last etcd member is protected")
	}, 5*time.Minute, 2*time.Second, "control plane never reported the protected last etcd member")

	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); time.Sleep(2 * time.Second) {
		held, err := clients.CAPI.Machine().Get(d.Namespace, d.Name, metav1.GetOptions{})
		require.NoError(t, err, "D was removed before the hand-off")
		require.Equal(t, d.UID, held.UID, "D was replaced while held")
		require.False(t, held.DeletionTimestamp.IsZero(), "D is not deleting")
		require.NotEmpty(t, held.Annotations[initProtectedPreTerminateHook], "D's pre-terminate hook was removed")

		bootstrap, err := clients.RKE.RKEBootstrap().Get(d.Namespace, d.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotContains(t, bootstrap.Annotations, capr.ForceRemoveEtcdAnnotation, "D was force-removed without a restore")

		ml, err := cluster.Machines(clients, c)
		require.NoError(t, err)
		require.Len(t, ml.Items, 1, "CAPI should count only the held D")
		require.Equal(t, d.UID, ml.Items[0].UID)
	}

	// Add capacity so E can join D. Scale back to one so no extra replacement is created.
	c, err = operations.Scale(clients, c, 0, 2, false)
	require.NoError(t, err)

	var e *capi.Machine
	require.Eventually(t, func() bool {
		ml, err := cluster.Machines(clients, c)
		if err != nil {
			return false
		}
		for i := range ml.Items {
			if ml.Items[i].UID != d.UID && ml.Items[i].DeletionTimestamp.IsZero() {
				e = ml.Items[i].DeepCopy()
				return true
			}
		}
		return false
	}, 10*time.Minute, 2*time.Second, "replacement E never appeared")

	c, err = operations.Scale(clients, c, 0, 1, false)
	require.NoError(t, err)
	require.Equal(t, "PodMachine", e.Spec.InfrastructureRef.Kind, "test requires the pod machine driver")

	require.Eventually(t, func() bool {
		s, err := initProtectedPlanSecret(clients, e)
		return err == nil && s.Annotations[capr.JoinedToAnnotation] == dJoinURL
	}, 10*time.Minute, 2*time.Second, "E never joined D's join URL")

	// E is promoted once the hand-off completes. D must never be force-removed before that.
	const replacementJoinMessage = "waiting for the replacement to join the existing etcd cluster through"
	sawJoinWait, promoted := false, false
	for deadline := time.Now().Add(20 * time.Minute); time.Now().Before(deadline) && !promoted; time.Sleep(2 * time.Second) {
		cp, err := clients.RKE.RKEControlPlane().Get(c.Namespace, c.Name, metav1.GetOptions{})
		require.NoError(t, err)
		if strings.Contains(capr.Ready.GetMessage(&cp.Status), replacementJoinMessage) {
			sawJoinWait = true
		}

		bootstrap, err := clients.RKE.RKEBootstrap().Get(d.Namespace, d.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
		if apierror.IsNotFound(err) {
			// D's cleanup finishes after E is promoted, so a missing bootstrap is only a failure if E is not the init node.
			eSecret, err := initProtectedPlanSecret(clients, e)
			require.NoError(t, err)
			if eSecret.Labels[capr.InitNodeLabel] != "true" {
				t.Fatal("D's bootstrap was removed before E became the init node")
			}
			promoted = true
			break
		}
		require.NoError(t, err)
		require.NotContains(t, bootstrap.Annotations, capr.ForceRemoveEtcdAnnotation, "D was force-removed before E became the init node")

		eSecret, err := initProtectedPlanSecret(clients, e)
		require.NoError(t, err)
		promoted = eSecret.Labels[capr.InitNodeLabel] == "true"
	}
	require.True(t, promoted, "E never became the init node")
	require.True(t, sawJoinWait, "control plane never reported the replacement join wait")

	// D is removed only after the hand-off, and the cluster must converge on E alone.
	require.NoError(t, waitForMachineGone(clients, d, 15*time.Minute))
	c, err = cluster.WaitForCreate(clients, c)
	require.NoError(t, err)

	ml, err := cluster.Machines(clients, c)
	require.NoError(t, err)
	require.Len(t, ml.Items, 1, "expected only E after the hand-off")
	require.Equal(t, e.UID, ml.Items[0].UID)

	currentE, err := clients.CAPI.Machine().Get(e.Namespace, e.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.True(t, currentE.Status.NodeRef.IsDefined(), "E has no NodeRef after the hand-off")
	eNodeName := currentE.Status.NodeRef.Name

	clusterClients, err := clients.ForCluster(c.Namespace, c.Name)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		nodes, err := clusterClients.Core.Node().List(metav1.ListOptions{})
		if err != nil {
			return false
		}
		var liveName string
		live := 0
		for i := range nodes.Items {
			if nodes.Items[i].DeletionTimestamp.IsZero() {
				live++
				liveName = nodes.Items[i].Name
			}
		}
		return live == 1 && liveName == eNodeName
	}, 5*time.Minute, 2*time.Second, "downstream nodes did not converge to E only")

	// nodesyncer publishes EtcdIsVoter asynchronously, so poll the v3 Node.
	require.Eventually(t, func() bool {
		nodes, err := clients.Mgmt.Node().List(c.Status.ClusterName, metav1.ListOptions{})
		if err != nil {
			return false
		}
		for i := range nodes.Items {
			n := &nodes.Items[i]
			if n.Status.NodeLabels[capr.MachineUIDLabel] != string(e.UID) {
				continue
			}
			for _, condition := range n.Status.InternalNodeStatus.Conditions {
				if condition.Type == corev1.NodeConditionType(initProtectedVoterCondition) && condition.Status == corev1.ConditionTrue {
					return true
				}
			}
		}
		return false
	}, 5*time.Minute, 2*time.Second, "E's management Node never reported EtcdIsVoter=True")

	require.Eventually(t, func() bool {
		pods, err := clusterClients.Core.Pod().List("cattle-system", metav1.ListOptions{LabelSelector: "app=cattle-cluster-agent"})
		if err != nil {
			return false
		}
		for i := range pods.Items {
			if pods.Items[i].Spec.NodeName == eNodeName && pods.Items[i].Status.Phase == corev1.PodRunning {
				return true
			}
		}
		return false
	}, 5*time.Minute, 2*time.Second, "cattle-cluster-agent is not Running on E's node")

	downstream, err = operations.GetAndVerifyDownstreamClientset(clients, c)
	require.NoError(t, err)
	got, err := downstream.CoreV1().ConfigMaps("default").Get(clients.Ctx, canary.Name, metav1.GetOptions{})
	require.NoError(t, err, "canary ConfigMap was lost during the hand-off")
	require.Equal(t, canary.UID, got.UID, "canary ConfigMap was recreated")
	require.Equal(t, canary.Data, got.Data, "canary ConfigMap data changed")

	err = cluster.EnsureMinimalConflictsWithThreshold(clients, c, cluster.SaneConflictMessageThreshold)
	require.NoError(t, err)
}

// initProtectedPlanSecret returns the plan Secret for a Machine through its RKEBootstrap.
func initProtectedPlanSecret(clients *clients.Clients, m *capi.Machine) (*corev1.Secret, error) {
	bootstrap, err := clients.RKE.RKEBootstrap().Get(m.Namespace, m.Spec.Bootstrap.ConfigRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return clients.Core.Secret().Get(bootstrap.Namespace, capr.PlanSecretFromBootstrapName(bootstrap.Name), metav1.GetOptions{})
}

// initProtectedPodFor resolves the systemd-node Pod that backs a PodMachine.
func initProtectedPodFor(t *testing.T, clients *clients.Clients, m *capi.Machine) (string, string) {
	t.Helper()
	var podNamespace, podName string
	require.Eventually(t, func() bool {
		infraMachine, err := external.GetObjectFromContractVersionedRef(clients.Ctx, clients.Client, m.Spec.InfrastructureRef, m.Namespace)
		if err != nil {
			return false
		}
		podNamespace = infraMachine.GetNamespace()
		podName = strings.ReplaceAll(infraMachine.GetName(), ".", "-")
		_, err = clients.Core.Pod().Get(podNamespace, podName, metav1.GetOptions{})
		return err == nil
	}, 5*time.Minute, 2*time.Second, "pod for machine %s never resolved", m.Name)
	return podNamespace, podName
}

// initProtectedPeerRejectCmd builds an iptables command for one REJECT rule on the etcd peer port.
func initProtectedPeerRejectCmd(action, chain string, iface []string) []string {
	cmd := []string{"iptables", action, chain}
	cmd = append(cmd, iface...)
	return append(cmd, "-p", "tcp", "--dport", initProtectedEtcdPeerPort, "-j", "REJECT")
}
