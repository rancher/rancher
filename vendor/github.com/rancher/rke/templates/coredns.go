package templates

const CoreDNSTemplate = `
---
{{- if eq .RBACConfig "rbac"}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: coredns
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: system:coredns
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  - services
  - pods
  - namespaces
  verbs:
  - list
  - watch
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: system:coredns
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:coredns
subjects:
- kind: ServiceAccount
  name: coredns
  namespace: kube-system
{{- end }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
    .:53 {
        errors
        health
        kubernetes {{.ClusterDomain}} {{ if .ReverseCIDRs }}{{ .ReverseCIDRs }}{{ else }}{{ "in-addr.arpa ip6.arpa" }}{{ end }} {
          pods insecure
	  {{- if .UpstreamNameservers }}
          upstream {{range $i, $v := .UpstreamNameservers}}{{if $i}} {{end}}{{.}}{{end}}
	  {{- else }}
          upstream
          {{- end }}
          fallthrough in-addr.arpa ip6.arpa
        }
        prometheus :9153
	{{- if .UpstreamNameservers }}
        proxy . {{range $i, $v := .UpstreamNameservers}}{{if $i}} {{end}}{{.}}{{end}}
	{{- else }}
        proxy . "/etc/resolv.conf"
	{{- end }}
        cache 30
        loop
        reload
        loadbalance
    }
---
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: coredns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/name: "CoreDNS"
spec:
  replicas: 1
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
  selector:
    matchLabels:
      k8s-app: kube-dns
  template:
    metadata:
      labels:
        k8s-app: kube-dns
    spec:
{{- if eq .RBACConfig "rbac"}}
      serviceAccountName: coredns
{{- end }}
      nodeSelector:
      {{ range $k, $v := .NodeSelector }}
        {{ $k }}: {{ $v }}
      {{ end }}
      tolerations:
        - key: "CriticalAddonsOnly"
          operator: "Exists"
      containers:
      - name: coredns
        image: {{.CoreDNSImage}}
        imagePullPolicy: IfNotPresent
        resources:
          limits:
            memory: 170Mi
          requests:
            cpu: 100m
            memory: 70Mi
        args: [ "-conf", "/etc/coredns/Corefile" ]
        volumeMounts:
        - name: config-volume
          mountPath: /etc/coredns
          readOnly: true
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
        - containerPort: 9153
          name: metrics
          protocol: TCP
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            add:
            - NET_BIND_SERVICE
            drop:
            - all
          readOnlyRootFilesystem: true
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 60
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 5
      dnsPolicy: Default
      volumes:
        - name: config-volume
          configMap:
            name: coredns
            items:
            - key: Corefile
              path: Corefile
---
apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  annotations:
    prometheus.io/port: "9153"
    prometheus.io/scrape: "true"
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: "CoreDNS"
spec:
  selector:
    k8s-app: kube-dns
  clusterIP: {{.ClusterDNSServer}}
  ports:
  - name: dns
    port: 53
    protocol: UDP
  - name: dns-tcp
    port: 53
    protocol: TCP
---
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: coredns-autoscaler
  namespace: kube-system
  labels:
    k8s-app: coredns-autoscaler
spec:
  template:
    metadata:
      labels:
        k8s-app: coredns-autoscaler
    spec:
      serviceAccountName: coredns-autoscaler
      containers:
      - name: autoscaler
        image: {{.CoreDNSAutoScalerImage}}
        resources:
            requests:
                cpu: "20m"
                memory: "10Mi"
        command:
          - /cluster-proportional-autoscaler
          - --namespace=kube-system
          - --configmap=coredns-autoscaler
          - --target=Deployment/coredns
          # When cluster is using large nodes(with more cores), "coresPerReplica" should dominate.
          # If using small nodes, "nodesPerReplica" should dominate.
          - --default-params={"linear":{"coresPerReplica":128,"nodesPerReplica":4,"min":1}}
          - --logtostderr=true
          - --v=2
{{- if eq .RBACConfig "rbac"}}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: coredns-autoscaler
  namespace: kube-system
  labels:
    kubernetes.io/cluster-service: "true"
    addonmanager.kubernetes.io/mode: Reconcile
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: system:coredns-autoscaler
rules:
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["list"]
  - apiGroups: [""]
    resources: ["replicationcontrollers/scale"]
    verbs: ["get", "update"]
  - apiGroups: ["extensions"]
    resources: ["deployments/scale", "replicasets/scale"]
    verbs: ["get", "update"]
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "create"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: system:coredns-autoscaler
subjects:
  - kind: ServiceAccount
    name: coredns-autoscaler
    namespace: kube-system
roleRef:
  kind: ClusterRole
  name: system:coredns-autoscaler
  apiGroup: rbac.authorization.k8s.io
{{- end }}`
