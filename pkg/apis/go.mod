module github.com/rancher/rancher/pkg/apis

go 1.14

replace (
	github.com/rancher/system-upgrade-controller/pkg/apis => github.com/ibuildthecloud/system-upgrade-controller/pkg/apis v0.0.0-20200823050544-4b08ab2b5a02
	k8s.io/client-go => k8s.io/client-go v0.20.0
)

require (
	github.com/pkg/errors v0.9.1
	github.com/rancher/eks-operator v1.0.6-rc1
	github.com/rancher/norman v0.0.0-20210225010917-c7fd1e24145b
	github.com/rancher/rke v1.3.0-rc1.0.20210218215557-dc70017c5941
	github.com/rancher/wrangler v0.7.3-0.20210219161540-ef7fe9ce2443
	github.com/sirupsen/logrus v1.6.0
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
)
