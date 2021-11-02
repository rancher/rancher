module github.com/rancher/rancher/pkg/apis

go 1.14

replace k8s.io/client-go => k8s.io/client-go v0.21.2

require (
	github.com/pkg/errors v0.9.1
	github.com/rancher/aks-operator v1.0.2
	github.com/rancher/eks-operator v1.1.1
	github.com/rancher/fleet/pkg/apis v0.0.0-20210918015053-5a141a6b18f0
	github.com/rancher/gke-operator v1.1.1
	github.com/rancher/norman v0.0.0-20210709145327-afd06f533ca3
	github.com/rancher/rke v1.3.1
	github.com/rancher/wrangler v0.8.7
	github.com/sirupsen/logrus v1.7.0
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	sigs.k8s.io/cluster-api v0.3.11-0.20210430180359-45b6080c2764
)
