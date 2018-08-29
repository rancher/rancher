package systemtemplate

var templateSource = `
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

---

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cattle-admin
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

apiVersion: extensions/v1beta1
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
      serviceAccountName: cattle
      containers:
        - name: cluster-register
          imagePullPolicy: IfNotPresent
          env:
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
      volumes:
      - name: cattle-credentials
        secret:
          secretName: cattle-credentials-{{.TokenKey}}

---

apiVersion: extensions/v1beta1
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
      - effect: NoExecute
        key: "node-role.kubernetes.io/etcd"
        value: "true"
      - effect: NoSchedule
        key: "node-role.kubernetes.io/controlplane"
        value: "true"
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
        - name: run
          mountPath: /run
        securityContext:
          privileged: true
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
  updateStrategy:
    type: RollingUpdate
`
