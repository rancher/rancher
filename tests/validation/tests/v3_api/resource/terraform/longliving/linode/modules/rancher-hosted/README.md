## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_aws"></a> [aws](#requirement\_aws) | ~> 4.15.1 |
| <a name="requirement_helm"></a> [helm](#requirement\_helm) | ~> 2.5.1 |
| <a name="requirement_linode"></a> [linode](#requirement\_linode) | ~> 1.27.2 |
| <a name="requirement_rancher2"></a> [rancher2](#requirement\_rancher2) | 1.21.0 |
| <a name="requirement_random"></a> [random](#requirement\_random) | ~> 3.2.0 |
| <a name="requirement_ssh"></a> [ssh](#requirement\_ssh) | ~> 1.2.0 |

## Providers

| Name | Version |
|------|---------|
| <a name="provider_aws"></a> [aws](#provider\_aws) | ~> 4.15.1 |
| <a name="provider_helm.rancher-hosted1"></a> [helm.rancher-hosted1](#provider\_helm.rancher-hosted1) | ~> 2.5.1 |
| <a name="provider_helm.rancher-hosted2"></a> [helm.rancher-hosted2](#provider\_helm.rancher-hosted2) | ~> 2.5.1 |
| <a name="provider_helm.rancher-hosted3"></a> [helm.rancher-hosted3](#provider\_helm.rancher-hosted3) | ~> 2.5.1 |
| <a name="provider_helm.rancher-super"></a> [helm.rancher-super](#provider\_helm.rancher-super) | ~> 2.5.1 |
| <a name="provider_linode"></a> [linode](#provider\_linode) | ~> 1.27.2 |
| <a name="provider_local"></a> [local](#provider\_local) | n/a |
| <a name="provider_null"></a> [null](#provider\_null) | n/a |
| <a name="provider_rancher2.admin"></a> [rancher2.admin](#provider\_rancher2.admin) | 1.21.0 |
| <a name="provider_rancher2.admin_hosted1"></a> [rancher2.admin\_hosted1](#provider\_rancher2.admin\_hosted1) | 1.21.0 |
| <a name="provider_rancher2.admin_hosted2"></a> [rancher2.admin\_hosted2](#provider\_rancher2.admin\_hosted2) | 1.21.0 |
| <a name="provider_rancher2.admin_hosted3"></a> [rancher2.admin\_hosted3](#provider\_rancher2.admin\_hosted3) | 1.21.0 |
| <a name="provider_rancher2.bootstrap"></a> [rancher2.bootstrap](#provider\_rancher2.bootstrap) | 1.21.0 |
| <a name="provider_rancher2.bootstrap_hosted1"></a> [rancher2.bootstrap\_hosted1](#provider\_rancher2.bootstrap\_hosted1) | 1.21.0 |
| <a name="provider_rancher2.bootstrap_hosted2"></a> [rancher2.bootstrap\_hosted2](#provider\_rancher2.bootstrap\_hosted2) | 1.21.0 |
| <a name="provider_rancher2.bootstrap_hosted3"></a> [rancher2.bootstrap\_hosted3](#provider\_rancher2.bootstrap\_hosted3) | 1.21.0 |
| <a name="provider_random"></a> [random](#provider\_random) | ~> 3.2.0 |
| <a name="provider_ssh"></a> [ssh](#provider\_ssh) | ~> 1.2.0 |

## Resources

| Name | Type |
|------|------|
| [aws_route53_record.hosted1_rancher](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route53_record) | resource |
| [aws_route53_record.hosted2_rancher](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route53_record) | resource |
| [aws_route53_record.hosted3_rancher](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route53_record) | resource |
| [aws_route53_record.super_rancher](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/route53_record) | resource |
| [helm_release.rancher_hosted1_server](https://registry.terraform.io/providers/hashicorp/helm/latest/docs/resources/release) | resource |
| [helm_release.rancher_hosted2_server](https://registry.terraform.io/providers/hashicorp/helm/latest/docs/resources/release) | resource |
| [helm_release.rancher_hosted3_server](https://registry.terraform.io/providers/hashicorp/helm/latest/docs/resources/release) | resource |
| [helm_release.rancher_server](https://registry.terraform.io/providers/hashicorp/helm/latest/docs/resources/release) | resource |
| [linode_firewall.load_balancers_firewall](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/firewall) | resource |
| [linode_firewall.rancher_clusters_firewall](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/firewall) | resource |
| [linode_firewall.rancher_custom_clusters_firewall](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/firewall) | resource |
| [linode_instance.custom_nodes1](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.custom_nodes2](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.custom_nodes3](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.hosted1_lb](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.hosted1_node1](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.hosted1_node2](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.hosted1_node3](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.hosted2_lb](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.hosted2_node1](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.hosted2_node2](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.hosted2_node3](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.hosted3_lb](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.hosted3_node1](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.hosted3_node2](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.hosted3_node3](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.super_lb](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.super_node1](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.super_node2](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [linode_instance.super_node3](https://registry.terraform.io/providers/linode/linode/latest/docs/resources/instance) | resource |
| [local_file.fullchain](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.k3s_token](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.k3s_token_hosted1](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.k3s_token_hosted2](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.k3s_token_hosted3](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.kube_config_hosted1_yaml](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.kube_config_hosted2_yaml](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.kube_config_hosted3_yaml](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.kube_config_server_yaml](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.privkey](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [null_resource.import_hosted_cluster1](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.import_hosted_cluster2](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.import_hosted_cluster3](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.wait_for_hosted1_rancher](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.wait_for_hosted2_rancher](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.wait_for_hosted3_rancher](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.wait_for_ingress_rollout_hosted1](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.wait_for_ingress_rollout_hosted2](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.wait_for_ingress_rollout_hosted3](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.wait_for_ingress_rollout_super](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [null_resource.wait_for_rancher](https://registry.terraform.io/providers/hashicorp/null/latest/docs/resources/resource) | resource |
| [rancher2_bootstrap.admin](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/bootstrap) | resource |
| [rancher2_bootstrap.admin_hosted1](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/bootstrap) | resource |
| [rancher2_bootstrap.admin_hosted2](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/bootstrap) | resource |
| [rancher2_bootstrap.admin_hosted3](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/bootstrap) | resource |
| [rancher2_cloud_credential.linode_rke2_hosted1](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cloud_credential) | resource |
| [rancher2_cloud_credential.linode_rke2_hosted2](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cloud_credential) | resource |
| [rancher2_cloud_credential.linode_rke2_hosted3](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cloud_credential) | resource |
| [rancher2_cluster.custom_cluster1](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cluster) | resource |
| [rancher2_cluster.custom_cluster2](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cluster) | resource |
| [rancher2_cluster.custom_cluster3](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cluster) | resource |
| [rancher2_cluster.hosted1](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cluster) | resource |
| [rancher2_cluster.hosted2](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cluster) | resource |
| [rancher2_cluster.hosted3](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cluster) | resource |
| [rancher2_cluster_sync.custom_cluster1](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cluster_sync) | resource |
| [rancher2_cluster_sync.custom_cluster2](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cluster_sync) | resource |
| [rancher2_cluster_sync.custom_cluster3](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cluster_sync) | resource |
| [rancher2_cluster_v2.linode_rke2_hosted1](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cluster_v2) | resource |
| [rancher2_cluster_v2.linode_rke2_hosted2](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cluster_v2) | resource |
| [rancher2_cluster_v2.linode_rke2_hosted3](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/cluster_v2) | resource |
| [rancher2_machine_config_v2.linode_rke2_control_plane_hosted1](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/machine_config_v2) | resource |
| [rancher2_machine_config_v2.linode_rke2_control_plane_hosted2](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/machine_config_v2) | resource |
| [rancher2_machine_config_v2.linode_rke2_control_plane_hosted3](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/machine_config_v2) | resource |
| [rancher2_machine_config_v2.linode_rke2_etcd_hosted1](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/machine_config_v2) | resource |
| [rancher2_machine_config_v2.linode_rke2_etcd_hosted2](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/machine_config_v2) | resource |
| [rancher2_machine_config_v2.linode_rke2_etcd_hosted3](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/machine_config_v2) | resource |
| [rancher2_machine_config_v2.linode_rke2_worker_hosted1](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/machine_config_v2) | resource |
| [rancher2_machine_config_v2.linode_rke2_worker_hosted2](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/machine_config_v2) | resource |
| [rancher2_machine_config_v2.linode_rke2_worker_hosted3](https://registry.terraform.io/providers/rancher/rancher2/1.21.0/docs/resources/machine_config_v2) | resource |
| [random_string.k3s_token](https://registry.terraform.io/providers/hashicorp/random/latest/docs/resources/string) | resource |
| [ssh_resource.retrieve_config_hosted1](https://registry.terraform.io/providers/loafoe/ssh/latest/docs/resources/resource) | resource |
| [ssh_resource.retrieve_config_hosted2](https://registry.terraform.io/providers/loafoe/ssh/latest/docs/resources/resource) | resource |
| [ssh_resource.retrieve_config_hosted3](https://registry.terraform.io/providers/loafoe/ssh/latest/docs/resources/resource) | resource |
| [ssh_resource.retrieve_config_super](https://registry.terraform.io/providers/loafoe/ssh/latest/docs/resources/resource) | resource |
| [ssh_resource.retrieve_token](https://registry.terraform.io/providers/loafoe/ssh/latest/docs/resources/resource) | resource |
| [ssh_resource.retrieve_token_hosted1](https://registry.terraform.io/providers/loafoe/ssh/latest/docs/resources/resource) | resource |
| [ssh_resource.retrieve_token_hosted2](https://registry.terraform.io/providers/loafoe/ssh/latest/docs/resources/resource) | resource |
| [ssh_resource.retrieve_token_hosted3](https://registry.terraform.io/providers/loafoe/ssh/latest/docs/resources/resource) | resource |

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
| <a name="input_load_balancers_domain"></a> [load\_balancers\_domain](#input\_load\_balancers\_domain) | Rancher load balancerd domain | `string` | n/a | yes |
| <a name="input_privkey"></a> [privkey](#input\_privkey) | The base64 private key certificate | `string` | n/a | yes |
| <a name="input_rancher_password"></a> [rancher\_password](#input\_rancher\_password) | The password that will be used to bootstrap Rancher | `string` | n/a | yes |
| <a name="input_rancher_version_hosted1"></a> [rancher\_version\_hosted1](#input\_rancher\_version\_hosted1) | The Hosted Rancher version, set same as super if not using different branches | `string` | `"2.6.8"` | no |
| <a name="input_rancher_version_hosted2"></a> [rancher\_version\_hosted2](#input\_rancher\_version\_hosted2) | The Hosted Rancher version, set same as super if not using different branches | `string` | `"2.5.14"` | no |
| <a name="input_rancher_version_hosted3"></a> [rancher\_version\_hosted3](#input\_rancher\_version\_hosted3) | The Hosted Rancher version, set same as super if not using different branches | `string` | `"2.6.8"` | no |
| <a name="input_rancher_version_super"></a> [rancher\_version\_super](#input\_rancher\_version\_super) | The Super Rancher version | `string` | `"2.6.5"` | no |
| <a name="input_rke2_cluster_version_hosted1"></a> [rke2\_cluster\_version\_hosted1](#input\_rke2\_cluster\_version\_hosted1) | The kubernetes version for the downstream cluster on the hosted1 | `string` | `"v1.21.4+rke2r2"` | no |
| <a name="input_rke2_cluster_version_hosted2"></a> [rke2\_cluster\_version\_hosted2](#input\_rke2\_cluster\_version\_hosted2) | The kubernetes version for the downstream cluster on the hosted2 | `string` | `"v1.21.4+rke2r2"` | no |
| <a name="input_rke2_cluster_version_hosted3"></a> [rke2\_cluster\_version\_hosted3](#input\_rke2\_cluster\_version\_hosted3) | The kubernetes version for the downstream cluster on the hosted3 | `string` | `"v1.21.4+rke2r2"` | no |
| <a name="input_ssh_authorized_key"></a> [ssh\_authorized\_key](#input\_ssh\_authorized\_key) | Public SSH key to add into linodes | `string` | n/a | yes |
| <a name="input_ssh_private_key"></a> [ssh\_private\_key](#input\_ssh\_private\_key) | The private key ssh key path to retrieve the kubeconfig and token from k3s | `string` | n/a | yes |
| <a name="input_super_load_balancer_subdomain"></a> [super\_load\_balancer\_subdomain](#input\_super\_load\_balancer\_subdomain) | Super Rancher load balancer subdomain | `string` | n/a | yes |
| <a name="input_zone_id"></a> [zone\_id](#input\_zone\_id) | The zone id to create DNS records on | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_hosted1_fqdn"></a> [hosted1\_fqdn](#output\_hosted1\_fqdn) | n/a |
| <a name="output_hosted2_fqdn"></a> [hosted2\_fqdn](#output\_hosted2\_fqdn) | n/a |
| <a name="output_hosted3_fqdn"></a> [hosted3\_fqdn](#output\_hosted3\_fqdn) | n/a |
| <a name="output_super_host_fqdn"></a> [super\_host\_fqdn](#output\_super\_host\_fqdn) | n/a |
