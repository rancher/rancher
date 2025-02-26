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
	nodeListEmptyMessageError = "node list is empty"
	externalIPAnnotation      = "rke.cattle.io/external-ip"
	downloadUrlFormat         = "https://%s/v3/nodes/%s:%s/nodeconfig"
	zipFileNameFormat         = "%s.zip"
	curlCommandFormat         = "curl -s -sSL -k -H '%s' %s --output %s"
	unzipCommandFormat        = "unzip -q %s -d %s"
	catCommandFormat          = "cat %s/%s/id_rsa"
	bashCommandFormat         = "%s && %s && %s"
)

// CreateSSHNode is a helper to create a SSH Node
func CreateSSHNode(client *rancher.Client, clusterName string, clusterID string) (*nodes.Node, error) {

	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return nil, err
	}

	provisioningClusterID, err := extensionsClusters.GetV1ProvisioningClusterByName(client, clusterName)
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
			return nil, errors.New(nodeListEmptyMessageError)
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
			return nil, errors.New(nodeListEmptyMessageError)
		}

		v3Node := v3NodeList.Items[0]

		externalIP := v3Node.Status.NodeAnnotations[externalIPAnnotation]

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
	downloadUrl := fmt.Sprintf(downloadUrlFormat, client.RancherConfig.Host, clusterID, v3Node.Name)
	autorizationBearer := fmt.Sprintf("Authorization:Bearer %s", client.RancherConfig.AdminToken)

	fileName := namegenerator.AppendRandomString("file-name")
	zipFile := fmt.Sprintf(zipFileNameFormat, fileName)

	curlCommand := fmt.Sprintf(curlCommandFormat, autorizationBearer, downloadUrl, zipFile)
	unzipCommand := fmt.Sprintf(unzipCommandFormat, zipFile, fileName)
	catCommand := fmt.Sprintf(catCommandFormat, fileName, v3Node.Status.NodeName)
	bashCommand := fmt.Sprintf(bashCommandFormat, curlCommand, unzipCommand, catCommand)

	execCmd := []string{"bash", "-c", bashCommand}
	return kubectl.Command(client, nil, clusterID, execCmd, "")
}
