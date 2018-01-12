package client

import (
	"github.com/rancher/norman/clientbase"
)

type Client struct {
	clientbase.APIBaseClient

	Namespace                     NamespaceOperations
	PersistentVolumeClaim         PersistentVolumeClaimOperations
	Ingress                       IngressOperations
	Secret                        SecretOperations
	ServiceAccountToken           ServiceAccountTokenOperations
	DockerCredential              DockerCredentialOperations
	Certificate                   CertificateOperations
	BasicAuth                     BasicAuthOperations
	SSHAuth                       SSHAuthOperations
	NamespacedSecret              NamespacedSecretOperations
	NamespacedServiceAccountToken NamespacedServiceAccountTokenOperations
	NamespacedDockerCredential    NamespacedDockerCredentialOperations
	NamespacedCertificate         NamespacedCertificateOperations
	NamespacedBasicAuth           NamespacedBasicAuthOperations
	NamespacedSSHAuth             NamespacedSSHAuthOperations
	Service                       ServiceOperations
	DNSRecord                     DNSRecordOperations
	Pod                           PodOperations
	Deployment                    DeploymentOperations
	StatefulSet                   StatefulSetOperations
	ReplicaSet                    ReplicaSetOperations
	ReplicationController         ReplicationControllerOperations
	DaemonSet                     DaemonSetOperations
	Workload                      WorkloadOperations
	ConfigMap                     ConfigMapOperations
}

func NewClient(opts *clientbase.ClientOpts) (*Client, error) {
	baseClient, err := clientbase.NewAPIClient(opts)
	if err != nil {
		return nil, err
	}

	client := &Client{
		APIBaseClient: baseClient,
	}

	client.Namespace = newNamespaceClient(client)
	client.PersistentVolumeClaim = newPersistentVolumeClaimClient(client)
	client.Ingress = newIngressClient(client)
	client.Secret = newSecretClient(client)
	client.ServiceAccountToken = newServiceAccountTokenClient(client)
	client.DockerCredential = newDockerCredentialClient(client)
	client.Certificate = newCertificateClient(client)
	client.BasicAuth = newBasicAuthClient(client)
	client.SSHAuth = newSSHAuthClient(client)
	client.NamespacedSecret = newNamespacedSecretClient(client)
	client.NamespacedServiceAccountToken = newNamespacedServiceAccountTokenClient(client)
	client.NamespacedDockerCredential = newNamespacedDockerCredentialClient(client)
	client.NamespacedCertificate = newNamespacedCertificateClient(client)
	client.NamespacedBasicAuth = newNamespacedBasicAuthClient(client)
	client.NamespacedSSHAuth = newNamespacedSSHAuthClient(client)
	client.Service = newServiceClient(client)
	client.DNSRecord = newDNSRecordClient(client)
	client.Pod = newPodClient(client)
	client.Deployment = newDeploymentClient(client)
	client.StatefulSet = newStatefulSetClient(client)
	client.ReplicaSet = newReplicaSetClient(client)
	client.ReplicationController = newReplicationControllerClient(client)
	client.DaemonSet = newDaemonSetClient(client)
	client.Workload = newWorkloadClient(client)
	client.ConfigMap = newConfigMapClient(client)

	return client, nil
}
