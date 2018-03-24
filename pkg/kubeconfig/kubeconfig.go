package kubeconfig

import (
	"bytes"
	"encoding/base64"
	"regexp"

	"github.com/rancher/rancher/pkg/settings"
)

const (
	certDelim = "\\\n      "
	firstLen  = 49
)

var (
	splitRegexp = regexp.MustCompile(`\S{1,76}`)
)

type data struct {
	ClusterName string
	Host        string
	ClusterID   string
	Cert        string
	User        string
	Username    string
	Password    string
	Token       string
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

func caCertString() string {
	buf := &bytes.Buffer{}

	certData := settings.CACerts.Get()
	if certData == "" {
		return ""
	}
	certData = base64.StdEncoding.EncodeToString([]byte(certData))

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

func ForTokenBased(clusterName, clusterID, host, username, token string) (string, error) {
	data := &data{
		ClusterName: clusterName,
		ClusterID:   clusterID,
		Host:        host,
		Cert:        caCertString(),
		User:        username,
		Token:       token,
	}

	if data.ClusterName == "" {
		data.ClusterName = data.ClusterID
	}

	buf := &bytes.Buffer{}
	err := tokenTemplate.Execute(buf, data)
	return buf.String(), err
}
