package templates

const CanalTemplate = `
{{if eq .RBACConfig "rbac"}}
---
# Calico Roles
# Pulled from https://docs.projectcalico.org/v2.5/getting-started/kubernetes/installation/hosted/rbac-kdd.yaml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: calico
rules:
  - apiGroups: [""]
    resources:
      - namespaces
    verbs:
      - get
      - list
      - watch
  - apiGroups: [""]
    resources:
      - pods/status
    verbs:
      - update
  - apiGroups: [""]
    resources:
      - pods
    verbs:
      - get
      - list
      - watch
  - apiGroups: [""]
    resources:
      - nodes
    verbs:
      - get
      - list
      - update
      - watch
  - apiGroups: ["extensions"]
    resources:
      - networkpolicies
    verbs:
      - get
      - list
      - watch
  - apiGroups: ["crd.projectcalico.org"]
    resources:
      - globalfelixconfigs
      - bgppeers
      - globalbgpconfigs
      - ippools
      - globalnetworkpolicies
    verbs:
      - create
      - get
      - list
      - update
      - watch

---

# Flannel roles
# Pulled from https://github.com/coreos/flannel/blob/master/Documentation/kube-flannel-rbac.yml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: flannel
rules:
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
  - apiGroups:
      - ""
    resources:
      - nodes
    verbs:
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - nodes/status
    verbs:
      - patch
---

# Bind the flannel ClusterRole to the canal ServiceAccount.
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: canal-flannel
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: flannel
subjects:
- kind: ServiceAccount
  name: canal
  namespace: kube-system

---

# Bind the calico ClusterRole to the canal ServiceAccount.
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: canal-calico
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: calico
subjects:
- kind: ServiceAccount
  name: canal
  namespace: kube-system

## end rbac
{{end}}

---
# This ConfigMap can be used to configure a self-hosted Canal installation.
kind: ConfigMap
apiVersion: v1
metadata:
  name: canal-config
  namespace: kube-system
data:
  # The interface used by canal for host <-> host communication.
  # If left blank, then the interface is chosen using the node's
  # default route.
  canal_iface: ""

  # Whether or not to masquerade traffic to destinations not within
  # the pod network.
  masquerade: "true"

  # The CNI network configuration to install on each node.
  cni_network_config: |-
    {
        "name": "rke-pod-network",
        "cniVersion": "0.3.0",
        "plugins": [
            {
                "type": "calico",
                "log_level": "info",
                "datastore_type": "kubernetes",
                "nodename": "__KUBERNETES_NODE_NAME__",
                "ipam": {
                    "type": "host-local",
                    "subnet": "usePodCidr"
                },
                "policy": {
                    "type": "k8s",
                    "k8s_api_root": "{{.APIRoot}}",
                    "k8s_client_certificate": "{{.ClientCertPath}}",
                    "k8s_client_key": "{{.ClientKeyPath}}",
                    "k8s_certificate_authority": "{{.ClientCAPath}}"
                },
                "kubernetes": {
                    "kubeconfig": "{{.KubeCfg}}"
                }
            },
            {
                "type": "portmap",
                "capabilities": {"portMappings": true},
                "snat": true
            }
        ]
    }

  # Flannel network configuration. Mounted into the flannel container.
  net-conf.json: |
    {
      "Network": "{{.ClusterCIDR}}",
      "Backend": {
        "Type": "vxlan"
      }
    }

---

# This manifest installs the calico/node container, as well
# as the Calico CNI plugins and network config on
# each master and worker node in a Kubernetes cluster.
kind: DaemonSet
apiVersion: extensions/v1beta1
metadata:
  name: canal
  namespace: kube-system
  labels:
    k8s-app: canal
spec:
  selector:
    matchLabels:
      k8s-app: canal
  template:
    metadata:
      labels:
        k8s-app: canal
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      hostNetwork: true
      serviceAccountName: canal
      tolerations:
        # this taint is set by all kubelets running '--cloud-provider=external'
        # so we should tolerate it to schedule the canal pods
        - key: node.cloudprovider.kubernetes.io/uninitialized
          value: "true"
          effect: NoSchedule
        # Allow the pod to run on the master abd etcd.  This is required for
        # the master to communicate with pods.
        - key: "node-role.kubernetes.io/master"
          operator: "Exists"
        - key: "node-role.kubernetes.io/etcd"
          operator: "Exists"
          effect: "NoExecute"
        # Mark the pod as a critical add-on for rescheduling.
        - key: "CriticalAddonsOnly"
          operator: "Exists"
      # Minimize downtime during a rolling upgrade or deletion; tell Kubernetes to do a "force
      # deletion": https://kubernetes.io/docs/concepts/workloads/pods/pod/#termination-of-pods.
      terminationGracePeriodSeconds: 0
      containers:
        # Runs calico/node container on each Kubernetes node.  This
        # container programs network policy and routes on each
        # host.
        - name: calico-node
          image: {{.NodeImage}}
          env:
            # Use Kubernetes API as the backing datastore.
            - name: DATASTORE_TYPE
              value: "kubernetes"
            # Enable felix logging.
            - name: FELIX_LOGSEVERITYSYS
              value: "info"
            # Don't enable BGP.
            - name: CALICO_NETWORKING_BACKEND
              value: "none"
            # Cluster type to identify the deployment type
            - name: CLUSTER_TYPE
              value: "k8s,canal"
            # Disable file logging so 'kubectl logs' works.
            - name: CALICO_DISABLE_FILE_LOGGING
              value: "true"
            # Period, in seconds, at which felix re-applies all iptables state
            - name: FELIX_IPTABLESREFRESHINTERVAL
              value: "60"
            # Disable IPV6 support in Felix.
            - name: FELIX_IPV6SUPPORT
              value: "false"
            # Wait for the datastore.
            - name: WAIT_FOR_DATASTORE
              value: "true"
            # No IP address needed.
            - name: IP
              value: ""
            - name: NODENAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            # Set Felix endpoint to host default action to ACCEPT.
            - name: FELIX_DEFAULTENDPOINTTOHOSTACTION
              value: "ACCEPT"
            - name: FELIX_HEALTHENABLED
              value: "true"
          securityContext:
            privileged: true
          resources:
            requests:
              cpu: 250m
          livenessProbe:
            httpGet:
              path: /liveness
              port: 9099
            periodSeconds: 10
            initialDelaySeconds: 10
            failureThreshold: 6
          readinessProbe:
            httpGet:
              path: /readiness
              port: 9099
            periodSeconds: 10
          volumeMounts:
            - mountPath: /lib/modules
              name: lib-modules
              readOnly: true
            - mountPath: /var/run/calico
              name: var-run-calico
              readOnly: false
            - mountPath: /etc/kubernetes
              name: etc-kubernetes
        # This container installs the Calico CNI binaries
        # and CNI network config file on each node.
        - name: install-cni
          image: {{.CNIImage}}
          command: ["/install-cni.sh"]
          env:
            - name: CNI_CONF_NAME
              value: "10-calico.conflist"
            # The CNI network config to install on each node.
            - name: CNI_NETWORK_CONFIG
              valueFrom:
                configMapKeyRef:
                  name: canal-config
                  key: cni_network_config
            - name: KUBERNETES_NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          volumeMounts:
            - mountPath: /host/opt/cni/bin
              name: cni-bin-dir
            - mountPath: /host/etc/cni/net.d
              name: cni-net-dir
            - mountPath: /etc/kubernetes
              name: etc-kubernetes
        # This container runs flannel using the kube-subnet-mgr backend
        # for allocating subnets.
        - name: kube-flannel
          image: {{.CanalFlannelImg}}
          command: [ "/opt/bin/flanneld", "--ip-masq", "--kube-subnet-mgr" ]
          securityContext:
            privileged: true
          env:
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: FLANNELD_IFACE
              valueFrom:
                configMapKeyRef:
                  name: canal-config
                  key: canal_iface
            - name: FLANNELD_IP_MASQ
              valueFrom:
                configMapKeyRef:
                  name: canal-config
                  key: masquerade
          volumeMounts:
          - name: run
            mountPath: /run
          - name: flannel-cfg
            mountPath: /etc/kube-flannel/
      volumes:
        # Used by calico/node.
        - name: lib-modules
          hostPath:
            path: /lib/modules
        - name: var-run-calico
          hostPath:
            path: /var/run/calico
        # Used to install CNI.
        - name: cni-bin-dir
          hostPath:
            path: /opt/cni/bin
        - name: cni-net-dir
          hostPath:
            path: /etc/cni/net.d
        # Used by flannel.
        - name: run
          hostPath:
            path: /run
        - name: flannel-cfg
          configMap:
            name: canal-config
        - name: etc-kubernetes
          hostPath:
            path: /etc/kubernetes


# Create all the CustomResourceDefinitions needed for
# Calico policy-only mode.
---

apiVersion: apiextensions.k8s.io/v1beta1
description: Calico Global Felix Configuration
kind: CustomResourceDefinition
metadata:
   name: globalfelixconfigs.crd.projectcalico.org
spec:
  scope: Cluster
  group: crd.projectcalico.org
  version: v1
  names:
    kind: GlobalFelixConfig
    plural: globalfelixconfigs
    singular: globalfelixconfig

---

apiVersion: apiextensions.k8s.io/v1beta1
description: Calico Global BGP Configuration
kind: CustomResourceDefinition
metadata:
  name: globalbgpconfigs.crd.projectcalico.org
spec:
  scope: Cluster
  group: crd.projectcalico.org
  version: v1
  names:
    kind: GlobalBGPConfig
    plural: globalbgpconfigs
    singular: globalbgpconfig

---

apiVersion: apiextensions.k8s.io/v1beta1
description: Calico IP Pools
kind: CustomResourceDefinition
metadata:
  name: ippools.crd.projectcalico.org
spec:
  scope: Cluster
  group: crd.projectcalico.org
  version: v1
  names:
    kind: IPPool
    plural: ippools
    singular: ippool

---

apiVersion: apiextensions.k8s.io/v1beta1
description: Calico Global Network Policies
kind: CustomResourceDefinition
metadata:
  name: globalnetworkpolicies.crd.projectcalico.org
spec:
  scope: Cluster
  group: crd.projectcalico.org
  version: v1
  names:
    kind: GlobalNetworkPolicy
    plural: globalnetworkpolicies
    singular: globalnetworkpolicy

---

apiVersion: v1
kind: ServiceAccount
metadata:
  name: canal
  namespace: kube-system`
