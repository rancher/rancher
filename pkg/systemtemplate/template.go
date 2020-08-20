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

apiVersion: rbac.authorization.k8s.io/v1beta1
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
  name: cattle-cluster-agent
  namespace: cattle-system
spec:
  selector:
    matchLabels:
      app: cattle-cluster-agent
  template:
    metadata:
      labels:
        app: cattle-cluster-agent
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
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            preference:
              matchExpressions:
              - key: node-role.kubernetes.io/controlplane
                operator: In
                values:
                - "true"
          - weight: 1
            preference:
              matchExpressions:
              - key: node-role.kubernetes.io/etcd
                operator: In
                values:
                - "true"
      serviceAccountName: cattle
      tolerations:
      - operator: Exists
      containers:
        - name: cluster-register
          imagePullPolicy: IfNotPresent
          env:
          - name: CATTLE_FEATURES
            value: "{{.Features}}"
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
          image: {{.AgentImage}}
          volumeMounts:
          - name: cattle-credentials
            mountPath: /cattle-credentials
            readOnly: true
      {{- if .PrivateRegistryConfig}}
      imagePullSecrets:
      - name: cattle-private-registry
      {{- end }}
      volumes:
      - name: cattle-credentials
        secret:
          secretName: cattle-credentials-{{.TokenKey}}
          defaultMode: 320

---

{{ if .IsRKE }}

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
      maxUnavailable: 25%

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
      maxUnavailable: 25%
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
