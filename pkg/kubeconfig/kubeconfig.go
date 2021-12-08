package kubeconfig

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	managementv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/settings"
)

const (
	certDelim = "\\\n      "
	firstLen  = 49
)

var (
	splitRegexp = regexp.MustCompile(`\S{1,76}`)
)

type kubeNode struct {
	ClusterName string
	Server      string
	Cert        string
	User        string
}

type data struct {
	ClusterName     string
	Host            string
	ClusterID       string
	Cert            string
	User            string
	Username        string
	Password        string
	Token           string
	EndpointEnabled bool
	Nodes           []kubeNode
}

func ForBasic(host, username, password string) (string, error) {
	data := &data{
		ClusterName: "cluster",
		Host:        host,
		Cert:        caCertString(),
		User:        username,
		Username:    username,
		Password:    password,
	}

	if data.ClusterName == "" {
		data.ClusterName = data.ClusterID
	}

	buf := &bytes.Buffer{}
	err := basicTemplate.Execute(buf, data)
	return buf.String(), err
}

func formatCertString(certData string) string {
	buf := &bytes.Buffer{}
	if len(certData) > firstLen {
		buf.WriteString(certData[:firstLen])
		certData = certData[firstLen:]
	} else {
		return certData
	}

	for _, part := range splitRegexp.FindAllStringSubmatch(certData, -1) {
		buf.WriteString(certDelim)
		buf.WriteString(part[0])
	}

	return buf.String()
}

func caCertString() string {
	certData := settings.CACerts.Get()
	if certData == "" {
		return ""
	}
	certData = base64.StdEncoding.EncodeToString([]byte(certData))
	return formatCertString(certData)
}

func getDefaultNode(clusterName, clusterID, host string) kubeNode {
	return kubeNode{
		Server:      fmt.Sprintf("https://%s/k8s/clusters/%s", host, clusterID),
		Cert:        caCertString(),
		ClusterName: clusterName,
		User:        clusterName,
	}
}

func ForTokenBased(clusterName, clusterID, host, token string) (string, error) {
	data := &data{
		ClusterName:     clusterName,
		ClusterID:       clusterID,
		Host:            host,
		Cert:            caCertString(),
		User:            clusterName,
		Token:           token,
		Nodes:           []kubeNode{getDefaultNode(clusterName, clusterID, host)},
		EndpointEnabled: false,
	}

	if data.ClusterName == "" {
		data.ClusterName = data.ClusterID
	}

	buf := &bytes.Buffer{}
	err := tokenTemplate.Execute(buf, data)
	return buf.String(), err
}

func ForClusterTokenBased(cluster *managementv3.Cluster, nodes []*mgmtv3.Node, clusterID, host, token string) (string, error) {
	clusterName := cluster.Name
	if clusterName == "" {
		clusterName = clusterID
	}

	nodesForConfig := []kubeNode{getDefaultNode(clusterName, clusterID, host)}

	if cluster.LocalClusterAuthEndpoint.FQDN != "" {
		fqdnCACerts := base64.StdEncoding.EncodeToString([]byte(cluster.LocalClusterAuthEndpoint.CACerts))
		clusterNode := kubeNode{
			ClusterName: clusterName + "-fqdn",
			Server:      "https://" + cluster.LocalClusterAuthEndpoint.FQDN,
			Cert:        formatCertString(fqdnCACerts),
			User:        clusterName,
		}
		nodesForConfig = append(nodesForConfig, clusterNode)
	} else {
		for _, n := range nodes {
			if n.Spec.ControlPlane {
				nodeName := clusterName + "-" + strings.TrimPrefix(n.Spec.RequestedHostname, clusterName+"-")
				clusterNode := kubeNode{
					ClusterName: nodeName,
					Server:      "https://" + node.GetEndpointNodeIP(n) + ":6443",
					Cert:        formatCertString(cluster.CACert),
					User:        clusterName,
				}
				nodesForConfig = append(nodesForConfig, clusterNode)
			}
		}
	}

	data := &data{
		ClusterName:     clusterName,
		ClusterID:       clusterID,
		Host:            host,
		Cert:            caCertString(),
		User:            clusterName,
		Token:           token,
		Nodes:           nodesForConfig,
		EndpointEnabled: true,
	}

	buf := &bytes.Buffer{}
	err := tokenTemplate.Execute(buf, data)
	return buf.String(), err
}
