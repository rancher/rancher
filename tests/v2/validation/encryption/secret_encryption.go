package encryption

import (
	steveclient "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/secrets"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	v1 "k8s.io/api/core/v1"
)

type SSHConfig struct {
	User string `json:"user" yaml:"user"`
}

const (
	secretKey                  = "testKey"
	secretValue                = "testValue"
	rke2EtcdEncryptionCheckCmd = `sudo ETCDCTL_API=3 etcdctl --cert /var/lib/rancher/rke2/server/tls/etcd/server-client.crt --key /var/lib/rancher/rke2/server/tls/etcd/server-client.key \
					--endpoints https://127.0.0.1:2379 --cacert /var/lib/rancher/rke2/server/tls/etcd/server-ca.crt get /registry/secrets/default/%s| hexdump -C`
	k3sEtcdEncryptionCheckCmd = `sudo ETCDCTL_API=3 etcdctl --cert /var/lib/rancher/k3s/server/tls/etcd/server-client.crt --key /var/lib/rancher/k3s/server/tls/etcd/server-client.key \
					--endpoints https://127.0.0.1:2379 --cacert /var/lib/rancher/k3s/server/tls/etcd/server-ca.crt get /registry/secrets/default/%s| hexdump -C`
	SSHConfigFileKey  = "sshConfig"
	EtcdCtlInstallCmd = `ETCD_VER=v3.5.0 && GOOGLE_URL=https://storage.googleapis.com/etcd && GITHUB_URL=https://github.com/etcd-io/etcd/releases/download &&
	DOWNLOAD_URL=${GOOGLE_URL} && rm -f /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz && rm -rf /tmp/etcd-download-test && mkdir -p /tmp/etcd-download-test &&
	curl -L ${DOWNLOAD_URL}/${ETCD_VER}/etcd-${ETCD_VER}-linux-amd64.tar.gz -o /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz &&
	tar xzvf /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz -C /tmp/etcd-download-test --strip-components=1 && rm -f /tmp/etcd-${ETCD_VER}-linux-amd64.tar.gz &&
	/tmp/etcd-download-test/etcd --version && /tmp/etcd-download-test/etcdctl version && /tmp/etcd-download-test/etcdutl version &&
	sudo cp /tmp/etcd-download-test/etcdctl /usr/local/bin/etcdctl && etcdctl version`
)

func createDeploymentWithSecret(steveClient *steveclient.Client, deploymentName string, secretName string, namespace string, podLabels map[string]string) (*steveclient.SteveAPIObject, *steveclient.SteveAPIObject, error) {
	secretTemplate := secrets.NewSecretTemplate(secretName, namespace, map[string][]byte{secretKey: []byte(secretValue)}, v1.SecretTypeOpaque)
	secretObj, err := steveClient.SteveType(secrets.SecretSteveType).Create(secretTemplate)
	if err != nil {
		return nil, nil, err
	}
	envVarFromSecret := []v1.EnvFromSource{
		{
			SecretRef: &v1.SecretEnvSource{
				LocalObjectReference: v1.LocalObjectReference{Name: secretObj.Name},
			},
		},
	}
	containerTemplate := workloads.NewContainer("ranchertest", "ranchertest/mytestcontainer", v1.PullAlways, []v1.VolumeMount{}, envVarFromSecret, nil, nil, nil)
	podTemplate := workloads.NewPodTemplate([]v1.Container{containerTemplate}, []v1.Volume{}, []v1.LocalObjectReference{}, podLabels)
	deploymentTemplate := workloads.NewDeploymentTemplate(deploymentName, namespace, podTemplate, true, podLabels)
	deploymentObj, err := steveClient.SteveType(workloads.DeploymentSteveType).Create(deploymentTemplate)
	if err != nil {
		return nil, nil, err
	}

	return secretObj, deploymentObj, nil
}
