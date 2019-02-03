package globaldns

var CloudflareDeploymentTemplate = `
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
        args:
        - --source=service
        - --source=ingress
        - --domain-filter={{.cloudflareDomain}} # (optional) limit to only example.com domains; change to match the zone created above.
        - --provider=cloudflare
        - --cloudflare-proxied # (optional) enable the proxy feature of Cloudflare (DDOS protection, CDN...)
        - --log-level=debug
        - --txt-owner-id={{.identifier}}
        - --publish-internal-services
        env:
        - name: CF_API_KEY
          value: "{{.apiKey}}"
        - name: CF_API_EMAIL
          value: "{{.apiEmail}}"`
