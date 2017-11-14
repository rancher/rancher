rke
========

Rancher Kubernetes Engine, an extremely simple, lightning fast Kubernetes installer that works everywhere.

## Download

Please check the [releases](https://github.com/rancher/rke/releases/) page.

## Requirements
- All cluster nodes should have Docker version 1.12 installed.
- The SSH user used for node access must be a member of the `docker` group:
```bash
usermod -aG docker <user_name>
```

## Running
Standing up a Kubernetes is as simple as creating a `cluster.yml` configuration file and and running the command:
```bash
./rke up --config cluster.yml
```
##### Full `cluster.yaml` example

```
---
auth:
  strategy: x509
  options:
    foo: bar
# supported plugins are:
# flannel
# calico
# canal
network:
  plugin: flannel
  options:
    foo: bar

nodes:
  - address: 1.1.1.1
    user: ubuntu
    role: [controlplane, etcd]
  - address: 2.2.2.2
    user: ubuntu
    role: [worker]
  - address: host1.example.com
    user: ubuntu
    role: [worker]
    hostname_override: node3
    internal_address: 192.168.1.6

services:
  etcd:
    image: quay.io/coreos/etcd:latest
  kube-api:
    image: rancher/k8s:v1.8.3-rancher2
    service_cluster_ip_range: 10.233.0.0/18
    extra_args:
      v: 4
  kube-controller:
    image: rancher/k8s:v1.8.3-rancher2
    cluster_cidr: 10.233.64.0/18
    service_cluster_ip_range: 10.233.0.0/18
  scheduler:
    image: rancher/k8s:v1.8.3-rancher2
  kubelet:
    image: rancher/k8s:v1.8.3-rancher2
    cluster_domain: cluster.local
    cluster_dns_server: 10.233.0.3
    infra_container_image: gcr.io/google_containers/pause-amd64:3.0
  kubeproxy:
    image: rancher/k8s:v1.8.3-rancher2

# all addon manifests MUST specify a namespace
addons: |-
    ---
    apiVersion: v1
    kind: Pod
    metadata:
      name: my-nginx
      namespace: default
    spec:
      containers:
      - name: my-nginx
        image: nginx
        ports:
        - containerPort: 80
```

## More details

More information about RKE design, configuration and usage can be found in this [blog post](http://rancher.com/an-introduction-to-rke/).


## License
Copyright (c) 2017 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
