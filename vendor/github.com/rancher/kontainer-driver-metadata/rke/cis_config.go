package rke

import (
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	reasonNoConfigFile = `Cluster provisioned by RKE doesn't require or maintain a configuration file for kube-apiserver.
All configuration is passed in as arguments at container run time.`
	reasonForPSP       = `Enabling Pod Security Policy can cause issues with many helm chart installations`
	reasonForNetPol    = `Enabling Network Policies can cause lot of unintended network traffic disruptions`
	reasonForDefaultNS = `A default namespace provides a flexible workspace to try out various deployments`
)

var rkeCIS15NotApplicableChecks = map[string]string{
	"1.1.1": reasonNoConfigFile,
	"1.1.2": reasonNoConfigFile,
	"1.1.3": reasonNoConfigFile,
	"1.1.4": reasonNoConfigFile,
	"1.1.5": reasonNoConfigFile,
	"1.1.6": reasonNoConfigFile,
	"1.1.7": reasonNoConfigFile,
	"1.1.8": reasonNoConfigFile,
}

var rkeCIS15SkippedChecks = map[string]string{
	"5.2.2": reasonForPSP,
	"5.2.3": reasonForPSP,
	"5.2.4": reasonForPSP,
	"5.2.5": reasonForPSP,
	"5.3.2": reasonForNetPol,
	"5.6.4": reasonForDefaultNS,
}

func loadCisConfigParams() map[string]v3.CisConfigParams {
	return map[string]v3.CisConfigParams{
		"default": {
			BenchmarkVersion: "rke-cis-1.5",
		},
		"v1.15": {
			BenchmarkVersion: "rke-cis-1.5",
		},
		"v1.16": {
			BenchmarkVersion: "rke-cis-1.5",
		},
		"v1.17": {
			BenchmarkVersion: "rke-cis-1.5",
		},
		"v1.18": {
			BenchmarkVersion: "rke-cis-1.5",
		},
	}
}

func loadCisBenchmarkVersionInfo() map[string]v3.CisBenchmarkVersionInfo {
	return map[string]v3.CisBenchmarkVersionInfo{
		"rke-cis-1.4": {
			Managed:              true,
			MinKubernetesVersion: "1.13",
			SkippedChecks:        map[string]string{},
			NotApplicableChecks:  map[string]string{},
		},
		"cis-1.4": {
			Managed:              false,
			MinKubernetesVersion: "1.13",
		},
		"rke-cis-1.5": {
			Managed:              true,
			MinKubernetesVersion: "1.15",
			SkippedChecks:        rkeCIS15SkippedChecks,
			NotApplicableChecks:  rkeCIS15NotApplicableChecks,
		},
		"cis-1.5": {
			Managed:              false,
			MinKubernetesVersion: "1.15",
		},
	}
}
