module github.com/rancher/types

go 1.12

replace (
	github.com/knative/pkg => github.com/rancher/pkg v0.0.0-20190514055449-b30ab9de040e
	github.com/matryer/moq => github.com/rancher/moq v0.0.0-20190404221404-ee5226d43009
)

require (
	github.com/coreos/prometheus-operator v0.25.0
	github.com/knative/pkg v0.0.0-20190817231834-12ee58e32cc8
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/rancher/norman v0.0.0-20190819172543-9c5479f6e5ca
	github.com/sirupsen/logrus v1.4.2
	k8s.io/api v0.0.0-20190805182251-6c9aa3caf3d6
	k8s.io/apiextensions-apiserver v0.0.0-20190805184801-2defa3e98ef1
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190805182715-88a2adca7e76+incompatible
	k8s.io/gengo v0.0.0-20190327210449-e17681d19d3a
	k8s.io/kube-aggregator v0.0.0-20190805183716-8439689952da
)
