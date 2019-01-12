package kubeconfig

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"regexp"

	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/services"
	managementv3 "github.com/rancher/types/client/management/v3"
)

const (
	certDelim = "\\\n      "
	firstLen  = 49
)

var (
	splitRegexp = regexp.MustCompile(`\S{1,76}`)
)

type node struct {
	ClusterName string
	Server      string
	Cert        string
	User        string
}

type data struct {
	ClusterName string
	Host        string
	ClusterID   string
	Cert        string
	User        string
	Username    string
	Password    string
	Token       string
	Nodes       []node
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

func getDefaultNode(clusterName, clusterID, host, username string) node {
	return node{
		Server:      fmt.Sprintf("https://%s/k8s/clusters/%s", host, clusterID),
		Cert:        caCertString(),
		ClusterName: clusterName,
		User:        username,
	}
}

func ForTokenBased(clusterName, clusterID, host, username, token string) (string, error) {
	data := &data{
		ClusterName: clusterName,
		ClusterID:   clusterID,
		Host:        host,
		Cert:        caCertString(),
		User:        username,
		Token:       token,
		Nodes:       []node{getDefaultNode(clusterName, clusterID, host, username)},
	}

	if data.ClusterName == "" {
		data.ClusterName = data.ClusterID
	}

	buf := &bytes.Buffer{}
	err := tokenTemplate.Execute(buf, data)
	return buf.String(), err
}

func ForClusterTokenBased(cluster *managementv3.Cluster, clusterID, host, username, token string) (string, error) {
	nodes := []node{getDefaultNode(cluster.Name, clusterID, host, username)}

	if cluster.LocalClusterAuthEndpoint.FQDN != "" {
		clusterNode := node{
			ClusterName: cluster.Name + "-fqdn",
			Server:      "https://" + cluster.LocalClusterAuthEndpoint.FQDN,
			Cert:        formatCertString(cluster.LocalClusterAuthEndpoint.CACerts),
			User:        username,
		}
		nodes = append(nodes, clusterNode)
	} else {
		var rkeNodes []managementv3.RKEConfigNode
		if cluster.AppliedSpec != nil && cluster.AppliedSpec.RancherKubernetesEngineConfig != nil {
			rkeNodes = cluster.AppliedSpec.RancherKubernetesEngineConfig.Nodes
		}
		for _, rkeNode := range rkeNodes {
			if !slice.ContainsString(rkeNode.Role, services.ControlRole) {
				continue
			}
			clusterNode := node{
				ClusterName: cluster.Name + "-" + rkeNode.HostnameOverride,
				Server:      "https://" + rkeNode.Address + ":6443",
				Cert:        formatCertString(cluster.CACert),
				User:        username,
			}
			nodes = append(nodes, clusterNode)
		}
	}

	data := &data{
		ClusterName: cluster.Name,
		ClusterID:   clusterID,
		Host:        host,
		Cert:        caCertString(),
		User:        username,
		Token:       token,
		Nodes:       nodes,
	}

	if data.ClusterName == "" {
		data.ClusterName = data.ClusterID
	}

	buf := &bytes.Buffer{}
	err := tokenTemplate.Execute(buf, data)
	return buf.String(), err
}
