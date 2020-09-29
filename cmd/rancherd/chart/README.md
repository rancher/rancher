# Rancherd

***Rancherd*** is a simplified version of rancher chart that is intended to be used by rancherd.

# Overview

Rancherd chart provision rancher in a daemonset, expose hostport 8080/8443 down to the container port(80/443), and use hostpath to mount cert if needed.

# Configuring certs for rancher server

Rancherd does not use cert-manger to provision certs, instead it looks for `/etc/rancher/ssl` on your hostpath for existing certs:

Private key: `/etc/rancher/ssl/key.pem`

Certificate: `/etc/rancher/ssl/cert.pem`

CA Certificate(self-signed): `/etc/rancher/ssl/cacerts.pem`

Additional CA Certificate: `/etc/ssl/certs/ca-additional.pem`

# Customize chart

Rancherd use helm-controller to bootstrap rancherd chart, in order to provide a customized value.yaml it has be to be done through helm-controller CRD. 
Here is an example of the manifest:

```yaml
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rancher
  namespace: kube-system
spec:
  valuesContent: |
    publicCA: true
```

Put this manifest on your host `/var/lib/rancher/rke2/server/manifests` before running rancherd.

#### Common Options

| Parameter                      | Default Value                                         | Description                                                                                                                                                                                                  |
| ------------------------------ | ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `addLocal`                     | "auto"                                                | ***string*** - Have Rancher detect and import the “local” Rancher server cluster [Import "local Cluster"](https://rancher.com/docs/rancher/v2.x/en/installation/options/chart-options/#import-local-cluster) |
| `auditLog.destination`         | "sidecar"                                             | ***string*** - Stream to sidecar container console or hostPath volume - *"sidecar, hostPath"*                                                                                                                |
| `auditLog.hostPath`            | "/var/log/rancher/audit"                              | ***string*** - log file destination on host (only applies when **auditLog.destination** is set to **hostPath**)                                                                                              |
| `auditLog.level`               | 0                                                     | ***int*** - set the [API Audit Log level](https://rancher.com/docs/rancher/v2.x/en/installation/api-auditing). 0 is off. [0-3]                                                                               |
| `auditLog.maxAge`              | 1                                                     | ***int*** - maximum number of days to retain old audit log files (only applies when **auditLog.destination** is set to **hostPath**)                                                                         |
| `auditLog.maxBackups`          | 1                                                     | int - maximum number of audit log files to retain (only applies when **auditLog.destination** is set to **hostPath**)                                                                                        |
| `auditLog.maxSize`             | 100                                                   | ***int*** - maximum size in megabytes of the audit log file before it gets rotated (only applies when **auditLog.destination** is set to **hostPath**)                                                       |
| `debug`                        | false                                                 | ***bool*** - set debug flag on rancher server                                                                                                                                                                |
| `extraEnv`                     | []                                                    | ***list*** - set additional environment variables for Rancher Note: *Available as of v2.2.0*                                                                                                                 |
| `imagePullSecrets`             | []                                                    | ***list*** - list of names of Secret resource containing private registry credentials                                                                                                                        |
| `proxy`                        | " "                                                   | ***string** - HTTP[S] proxy server for Rancher                                                                                                                                                               |
| `noProxy`                      | "127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16" | ***string*** - comma separated list of hostnames or ip address not to use the proxy                                                                                                                          |
| `resources`                    | {}                                                    | ***map*** - rancher pod resource requests & limits                                                                                                                                                           |
| `rancherImage`                 | "rancher/rancher"                                     | ***string*** - rancher image source                                                                                                                                                                          |
| `rancherImageTag`              | same as chart version                                 | ***string*** - rancher/rancher image tag                                                                                                                                                                     |
| `rancherImagePullPolicy`       | "IfNotPresent"                                        | ***string*** - Override imagePullPolicy for rancher server images - *"Always", "Never", "IfNotPresent"*                                                                                                      |
| `systemDefaultRegistry`        | ""                                                    | ***string*** - private registry to be used for all system Docker images, e.g., [http://registry.example.com/] *Available as of v2.3.0*                                                                       |
| `useBundledSystemChart`        | false                                                 | ***bool*** - select to use the system-charts packaged with Rancher server. This option is used for air gapped installations.  *Available as of v2.3.0*                                                       |
| `publicCA`                     | false                                                 | ***bool*** - Set to true if your cert is signed by a public CA                                                                                                                                               |