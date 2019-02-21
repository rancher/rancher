package globaldns

var LinodeDeploymentTemplate = `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: {{.providerName}}
spec:
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: {{.providerName}}
    spec:
      serviceAccountName: external-dns
      containers:
      - name: {{.providerName}}
        image: {{.externalDnsImage}}
        args:
        - --source=service
        - --source=ingress
        - --domain-filter={{.rootDomain}}
        - --provider=linode
        - --registry=txt
        - --txt-owner-id=my-identifier
        - --log-level=debug
        - --publish-internal-services
        env:
        - name: LINODE_TOKEN
          value: {{.secretKey}}
        volumeMounts:
        - name: config
          mountPath: /etc/linode
          readOnly: true
        - mountPath: /usr/share/zoneinfo
          name: zoneinfo
          readOnly: true
      volumes:
      - name: zoneinfo
        hostPath:
          path: /usr/share/zoneinfo`
