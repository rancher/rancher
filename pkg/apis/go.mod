module github.com/rancher/rancher/pkg/apis

go 1.14

replace k8s.io/client-go => k8s.io/client-go v0.22.3

require (
	github.com/pkg/errors v0.9.1
	github.com/rancher/aks-operator v1.0.4-rc1
	github.com/rancher/eks-operator v1.1.3-rc1
	github.com/rancher/fleet/pkg/apis v0.0.0-20210918015053-5a141a6b18f0
	github.com/rancher/gke-operator v1.1.3-rc1
	github.com/rancher/norman v0.0.0-20220107203912-4feb41eafabd
	github.com/rancher/rke v1.3.7
	github.com/rancher/wrangler v0.8.11-0.20220120160420-18c996a8e956
	github.com/sirupsen/logrus v1.8.1
	k8s.io/api v0.22.3
	k8s.io/apimachinery v0.22.3
	sigs.k8s.io/cluster-api v1.0.2
)
