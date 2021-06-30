module github.com/rancher/rancher/pkg/apis

go 1.14

replace k8s.io/client-go => k8s.io/client-go v0.21.0

require (
	github.com/pkg/errors v0.9.1
	github.com/rancher/aks-operator v1.0.1-rc14
	github.com/rancher/eks-operator v1.1.1-rc3
	github.com/rancher/fleet/pkg/apis v0.0.0-20210608014113-99e848822739
	github.com/rancher/gke-operator v1.1.1-rc5
	github.com/rancher/norman v0.0.0-20210608202517-59b3523c3133
	github.com/rancher/rke v1.3.0-rc8
	github.com/rancher/system-upgrade-controller/pkg/apis v0.0.0-20210424054953-634d28b7def3
	github.com/rancher/wrangler v0.8.1-0.20210618171953-ab479ee75244
	github.com/sirupsen/logrus v1.7.0
	k8s.io/api v0.21.0
	k8s.io/apimachinery v0.21.0
	sigs.k8s.io/cluster-api v0.3.11-0.20210430180359-45b6080c2764
)
