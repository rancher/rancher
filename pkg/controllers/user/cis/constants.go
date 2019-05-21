package cis

const (
	DefaultNamespaceForCis = "heptio-sonobuoy"
	// This is hardcoded in the sonobuoy tool

	DefaultSonobuoyPodName = "sonobuoy"

	RunCISScanAnnotation           = "field.cattle.io/runCisScan"
	CisHelmChartDeployedAnnotation = "field.cattle.io/cisSystemChartDeployed"
	SonobuoyCompletionAnnotation   = "field.cattle.io/sonobuoyDone"

	creatorIDAnno  = "field.cattle.io/creatorId"
	defaultAppName = "rancher-cis-benchmark"
)
