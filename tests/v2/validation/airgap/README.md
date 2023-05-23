# Standalone Configs - Corral

## Getting Started
You should have a basic understanding of Corral before running these tests. In order to run the entire airgap package set the package to `airgap/...` Your GO suite should be set to blank. 
In your config file, set the following:
```yaml
corralRancherHA:
  name: rancherha # this is the name of your aigap corral package if it hasn't been created beforehand
provisioningInput:
  cni:
  - calico
  # For the kubernetes versions please set it to what is appropriate for that release check
  k3sKubernetesVersion:
  - v1.23
  - v1.24
  - v1.25
  rke1KubernetesVersion:
  - v1.23
  - v1.24
  - v1.25
  rke2KubernetesVersion:
  - v1.25
  - v1.24
  - v1.23
registries:
  existingNoAuthRegistry: <value> # only set this if the registry was created beforehand just do `corral vars <corral> registry_fqdn` to get the registry hostname 
corralPackages:
  corralPackageImages:
    airgapCustomCluster: dist/aws-rancher-custom-cluster-true
    rancherHA: dist/aws-aws-registry-standalone-rke2-rancher-airgap-calico-true-2.15.1-1.8.0 # the name of the corral rancher is configurable with config entry above
    ...
  hasDebug: <bool, default=false>
  hasCleanup: <bool, default=true>
  hasSetCorralSSHKeys: <bool, default=false> # If you are creating the airgap rancher instance in the same test run, please set this to true so then the air gap cluster can communicate with the rancher instance. If the rancher instance was created beforehand this boolean is ignored.
corralConfigs:
  corralConfigUser: <string, default="jenkauto">
  corralConfigVars:
    <var1>: <string, "val1"> # for now only aws is supported, so use the appropriate aws vars
    bastion_ip: <val> # if the air gap rancher instance is created beforehand (not in the same job) set this to the registry public IP, otherwise it is automatically done in the job.
    corral_private_key: <val> # only set this if you have created the airgap rancher instance beforehand. By doing `corral vars <corral> corral_private_key`
    corral_public_key: <val> # only set this if you have created the airgap rancher instance beforehand. By doing `corral vars <corral> corral_private_key`
    registry_cert: <val> # cert for the registry
    registry_key: <val>  # key for the registry
    ...
  corralSSHPath: <string, optional, mostly for local testing>
```
Note: `corralConfigUser` will be the prefix for all resources created in your provider. 
From there, your `corralConfigVars` should contain the parameters necessary to run the test. You can see what variables need to be set by navigating to your corral package folder and checking the `manifest.yaml` variables.