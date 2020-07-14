module github.com/rancher/rancher/pkg/apis

go 1.13

replace k8s.io/client-go => k8s.io/client-go v0.18.0

require (
	github.com/pkg/errors v0.9.1
	github.com/rancher/norman v0.0.0-20200712070704-9bd3c4fd35e8
	github.com/rancher/rke v1.2.0-rc2.0.20200712062933-4c1d3db2b0c1
	github.com/sirupsen/logrus v1.6.0
	k8s.io/api v0.18.0
	k8s.io/apimachinery v0.18.0
)
