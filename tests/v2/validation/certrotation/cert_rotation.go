package certrotation

import (
	"context"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/rancher/norman/types"
	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/sshkeys"
	"github.com/rancher/shepherd/pkg/nodes"
	"github.com/rancher/shepherd/pkg/wait"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace                     = "fleet-default"
	provisioningSteveResourceType = "provisioning.cattle.io.cluster"
	machineSteveResourceType      = "cluster.x-k8s.io.machine"
	machineSteveAnnotation        = "cluster.x-k8s.io/machine"
	etcdLabel                     = "node-role.kubernetes.io/etcd"
	clusterLabel                  = "cluster.x-k8s.io/cluster-name"
	certFileExtension             = ".crt"
	pemFileExtension              = ".pem"

	privateKeySSHKeyRegExPattern = `-----BEGIN RSA PRIVATE KEY-{3,}\n([\s\S]*?)\n-{3,}END RSA PRIVATE KEY-----`
)

// rotateCerts rotates the certificates in a RKE2/K3S downstream cluster.
func rotateCerts(client *rancher.Client, clusterName string) error {
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}

	id, err := clusters.GetV1ProvisioningClusterByName(client, clusterName)
	if err != nil {
		return err
	}

	cluster, err := adminClient.Steve.SteveType(provisioningSteveResourceType).ByID(id)
	if err != nil {
		return err
	}

	clusterSpec := &apiv1.ClusterSpec{}
	err = v1.ConvertToK8sType(cluster.Spec, clusterSpec)
	if err != nil {
		return err
	}

	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return err
	}

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	nodeList, err := steveclient.SteveType("node").List(nil)
	if err != nil {
		return err
	}

	nodeCertificates := map[string]map[string]string{}

	sshUser, err := sshkeys.GetSSHUser(client, cluster)
	if err != nil {
		return err
	}
	if sshUser == "" {
		return errors.New("sshUser does not exist")
	}

	for _, node := range nodeList.Data {
		newCertificate, err := getCertificatesFromMachine(client, &node, sshUser)
		if err != nil {
			return err
		}

		nodeCertificates[node.ID] = newCertificate
	}

	updatedCluster := *cluster
	generation := int64(1)
	if clusterSpec.RKEConfig.RotateCertificates != nil {
		generation = clusterSpec.RKEConfig.RotateCertificates.Generation + 1
	}

	clusterSpec.RKEConfig.RotateCertificates = &rkev1.RotateCertificates{
		Generation: generation,
	}

	updatedCluster.Spec = *clusterSpec

	_, err = client.Steve.SteveType(provisioningSteveResourceType).Update(cluster, updatedCluster)
	if err != nil {
		return err
	}

	logrus.Infof("updated cluster, certs are rotating...")
	kubeRKEClient, err := client.GetKubeAPIRKEClient()
	if err != nil {
		return err
	}

	result, err := kubeRKEClient.RKEControlPlanes(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	checkFunc := provisioning.CertRotationCompleteCheckFunc(generation)
	logrus.Infof("waiting for certs to rotate, checking status now...")
	err = wait.WatchWait(result, checkFunc)
	if err != nil {
		return err
	}

	kubeProvisioningClient, err := client.GetKubeAPIProvisioningClient()
	if err != nil {
		return err
	}

	clusterWait, err := kubeProvisioningClient.Clusters("fleet-default").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	clusterCheckFunc := clusters.IsProvisioningClusterReady
	logrus.Infof("waiting for cluster to become active again, checking status now...")
	err = wait.WatchWait(clusterWait, clusterCheckFunc)
	if err != nil {
		return err
	}

	postRotatedCertificates := map[string]map[string]string{}

	for _, node := range nodeList.Data {
		newCertificate, err := getCertificatesFromMachine(client, &node, sshUser)
		if err != nil {
			return err
		}

		postRotatedCertificates[node.ID] = newCertificate
	}

	isAllCertRotated := compareCertificatesFromMachines(nodeCertificates, postRotatedCertificates)
	if !isAllCertRotated {
		return errors.New("certs weren't properly rotated")
	}

	logrus.Infof("Cert Rotation Complete.")
	return nil
}

// rotateRKE1Certs rotates the certificates in a RKE1 downstream cluster.
func rotateRKE1Certs(client *rancher.Client, clusterName string) error {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return err
	}

	nodeList, err := client.Management.Node.List(&types.ListOpts{Filters: map[string]interface{}{
		"clusterId": clusterID,
	}})
	if err != nil {
		return err
	}

	nodeCertificates := map[string]map[string]string{}

	for _, node := range nodeList.Data {
		newCertificate, err := getCertificatesFromV3Node(client, &node)
		if err != nil {
			return err
		}

		nodeCertificates[node.ID] = newCertificate
	}

	cluster, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return err
	}

	_, err = client.Management.Cluster.ActionRotateCertificates(
		cluster,
		&management.RotateCertificateInput{CACertificates: false, Services: ""},
	)
	if err != nil {
		return err
	}

	logrus.Infof("updated cluster, certs are rotating...")

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}

	err = clusters.WaitClusterToBeUpgraded(adminClient, clusterID)
	if err != nil {
		return err
	}

	postRotatedCertificates := map[string]map[string]string{}

	for _, node := range nodeList.Data {
		newCertificate, err := getCertificatesFromV3Node(client, &node)
		if err != nil {
			return err
		}

		postRotatedCertificates[node.ID] = newCertificate
	}

	isAllCertRotated := compareCertificatesFromMachines(nodeCertificates, postRotatedCertificates)
	if !isAllCertRotated {
		return errors.New("certs weren't properly rotated")
	}

	return nil
}

func compareCertificatesFromMachines(certObject1, certObject2 map[string]map[string]string) bool {
	isRotated := true
	for nodeID := range certObject1 {
		for certType := range certObject1[nodeID] {

			if certObject1[nodeID][certType] == certObject2[nodeID][certType] {
				logrus.Infof("non-rotated cert info:")
				logrus.Infof("%s %s was not updated: %s", nodeID, certType, certObject1[nodeID][certType])

				isRotated = false
			}
		}
	}
	return isRotated
}

func getCertificatesFromMachine(client *rancher.Client, machineNode *v1.SteveAPIObject, sshUser string) (map[string]string, error) {
	certificates := map[string]string{}

	sshNode, err := sshkeys.GetSSHNodeFromMachine(client, sshUser, machineNode)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Getting certificates from machine: %s", machineNode.Name)

	clusterType := machineNode.Labels["node.kubernetes.io/instance-type"]
	certsPath := "/var/lib/rancher/" + clusterType + "/server/tls/"

	certsList := []string{
		"client-admin",
		"client-auth-proxy",
		"client-controller",
		"client-kube-apiserver",
		"client-kubelet",
		"client-kube-proxy",

		"client-" + clusterType + "-cloud-controller",
		"client-" + clusterType + "-controller",

		"client-scheduler",
		"client-supervisor",
		"etcd/client",
		"etcd/peer-server-client",
		"kube-controller-manager/kube-controller-manager",
		"kube-scheduler/kube-scheduler",
		"serving-kube-apiserver",
	}

	for _, filename := range certsList {
		// ignoring errors since node roles have different subsets of the certs.
		certString, _ := sshNode.ExecuteCommand("openssl x509 -enddate -noout -in " + certsPath + filename + certFileExtension)

		if certString != "" {
			certificates[filename] = certString
		}
	}

	return certificates, nil
}

func getCertificatesFromV3Node(client *rancher.Client, v3Node *management.Node) (map[string]string, error) {
	certificates := map[string]string{}

	sshNode, err := getSSHNodeFromV3Node(client, v3Node)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Getting certificates from machine: %s", v3Node.ID)

	certsPath := "/etc/kubernetes/ssl/"

	certsList := []string{
		"kube-apiserver",
		"kube-apiserver-proxy-client",
		"$(find /etc/kubernetes/ssl -name 'kube-etcd-*' | grep -v \"key\")",
		"kube-controller-manager",
		// Due to GH issue https://github.com/rancher/rancher/issues/44993, kube-node and kube-proxy are commented out.
		//"kube-node",
		//"kube-proxy",
		"kube-scheduler",
	}

	for _, filename := range certsList {
		fullAbsolutePath := certsPath + filename + pemFileExtension
		if strings.Contains(filename, "etcd") {
			fullAbsolutePath = filename
		}

		certString, _ := sshNode.ExecuteCommand("openssl x509 -enddate -noout -in " + fullAbsolutePath)

		if certString != "" {
			certificates[filename] = certString
		}
	}

	return certificates, nil
}

func downloadRKE1SSHKeys(client *rancher.Client, v3Node *management.Node) ([]byte, error) {
	sshKeyLink := v3Node.Links["nodeConfig"]

	req, err := http.NewRequest("GET", sshKeyLink, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+client.RancherConfig.AdminToken)

	resp, err := client.Management.APIBaseClient.Ops.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	privateSSHKeyRegEx := regexp.MustCompile(privateKeySSHKeyRegExPattern)
	privateSSHKey := privateSSHKeyRegEx.FindString(string(bodyBytes))

	return []byte(privateSSHKey), err
}

func getSSHNodeFromV3Node(client *rancher.Client, v3Node *management.Node) (*nodes.Node, error) {
	sshkey, err := downloadRKE1SSHKeys(client, v3Node)
	if err != nil {
		return nil, err
	}

	clusterNode := &nodes.Node{
		NodeID:          v3Node.ID,
		PublicIPAddress: v3Node.ExternalIPAddress,
		SSHUser:         v3Node.SshUser,
		SSHKey:          sshkey,
	}

	return clusterNode, nil
}
