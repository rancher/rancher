package globaldns

var Route53DeploymentTemplate = `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: {{.deploymentName}}
spec:
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: {{.deploymentName}}
    spec:
      serviceAccountName: external-dns
      containers:
      - name: {{.deploymentName}}
        image: registry.opensource.zalan.do/teapot/external-dns:latest
        env:
        - name: AWS_SECRET_ACCESS_KEY
          value: {{.awsSecretKey}}
        - name: AWS_ACCESS_KEY_ID
          value: {{.awsAccessKey}}
        args:
        - --source=service
        - --source=ingress
        - --domain-filter={{.route53Domain}}
        - --provider=aws
        - --aws-zone-type=public # only look at public hosted zones (valid values are public, private or no value for both)
        - --registry=txt
        - --txt-owner-id=my-identifier
        - --log-level=debug
        - --publish-internal-services`

var ExternalDNSServiceAcct = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: external-dns
`

var ExternalDNSClusterRole = `
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: external-dns
rules:
- apiGroups: [""]
  resources: ["services"]
  verbs: ["get","watch","list"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get","watch","list"]
- apiGroups: ["extensions"] 
  resources: ["ingresses"] 
  verbs: ["get","watch","list"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["list"]
`

var ExternalDNSClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: external-dns-viewer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: external-dns
subjects:
- kind: ServiceAccount
  name: external-dns
  namespace: cattle-global-data
`
