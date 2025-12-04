# Native CAPI Infrastructure Provider Support

This document describes how to use native Cluster API (CAPI) infrastructure providers with Rancher's v2prov (provisioning v2) system instead of the default `rancher/machine`-based provisioning.

## Overview

Rancher's provisioning v2 system supports two modes of machine provisioning:

1. **Rancher Machine-based Provisioning** (default): Uses `rancher/machine` to provision VMs through node drivers. This creates resources like `VmwarevsphereConfig` and `VmwarevsphereMachineTemplate` in the `rke-machine-config.cattle.io/v1` and `rke-machine.cattle.io/v1` API groups.

2. **Native CAPI Infrastructure Provider Provisioning**: Uses native CAPI infrastructure providers like CAPV (Cluster API Provider vSphere), CAPA (Cluster API Provider AWS), etc. This allows users to leverage the full functionality of these providers while still using Rancher for cluster management.

## Prerequisites

Before using native CAPI infrastructure providers, ensure:

1. The CAPI infrastructure provider is installed in your Rancher management cluster
2. The necessary CRDs for the infrastructure provider are available
3. Credentials and configurations required by the infrastructure provider are properly set up

## Using Native CAPI Infrastructure Providers

### Cluster-Level Infrastructure

To use a native CAPI infrastructure cluster instead of the default `RKECluster`:

1. Create your infrastructure cluster resource (e.g., `VSphereCluster` for CAPV)
2. Reference it in your `provisioning.cattle.io/v1` Cluster spec using `infrastructureRef`

Example with CAPV (VSphereCluster):

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereCluster
metadata:
  name: my-cluster-infra
  namespace: fleet-default
spec:
  controlPlaneEndpoint:
    host: 10.0.0.100
    port: 6443
  server: vcenter.example.com
  thumbprint: "..."
  identityRef:
    kind: Secret
    name: vsphere-credentials
---
apiVersion: provisioning.cattle.io/v1
kind: Cluster
metadata:
  name: my-cluster
  namespace: fleet-default
spec:
  kubernetesVersion: v1.28.0+rke2r1
  rkeConfig:
    infrastructureRef:
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
      kind: VSphereCluster
      name: my-cluster-infra
      namespace: fleet-default
    machinePools:
      - name: pool1
        etcdRole: true
        controlPlaneRole: true
        workerRole: true
        quantity: 3
        machineConfigRef:
          apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
          kind: VSphereMachineTemplate
          name: my-cluster-machines
```

### Machine Pool-Level Infrastructure

Each machine pool can reference either:
- A Rancher machine config (`rke-machine-config.cattle.io/v1`)
- A native CAPI machine template

Example with CAPV (VSphereMachineTemplate):

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereMachineTemplate
metadata:
  name: my-cluster-machines
  namespace: fleet-default
spec:
  template:
    spec:
      cloneMode: linkedClone
      datacenter: dc0
      datastore: datastore0
      diskGiB: 40
      folder: /dc0/vm/folder0
      memoryMiB: 8192
      network:
        devices:
          - dhcp4: true
            networkName: network0
      numCPUs: 4
      resourcePool: /dc0/host/cluster0/Resources/pool0
      server: vcenter.example.com
      template: /dc0/vm/templates/ubuntu-2204-kube-v1.28.0
      thumbprint: "..."
```

## Supported Infrastructure Providers

The following CAPI infrastructure providers have been tested with Rancher v2prov:

| Provider | API Group | Infrastructure Cluster | Machine Template |
|----------|-----------|----------------------|------------------|
| CAPV (vSphere) | infrastructure.cluster.x-k8s.io/v1beta1 | VSphereCluster | VSphereMachineTemplate |
| CAPA (AWS) | infrastructure.cluster.x-k8s.io/v1beta2 | AWSCluster | AWSMachineTemplate |
| CAPZ (Azure) | infrastructure.cluster.x-k8s.io/v1beta1 | AzureCluster | AzureMachineTemplate |

## Considerations

### Bootstrap Provider

When using native CAPI infrastructure providers, Rancher still uses its own bootstrap provider (`RKEBootstrap`) to:
- Install RKE2/K3s on provisioned machines
- Configure the Kubernetes components
- Handle node labels and taints

### Control Plane

The `RKEControlPlane` is always used regardless of the infrastructure provider. This ensures Rancher maintains control over:
- Kubernetes version management
- Cluster upgrades
- Etcd backup and restore
- Certificate rotation

### Machine Address Discovery

For native CAPI infrastructure providers, machine addresses are discovered from the infrastructure machine's status rather than from the rancher/machine state secret. Ensure your infrastructure provider properly populates the machine addresses in the status.

### Cloud Credentials

When using native CAPI infrastructure providers:
- The `cloudCredentialSecretName` field in the cluster spec is ignored for infrastructure provisioning
- Credentials must be configured according to the infrastructure provider's requirements (typically via Kubernetes Secrets referenced by the infrastructure resources)

## Troubleshooting

### Infrastructure Cluster Not Ready

If the CAPI cluster shows `InfrastructureReady: false`:
1. Check the infrastructure cluster resource for conditions and errors
2. Verify the infrastructure provider controller is running
3. Check credentials and network connectivity to the infrastructure

### Machines Not Provisioning

If machines are not being created:
1. Verify the machine template exists and is valid
2. Check the machine deployment events for errors
3. Review the infrastructure provider controller logs

## Migration from Rancher Machine

To migrate existing clusters from rancher/machine to native CAPI providers:

1. This is currently not supported for in-place migration
2. Create a new cluster with native CAPI infrastructure
3. Migrate workloads to the new cluster
4. Decommission the old cluster
