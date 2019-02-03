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
        image: {{.externalDnsImage}}
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
        - --txt-owner-id={{.identifier}}
        - --log-level=debug
        - --publish-internal-services`
