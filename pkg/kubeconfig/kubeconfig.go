package kubeconfig

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	clientv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	normanv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/node"
	"github.com/rancher/rancher/pkg/settings"
)

const (
	certDelim = "\\\n      "
	firstLen  = 49
)

var splitRegexp = regexp.MustCompile(`\S{1,76}`)

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

type Cluster struct {
	Name   string
	Server string
	Cert   string
}
type User struct {
	Name      string
	Token     string
	Host      string
	ClusterID string
}

type Context struct {
	Name    string
	User    string
	Cluster string
}

type KubeConfig struct {
	Clusters       []Cluster
	Users          []User
	Contexts       []Context
	CurrentContext string
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

func ForClusterTokenBased(cluster *clientv3.Cluster, nodes []*normanv3.Node, clusterID, host, token string) (string, error) {
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

type Input struct {
	ClusterID        string
	Cluster          *apiv3.Cluster
	Nodes            []*apiv3.Node
	TokenKey         string
	IsCurrentContext bool
}

func Generate(host string, input []Input) (string, error) {
	k := KubeConfig{}
	caCert := caCertString()

	for _, entry := range input {
		clusterName := entry.Cluster.Spec.DisplayName
		if clusterName == "" {
			clusterName = entry.ClusterID
		}

		k.Clusters = append(k.Clusters, Cluster{
			Name:   clusterName,
			Server: fmt.Sprintf("https://%s/k8s/clusters/%s", host, entry.ClusterID),
			Cert:   caCert,
		})
		k.Users = append(k.Users, User{
			Name:      clusterName,
			Token:     entry.TokenKey,
			ClusterID: entry.ClusterID,
			Host:      host,
		})
		k.Contexts = append(k.Contexts, Context{
			Name:    clusterName,
			Cluster: clusterName,
			User:    clusterName,
		})
		if entry.IsCurrentContext {
			k.CurrentContext = clusterName
		}

		if !entry.Cluster.Spec.LocalClusterAuthEndpoint.Enabled {
			continue
		}

		if authEndpoint := entry.Cluster.Spec.LocalClusterAuthEndpoint; authEndpoint.FQDN != "" {
			fqdnName := clusterName + "-fqdn"
			k.Clusters = append(k.Clusters, Cluster{
				Name:   fqdnName,
				Server: "https://" + authEndpoint.FQDN,
				Cert:   formatCertString(base64.StdEncoding.EncodeToString([]byte(authEndpoint.CACerts))),
			})
			k.Contexts = append(k.Contexts, Context{
				Name:    fqdnName,
				Cluster: fqdnName,
				User:    clusterName,
			})

			continue
		}

		for _, clusterNode := range entry.Nodes {
			if clusterNode.Spec.ControlPlane {
				nodeName := clusterName + "-" + strings.TrimPrefix(clusterNode.Spec.RequestedHostname, clusterName+"-")
				k.Clusters = append(k.Clusters, Cluster{
					Name:   nodeName,
					Server: "https://" + node.GetEndpointNodeIP(clusterNode) + ":6443",
					Cert:   formatCertString(entry.Cluster.Status.CACert),
				})
				k.Contexts = append(k.Contexts, Context{
					Name:    nodeName,
					Cluster: nodeName,
					User:    clusterName,
				})
			}
		}
	}

	if k.CurrentContext == "" {
		k.CurrentContext = k.Contexts[0].Name
	}

	buf := &bytes.Buffer{}
	err := multiClusterTemplate.Execute(buf, k)

	return buf.String(), err
}
