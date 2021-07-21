module github.com/rancher/rancher/tests/go-validation

go 1.16

replace (
	git.apache.org/thrift.git => github.com/apache/thrift v0.12.0

	github.com/docker/distribution => github.com/docker/distribution v2.7.1+incompatible // oras dep requires a replace is set
	github.com/docker/docker => github.com/docker/docker v20.10.6+incompatible // oras dep requires a replace is set

	github.com/knative/pkg => github.com/rancher/pkg v0.0.0-20190514055449-b30ab9de040e
	github.com/matryer/moq => github.com/rancher/moq v0.0.0-20200712062324-13d1f37d2d77

	github.com/opencontainers/runc => github.com/opencontainers/runc v1.0.0-rc10
	github.com/rancher/rancher => ../../

	github.com/rancher/rancher/pkg/apis => ../../pkg/apis
	github.com/rancher/rancher/pkg/client => ../../pkg/client

	google.golang.org/grpc => google.golang.org/grpc v1.29.1 // etcd depends on google.golang.org/grpc/naming which was removed in grpc v1.30.0
	helm.sh/helm/v3 => github.com/rancher/helm/v3 v3.5.4-rancher.1

	k8s.io/api => k8s.io/api v0.21.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.0
	k8s.io/apiserver => k8s.io/apiserver v0.21.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.21.0
	k8s.io/client-go => github.com/rancher/client-go v0.21.0-rancher.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.21.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.21.0
	k8s.io/code-generator => k8s.io/code-generator v0.21.0
	k8s.io/component-base => k8s.io/component-base v0.21.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.21.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.21.0
	k8s.io/cri-api => k8s.io/cri-api v0.21.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.21.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.21.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.21.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.21.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.21.0
	k8s.io/kubectl => k8s.io/kubectl v0.21.0
	k8s.io/kubelet => k8s.io/kubelet v0.21.0
	k8s.io/kubernetes => k8s.io/kubernetes v1.21.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.21.0
	k8s.io/metrics => k8s.io/metrics v0.21.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.21.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.21.0
	sigs.k8s.io/cluster-api => github.com/rancher/cluster-api v0.3.11-0.20210514043303-8726f6e84d41
)

require (
	github.com/rancher/rancher v0.0.0-00010101000000-000000000000
	github.com/rancher/rancher/pkg/apis v0.0.0
	github.com/rancher/wrangler v0.8.1-0.20210618171953-ab479ee75244
	github.com/stretchr/testify v1.7.0
	k8s.io/api v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v12.0.0+incompatible
)
