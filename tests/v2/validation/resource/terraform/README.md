# Terraform Chart Installations

Scripts to target an existing cluster for chart installations or to provision a cluster with charts installed

---
#### NOTES
- Leaving v2 chart versions blank ("") will install the latest version in scope for release catalog
- v1 target clusters require manual installation of v1 cluster-monitoring, v1 provisioned clusters will include v1 cluster-monitoring
- v1 Istio is deprecated after 2.5x, this chart can be installed using a supported version
- v2 Rancher Backups will always be installed into the local cluster, proper admin permissions are required `install_rancher_backups = 1` will indicate sufficent access from `rancher_token_key` to install v2 Rancher Backups, while `install_rancher_backups = 0` will skip this chart installation
- v2 Neuvector requires a cluster type parameter, use `neuvector_docker_runtime = true` for RKE1 and `neuvector_k3s_runtime = true` for RKE2 target clusters. Provisioned clusters will be RKE1 docker installations via node template
- v2 Neuvector is not in scope for 2.5x (Partner Charts) `install_rancher_neuvector = 1` will install v2 Neuvector, while  `install_rancher_neuvector = 0` will skip this chart installation
___

## Setup
To run locally you will need to:
- create a file titled `terraform.tfvars` nested inside the folder of the script to be executed
- add desired values to corresponding environment variables

Sample `terraform.tfvars` for v1 charts
```
rancher_api_url = ""
rancher_token_key = ""
project_id = ""
cluster_provider = ""
cluster_name = ""
aws_ami = ""
aws_ami_user = "
aws_vpc_id = ""
aws_zone = "" 
aws_region = ""
aws_root_size = ""
aws_instance_type = ""
aws_volume_type = ""
worker_count = 
rke_version = ""
rancher_istio_version = ""
rancher_logging_endpoint = ""
rancher_longhorn_prereq_version = ""
```

Sample `terraform.tfvars` for v2 charts
```
rancher_api_url = ""
rancher_token_key = ""
cluster_id = ""
project_id = ""
rancher_monitoring_version = ""
rancher_alerting_version = ""
rancher_kiali_version = ""
rancher_tracing_version = ""
rancher_istio_version = ""
rancher_logging_version = ""
rancher_cis_version = ""
rancher_gatekeeper_version = ""
rancher_longhorn_version = ""
rancher_longhorn_prereq_version = ""
rancher_backup_version = ""
rancher_neuvector_version = ""
install_rancher_neuvector = 1
install_rancher_backups = 0
neuvector_docker_runtime = true
neuvector_k3s_runtime = false
neuvector_crio_runtime = false
neuvector_containerd_runtime = false
cluster_provider = ""
cluster_name = ""
aws_ami = ""
aws_ami_user = ""
aws_vpc_id = ""
aws_zone = "" 
aws_region = ""
aws_root_size = ""
aws_instance_type = ""
aws_volume_type = ""
worker_count = 
rke_version = "" 
```
___
## To Execute
Run the following commands from inside the desired folder to initiate the script:
- `terraform init`
- `terraform apply`