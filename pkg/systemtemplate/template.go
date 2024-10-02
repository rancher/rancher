package systemtemplate

var templateSource = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: proxy-clusterrole-kubeapiserver
rules:
- apiGroups: [""]
  resources:
  - nodes/metrics
  - nodes/proxy
  - nodes/stats
  - nodes/log
  - nodes/spec
  verbs: ["get", "list", "watch", "create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: proxy-role-binding-kubernetes-master
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: proxy-clusterrole-kubeapiserver
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: kube-apiserver
---
apiVersion: v1
kind: Namespace
metadata:
  name: cattle-system

---

apiVersion: v1
kind: ServiceAccount
metadata:
  name: cattle
  namespace: cattle-system

---

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cattle-admin-binding
  namespace: cattle-system
  labels:
    cattle.io/creator: "norman"
subjects:
- kind: ServiceAccount
  name: cattle
  namespace: cattle-system
roleRef:
  kind: ClusterRole
  name: cattle-admin
  apiGroup: rbac.authorization.k8s.io

---

apiVersion: v1
kind: Secret
metadata:
  name: cattle-credentials-{{.TokenKey}}
  namespace: cattle-system
type: Opaque
data:
  url: "{{.URL}}"
  token: "{{.Token}}"
  namespace: "{{.Namespace}}"

---

{{- if .PrivateRegistryConfig}}
apiVersion: v1
kind: Secret
metadata:
  name: cattle-private-registry
  namespace: cattle-system
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: "{{.PrivateRegistryConfig}}"

---
{{- end }}

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cattle-admin
  labels:
    cattle.io/creator: "norman"
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
- nonResourceURLs:
  - '*'
  verbs:
  - '*'

---

apiVersion: apps/v1
kind: Deployment
metadata:
  {{- if .IsPreBootstrap }}
  name: cattle-cluster-agent-bootstrap
  {{- else }}
  name: cattle-cluster-agent
  {{- end }}
  namespace: cattle-system
  annotations:
    management.cattle.io/scale-available: "2"
spec:
  selector:
    matchLabels:
      app: cattle-cluster-agent
  template:
    metadata:
      labels:
        app: cattle-cluster-agent
    spec:
      {{- if .Affinity }}
      affinity:
{{ .Affinity | indent 8 }}
      {{- end }}
      serviceAccountName: cattle
      tolerations:
      {{- if .IsPreBootstrap }}
      # tolerations wrt running on the pre-bootstrapped node
      - effect: NoSchedule
        key: node-role.kubernetes.io/controlplane
        value: "true"
      - effect: NoSchedule
        key: "node-role.kubernetes.io/control-plane"
        operator: "Exists"
      - effect: NoSchedule
        key: "node-role.kubernetes.io/master"
        operator: "Exists"
      - effect: NoExecute
        key: "node-role.kubernetes.io/etcd"
        operator: "Exists"
      - effect: NoSchedule
        key: node.cloudprovider.kubernetes.io/uninitialized
        operator: "Exists"
      {{- else if .Tolerations }}
      # Tolerations added based on found taints on controlplane nodes
{{ .Tolerations | indent 6 }}
      {{- else }}
      # No taints or no controlplane nodes found, added defaults
      - effect: NoSchedule
        key: node-role.kubernetes.io/controlplane
        value: "true"
      - effect: NoSchedule
        key: "node-role.kubernetes.io/control-plane"
        operator: "Exists"
      - effect: NoSchedule
        key: "node-role.kubernetes.io/master"
        operator: "Exists"
      {{- end }}
      {{- if .AppendTolerations }}
{{ .AppendTolerations | indent 6 }}
      {{- end }}
      containers:
        - name: cluster-register
          imagePullPolicy: IfNotPresent
          {{- if .ResourceRequirements }}
          resources:
{{ .ResourceRequirements | indent 12 }}
          {{- end }}
          env:
          {{- if ne .Features "" }}
          - name: CATTLE_FEATURES
            value: "{{.Features}}"
          {{- end }}
          - name: CATTLE_IS_RKE
            value: "{{.IsRKE}}"
          - name: CATTLE_SERVER
            value: "{{.URLPlain}}"
          - name: CATTLE_CA_CHECKSUM
            value: "{{.CAChecksum}}"
          - name: CATTLE_CLUSTER
            value: "true"
          - name: CATTLE_K8S_MANAGED
            value: "true"
          - name: CATTLE_CLUSTER_REGISTRY
            value: "{{.ClusterRegistry}}"
          {{- if .IsPreBootstrap }}
          # since we're on the host network, talk to the apiserver over localhost
          {{- end }}
      {{- if .AgentEnvVars}}
{{ .AgentEnvVars | indent 10 }}
      {{- end }}
          image: {{.AgentImage}}
          volumeMounts:
          - name: cattle-credentials
            mountPath: /cattle-credentials
            readOnly: true
      {{- if .PrivateRegistryConfig}}
      imagePullSecrets:
      - name: cattle-private-registry
      {{- end }}
      {{- if .IsPreBootstrap }}
      # use hostNetwork since the CNI (and coreDNS) is not up yet
      hostNetwork: true
      {{- end }}
      volumes:
      - name: cattle-credentials
        secret:
          secretName: cattle-credentials-{{.TokenKey}}
          defaultMode: 320
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
{{ if .IsRKE }}

---

apiVersion: apps/v1
kind: DaemonSet
metadata:
    name: cattle-node-agent
    namespace: cattle-system
spec:
  selector:
    matchLabels:
      app: cattle-agent
  template:
    metadata:
      labels:
        app: cattle-agent
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: beta.kubernetes.io/os
                  operator: NotIn
                  values:
                    - windows
      hostNetwork: true
      serviceAccountName: cattle
      tolerations:
      - operator: Exists
      containers:
      - name: agent
        image: {{.AgentImage}}
        imagePullPolicy: IfNotPresent
        env:
        - name: CATTLE_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: CATTLE_SERVER
          value: "{{.URLPlain}}"
        - name: CATTLE_CA_CHECKSUM
          value: "{{.CAChecksum}}"
        - name: CATTLE_CLUSTER
          value: "false"
        - name: CATTLE_K8S_MANAGED
          value: "true"
        - name: CATTLE_AGENT_CONNECT
          value: "true"
      {{- if .AgentEnvVars}}
{{ .AgentEnvVars | indent 8 }}
      {{- end }}
        volumeMounts:
        - name: cattle-credentials
          mountPath: /cattle-credentials
          readOnly: true
        - name: k8s-ssl
          mountPath: /etc/kubernetes
        - name: var-run
          mountPath: /var/run
          mountPropagation: HostToContainer
        - name: run
          mountPath: /run
          mountPropagation: HostToContainer
        - name: docker-certs
          mountPath: /etc/docker/certs.d
        securityContext:
          privileged: true
      {{- if .PrivateRegistryConfig}}
      imagePullSecrets:
      - name: cattle-private-registry
      {{- end }}
      volumes:
      - name: k8s-ssl
        hostPath:
          path: /etc/kubernetes
          type: DirectoryOrCreate
      - name: var-run
        hostPath:
          path: /var/run
          type: DirectoryOrCreate
      - name: run
        hostPath:
          path: /run
          type: DirectoryOrCreate
      - name: cattle-credentials
        secret:
          secretName: cattle-credentials-{{.TokenKey}}
          defaultMode: 320
      - hostPath:
          path: /etc/docker/certs.d
          type: DirectoryOrCreate
        name: docker-certs
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 50%

{{- end }}

{{- if .IsWindowsCluster}}

---

apiVersion: apps/v1
kind: DaemonSet
metadata:
    name: cattle-node-agent-windows
    namespace: cattle-system
spec:
  selector:
    matchLabels:
      app: cattle-agent-windows
  template:
    metadata:
      labels:
        app: cattle-agent-windows
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: beta.kubernetes.io/os
                  operator: NotIn
                  values:
                    - linux
      serviceAccountName: cattle
      tolerations:
      - operator: Exists
      containers:
      - name: agent
        image: {{.AgentImage}}
        imagePullPolicy: IfNotPresent
        env:
        - name: CATTLE_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: CATTLE_SERVER
          value: "{{.URLPlain}}"
        - name: CATTLE_CA_CHECKSUM
          value: "{{.CAChecksum}}"
        - name: CATTLE_CLUSTER
          value: "false"
        - name: CATTLE_K8S_MANAGED
          value: "true"
        - name: CATTLE_AGENT_CONNECT
          value: "true"
        volumeMounts:
        - name: cattle-credentials
          mountPath: c:/cattle-credentials
          readOnly: true
        - name: k8s-ssl
          mountPath: c:/etc/kubernetes
        - name: run
          mountPath: c:/run
        - name: docker-certs
          mountPath: c:/etc/docker/certs.d
        - name: docker-pipe
          mountPath: \\.\pipe\docker_engine
        - name: wins-pipe
          mountPath: \\.\pipe\rancher_wins
        - name: wins-config
          mountPath: c:/etc/rancher/wins
      volumes:
      - name: k8s-ssl
        hostPath:
          path: c:/etc/kubernetes
          type: DirectoryOrCreate
      - name: run
        hostPath:
          path: c:/run
          type: DirectoryOrCreate
      - name: cattle-credentials
        secret:
          secretName: cattle-credentials-{{.TokenKey}}
      - name: docker-certs
        hostPath:
          path: c:/ProgramData/docker/certs.d
          type: DirectoryOrCreate
      - name: docker-pipe
        hostPath:
          path: \\.\pipe\docker_engine
      - name: wins-pipe
        hostPath:
          path: \\.\pipe\rancher_wins
      - name: wins-config
        hostPath:
          path: c:/etc/rancher/wins
          type: DirectoryOrCreate
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 50%
{{- end }}

{{- if .AuthImage}}

---

apiVersion: apps/v1
kind: DaemonSet
metadata:
    name: kube-api-auth
    namespace: cattle-system
spec:
  selector:
    matchLabels:
      app: kube-api-auth
  template:
    metadata:
      labels:
        app: kube-api-auth
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: beta.kubernetes.io/os
                  operator: NotIn
                  values:
                    - windows
                - key: node-role.kubernetes.io/controlplane
                  operator: In
                  values:
                    - "true"
              - matchExpressions:
                - key: beta.kubernetes.io/os
                  operator: NotIn
                  values:
                    - windows
                - key: node-role.kubernetes.io/control-plane
                  operator: In
                  values:
                    - "true"
              - matchExpressions:
                - key: beta.kubernetes.io/os
                  operator: NotIn
                  values:
                    - windows
                - key: node-role.kubernetes.io/master
                  operator: In
                  values:
                    - "true"
      hostNetwork: true
      serviceAccountName: cattle
      tolerations:
      - operator: Exists
      containers:
      - name: kube-api-auth
        image: {{.AuthImage}}
        imagePullPolicy: IfNotPresent
        volumeMounts:
        - name: k8s-ssl
          mountPath: /etc/kubernetes
        securityContext:
          privileged: true
      {{- if .PrivateRegistryConfig}}
      imagePullSecrets:
      - name: cattle-private-registry
      {{- end }}
      volumes:
      - name: k8s-ssl
        hostPath:
          path: /etc/kubernetes
          type: DirectoryOrCreate
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 25%
{{- end }}
---
apiVersion: v1
kind: Service
metadata:
  name: cattle-cluster-agent
  namespace: cattle-system
spec:
  ports:
  - port: 80
    targetPort: 80
    protocol: TCP
    name: http
  - port: 443
    targetPort: 444
    protocol: TCP
    name: https-internal
  selector:
    app: cattle-cluster-agent
`

var (
	AuthDaemonSet = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
    name: kube-api-auth
    namespace: cattle-system
`
	NodeAgentDaemonSet = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
    name: cattle-node-agent
    namespace: cattle-system
`
)
