module github.com/rancher/rancher/pkg/apis

go 1.14

replace (
	github.com/rancher/system-upgrade-controller/pkg/apis => github.com/ibuildthecloud/system-upgrade-controller/pkg/apis v0.0.0-20200823050544-4b08ab2b5a02
	k8s.io/client-go => k8s.io/client-go v0.20.0
	sigs.k8s.io/cluster-api => github.com/rancher/cluster-api v0.3.11-0.20210219162658-745452a60720
)

require (
	github.com/pkg/errors v0.9.1
	github.com/rancher/eks-operator v1.0.6-rc1
	github.com/rancher/gke-operator v1.0.1-rc6
	github.com/rancher/norman v0.0.0-20210225010917-c7fd1e24145b
	github.com/rancher/rke v1.3.0-rc1.0.20210218215557-dc70017c5941
	github.com/rancher/wrangler v0.8.1-0.20210419181004-70e0f567025d
	github.com/sirupsen/logrus v1.6.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	sigs.k8s.io/cluster-api v0.3.11-0.20210219155426-bc756c4e7ed0
)
