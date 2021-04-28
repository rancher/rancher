module github.com/rancher/rancher/pkg/apis

go 1.14

replace (
	k8s.io/client-go => k8s.io/client-go v0.20.0
	sigs.k8s.io/cluster-api => github.com/rancher/cluster-api v0.3.11-0.20210219162658-745452a60720
)

require (
	github.com/pkg/errors v0.9.1
	github.com/rancher/eks-operator v1.0.6-rc1
	github.com/rancher/fleet/pkg/apis v0.0.0-20210422223946-04ef5f7e36c2
	github.com/rancher/gke-operator v1.0.1-rc9
	github.com/rancher/norman v0.0.0-20210423002317-8e6ffc77a819
	github.com/rancher/rke v1.3.0-rc1.0.20210421002614-3c0f9553436b
	github.com/rancher/wrangler v0.8.1-0.20210427175008-5cff59c4c16e
	github.com/sirupsen/logrus v1.6.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	sigs.k8s.io/cluster-api v0.3.11-0.20210219155426-bc756c4e7ed0
)
