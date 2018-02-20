package clusteregistrationtokens

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
  name: cattle
  namespace: cattle-system
subjects:
- kind: ServiceAccount
  name: cattle
  namespace: cattle-system
roleRef:
  kind: ClusterRole
  name: cluster-admin
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

apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: cattle-agent
  namespace: cattle-system
spec:
  template:
    metadata:
      labels:
        app: cluster-register
    spec:
      serviceAccountName: cattle
      containers:
        - name: cluster-register
          imagePullPolicy: Always
          env:
          - name: CATTLE_SERVER
            value: "{{.URLPlain}}"
          - name: CATTLE_CA_CHECKSUM
            value: "{{.CAChecksum}}"
          - name: CATTLE_CLUSTER
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
`
