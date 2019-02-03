package globaldns

var AlidnsDeploymentTemplate = `
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
        - --provider=alibabacloud
        - --alibaba-cloud-zone-type=public # only look at public hosted zones (valid values are public, private or no value for both)
        - --registry=txt
        - --txt-owner-id=my-identifier
        - --alibaba-cloud-config-file=/etc/alibaba/config.yaml
        - --log-level=debug
        - --publish-internal-services
        volumeMounts:
        - name: config
          mountPath: /etc/alibaba
          readOnly: true
        - mountPath: /usr/share/zoneinfo
          name: zoneinfo
          readOnly: true
      volumes:
      - name: config
        secret:
          secretName: {{.providerName}}
      - name: zoneinfo
        hostPath:
          path: /usr/share/zoneinfo`

var AlidnsSecretTemplate = `
apiVersion: v1
kind: Secret
metadata:
  name: {{.providerName}}
type: Opaque
stringData:
  config.yaml: |-
    accessKeyId: {{.accessKey}}
    accessKeySecret: {{.secretKey}}`
