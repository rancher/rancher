# kontainer-driver-metadata

#### Run ####
  * `./kontainer-driver-metadata --write-data=true` loads data under /rke into data/data.json
  * vendor changes into RKE
  * rancher >=2.3 will listen on changes in data.json and load automatically

#### How to add to data ####
- ***K8sVersionRKESystemImages*** map[string]v3.RKESystemImages
    - introduce new k8s version along with required system images, exact version required (eg 1.13.5-rancher1-1)

- ***K8sVersionServiceOptions*** map[string]v3.KubernetesServicesOptions
    - mostly done on major k8s version basis, add minor specific version if needed
    - preference order while reading, minor_version > major_version (eg 1.13.5-rancher1-2 > 1.13)

- ***K8sVersionToRancherVersions / K8sVersionToRKEVersions*** minRKE, maxRKE, minRancher, maxRancher)
    - exact version required (eg 1.13.5-rancher1-1)
    - min and max versions are to limit, nil or "" will be considered as "allowed" for all rancher and rke versions
    - add minRKE and maxRKE
        From RKE
            - Only versions that meet the requirements will be loaded for system images
            - K8sVersionsCurrent - max(minor_versions) for each major_version
    - add minRancher and maxRancher
        From Rancher
            - Only versions that meet minRancher will be loaded for system images (not max, to handle upgrades)
            - K8sVersionsCurrent - max(minor_versions) for each major_version if maxVersion < rancherVersion

- ***DefaultK8sVersions*** (RancherDefaultK8sVersions, RKEDefaultK8sVersions) map[string]string -- need to combine these in one --
    - for every new rancher and rke version, add default k8s version
    - mostly will be required to update when introducing a new k8s version
    - if not present, rancher and rke will fallback on "default" key

- ***K8sVersionedTemplates*** map[string]map[string]string
    - map[addon_name]map[version_]template
    - addon_names currently are : calico, canal, flannel, weave, coreDNS, kubeDNS, metricsServer, nginxIngress
    - version_num are mostly major version (eg v1.13) or "default"
        - can introduce minor version if required (eg v1.13.5)
        - preference order while reading, minor_version > major_version > "default"

- Follow same for K8sVersionWindowsSystemImages, K8sVersionWindowsServiceOptions from 1 and 2

- For detailed explanations,
	- [rke_example](examples/rke_example.yml)
	- [rancher_example](examples/rancher_example.yml)

#### Structure ####
```
metadata
- data
  - data.json
- rke
  - templates
    - calico
    - coredns
    - etc
  - k8s_defaults
  - k8s_rancher_rke_versions
  - k8s_system_images
  - k8s_service_options
  - k8s_windows_defaults
```
