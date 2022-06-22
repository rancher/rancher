# README

This terraform module provides a way to provision a couple of Rancher Hosted instances below a Super Rancher and each with a downstream custom cluster where Rancher v2 Apps are installed. Everything using the Linode Linodes.

- Super Rancher
  - Hosted Rancher 1
    - Custom Cluster 1
  - Hosted Rancher 2
    - Custom Cluster 2

So far only valid certs are supported so the certs should be inside the rancher_hosted module. The certificates are the standard QA certificates we use for automation so they have to be replaced every three months in the nginx load balancers (see nginx configuration template path)

- `fullchain.pem`
- `privkey.pem`

These variables are required to create the files:

- `privkey="<base64 string of the privkey.pem"`
- `fullchain="<base64 string of the fullchain.pem"`

The nginx load balancer configuration template path: `modules/rancher-hosted/scripts/nginx/nginx.conf`

A path to the private ssh key using the `ssh_private_key_path` variable is also required, this is for the recollection of the k3s kubeconfig files and k3s token files too. This key should match with the public `ssh_authorized_key` variable provided for the Linode ssh configuration.

A `terraform.tfvars` is required to set the variables easily for the module to work, an example of such file:

```bash
linode_token="<the linode token>"
linode_root_password="<linode strong password>"
linode_resource_prefix="<linode resources prefix"
zone_id="<Route53 zone ID"
load_balancers_domain="<route53 domain>"
super_load_balancer_subdomain="subdomain for the super Rancher"
hosted1_load_balancer_subdomain="subdomain for the hosted1 Rancher"
hosted2_load_balancer_subdomain="subdomain for the hosted2 Rancher"
ssh_authorized_key="ssh-rsa AAAA..."
rancher_password="<Rancher password> this will be set to all the Ranchers"
rancher_version_super="2.6.5"
rancher_version_hosted1="2.6.5"
rancher_version_hosted2="2.5.14"
rke2_cluster_version_hosted1="v1.21.4+rke2r2"
rke2_cluster_version_hosted2="v1.21.4+rke2r2"
rke2_cluster_version_hosted3="v1.21.4+rke2r2"
k3s_version_super="v1.22.9+k3s1"
k3s_version_hosted1="v1.22.9+k3s1"
k3s_version_hosted2="v1.20.12+k3s1"
ssh_private_key="<base64 string of the ssh private key"
privkey="<base64 string of the privkey.pem"
fullchain="<base64 string of the fullchain.pem"
```

After these requirements are met it'll be just matter of:

- `terraform get` 
- `terraform init` 
- `terraform apply`

## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.0 |


## Modules

| Name | Source | Version |
|------|--------|---------|
| <a name="module_rancher_hosted"></a> [rancher\_hosted](#module\_rancher\_hosted) | ./modules/rancher-hosted | n/a |

## Resources

No resources.

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_docker_version"></a> [docker\_version](#input\_docker\_version) | The default Docker version to use for the long living environment | `string` | `"20.10"` | no |
| <a name="input_fullchain"></a> [fullchain](#input\_fullchain) | The base64 fullchain certificate | `string` | n/a | yes |
| <a name="input_hosted1_load_balancer_subdomain"></a> [hosted1\_load\_balancer\_subdomain](#input\_hosted1\_load\_balancer\_subdomain) | Rancher load balancer subdomain of the hosted1 cluster | `string` | n/a | yes |
| <a name="input_hosted2_load_balancer_subdomain"></a> [hosted2\_load\_balancer\_subdomain](#input\_hosted2\_load\_balancer\_subdomain) | Rancher load balancer subdomain of the hosted2 cluster | `string` | n/a | yes |
| <a name="input_hosted3_load_balancer_subdomain"></a> [hosted3\_load\_balancer\_subdomain](#input\_hosted3\_load\_balancer\_subdomain) | Rancher load balancer subdomain of the hosted3 cluster | `string` | n/a | yes |
| <a name="input_k3s_version_hosted1"></a> [k3s\_version\_hosted1](#input\_k3s\_version\_hosted1) | The Hosted 1 k3s version | `string` | `"v1.22.9+k3s1"` | no |
| <a name="input_k3s_version_hosted2"></a> [k3s\_version\_hosted2](#input\_k3s\_version\_hosted2) | The Hosted 2 k3s version | `string` | `"v1.20.12+k3s1"` | no |
| <a name="input_k3s_version_hosted3"></a> [k3s\_version\_hosted3](#input\_k3s\_version\_hosted3) | The Hosted 2 k3s version | `string` | `"v1.20.12+k3s1"` | no |
| <a name="input_k3s_version_super"></a> [k3s\_version\_super](#input\_k3s\_version\_super) | The Super Rancher version | `string` | `"v1.22.9+k3s1"` | no |
| <a name="input_linode_resource_prefix"></a> [linode\_resource\_prefix](#input\_linode\_resource\_prefix) | Linode Resource Prefix | `string` | n/a | yes |
| <a name="input_linode_root_password"></a> [linode\_root\_password](#input\_linode\_root\_password) | root password to use by default in linodes | `string` | n/a | yes |
| <a name="input_linode_token"></a> [linode\_token](#input\_linode\_token) | Linode Token | `string` | n/a | yes |
| <a name="input_load_balancers_domain"></a> [load\_balancers\_domain](#input\_load\_balancers\_domain) | Rancher load balancers domain | `string` | n/a | yes |
| <a name="input_privkey"></a> [privkey](#input\_privkey) | The base64 private key certificate | `string` | n/a | yes |
| <a name="input_rancher_password"></a> [rancher\_password](#input\_rancher\_password) | The password that will be used to bootstrap Rancher | `string` | n/a | yes |
| <a name="input_rancher_version_hosted1"></a> [rancher\_version\_hosted1](#input\_rancher\_version\_hosted1) | The Hosted Rancher version, set same as super if not using different branches | `string` | `"2.6.8"` | no |
| <a name="input_rancher_version_hosted2"></a> [rancher\_version\_hosted2](#input\_rancher\_version\_hosted2) | The Hosted Rancher version, set same as super if not using different branches | `string` | `"2.5.14"` | no |
| <a name="input_rancher_version_hosted3"></a> [rancher\_version\_hosted3](#input\_rancher\_version\_hosted3) | The Hosted Rancher version, set same as super if not using different branches | `string` | `"2.6.8"` | no |
| <a name="input_rancher_version_super"></a> [rancher\_version\_super](#input\_rancher\_version\_super) | The Super Rancher version | `string` | `"2.6.5"` | no |
| <a name="input_rke2_cluster_version_hosted1"></a> [rke2\_cluster\_version\_hosted1](#input\_rke2\_cluster\_version\_hosted1) | The kubernetes version for the downstream cluster on the hosted1 if Rancher 2.6+ | `string` | `"v1.21.4+rke2r2"` | no |
| <a name="input_rke2_cluster_version_hosted2"></a> [rke2\_cluster\_version\_hosted2](#input\_rke2\_cluster\_version\_hosted2) | The kubernetes version for the downstream cluster on the hosted2 if Rancher 2.6+ | `string` | `"v1.21.4+rke2r2"` | no |
| <a name="input_rke2_cluster_version_hosted3"></a> [rke2\_cluster\_version\_hosted3](#input\_rke2\_cluster\_version\_hosted3) | The kubernetes version for the downstream cluster on the hosted3 if Rancher 2.6+ | `string` | `"v1.21.4+rke2r2"` | no |
| <a name="input_ssh_authorized_key"></a> [ssh\_authorized\_key](#input\_ssh\_authorized\_key) | Public SSH key to add into linodes | `string` | n/a | yes |
| <a name="input_ssh_private_key"></a> [ssh\_private\_key](#input\_ssh\_private\_key) | The private key ssh key path to retrieve the kubeconfig and token from k3s | `string` | n/a | yes |
| <a name="input_super_load_balancer_subdomain"></a> [super\_load\_balancer\_subdomain](#input\_super\_load\_balancer\_subdomain) | Super Rancher load balancer subdomain | `string` | n/a | yes |
| <a name="input_zone_id"></a> [zone\_id](#input\_zone\_id) | The zone id to create DNS records on | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_hosted1_url"></a> [hosted1\_url](#output\_hosted1\_url) | n/a |
| <a name="output_hosted2_url"></a> [hosted2\_url](#output\_hosted2\_url) | n/a |
| <a name="output_hosted3_url"></a> [hosted3\_url](#output\_hosted3\_url) | n/a |
| <a name="output_super_host_url"></a> [super\_host\_url](#output\_super\_host\_url) | n/a |
