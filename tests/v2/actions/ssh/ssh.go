package ssh

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/sshkeys"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/nodes"
	"github.com/sirupsen/logrus"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionsClusters "github.com/rancher/shepherd/extensions/clusters"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	NodeListEmptyMessageError = "node list is empty"
	ExternalIPAnnotation      = "rke.cattle.io/external-ip"
)

// CreateSSH is a helper to create a SSH Node
func CreateSSH(client *rancher.Client, steveClient *steveV1.Client, clusterName string, clusterID string) (*nodes.Node, error) {

	provisioningClusterID, err := extensionsClusters.GetV1ProvisioningClusterByName(client, client.RancherConfig.ClusterName)
	if err != nil {
		return nil, err
	}

	cluster, err := client.Steve.SteveType(extensionsClusters.ProvisioningSteveResourceType).ByID(provisioningClusterID)
	if err != nil {
		return nil, err
	}

	newCluster := &provv1.Cluster{}
	err = steveV1.ConvertToK8sType(cluster, newCluster)
	if err != nil {
		return nil, err
	}

	sshNode := &nodes.Node{}
	if strings.Contains(newCluster.Spec.KubernetesVersion, "rke2") || strings.Contains(newCluster.Spec.KubernetesVersion, "k3s") {
		_, stevecluster, err := extensionsClusters.GetProvisioningClusterByName(client, clusterName, provisioninginput.Namespace)
		if err != nil {
			return nil, err
		}

		sshUser, err := sshkeys.GetSSHUser(client, stevecluster)
		if err != nil {
			return nil, err
		}

		logrus.Infof("Getting the node using the label [%v]", clusters.LabelWorker)
		query, err := url.ParseQuery(clusters.LabelWorker)
		if err != nil {
			return nil, err
		}

		nodeList, err := steveClient.SteveType("node").List(query)
		if err != nil {
			return nil, err
		}

		if len(nodeList.Data) == 0 {
			return nil, errors.New(NodeListEmptyMessageError)
		}

		firstMachine := nodeList.Data[0]

		sshNode, err = sshkeys.GetSSHNodeFromMachine(client, sshUser, &firstMachine)
		if err != nil {
			return nil, err
		}
	} else {
		v3NodeList, err := client.WranglerContext.Mgmt.Node().List(clusterID, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		if len(v3NodeList.Items) == 0 {
			return nil, errors.New(NodeListEmptyMessageError)
		}

		v3Node := v3NodeList.Items[0]

		externalIP := v3Node.Status.NodeAnnotations[ExternalIPAnnotation]

		sshKey, err := downloadRKESSHKey(client, clusterID, &v3Node)
		if err != nil {
			return nil, err
		}

		sshNode = &nodes.Node{
			NodeID:          v3Node.Name,
			PublicIPAddress: externalIP,
			SSHUser:         v3Node.Status.NodeConfig.User,
			SSHKey:          []byte(sshKey),
		}
	}

	return sshNode, nil
}

func downloadRKESSHKey(client *rancher.Client, clusterID string, v3Node *v3.Node) (string, error) {
	downloadUrl := fmt.Sprintf("https://%s/v3/nodes/%s:%s/nodeconfig", client.RancherConfig.Host, clusterID, v3Node.Name)
	autorizationBearer := fmt.Sprintf("Authorization:Bearer %s", client.RancherConfig.AdminToken)

	fileName := namegenerator.AppendRandomString("file-name")
	zipFile := fmt.Sprintf("%s.zip", fileName)

	curlCommand := fmt.Sprintf("curl -s -sSL -k -H '%s' %s --output %s", autorizationBearer, downloadUrl, zipFile)
	unzipCommand := fmt.Sprintf("unzip -q %s -d %s", zipFile, fileName)
	catCommand := fmt.Sprintf("cat %s/%s/id_rsa", fileName, v3Node.Status.NodeName)
	bashCommand := fmt.Sprintf("%s && %s && %s", curlCommand, unzipCommand, catCommand)

	execCmd := []string{"bash", "-c", bashCommand}
	return kubectl.Command(client, nil, clusterID, execCmd, "")
}
