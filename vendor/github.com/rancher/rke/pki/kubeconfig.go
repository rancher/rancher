package pki

import "encoding/base64"

func getKubeConfigX509(kubernetesURL string, clusterName string, componentName string, caPath string, crtPath string, keyPath string) string {
	return `apiVersion: v1
kind: Config
clusters:
- cluster:
    api-version: v1
    certificate-authority: ` + caPath + `
    server: "` + kubernetesURL + `"
  name: "` + clusterName + `"
contexts:
- context:
    cluster: "` + clusterName + `"
    user: "` + componentName + `-` + clusterName + `"
  name: "` + clusterName + `"
current-context: "` + clusterName + `"
users:
- name: "` + componentName + `-` + clusterName + `"
  user:
    client-certificate: ` + crtPath + `
    client-key: ` + keyPath + ``
}

func GetKubeConfigX509WithData(kubernetesURL string, clusterName string, componentName string, cacrt string, crt string, key string) string {
	return `apiVersion: v1
kind: Config
clusters:
- cluster:
    api-version: v1
    certificate-authority-data: ` + base64.StdEncoding.EncodeToString([]byte(cacrt)) + `
    server: "` + kubernetesURL + `"
  name: "` + clusterName + `"
contexts:
- context:
    cluster: "` + clusterName + `"
    user: "` + componentName + `-` + clusterName + `"
  name: "` + clusterName + `"
current-context: "` + clusterName + `"
users:
- name: "` + componentName + `-` + clusterName + `"
  user:
    client-certificate-data: ` + base64.StdEncoding.EncodeToString([]byte(crt)) + `
    client-key-data: ` + base64.StdEncoding.EncodeToString([]byte(key)) + ``
}
