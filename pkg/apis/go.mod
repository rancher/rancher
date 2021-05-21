module github.com/rancher/rancher/pkg/apis

go 1.14

replace k8s.io/client-go => k8s.io/client-go v0.21.0

require (
	github.com/pkg/errors v0.9.1
	github.com/rancher/aks-operator v1.0.1-rc4
	github.com/rancher/eks-operator v1.0.6-rc1
	github.com/rancher/fleet/pkg/apis v0.0.0-20210428191153-f414eab0e4de
	github.com/rancher/gke-operator v1.0.1
	github.com/rancher/norman v0.0.0-20210513204752-e48df26b54bd
	github.com/rancher/rke v1.3.0-rc1.0.20210521165717-bb0d38e303a5
	github.com/rancher/wrangler v0.8.1-0.20210521213200-39dd8bf93e9f
	github.com/sirupsen/logrus v1.7.0
	k8s.io/api v0.21.0
	k8s.io/apimachinery v0.21.0
	sigs.k8s.io/cluster-api v0.3.11-0.20210430180359-45b6080c2764
)
