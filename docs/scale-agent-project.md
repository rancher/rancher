
# Feature: Scale Rancher Cluster Agent

## Overview

The normal/existing Rancher agent we build (check the repo directory, file final-minimal-agent.diff, in which we compiled a smaller image, a minimal agent build) a cluster agenet image and upload to the registry. The agent image will be launched later by applying the kubectl to the yaml file.

We need to first in the program, given the rancher server URL, and user's bearer token, and the cluster name user want to create, it will create a cluster in Rnacher server, and get the URL for importing cluster to the Rancher server. Curl to the URL of importing will return a yaml file which can be used to apply through kubectl to create the cluster agent.

The agent image will be run in the cluster as a deployment, and one copy of the pod is created. the pod will further communicate with the Rancher server for download further agents, webhook operations, etc. The agent then also responsible for collect the cluster information, such as node, pod, service, secret, etc information, and send it back to the Rancher server.

This feature is to create a special agent program, launches websocket to the Rancher server, just as in the normal Rancher agent, and get the instructions from the Rancher server.

This scale cluster-agent program does not have a cluster to run on, it does not have a kubernetes cluster to run on, it is a standalone program, which can be run on any machine, such as a laptop, desktop, or server. It will connect to the Rancher server and get the instructions from the Rancher server. This program will send fake cluster information as defined in the input of this program (besides the rancher server URL and user's bearer token, the cluster name, and the cluster information such as node, pod, service, secret, etc information), and send it back to the Rancher server. We can create a text file to generate one line for each cluster. say we have two nodes, then auto generate node names for to nodes, cpu, and memory information, pods running, etc. info).

Study the normal rancher cluster agent code, to include all the necessary code to communicate with the Rancher server, such as the websocket connection, the authentication, and the sending of cluster information.

From rancher server point of view, it is communicating through the websocket to a downstream real kubernetes cluster agent, which is running on a real kubernetes cluster. The Rancher server will not know the difference between the real cluster agent and this scale cluster agent.

First lets do this cluster-agent part first, later we need to extend to simulate other agents, such as fleet-agent, etc.

## File Structure

create a new directory under the scale-agent-project directory, name it `scale-cluster-agent`. which will contain necessary golang files to build the scale cluster agent program. and README.md file to describe the project.

extend the 'Makefile' in the repo, and we can use 'make scale-cluster-agent' to build the scale cluster agent program. The program will be built in the `bin/scale-cluster-agent` directory.
the program can be just a goland program, it does not need to be a container image.

we can use a created directory in my home directory, such as `~/.scale-cluster-agent/config` to store the configuration file, such as rancher server URL, user's bearer token, etc. The program will read the configuration file from this directory. The 'cluster-name' is passed in as a program's input.

## cluster description file

in the `~/.scale-cluster-agent/config` directory, we can create a file named `cluster.yaml`, which contains the cluster information, such as node, pod, service, secret, etc. The program will read this file and send the information to the Rancher server. for each new cluster this program creates, we can extend the node-name, pod-name, etc w/ the cluster name we specified in the program RESTful api param input. Please create a sample 'cluster.yaml' file to use. I can muanlly modify it if needed.

the cluster should be a k3s cluster, here is some sample cluster information:

kubectl get node
NAME          STATUS   ROLES                       AGE   VERSION
plex1-7050m   Ready    control-plane,etcd,master   42h   v1.28.5+k3s1   10.244.244.1   <none>        0.0.0-poc-k3s-june11-aca5d463-dirty-2025-07-31.04.42-kubevirt-amd64   6.1.112-linuxkit-63f4d774fbc8   containerd://1.7.11-k3s2
plex3-7050m   Ready    control-plane,etcd,master   39h   v1.28.5+k3s1   10.244.244.3   <none>        0.0.0-poc-k3s-june11-aca5d463-dirty-2025-07-31.04.42-kubevirt-amd64   6.1.112-linuxkit-63f4d774fbc8   containerd://1.7.11-k3s2


kubectl get pod -A
kube-system       coredns-6799fbcd5-lgj8v                             1/1     Running     3 (39h ago)    42h   10.42.0.97     plex1-7050m   <none>           <none>
kube-system       descheduler-job-x45vh                               0/1     Completed   0              39h   <none>         plex1-7050m   <none>           <none>
kube-system       helm-install-traefik-977w7                          0/1     Completed   1              42h   <none>         plex1-7050m   <none>           <none>
kube-system       helm-install-traefik-crd-kjn5g                      0/1     Completed   0              42h   <none>         plex1-7050m   <none>           <none>
kube-system       kube-multus-ds-vpmh9                                1/1     Running     3 (39h ago)    42h   10.244.244.1   plex1-7050m   <none>           <none>
kube-system       kube-multus-ds-wtnxz                                1/1     Running     0              39h   10.244.244.3   plex3-7050m   <none>           <none>
kube-system       metrics-server-67c658944b-2dsk5                     1/1     Running     3 (39h ago)    42h   10.42.0.110    plex1-7050m   <none>           <none>
kube-system       svclb-traefik-731eb113-cxmk7                        2/2     Running     6 (39h ago)    42h   10.42.0.106    plex1-7050m   <none>           <none>
kube-system       svclb-traefik-731eb113-m66zx                        2/2     Running     0              39h   10.42.1.2      plex3-7050m   <none>           <none>
kube-system       traefik-f4564c4f4-xz9t5                             1/1     Running     3 (39h ago)    42h   10.42.0.105    plex1-7050m   <none>           <none>

kubectl get svc -A -o wide
default           kubernetes                    ClusterIP      10.43.0.1       <none>                      443/TCP                      42h   <none>
kube-system       kube-dns                      ClusterIP      10.43.0.10      <none>                      53/UDP,53/TCP,9153/TCP       42h   k8s-app=kube-dns
kube-system       metrics-server                ClusterIP      10.43.73.33     <none>                      443/TCP                      42h   k8s-app=metrics-server
kube-system       traefik                       LoadBalancer   10.43.185.148   10.244.244.1,10.244.244.3   80:32522/TCP,443:32443/TCP   42h   app.kubernetes.io/instance=traefik-kube-system,app.kubernetes.io/name=traefik

We need some non-k3s items, like deployment of nginx, grafana, etc. to simulate the real cluster

## Scalable Testing

The user can use a script to run this program, and pass in the cluster name, and the program will send request to create a cluster in the Rancher server, and from the URL, get the yaml and download the yaml file for importing. then the program launches the websocket to the rancher server, and send the cluster information to the Rancher server. The Rancher server will not know the difference between the real cluster agent and this scale cluster agent.

So we don't run this program for every cluster user create. But this program is run as a daemon. It will accept the user cluster creation input through a JSON with http REST API, which specifies the cluster name. The program will launch and maintain the websocket connection, and periodically report the status of the cluster and download request. For the first phase, we only need to make rancher server to 'see' this cluster as live.

The Goal is to scalablly creating thousands of clusters for rancher server to manage, and test the rancher server's scalability and finding out the performance bottleneck on the rancher server.

## Critical Requirement: Active Cluster State

**IMPORTANT**: The clusters created by this program must become **ACTIVE** on the Rancher server side, not just "pending". This means:

1. **Complete Cluster Registration**: The import YAML must be successfully applied to make clusters active
2. **Successful WebSocket Connections**: The agent must establish real WebSocket connections to Rancher
3. **Data Transmission**: The agent must successfully send simulated cluster data to Rancher
4. **Rancher Server Recognition**: Rancher must see these clusters as live, active clusters

**Current Status**: The program creates clusters but they remain in "pending" state and WebSocket connections fail. This is NOT sufficient for performance testing.

**Required Fix**: Implement proper cluster registration that makes clusters truly active on the Rancher server.

## Config file

I have created the config file:
☁  .scale-cluster-agent  cat config 
RancherURL:https://green-cluster.shen.nu/
BearerToken:token-xb8fs:7fx9fjjjmr5w5t...

Please use this to communicate with my rancher server.


## Real Cluster Agent Running in Cluster Register With Ranch Logging

the log file is in 'docs/real-cluster-agent-logging.md'

## Another Cluster Agent Running in Cluster with More Debugging

the log file is in 'docs/second-debug-logging.md'

## Comparison logs for successful run w/ real agent/cluster and our simulation agent

### the yaml file, cluster-agent logs, and rancher server logs for real agent

saved at 'docs/Detail-Success-agent-register-logs.md'

### the rancher logs for our simulation agent for 'test-cluster-007'

saved at 'docs/Rancher-logs-For-test-cluster-007.md'
