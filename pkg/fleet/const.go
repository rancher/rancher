package fleet

const (
	ReleaseNamespace         = "cattle-fleet-system"
	ReleaseLocalNamespace    = "cattle-fleet-local-system"
	ReleaseClustersNamespace = "cattle-fleet-clusters-system"
	ReleaseLegacyNamespace   = "fleet-system"

	ChartName    = "fleet"
	CRDChartName = "fleet-crd"

	ClustersDefaultNamespace = "fleet-default"
	ClustersLocalNamespace   = "fleet-local"

	// KubeConfigReferenceableFromLabel names the one namespace whose Fleet
	// Clusters may use a kubeconfig secret from the secret's namespace.
	KubeConfigReferenceableFromLabel = "fleet.cattle.io/kubeconfig-referenceable-from"
)
