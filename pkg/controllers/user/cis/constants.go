package cis

const (
	DefaultNamespaceForCis = "heptio-sonobuoy"
	// This is hardcoded in the sonobuoy tool

	DefaultSonobuoyPodName = "sonobuoy"

	RunCISScanAnnotation         = "field.cattle.io/runCisScan"
	SonobuoyCompletionAnnotation = "field.cattle.io/sonobuoyDone"
	CisHelmChartOwner            = "field.cattle.io/clusterScanOwner"

	creatorIDAnno = "field.cattle.io/creatorId"
)
