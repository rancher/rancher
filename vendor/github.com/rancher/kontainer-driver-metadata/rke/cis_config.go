package rke

import (
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

func loadCisConfigParams() map[string]v3.CisConfigParams {
	return map[string]v3.CisConfigParams{
		"default": {
			BenchmarkVersion: "rke-cis-1.4",
		},
		"v1.15": {
			BenchmarkVersion: "rke-cis-1.4",
		},
	}
}

func loadCisBenchmarkVersionInfo() map[string]v3.CisBenchmarkVersionInfo {
	return map[string]v3.CisBenchmarkVersionInfo{
		"rke-cis-1.4": {
			MinKubernetesVersion: "1.13",
		},
		"cis-1.4": {
			MinKubernetesVersion: "1.13",
		},
		"rke-cis-1.5": {
			MinKubernetesVersion: "1.15",
		},
		"cis-1.5": {
			MinKubernetesVersion: "1.15",
		},
	}
}
