# Terraform Chart Installations

Scripts to target an existing cluster for chart installations or to provision a cluster with charts installed

---
#### NOTES
- Leaving v2 chart versions blank ("") will install the latest version in scope for release catalog
- v1 target clusters require manual installation of v1 cluster-monitoring, v1 provisioned clusters will include v1 cluster-monitoring
- v1 Istio is deprecated after 2.5x, this chart can be installed using a supported version
- v2 Rancher Backups will always be installed into the local cluster, proper permissions are required `is_admin = 1` will indicate sufficent access to install v2 Rancher Backups, `is_admin = 0` will skip this chart installation
- v2 Neuvector requires a cluster type parameter, use `docker_install = true` for RKE1 and `k3s_install = true` for RKE2 target clusters. Provisioned clusters will be RKE1 docker installations via node template
- v2 Neuvector is not in scope for 2.5x (Partner Charts) `rancher_version_26_or_higher = 1` will install v2 Neuvector, while  `rancher_version_26_or_higher = 0` will skip this chart installation
___

## Setup
To run locally you will need to:
- create a file titled `terraform.tfvars` nested inside the folder of the script to be exected
- add desired values to corresponding environment variables

Sample `terraform.tfvars` for v1 charts
```
rancher_api_url = ""
rancher_token_key = ""
project_id = ""
cluster_provider = ""
cluster_name = ""
ami = ""
ami_user = ""
node_name = ""
vpc_id = ""
zone = "" 
aws_region = ""
root_size = ""
instance_type = ""
worker_count = 
k8s_version = ""
istio_version = ""
```

Sample `terraform.tfvars` for v2 charts
```
rancher_api_url = ""
rancher_token_key = ""
cluster_id = ""
project_id = ""
monitoring_version = ""
alerting_version = ""
kiali_version = ""
tracing_version = ""
istio_version = ""
logging_version = ""
cis_version = ""
gatekeeper_version = ""
longhorn_version = ""
backup_version = ""
neuvector_version = ""
rancher_version_26_or_higher = 1
is_admin = 0
docker_install = true
k3s_install = false
cluster_provider = ""
cluster_name = ""
ami = ""
ami_user = ""
node_name = ""
vpc_id = ""
zone = "" 
aws_region = ""
root_size = ""
instance_type = ""
worker_count = 
k8s_version = "" 
```
___
## To Execute
Run the following commands from inside the desired folder to initiate the script:
- `terraform init`
- `terraform apply`