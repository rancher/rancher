package rke

import v3 "github.com/rancher/types/apis/management.cattle.io/v3"

func loadRancherDefaultK8sVersions() map[string]string {
	/*
		Just mention the major version, the latest minor version will be
		automatically picked based on Rancher's max/min version information.
	*/
	return map[string]string{
		"2.3.0": "v1.15.x",
		"2.3.1": "v1.15.x",
		"2.3":   "v1.15.x",
		// rancher will use default if its version is absent
		"default": "v1.15.x",
	}
}

func loadRKEDefaultK8sVersions() map[string]string {
	return map[string]string{
		"0.3": "v1.15.5-rancher2-2",
		// rke will use default if its version is absent
		"default": "v1.15.5-rancher2-2",
	}
}

/*
MaxRancherVersion: Last Rancher version having this k8s in k8sVersionsCurrent
DeprecateRancherVersion: No create/update allowed for RKE >= DeprecateRancherVersion
MaxRKEVersion: Last RKE version having this k8s in k8sVersionsCurrent
DeprecateRKEVersion: No create/update allowed for RKE >= DeprecateRKEVersion
*/

func loadK8sVersionInfo() map[string]v3.K8sVersionInfo {
	return map[string]v3.K8sVersionInfo{
		"v1.8": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.9": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.10": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.11": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.12": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.13": {
			MaxRancherVersion: "2.3.1",
			MaxRKEVersion:     "0.3.1",
		},
		"v1.15.5-rancher1-1": {
			MaxRancherVersion: "2.2.9",
			MaxRKEVersion:     "0.2.8",
		},
		"v1.8.10-rancher1-1": {
			DeprecateRKEVersion:     "0.2.2",
			DeprecateRancherVersion: "2.2",
		},
		"v1.8.11-rancher1": {
			DeprecateRKEVersion:     "0.2.2",
			DeprecateRancherVersion: "2.2",
		},
		"v1.9.7-rancher1": {
			DeprecateRKEVersion:     "0.2.2",
			DeprecateRancherVersion: "2.2",
		},
		"v1.10.1-rancher1": {
			DeprecateRKEVersion:     "0.2.2",
			DeprecateRancherVersion: "2.2",
		},
	}
}
