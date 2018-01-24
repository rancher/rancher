package services

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/rancher/rke/docker"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/pki"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func runKubeAPI(ctx context.Context, host *hosts.Host, etcdHosts []*hosts.Host, kubeAPIService v3.KubeAPIService, authorizationMode string, df hosts.DialerFactory) error {
	etcdConnString := GetEtcdConnString(etcdHosts)
	imageCfg, hostCfg := buildKubeAPIConfig(host, kubeAPIService, etcdConnString, authorizationMode)
	if err := docker.DoRunContainer(ctx, host.DClient, imageCfg, hostCfg, KubeAPIContainerName, host.Address, ControlRole); err != nil {
		return err
	}
	return runHealthcheck(ctx, host, KubeAPIPort, true, KubeAPIContainerName, df)
}

func removeKubeAPI(ctx context.Context, host *hosts.Host) error {
	return docker.DoRemoveContainer(ctx, host.DClient, KubeAPIContainerName, host.Address)
}

func buildKubeAPIConfig(host *hosts.Host, kubeAPIService v3.KubeAPIService, etcdConnString, authorizationMode string) (*container.Config, *container.HostConfig) {
	imageCfg := &container.Config{
		Image: kubeAPIService.Image,
		Entrypoint: []string{"/opt/rke/entrypoint.sh",
			"kube-apiserver",
			"--insecure-bind-address=127.0.0.1",
			"--bind-address=0.0.0.0",
			"--insecure-port=0",
			"--secure-port=6443",
			"--cloud-provider=",
			"--allow_privileged=true",
			"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname",
			"--service-cluster-ip-range=" + kubeAPIService.ServiceClusterIPRange,
			"--admission-control=ServiceAccount,NamespaceLifecycle,LimitRanger,PersistentVolumeLabel,DefaultStorageClass,ResourceQuota,DefaultTolerationSeconds",
			"--runtime-config=batch/v2alpha1",
			"--runtime-config=authentication.k8s.io/v1beta1=true",
			"--storage-backend=etcd3",
			"--client-ca-file=" + pki.CACertPath,
			"--tls-cert-file=" + pki.KubeAPICertPath,
			"--tls-private-key-file=" + pki.KubeAPIKeyPath,
			"--service-account-key-file=" + pki.KubeAPIKeyPath},
	}
	imageCfg.Cmd = append(imageCfg.Cmd, "--etcd-servers="+etcdConnString)

	if authorizationMode == RBACAuthorizationMode {
		imageCfg.Cmd = append(imageCfg.Cmd, "--authorization-mode=RBAC")
	}
	if kubeAPIService.PodSecurityPolicy {
		imageCfg.Cmd = append(imageCfg.Cmd, "--runtime-config=extensions/v1beta1/podsecuritypolicy=true", "--admission-control=PodSecurityPolicy")
	}
	hostCfg := &container.HostConfig{
		VolumesFrom: []string{
			SidekickContainerName,
		},
		Binds: []string{
			"/etc/kubernetes:/etc/kubernetes",
		},
		NetworkMode:   "host",
		RestartPolicy: container.RestartPolicy{Name: "always"},
	}

	for arg, value := range kubeAPIService.ExtraArgs {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		imageCfg.Entrypoint = append(imageCfg.Entrypoint, cmd)
	}
	return imageCfg, hostCfg
}
