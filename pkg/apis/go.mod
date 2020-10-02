module github.com/rancher/rancher/pkg/apis

go 1.14

replace (
	github.com/rancher/system-upgrade-controller/pkg/apis => github.com/ibuildthecloud/system-upgrade-controller/pkg/apis v0.0.0-20200823050544-4b08ab2b5a02
	k8s.io/client-go => k8s.io/client-go v0.18.8
)

require (
	github.com/pkg/errors v0.9.1
	github.com/rancher/eks-operator v1.0.1
	github.com/rancher/norman v0.0.0-20200930000340-693d65aaffe3
	github.com/rancher/rke v1.2.0-rc16
	github.com/rancher/wrangler v0.7.2
	github.com/sirupsen/logrus v1.6.0
	golang.org/x/net v0.0.0-20200625001655-4c5254603344 // indirect
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
)
