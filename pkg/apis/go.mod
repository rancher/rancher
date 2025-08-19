module github.com/rancher/rancher/pkg/apis

go 1.24.0

toolchain go1.24.6

replace (
	golang.org/x/net => golang.org/x/net v0.36.0 // CVE-2025-22870

	k8s.io/api => k8s.io/api v0.33.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.33.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.33.4
	k8s.io/apiserver => k8s.io/apiserver v0.33.4
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.33.4
	k8s.io/client-go => k8s.io/client-go v0.33.4
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.33.4
	k8s.io/component-base => k8s.io/component-base v0.33.4
	k8s.io/controller-manager => k8s.io/controller-manager v0.33.4
	k8s.io/cri-api => k8s.io/cri-api v0.33.4
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.33.4
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.33.4
	k8s.io/endpointslice => k8s.io/endpointslice v0.33.4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.33.4
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.33.4
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.33.4
	k8s.io/kubectl => k8s.io/kubectl v0.33.4
	k8s.io/kubelet => k8s.io/kubelet v0.33.4
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.30.14
	k8s.io/mount-utils => k8s.io/mount-utils v0.33.4
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.33.4
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.33.4
	sigs.k8s.io/structured-merge-diff/v4 => sigs.k8s.io/structured-merge-diff/v4 v4.4.1
)

require (
	github.com/rancher/aks-operator v1.9.10
	github.com/rancher/eks-operator v1.9.10
	github.com/rancher/fleet/pkg/apis v0.10.13
	github.com/rancher/gke-operator v1.9.10
	github.com/rancher/norman v0.3.1
	github.com/rancher/rke v1.6.12-rc.1
	github.com/rancher/wrangler/v3 v3.1.0
	github.com/sirupsen/logrus v1.9.3
	k8s.io/api v0.33.4
	k8s.io/apimachinery v0.33.4
	sigs.k8s.io/cluster-api v1.7.3
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gnostic-models v0.6.9 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rancher/lasso v0.2.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/otel v1.33.0 // indirect
	go.opentelemetry.io/otel/trace v1.33.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/term v0.32.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.31.1 // indirect
	k8s.io/apiserver v0.33.4 // indirect
	k8s.io/client-go v12.0.0+incompatible // indirect
	k8s.io/component-base v0.33.4 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250318190949-c8a335a9a2ff // indirect
	k8s.io/kubernetes v1.30.10 // indirect
	k8s.io/utils v0.0.0-20250502105355-0f33e8f1c979 // indirect
	sigs.k8s.io/cli-utils v0.37.2 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.6.0 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)
