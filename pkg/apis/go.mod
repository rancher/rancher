module github.com/rancher/rancher/pkg/apis

go 1.25.0

toolchain go1.25.4

replace (
	github.com/rancher/aks-operator => github.com/bhartigautam156/aks-operator v0.0.0-20260102071655-1ea0d45c928d
	github.com/rancher/ali-operator => github.com/bhartigautam156/ali-operator v0.0.0-20260105083130-a6a93511cfce
	github.com/rancher/eks-operator => github.com/bhartigautam156/eks-operator v0.0.0-20260102073546-c6cae43daf66
	github.com/rancher/fleet/pkg/apis => github.com/bhartigautam156/fleet/pkg/apis v0.0.0-20260105081654-10f845d48886
	github.com/rancher/gke-operator => github.com/bhartigautam156/gke-operator v0.0.0-20260102074454-9b11d173d5a7
	github.com/rancher/norman => github.com/bhartigautam156/norman v0.0.0-20251230120334-71f332fee56c
	github.com/rancher/rke => github.com/bhartigautam156/rke v0.0.0-20260105055322-f255d24b8c6a
	github.com/rancher/wrangler/v3 => github.com/bhartigautam156/wrangler/v3 v3.3.1-0.20251229122518-17d8c43b27b8
)

replace (
	k8s.io/api => k8s.io/api v0.35.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.35.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.35.0
	k8s.io/apiserver => k8s.io/apiserver v0.35.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.35.0
	k8s.io/client-go => k8s.io/client-go v0.35.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.35.0
	k8s.io/component-base => k8s.io/component-base v0.35.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.35.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.35.0
	k8s.io/cri-api => k8s.io/cri-api v0.35.0
	k8s.io/cri-client => k8s.io/cri-client v0.35.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.35.0
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.35.0
	k8s.io/endpointslice => k8s.io/endpointslice v0.35.0
	k8s.io/externaljwt => k8s.io/externaljwt v0.35.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.35.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.35.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.35.0
	k8s.io/kubectl => k8s.io/kubectl v0.35.0
	k8s.io/kubelet => k8s.io/kubelet v0.35.0
	k8s.io/kubernetes => k8s.io/kubernetes v1.35.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.35.0
	k8s.io/metrics => k8s.io/metrics v0.35.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.35.0
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.35.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.35.0
)

require (
	github.com/rancher/aks-operator v1.13.0-rc.4
	github.com/rancher/ali-operator v1.13.0-rc.2
	github.com/rancher/eks-operator v1.13.0-rc.4
	github.com/rancher/fleet/pkg/apis v0.15.0-alpha.4
	github.com/rancher/gke-operator v1.13.0-rc.3
	github.com/rancher/norman v0.8.1
	github.com/rancher/rke v1.8.0
	github.com/rancher/wrangler/v3 v3.3.1
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.11.1
	k8s.io/api v0.35.0
	k8s.io/apimachinery v0.35.0
	sigs.k8s.io/cluster-api v1.10.6
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/evanphx/json-patch v5.9.11+incompatible // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/rancher/lasso v0.2.5 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/oauth2 v0.33.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/term v0.37.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.35.0 // indirect
	k8s.io/apiserver v0.35.0 // indirect
	k8s.io/client-go v12.0.0+incompatible // indirect
	k8s.io/component-base v0.35.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250910181357-589584f1c912 // indirect
	k8s.io/kubernetes v1.34.1 // indirect
	k8s.io/utils v0.0.0-20251002143259-bc988d571ff4 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)
