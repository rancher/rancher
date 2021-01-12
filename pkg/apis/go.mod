module github.com/rancher/rancher/pkg/apis

go 1.14

replace (
	github.com/rancher/system-upgrade-controller/pkg/apis => github.com/ibuildthecloud/system-upgrade-controller/pkg/apis v0.0.0-20200823050544-4b08ab2b5a02
	k8s.io/client-go => k8s.io/client-go v0.20.0
)

require (
	github.com/pkg/errors v0.9.1
	github.com/rancher/eks-operator v1.0.4
	github.com/rancher/norman v0.0.0-20200930000340-693d65aaffe3
	github.com/rancher/rke v1.2.0-rc9.0.20210106190313-91aed199f04c
	github.com/rancher/wrangler v0.7.3-0.20201028210318-d73835950c29
	github.com/sirupsen/logrus v1.6.0
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
)
