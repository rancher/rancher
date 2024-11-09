# Fleet in Airgap

## Getting Started
You should have a basic understanding of Corral before running this test. In order to run the entire airgap package set the package to `fleet/provisioning/airgap/...` Your GO suite should be set to blank. 
Running fleet in airgap, in terms of user config, is exactly what you would input for provisioning in airgap; a working corral config that points to your airgapped resources. The only differences are:
* this test only runs 1 k8s version instead of the entire list
* this test only uses rke2, as custer type doesn't matter for fleet at this time


In your config file, set the following:
```yaml
provisioningInput:
  # For the cni and kubernetes versions please leave blank unless you'd like to specifically test a version. Defaults to latest k8s version and calico. 
  cni:
  - calico
  rke2KubernetesVersion:
  - v1.xx.xx
registries:
  existingNoAuthRegistry: <value> # only set this if the registry was created beforehand just do `corral vars <corral> registry_fqdn` to get the registry hostname 
corralPackages:
  corralPackageImages:
    airgapCustomCluster: .../dist/aws-rancher-custom-cluster-true
    rancherHA: .../dist/aws-aws-registry-standalone-rke2-rancher-airgap-calico-true-2.15.1-1.11.0 # the name of the corral rancher is configurable with config entry above
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