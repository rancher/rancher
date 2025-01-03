By installing this application, you accept the [End User License Agreement & Terms & Conditions](https://www.suse.com/licensing/eula/).

# Rancher

***Rancher*** is open source software that combines everything an organization needs to adopt and run containers in production. Built on Kubernetes, Rancher makes it easy for DevOps teams to test, deploy and manage their applications.

### Introduction

This chart bootstraps a [Rancher Server](https://ranchermanager.docs.rancher.com/pages-for-subheaders/install-upgrade-on-a-kubernetes-cluster) on a Kubernetes cluster using the [Helm](https://helm.sh/) package manager. For a Rancher Supported Deployment please follow our [HA install instructions](https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/kubernetes-cluster-setup/high-availability-installs).


### Prerequisites Details

*For installations covered under [Rancher Support SLA](https://www.suse.com/suse-rancher/support-matrix/all-supported-versions) the target cluster must be **[RKE1](https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/kubernetes-cluster-setup/rke1-for-rancher)**, **[RKE2](https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/kubernetes-cluster-setup/rke2-for-rancher)**, **[K3s](https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/kubernetes-cluster-setup/k3s-for-rancher)**, **[AKS](https://ranchermanager.docs.rancher.com/getting-started/installation-and-upgrade/install-upgrade-on-a-kubernetes-cluster/rancher-on-aks)**, **[EKS](https://ranchermanager.docs.rancher.com/getting-started/installation-and-upgrade/install-upgrade-on-a-kubernetes-cluster/rancher-on-amazon-eks)**, or **[GKE](https://ranchermanager.docs.rancher.com/getting-started/installation-and-upgrade/install-upgrade-on-a-kubernetes-cluster/rancher-on-gke)**.*

Make sure the node(s) for the Rancher server fulfill the following requirements:

[Operating Systems and Container Runtime Requirements](https://ranchermanager.docs.rancher.com/pages-for-subheaders/installation-requirements#operating-systems-and-container-runtime-requirements)
[Hardware Requirements](https://ranchermanager.docs.rancher.com/pages-for-subheaders/installation-requirements#hardware-requirements)

- [CPU and Memory](https://ranchermanager.docs.rancher.com/pages-for-subheaders/installation-requirements#cpu-and-memory)
- [Ingress](https://ranchermanager.docs.rancher.com/pages-for-subheaders/installation-requirements#ingress)
- [Disks](https://ranchermanager.docs.rancher.com/pages-for-subheaders/installation-requirements#disks)

[Networking Requirements](https://ranchermanager.docs.rancher.com/pages-for-subheaders/installation-requirements#networking-requirements)
- [Node IP Addresses](https://ranchermanager.docs.rancher.com/pages-for-subheaders/installation-requirements#node-ip-addresses)
- [Port Requirements](https://ranchermanager.docs.rancher.com/pages-for-subheaders/installation-requirements#port-requirements)

[Install the Required CLI Tools](https://ranchermanager.docs.rancher.com/pages-for-subheaders/cli-with-rancher)

- [kubectl](https://ranchermanager.docs.rancher.com/reference-guides/cli-with-rancher/kubectl-utility) - Kubernetes command-line tool.
- [helm](https://docs.helm.sh/using_helm/#installing-helm) - Package management for Kubernetes. Refer to the [Helm version requirements](https://ranchermanager.docs.rancher.com/getting-started/installation-and-upgrade/resources/helm-version-requirements) to choose a version of Helm to install Rancher.

For a list of best practices that we recommend for running the Rancher server in production, refer to the [best practices section](https://ranchermanager.docs.rancher.com/pages-for-subheaders/best-practices).

## Installing Rancher

For production environments, we recommend installing Rancher in a [high-availability Kubernetes installation](https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/kubernetes-cluster-setup/high-availability-installs) so that your user base can always access Rancher Server. When installed in a Kubernetes cluster, Rancher will integrate with the cluster’s etcd database and take advantage of Kubernetes scheduling for high-availability.

Optional: Installing Rancher on a [Single-node](https://ranchermanager.docs.rancher.com/pages-for-subheaders/rancher-on-a-single-node-with-docker) Kubernetes Cluster

#### Add the Helm Chart Repository

Use [helm repo add](https://helm.sh/docs/helm/helm_repo_add/) command to add the Helm chart repository that contains charts to install Rancher. For more information about the repository choices and which is best for your use case, see Choosing a Version of Rancher.

```bash
helm repo add rancher-latest https://releases.rancher.com/server-charts/latest
```

#### Create a Namespace for Rancher

We’ll need to define a Kubernetes namespace where the resources created by the Chart should be installed. This should always be cattle-system:

```bash
kubectl create namespace cattle-system
```

#### Choose your SSL Configuration

The Rancher management server is designed to be secure by default and requires SSL/TLS configuration.

There are three recommended options for the source of the certificate used for TLS termination at the Rancher server:

- [Rancher-generated TLS certificate](https://ranchermanager.docs.rancher.com/pages-for-subheaders/install-upgrade-on-a-kubernetes-cluster#3-choose-your-ssl-configuration)
- [Let’s Encrypt](https://ranchermanager.docs.rancher.com/pages-for-subheaders/install-upgrade-on-a-kubernetes-cluster#3-choose-your-ssl-configuration)
- [Bring your own certificate](https://ranchermanager.docs.rancher.com/pages-for-subheaders/install-upgrade-on-a-kubernetes-cluster#3-choose-your-ssl-configuration)

#### Install cert-manager

This step is only required to use certificates issued by Rancher’s generated CA **`(ingress.tls.source=rancher)`** or to request Let’s Encrypt issued certificates **`(ingress.tls.source=letsEncrypt)`**.

[These instructions are adapted from the official cert-manager documentation.](https://ranchermanager.docs.rancher.com/pages-for-subheaders/install-upgrade-on-a-kubernetes-cluster#4-install-cert-manager)

#### Install Rancher with Helm and Your Chosen Certificate Option

- [Rancher to generated certificates](https://ranchermanager.docs.rancher.com/pages-for-subheaders/install-upgrade-on-a-kubernetes-cluster#5-install-rancher-with-helm-and-your-chosen-certificate-option)
```bash
helm install rancher rancher-latest/rancher \
  --namespace cattle-system \
  --set hostname=rancher.my.org
```

- [Let’s Encrypt](https://ranchermanager.docs.rancher.com/pages-for-subheaders/install-upgrade-on-a-kubernetes-cluster#5-install-rancher-with-helm-and-your-chosen-certificate-option)

```bash
helm install rancher rancher-latest/rancher \
  --namespace cattle-system \
  --set hostname=rancher.my.org \
  --set ingress.tls.source=letsEncrypt \
  --set letsEncrypt.email=me@example.org
```

- [Certificates from Files](https://ranchermanager.docs.rancher.com/pages-for-subheaders/install-upgrade-on-a-kubernetes-cluster#5-install-rancher-with-helm-and-your-chosen-certificate-option)

```bash
helm install rancher rancher-latest/rancher \
  --namespace cattle-system \
  --set hostname=rancher.my.org \
  --set ingress.tls.source=secret
```

*If you are using a Private CA signed certificate , add **--set privateCA=true** to the command:`*

```bash
helm install rancher rancher-latest/rancher \
  --namespace cattle-system \
  --set hostname=rancher.my.org \
  --set ingress.tls.source=secret \
  --set privateCA=true
```

#### Verify that the Rancher Server is Successfully Deployed

After adding the secrets, check if Rancher was rolled out successfully:

```bash
kubectl -n cattle-system rollout status deploy/rancher
Waiting for deployment "rancher" rollout to finish: 0 of 3 updated replicas are available...
deployment "rancher" successfully rolled out
```

If you see the following **`error: error: deployment "rancher" exceeded its progress deadline`**, you can check the status of the deployment by running the following command:

```bash
kubectl -n cattle-system get deploy rancher
NAME      DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
rancher   3         3         3            3           3m
```

It should show the same count for **`DESIRED`** and **`AVAILABLE`**.

#### Save Your Options

Make sure you save the **`--set`** options you used. You will need to use the same options when you upgrade Rancher to new versions with Helm.

#### Finishing Up

That’s it. You should have a functional Rancher server.

In a web browser, go to the DNS name that forwards traffic to your load balancer. Then you should be greeted by the colorful login page.

Doesn’t work? Take a look at the [Troubleshooting Page](https://ranchermanager.docs.rancher.com/troubleshooting/general-troubleshooting)

***All of these instructions are defined in detailed in the [Rancher Documentation](https://ranchermanager.docs.rancher.com/pages-for-subheaders/install-upgrade-on-a-kubernetes-cluster#install-the-rancher-helm-chart).***

### Helm Chart Options for Kubernetes Installations

The full [Helm Chart Options](https://ranchermanager.docs.rancher.com/getting-started/installation-and-upgrade/installation-references/helm-chart-options) can be found here.

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`.

#### Common Options

| Parameter                 | Default Value | Description                                                                                  |
| ------------------------- | ------------- | -------------------------------------------------------------------------------------------- |
| `hostname`                | " "           | ***string*** - the Fully Qualified Domain Name for your Rancher Server                       |
| `ingress.tls.source`      | "rancher"     | ***string*** - Where to get the cert for the ingress. - "***rancher, letsEncrypt, secret***" |
| `letsEncrypt.email`       | " "           | ***string*** - Your email address                                                            |
| `letsEncrypt.environment` | "production"  | ***string*** - Valid options: "***staging, production***"                                    |
| `privateCA`               | false         | ***bool*** - Set to true if your cert is signed by a private CA                              |

#### Advanced Options

| Parameter                                | Default Value                                                             | Description                                                                                                                                                                                                                                                                             |
| ---------------------------------------- | ------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `additionalTrustedCAs`                   | false                                                                     | ***bool*** - [See Additional Trusted CAs Server](https://ranchermanager.docs.rancher.com/getting-started/installation-and-upgrade/installation-references/helm-chart-options#additional-trusted-cas)                                                                                                                                   |
| `addLocal`                               | "true"                                                                    | ***string*** - As of Rancher v2.5.0 this flag is deprecated and must be set to "true"                                                                                                                                                                                                   |
| `antiAffinity`                           | "preferred"                                                               | ***string*** - AntiAffinity rule for Rancher pods - *"preferred, required"*                                                                                                                                                                                                             |
| `replicas`                               | 3                                                                         | ***int*** - Number of replicas of Rancher pods                                                                                                                                                                                                                                          |
| `auditLog.destination`                   | "sidecar"                                                                 | ***string*** - Stream to sidecar container console or hostPath volume - *"sidecar, hostPath"*                                                                                                                                                                                           |
| `auditLog.hostPath`                      | "/var/log/rancher/audit"                                                  | ***string*** - log file destination on host (only applies when **auditLog.destination** is set to **hostPath**)                                                                                                                                                                         |
| `auditLog.level`                         | 0                                                                         | ***int*** - set the [API Audit Log level](https://ranchermanager.docs.rancher.com/how-to-guides/advanced-user-guides/enable-api-audit-log#audit-log-levels). 0 is off. [0-3]                                                                                                                                                          |
| `auitLog.enabled`                        | false                                                                     | ***bool*** - enable the rancher audit logging system |
| `auditLog.maxAge`                        | 1                                                                         | ***int*** - maximum number of days to retain old audit log files (only applies when **auditLog.destination** is set to **hostPath**)                                                                                                                                                    |
| `auditLog.maxBackup`                     | 1                                                                         | int - maximum number of audit log files to retain (only applies when **auditLog.destination** is set to **hostPath**)                                                                                                                                                                   |
| `auditLog.maxSize`                       | 100                                                                       | ***int*** - maximum size in megabytes of the audit log file before it gets rotated (only applies when **auditLog.destination** is set to **hostPath**)                                                                                                                                  |
| `auditLog.image.repository`              | "rancher/mirrored-bci-micro"                                              | ***string*** - Location for the image used to collect audit logs *Note: Available as of v2.7.0*                                                                                                                                                                                         |
| `auditLog.image.tag`                     | "15.4.14.3"                                                               | ***string*** - Tag for the image used to collect audit logs *Note: Available as of v2.7.0*                                                                                                                                                                                              |
| `auditLog.image.pullPolicy`              | "IfNotPresent"                                                            | ***string*** - Override imagePullPolicy for auditLog images - *"Always", "Never", "IfNotPresent"* *Note: Available as of v2.7.0*                                                                                                                                                        |
| `busyboxImage`                           | ""                                                                        | ***string*** - *Deprecated `auditlog.image.repository` should be used to control auditing sidecar image.* Image location for busybox image used to collect audit logs *Note: Available as of v2.2.0, and  Deprecated as of v2.7.0*                                                      |
| `busyboxImagePullPolicy`                 | "IfNotPresent"                                                            | ***string*** - - *Deprecated `auditlog.image.pullPolicy` should be used to control auditing sidecar image.* Override imagePullPolicy for busybox images - *"Always", "Never", "IfNotPresent"* *Deprecated as of v2.7.0*                                                                 |
| `debug`                                  | false                                                                     | ***bool*** - set debug flag on rancher server                                                                                                                                                                                                                                           |
| `certmanager.version`                    | " "                                                                       | ***string*** - set cert-manager compatibility                                                                                                                                                                                                                                           |
| `extraEnv`                               | []                                                                        | ***list*** - set additional environment variables for Rancher Note: *Available as of v2.2.0*                                                                                                                                                                                            |
| `imagePullSecrets`                       | []                                                                        | ***list*** - list of names of Secret resource containing private registry credentials                                                                                                                                                                                                   |
| `ingress.enabled`                        | true                                                                      | ***bool*** - install ingress resource                                                                                                                                                                                                                                                   |
| `ingress.ingressClassName`               | " "                                                                       | ***string*** - class name of ingress if not set manually or by the ingress controller's defaults                                                                                                                                                                                        |
| `ingress.includeDefaultExtraAnnotations` | true                                                                      | ***bool*** - Add default nginx annotations                                                                                                                                                                                                                                              |
| `ingress.extraAnnotations`               | {}                                                                        | ***map*** - additional annotations to customize the ingress                                                                                                                                                                                                                             |
| `ingress.configurationSnippet`           | " "                                                                       | ***string*** - Add additional Nginx configuration. Can be used for proxy configuration. Note: *Available as of v2.0.15, v2.1.10 and v2.2.4*                                                                                                                                             |
| `service.annotations`                    | {}                                                                        | ***map*** - annotations to customize the service                                                                                                                                                                                                                                        |
| `service.type`                           | " "                                                                       | ***string*** - Override the type used for the service - *"NodePort", "LoadBalancer", "ClusterIP"*                                                                                                                                                                                       |
| `letsEncrypt.ingress.class`              | " "                                                                       | ***string*** - optional ingress class for the cert-manager acmesolver ingress that responds to the Let’s *Encrypt ACME challenges*                                                                                                                                                      |
| `proxy`                                  | " "                                                                       | ***string** - HTTP[S] proxy server for Rancher                                                                                                                                                                                                                                          |
| `noProxy`                                | "127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,.svc,.cluster.local" | ***string*** - comma separated list of hostnames or ip address not to use the proxy                                                                                                                                                                                                     |
| `resources`                              | {}                                                                        | ***map*** - rancher pod resource requests & limits                                                                                                                                                                                                                                      |
| `rancherImage`                           | "rancher/rancher"                                                         | ***string*** - rancher image source                                                                                                                                                                                                                                                     |
| `rancherImageTag`                        | same as chart version                                                     | ***string*** - rancher/rancher image tag                                                                                                                                                                                                                                                |
| `rancherImagePullPolicy`                 | "IfNotPresent"                                                            | ***string*** - Override imagePullPolicy for rancher server images - *"Always", "Never", "IfNotPresent"*                                                                                                                                                                                 |
| `tls`                                    | "ingress"                                                                 | ***string*** - See External TLS Termination for details. - *"ingress, external"*                                                                                                                                                                                                        |
| `systemDefaultRegistry`                  | ""                                                                        | ***string*** - private registry to be used for all system Docker images, e.g., [http://registry.example.com/] *Available as of v2.3.0*                                                                                                                                                  |
| `useBundledSystemChart`                  | false                                                                     | ***bool*** - select to use the system-charts packaged with Rancher server. This option is used for air gapped installations.  *Available as of v2.3.0*                                                                                                                                  |
| `customLogos.enabled`                    | false                                                                     | ***bool*** - Enabled [Ember Rancher UI (cluster manager) custom logos](https://github.com/rancher/ui/tree/master/public/assets/images/logos) and [Vue Rancher UI (cluster explorer) custom logos](https://github.com/rancher/dashboard/tree/master/shell/assets/images/pl) persistence volume |
| `customLogos.volumeSubpaths.emberUi`     | "ember"                                                                   | ***string*** - Volume subpath for [Ember Rancher UI (cluster manager) custom logos](https://github.com/rancher/ui/tree/master/public/assets/images/logos) persistence                                                                                                                   |
| `customLogos.volumeSubpaths.vueUi`       | "vue"                                                                     | ***string*** - Volume subpath for [Vue Rancher UI (cluster explorer) custom logos](https://github.com/rancher/dashboard/tree/master/shell/assets/images/pl) persistence                                                                                                                       |
| `customLogos.volumeName`                 | ""                                                                        | ***string*** - Use an existing volume. Custom logos should be copied to the proper `volume/subpath` folder by the user. Optional for persistentVolumeClaim, required for configMap                                                                                                      |
| `customLogos.storageClass`               | ""                                                                        | ***string*** - Set custom logos persistentVolumeClaim storage class. Required for dynamic pv                                                                                                                                                                                            |
| `customLogos.accessMode`                 | "ReadWriteOnce"                                                           | ***string*** - Set custom persistentVolumeClaim access mode                                                                                                                                                                                                                             |
| `customLogos.size`                       | "1Gi"                                                                     | ***string*** - Set custom persistentVolumeClaim size                                                                                                                                                                                                                                    |
