package rke

import "github.com/rancher/types/apis/management.cattle.io/v3"

func loadRancherDefaultK8sVersions() map[string]string {
	return map[string]string{
		"2.3": "v1.14.3-rancher1-1",
		// rancher will use default if its version is absent
		"default": "v1.14.3-rancher1-1",
	}
}

func loadRKEDefaultK8sVersions() map[string]string {
	return map[string]string{
		"0.2.3": "v1.14.3-rancher1-1",
		// rke will use default if its version is absent
		"default": "v1.14.3-rancher1-1",
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

func loadK8sVersionInfoPrev() map[string]v3.K8sVersionInfo {
	return map[string]v3.K8sVersionInfo{
		"v1.8.10-rancher1-1": {
			MaxRancherVersion:       "2.2",
			DeprecateRancherVersion: "2.2",
			MaxRKEVersion:           "0.2.2",
			DeprecateRKEVersion:     "0.2.2",
		},
		"v1.8.11-rancher1": {
			MaxRancherVersion:       "2.2",
			DeprecateRancherVersion: "2.2",
			MaxRKEVersion:           "0.2.2",
			DeprecateRKEVersion:     "0.2.2",
		},
		"v1.8.11-rancher2-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.9.5-rancher1-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.9.7-rancher1": {
			MaxRancherVersion:       "2.2",
			DeprecateRancherVersion: "2.2",
			MaxRKEVersion:           "0.2.2",
			DeprecateRKEVersion:     "0.2.2",
		},
		"v1.9.7-rancher2-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.9.7-rancher2-2": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.10.0-rancher1-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.10.1-rancher1": {
			MaxRancherVersion:       "2.2",
			DeprecateRancherVersion: "2.2",
			MaxRKEVersion:           "0.2.2",
			DeprecateRKEVersion:     "0.2.2",
		},
		"v1.10.1-rancher2-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.10.3-rancher2-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.10.5-rancher1-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.10.5-rancher1-2": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.10.11-rancher1-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.10.12-rancher1-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.11.1-rancher1-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.11.2-rancher1-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.11.2-rancher1-2": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.11.3-rancher1-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.11.5-rancher1-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.11.8-rancher1-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.11.6-rancher1-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.11.9-rancher1-1": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.11.9-rancher1-2": {
			MaxRancherVersion: "2.2",
			MaxRKEVersion:     "0.2.2",
		},
		"v1.12.0-rancher1-1": {},
		"v1.12.1-rancher1-1": {},
		"v1.12.3-rancher1-1": {},
		"v1.12.4-rancher1-1": {},
		"v1.12.5-rancher1-1": {},
		"v1.12.5-rancher1-2": {},
		"v1.12.6-rancher1-1": {},
		"v1.12.6-rancher1-2": {},
		"v1.12.7-rancher1-1": {},
		"v1.12.7-rancher1-2": {},
		"v1.12.7-rancher1-3": {},
		"v1.12.9-rancher1-1": {},
		"v1.13.1-rancher1-1": {},
		"v1.13.1-rancher1-2": {},
		"v1.13.4-rancher1-1": {},
		"v1.13.4-rancher1-2": {},
		"v1.13.5-rancher1-2": {},
		"v1.13.5-rancher1-1": {},
		"v1.13.5-rancher1-3": {},
		"v1.13.7-rancher1-1": {},
		"v1.14.1-rancher1-1": {},
		"v1.14.1-rancher1-2": {},
		"v1.14.3-rancher1-1": {},
		"v1.15.0-rancher1-1": {},
	}
}
