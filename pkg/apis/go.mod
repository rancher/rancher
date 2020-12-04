module github.com/rancher/rancher/pkg/apis

go 1.14

replace (
	github.com/rancher/system-upgrade-controller/pkg/apis => github.com/ibuildthecloud/system-upgrade-controller/pkg/apis v0.0.0-20200823050544-4b08ab2b5a02
	k8s.io/client-go => k8s.io/client-go v0.18.8
)

require (
	github.com/pkg/errors v0.9.1
	github.com/rancher/eks-operator v1.0.4
	github.com/rancher/norman v0.0.0-20200908202416-992a35eef40f
	github.com/rancher/rke v1.2.0-rc9.0.20201204145714-816d4cd130a9
	github.com/rancher/wrangler v0.7.3-0.20201020003736-e86bc912dfac
	github.com/sirupsen/logrus v1.6.0
	golang.org/x/net v0.0.0-20200625001655-4c5254603344 // indirect
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
)
