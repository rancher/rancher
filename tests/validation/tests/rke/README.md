## To use locally:

### General Setup:
- To run locally (not as container), setup a python virtualenv and pip install -r requirements.txt
- create .ssh/ in main validations/ directory to store a ssh key or cert

#### For AWS:
- For AWS create a Key Pair cert and download it to validations/.ssh/
- export key filename with environment variable AWS_SSH_KEY_NAME


### Setup:
- create .ssh/ in main validations/ directory
- create a aws.pem cert for tests
- rke and kubectl need to be in your path
- pip install requirements.txt (recommend using virtualenv before this step)
- export following variables:

```
DEBUG=[true|false]  to see the output while running
OS_VERSION=[ubuntu-16.04|rhel-7.4]
DOCKER_VERSION=[latest|17.03|1.13.1|1.12.6]
CLOUD_PROVIDER=AWS
AWS_SSH_KEY_NAME=aws_cert.pem
AWS_ACCESS_KEY_ID=your AWS access key id
AWS_SECRET_ACCESS_KEY=your AWS secret jet
```

### Run:
    pytest -s rke_tests/


## To run locally in container:
- save ssh pem file from AWS in validations/.ssh
- export following variables:

```
OS_VERSION=[ubuntu-16.04|rhel-7.4]
DOCKER_VERSION=[latest|17.03|1.13.1|1.12.6]
KUBECTL_VERSION=
RKE_VERSION=
CLOUD_PROVIDER=AWS
AWS_SSH_KEY_NAME=aws_cert.pem
AWS_ACCESS_KEY_ID=
AWS_SECRET_ACCESS_KEY=
```
- run scripts/rke/configure.sh
- run scripts/rke/build.sh
- run docker run --rm -v ~/path/to/rancher-validation:/src/rancher-validation rancher-validation-tests -c pytest -s rke_tests/


## RKE Template Referenece

All other fields will be replaced if passed in to build_rke_template as key value pairs
For Nodes, the 'N' should be replace with index starting at 0

| TEMPLATE_FIELD_NAME      | Description                                   |
| ------------------------ |:---------------------------------------------:|
| ip_address_N             | Replaced with node N's IP Address             |
| dns_hostname_N           | Replaced with node N's FQDN                   |
| ssh_user_N               | Replaced with node N's ssh user               |
| ssh_key_N                | Replaced with node N's actual private key     |
| ssh_key_path_N           | Replaced with node N's path to ssh key        |
| internal_address_N       | Replaced with node N's private IP Address     |
| k8_rancher_image         | Used to determine k8s images used in services |
| network_plugin           | Used to determine rke network plugin          |
| master_ssh_key_path      | Replaced by path of the master ssh for tests  |
