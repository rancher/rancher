# Release HA Upgrade Validation

Release HA upgrade is an automation task for the Rancher release testing process. It heavily relies on the framework's configuration file generation. Its initial configuration contains two main components:

1.  HA Configuration: Parent field that has the HA options, such as rancher version to deploy, or rancher version to upgrade.
2.  Clusters Configuration: Parent field that has the downstream options. This part maintains the match of the matrix for the configuration file generation. This matrix enables the test to generate individual configuration files per cluster type, providers, image, and SSH user to the given images.

You can check the whole configuration, [here](../../../../../framework/extensions/pipelineutil/releaseupgrade.go).

### HA Configuration
  
```yaml
ha:
  host: "" # String, needs to be URL
  chartVersion: "" # String without any prefix, no v in front of version
  chartVersionToUpgrade: "" # String without any prefix, no v in front of version
  rkeVersion: "" # String with prefix v 
  imageTag: "" # String with prefix v 
  imageTagToUpgrade: "" # String with prefix v 
  helmRepo: "" # String
  certOption: "" # String
  insecure: false # String
  cleanup: false  # String, default is true
```

### Clusters Configuration

Below an example test configuration with 1 Node Provider, 1 Custom; RKE1, K3s, and RKE2 types. And 3 Hosted Clusters:

```yaml
clusters:
  rke1:
    nodeProvider:
      - provider: "" # Name of the provider
        kubernetesVersion: "vMYVERSION" # String with prefix v, as UI shows
        kubernetesVersionToUpgrade: "vMYVERSION" # String with prefix v, as UI shows
        image: "" # String
        sshUser: "" # String
        cni: ["calico"] # Slice of strings, options can be found in provisioning configuration
        enabledFeatures:
          chart: false # Boolean, pre/post upgrade checks, default is false
          ingress: false # Boolean, pre/post upgrade checks, default is false
    # This is a slice of structs, elements are expandable
    custom:
      - provider: "" # Name of the provider
        kubernetesVersion: "vMYVERSION" # String with prefix v, as UI shows
        kubernetesVersionToUpgrade: "vMYVERSION" # String with prefix v, as UI shows
        image: "" # String
        sshUser: "" # String
        cni: ["calico"] # Slice of strings, options can be found in provisioning configuration
        enabledFeatures:
          chart: false # Boolean, pre/post upgrade checks, default is false
          ingress: false # Boolean, pre/post upgrade checks, default is false
    # This is a slice of structs, elements are expandable
  rke2:
    nodeProvider:
      - provider: "" # Name of the provider
        kubernetesVersion: "vMYVERSION" # String with prefix v, as UI shows
        kubernetesVersionToUpgrade: "vMYVERSION" # String with prefix v, as UI shows
        image: "" # String
        sshUser: "" # String
        cni: ["calico"] # Slice of strings, options can be found in provisioning configuration
        enabledFeatures:
          chart: false # Boolean, pre/post upgrade checks, default is false
          ingress: false # Boolean, pre/post upgrade checks, default is false
      - provider: "aws"
        kubernetesVersion: "vMYVERSION" # String with prefix v, as UI shows
        kubernetesVersionToUpgrade: "vMYVERSION" # String with prefix v, as UI shows
        image: "" # String
        sshUser: "ubuntu"
        cni: ["calico"]
        enabledFeatures:
          chart: false # Boolean, pre/post upgrade checks, default is false
          ingress: false # Boolean, pre/post upgrade checks, default is false
    # This is a slice of structs, elements are expandable
    custom:
      - provider: "" # Name of the provider
        kubernetesVersion: "vMYVERSION" # String with prefix v, as UI shows
        kubernetesVersionToUpgrade: "vMYVERSION" # String with prefix v, as UI shows
        image: "" # String
        sshUser: "" # String
        cni: ["calico"] # Slice of strings, options can be found in provisioning configuration
        enabledFeatures:
          chart: false # Boolean, pre/post upgrade checks, default is false
          ingress: false # Boolean, pre/post upgrade checks, default is false
    # This is a slice of structs, elements are expandable
  k3s:
    nodeProvider:
      - provider: "" # Name of the provider
        kubernetesVersion: "vMYVERSION" # String with prefix v, as UI shows
        kubernetesVersionToUpgrade: "vMYVERSION" # String with prefix v, as UI shows
        image: "" # String
        sshUser: "" # String
        cni: ["calico"] # Slice of strings, options can be found in provisioning configuration
        enabledFeatures:
          chart: false # Boolean, pre/post upgrade checks, default is false
          ingress: false # Boolean, pre/post upgrade checks, default is false
    # This is a slice of structs, elements are expandable
      - provider: "" # Name of the provider
        kubernetesVersion: "vMYVERSION" # String with prefix v, as UI shows
        kubernetesVersionToUpgrade: "vMYVERSION" # String with prefix v, as UI shows
        image: "" # String
        sshUser: "" # String
        cni: ["calico"] # Slice of strings, options can be found in provisioning configuration
        enabledFeatures:
          chart: false # Boolean, pre/post upgrade checks, default is false
          ingress: false # Boolean, pre/post upgrade checks, default is false
    # This is a slice of structs, elements are expandable
    custom:
      - provider: "" # Name of the provider
        kubernetesVersion: "vMYVERSION" # String with prefix v, as UI shows
        kubernetesVersionToUpgrade: "vMYVERSION" # String with prefix v, as UI shows
        image: "" # String
        sshUser: "" # String
        cni: ["calico"] # Slice of strings, options can be found in provisioning configuration
        enabledFeatures:
          chart: false # Boolean, pre/post upgrade checks, default is false
          ingress: false # Boolean, pre/post upgrade checks, default is false
    # This is a slice of structs, elements are expandable
  hosted:
    - provider: "" # Name of the provider
      kubernetesVersion: "MYVERSION" # String without prefix v, as UI shows
      kubernetesVersionToUpgrade: "MYVERSION" # String without prefix v, as UI shows
      enabledFeatures:
         chart: false # Boolean, pre/post upgrade checks, default is false
         ingress: false # Boolean, pre/post upgrade checks, default is false
    - provider: "" # Name of the provider
      kubernetesVersion: "MYVERSION" # String without prefix v, as UI shows
      kubernetesVersionToUpgrade: "MYVERSION" # String without prefix v, as UI shows
      enabledFeatures:
         chart: false # Boolean, pre/post upgrade checks, default is false
         ingress: false # Boolean, pre/post upgrade checks, default is false
    - provider: "" # Name of the provider
      kubernetesVersion: "MYVERSION" # String without prefix v, as UI shows
      kubernetesVersionToUpgrade: "MYVERSION" # String without prefix v, as UI shows
      enabledFeatures:
         chart: false # Boolean, pre/post upgrade checks, default is false
         ingress: false # Boolean, pre/post upgrade checks, default is false
    # This is a slice of structs, elements are expandable
```

### Other Required Configuration

Depending on the cluster types you want to test, the following configurations are required:

1. [RKE1 Provisioning](../../../provisioning/rke1/README.md)
2. [RKE2 Provisioning](../../../provisioning/rke2/README.md)
3. [K3s Provisioning](../../../provisioning/k3s/README.md)
4. [Hosted Provider Provisioning](../../../provisioning/hosted/README.md)

*The fields that are declared in the [clusters](#clusters-configuration) input above, are going to overwrite the values that are given as provisioning configuration.*

In addition to this configuration generation, other dependent factors are:

1.  An environment flag called **UpdateClusterName** updates the test configuration's YAML cluster name field, in all provisioning tests, after the cluster creation steps.
2.  Provider names and different user types, particularly for all provisioning tests, are hardcoded in their corresponding test case regexp while generating the configuration files.