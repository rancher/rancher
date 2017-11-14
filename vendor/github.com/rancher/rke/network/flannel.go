package network

import "fmt"

func GetFlannelManifest(flannelConfig map[string]string) string {
	var extraArgs string
	if len(flannelConfig[FlannelIface]) > 0 {
		extraArgs = fmt.Sprintf(",--iface=%s", flannelConfig[FlannelIface])
	}
	return `
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: kube-flannel-cfg
  namespace: "kube-system"
  labels:
    tier: node
    app: flannel
data:
  cni-conf.json: |
    {
      "name":"cbr0",
      "cniVersion":"0.3.1",
      "plugins":[
        {
          "type":"flannel",
          "delegate":{
            "forceAddress":true,
            "isDefaultGateway":true
          }
        },
        {
          "type":"portmap",
          "capabilities":{
            "portMappings":true
          }
        }
      ]
    }
  net-conf.json: |
    {
      "Network": "` + flannelConfig[ClusterCIDR] + `",
      "Backend": {
        "Type": "vxlan"
      }
    }
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: kube-flannel
  namespace: "kube-system"
  labels:
    tier: node
    k8s-app: flannel
spec:
  template:
    metadata:
      labels:
        tier: node
        k8s-app: flannel
    spec:
      containers:
      - name: kube-flannel
        image: ` + flannelConfig[FlannelImage] + `
        imagePullPolicy: IfNotPresent
        resources:
          limits:
            cpu: 300m
            memory: 500M
          requests:
            cpu: 150m
            memory: 64M
        command: ["/opt/bin/flanneld","--ip-masq","--kube-subnet-mgr"` + extraArgs + `]
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
        volumeMounts:
        - name: run
          mountPath: /run
        - name: cni
          mountPath: /etc/cni/net.d
        - name: flannel-cfg
          mountPath: /etc/kube-flannel/
      - name: install-cni
        image: ` + flannelConfig[FlannelCNIImage] + `
        command: ["/install-cni.sh"]
        env:
        # The CNI network config to install on each node.
        - name: CNI_NETWORK_CONFIG
          valueFrom:
            configMapKeyRef:
              name: kube-flannel-cfg
              key: cni-conf.json
        - name: CNI_CONF_NAME
          value: "10-flannel.conflist"
        volumeMounts:
        - name: cni
          mountPath: /host/etc/cni/net.d
        - name: host-cni-bin
          mountPath: /host/opt/cni/bin/
      hostNetwork: true
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      volumes:
        - name: run
          hostPath:
            path: /run
        - name: cni
          hostPath:
            path: /etc/cni/net.d
        - name: flannel-cfg
          configMap:
            name: kube-flannel-cfg
        - name: host-cni-bin
          hostPath:
            path: /opt/cni/bin
  updateStrategy:
    rollingUpdate:
      maxUnavailable: 20%
    type: RollingUpdate`
}
