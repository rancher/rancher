package clusters

type ClusterType string

const (
	K3SClusterType  ClusterType = "k3s"
	RKE1ClusterType ClusterType = "rke1"
	RKE2ClusterType ClusterType = "rke2"
)

func (p ClusterType) String() string {
	return string(p)
}
