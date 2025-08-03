# Have a real cluster agent to join, and record the logs from various places

I have imported a cluster in rancher and applys the register yaml file, and record the cluster agent container logs, and also the rancher side logs to see if we can get more insight to the agent registre communication

## register yaml

[kube] root@16d681ee-c15f-4d3d-ad47-bc4526de1fd0:/persist/zks-net$ cat new2-cluster-reigster.yaml

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
  name: cattle-credentials-57f0905601
  namespace: cattle-system
type: Opaque
data:
  url: "aHR0cHM6Ly9ncmVlbi1jbHVzdGVyLnNoZW4ubnU="
  token: "ejVrd2dxdmpiOHZuaGN0czZzMnM1bHM5N25icXY0bWZ3OHBtN3R6cjZtcXE5Y2NnZmJtaDRz"
  namespace: ""

---

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
      affinity:
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - preference:
              matchExpressions:
              - key: node-role.kubernetes.io/controlplane
                operator: In
                values:
                - "true"
            weight: 100
          - preference:
              matchExpressions:
              - key: node-role.kubernetes.io/control-plane
                operator: In
                values:
                - "true"
            weight: 100
          - preference:
              matchExpressions:
              - key: node-role.kubernetes.io/master
                operator: In
                values:
                - "true"
            weight: 100
          - preference:
              matchExpressions:
              - key: cattle.io/cluster-agent
                operator: In
                values:
                - "true"
            weight: 1
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: beta.kubernetes.io/os
                operator: NotIn
                values:
                - windows
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - cattle-cluster-agent
              topologyKey: kubernetes.io/hostname
            weight: 100
      serviceAccountName: cattle
      tolerations:
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
      containers:
        - name: cluster-register
          imagePullPolicy: IfNotPresent
          env:
          - name: CATTLE_IS_RKE
            value: "false"
          - name: CATTLE_SERVER
            value: "https://green-cluster.shen.nu"
          - name: CATTLE_CA_CHECKSUM
            value: "22b557a27055b33606b6559f37703928d3e4ad79f110b407d04986e1843543d1"
          - name: CATTLE_CLUSTER
            value: "true"
          - name: CATTLE_K8S_MANAGED
            value: "true"
          - name: CATTLE_CLUSTER_REGISTRY
            value: ""
          - name: CATTLE_CREDENTIAL_NAME
            value: cattle-credentials-57f0905601
          - name: CATTLE_SERVER_VERSION
            value: v2.11.3
          - name: CATTLE_INSTALL_UUID
            value: d00d2140-d7e1-43bd-9edc-1cd0ca12210d
          - name: CATTLE_INGRESS_IP_DOMAIN
            value: sslip.io
          - name: STRICT_VERIFY
            value: "false"
          image: naimingshen/rancher-agent:latest-head
          volumeMounts:
          - name: cattle-credentials
            mountPath: /cattle-credentials
            readOnly: true
      volumes:
      - name: cattle-credentials
        secret:
          secretName: cattle-credentials-57f0905601
          defaultMode: 320
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1

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
[kube] root@16d681ee-c15f-4d3d-ad47-bc4526de1fd0:/persist/zks-net$ 

## cluster agent logs

[kube] root@16d681ee-c15f-4d3d-ad47-bc4526de1fd0:/persist/zks-net$ kubectl get pod -n cattle-system
NAME                                    READY   STATUS    RESTARTS   AGE
cattle-cluster-agent-7dfc494457-hzsn9   1/1     Running   0          37s
cattle-cluster-agent-7dfc494457-mlmzf   1/1     Running   0          32s
[kube] root@16d681ee-c15f-4d3d-ad47-bc4526de1fd0:/persist/zks-net$ kubectl logs -f -n cattle-system cattle-cluster-agent-7dfc494457-hzsn9
time="2025-08-03T19:18:17Z" level=info msg="starting cattle-credential-cleanup goroutine in the background"
time="2025-08-03T19:18:17Z" level=info msg="Listening on /tmp/log.sock"
time="2025-08-03T19:18:17Z" level=info msg="Rancher agent version dev is starting"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Cluster.Params() returning: map[cluster:map[address:10.43.0.1:443 caCert:LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3SGhjTk1qVXdOek14TURJd01qTXlXaGNOTXpVd056STVNREl3TWpNeQpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUK0tDazFjWkE0Z1pyOGJEV0JSWFNtSEZpZGZ0UFBYRm82VUorM0RCTWgKTUJocVBuSVFWcjRxR25SbXRuL3hNd3ViZzV3a2FlNXhDWWZNU0R2aGJxcVBvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXBpcWMwazEwSXVKVnZ1dzBhaHgyCi9uSHl3TFF3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUlnS0ZiSWJhKzd0Tm9CVWZvVis3dEdEN3FRTWdjVEgyeXkKU2pKYTFOODNkNW9DSVFEWEZ4VERELzhQNFF3WDNsRDg0ZU9wQlVTMENZZC8zNXdPb2JaT3VVNWUxUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K token:eyJhbGciOiJSUzI1NiIsImtpZCI6InBlSXFkTGYwVmVfZVVqUGNKZzFYSUExV25XbGtRSDd5bnNyZTcwTWQ2bncifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJjYXR0bGUtc3lzdGVtIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImNhdHRsZS10b2tlbi1uaDZnbiIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJjYXR0bGUiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiJiZGUwMWY1My03N2QyLTRjMDgtYjMwYy1jYTE5MjIxYTYzOWUiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6Y2F0dGxlLXN5c3RlbTpjYXR0bGUifQ.s3GomNMhMVHdp6gpwEM1NrHO6OjfzsYqWmXKTnpdxbWPCPEK17C3xXPbv_XTwvwrHMspFyhgmggCthkxBPD-fFyK34qTM9nAq6W6i_CbtReKpL79xIGTASqvozBIvLPC82smUhVylj7qc6JLtdsKjlBXfbUkC7CPAIbjxzOJQYjolPpEwEYh9e9VZgqfGaAF22dLSvzb-IxFadldI22nl1O0VoqyVg32NJdVk3jveHv158HVzorQUxGqPDcHiryBePtp_dlZVBvFPPcLZfLmv0ywSAa2Jy_93C9oYd8ZTa-3JORONEb5SWi4oJQRK5Bb1vvo1DfAXV1PNZwEc9UeHA]]"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Raw params: map[cluster:map[address:10.43.0.1:443 caCert:LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3SGhjTk1qVXdOek14TURJd01qTXlXaGNOTXpVd056STVNREl3TWpNeQpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUK0tDazFjWkE0Z1pyOGJEV0JSWFNtSEZpZGZ0UFBYRm82VUorM0RCTWgKTUJocVBuSVFWcjRxR25SbXRuL3hNd3ViZzV3a2FlNXhDWWZNU0R2aGJxcVBvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXBpcWMwazEwSXVKVnZ1dzBhaHgyCi9uSHl3TFF3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUlnS0ZiSWJhKzd0Tm9CVWZvVis3dEdEN3FRTWdjVEgyeXkKU2pKYTFOODNkNW9DSVFEWEZ4VERELzhQNFF3WDNsRDg0ZU9wQlVTMENZZC8zNXdPb2JaT3VVNWUxUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K token:eyJhbGciOiJSUzI1NiIsImtpZCI6InBlSXFkTGYwVmVfZVVqUGNKZzFYSUExV25XbGtRSDd5bnNyZTcwTWQ2bncifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJjYXR0bGUtc3lzdGVtIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImNhdHRsZS10b2tlbi1uaDZnbiIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJjYXR0bGUiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiJiZGUwMWY1My03N2QyLTRjMDgtYjMwYy1jYTE5MjIxYTYzOWUiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6Y2F0dGxlLXN5c3RlbTpjYXR0bGUifQ.s3GomNMhMVHdp6gpwEM1NrHO6OjfzsYqWmXKTnpdxbWPCPEK17C3xXPbv_XTwvwrHMspFyhgmggCthkxBPD-fFyK34qTM9nAq6W6i_CbtReKpL79xIGTASqvozBIvLPC82smUhVylj7qc6JLtdsKjlBXfbUkC7CPAIbjxzOJQYjolPpEwEYh9e9VZgqfGaAF22dLSvzb-IxFadldI22nl1O0VoqyVg32NJdVk3jveHv158HVzorQUxGqPDcHiryBePtp_dlZVBvFPPcLZfLmv0ywSAa2Jy_93C9oYd8ZTa-3JORONEb5SWi4oJQRK5Bb1vvo1DfAXV1PNZwEc9UeHA]]"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: JSON bytes: {\"cluster\":{\"address\":\"10.43.0.1:443\",\"caCert\":\"LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3SGhjTk1qVXdOek14TURJd01qTXlXaGNOTXpVd056STVNREl3TWpNeQpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUK0tDazFjWkE0Z1pyOGJEV0JSWFNtSEZpZGZ0UFBYRm82VUorM0RCTWgKTUJocVBuSVFWcjRxR25SbXRuL3hNd3ViZzV3a2FlNXhDWWZNU0R2aGJxcVBvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXBpcWMwazEwSXVKVnZ1dzBhaHgyCi9uSHl3TFF3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUlnS0ZiSWJhKzd0Tm9CVWZvVis3dEdEN3FRTWdjVEgyeXkKU2pKYTFOODNkNW9DSVFEWEZ4VERELzhQNFF3WDNsRDg0ZU9wQlVTMENZZC8zNXdPb2JaT3VVNWUxUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K\",\"token\":\"eyJhbGciOiJSUzI1NiIsImtpZCI6InBlSXFkTGYwVmVfZVVqUGNKZzFYSUExV25XbGtRSDd5bnNyZTcwTWQ2bncifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJjYXR0bGUtc3lzdGVtIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImNhdHRsZS10b2tlbi1uaDZnbiIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJjYXR0bGUiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiJiZGUwMWY1My03N2QyLTRjMDgtYjMwYy1jYTE5MjIxYTYzOWUiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6Y2F0dGxlLXN5c3RlbTpjYXR0bGUifQ.s3GomNMhMVHdp6gpwEM1NrHO6OjfzsYqWmXKTnpdxbWPCPEK17C3xXPbv_XTwvwrHMspFyhgmggCthkxBPD-fFyK34qTM9nAq6W6i_CbtReKpL79xIGTASqvozBIvLPC82smUhVylj7qc6JLtdsKjlBXfbUkC7CPAIbjxzOJQYjolPpEwEYh9e9VZgqfGaAF22dLSvzb-IxFadldI22nl1O0VoqyVg32NJdVk3jveHv158HVzorQUxGqPDcHiryBePtp_dlZVBvFPPcLZfLmv0ywSAa2Jy_93C9oYd8ZTa-3JORONEb5SWi4oJQRK5Bb1vvo1DfAXV1PNZwEc9UeHA\"}}"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: TokenAndURL() - CATTLE_TOKEN: "
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: TokenAndURL() - CATTLE_SERVER: https://green-cluster.shen.nu"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Cluster.TokenAndURL() - token: wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Cluster.TokenAndURL() - url: https://green-cluster.shen.nu"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Token: wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Server: https://green-cluster.shen.nu"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Headers - Token: wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Headers - Params (base64): eyJjbHVzdGVyIjp7ImFkZHJlc3MiOiIxMC40My4wLjE6NDQzIiwiY2FDZXJ0IjoiTFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSmtla05EUVZJeVowRjNTVUpCWjBsQ1FVUkJTMEpuWjNGb2EycFBVRkZSUkVGcVFXcE5VMFYzU0hkWlJGWlJVVVJFUW1oeVRUTk5kR015Vm5rS1pHMVdlVXhYVG1oUlJFVXpUbFJOTlUxcVkzcE9WRWwzU0doalRrMXFWWGRPZWsxNFRVUkpkMDFxVFhsWGFHTk9UWHBWZDA1NlNUVk5SRWwzVFdwTmVRcFhha0ZxVFZORmQwaDNXVVJXVVZGRVJFSm9jazB6VFhSak1sWjVaRzFXZVV4WFRtaFJSRVV6VGxSTk5VMXFZM3BPVkVsM1YxUkJWRUpuWTNGb2EycFBDbEJSU1VKQ1oyZHhhR3RxVDFCUlRVSkNkMDVEUVVGVUswdERhekZqV2tFMFoxcHlPR0pFVjBKU1dGTnRTRVpwWkdaMFVGQllSbTgyVlVvck0wUkNUV2dLVFVKb2NWQnVTVkZXY2pSeFIyNVNiWFJ1TDNoTmQzVmlaelYzYTJGbE5YaERXV1pOVTBSMmFHSnhjVkJ2TUVsM1VVUkJUMEpuVGxaSVVUaENRV1k0UlFwQ1FVMURRWEZSZDBSM1dVUldVakJVUVZGSUwwSkJWWGRCZDBWQ0wzcEJaRUpuVGxaSVVUUkZSbWRSVlhCcGNXTXdhekV3U1hWS1ZuWjFkekJoYUhneUNpOXVTSGwzVEZGM1EyZFpTVXR2V2tsNmFqQkZRWGRKUkZOQlFYZFNVVWxuUzBaaVNXSmhLemQwVG05Q1ZXWnZWaXMzZEVkRU4zRlJUV2RqVkVneWVYa0tVMnBLWVRGT09ETmtOVzlEU1ZGRVdFWjRWRVJFTHpoUU5GRjNXRE5zUkRnMFpVOXdRbFZUTUVOWlpDOHpOWGRQYjJKYVQzVlZOV1V4VVQwOUNpMHRMUzB0UlU1RUlFTkZVbFJKUmtsRFFWUkZMUzB0TFMwSyIsInRva2VuIjoiZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNkluQmxTWEZrVEdZd1ZtVmZaVlZxVUdOS1p6RllTVUV4VjI1WGJHdFJTRGQ1Ym5OeVpUY3dUV1EyYm5jaWZRLmV5SnBjM01pT2lKcmRXSmxjbTVsZEdWekwzTmxjblpwWTJWaFkyTnZkVzUwSWl3aWEzVmlaWEp1WlhSbGN5NXBieTl6WlhKMmFXTmxZV05qYjNWdWRDOXVZVzFsYzNCaFkyVWlPaUpqWVhSMGJHVXRjM2x6ZEdWdElpd2lhM1ZpWlhKdVpYUmxjeTVwYnk5elpYSjJhV05sWVdOamIzVnVkQzl6WldOeVpYUXVibUZ0WlNJNkltTmhkSFJzWlMxMGIydGxiaTF1YURabmJpSXNJbXQxWW1WeWJtVjBaWE11YVc4dmMyVnlkbWxqWldGalkyOTFiblF2YzJWeWRtbGpaUzFoWTJOdmRXNTBMbTVoYldVaU9pSmpZWFIwYkdVaUxDSnJkV0psY201bGRHVnpMbWx2TDNObGNuWnBZMlZoWTJOdmRXNTBMM05sY25acFkyVXRZV05qYjNWdWRDNTFhV1FpT2lKaVpHVXdNV1kxTXkwM04yUXlMVFJqTURndFlqTXdZeTFqWVRFNU1qSXhZVFl6T1dVaUxDSnpkV0lpT2lKemVYTjBaVzA2YzJWeWRtbGpaV0ZqWTI5MWJuUTZZMkYwZEd4bExYTjVjM1JsYlRwallYUjBiR1VpZlEuczNHb21OTWhNVkhkcDZncHdFTTFOckhPNk9qZnpzWXFXbVhLVG5wZHhiV1BDUEVLMTdDM3hYUGJ2X1hUd3Z3ckhNc3BGeWhnbWdnQ3Roa3hCUEQtZkZ5SzM0cVRNOW5BcTZXNmlfQ2J0UmVLcEw3OXhJR1RBU3F2b3pCSXZMUEM4MnNtVWhWeWxqN3FjNkpMdGRzS2psQlhmYlVrQzdDUEFJYmp4ek9KUVlqb2xQcEV3RVloOWU5VlpncWZHYUFGMjJkTFN2emItSXhGYWRsZEkyMm5sMU8wVm9xeVZnMzJOSmRWazNqdmVIdjE1OEhWem9yUVV4R3FQRGNIaXJ5QmVQdHBfZGxaVkJ2RlBQY0xaZkxtdjB5d1NBYTJKeV85M0M5b1lkOFpUYS0zSk9ST05FYjVTV2k0b0pRUks1QmIxdnZvMURmQVhWMVBOWndFYzlVZUhBIn19"
time="2025-08-03T19:18:17Z" level=warning msg="TLS certificate verification is DISABLED (forced by code)"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: WebSocket URL: wss://green-cluster.shen.nu/v3/connect/register"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: isConnect(): false"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Headers being sent to WebSocket: map[X-API-Tunnel-Params:[eyJjbHVzdGVyIjp7ImFkZHJlc3MiOiIxMC40My4wLjE6NDQzIiwiY2FDZXJ0IjoiTFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSmtla05EUVZJeVowRjNTVUpCWjBsQ1FVUkJTMEpuWjNGb2EycFBVRkZSUkVGcVFXcE5VMFYzU0hkWlJGWlJVVVJFUW1oeVRUTk5kR015Vm5rS1pHMVdlVXhYVG1oUlJFVXpUbFJOTlUxcVkzcE9WRWwzU0doalRrMXFWWGRPZWsxNFRVUkpkMDFxVFhsWGFHTk9UWHBWZDA1NlNUVk5SRWwzVFdwTmVRcFhha0ZxVFZORmQwaDNXVVJXVVZGRVJFSm9jazB6VFhSak1sWjVaRzFXZVV4WFRtaFJSRVV6VGxSTk5VMXFZM3BPVkVsM1YxUkJWRUpuWTNGb2EycFBDbEJSU1VKQ1oyZHhhR3RxVDFCUlRVSkNkMDVEUVVGVUswdERhekZqV2tFMFoxcHlPR0pFVjBKU1dGTnRTRVpwWkdaMFVGQllSbTgyVlVvck0wUkNUV2dLVFVKb2NWQnVTVkZXY2pSeFIyNVNiWFJ1TDNoTmQzVmlaelYzYTJGbE5YaERXV1pOVTBSMmFHSnhjVkJ2TUVsM1VVUkJUMEpuVGxaSVVUaENRV1k0UlFwQ1FVMURRWEZSZDBSM1dVUldVakJVUVZGSUwwSkJWWGRCZDBWQ0wzcEJaRUpuVGxaSVVUUkZSbWRSVlhCcGNXTXdhekV3U1hWS1ZuWjFkekJoYUhneUNpOXVTSGwzVEZGM1EyZFpTVXR2V2tsNmFqQkZRWGRKUkZOQlFYZFNVVWxuUzBaaVNXSmhLemQwVG05Q1ZXWnZWaXMzZEVkRU4zRlJUV2RqVkVneWVYa0tVMnBLWVRGT09ETmtOVzlEU1ZGRVdFWjRWRVJFTHpoUU5GRjNXRE5zUkRnMFpVOXdRbFZUTUVOWlpDOHpOWGRQYjJKYVQzVlZOV1V4VVQwOUNpMHRMUzB0UlU1RUlFTkZVbFJKUmtsRFFWUkZMUzB0TFMwSyIsInRva2VuIjoiZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNkluQmxTWEZrVEdZd1ZtVmZaVlZxVUdOS1p6RllTVUV4VjI1WGJHdFJTRGQ1Ym5OeVpUY3dUV1EyYm5jaWZRLmV5SnBjM01pT2lKcmRXSmxjbTVsZEdWekwzTmxjblpwWTJWaFkyTnZkVzUwSWl3aWEzVmlaWEp1WlhSbGN5NXBieTl6WlhKMmFXTmxZV05qYjNWdWRDOXVZVzFsYzNCaFkyVWlPaUpqWVhSMGJHVXRjM2x6ZEdWdElpd2lhM1ZpWlhKdVpYUmxjeTVwYnk5elpYSjJhV05sWVdOamIzVnVkQzl6WldOeVpYUXVibUZ0WlNJNkltTmhkSFJzWlMxMGIydGxiaTF1YURabmJpSXNJbXQxWW1WeWJtVjBaWE11YVc4dmMyVnlkbWxqWldGalkyOTFiblF2YzJWeWRtbGpaUzFoWTJOdmRXNTBMbTVoYldVaU9pSmpZWFIwYkdVaUxDSnJkV0psY201bGRHVnpMbWx2TDNObGNuWnBZMlZoWTJOdmRXNTBMM05sY25acFkyVXRZV05qYjNWdWRDNTFhV1FpT2lKaVpHVXdNV1kxTXkwM04yUXlMVFJqTURndFlqTXdZeTFqWVRFNU1qSXhZVFl6T1dVaUxDSnpkV0lpT2lKemVYTjBaVzA2YzJWeWRtbGpaV0ZqWTI5MWJuUTZZMkYwZEd4bExYTjVjM1JsYlRwallYUjBiR1VpZlEuczNHb21OTWhNVkhkcDZncHdFTTFOckhPNk9qZnpzWXFXbVhLVG5wZHhiV1BDUEVLMTdDM3hYUGJ2X1hUd3Z3ckhNc3BGeWhnbWdnQ3Roa3hCUEQtZkZ5SzM0cVRNOW5BcTZXNmlfQ2J0UmVLcEw3OXhJR1RBU3F2b3pCSXZMUEM4MnNtVWhWeWxqN3FjNkpMdGRzS2psQlhmYlVrQzdDUEFJYmp4ek9KUVlqb2xQcEV3RVloOWU5VlpncWZHYUFGMjJkTFN2emItSXhGYWRsZEkyMm5sMU8wVm9xeVZnMzJOSmRWazNqdmVIdjE1OEhWem9yUVV4R3FQRGNIaXJ5QmVQdHBfZGxaVkJ2RlBQY0xaZkxtdjB5d1NBYTJKeV85M0M5b1lkOFpUYS0zSk9ST05FYjVTV2k0b0pRUks1QmIxdnZvMURmQVhWMVBOWndFYzlVZUhBIn19] X-API-Tunnel-Token:[wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2]]"
time="2025-08-03T19:18:17Z" level=info msg="Connecting to wss://green-cluster.shen.nu/v3/connect/register with token starting with wnztgbttbld9njrv48st95qr665"
time="2025-08-03T19:18:17Z" level=info msg="Connecting to proxy" url="wss://green-cluster.shen.nu/v3/connect/register"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: onConnect called - WebSocket connection established"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Calling ConfigClient at https://green-cluster.shen.nu/v3/connect/config"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient called with url: https://green-cluster.shen.nu/v3/connect/config"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient headers: map[X-API-Tunnel-Params:[eyJjbHVzdGVyIjp7ImFkZHJlc3MiOiIxMC40My4wLjE6NDQzIiwiY2FDZXJ0IjoiTFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSmtla05EUVZJeVowRjNTVUpCWjBsQ1FVUkJTMEpuWjNGb2EycFBVRkZSUkVGcVFXcE5VMFYzU0hkWlJGWlJVVVJFUW1oeVRUTk5kR015Vm5rS1pHMVdlVXhYVG1oUlJFVXpUbFJOTlUxcVkzcE9WRWwzU0doalRrMXFWWGRPZWsxNFRVUkpkMDFxVFhsWGFHTk9UWHBWZDA1NlNUVk5SRWwzVFdwTmVRcFhha0ZxVFZORmQwaDNXVVJXVVZGRVJFSm9jazB6VFhSak1sWjVaRzFXZVV4WFRtaFJSRVV6VGxSTk5VMXFZM3BPVkVsM1YxUkJWRUpuWTNGb2EycFBDbEJSU1VKQ1oyZHhhR3RxVDFCUlRVSkNkMDVEUVVGVUswdERhekZqV2tFMFoxcHlPR0pFVjBKU1dGTnRTRVpwWkdaMFVGQllSbTgyVlVvck0wUkNUV2dLVFVKb2NWQnVTVkZXY2pSeFIyNVNiWFJ1TDNoTmQzVmlaelYzYTJGbE5YaERXV1pOVTBSMmFHSnhjVkJ2TUVsM1VVUkJUMEpuVGxaSVVUaENRV1k0UlFwQ1FVMURRWEZSZDBSM1dVUldVakJVUVZGSUwwSkJWWGRCZDBWQ0wzcEJaRUpuVGxaSVVUUkZSbWRSVlhCcGNXTXdhekV3U1hWS1ZuWjFkekJoYUhneUNpOXVTSGwzVEZGM1EyZFpTVXR2V2tsNmFqQkZRWGRKUkZOQlFYZFNVVWxuUzBaaVNXSmhLemQwVG05Q1ZXWnZWaXMzZEVkRU4zRlJUV2RqVkVneWVYa0tVMnBLWVRGT09ETmtOVzlEU1ZGRVdFWjRWRVJFTHpoUU5GRjNXRE5zUkRnMFpVOXdRbFZUTUVOWlpDOHpOWGRQYjJKYVQzVlZOV1V4VVQwOUNpMHRMUzB0UlU1RUlFTkZVbFJKUmtsRFFWUkZMUzB0TFMwSyIsInRva2VuIjoiZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNkluQmxTWEZrVEdZd1ZtVmZaVlZxVUdOS1p6RllTVUV4VjI1WGJHdFJTRGQ1Ym5OeVpUY3dUV1EyYm5jaWZRLmV5SnBjM01pT2lKcmRXSmxjbTVsZEdWekwzTmxjblpwWTJWaFkyTnZkVzUwSWl3aWEzVmlaWEp1WlhSbGN5NXBieTl6WlhKMmFXTmxZV05qYjNWdWRDOXVZVzFsYzNCaFkyVWlPaUpqWVhSMGJHVXRjM2x6ZEdWdElpd2lhM1ZpWlhKdVpYUmxjeTVwYnk5elpYSjJhV05sWVdOamIzVnVkQzl6WldOeVpYUXVibUZ0WlNJNkltTmhkSFJzWlMxMGIydGxiaTF1YURabmJpSXNJbXQxWW1WeWJtVjBaWE11YVc4dmMyVnlkbWxqWldGalkyOTFiblF2YzJWeWRtbGpaUzFoWTJOdmRXNTBMbTVoYldVaU9pSmpZWFIwYkdVaUxDSnJkV0psY201bGRHVnpMbWx2TDNObGNuWnBZMlZoWTJOdmRXNTBMM05sY25acFkyVXRZV05qYjNWdWRDNTFhV1FpT2lKaVpHVXdNV1kxTXkwM04yUXlMVFJqTURndFlqTXdZeTFqWVRFNU1qSXhZVFl6T1dVaUxDSnpkV0lpT2lKemVYTjBaVzA2YzJWeWRtbGpaV0ZqWTI5MWJuUTZZMkYwZEd4bExYTjVjM1JsYlRwallYUjBiR1VpZlEuczNHb21OTWhNVkhkcDZncHdFTTFOckhPNk9qZnpzWXFXbVhLVG5wZHhiV1BDUEVLMTdDM3hYUGJ2X1hUd3Z3ckhNc3BGeWhnbWdnQ3Roa3hCUEQtZkZ5SzM0cVRNOW5BcTZXNmlfQ2J0UmVLcEw3OXhJR1RBU3F2b3pCSXZMUEM4MnNtVWhWeWxqN3FjNkpMdGRzS2psQlhmYlVrQzdDUEFJYmp4ek9KUVlqb2xQcEV3RVloOWU5VlpncWZHYUFGMjJkTFN2emItSXhGYWRsZEkyMm5sMU8wVm9xeVZnMzJOSmRWazNqdmVIdjE1OEhWem9yUVV4R3FQRGNIaXJ5QmVQdHBfZGxaVkJ2RlBQY0xaZkxtdjB5d1NBYTJKeV85M0M5b1lkOFpUYS0zSk9ST05FYjVTV2k0b0pRUks1QmIxdnZvMURmQVhWMVBOWndFYzlVZUhBIn19] X-API-Tunnel-Token:[wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2]]"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient - calling getConfig"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient - got config successfully: &{ClusterName: Certs: Processes:map[] Files:[] NodeVersion:0 AgentCheckInterval:0}"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient - executing plan"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient - ExecutePlan completed successfully"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient - returning interval: 120"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient successful, interval: 120"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: isCluster() is true, calling rancher.Run()"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: rancher.Run() called"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: setupSteveAggregation called"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: setupSteveAggregation - got client config"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: setupSteveAggregation - created kubernetes client"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Cluster.TokenAndURL() - token: wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Cluster.TokenAndURL() - url: https://green-cluster.shen.nu"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Applying Steve aggregation secret"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Steve aggregation secret applied successfully"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: setupSteveAggregation completed successfully"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: setupSteveAggregation completed"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Got client config successfully"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Created core factory successfully"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Started core factory successfully"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: rancher.Run() completed successfully, started=true"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: rancher.Run() completed successfully"
time="2025-08-03T19:18:17Z" level=info msg="Starting /v1, Kind=Service controller"
time="2025-08-03T19:18:19Z" level=info msg="Running in single server mode, will not peer connections"
time="2025-08-03T19:18:19Z" level=info msg="Applying CRD features.management.cattle.io"
time="2025-08-03T19:18:22Z" level=info msg="Updating embedded CRD clusterrepos.catalog.cattle.io"
time="2025-08-03T19:18:22Z" level=info msg="Updating embedded CRD clusterproxyconfigs.management.cattle.io"
time="2025-08-03T19:18:22Z" level=info msg="Updating embedded CRD uiplugins.catalog.cattle.io"
time="2025-08-03T19:18:23Z" level=info msg="Applying CRD plans.upgrade.cattle.io"
time="2025-08-03T19:18:23Z" level=info msg="Applying CRD navlinks.ui.cattle.io"
time="2025-08-03T19:18:23Z" level=info msg="Applying CRD podsecurityadmissionconfigurationtemplates.management.cattle.io"
time="2025-08-03T19:18:23Z" level=info msg="Applying CRD clusters.management.cattle.io"
time="2025-08-03T19:18:24Z" level=info msg="Applying CRD apiservices.management.cattle.io"
time="2025-08-03T19:18:25Z" level=info msg="Applying CRD clusterregistrationtokens.management.cattle.io"
time="2025-08-03T19:18:25Z" level=info msg="Applying CRD settings.management.cattle.io"
time="2025-08-03T19:18:26Z" level=info msg="Applying CRD preferences.management.cattle.io"
time="2025-08-03T19:18:26Z" level=info msg="Applying CRD features.management.cattle.io"
time="2025-08-03T19:18:26Z" level=info msg="Applying CRD operations.catalog.cattle.io"
time="2025-08-03T19:18:27Z" level=info msg="Applying CRD apps.catalog.cattle.io"
time="2025-08-03T19:18:33Z" level=info msg="Starting API controllers"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Cluster controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=User controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Token controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=UserAttribute controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Group controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting /v1, Kind=Secret controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=GroupMember controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting /v1, Kind=ConfigMap controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting /v1, Kind=ServiceAccount controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting /v1, Kind=Secret controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting rbac.authorization.k8s.io/v1, Kind=ClusterRole controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting rbac.authorization.k8s.io/v1, Kind=RoleBinding controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Cluster controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting /v1, Kind=Namespace controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting rbac.authorization.k8s.io/v1, Kind=Role controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=APIService controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Token controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=User controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Setting controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Preference controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Feature controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=ClusterRegistrationToken controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting catalog.cattle.io/v1, Kind=ClusterRepo controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting apiextensions.k8s.io/v1, Kind=CustomResourceDefinition controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting apiregistration.k8s.io/v1, Kind=APIService controller"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=info msg="Starting steve aggregation client"
time="2025-08-03T19:18:33Z" level=info msg="Starting rbac.authorization.k8s.io/v1, Kind=ClusterRoleBinding controller"
I0803 19:18:33.783774       1 leaderelection.go:257] attempting to acquire leader lease kube-system/cattle-controllers...
time="2025-08-03T19:18:33Z" level=info msg="Listening on :443"
time="2025-08-03T19:18:33Z" level=info msg="Listening on :80"
time="2025-08-03T19:18:33Z" level=info msg="certificate CN=dynamic,O=dynamic signed by CN=dynamiclistener-ca@1754248688,O=dynamiclistener-org: notBefore=2025-08-03 19:18:08 +0000 UTC notAfter=2026-08-03 19:18:33 +0000 UTC"
time="2025-08-03T19:18:33Z" level=warning msg="dynamiclistener [::]:443: no cached certificate available for preload - deferring certificate load until storage initialization or first client request"
time="2025-08-03T19:18:33Z" level=info msg="Active TLS secret / (ver=) (count 4): map[listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=1601AC5A071BE96A157F79641FD413E493A8F413]"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
I0803 19:18:33.801717       1 leaderelection.go:271] successfully acquired lease kube-system/cattle-controllers
time="2025-08-03T19:18:33Z" level=info msg="Starting ServiceAccountSecretCleaner with 3 secrets"
time="2025-08-03T19:18:33Z" level=info msg="Active TLS secret cattle-system/serving-cert (ver=2533480) (count 5): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=33EB01D5009B09B183DC7AC79D7B40689B56BFB1]"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=info msg="Listening on :444"
time="2025-08-03T19:18:33Z" level=warning msg="dynamiclistener [::]:444: no cached certificate available for preload - deferring certificate load until storage initialization or first client request"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=info msg="Starting /v1, Kind=Secret controller"
time="2025-08-03T19:18:33Z" level=info msg="certificate CN=dynamic,O=dynamic signed by CN=dynamiclistener-ca@1754248688,O=dynamiclistener-org: notBefore=2025-08-03 19:18:08 +0000 UTC notAfter=2026-08-03 19:18:33 +0000 UTC"
time="2025-08-03T19:18:33Z" level=info msg="Updating TLS secret for cattle-system/serving-cert (count: 6): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=9F5590F9FB612EC47D43F9B1DE459AECE70C1293]"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=info msg="Active TLS secret cattle-system/serving-cert (ver=2533770) (count 6): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=9F5590F9FB612EC47D43F9B1DE459AECE70C1293]"
time="2025-08-03T19:18:34Z" level=info msg="Updating TLS secret for cattle-system/serving-cert (count: 6): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=9F5590F9FB612EC47D43F9B1DE459AECE70C1293]"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=info msg="Updating TLS secret for cattle-system/serving-cert (count: 7): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-10.42.1.36:10.42.1.36 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=961F3F8E797E45CCDAF3EA0BC337BFD5D32CC374]"
time="2025-08-03T19:18:35Z" level=info msg="Active TLS secret cattle-system/serving-cert (ver=2533782) (count 7): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-10.42.1.36:10.42.1.36 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=961F3F8E797E45CCDAF3EA0BC337BFD5D32CC374]"
time="2025-08-03T19:18:35Z" level=info msg="Registering namespaceHandler for adding labels "
W0803 19:18:35.695284       1 warnings.go:70] v1 ComponentStatus is deprecated in v1.19+
time="2025-08-03T19:18:35Z" level=info msg="Starting catalog.cattle.io/v1, Kind=App controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting /v1, Kind=Node controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting /v1, Kind=Endpoints controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting apps/v1, Kind=StatefulSet controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting apps/v1, Kind=DaemonSet controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting admissionregistration.k8s.io/v1, Kind=ValidatingWebhookConfiguration controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting catalog.cattle.io/v1, Kind=Operation controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting apps/v1, Kind=ReplicaSet controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting /v1, Kind=Service controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting /v1, Kind=Pod controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting batch/v1, Kind=Job controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting networking.k8s.io/v1, Kind=Ingress controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting apps/v1, Kind=Deployment controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting upgrade.cattle.io/v1, Kind=Plan controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting admissionregistration.k8s.io/v1, Kind=MutatingWebhookConfiguration controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting batch/v1, Kind=CronJob controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting /v1, Kind=ReplicationController controller"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:36Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:36Z" level=info msg="namespaceHandler: addProjectIDLabelToNamespace: adding label field.cattle.io/projectId=p-brmwv to namespace=cattle-fleet-system"
time="2025-08-03T19:18:36Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
W0803 19:18:36.099062       1 warnings.go:70] kubevirt.io/v1 VirtualMachineInstancePresets is now deprecated and will be removed in v2.
time="2025-08-03T19:18:36Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apps/v1, Kind=ControllerRevision"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for rbac.authorization.k8s.io/v1, Kind=Role"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for k8s.cni.cncf.io/v1, Kind=NetworkAttachmentDefinition"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackupVolume"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for snapshot.kubevirt.io/v1alpha1, Kind=VirtualMachineSnapshot"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=StorageProfile"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackupTarget"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Node"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for pool.kubevirt.io/v1alpha1, Kind=VirtualMachinePool"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=AuthConfig"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=IngressRoute"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for admissionregistration.k8s.io/v1, Kind=MutatingWebhookConfiguration"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Group"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachine"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for admissionregistration.k8s.io/v1, Kind=ValidatingWebhookConfiguration"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apiextensions.k8s.io/v1, Kind=CustomResourceDefinition"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apiregistration.k8s.io/v1, Kind=APIService"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=VolumeImportSource"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=ReplicationController"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachineInstancePreset"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackingImageManager"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=SystemBackup"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for instancetype.kubevirt.io/v1beta1, Kind=VirtualMachineClusterInstancetype"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Setting"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=CDI"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for autoscaling/v2, Kind=HorizontalPodAutoscaler"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Token"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for helm.cattle.io/v1, Kind=HelmChart"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for instancetype.kubevirt.io/v1beta1, Kind=VirtualMachineInstancetype"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=ClusterProxyConfig"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Snapshot"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apps/v1, Kind=StatefulSet"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for upgrade.cattle.io/v1, Kind=Plan"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=PodSecurityAdmissionConfigurationTemplate"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Namespace"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=CSIDriver"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackingImage"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=Middleware"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=KubeVirt"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=SupportBundle"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for node.k8s.io/v1, Kind=RuntimeClass"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Secret"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=ConfigMap"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=StorageClass"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=MiddlewareTCP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=DataVolume"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for flowcontrol.apiserver.k8s.io/v1beta3, Kind=PriorityLevelConfiguration"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=IngressRouteUDP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Backup"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Event"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=ShareManager"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for certificates.k8s.io/v1, Kind=CertificateSigningRequest"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for discovery.k8s.io/v1, Kind=EndpointSlice"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=DataImportCron"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for snapshot.kubevirt.io/v1alpha1, Kind=VirtualMachineSnapshotContent"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Setting"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=APIService"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for networking.k8s.io/v1, Kind=Ingress"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=ServersTransport"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=SystemRestore"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for k3s.cattle.io/v1, Kind=ETCDSnapshotFile"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=LimitRange"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Engine"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for export.kubevirt.io/v1alpha1, Kind=VirtualMachineExport"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apps/v1, Kind=Deployment"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for rbac.authorization.k8s.io/v1, Kind=ClusterRole"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for catalog.cattle.io/v1, Kind=UIPlugin"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Service"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for scheduling.k8s.io/v1, Kind=PriorityClass"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=TLSStore"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Node"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Feature"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=VolumeAttachment"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Endpoints"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Pod"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for instancetype.kubevirt.io/v1beta1, Kind=VirtualMachinePreference"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Cluster"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for ui.cattle.io/v1, Kind=NavLink"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=ServiceAccount"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachineInstanceMigration"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for networking.k8s.io/v1, Kind=IngressClass"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=IngressRouteTCP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=CDIConfig"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=ObjectTransfer"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=IngressRouteUDP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=VolumeAttachment"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachineInstanceReplicaSet"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for batch/v1, Kind=Job"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=ServersTransportTCP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackingImageDataSource"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for events.k8s.io/v1, Kind=Event"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=CSINode"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=UserAttribute"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=MiddlewareTCP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=Middleware"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=GroupMember"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for k3s.cattle.io/v1, Kind=Addon"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for flowcontrol.apiserver.k8s.io/v1beta3, Kind=FlowSchema"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for snapshot.kubevirt.io/v1alpha1, Kind=VirtualMachineRestore"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=CSIStorageCapacity"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=ResourceQuota"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Preference"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for migrations.kubevirt.io/v1alpha1, Kind=MigrationPolicy"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=RecurringJob"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=IngressRouteTCP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=TraefikService"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for catalog.cattle.io/v1, Kind=Operation"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=DataSource"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for policy/v1, Kind=PodDisruptionBudget"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=TLSOption"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackupBackingImage"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=User"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apps/v1, Kind=DaemonSet"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=TLSStore"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=ServersTransport"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for batch/v1, Kind=CronJob"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=InstanceManager"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Replica"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for networking.k8s.io/v1, Kind=NetworkPolicy"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachineInstance"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for rbac.authorization.k8s.io/v1, Kind=RoleBinding"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=PersistentVolume"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Orphan"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for instancetype.kubevirt.io/v1beta1, Kind=VirtualMachineClusterPreference"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for coordination.k8s.io/v1, Kind=Lease"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=TLSOption"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=EngineImage"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=IngressRoute"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for catalog.cattle.io/v1, Kind=ClusterRepo"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for rbac.authorization.k8s.io/v1, Kind=ClusterRoleBinding"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=VolumeUploadSource"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Volume"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for clone.kubevirt.io/v1alpha1, Kind=VirtualMachineClone"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for helm.cattle.io/v1, Kind=HelmChartConfig"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=PodTemplate"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=TraefikService"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apps/v1, Kind=ReplicaSet"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=PersistentVolumeClaim"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for catalog.cattle.io/v1, Kind=App"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=ClusterRegistrationToken"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=VolumeCloneSource"
W0803 19:18:36.312315       1 warnings.go:70] kubevirt.io/v1 VirtualMachineInstancePresets is now deprecated and will be removed in v2.
W0803 19:18:36.432370       1 warnings.go:70] kubevirt.io/v1 VirtualMachineInstancePresets is now deprecated and will be removed in v2.
time="2025-08-03T19:18:36Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:37Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:38Z" level=info msg="Steve auth startup complete"
W0803 19:18:38.461336       1 warnings.go:70] kubevirt.io/v1 VirtualMachineInstancePresets is now deprecated and will be removed in v2.
W0803 19:18:38.485624       1 warnings.go:70] v1 ComponentStatus is deprecated in v1.19+
time="2025-08-03T19:18:38Z" level=info msg="ServiceAccountSecretCleaner has no secrets remaining - terminating at 5.00085108s"
time="2025-08-03T19:18:41Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:18:46Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:18:48Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:51Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:18:56Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:19:01Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:19:06Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:19:08Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:19:11Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:19:16Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
^C
[kube] root@16d681ee-c15f-4d3d-ad47-bc4526de1fd0:/persist/zks-net$ 

## Anoter cluster agent pod logs, not sure why we need two pods

[kube] root@16d681ee-c15f-4d3d-ad47-bc4526de1fd0:/persist/zks-net$ kubectl get pod -n cattle-system
NAME                                    READY   STATUS    RESTARTS   AGE
cattle-cluster-agent-7dfc494457-hzsn9   1/1     Running   0          37s
cattle-cluster-agent-7dfc494457-mlmzf   1/1     Running   0          32s
[kube] root@16d681ee-c15f-4d3d-ad47-bc4526de1fd0:/persist/zks-net$ kubectl logs -f -n cattle-system cattle-cluster-agent-7dfc494457-hzsn9
time="2025-08-03T19:18:17Z" level=info msg="starting cattle-credential-cleanup goroutine in the background"
time="2025-08-03T19:18:17Z" level=info msg="Listening on /tmp/log.sock"
time="2025-08-03T19:18:17Z" level=info msg="Rancher agent version dev is starting"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Cluster.Params() returning: map[cluster:map[address:10.43.0.1:443 caCert:LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3SGhjTk1qVXdOek14TURJd01qTXlXaGNOTXpVd056STVNREl3TWpNeQpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUK0tDazFjWkE0Z1pyOGJEV0JSWFNtSEZpZGZ0UFBYRm82VUorM0RCTWgKTUJocVBuSVFWcjRxR25SbXRuL3hNd3ViZzV3a2FlNXhDWWZNU0R2aGJxcVBvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXBpcWMwazEwSXVKVnZ1dzBhaHgyCi9uSHl3TFF3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUlnS0ZiSWJhKzd0Tm9CVWZvVis3dEdEN3FRTWdjVEgyeXkKU2pKYTFOODNkNW9DSVFEWEZ4VERELzhQNFF3WDNsRDg0ZU9wQlVTMENZZC8zNXdPb2JaT3VVNWUxUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K token:eyJhbGciOiJSUzI1NiIsImtpZCI6InBlSXFkTGYwVmVfZVVqUGNKZzFYSUExV25XbGtRSDd5bnNyZTcwTWQ2bncifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJjYXR0bGUtc3lzdGVtIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImNhdHRsZS10b2tlbi1uaDZnbiIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJjYXR0bGUiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiJiZGUwMWY1My03N2QyLTRjMDgtYjMwYy1jYTE5MjIxYTYzOWUiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6Y2F0dGxlLXN5c3RlbTpjYXR0bGUifQ.s3GomNMhMVHdp6gpwEM1NrHO6OjfzsYqWmXKTnpdxbWPCPEK17C3xXPbv_XTwvwrHMspFyhgmggCthkxBPD-fFyK34qTM9nAq6W6i_CbtReKpL79xIGTASqvozBIvLPC82smUhVylj7qc6JLtdsKjlBXfbUkC7CPAIbjxzOJQYjolPpEwEYh9e9VZgqfGaAF22dLSvzb-IxFadldI22nl1O0VoqyVg32NJdVk3jveHv158HVzorQUxGqPDcHiryBePtp_dlZVBvFPPcLZfLmv0ywSAa2Jy_93C9oYd8ZTa-3JORONEb5SWi4oJQRK5Bb1vvo1DfAXV1PNZwEc9UeHA]]"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Raw params: map[cluster:map[address:10.43.0.1:443 caCert:LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3SGhjTk1qVXdOek14TURJd01qTXlXaGNOTXpVd056STVNREl3TWpNeQpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUK0tDazFjWkE0Z1pyOGJEV0JSWFNtSEZpZGZ0UFBYRm82VUorM0RCTWgKTUJocVBuSVFWcjRxR25SbXRuL3hNd3ViZzV3a2FlNXhDWWZNU0R2aGJxcVBvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXBpcWMwazEwSXVKVnZ1dzBhaHgyCi9uSHl3TFF3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUlnS0ZiSWJhKzd0Tm9CVWZvVis3dEdEN3FRTWdjVEgyeXkKU2pKYTFOODNkNW9DSVFEWEZ4VERELzhQNFF3WDNsRDg0ZU9wQlVTMENZZC8zNXdPb2JaT3VVNWUxUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K token:eyJhbGciOiJSUzI1NiIsImtpZCI6InBlSXFkTGYwVmVfZVVqUGNKZzFYSUExV25XbGtRSDd5bnNyZTcwTWQ2bncifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJjYXR0bGUtc3lzdGVtIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImNhdHRsZS10b2tlbi1uaDZnbiIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJjYXR0bGUiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiJiZGUwMWY1My03N2QyLTRjMDgtYjMwYy1jYTE5MjIxYTYzOWUiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6Y2F0dGxlLXN5c3RlbTpjYXR0bGUifQ.s3GomNMhMVHdp6gpwEM1NrHO6OjfzsYqWmXKTnpdxbWPCPEK17C3xXPbv_XTwvwrHMspFyhgmggCthkxBPD-fFyK34qTM9nAq6W6i_CbtReKpL79xIGTASqvozBIvLPC82smUhVylj7qc6JLtdsKjlBXfbUkC7CPAIbjxzOJQYjolPpEwEYh9e9VZgqfGaAF22dLSvzb-IxFadldI22nl1O0VoqyVg32NJdVk3jveHv158HVzorQUxGqPDcHiryBePtp_dlZVBvFPPcLZfLmv0ywSAa2Jy_93C9oYd8ZTa-3JORONEb5SWi4oJQRK5Bb1vvo1DfAXV1PNZwEc9UeHA]]"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: JSON bytes: {\"cluster\":{\"address\":\"10.43.0.1:443\",\"caCert\":\"LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3SGhjTk1qVXdOek14TURJd01qTXlXaGNOTXpVd056STVNREl3TWpNeQpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUK0tDazFjWkE0Z1pyOGJEV0JSWFNtSEZpZGZ0UFBYRm82VUorM0RCTWgKTUJocVBuSVFWcjRxR25SbXRuL3hNd3ViZzV3a2FlNXhDWWZNU0R2aGJxcVBvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXBpcWMwazEwSXVKVnZ1dzBhaHgyCi9uSHl3TFF3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUlnS0ZiSWJhKzd0Tm9CVWZvVis3dEdEN3FRTWdjVEgyeXkKU2pKYTFOODNkNW9DSVFEWEZ4VERELzhQNFF3WDNsRDg0ZU9wQlVTMENZZC8zNXdPb2JaT3VVNWUxUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K\",\"token\":\"eyJhbGciOiJSUzI1NiIsImtpZCI6InBlSXFkTGYwVmVfZVVqUGNKZzFYSUExV25XbGtRSDd5bnNyZTcwTWQ2bncifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJjYXR0bGUtc3lzdGVtIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImNhdHRsZS10b2tlbi1uaDZnbiIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJjYXR0bGUiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiJiZGUwMWY1My03N2QyLTRjMDgtYjMwYy1jYTE5MjIxYTYzOWUiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6Y2F0dGxlLXN5c3RlbTpjYXR0bGUifQ.s3GomNMhMVHdp6gpwEM1NrHO6OjfzsYqWmXKTnpdxbWPCPEK17C3xXPbv_XTwvwrHMspFyhgmggCthkxBPD-fFyK34qTM9nAq6W6i_CbtReKpL79xIGTASqvozBIvLPC82smUhVylj7qc6JLtdsKjlBXfbUkC7CPAIbjxzOJQYjolPpEwEYh9e9VZgqfGaAF22dLSvzb-IxFadldI22nl1O0VoqyVg32NJdVk3jveHv158HVzorQUxGqPDcHiryBePtp_dlZVBvFPPcLZfLmv0ywSAa2Jy_93C9oYd8ZTa-3JORONEb5SWi4oJQRK5Bb1vvo1DfAXV1PNZwEc9UeHA\"}}"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: TokenAndURL() - CATTLE_TOKEN: "
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: TokenAndURL() - CATTLE_SERVER: https://green-cluster.shen.nu"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Cluster.TokenAndURL() - token: wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Cluster.TokenAndURL() - url: https://green-cluster.shen.nu"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Token: wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Server: https://green-cluster.shen.nu"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Headers - Token: wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Headers - Params (base64): eyJjbHVzdGVyIjp7ImFkZHJlc3MiOiIxMC40My4wLjE6NDQzIiwiY2FDZXJ0IjoiTFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSmtla05EUVZJeVowRjNTVUpCWjBsQ1FVUkJTMEpuWjNGb2EycFBVRkZSUkVGcVFXcE5VMFYzU0hkWlJGWlJVVVJFUW1oeVRUTk5kR015Vm5rS1pHMVdlVXhYVG1oUlJFVXpUbFJOTlUxcVkzcE9WRWwzU0doalRrMXFWWGRPZWsxNFRVUkpkMDFxVFhsWGFHTk9UWHBWZDA1NlNUVk5SRWwzVFdwTmVRcFhha0ZxVFZORmQwaDNXVVJXVVZGRVJFSm9jazB6VFhSak1sWjVaRzFXZVV4WFRtaFJSRVV6VGxSTk5VMXFZM3BPVkVsM1YxUkJWRUpuWTNGb2EycFBDbEJSU1VKQ1oyZHhhR3RxVDFCUlRVSkNkMDVEUVVGVUswdERhekZqV2tFMFoxcHlPR0pFVjBKU1dGTnRTRVpwWkdaMFVGQllSbTgyVlVvck0wUkNUV2dLVFVKb2NWQnVTVkZXY2pSeFIyNVNiWFJ1TDNoTmQzVmlaelYzYTJGbE5YaERXV1pOVTBSMmFHSnhjVkJ2TUVsM1VVUkJUMEpuVGxaSVVUaENRV1k0UlFwQ1FVMURRWEZSZDBSM1dVUldVakJVUVZGSUwwSkJWWGRCZDBWQ0wzcEJaRUpuVGxaSVVUUkZSbWRSVlhCcGNXTXdhekV3U1hWS1ZuWjFkekJoYUhneUNpOXVTSGwzVEZGM1EyZFpTVXR2V2tsNmFqQkZRWGRKUkZOQlFYZFNVVWxuUzBaaVNXSmhLemQwVG05Q1ZXWnZWaXMzZEVkRU4zRlJUV2RqVkVneWVYa0tVMnBLWVRGT09ETmtOVzlEU1ZGRVdFWjRWRVJFTHpoUU5GRjNXRE5zUkRnMFpVOXdRbFZUTUVOWlpDOHpOWGRQYjJKYVQzVlZOV1V4VVQwOUNpMHRMUzB0UlU1RUlFTkZVbFJKUmtsRFFWUkZMUzB0TFMwSyIsInRva2VuIjoiZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNkluQmxTWEZrVEdZd1ZtVmZaVlZxVUdOS1p6RllTVUV4VjI1WGJHdFJTRGQ1Ym5OeVpUY3dUV1EyYm5jaWZRLmV5SnBjM01pT2lKcmRXSmxjbTVsZEdWekwzTmxjblpwWTJWaFkyTnZkVzUwSWl3aWEzVmlaWEp1WlhSbGN5NXBieTl6WlhKMmFXTmxZV05qYjNWdWRDOXVZVzFsYzNCaFkyVWlPaUpqWVhSMGJHVXRjM2x6ZEdWdElpd2lhM1ZpWlhKdVpYUmxjeTVwYnk5elpYSjJhV05sWVdOamIzVnVkQzl6WldOeVpYUXVibUZ0WlNJNkltTmhkSFJzWlMxMGIydGxiaTF1YURabmJpSXNJbXQxWW1WeWJtVjBaWE11YVc4dmMyVnlkbWxqWldGalkyOTFiblF2YzJWeWRtbGpaUzFoWTJOdmRXNTBMbTVoYldVaU9pSmpZWFIwYkdVaUxDSnJkV0psY201bGRHVnpMbWx2TDNObGNuWnBZMlZoWTJOdmRXNTBMM05sY25acFkyVXRZV05qYjNWdWRDNTFhV1FpT2lKaVpHVXdNV1kxTXkwM04yUXlMVFJqTURndFlqTXdZeTFqWVRFNU1qSXhZVFl6T1dVaUxDSnpkV0lpT2lKemVYTjBaVzA2YzJWeWRtbGpaV0ZqWTI5MWJuUTZZMkYwZEd4bExYTjVjM1JsYlRwallYUjBiR1VpZlEuczNHb21OTWhNVkhkcDZncHdFTTFOckhPNk9qZnpzWXFXbVhLVG5wZHhiV1BDUEVLMTdDM3hYUGJ2X1hUd3Z3ckhNc3BGeWhnbWdnQ3Roa3hCUEQtZkZ5SzM0cVRNOW5BcTZXNmlfQ2J0UmVLcEw3OXhJR1RBU3F2b3pCSXZMUEM4MnNtVWhWeWxqN3FjNkpMdGRzS2psQlhmYlVrQzdDUEFJYmp4ek9KUVlqb2xQcEV3RVloOWU5VlpncWZHYUFGMjJkTFN2emItSXhGYWRsZEkyMm5sMU8wVm9xeVZnMzJOSmRWazNqdmVIdjE1OEhWem9yUVV4R3FQRGNIaXJ5QmVQdHBfZGxaVkJ2RlBQY0xaZkxtdjB5d1NBYTJKeV85M0M5b1lkOFpUYS0zSk9ST05FYjVTV2k0b0pRUks1QmIxdnZvMURmQVhWMVBOWndFYzlVZUhBIn19"
time="2025-08-03T19:18:17Z" level=warning msg="TLS certificate verification is DISABLED (forced by code)"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: WebSocket URL: wss://green-cluster.shen.nu/v3/connect/register"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: isConnect(): false"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Headers being sent to WebSocket: map[X-API-Tunnel-Params:[eyJjbHVzdGVyIjp7ImFkZHJlc3MiOiIxMC40My4wLjE6NDQzIiwiY2FDZXJ0IjoiTFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSmtla05EUVZJeVowRjNTVUpCWjBsQ1FVUkJTMEpuWjNGb2EycFBVRkZSUkVGcVFXcE5VMFYzU0hkWlJGWlJVVVJFUW1oeVRUTk5kR015Vm5rS1pHMVdlVXhYVG1oUlJFVXpUbFJOTlUxcVkzcE9WRWwzU0doalRrMXFWWGRPZWsxNFRVUkpkMDFxVFhsWGFHTk9UWHBWZDA1NlNUVk5SRWwzVFdwTmVRcFhha0ZxVFZORmQwaDNXVVJXVVZGRVJFSm9jazB6VFhSak1sWjVaRzFXZVV4WFRtaFJSRVV6VGxSTk5VMXFZM3BPVkVsM1YxUkJWRUpuWTNGb2EycFBDbEJSU1VKQ1oyZHhhR3RxVDFCUlRVSkNkMDVEUVVGVUswdERhekZqV2tFMFoxcHlPR0pFVjBKU1dGTnRTRVpwWkdaMFVGQllSbTgyVlVvck0wUkNUV2dLVFVKb2NWQnVTVkZXY2pSeFIyNVNiWFJ1TDNoTmQzVmlaelYzYTJGbE5YaERXV1pOVTBSMmFHSnhjVkJ2TUVsM1VVUkJUMEpuVGxaSVVUaENRV1k0UlFwQ1FVMURRWEZSZDBSM1dVUldVakJVUVZGSUwwSkJWWGRCZDBWQ0wzcEJaRUpuVGxaSVVUUkZSbWRSVlhCcGNXTXdhekV3U1hWS1ZuWjFkekJoYUhneUNpOXVTSGwzVEZGM1EyZFpTVXR2V2tsNmFqQkZRWGRKUkZOQlFYZFNVVWxuUzBaaVNXSmhLemQwVG05Q1ZXWnZWaXMzZEVkRU4zRlJUV2RqVkVneWVYa0tVMnBLWVRGT09ETmtOVzlEU1ZGRVdFWjRWRVJFTHpoUU5GRjNXRE5zUkRnMFpVOXdRbFZUTUVOWlpDOHpOWGRQYjJKYVQzVlZOV1V4VVQwOUNpMHRMUzB0UlU1RUlFTkZVbFJKUmtsRFFWUkZMUzB0TFMwSyIsInRva2VuIjoiZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNkluQmxTWEZrVEdZd1ZtVmZaVlZxVUdOS1p6RllTVUV4VjI1WGJHdFJTRGQ1Ym5OeVpUY3dUV1EyYm5jaWZRLmV5SnBjM01pT2lKcmRXSmxjbTVsZEdWekwzTmxjblpwWTJWaFkyTnZkVzUwSWl3aWEzVmlaWEp1WlhSbGN5NXBieTl6WlhKMmFXTmxZV05qYjNWdWRDOXVZVzFsYzNCaFkyVWlPaUpqWVhSMGJHVXRjM2x6ZEdWdElpd2lhM1ZpWlhKdVpYUmxjeTVwYnk5elpYSjJhV05sWVdOamIzVnVkQzl6WldOeVpYUXVibUZ0WlNJNkltTmhkSFJzWlMxMGIydGxiaTF1YURabmJpSXNJbXQxWW1WeWJtVjBaWE11YVc4dmMyVnlkbWxqWldGalkyOTFiblF2YzJWeWRtbGpaUzFoWTJOdmRXNTBMbTVoYldVaU9pSmpZWFIwYkdVaUxDSnJkV0psY201bGRHVnpMbWx2TDNObGNuWnBZMlZoWTJOdmRXNTBMM05sY25acFkyVXRZV05qYjNWdWRDNTFhV1FpT2lKaVpHVXdNV1kxTXkwM04yUXlMVFJqTURndFlqTXdZeTFqWVRFNU1qSXhZVFl6T1dVaUxDSnpkV0lpT2lKemVYTjBaVzA2YzJWeWRtbGpaV0ZqWTI5MWJuUTZZMkYwZEd4bExYTjVjM1JsYlRwallYUjBiR1VpZlEuczNHb21OTWhNVkhkcDZncHdFTTFOckhPNk9qZnpzWXFXbVhLVG5wZHhiV1BDUEVLMTdDM3hYUGJ2X1hUd3Z3ckhNc3BGeWhnbWdnQ3Roa3hCUEQtZkZ5SzM0cVRNOW5BcTZXNmlfQ2J0UmVLcEw3OXhJR1RBU3F2b3pCSXZMUEM4MnNtVWhWeWxqN3FjNkpMdGRzS2psQlhmYlVrQzdDUEFJYmp4ek9KUVlqb2xQcEV3RVloOWU5VlpncWZHYUFGMjJkTFN2emItSXhGYWRsZEkyMm5sMU8wVm9xeVZnMzJOSmRWazNqdmVIdjE1OEhWem9yUVV4R3FQRGNIaXJ5QmVQdHBfZGxaVkJ2RlBQY0xaZkxtdjB5d1NBYTJKeV85M0M5b1lkOFpUYS0zSk9ST05FYjVTV2k0b0pRUks1QmIxdnZvMURmQVhWMVBOWndFYzlVZUhBIn19] X-API-Tunnel-Token:[wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2]]"
time="2025-08-03T19:18:17Z" level=info msg="Connecting to wss://green-cluster.shen.nu/v3/connect/register with token starting with wnztgbttbld9njrv48st95qr665"
time="2025-08-03T19:18:17Z" level=info msg="Connecting to proxy" url="wss://green-cluster.shen.nu/v3/connect/register"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: onConnect called - WebSocket connection established"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Calling ConfigClient at https://green-cluster.shen.nu/v3/connect/config"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient called with url: https://green-cluster.shen.nu/v3/connect/config"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient headers: map[X-API-Tunnel-Params:[eyJjbHVzdGVyIjp7ImFkZHJlc3MiOiIxMC40My4wLjE6NDQzIiwiY2FDZXJ0IjoiTFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSmtla05EUVZJeVowRjNTVUpCWjBsQ1FVUkJTMEpuWjNGb2EycFBVRkZSUkVGcVFXcE5VMFYzU0hkWlJGWlJVVVJFUW1oeVRUTk5kR015Vm5rS1pHMVdlVXhYVG1oUlJFVXpUbFJOTlUxcVkzcE9WRWwzU0doalRrMXFWWGRPZWsxNFRVUkpkMDFxVFhsWGFHTk9UWHBWZDA1NlNUVk5SRWwzVFdwTmVRcFhha0ZxVFZORmQwaDNXVVJXVVZGRVJFSm9jazB6VFhSak1sWjVaRzFXZVV4WFRtaFJSRVV6VGxSTk5VMXFZM3BPVkVsM1YxUkJWRUpuWTNGb2EycFBDbEJSU1VKQ1oyZHhhR3RxVDFCUlRVSkNkMDVEUVVGVUswdERhekZqV2tFMFoxcHlPR0pFVjBKU1dGTnRTRVpwWkdaMFVGQllSbTgyVlVvck0wUkNUV2dLVFVKb2NWQnVTVkZXY2pSeFIyNVNiWFJ1TDNoTmQzVmlaelYzYTJGbE5YaERXV1pOVTBSMmFHSnhjVkJ2TUVsM1VVUkJUMEpuVGxaSVVUaENRV1k0UlFwQ1FVMURRWEZSZDBSM1dVUldVakJVUVZGSUwwSkJWWGRCZDBWQ0wzcEJaRUpuVGxaSVVUUkZSbWRSVlhCcGNXTXdhekV3U1hWS1ZuWjFkekJoYUhneUNpOXVTSGwzVEZGM1EyZFpTVXR2V2tsNmFqQkZRWGRKUkZOQlFYZFNVVWxuUzBaaVNXSmhLemQwVG05Q1ZXWnZWaXMzZEVkRU4zRlJUV2RqVkVneWVYa0tVMnBLWVRGT09ETmtOVzlEU1ZGRVdFWjRWRVJFTHpoUU5GRjNXRE5zUkRnMFpVOXdRbFZUTUVOWlpDOHpOWGRQYjJKYVQzVlZOV1V4VVQwOUNpMHRMUzB0UlU1RUlFTkZVbFJKUmtsRFFWUkZMUzB0TFMwSyIsInRva2VuIjoiZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNkluQmxTWEZrVEdZd1ZtVmZaVlZxVUdOS1p6RllTVUV4VjI1WGJHdFJTRGQ1Ym5OeVpUY3dUV1EyYm5jaWZRLmV5SnBjM01pT2lKcmRXSmxjbTVsZEdWekwzTmxjblpwWTJWaFkyTnZkVzUwSWl3aWEzVmlaWEp1WlhSbGN5NXBieTl6WlhKMmFXTmxZV05qYjNWdWRDOXVZVzFsYzNCaFkyVWlPaUpqWVhSMGJHVXRjM2x6ZEdWdElpd2lhM1ZpWlhKdVpYUmxjeTVwYnk5elpYSjJhV05sWVdOamIzVnVkQzl6WldOeVpYUXVibUZ0WlNJNkltTmhkSFJzWlMxMGIydGxiaTF1YURabmJpSXNJbXQxWW1WeWJtVjBaWE11YVc4dmMyVnlkbWxqWldGalkyOTFiblF2YzJWeWRtbGpaUzFoWTJOdmRXNTBMbTVoYldVaU9pSmpZWFIwYkdVaUxDSnJkV0psY201bGRHVnpMbWx2TDNObGNuWnBZMlZoWTJOdmRXNTBMM05sY25acFkyVXRZV05qYjNWdWRDNTFhV1FpT2lKaVpHVXdNV1kxTXkwM04yUXlMVFJqTURndFlqTXdZeTFqWVRFNU1qSXhZVFl6T1dVaUxDSnpkV0lpT2lKemVYTjBaVzA2YzJWeWRtbGpaV0ZqWTI5MWJuUTZZMkYwZEd4bExYTjVjM1JsYlRwallYUjBiR1VpZlEuczNHb21OTWhNVkhkcDZncHdFTTFOckhPNk9qZnpzWXFXbVhLVG5wZHhiV1BDUEVLMTdDM3hYUGJ2X1hUd3Z3ckhNc3BGeWhnbWdnQ3Roa3hCUEQtZkZ5SzM0cVRNOW5BcTZXNmlfQ2J0UmVLcEw3OXhJR1RBU3F2b3pCSXZMUEM4MnNtVWhWeWxqN3FjNkpMdGRzS2psQlhmYlVrQzdDUEFJYmp4ek9KUVlqb2xQcEV3RVloOWU5VlpncWZHYUFGMjJkTFN2emItSXhGYWRsZEkyMm5sMU8wVm9xeVZnMzJOSmRWazNqdmVIdjE1OEhWem9yUVV4R3FQRGNIaXJ5QmVQdHBfZGxaVkJ2RlBQY0xaZkxtdjB5d1NBYTJKeV85M0M5b1lkOFpUYS0zSk9ST05FYjVTV2k0b0pRUks1QmIxdnZvMURmQVhWMVBOWndFYzlVZUhBIn19] X-API-Tunnel-Token:[wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2]]"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient - calling getConfig"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient - got config successfully: &{ClusterName: Certs: Processes:map[] Files:[] NodeVersion:0 AgentCheckInterval:0}"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient - executing plan"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient - ExecutePlan completed successfully"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient - returning interval: 120"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: ConfigClient successful, interval: 120"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: isCluster() is true, calling rancher.Run()"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: rancher.Run() called"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: setupSteveAggregation called"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: setupSteveAggregation - got client config"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: setupSteveAggregation - created kubernetes client"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Cluster.TokenAndURL() - token: wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Cluster.TokenAndURL() - url: https://green-cluster.shen.nu"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Applying Steve aggregation secret"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Steve aggregation secret applied successfully"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: setupSteveAggregation completed successfully"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: setupSteveAggregation completed"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Got client config successfully"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Created core factory successfully"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: Started core factory successfully"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: rancher.Run() completed successfully, started=true"
time="2025-08-03T19:18:17Z" level=info msg="DEBUG: rancher.Run() completed successfully"
time="2025-08-03T19:18:17Z" level=info msg="Starting /v1, Kind=Service controller"
time="2025-08-03T19:18:19Z" level=info msg="Running in single server mode, will not peer connections"
time="2025-08-03T19:18:19Z" level=info msg="Applying CRD features.management.cattle.io"
time="2025-08-03T19:18:22Z" level=info msg="Updating embedded CRD clusterrepos.catalog.cattle.io"
time="2025-08-03T19:18:22Z" level=info msg="Updating embedded CRD clusterproxyconfigs.management.cattle.io"
time="2025-08-03T19:18:22Z" level=info msg="Updating embedded CRD uiplugins.catalog.cattle.io"
time="2025-08-03T19:18:23Z" level=info msg="Applying CRD plans.upgrade.cattle.io"
time="2025-08-03T19:18:23Z" level=info msg="Applying CRD navlinks.ui.cattle.io"
time="2025-08-03T19:18:23Z" level=info msg="Applying CRD podsecurityadmissionconfigurationtemplates.management.cattle.io"
time="2025-08-03T19:18:23Z" level=info msg="Applying CRD clusters.management.cattle.io"
time="2025-08-03T19:18:24Z" level=info msg="Applying CRD apiservices.management.cattle.io"
time="2025-08-03T19:18:25Z" level=info msg="Applying CRD clusterregistrationtokens.management.cattle.io"
time="2025-08-03T19:18:25Z" level=info msg="Applying CRD settings.management.cattle.io"
time="2025-08-03T19:18:26Z" level=info msg="Applying CRD preferences.management.cattle.io"
time="2025-08-03T19:18:26Z" level=info msg="Applying CRD features.management.cattle.io"
time="2025-08-03T19:18:26Z" level=info msg="Applying CRD operations.catalog.cattle.io"
time="2025-08-03T19:18:27Z" level=info msg="Applying CRD apps.catalog.cattle.io"
time="2025-08-03T19:18:33Z" level=info msg="Starting API controllers"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Cluster controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=User controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Token controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=UserAttribute controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Group controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting /v1, Kind=Secret controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=GroupMember controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting /v1, Kind=ConfigMap controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting /v1, Kind=ServiceAccount controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting /v1, Kind=Secret controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting rbac.authorization.k8s.io/v1, Kind=ClusterRole controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting rbac.authorization.k8s.io/v1, Kind=RoleBinding controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Cluster controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting /v1, Kind=Namespace controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting rbac.authorization.k8s.io/v1, Kind=Role controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=APIService controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Token controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=User controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Setting controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Preference controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=Feature controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting management.cattle.io/v3, Kind=ClusterRegistrationToken controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting catalog.cattle.io/v1, Kind=ClusterRepo controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting apiextensions.k8s.io/v1, Kind=CustomResourceDefinition controller"
time="2025-08-03T19:18:33Z" level=info msg="Starting apiregistration.k8s.io/v1, Kind=APIService controller"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=info msg="Starting steve aggregation client"
time="2025-08-03T19:18:33Z" level=info msg="Starting rbac.authorization.k8s.io/v1, Kind=ClusterRoleBinding controller"
I0803 19:18:33.783774       1 leaderelection.go:257] attempting to acquire leader lease kube-system/cattle-controllers...
time="2025-08-03T19:18:33Z" level=info msg="Listening on :443"
time="2025-08-03T19:18:33Z" level=info msg="Listening on :80"
time="2025-08-03T19:18:33Z" level=info msg="certificate CN=dynamic,O=dynamic signed by CN=dynamiclistener-ca@1754248688,O=dynamiclistener-org: notBefore=2025-08-03 19:18:08 +0000 UTC notAfter=2026-08-03 19:18:33 +0000 UTC"
time="2025-08-03T19:18:33Z" level=warning msg="dynamiclistener [::]:443: no cached certificate available for preload - deferring certificate load until storage initialization or first client request"
time="2025-08-03T19:18:33Z" level=info msg="Active TLS secret / (ver=) (count 4): map[listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=1601AC5A071BE96A157F79641FD413E493A8F413]"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
I0803 19:18:33.801717       1 leaderelection.go:271] successfully acquired lease kube-system/cattle-controllers
time="2025-08-03T19:18:33Z" level=info msg="Starting ServiceAccountSecretCleaner with 3 secrets"
time="2025-08-03T19:18:33Z" level=info msg="Active TLS secret cattle-system/serving-cert (ver=2533480) (count 5): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=33EB01D5009B09B183DC7AC79D7B40689B56BFB1]"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=info msg="Listening on :444"
time="2025-08-03T19:18:33Z" level=warning msg="dynamiclistener [::]:444: no cached certificate available for preload - deferring certificate load until storage initialization or first client request"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:33Z" level=info msg="Starting /v1, Kind=Secret controller"
time="2025-08-03T19:18:33Z" level=info msg="certificate CN=dynamic,O=dynamic signed by CN=dynamiclistener-ca@1754248688,O=dynamiclistener-org: notBefore=2025-08-03 19:18:08 +0000 UTC notAfter=2026-08-03 19:18:33 +0000 UTC"
time="2025-08-03T19:18:33Z" level=info msg="Updating TLS secret for cattle-system/serving-cert (count: 6): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=9F5590F9FB612EC47D43F9B1DE459AECE70C1293]"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=info msg="Active TLS secret cattle-system/serving-cert (ver=2533770) (count 6): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=9F5590F9FB612EC47D43F9B1DE459AECE70C1293]"
time="2025-08-03T19:18:34Z" level=info msg="Updating TLS secret for cattle-system/serving-cert (count: 6): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=9F5590F9FB612EC47D43F9B1DE459AECE70C1293]"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=info msg="Updating TLS secret for cattle-system/serving-cert (count: 7): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-10.42.1.36:10.42.1.36 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=961F3F8E797E45CCDAF3EA0BC337BFD5D32CC374]"
time="2025-08-03T19:18:35Z" level=info msg="Active TLS secret cattle-system/serving-cert (ver=2533782) (count 7): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-10.42.1.36:10.42.1.36 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=961F3F8E797E45CCDAF3EA0BC337BFD5D32CC374]"
time="2025-08-03T19:18:35Z" level=info msg="Registering namespaceHandler for adding labels "
W0803 19:18:35.695284       1 warnings.go:70] v1 ComponentStatus is deprecated in v1.19+
time="2025-08-03T19:18:35Z" level=info msg="Starting catalog.cattle.io/v1, Kind=App controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting /v1, Kind=Node controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting /v1, Kind=Endpoints controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting apps/v1, Kind=StatefulSet controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting apps/v1, Kind=DaemonSet controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting admissionregistration.k8s.io/v1, Kind=ValidatingWebhookConfiguration controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting catalog.cattle.io/v1, Kind=Operation controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting apps/v1, Kind=ReplicaSet controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting /v1, Kind=Service controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting /v1, Kind=Pod controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting batch/v1, Kind=Job controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting networking.k8s.io/v1, Kind=Ingress controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting apps/v1, Kind=Deployment controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting upgrade.cattle.io/v1, Kind=Plan controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting admissionregistration.k8s.io/v1, Kind=MutatingWebhookConfiguration controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting batch/v1, Kind=CronJob controller"
time="2025-08-03T19:18:35Z" level=info msg="Starting /v1, Kind=ReplicationController controller"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:36Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:36Z" level=info msg="namespaceHandler: addProjectIDLabelToNamespace: adding label field.cattle.io/projectId=p-brmwv to namespace=cattle-fleet-system"
time="2025-08-03T19:18:36Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
W0803 19:18:36.099062       1 warnings.go:70] kubevirt.io/v1 VirtualMachineInstancePresets is now deprecated and will be removed in v2.
time="2025-08-03T19:18:36Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apps/v1, Kind=ControllerRevision"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for rbac.authorization.k8s.io/v1, Kind=Role"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for k8s.cni.cncf.io/v1, Kind=NetworkAttachmentDefinition"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackupVolume"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for snapshot.kubevirt.io/v1alpha1, Kind=VirtualMachineSnapshot"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=StorageProfile"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackupTarget"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Node"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for pool.kubevirt.io/v1alpha1, Kind=VirtualMachinePool"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=AuthConfig"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=IngressRoute"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for admissionregistration.k8s.io/v1, Kind=MutatingWebhookConfiguration"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Group"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachine"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for admissionregistration.k8s.io/v1, Kind=ValidatingWebhookConfiguration"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apiextensions.k8s.io/v1, Kind=CustomResourceDefinition"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apiregistration.k8s.io/v1, Kind=APIService"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=VolumeImportSource"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=ReplicationController"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachineInstancePreset"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackingImageManager"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=SystemBackup"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for instancetype.kubevirt.io/v1beta1, Kind=VirtualMachineClusterInstancetype"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Setting"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=CDI"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for autoscaling/v2, Kind=HorizontalPodAutoscaler"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Token"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for helm.cattle.io/v1, Kind=HelmChart"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for instancetype.kubevirt.io/v1beta1, Kind=VirtualMachineInstancetype"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=ClusterProxyConfig"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Snapshot"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apps/v1, Kind=StatefulSet"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for upgrade.cattle.io/v1, Kind=Plan"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=PodSecurityAdmissionConfigurationTemplate"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Namespace"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=CSIDriver"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackingImage"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=Middleware"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=KubeVirt"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=SupportBundle"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for node.k8s.io/v1, Kind=RuntimeClass"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Secret"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=ConfigMap"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=StorageClass"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=MiddlewareTCP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=DataVolume"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for flowcontrol.apiserver.k8s.io/v1beta3, Kind=PriorityLevelConfiguration"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=IngressRouteUDP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Backup"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Event"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=ShareManager"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for certificates.k8s.io/v1, Kind=CertificateSigningRequest"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for discovery.k8s.io/v1, Kind=EndpointSlice"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=DataImportCron"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for snapshot.kubevirt.io/v1alpha1, Kind=VirtualMachineSnapshotContent"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Setting"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=APIService"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for networking.k8s.io/v1, Kind=Ingress"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=ServersTransport"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=SystemRestore"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for k3s.cattle.io/v1, Kind=ETCDSnapshotFile"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=LimitRange"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Engine"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for export.kubevirt.io/v1alpha1, Kind=VirtualMachineExport"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apps/v1, Kind=Deployment"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for rbac.authorization.k8s.io/v1, Kind=ClusterRole"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for catalog.cattle.io/v1, Kind=UIPlugin"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Service"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for scheduling.k8s.io/v1, Kind=PriorityClass"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=TLSStore"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Node"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Feature"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=VolumeAttachment"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Endpoints"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=Pod"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for instancetype.kubevirt.io/v1beta1, Kind=VirtualMachinePreference"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Cluster"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for ui.cattle.io/v1, Kind=NavLink"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=ServiceAccount"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachineInstanceMigration"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for networking.k8s.io/v1, Kind=IngressClass"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=IngressRouteTCP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=CDIConfig"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=ObjectTransfer"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=IngressRouteUDP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=VolumeAttachment"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachineInstanceReplicaSet"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for batch/v1, Kind=Job"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=ServersTransportTCP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackingImageDataSource"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for events.k8s.io/v1, Kind=Event"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=CSINode"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=UserAttribute"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=MiddlewareTCP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=Middleware"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=GroupMember"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for k3s.cattle.io/v1, Kind=Addon"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for flowcontrol.apiserver.k8s.io/v1beta3, Kind=FlowSchema"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for snapshot.kubevirt.io/v1alpha1, Kind=VirtualMachineRestore"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=CSIStorageCapacity"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=ResourceQuota"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Preference"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for migrations.kubevirt.io/v1alpha1, Kind=MigrationPolicy"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=RecurringJob"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=IngressRouteTCP"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=TraefikService"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for catalog.cattle.io/v1, Kind=Operation"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=DataSource"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for policy/v1, Kind=PodDisruptionBudget"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=TLSOption"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackupBackingImage"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=User"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apps/v1, Kind=DaemonSet"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=TLSStore"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=ServersTransport"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for batch/v1, Kind=CronJob"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=InstanceManager"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Replica"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for networking.k8s.io/v1, Kind=NetworkPolicy"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachineInstance"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for rbac.authorization.k8s.io/v1, Kind=RoleBinding"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=PersistentVolume"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Orphan"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for instancetype.kubevirt.io/v1beta1, Kind=VirtualMachineClusterPreference"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for coordination.k8s.io/v1, Kind=Lease"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=TLSOption"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=EngineImage"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=IngressRoute"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for catalog.cattle.io/v1, Kind=ClusterRepo"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for rbac.authorization.k8s.io/v1, Kind=ClusterRoleBinding"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=VolumeUploadSource"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Volume"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for clone.kubevirt.io/v1alpha1, Kind=VirtualMachineClone"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for helm.cattle.io/v1, Kind=HelmChartConfig"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=PodTemplate"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=TraefikService"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for apps/v1, Kind=ReplicaSet"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for /v1, Kind=PersistentVolumeClaim"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for catalog.cattle.io/v1, Kind=App"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=ClusterRegistrationToken"
time="2025-08-03T19:18:36Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=VolumeCloneSource"
W0803 19:18:36.312315       1 warnings.go:70] kubevirt.io/v1 VirtualMachineInstancePresets is now deprecated and will be removed in v2.
W0803 19:18:36.432370       1 warnings.go:70] kubevirt.io/v1 VirtualMachineInstancePresets is now deprecated and will be removed in v2.
time="2025-08-03T19:18:36Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:37Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:38Z" level=info msg="Steve auth startup complete"
W0803 19:18:38.461336       1 warnings.go:70] kubevirt.io/v1 VirtualMachineInstancePresets is now deprecated and will be removed in v2.
W0803 19:18:38.485624       1 warnings.go:70] v1 ComponentStatus is deprecated in v1.19+
time="2025-08-03T19:18:38Z" level=info msg="ServiceAccountSecretCleaner has no secrets remaining - terminating at 5.00085108s"
time="2025-08-03T19:18:41Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:18:46Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:18:48Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:51Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:18:56Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:19:01Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:19:06Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:19:08Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:19:11Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
time="2025-08-03T19:19:16Z" level=error msg="Failed to find system chart rancher-webhook will try again in 5 seconds: configmaps \"\" not found"
^C
[kube] root@16d681ee-c15f-4d3d-ad47-bc4526de1fd0:/persist/zks-net$ kubectl get pod -n cattle-system
NAME                                    READY   STATUS    RESTARTS   AGE
cattle-cluster-agent-7dfc494457-hzsn9   1/1     Running   0          4m27s
cattle-cluster-agent-7dfc494457-mlmzf   1/1     Running   0          4m22s
[kube] root@16d681ee-c15f-4d3d-ad47-bc4526de1fd0:/persist/zks-net$ kubectl logs -f -n cattle-system cattle-cluster-agent-7dfc494457-mlmzf
time="2025-08-03T19:18:23Z" level=info msg="starting cattle-credential-cleanup goroutine in the background"
time="2025-08-03T19:18:23Z" level=info msg="Listening on /tmp/log.sock"
time="2025-08-03T19:18:23Z" level=info msg="Rancher agent version dev is starting"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Cluster.Params() returning: map[cluster:map[address:10.43.0.1:443 caCert:LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3SGhjTk1qVXdOek14TURJd01qTXlXaGNOTXpVd056STVNREl3TWpNeQpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUK0tDazFjWkE0Z1pyOGJEV0JSWFNtSEZpZGZ0UFBYRm82VUorM0RCTWgKTUJocVBuSVFWcjRxR25SbXRuL3hNd3ViZzV3a2FlNXhDWWZNU0R2aGJxcVBvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXBpcWMwazEwSXVKVnZ1dzBhaHgyCi9uSHl3TFF3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUlnS0ZiSWJhKzd0Tm9CVWZvVis3dEdEN3FRTWdjVEgyeXkKU2pKYTFOODNkNW9DSVFEWEZ4VERELzhQNFF3WDNsRDg0ZU9wQlVTMENZZC8zNXdPb2JaT3VVNWUxUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K token:eyJhbGciOiJSUzI1NiIsImtpZCI6InBlSXFkTGYwVmVfZVVqUGNKZzFYSUExV25XbGtRSDd5bnNyZTcwTWQ2bncifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJjYXR0bGUtc3lzdGVtIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImNhdHRsZS10b2tlbi1uaDZnbiIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJjYXR0bGUiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiJiZGUwMWY1My03N2QyLTRjMDgtYjMwYy1jYTE5MjIxYTYzOWUiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6Y2F0dGxlLXN5c3RlbTpjYXR0bGUifQ.s3GomNMhMVHdp6gpwEM1NrHO6OjfzsYqWmXKTnpdxbWPCPEK17C3xXPbv_XTwvwrHMspFyhgmggCthkxBPD-fFyK34qTM9nAq6W6i_CbtReKpL79xIGTASqvozBIvLPC82smUhVylj7qc6JLtdsKjlBXfbUkC7CPAIbjxzOJQYjolPpEwEYh9e9VZgqfGaAF22dLSvzb-IxFadldI22nl1O0VoqyVg32NJdVk3jveHv158HVzorQUxGqPDcHiryBePtp_dlZVBvFPPcLZfLmv0ywSAa2Jy_93C9oYd8ZTa-3JORONEb5SWi4oJQRK5Bb1vvo1DfAXV1PNZwEc9UeHA]]"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Raw params: map[cluster:map[address:10.43.0.1:443 caCert:LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3SGhjTk1qVXdOek14TURJd01qTXlXaGNOTXpVd056STVNREl3TWpNeQpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUK0tDazFjWkE0Z1pyOGJEV0JSWFNtSEZpZGZ0UFBYRm82VUorM0RCTWgKTUJocVBuSVFWcjRxR25SbXRuL3hNd3ViZzV3a2FlNXhDWWZNU0R2aGJxcVBvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXBpcWMwazEwSXVKVnZ1dzBhaHgyCi9uSHl3TFF3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUlnS0ZiSWJhKzd0Tm9CVWZvVis3dEdEN3FRTWdjVEgyeXkKU2pKYTFOODNkNW9DSVFEWEZ4VERELzhQNFF3WDNsRDg0ZU9wQlVTMENZZC8zNXdPb2JaT3VVNWUxUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K token:eyJhbGciOiJSUzI1NiIsImtpZCI6InBlSXFkTGYwVmVfZVVqUGNKZzFYSUExV25XbGtRSDd5bnNyZTcwTWQ2bncifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJjYXR0bGUtc3lzdGVtIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImNhdHRsZS10b2tlbi1uaDZnbiIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJjYXR0bGUiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiJiZGUwMWY1My03N2QyLTRjMDgtYjMwYy1jYTE5MjIxYTYzOWUiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6Y2F0dGxlLXN5c3RlbTpjYXR0bGUifQ.s3GomNMhMVHdp6gpwEM1NrHO6OjfzsYqWmXKTnpdxbWPCPEK17C3xXPbv_XTwvwrHMspFyhgmggCthkxBPD-fFyK34qTM9nAq6W6i_CbtReKpL79xIGTASqvozBIvLPC82smUhVylj7qc6JLtdsKjlBXfbUkC7CPAIbjxzOJQYjolPpEwEYh9e9VZgqfGaAF22dLSvzb-IxFadldI22nl1O0VoqyVg32NJdVk3jveHv158HVzorQUxGqPDcHiryBePtp_dlZVBvFPPcLZfLmv0ywSAa2Jy_93C9oYd8ZTa-3JORONEb5SWi4oJQRK5Bb1vvo1DfAXV1PNZwEc9UeHA]]"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: JSON bytes: {\"cluster\":{\"address\":\"10.43.0.1:443\",\"caCert\":\"LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdGMyVnkKZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3SGhjTk1qVXdOek14TURJd01qTXlXaGNOTXpVd056STVNREl3TWpNeQpXakFqTVNFd0h3WURWUVFEREJock0zTXRjMlZ5ZG1WeUxXTmhRREUzTlRNNU1qY3pOVEl3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFUK0tDazFjWkE0Z1pyOGJEV0JSWFNtSEZpZGZ0UFBYRm82VUorM0RCTWgKTUJocVBuSVFWcjRxR25SbXRuL3hNd3ViZzV3a2FlNXhDWWZNU0R2aGJxcVBvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVXBpcWMwazEwSXVKVnZ1dzBhaHgyCi9uSHl3TFF3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUlnS0ZiSWJhKzd0Tm9CVWZvVis3dEdEN3FRTWdjVEgyeXkKU2pKYTFOODNkNW9DSVFEWEZ4VERELzhQNFF3WDNsRDg0ZU9wQlVTMENZZC8zNXdPb2JaT3VVNWUxUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K\",\"token\":\"eyJhbGciOiJSUzI1NiIsImtpZCI6InBlSXFkTGYwVmVfZVVqUGNKZzFYSUExV25XbGtRSDd5bnNyZTcwTWQ2bncifQ.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJjYXR0bGUtc3lzdGVtIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImNhdHRsZS10b2tlbi1uaDZnbiIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJjYXR0bGUiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC51aWQiOiJiZGUwMWY1My03N2QyLTRjMDgtYjMwYy1jYTE5MjIxYTYzOWUiLCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6Y2F0dGxlLXN5c3RlbTpjYXR0bGUifQ.s3GomNMhMVHdp6gpwEM1NrHO6OjfzsYqWmXKTnpdxbWPCPEK17C3xXPbv_XTwvwrHMspFyhgmggCthkxBPD-fFyK34qTM9nAq6W6i_CbtReKpL79xIGTASqvozBIvLPC82smUhVylj7qc6JLtdsKjlBXfbUkC7CPAIbjxzOJQYjolPpEwEYh9e9VZgqfGaAF22dLSvzb-IxFadldI22nl1O0VoqyVg32NJdVk3jveHv158HVzorQUxGqPDcHiryBePtp_dlZVBvFPPcLZfLmv0ywSAa2Jy_93C9oYd8ZTa-3JORONEb5SWi4oJQRK5Bb1vvo1DfAXV1PNZwEc9UeHA\"}}"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: TokenAndURL() - CATTLE_TOKEN: "
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: TokenAndURL() - CATTLE_SERVER: https://green-cluster.shen.nu"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Cluster.TokenAndURL() - token: wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Cluster.TokenAndURL() - url: https://green-cluster.shen.nu"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Token: wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Server: https://green-cluster.shen.nu"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Headers - Token: wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Headers - Params (base64): eyJjbHVzdGVyIjp7ImFkZHJlc3MiOiIxMC40My4wLjE6NDQzIiwiY2FDZXJ0IjoiTFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSmtla05EUVZJeVowRjNTVUpCWjBsQ1FVUkJTMEpuWjNGb2EycFBVRkZSUkVGcVFXcE5VMFYzU0hkWlJGWlJVVVJFUW1oeVRUTk5kR015Vm5rS1pHMVdlVXhYVG1oUlJFVXpUbFJOTlUxcVkzcE9WRWwzU0doalRrMXFWWGRPZWsxNFRVUkpkMDFxVFhsWGFHTk9UWHBWZDA1NlNUVk5SRWwzVFdwTmVRcFhha0ZxVFZORmQwaDNXVVJXVVZGRVJFSm9jazB6VFhSak1sWjVaRzFXZVV4WFRtaFJSRVV6VGxSTk5VMXFZM3BPVkVsM1YxUkJWRUpuWTNGb2EycFBDbEJSU1VKQ1oyZHhhR3RxVDFCUlRVSkNkMDVEUVVGVUswdERhekZqV2tFMFoxcHlPR0pFVjBKU1dGTnRTRVpwWkdaMFVGQllSbTgyVlVvck0wUkNUV2dLVFVKb2NWQnVTVkZXY2pSeFIyNVNiWFJ1TDNoTmQzVmlaelYzYTJGbE5YaERXV1pOVTBSMmFHSnhjVkJ2TUVsM1VVUkJUMEpuVGxaSVVUaENRV1k0UlFwQ1FVMURRWEZSZDBSM1dVUldVakJVUVZGSUwwSkJWWGRCZDBWQ0wzcEJaRUpuVGxaSVVUUkZSbWRSVlhCcGNXTXdhekV3U1hWS1ZuWjFkekJoYUhneUNpOXVTSGwzVEZGM1EyZFpTVXR2V2tsNmFqQkZRWGRKUkZOQlFYZFNVVWxuUzBaaVNXSmhLemQwVG05Q1ZXWnZWaXMzZEVkRU4zRlJUV2RqVkVneWVYa0tVMnBLWVRGT09ETmtOVzlEU1ZGRVdFWjRWRVJFTHpoUU5GRjNXRE5zUkRnMFpVOXdRbFZUTUVOWlpDOHpOWGRQYjJKYVQzVlZOV1V4VVQwOUNpMHRMUzB0UlU1RUlFTkZVbFJKUmtsRFFWUkZMUzB0TFMwSyIsInRva2VuIjoiZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNkluQmxTWEZrVEdZd1ZtVmZaVlZxVUdOS1p6RllTVUV4VjI1WGJHdFJTRGQ1Ym5OeVpUY3dUV1EyYm5jaWZRLmV5SnBjM01pT2lKcmRXSmxjbTVsZEdWekwzTmxjblpwWTJWaFkyTnZkVzUwSWl3aWEzVmlaWEp1WlhSbGN5NXBieTl6WlhKMmFXTmxZV05qYjNWdWRDOXVZVzFsYzNCaFkyVWlPaUpqWVhSMGJHVXRjM2x6ZEdWdElpd2lhM1ZpWlhKdVpYUmxjeTVwYnk5elpYSjJhV05sWVdOamIzVnVkQzl6WldOeVpYUXVibUZ0WlNJNkltTmhkSFJzWlMxMGIydGxiaTF1YURabmJpSXNJbXQxWW1WeWJtVjBaWE11YVc4dmMyVnlkbWxqWldGalkyOTFiblF2YzJWeWRtbGpaUzFoWTJOdmRXNTBMbTVoYldVaU9pSmpZWFIwYkdVaUxDSnJkV0psY201bGRHVnpMbWx2TDNObGNuWnBZMlZoWTJOdmRXNTBMM05sY25acFkyVXRZV05qYjNWdWRDNTFhV1FpT2lKaVpHVXdNV1kxTXkwM04yUXlMVFJqTURndFlqTXdZeTFqWVRFNU1qSXhZVFl6T1dVaUxDSnpkV0lpT2lKemVYTjBaVzA2YzJWeWRtbGpaV0ZqWTI5MWJuUTZZMkYwZEd4bExYTjVjM1JsYlRwallYUjBiR1VpZlEuczNHb21OTWhNVkhkcDZncHdFTTFOckhPNk9qZnpzWXFXbVhLVG5wZHhiV1BDUEVLMTdDM3hYUGJ2X1hUd3Z3ckhNc3BGeWhnbWdnQ3Roa3hCUEQtZkZ5SzM0cVRNOW5BcTZXNmlfQ2J0UmVLcEw3OXhJR1RBU3F2b3pCSXZMUEM4MnNtVWhWeWxqN3FjNkpMdGRzS2psQlhmYlVrQzdDUEFJYmp4ek9KUVlqb2xQcEV3RVloOWU5VlpncWZHYUFGMjJkTFN2emItSXhGYWRsZEkyMm5sMU8wVm9xeVZnMzJOSmRWazNqdmVIdjE1OEhWem9yUVV4R3FQRGNIaXJ5QmVQdHBfZGxaVkJ2RlBQY0xaZkxtdjB5d1NBYTJKeV85M0M5b1lkOFpUYS0zSk9ST05FYjVTV2k0b0pRUks1QmIxdnZvMURmQVhWMVBOWndFYzlVZUhBIn19"
time="2025-08-03T19:18:23Z" level=warning msg="TLS certificate verification is DISABLED (forced by code)"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: WebSocket URL: wss://green-cluster.shen.nu/v3/connect/register"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: isConnect(): false"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Headers being sent to WebSocket: map[X-API-Tunnel-Params:[eyJjbHVzdGVyIjp7ImFkZHJlc3MiOiIxMC40My4wLjE6NDQzIiwiY2FDZXJ0IjoiTFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSmtla05EUVZJeVowRjNTVUpCWjBsQ1FVUkJTMEpuWjNGb2EycFBVRkZSUkVGcVFXcE5VMFYzU0hkWlJGWlJVVVJFUW1oeVRUTk5kR015Vm5rS1pHMVdlVXhYVG1oUlJFVXpUbFJOTlUxcVkzcE9WRWwzU0doalRrMXFWWGRPZWsxNFRVUkpkMDFxVFhsWGFHTk9UWHBWZDA1NlNUVk5SRWwzVFdwTmVRcFhha0ZxVFZORmQwaDNXVVJXVVZGRVJFSm9jazB6VFhSak1sWjVaRzFXZVV4WFRtaFJSRVV6VGxSTk5VMXFZM3BPVkVsM1YxUkJWRUpuWTNGb2EycFBDbEJSU1VKQ1oyZHhhR3RxVDFCUlRVSkNkMDVEUVVGVUswdERhekZqV2tFMFoxcHlPR0pFVjBKU1dGTnRTRVpwWkdaMFVGQllSbTgyVlVvck0wUkNUV2dLVFVKb2NWQnVTVkZXY2pSeFIyNVNiWFJ1TDNoTmQzVmlaelYzYTJGbE5YaERXV1pOVTBSMmFHSnhjVkJ2TUVsM1VVUkJUMEpuVGxaSVVUaENRV1k0UlFwQ1FVMURRWEZSZDBSM1dVUldVakJVUVZGSUwwSkJWWGRCZDBWQ0wzcEJaRUpuVGxaSVVUUkZSbWRSVlhCcGNXTXdhekV3U1hWS1ZuWjFkekJoYUhneUNpOXVTSGwzVEZGM1EyZFpTVXR2V2tsNmFqQkZRWGRKUkZOQlFYZFNVVWxuUzBaaVNXSmhLemQwVG05Q1ZXWnZWaXMzZEVkRU4zRlJUV2RqVkVneWVYa0tVMnBLWVRGT09ETmtOVzlEU1ZGRVdFWjRWRVJFTHpoUU5GRjNXRE5zUkRnMFpVOXdRbFZUTUVOWlpDOHpOWGRQYjJKYVQzVlZOV1V4VVQwOUNpMHRMUzB0UlU1RUlFTkZVbFJKUmtsRFFWUkZMUzB0TFMwSyIsInRva2VuIjoiZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNkluQmxTWEZrVEdZd1ZtVmZaVlZxVUdOS1p6RllTVUV4VjI1WGJHdFJTRGQ1Ym5OeVpUY3dUV1EyYm5jaWZRLmV5SnBjM01pT2lKcmRXSmxjbTVsZEdWekwzTmxjblpwWTJWaFkyTnZkVzUwSWl3aWEzVmlaWEp1WlhSbGN5NXBieTl6WlhKMmFXTmxZV05qYjNWdWRDOXVZVzFsYzNCaFkyVWlPaUpqWVhSMGJHVXRjM2x6ZEdWdElpd2lhM1ZpWlhKdVpYUmxjeTVwYnk5elpYSjJhV05sWVdOamIzVnVkQzl6WldOeVpYUXVibUZ0WlNJNkltTmhkSFJzWlMxMGIydGxiaTF1YURabmJpSXNJbXQxWW1WeWJtVjBaWE11YVc4dmMyVnlkbWxqWldGalkyOTFiblF2YzJWeWRtbGpaUzFoWTJOdmRXNTBMbTVoYldVaU9pSmpZWFIwYkdVaUxDSnJkV0psY201bGRHVnpMbWx2TDNObGNuWnBZMlZoWTJOdmRXNTBMM05sY25acFkyVXRZV05qYjNWdWRDNTFhV1FpT2lKaVpHVXdNV1kxTXkwM04yUXlMVFJqTURndFlqTXdZeTFqWVRFNU1qSXhZVFl6T1dVaUxDSnpkV0lpT2lKemVYTjBaVzA2YzJWeWRtbGpaV0ZqWTI5MWJuUTZZMkYwZEd4bExYTjVjM1JsYlRwallYUjBiR1VpZlEuczNHb21OTWhNVkhkcDZncHdFTTFOckhPNk9qZnpzWXFXbVhLVG5wZHhiV1BDUEVLMTdDM3hYUGJ2X1hUd3Z3ckhNc3BGeWhnbWdnQ3Roa3hCUEQtZkZ5SzM0cVRNOW5BcTZXNmlfQ2J0UmVLcEw3OXhJR1RBU3F2b3pCSXZMUEM4MnNtVWhWeWxqN3FjNkpMdGRzS2psQlhmYlVrQzdDUEFJYmp4ek9KUVlqb2xQcEV3RVloOWU5VlpncWZHYUFGMjJkTFN2emItSXhGYWRsZEkyMm5sMU8wVm9xeVZnMzJOSmRWazNqdmVIdjE1OEhWem9yUVV4R3FQRGNIaXJ5QmVQdHBfZGxaVkJ2RlBQY0xaZkxtdjB5d1NBYTJKeV85M0M5b1lkOFpUYS0zSk9ST05FYjVTV2k0b0pRUks1QmIxdnZvMURmQVhWMVBOWndFYzlVZUhBIn19] X-API-Tunnel-Token:[wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2]]"
time="2025-08-03T19:18:23Z" level=info msg="Connecting to wss://green-cluster.shen.nu/v3/connect/register with token starting with wnztgbttbld9njrv48st95qr665"
time="2025-08-03T19:18:23Z" level=info msg="Connecting to proxy" url="wss://green-cluster.shen.nu/v3/connect/register"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: onConnect called - WebSocket connection established"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Calling ConfigClient at https://green-cluster.shen.nu/v3/connect/config"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: ConfigClient called with url: https://green-cluster.shen.nu/v3/connect/config"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: ConfigClient headers: map[X-API-Tunnel-Params:[eyJjbHVzdGVyIjp7ImFkZHJlc3MiOiIxMC40My4wLjE6NDQzIiwiY2FDZXJ0IjoiTFMwdExTMUNSVWRKVGlCRFJWSlVTVVpKUTBGVVJTMHRMUzB0Q2sxSlNVSmtla05EUVZJeVowRjNTVUpCWjBsQ1FVUkJTMEpuWjNGb2EycFBVRkZSUkVGcVFXcE5VMFYzU0hkWlJGWlJVVVJFUW1oeVRUTk5kR015Vm5rS1pHMVdlVXhYVG1oUlJFVXpUbFJOTlUxcVkzcE9WRWwzU0doalRrMXFWWGRPZWsxNFRVUkpkMDFxVFhsWGFHTk9UWHBWZDA1NlNUVk5SRWwzVFdwTmVRcFhha0ZxVFZORmQwaDNXVVJXVVZGRVJFSm9jazB6VFhSak1sWjVaRzFXZVV4WFRtaFJSRVV6VGxSTk5VMXFZM3BPVkVsM1YxUkJWRUpuWTNGb2EycFBDbEJSU1VKQ1oyZHhhR3RxVDFCUlRVSkNkMDVEUVVGVUswdERhekZqV2tFMFoxcHlPR0pFVjBKU1dGTnRTRVpwWkdaMFVGQllSbTgyVlVvck0wUkNUV2dLVFVKb2NWQnVTVkZXY2pSeFIyNVNiWFJ1TDNoTmQzVmlaelYzYTJGbE5YaERXV1pOVTBSMmFHSnhjVkJ2TUVsM1VVUkJUMEpuVGxaSVVUaENRV1k0UlFwQ1FVMURRWEZSZDBSM1dVUldVakJVUVZGSUwwSkJWWGRCZDBWQ0wzcEJaRUpuVGxaSVVUUkZSbWRSVlhCcGNXTXdhekV3U1hWS1ZuWjFkekJoYUhneUNpOXVTSGwzVEZGM1EyZFpTVXR2V2tsNmFqQkZRWGRKUkZOQlFYZFNVVWxuUzBaaVNXSmhLemQwVG05Q1ZXWnZWaXMzZEVkRU4zRlJUV2RqVkVneWVYa0tVMnBLWVRGT09ETmtOVzlEU1ZGRVdFWjRWRVJFTHpoUU5GRjNXRE5zUkRnMFpVOXdRbFZUTUVOWlpDOHpOWGRQYjJKYVQzVlZOV1V4VVQwOUNpMHRMUzB0UlU1RUlFTkZVbFJKUmtsRFFWUkZMUzB0TFMwSyIsInRva2VuIjoiZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNkluQmxTWEZrVEdZd1ZtVmZaVlZxVUdOS1p6RllTVUV4VjI1WGJHdFJTRGQ1Ym5OeVpUY3dUV1EyYm5jaWZRLmV5SnBjM01pT2lKcmRXSmxjbTVsZEdWekwzTmxjblpwWTJWaFkyTnZkVzUwSWl3aWEzVmlaWEp1WlhSbGN5NXBieTl6WlhKMmFXTmxZV05qYjNWdWRDOXVZVzFsYzNCaFkyVWlPaUpqWVhSMGJHVXRjM2x6ZEdWdElpd2lhM1ZpWlhKdVpYUmxjeTVwYnk5elpYSjJhV05sWVdOamIzVnVkQzl6WldOeVpYUXVibUZ0WlNJNkltTmhkSFJzWlMxMGIydGxiaTF1YURabmJpSXNJbXQxWW1WeWJtVjBaWE11YVc4dmMyVnlkbWxqWldGalkyOTFiblF2YzJWeWRtbGpaUzFoWTJOdmRXNTBMbTVoYldVaU9pSmpZWFIwYkdVaUxDSnJkV0psY201bGRHVnpMbWx2TDNObGNuWnBZMlZoWTJOdmRXNTBMM05sY25acFkyVXRZV05qYjNWdWRDNTFhV1FpT2lKaVpHVXdNV1kxTXkwM04yUXlMVFJqTURndFlqTXdZeTFqWVRFNU1qSXhZVFl6T1dVaUxDSnpkV0lpT2lKemVYTjBaVzA2YzJWeWRtbGpaV0ZqWTI5MWJuUTZZMkYwZEd4bExYTjVjM1JsYlRwallYUjBiR1VpZlEuczNHb21OTWhNVkhkcDZncHdFTTFOckhPNk9qZnpzWXFXbVhLVG5wZHhiV1BDUEVLMTdDM3hYUGJ2X1hUd3Z3ckhNc3BGeWhnbWdnQ3Roa3hCUEQtZkZ5SzM0cVRNOW5BcTZXNmlfQ2J0UmVLcEw3OXhJR1RBU3F2b3pCSXZMUEM4MnNtVWhWeWxqN3FjNkpMdGRzS2psQlhmYlVrQzdDUEFJYmp4ek9KUVlqb2xQcEV3RVloOWU5VlpncWZHYUFGMjJkTFN2emItSXhGYWRsZEkyMm5sMU8wVm9xeVZnMzJOSmRWazNqdmVIdjE1OEhWem9yUVV4R3FQRGNIaXJ5QmVQdHBfZGxaVkJ2RlBQY0xaZkxtdjB5d1NBYTJKeV85M0M5b1lkOFpUYS0zSk9ST05FYjVTV2k0b0pRUks1QmIxdnZvMURmQVhWMVBOWndFYzlVZUhBIn19] X-API-Tunnel-Token:[wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2]]"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: ConfigClient - calling getConfig"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: ConfigClient - got config successfully: &{ClusterName: Certs: Processes:map[] Files:[] NodeVersion:0 AgentCheckInterval:0}"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: ConfigClient - executing plan"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: ConfigClient - ExecutePlan completed successfully"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: ConfigClient - returning interval: 120"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: ConfigClient successful, interval: 120"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: isCluster() is true, calling rancher.Run()"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: rancher.Run() called"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: setupSteveAggregation called"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: setupSteveAggregation - got client config"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: setupSteveAggregation - created kubernetes client"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Cluster.TokenAndURL() - token: wnztgbttbld9njrv48st95qr665lljsl8g4sddt9k5cqmhrg55gxr2"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Cluster.TokenAndURL() - url: https://green-cluster.shen.nu"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Applying Steve aggregation secret"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Steve aggregation secret applied successfully"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: setupSteveAggregation completed successfully"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: setupSteveAggregation completed"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Got client config successfully"
time="2025-08-03T19:18:23Z" level=info msg="DEBUG: Created core factory successfully"
time="2025-08-03T19:18:24Z" level=info msg="DEBUG: Started core factory successfully"
time="2025-08-03T19:18:24Z" level=info msg="DEBUG: rancher.Run() completed successfully, started=true"
time="2025-08-03T19:18:24Z" level=info msg="DEBUG: rancher.Run() completed successfully"
time="2025-08-03T19:18:24Z" level=info msg="Starting /v1, Kind=Service controller"
time="2025-08-03T19:18:24Z" level=info msg="Running in single server mode, will not peer connections"
time="2025-08-03T19:18:25Z" level=info msg="Applying CRD features.management.cattle.io"
time="2025-08-03T19:18:28Z" level=info msg="Updating embedded CRD clusterrepos.catalog.cattle.io"
time="2025-08-03T19:18:28Z" level=info msg="Updating embedded CRD clusterproxyconfigs.management.cattle.io"
time="2025-08-03T19:18:28Z" level=info msg="Updating embedded CRD uiplugins.catalog.cattle.io"
time="2025-08-03T19:18:29Z" level=info msg="Applying CRD plans.upgrade.cattle.io"
time="2025-08-03T19:18:29Z" level=info msg="Applying CRD navlinks.ui.cattle.io"
time="2025-08-03T19:18:29Z" level=info msg="Applying CRD podsecurityadmissionconfigurationtemplates.management.cattle.io"
time="2025-08-03T19:18:29Z" level=info msg="Applying CRD clusters.management.cattle.io"
time="2025-08-03T19:18:29Z" level=info msg="Applying CRD apiservices.management.cattle.io"
time="2025-08-03T19:18:30Z" level=info msg="Applying CRD clusterregistrationtokens.management.cattle.io"
time="2025-08-03T19:18:30Z" level=info msg="Applying CRD settings.management.cattle.io"
time="2025-08-03T19:18:31Z" level=info msg="Applying CRD preferences.management.cattle.io"
time="2025-08-03T19:18:31Z" level=info msg="Applying CRD features.management.cattle.io"
time="2025-08-03T19:18:31Z" level=info msg="Applying CRD operations.catalog.cattle.io"
time="2025-08-03T19:18:31Z" level=info msg="Applying CRD apps.catalog.cattle.io"
time="2025-08-03T19:18:33Z" level=info msg="Starting API controllers"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=GroupMember controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=Token controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=Group controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=Cluster controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=User controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=UserAttribute controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting /v1, Kind=Secret controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting apiregistration.k8s.io/v1, Kind=APIService controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=Setting controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=User controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting /v1, Kind=Namespace controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=Cluster controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=Token controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=Feature controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting rbac.authorization.k8s.io/v1, Kind=ClusterRoleBinding controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting apiextensions.k8s.io/v1, Kind=CustomResourceDefinition controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting rbac.authorization.k8s.io/v1, Kind=Role controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=ClusterRegistrationToken controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting rbac.authorization.k8s.io/v1, Kind=ClusterRole controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting catalog.cattle.io/v1, Kind=ClusterRepo controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=Preference controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting /v1, Kind=ConfigMap controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting /v1, Kind=ServiceAccount controller"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=info msg="Starting management.cattle.io/v3, Kind=APIService controller"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
I0803 19:18:34.963463       1 leaderelection.go:257] attempting to acquire leader lease kube-system/cattle-controllers...
time="2025-08-03T19:18:34Z" level=info msg="Starting /v1, Kind=Secret controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting rbac.authorization.k8s.io/v1, Kind=RoleBinding controller"
time="2025-08-03T19:18:34Z" level=info msg="Starting steve aggregation client"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:34Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=info msg="Listening on :443"
time="2025-08-03T19:18:35Z" level=info msg="Listening on :80"
time="2025-08-03T19:18:35Z" level=info msg="certificate CN=dynamic,O=dynamic signed by CN=dynamiclistener-ca@1754248688,O=dynamiclistener-org: notBefore=2025-08-03 19:18:08 +0000 UTC notAfter=2026-08-03 19:18:35 +0000 UTC"
time="2025-08-03T19:18:35Z" level=warning msg="dynamiclistener [::]:443: no cached certificate available for preload - deferring certificate load until storage initialization or first client request"
time="2025-08-03T19:18:35Z" level=info msg="Active TLS secret / (ver=) (count 4): map[listener.cattle.io/cn-10.42.1.36:10.42.1.36 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=5E2518D57AE08F4B2BEEF1DEDA977FA166BB2635]"
time="2025-08-03T19:18:35Z" level=info msg="Active TLS secret cattle-system/serving-cert (ver=2533770) (count 6): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=9F5590F9FB612EC47D43F9B1DE459AECE70C1293]"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=info msg="Listening on :444"
time="2025-08-03T19:18:35Z" level=warning msg="dynamiclistener [::]:444: no cached certificate available for preload - deferring certificate load until storage initialization or first client request"
time="2025-08-03T19:18:35Z" level=info msg="certificate CN=dynamic,O=dynamic signed by CN=dynamiclistener-ca@1754248688,O=dynamiclistener-org: notBefore=2025-08-03 19:18:08 +0000 UTC notAfter=2026-08-03 19:18:35 +0000 UTC"
time="2025-08-03T19:18:35Z" level=info msg="Updating TLS secret for cattle-system/serving-cert (count: 7): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-10.42.1.36:10.42.1.36 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=961F3F8E797E45CCDAF3EA0BC337BFD5D32CC374]"
time="2025-08-03T19:18:35Z" level=info msg="Starting /v1, Kind=Secret controller"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=info msg="Active TLS secret cattle-system/serving-cert (ver=2533782) (count 7): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-10.42.1.36:10.42.1.36 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=961F3F8E797E45CCDAF3EA0BC337BFD5D32CC374]"
time="2025-08-03T19:18:35Z" level=info msg="Updating TLS secret for cattle-system/serving-cert (count: 7): map[field.cattle.io/projectId:c-9stwq:p-brmwv listener.cattle.io/cn-10.42.0.128:10.42.0.128 listener.cattle.io/cn-10.42.0.129:10.42.0.129 listener.cattle.io/cn-10.42.1.36:10.42.1.36 listener.cattle.io/cn-127.0.0.1:127.0.0.1 listener.cattle.io/cn-localhost:localhost listener.cattle.io/cn-rancher.cattle-system:rancher.cattle-system listener.cattle.io/fingerprint:SHA1=961F3F8E797E45CCDAF3EA0BC337BFD5D32CC374]"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:35Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:36Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:36Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
W0803 19:18:36.662286       1 warnings.go:70] v1 ComponentStatus is deprecated in v1.19+
W0803 19:18:36.720499       1 warnings.go:70] kubevirt.io/v1 VirtualMachineInstancePresets is now deprecated and will be removed in v2.
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=MiddlewareTCP"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for autoscaling/v2, Kind=HorizontalPodAutoscaler"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=ServersTransport"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackupBackingImage"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Group"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for catalog.cattle.io/v1, Kind=UIPlugin"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachine"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for export.kubevirt.io/v1alpha1, Kind=VirtualMachineExport"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=Node"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for networking.k8s.io/v1, Kind=NetworkPolicy"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for discovery.k8s.io/v1, Kind=EndpointSlice"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for ui.cattle.io/v1, Kind=NavLink"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=CSINode"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for batch/v1, Kind=Job"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for snapshot.kubevirt.io/v1alpha1, Kind=VirtualMachineSnapshotContent"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Orphan"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=Middleware"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Setting"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=VolumeAttachment"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for rbac.authorization.k8s.io/v1, Kind=ClusterRole"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=CSIStorageCapacity"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachineInstance"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackupVolume"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=TraefikService"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=TLSOption"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for batch/v1, Kind=CronJob"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=AuthConfig"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for flowcontrol.apiserver.k8s.io/v1beta3, Kind=PriorityLevelConfiguration"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=Namespace"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=TLSStore"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for snapshot.kubevirt.io/v1alpha1, Kind=VirtualMachineRestore"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachineInstancePreset"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Token"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=SupportBundle"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=ServersTransportTCP"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=IngressRouteUDP"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=StorageClass"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Backup"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Replica"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=PodTemplate"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=ClusterProxyConfig"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Feature"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Engine"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for migrations.kubevirt.io/v1alpha1, Kind=MigrationPolicy"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for certificates.k8s.io/v1, Kind=CertificateSigningRequest"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for scheduling.k8s.io/v1, Kind=PriorityClass"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for instancetype.kubevirt.io/v1beta1, Kind=VirtualMachineInstancetype"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for clone.kubevirt.io/v1alpha1, Kind=VirtualMachineClone"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=IngressRouteTCP"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for helm.cattle.io/v1, Kind=HelmChartConfig"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for catalog.cattle.io/v1, Kind=App"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for events.k8s.io/v1, Kind=Event"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for rbac.authorization.k8s.io/v1, Kind=Role"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=APIService"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=ClusterRegistrationToken"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for apiextensions.k8s.io/v1, Kind=CustomResourceDefinition"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=Secret"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Snapshot"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=LimitRange"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=SystemRestore"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=PersistentVolumeClaim"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackingImageDataSource"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for apps/v1, Kind=DaemonSet"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackingImage"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for flowcontrol.apiserver.k8s.io/v1beta3, Kind=FlowSchema"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Volume"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for instancetype.kubevirt.io/v1beta1, Kind=VirtualMachinePreference"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackupTarget"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for catalog.cattle.io/v1, Kind=ClusterRepo"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=ServersTransport"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=PersistentVolume"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=Middleware"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=UserAttribute"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=Event"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Setting"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=ReplicationController"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for apps/v1, Kind=StatefulSet"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for admissionregistration.k8s.io/v1, Kind=MutatingWebhookConfiguration"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=IngressRoute"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Preference"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for apps/v1, Kind=Deployment"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for k3s.cattle.io/v1, Kind=ETCDSnapshotFile"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=GroupMember"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for admissionregistration.k8s.io/v1, Kind=ValidatingWebhookConfiguration"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for helm.cattle.io/v1, Kind=HelmChart"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=ResourceQuota"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=VolumeImportSource"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for k3s.cattle.io/v1, Kind=Addon"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for networking.k8s.io/v1, Kind=Ingress"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for node.k8s.io/v1, Kind=RuntimeClass"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=Service"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for instancetype.kubevirt.io/v1beta1, Kind=VirtualMachineClusterPreference"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=User"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=IngressRoute"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for coordination.k8s.io/v1, Kind=Lease"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for upgrade.cattle.io/v1, Kind=Plan"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=Node"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=VolumeUploadSource"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=StorageProfile"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=KubeVirt"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=IngressRouteTCP"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for rbac.authorization.k8s.io/v1, Kind=RoleBinding"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=IngressRouteUDP"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=CSIDriver"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for policy/v1, Kind=PodDisruptionBudget"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachineInstanceMigration"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for kubevirt.io/v1, Kind=VirtualMachineInstanceReplicaSet"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=RecurringJob"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for apps/v1, Kind=ControllerRevision"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for apps/v1, Kind=ReplicaSet"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=ConfigMap"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=MiddlewareTCP"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=Endpoints"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=DataSource"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=CDI"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for catalog.cattle.io/v1, Kind=Operation"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.io/v1alpha1, Kind=TLSOption"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=Pod"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=PodSecurityAdmissionConfigurationTemplate"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=ShareManager"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=InstanceManager"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=EngineImage"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=DataVolume"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for apiregistration.k8s.io/v1, Kind=APIService"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for management.cattle.io/v3, Kind=Cluster"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=CDIConfig"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for rbac.authorization.k8s.io/v1, Kind=ClusterRoleBinding"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for /v1, Kind=ServiceAccount"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for pool.kubevirt.io/v1alpha1, Kind=VirtualMachinePool"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=BackingImageManager"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for networking.k8s.io/v1, Kind=IngressClass"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=ObjectTransfer"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=TraefikService"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for storage.k8s.io/v1, Kind=VolumeAttachment"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for traefik.containo.us/v1alpha1, Kind=TLSStore"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for instancetype.kubevirt.io/v1beta1, Kind=VirtualMachineClusterInstancetype"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for snapshot.kubevirt.io/v1alpha1, Kind=VirtualMachineSnapshot"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for k8s.cni.cncf.io/v1, Kind=NetworkAttachmentDefinition"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=DataImportCron"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for cdi.kubevirt.io/v1beta1, Kind=VolumeCloneSource"
time="2025-08-03T19:18:37Z" level=info msg="Watching metadata for longhorn.io/v1beta2, Kind=SystemBackup"
W0803 19:18:37.345062       1 warnings.go:70] kubevirt.io/v1 VirtualMachineInstancePresets is now deprecated and will be removed in v2.
W0803 19:18:37.444736       1 warnings.go:70] kubevirt.io/v1 VirtualMachineInstancePresets is now deprecated and will be removed in v2.
time="2025-08-03T19:18:37Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:37Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:40Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:40Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:45Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:45Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:55Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:18:55Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:19:15Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:19:15Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:19:45Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:19:45Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:20:15Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:20:15Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:20:45Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:20:45Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:21:15Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:21:15Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:21:45Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:21:45Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:22:15Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:22:15Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:22:45Z" level=error msg="error syncing 'rancher-partner-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://git.rancher.io/partner-charts management-state/git-repo/rancher-partner-charts/8f17acdce9bffd6e05a58a3798840e408c4ea71783381ecd2e9af30baad65974 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"
time="2025-08-03T19:22:45Z" level=error msg="error syncing 'rancher-charts': handler helm-clusterrepo-ensure: ensure failure: git clone --depth=1 -n -- https://github.com/vadim-zededa/rancher-charts management-state/git-repo/rancher-charts/5c1a8b6e4a9b013eaee28aeea01a9dfc7b421290e801f782cd893c7aed66bd96 error: exec: \"git\": executable file not found in $PATH, detail: , requeuing"

## The Racher server side logs when apply this yaml to the cluster




2025/08/03 19:09:33 [INFO] [mgmt-cluster-rbac-delete] Creating namespace c-m-s9smgxkz
2025/08/03 19:09:34 [ERROR] [rkecluster] fleet-default/new-k3s-cluster1: error getting CAPI cluster no matching controller owner ref
2025/08/03 19:09:34 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler rke-cluster: no matching controller owner ref, requeuing
2025/08/03 19:09:34 [ERROR] [rkecluster] fleet-default/new-k3s-cluster1: error getting CAPI cluster no matching controller owner ref
2025/08/03 19:09:34 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler rke-cluster: no matching controller owner ref, requeuing
2025/08/03 19:09:34 [ERROR] [rkecluster] fleet-default/new-k3s-cluster1: error getting CAPI cluster no matching controller owner ref
2025/08/03 19:09:34 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler rke-cluster: no matching controller owner ref, requeuing
2025/08/03 19:09:34 [ERROR] [rkecluster] fleet-default/new-k3s-cluster1: error getting CAPI cluster no matching controller owner ref
2025/08/03 19:09:34 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler rke-cluster: no matching controller owner ref, requeuing
2025/08/03 19:09:34 [ERROR] [rkecluster] fleet-default/new-k3s-cluster1: error getting CAPI cluster no matching controller owner ref
2025/08/03 19:09:34 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler rke-cluster: no matching controller owner ref, requeuing
2025/08/03 19:09:34 [ERROR] [rkecluster] fleet-default/new-k3s-cluster1: error getting CAPI cluster no matching controller owner ref
2025/08/03 19:09:34 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler rke-cluster: no matching controller owner ref, requeuing
2025/08/03 19:09:34 [INFO] [mgmt-cluster-rbac-delete] Creating Default project for cluster c-m-s9smgxkz
2025/08/03 19:09:34 [ERROR] [rkecluster] fleet-default/new-k3s-cluster1: error getting CAPI cluster no matching controller owner ref
2025/08/03 19:09:34 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler rke-cluster: no matching controller owner ref, requeuing
2025/08/03 19:09:34 [INFO] [mgmt-project-rbac-create] Creating namespace c-m-s9smgxkz-p-n8k9h
2025/08/03 19:09:34 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-m-s9smgxkz, err=Operation cannot be fulfilled on namespaces "c-m-s9smgxkz": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:09:34 [INFO] [mgmt-cluster-rbac-delete] Creating System project for cluster c-m-s9smgxkz
2025/08/03 19:09:35 [INFO] [mgmt-project-rbac-create] Creating creator projectRoleTemplateBinding for user user-8kn8j for project p-n8k9h
2025/08/03 19:09:35 [ERROR] [planner] rkecluster fleet-default/new-k3s-cluster1: error during plan processing: no matching controller owner ref
2025/08/03 19:09:35 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-m-s9smgxkz, err=Operation cannot be fulfilled on namespaces "c-m-s9smgxkz": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:09:35 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler planner: no matching controller owner ref, requeuing
2025/08/03 19:09:35 [ERROR] [planner] rkecluster fleet-default/new-k3s-cluster1: error during plan processing: no matching controller owner ref
2025/08/03 19:09:35 [ERROR] [rkecluster] fleet-default/new-k3s-cluster1: error getting CAPI cluster no matching controller owner ref
2025/08/03 19:09:35 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler rke-cluster: no matching controller owner ref, requeuing
2025/08/03 19:09:35 [INFO] [mgmt-project-rbac-create] Creating namespace c-m-s9smgxkz-p-vcw2z
2025/08/03 19:09:35 [INFO] [mgmt-cluster-rbac-delete] Updating cluster c-m-s9smgxkz
2025/08/03 19:09:35 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler planner: no matching controller owner ref, requeuing
2025/08/03 19:09:35 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-m-s9smgxkz, err=Operation cannot be fulfilled on namespaces "c-m-s9smgxkz": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:09:35 [ERROR] [planner] rkecluster fleet-default/new-k3s-cluster1: error during plan processing: no matching controller owner ref
2025/08/03 19:09:35 [INFO] [mgmt-project-rbac-create] Setting InitialRolesPopulated condition on project p-n8k9h
2025/08/03 19:09:35 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler planner: no matching controller owner ref, requeuing
2025/08/03 19:09:35 [ERROR] [planner] rkecluster fleet-default/new-k3s-cluster1: error during plan processing: no matching controller owner ref
2025/08/03 19:09:35 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler planner: no matching controller owner ref, requeuing
2025/08/03 19:09:35 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-m-s9smgxkz, err=Operation cannot be fulfilled on namespaces "c-m-s9smgxkz": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:09:35 [INFO] [mgmt-auth-prtb-controller] Creating role/clusterRole p-n8k9h-projectowner
2025/08/03 19:09:35 [INFO] [mgmt-project-rbac-create] Updating project p-n8k9h
2025/08/03 19:09:35 [INFO] [mgmt-cluster-rbac-delete] Creating creator clusterRoleTemplateBinding for user user-8kn8j for cluster c-m-s9smgxkz
2025/08/03 19:09:35 [ERROR] [planner] rkecluster fleet-default/new-k3s-cluster1: error during plan processing: no matching controller owner ref
2025/08/03 19:09:35 [INFO] [mgmt-project-rbac-create] Creating creator projectRoleTemplateBinding for user user-8kn8j for project p-vcw2z
2025/08/03 19:09:35 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler planner: no matching controller owner ref, requeuing
2025/08/03 19:09:35 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for membership in project p-n8k9h for subject user-8kn8j
2025/08/03 19:09:35 [ERROR] [planner] rkecluster fleet-default/new-k3s-cluster1: error during plan processing: no matching controller owner ref
2025/08/03 19:09:35 [INFO] [mgmt-auth-prtb-controller] Creating role/clusterRole c-m-s9smgxkz-clustermember
2025/08/03 19:09:35 [INFO] [mgmt-project-rbac-create] Setting InitialRolesPopulated condition on project p-vcw2z
2025/08/03 19:09:35 [INFO] [mgmt-project-rbac-create] Updating project p-n8k9h
2025/08/03 19:09:35 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler planner: no matching controller owner ref, requeuing
2025/08/03 19:09:35 [INFO] [mgmt-auth-prtb-controller] Creating clusterRoleBinding for membership in cluster c-m-s9smgxkz for subject user-8kn8j
2025/08/03 19:09:35 [INFO] [mgmt-auth-crtb-controller] Creating role/clusterRole c-m-s9smgxkz-clusterowner
2025/08/03 19:09:35 [INFO] [mgmt-project-rbac-create] Updating project p-vcw2z
2025/08/03 19:09:35 [INFO] [mgmt-auth-prtb-controller] Creating role/clusterRole p-vcw2z-projectowner
2025/08/03 19:09:35 [INFO] [mgmt-auth-prtb-controller] Creating role project-owner in namespace c-m-s9smgxkz-p-n8k9h
2025/08/03 19:09:35 [INFO] [mgmt-auth-crtb-controller] Creating clusterRoleBinding for membership in cluster c-m-s9smgxkz for subject user-8kn8j
2025/08/03 19:09:35 [ERROR] [rkecluster] fleet-default/new-k3s-cluster1: error getting CAPI cluster no matching controller owner ref
2025/08/03 19:09:35 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler rke-cluster: no matching controller owner ref, requeuing
2025/08/03 19:09:35 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for membership in project p-vcw2z for subject user-8kn8j
2025/08/03 19:09:35 [ERROR] [planner] rkecluster fleet-default/new-k3s-cluster1: error during plan processing: no matching controller owner ref
2025/08/03 19:09:35 [INFO] [mgmt-auth-prtb-controller] Creating role admin in namespace c-m-s9smgxkz-p-n8k9h
2025/08/03 19:09:35 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-m-s9smgxkz-p-n8k9h, err=Operation cannot be fulfilled on namespaces "c-m-s9smgxkz-p-n8k9h": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:09:35 [INFO] [mgmt-auth-crtb-controller] Creating role cluster-owner in namespace c-m-s9smgxkz
2025/08/03 19:09:35 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler planner: no matching controller owner ref, requeuing
2025/08/03 19:09:35 [INFO] [mgmt-auth-prtb-controller] Updating clusterRoleBinding crb-u2niihd3fu for cluster membership in cluster c-m-s9smgxkz for subject user-8kn8j
2025/08/03 19:09:35 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role project-owner in namespace c-m-s9smgxkz-p-n8k9h
2025/08/03 19:09:35 [INFO] [mgmt-project-rbac-create] Updating project p-vcw2z
2025/08/03 19:09:35 [INFO] [mgmt-cluster-rbac-delete] Setting InitialRolesPopulated condition on cluster c-m-s9smgxkz
2025/08/03 19:09:35 [INFO] [mgmt-cluster-rbac-delete] Updating cluster c-m-s9smgxkz
2025/08/03 19:09:35 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject user-8kn8j with role cluster-owner in namespace c-m-s9smgxkz
2025/08/03 19:09:35 [INFO] [mgmt-auth-prtb-controller] Creating role project-owner in namespace c-m-s9smgxkz-p-vcw2z
2025/08/03 19:09:35 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role admin in namespace c-m-s9smgxkz-p-n8k9h
2025/08/03 19:09:36 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-m-s9smgxkz-p-n8k9h, err=Operation cannot be fulfilled on namespaces "c-m-s9smgxkz-p-n8k9h": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:09:36 [INFO] [mgmt-auth-prtb-controller] Creating role admin in namespace c-m-s9smgxkz-p-vcw2z
2025/08/03 19:09:36 [INFO] [mgmt-auth-crtb-controller] Creating role cluster-owner in namespace c-m-s9smgxkz-p-n8k9h
2025/08/03 19:09:36 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-m-s9smgxkz-p-vcw2z, err=Operation cannot be fulfilled on namespaces "c-m-s9smgxkz-p-vcw2z": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:09:36 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role project-owner in namespace c-m-s9smgxkz-p-vcw2z
2025/08/03 19:09:36 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject user-8kn8j with role cluster-owner in namespace c-m-s9smgxkz-p-n8k9h
2025/08/03 19:09:36 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for infrastructure ready
2025/08/03 19:09:36 [INFO] [mgmt-auth-crtb-controller] Creating role cluster-owner in namespace c-m-s9smgxkz-p-vcw2z
2025/08/03 19:09:36 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role admin in namespace c-m-s9smgxkz-p-vcw2z
2025/08/03 19:09:36 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-m-s9smgxkz-p-vcw2z, err=Operation cannot be fulfilled on namespaces "c-m-s9smgxkz-p-vcw2z": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:09:36 [ERROR] unable to update cluster c-m-s9smgxkz with sync annotation, grs will re-enqueue on change: Operation cannot be fulfilled on clusters.management.cattle.io "c-m-s9smgxkz": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:09:36 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject user-8kn8j with role cluster-owner in namespace c-m-s9smgxkz-p-vcw2z
2025/08/03 19:09:36 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for infrastructure ready
2025/08/03 19:09:36 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for infrastructure ready
2025/08/03 19:09:36 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for infrastructure ready
2025/08/03 19:09:36 [INFO] [mgmt-cluster-rbac-delete] Updating cluster c-m-s9smgxkz
2025/08/03 19:09:36 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for at least one control plane, etcd, and worker node to be registered
2025/08/03 19:09:36 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for at least one control plane, etcd, and worker node to be registered
2025/08/03 19:09:37 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for at least one control plane, etcd, and worker node to be registered
2025/08/03 19:09:37 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for at least one control plane, etcd, and worker node to be registered
2025/08/03 19:09:37 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for at least one control plane, etcd, and worker node to be registered
2025/08/03 19:09:38 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for at least one control plane, etcd, and worker node to be registered
2025/08/03 19:09:38 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for at least one control plane, etcd, and worker node to be registered
2025/08/03 19:09:38 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for at least one control plane, etcd, and worker node to be registered
2025/08/03 19:09:38 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for at least one control plane, etcd, and worker node to be registered
2025/08/03 19:09:38 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for at least one control plane, etcd, and worker node to be registered
2025/08/03 19:09:40 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for at least one control plane, etcd, and worker node to be registered
2025/08/03 19:09:58 [INFO] [planner] rkecluster fleet-default/new-k3s-cluster1: waiting for at least one control plane, etcd, and worker node to be registered
2025/08/03 19:09:59 [INFO] Deleting cluster [c-m-s9smgxkz]
2025/08/03 19:09:59 [ERROR] error syncing 'fleet-default/new-k3s-cluster1': handler manage-system-upgrade-controller: failed to delete fleet-default/new-k3s-cluster1-managed-system-upgrade-co-ef3c4 management.cattle.io/v3, Kind=ManagedChart for manage-system-upgrade-controller fleet-default/new-k3s-cluster1: managedcharts.management.cattle.io "new-k3s-cluster1-managed-system-upgrade-co-ef3c4" not found, requeuing
2025/08/03 19:10:03 [INFO] [mgmt-cluster-rbac-remove] Deleting project p-vcw2z
2025/08/03 19:10:03 [INFO] [mgmt-cluster-rbac-remove] Deleting namespace c-m-s9smgxkz
2025/08/03 19:10:03 [ERROR] error syncing 'c-m-s9smgxkz/crt-bmk58': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:03 [ERROR] error syncing 'c-m-s9smgxkz/default-token': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:03 [ERROR] error syncing 'c-m-s9smgxkz/crt-bmk58': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:03 [ERROR] error syncing 'c-m-s9smgxkz/default-token': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:03 [ERROR] error syncing 'c-m-s9smgxkz/default-token': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:03 [ERROR] error syncing 'c-m-s9smgxkz/crt-bmk58': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:03 [INFO] [mgmt-cluster-rbac-remove] Deleting project p-vcw2z
2025/08/03 19:10:03 [INFO] [mgmt-project-rbac-remove] Deleting namespace c-m-s9smgxkz-p-vcw2z
2025/08/03 19:10:03 [ERROR] error syncing 'c-m-s9smgxkz': handler user-controllers-controller: userControllersController: failed to clean finalizers for cluster c-m-s9smgxkz: clusters.management.cattle.io "c-m-s9smgxkz" not found, handler mgmt-cluster-rbac-remove: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:03 [ERROR] error syncing 'c-m-s9smgxkz/default-token': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:03 [ERROR] error syncing 'c-m-s9smgxkz/crt-bmk58': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:03 [ERROR] error syncing 'c-m-s9smgxkz/crt-bmk58': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:03 [ERROR] error syncing 'c-m-s9smgxkz/default-token': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:04 [ERROR] error syncing 'c-m-s9smgxkz/crt-bmk58': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:04 [ERROR] error syncing 'c-m-s9smgxkz/default-token': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:04 [ERROR] error syncing 'c-m-s9smgxkz/default-token': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:04 [ERROR] error syncing 'c-m-s9smgxkz/crt-bmk58': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:05 [ERROR] error syncing 'c-m-s9smgxkz/crt-bmk58': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:05 [ERROR] error syncing 'c-m-s9smgxkz/default-token': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:06 [ERROR] error syncing 'c-m-s9smgxkz/crt-bmk58': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:06 [ERROR] error syncing 'c-m-s9smgxkz/default-token': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:08 [ERROR] error syncing 'c-m-s9smgxkz/crt-bmk58': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:08 [ERROR] error syncing 'c-m-s9smgxkz/default-token': handler cluster-registration-token: clusters.management.cattle.io "c-m-s9smgxkz" not found, requeuing
2025/08/03 19:10:08 [INFO] [mgmt-auth-prtb-controller] Deleting roleBinding rb-pvbcq7kt3q
2025/08/03 19:10:08 [INFO] [mgmt-auth-prtb-controller] Updating owner label for roleBinding crb-u2niihd3fu
2025/08/03 19:10:09 [INFO] [mgmt-project-rbac-remove] Deleting namespace c-m-s9smgxkz-p-n8k9h
2025/08/03 19:10:09 [INFO] [mgmt-auth-crtb-controller] Deleting roleBinding crb-2cmpsjqmc6
2025/08/03 19:10:14 [INFO] [mgmt-auth-prtb-controller] Deleting roleBinding crb-u2niihd3fu
W0803 19:10:20.492368      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:20.562805      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:20.602821      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
2025/08/03 19:10:20 [INFO] [mgmt-cluster-rbac-delete] Creating namespace c-785dp
2025/08/03 19:10:20 [INFO] [mgmt-cluster-rbac-delete] Creating Default project for cluster c-785dp
2025/08/03 19:10:20 [INFO] [mgmt-project-rbac-create] Creating namespace c-785dp-p-2zmlz
2025/08/03 19:10:20 [INFO] [mgmt-cluster-rbac-delete] Creating System project for cluster c-785dp
2025/08/03 19:10:20 [INFO] [mgmt-project-rbac-create] Creating namespace c-785dp-p-88xcd
2025/08/03 19:10:20 [INFO] [mgmt-cluster-rbac-delete] Updating cluster c-785dp
2025/08/03 19:10:20 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-785dp, err=Operation cannot be fulfilled on namespaces "c-785dp": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:10:20 [INFO] [mgmt-project-rbac-create] Creating creator projectRoleTemplateBinding for user user-8kn8j for project p-2zmlz
2025/08/03 19:10:20 [INFO] [mgmt-project-rbac-create] Creating creator projectRoleTemplateBinding for user user-8kn8j for project p-88xcd
W0803 19:10:20.802522      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
2025/08/03 19:10:20 [INFO] [mgmt-cluster-rbac-delete] Creating creator clusterRoleTemplateBinding for user user-8kn8j for cluster c-785dp
2025/08/03 19:10:20 [INFO] [mgmt-project-rbac-create] Setting InitialRolesPopulated condition on project p-2zmlz
2025/08/03 19:10:20 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-785dp, err=Operation cannot be fulfilled on namespaces "c-785dp": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:10:20 [INFO] [mgmt-auth-prtb-controller] Creating role/clusterRole p-2zmlz-projectowner
2025/08/03 19:10:20 [INFO] [mgmt-project-rbac-create] Setting InitialRolesPopulated condition on project p-88xcd
2025/08/03 19:10:20 [INFO] [mgmt-auth-crtb-controller] Creating role/clusterRole c-785dp-clusterowner
2025/08/03 19:10:20 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for membership in project p-2zmlz for subject user-8kn8j
2025/08/03 19:10:20 [INFO] [mgmt-project-rbac-create] Updating project p-2zmlz
2025/08/03 19:10:20 [INFO] [mgmt-auth-crtb-controller] Creating clusterRoleBinding for membership in cluster c-785dp for subject user-8kn8j
2025/08/03 19:10:20 [INFO] [mgmt-auth-prtb-controller] Creating role/clusterRole p-88xcd-projectowner
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Creating role/clusterRole c-785dp-clustermember
W0803 19:10:21.022727      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
2025/08/03 19:10:21 [INFO] [mgmt-cluster-rbac-delete] Setting InitialRolesPopulated condition on cluster c-785dp
2025/08/03 19:10:21 [INFO] [mgmt-cluster-rbac-delete] Updating cluster c-785dp
2025/08/03 19:10:21 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-785dp-p-2zmlz, err=Operation cannot be fulfilled on namespaces "c-785dp-p-2zmlz": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:10:21 [INFO] [mgmt-auth-crtb-controller] Creating role cluster-owner in namespace c-785dp
2025/08/03 19:10:21 [INFO] [mgmt-project-rbac-create] Updating project p-88xcd
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Creating clusterRoleBinding for membership in cluster c-785dp for subject user-8kn8j
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for membership in project p-88xcd for subject user-8kn8j
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Updating clusterRoleBinding crb-4rhn6yann2 for cluster membership in cluster c-785dp for subject user-8kn8j
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Creating role project-owner in namespace c-785dp-p-2zmlz
2025/08/03 19:10:21 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject user-8kn8j with role cluster-owner in namespace c-785dp
2025/08/03 19:10:21 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-785dp-p-88xcd, err=Operation cannot be fulfilled on namespaces "c-785dp-p-88xcd": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:10:21 [INFO] [mgmt-project-rbac-create] Updating project p-2zmlz
2025/08/03 19:10:21 [INFO] [mgmt-project-rbac-create] Updating project p-88xcd
2025/08/03 19:10:21 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-785dp-p-2zmlz, err=Operation cannot be fulfilled on namespaces "c-785dp-p-2zmlz": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Creating role admin in namespace c-785dp-p-2zmlz
2025/08/03 19:10:21 [INFO] [mgmt-auth-crtb-controller] Creating role cluster-owner in namespace c-785dp-p-88xcd
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role project-owner in namespace c-785dp-p-2zmlz
2025/08/03 19:10:21 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject user-8kn8j with role cluster-owner in namespace c-785dp-p-88xcd
2025/08/03 19:10:21 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-785dp-p-2zmlz, err=Operation cannot be fulfilled on namespaces "c-785dp-p-2zmlz": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:10:21 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-785dp-p-88xcd, err=Operation cannot be fulfilled on namespaces "c-785dp-p-88xcd": the object has been modified; please apply your changes to the latest version and try again
W0803 19:10:21.222728      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
2025/08/03 19:10:21 [INFO] [mgmt-auth-crtb-controller] Creating role cluster-owner in namespace c-785dp-p-2zmlz
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role admin in namespace c-785dp-p-2zmlz
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Updating clusterRoleBinding crb-4rhn6yann2 for cluster membership in cluster c-785dp for subject user-8kn8j
2025/08/03 19:10:21 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject user-8kn8j with role cluster-owner in namespace c-785dp-p-2zmlz
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Creating role project-owner in namespace c-785dp-p-88xcd
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Creating role admin in namespace c-785dp-p-88xcd
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role admin in namespace c-785dp-p-88xcd
2025/08/03 19:10:21 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role project-owner in namespace c-785dp-p-88xcd
2025/08/03 19:10:21 [INFO] [mgmt-cluster-rbac-delete] Updating cluster c-785dp
W0803 19:10:21.560728      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:21.760360      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:21.780752      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:21.858630      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:21.889966      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:21.973656      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:22.110007      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:22.196927      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:22.225863      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:22.246718      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:22.276705      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:22.303565      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:22.337523      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:25.588604      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:40.586472      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:55.214770      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:55.242819      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
W0803 19:10:55.313348      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
2025/08/03 19:10:55 [INFO] Deleting cluster [c-785dp]
W0803 19:10:55.347940      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
2025/08/03 19:10:55 [INFO] [mgmt-cluster-rbac-remove] Deleting project p-88xcd
2025/08/03 19:10:55 [INFO] [mgmt-cluster-rbac-remove] Deleting namespace c-785dp
W0803 19:10:55.420580      46 warnings.go:70] The annotation [rancher.io/imported-cluster-version-management] takes effect only on imported RKE2/K3s cluster, please consider removing it from cluster [c-785dp]
2025/08/03 19:10:55 [INFO] [mgmt-project-rbac-remove] Deleting namespace c-785dp-p-88xcd
2025/08/03 19:11:00 [INFO] [mgmt-auth-crtb-controller] Deleting roleBinding crb-yidyuv2fra
2025/08/03 19:11:00 [INFO] [mgmt-auth-crtb-controller] Deleting rolebinding creator-cluster-owner-cluster-owner in namespace c-785dp-p-2zmlz for crtb creator-cluster-owner
2025/08/03 19:11:00 [INFO] [mgmt-project-rbac-remove] Deleting namespace c-785dp-p-2zmlz
2025/08/03 19:11:01 [INFO] [mgmt-auth-prtb-controller] Updating owner label for roleBinding crb-4rhn6yann2
2025/08/03 19:11:05 [INFO] [mgmt-auth-prtb-controller] Deleting roleBinding crb-4rhn6yann2
2025/08/03 19:11:22 [INFO] [mgmt-cluster-rbac-delete] Creating namespace c-9stwq
2025/08/03 19:11:22 [INFO] [mgmt-cluster-rbac-delete] Creating Default project for cluster c-9stwq
2025/08/03 19:11:22 [INFO] [mgmt-project-rbac-create] Creating namespace c-9stwq-p-vrn92
2025/08/03 19:11:22 [INFO] [mgmt-cluster-rbac-delete] Creating System project for cluster c-9stwq
2025/08/03 19:11:22 [INFO] [mgmt-cluster-rbac-delete] Updating cluster c-9stwq
2025/08/03 19:11:22 [INFO] [mgmt-project-rbac-create] Creating namespace c-9stwq-p-brmwv
2025/08/03 19:11:22 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-9stwq, err=Operation cannot be fulfilled on namespaces "c-9stwq": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:11:22 [INFO] [mgmt-project-rbac-create] Creating creator projectRoleTemplateBinding for user user-8kn8j for project p-vrn92
2025/08/03 19:11:22 [INFO] [mgmt-cluster-rbac-delete] Creating creator clusterRoleTemplateBinding for user user-8kn8j for cluster c-9stwq
2025/08/03 19:11:22 [INFO] [mgmt-project-rbac-create] Setting InitialRolesPopulated condition on project p-vrn92
2025/08/03 19:11:22 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-9stwq, err=Operation cannot be fulfilled on namespaces "c-9stwq": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:11:22 [INFO] [mgmt-auth-prtb-controller] Creating role/clusterRole p-vrn92-projectowner
2025/08/03 19:11:22 [INFO] [mgmt-auth-crtb-controller] Creating role/clusterRole c-9stwq-clusterowner
2025/08/03 19:11:22 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for membership in project p-vrn92 for subject user-8kn8j
2025/08/03 19:11:22 [INFO] [mgmt-project-rbac-create] Updating project p-vrn92
2025/08/03 19:11:22 [INFO] [mgmt-project-rbac-create] Creating creator projectRoleTemplateBinding for user user-8kn8j for project p-brmwv
2025/08/03 19:11:22 [INFO] [mgmt-auth-prtb-controller] Creating role/clusterRole c-9stwq-clustermember
2025/08/03 19:11:22 [INFO] [mgmt-auth-crtb-controller] Creating clusterRoleBinding for membership in cluster c-9stwq for subject user-8kn8j
2025/08/03 19:11:22 [INFO] [mgmt-project-rbac-create] Setting InitialRolesPopulated condition on project p-brmwv
2025/08/03 19:11:22 [INFO] [mgmt-cluster-rbac-delete] Setting InitialRolesPopulated condition on cluster c-9stwq
2025/08/03 19:11:22 [INFO] [mgmt-auth-prtb-controller] Creating clusterRoleBinding for membership in cluster c-9stwq for subject user-8kn8j
2025/08/03 19:11:22 [INFO] [mgmt-cluster-rbac-delete] Updating cluster c-9stwq
2025/08/03 19:11:22 [INFO] [mgmt-auth-prtb-controller] Creating role/clusterRole p-brmwv-projectowner
2025/08/03 19:11:22 [INFO] [mgmt-auth-crtb-controller] Creating role cluster-owner in namespace c-9stwq
2025/08/03 19:11:22 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-9stwq-p-vrn92, err=Operation cannot be fulfilled on namespaces "c-9stwq-p-vrn92": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:11:22 [INFO] [mgmt-auth-prtb-controller] Creating role project-owner in namespace c-9stwq-p-vrn92
2025/08/03 19:11:22 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for membership in project p-brmwv for subject user-8kn8j
2025/08/03 19:11:22 [INFO] [mgmt-project-rbac-create] Updating project p-brmwv
2025/08/03 19:11:22 [INFO] [mgmt-project-rbac-create] Updating project p-vrn92
2025/08/03 19:11:22 [INFO] [mgmt-auth-prtb-controller] Creating role admin in namespace c-9stwq-p-vrn92
2025/08/03 19:11:22 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject user-8kn8j with role cluster-owner in namespace c-9stwq
2025/08/03 19:11:22 [INFO] [mgmt-auth-prtb-controller] Updating clusterRoleBinding crb-jmxdsxfmf2 for cluster membership in cluster c-9stwq for subject user-8kn8j
2025/08/03 19:11:22 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role project-owner in namespace c-9stwq-p-vrn92
2025/08/03 19:11:22 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-9stwq-p-vrn92, err=Operation cannot be fulfilled on namespaces "c-9stwq-p-vrn92": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:11:22 [INFO] [mgmt-auth-crtb-controller] Creating role cluster-owner in namespace c-9stwq-p-vrn92
2025/08/03 19:11:22 [INFO] [mgmt-project-rbac-create] Updating project p-brmwv
2025/08/03 19:11:23 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role admin in namespace c-9stwq-p-vrn92
2025/08/03 19:11:23 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-9stwq-p-brmwv, err=Operation cannot be fulfilled on namespaces "c-9stwq-p-brmwv": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:11:23 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject user-8kn8j with role cluster-owner in namespace c-9stwq-p-vrn92
2025/08/03 19:11:23 [INFO] [mgmt-auth-prtb-controller] Updating clusterRoleBinding crb-jmxdsxfmf2 for cluster membership in cluster c-9stwq for subject user-8kn8j
2025/08/03 19:11:23 [INFO] [mgmt-auth-prtb-controller] Creating role project-owner in namespace c-9stwq-p-brmwv
2025/08/03 19:11:23 [INFO] [mgmt-auth-crtb-controller] Creating role cluster-owner in namespace c-9stwq-p-brmwv
2025/08/03 19:11:23 [INFO] [mgmt-auth-prtb-controller] Creating role admin in namespace c-9stwq-p-brmwv
2025/08/03 19:11:23 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject user-8kn8j with role cluster-owner in namespace c-9stwq-p-brmwv
2025/08/03 19:11:23 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-9stwq-p-brmwv, err=Operation cannot be fulfilled on namespaces "c-9stwq-p-brmwv": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:11:23 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-9stwq-p-vrn92, err=Operation cannot be fulfilled on namespaces "c-9stwq-p-vrn92": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:11:23 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role project-owner in namespace c-9stwq-p-brmwv
2025/08/03 19:11:23 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-9stwq-p-brmwv, err=Operation cannot be fulfilled on namespaces "c-9stwq-p-brmwv": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:11:23 [INFO] [mgmt-auth-prtb-controller] Creating roleBinding for subject user-8kn8j with role admin in namespace c-9stwq-p-brmwv
2025/08/03 19:11:23 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=c-9stwq-p-brmwv, err=Operation cannot be fulfilled on namespaces "c-9stwq-p-brmwv": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:11:23 [INFO] [mgmt-cluster-rbac-delete] Updating cluster c-9stwq









2025/08/03 19:17:46 [INFO] Handling backend connection request [c-9stwq]
2025/08/03 19:17:46 [INFO] Starting cluster controllers for c-9stwq
2025/08/03 19:17:47 [INFO] Starting rbac.authorization.k8s.io/v1, Kind=ClusterRoleBinding controller
2025/08/03 19:17:47 [INFO] Starting rbac.authorization.k8s.io/v1, Kind=RoleBinding controller
2025/08/03 19:17:47 [INFO] Starting /v1, Kind=ResourceQuota controller
2025/08/03 19:17:47 [INFO] Starting /v1, Kind=Node controller
2025/08/03 19:17:47 [INFO] Starting /v1, Kind=Secret controller
2025/08/03 19:17:47 [INFO] Starting apiregistration.k8s.io/v1, Kind=APIService controller
2025/08/03 19:17:47 [INFO] Starting cluster agent for c-9stwq [owner=true]
2025/08/03 19:17:47 [INFO] Starting /v1, Kind=Namespace controller
2025/08/03 19:17:47 [INFO] Starting /v1, Kind=LimitRange controller
2025/08/03 19:17:47 [INFO] Starting /v1, Kind=ServiceAccount controller
2025/08/03 19:17:47 [INFO] Starting rbac.authorization.k8s.io/v1, Kind=Role controller
2025/08/03 19:17:47 [INFO] Starting rbac.authorization.k8s.io/v1, Kind=ClusterRole controller
2025/08/03 19:17:47 [INFO] Creating clusterRole for roleTemplate Project Owner (project-owner).
2025/08/03 19:17:47 [INFO] RDPClient: certificate updated successfully
2025/08/03 19:17:47 [INFO] Creating clusterRole for roleTemplate Create Namespaces (create-ns).
2025/08/03 19:17:47 [INFO] Creating clusterRole for roleTemplate Cluster Owner (cluster-owner).
2025/08/03 19:17:47 [INFO] Creating clusterRole project-owner-promoted for project access to global resource.
2025/08/03 19:17:47 [INFO] Creating clusterRole for roleTemplate Cluster Owner (cluster-owner).
2025/08/03 19:17:47 [INFO] Created machine for node [plex3-7050m]
2025/08/03 19:17:48 [INFO] Creating clusterRoleBinding User user-8kn8j Role cluster-owner
2025/08/03 19:17:48 [INFO] Creating clusterRole project-owner-promoted for project access to global resource.
2025/08/03 19:17:48 [INFO] Created machine for node [plex1-7050m]
2025/08/03 19:17:48 [INFO] Creating user for principal system://c-9stwq
2025/08/03 19:17:48 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=cattle-system, err=Operation cannot be fulfilled on namespaces "cattle-system": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:17:48 [INFO] Trying to create an already existing clusterRole project-owner-promoted for project access to global resource.
2025/08/03 19:17:48 [INFO] Updating clusterRole project-owner-promoted for project access to global resource.
2025/08/03 19:17:48 [INFO] Creating globalRoleBindings for u-6j2l6mpfir
2025/08/03 19:17:48 [INFO] EnsureSecretForServiceAccount: waiting for secret [cattle-impersonation-system:cattle-impersonation-user-8kn8j-token-fp8jz] for service account [cattle-impersonation-system:cattle-impersonation-user-8kn8j] to be populated with token
2025/08/03 19:17:48 [INFO] Updating clusterRole project-owner-promoted for project access to global resource.
2025/08/03 19:17:48 [INFO] EnsureSecretForServiceAccount: got the service account token for service account [cattle-impersonation-system:cattle-impersonation-user-8kn8j] in 115.95525ms
2025/08/03 19:17:48 [INFO] Updating clusterRole project-owner-promoted for project access to global resource.
2025/08/03 19:17:48 [INFO] Rolling back ServiceAccount secret for [cattle-impersonation-system:cattle-impersonation-user-8kn8j-token-fmt44]
2025/08/03 19:17:48 [INFO] Creating roleBinding User user-8kn8j Role admin in default
2025/08/03 19:17:48 [INFO] Creating new GlobalRoleBinding for GlobalRoleBinding grb-vnxgf
2025/08/03 19:17:48 [INFO] [mgmt-auth-grb-controller] Creating clusterRoleBinding for globalRoleBinding grb-vnxgf for user u-6j2l6mpfir with role cattle-globalrole-user
2025/08/03 19:17:48 [INFO] Updating clusterRole project-owner-promoted for project access to global resource.
2025/08/03 19:17:48 [INFO] Updating clusterRole project-owner-promoted for project access to global resource.
2025/08/03 19:17:48 [INFO] Creating system token for u-6j2l6mpfir, token: agent-u-6j2l6mpfir
2025/08/03 19:17:48 [INFO] Creating roleBinding User user-8kn8j Role project-owner in default
2025/08/03 19:17:48 [INFO] [mgmt-auth-crtb-controller] Creating clusterRoleBinding for membership in cluster c-9stwq for subject u-6j2l6mpfir
2025/08/03 19:17:48 [INFO] Creating roleBinding User user-8kn8j Role project-owner in cattle-system
2025/08/03 19:17:48 [INFO] Creating roleBinding User user-8kn8j Role admin in kube-node-lease
2025/08/03 19:17:48 [INFO] Creating roleBinding User user-8kn8j Role project-owner in default
2025/08/03 19:17:48 [INFO] Updating clusterRole project-owner-promoted for project access to global resource.
2025/08/03 19:17:48 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject u-6j2l6mpfir with role cluster-owner in namespace c-9stwq
2025/08/03 19:17:48 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject u-6j2l6mpfir with role cluster-owner in namespace c-9stwq-p-vrn92
2025/08/03 19:17:48 [INFO] Creating roleBinding User user-8kn8j Role project-owner in kube-public
2025/08/03 19:17:48 [INFO] Creating roleBinding User user-8kn8j Role admin in cattle-system
2025/08/03 19:17:48 [INFO] Creating clusterRoleBinding for project access to global resource for subject user-8kn8j role create-ns.
2025/08/03 19:17:48 [INFO] Updating clusterRole project-owner-promoted for project access to global resource.
2025/08/03 19:17:48 [INFO] Creating roleBinding User user-8kn8j Role project-owner in kube-system
2025/08/03 19:17:48 [INFO] Creating roleBinding User user-8kn8j Role project-owner in kube-node-lease
2025/08/03 19:17:48 [INFO] Updating clusterRole project-owner-promoted for project access to global resource.
2025/08/03 19:17:48 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject u-6j2l6mpfir with role cluster-owner in namespace c-9stwq-p-brmwv
2025/08/03 19:17:48 [INFO] Creating clusterRoleBinding for project access to global resource for subject user-8kn8j role p-brmwv-namespaces-edit.
2025/08/03 19:17:48 [INFO] Updating clusterRoleBinding crb-2ws74jhhbq for project access to global resource for subject user-8kn8j role create-ns.
2025/08/03 19:17:49 [INFO] Creating roleBinding User user-8kn8j Role project-owner in kube-system
2025/08/03 19:17:49 [INFO] Creating roleBinding User user-8kn8j Role admin in kube-public
2025/08/03 19:17:49 [INFO] Creating roleBinding User user-8kn8j Role admin in kube-system
2025/08/03 19:17:49 [INFO] Creating clusterRoleBinding for project access to global resource for subject user-8kn8j role project-owner-promoted.
2025/08/03 19:17:49 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=cattle-impersonation-system, err=Operation cannot be fulfilled on namespaces "cattle-impersonation-system": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:17:49 [INFO] Creating clusterRoleBinding for project access to global resource for subject user-8kn8j role p-vrn92-namespaces-edit.
2025/08/03 19:17:49 [INFO] Creating roleBinding User user-8kn8j Role admin in cattle-impersonation-system
2025/08/03 19:17:49 [INFO] Creating roleBinding User user-8kn8j Role project-owner in cattle-impersonation-system
2025/08/03 19:17:49 [INFO] Updating clusterRoleBinding crb-ugwe2mfnyv for project access to global resource for subject user-8kn8j role project-owner-promoted.
2025/08/03 19:17:49 [INFO] Creating roleBinding User user-8kn8j Role project-owner in cattle-impersonation-system
2025/08/03 19:17:49 [INFO] Updating clusterRoleBinding crb-ugwe2mfnyv for project access to global resource for subject user-8kn8j role project-owner-promoted.
2025/08/03 19:17:49 [INFO] Creating clusterRoleBinding User u-6j2l6mpfir Role cluster-owner
2025/08/03 19:17:49 [INFO] EnsureSecretForServiceAccount: waiting for secret [cattle-impersonation-system:cattle-impersonation-u-6j2l6mpfir-token-bns4g] for service account [cattle-impersonation-system:cattle-impersonation-u-6j2l6mpfir] to be populated with token
2025/08/03 19:17:49 [INFO] EnsureSecretForServiceAccount: waiting for secret [cattle-impersonation-system:cattle-impersonation-u-6j2l6mpfir-token-bns4g] for service account [cattle-impersonation-system:cattle-impersonation-u-6j2l6mpfir] to be populated with token
2025/08/03 19:17:49 [INFO] EnsureSecretForServiceAccount: got the service account token for service account [cattle-impersonation-system:cattle-impersonation-u-6j2l6mpfir] in 47.213074ms
2025/08/03 19:17:50 [INFO] EnsureSecretForServiceAccount: got the service account token for service account [cattle-impersonation-system:cattle-impersonation-u-6j2l6mpfir] in 55.201243ms
2025/08/03 19:17:56 [INFO] Creating system token for u-6j2l6mpfir, token: agent-u-6j2l6mpfir
2025/08/03 19:17:59 [INFO] Handling backend connection request [c-9stwq]
2025/08/03 19:18:05 [INFO] Redeploy Rancher Agents is needed for c-9stwq: forceDeploy=false, agent/auth image changed=false, private repo changed=false, agent features changed=true
2025/08/03 19:18:05 [INFO] Creating system token for u-6j2l6mpfir, token: agent-u-6j2l6mpfir
2025/08/03 19:18:08 [ERROR] Failed to handle tunnel request from remote address 94.140.19.73:12968: response 401: failed authentication
2025/08/03 19:18:08 [INFO] Handling backend connection request [stv-cluster-c-9stwq]
2025/08/03 19:18:15 [INFO] Redeploy Rancher Agents is needed for c-9stwq: forceDeploy=false, agent/auth image changed=false, private repo changed=false, agent features changed=true
2025/08/03 19:18:15 [INFO] Creating system token for u-6j2l6mpfir, token: agent-u-6j2l6mpfir
2025/08/03 19:18:16 [INFO] Handling backend connection request [c-9stwq]
2025/08/03 19:18:17 [INFO] Handling backend connection request [c-9stwq]
2025/08/03 19:18:17 [INFO] error in remotedialer server [400]: websocket: close 1006 (abnormal closure): unexpected EOF
2025/08/03 19:18:17 [INFO] error in remotedialer server [400]: websocket: close 1006 (abnormal closure): unexpected EOF
2025/08/03 19:18:23 [INFO] [mgmt-auth-crtb-controller] Creating clusterRoleBinding for membership in cluster c-9stwq for subject u-esphmo3gcc
2025/08/03 19:18:23 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject u-esphmo3gcc with role cluster-owner in namespace c-9stwq
2025/08/03 19:18:23 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject u-esphmo3gcc with role cluster-owner in namespace c-9stwq-p-vrn92
2025/08/03 19:18:23 [INFO] Redeploy Rancher Agents is needed for c-9stwq: forceDeploy=false, agent/auth image changed=false, private repo changed=false, agent features changed=true
2025/08/03 19:18:23 [INFO] Creating system token for u-6j2l6mpfir, token: agent-u-6j2l6mpfir
2025/08/03 19:18:23 [INFO] [mgmt-auth-crtb-controller] Creating roleBinding for subject u-esphmo3gcc with role cluster-owner in namespace c-9stwq-p-brmwv
2025/08/03 19:18:23 [INFO] Handling backend connection request [c-9stwq]
2025/08/03 19:18:23 [INFO] namespaceHandler: addProjectIDLabelToNamespace: adding label field.cattle.io/projectId=p-ljxb7 to namespace=cluster-fleet-default-c-9stwq-357e5891626c
2025/08/03 19:18:23 [ERROR] namespaceHandler: Sync: error adding project id label to namespace err=Operation cannot be fulfilled on namespaces "cluster-fleet-default-c-9stwq-357e5891626c": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:18:23 [INFO] namespaceHandler: addProjectIDLabelToNamespace: adding label field.cattle.io/projectId=p-ljxb7 to namespace=cluster-fleet-default-c-9stwq-357e5891626c
2025/08/03 19:18:23 [ERROR] namespaceHandler: Sync: error adding project id label to namespace err=Operation cannot be fulfilled on namespaces "cluster-fleet-default-c-9stwq-357e5891626c": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:18:23 [INFO] namespaceHandler: addProjectIDLabelToNamespace: adding label field.cattle.io/projectId=p-ljxb7 to namespace=cluster-fleet-default-c-9stwq-357e5891626c
2025/08/03 19:18:23 [INFO] Creating clusterRoleBinding User u-esphmo3gcc Role cluster-owner
2025/08/03 19:18:24 [INFO] Rolling back ServiceAccount secret for [cattle-impersonation-system:cattle-impersonation-u-esphmo3gcc-token-wbws5]
2025/08/03 19:18:24 [INFO] EnsureSecretForServiceAccount: waiting for secret [cattle-impersonation-system:cattle-impersonation-u-esphmo3gcc-token-f84zb] for service account [cattle-impersonation-system:cattle-impersonation-u-esphmo3gcc] to be populated with token
2025/08/03 19:18:24 [INFO] EnsureSecretForServiceAccount: got the service account token for service account [cattle-impersonation-system:cattle-impersonation-u-esphmo3gcc] in 197.848082ms
2025/08/03 19:18:24 [INFO] error in remotedialer server [400]: websocket: close 1006 (abnormal closure): unexpected EOF
2025/08/03 19:18:29 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=cattle-fleet-system, err=Operation cannot be fulfilled on namespaces "cattle-fleet-system": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:18:29 [INFO] Creating roleBinding User user-8kn8j Role project-owner in cattle-fleet-system
2025/08/03 19:18:30 [INFO] Creating roleBinding User user-8kn8j Role admin in cattle-fleet-system
2025/08/03 19:18:30 [INFO] error in remotedialer server [400]: websocket: close 1006 (abnormal closure): unexpected EOF
W0803 19:18:30.312387      46 reflector.go:492] /root/.cache/go/modcache/k8s.io/client-go@v0.32.2/tools/cache/reflector.go:251: watch of *v1.ClusterRole ended with: an error on the server ("unable to decode an event from the watch stream: tunnel disconnect") has prevented the request from succeeding
W0803 19:18:30.312559      46 reflector.go:492] /root/.cache/go/modcache/k8s.io/client-go@v0.32.2/tools/cache/reflector.go:251: watch of *v1.LimitRange ended with: an error on the server ("unable to decode an event from the watch stream: tunnel disconnect") has prevented the request from succeeding
W0803 19:18:30.312581      46 reflector.go:492] /root/.cache/go/modcache/k8s.io/client-go@v0.32.2/tools/cache/reflector.go:251: watch of *v1.Node ended with: an error on the server ("unable to decode an event from the watch stream: tunnel disconnect") has prevented the request from succeeding
W0803 19:18:30.312595      46 reflector.go:492] /root/.cache/go/modcache/k8s.io/client-go@v0.32.2/tools/cache/reflector.go:251: watch of *v1.ResourceQuota ended with: an error on the server ("unable to decode an event from the watch stream: tunnel disconnect") has prevented the request from succeeding
W0803 19:18:30.312741      46 reflector.go:492] /root/.cache/go/modcache/k8s.io/client-go@v0.32.2/tools/cache/reflector.go:251: watch of *v1.ServiceAccount ended with: an error on the server ("unable to decode an event from the watch stream: tunnel disconnect") has prevented the request from succeeding
W0803 19:18:30.312762      46 reflector.go:492] /root/.cache/go/modcache/k8s.io/client-go@v0.32.2/tools/cache/reflector.go:251: watch of *v1.ClusterRoleBinding ended with: an error on the server ("unable to decode an event from the watch stream: tunnel disconnect") has prevented the request from succeeding
W0803 19:18:30.312987      46 reflector.go:492] /root/.cache/go/modcache/k8s.io/client-go@v0.32.2/tools/cache/reflector.go:251: watch of *v1.Secret ended with: an error on the server ("unable to decode an event from the watch stream: tunnel disconnect") has prevented the request from succeeding
W0803 19:18:30.313010      46 reflector.go:492] /root/.cache/go/modcache/k8s.io/client-go@v0.32.2/tools/cache/reflector.go:251: watch of *v1.APIService ended with: an error on the server ("unable to decode an event from the watch stream: tunnel disconnect") has prevented the request from succeeding
W0803 19:18:30.313025      46 reflector.go:492] /root/.cache/go/modcache/k8s.io/client-go@v0.32.2/tools/cache/reflector.go:251: watch of *v1.RoleBinding ended with: an error on the server ("unable to decode an event from the watch stream: tunnel disconnect") has prevented the request from succeeding
W0803 19:18:30.313067      46 reflector.go:492] /root/.cache/go/modcache/k8s.io/client-go@v0.32.2/tools/cache/reflector.go:251: watch of *v1.Namespace ended with: an error on the server ("unable to decode an event from the watch stream: tunnel disconnect") has prevented the request from succeeding
W0803 19:18:30.313609      46 reflector.go:492] /root/.cache/go/modcache/k8s.io/client-go@v0.32.2/tools/cache/reflector.go:251: watch of *v1.Role ended with: an error on the server ("unable to decode an event from the watch stream: tunnel disconnect") has prevented the request from succeeding
2025/08/03 19:18:30 [ERROR] error syncing 'cattle-fleet-system': handler namespace-auth: ensuring PRTBs are added to namespace cattle-fleet-system: couldn't ensure binding creator-project-owner in cattle-fleet-system: Post "https://10.43.0.1:443/apis/rbac.authorization.k8s.io/v1/namespaces/cattle-fleet-system/rolebindings": tunnel disconnect, requeuing
2025/08/03 19:18:30 [INFO] Creating roleBinding User user-8kn8j Role admin in cattle-fleet-system
2025/08/03 19:18:31 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=cattle-fleet-system, err=Operation cannot be fulfilled on namespaces "cattle-fleet-system": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:18:31 [INFO] Creating roleBinding User user-8kn8j Role admin in cattle-fleet-system
2025/08/03 19:18:31 [ERROR] defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=cattle-fleet-system, err=Operation cannot be fulfilled on namespaces "cattle-fleet-system": the object has been modified; please apply your changes to the latest version and try again
2025/08/03 19:18:31 [INFO] Creating roleBinding User user-8kn8j Role admin in cattle-fleet-system
2025/08/03 19:18:31 [INFO] Redeploy Rancher Agents is needed for c-9stwq: forceDeploy=false, agent/auth image changed=false, private repo changed=false, agent features changed=true
2025/08/03 19:18:31 [INFO] Creating system token for u-6j2l6mpfir, token: agent-u-6j2l6mpfir
2025/08/03 19:18:33 [ERROR] Failed to handle tunnel request from remote address 94.140.19.73:48036: response 401: failed authentication
2025/08/03 19:18:34 [INFO] Handling backend connection request [stv-cluster-c-9stwq]
2025/08/03 19:18:35 [ERROR] Failed to handle tunnel request from remote address 94.140.19.73:7567: response 401: failed authentication
2025/08/03 19:18:35 [INFO] Handling backend connection request [stv-cluster-c-9stwq]

