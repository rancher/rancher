module github.com/rancher/rancher/pkg/apis

go 1.26.0

toolchain go1.26.6

replace (
	github.com/rancher/aks-operator => github.com/Abhishek-Valaboju/aks-operator v1.14.0-rc.1.0.20260902074845-42b70cc84c64
	github.com/rancher/ali-operator => github.com/Abhishek-Valaboju/ali-operator v1.14.0-rc.1.0.20260902103845-8111ed396886
	github.com/rancher/eks-operator => github.com/Abhishek-Valaboju/eks-operator v1.15.0-rc.1.0.20260902080038-96744e1194d9
	github.com/rancher/gke-operator => github.com/Abhishek-Valaboju/gke-operator v1.15.0-rc.1.0.20260902111650-ef3130554dcb
	github.com/rancher/lasso => github.com/Abhishek-Valaboju/lasso v0.2.9-0.20260828055838-34eaedfec2e7
	github.com/rancher/norman => github.com/Abhishek-Valaboju/norman v0.10.1-0.20260828094032-75fa12a9114f
	github.com/rancher/wrangler/v3 => github.com/Abhishek-Valaboju/wrangler/v3 v3.5.1-rc.1.0.20260828064927-bed3da9cc545
)

replace (
	github.com/rancher/rancher/pkg/plan => ../plan
	k8s.io/api => k8s.io/api v0.37.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.37.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.37.0
	k8s.io/apiserver => k8s.io/apiserver v0.37.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.37.0
	k8s.io/client-go => k8s.io/client-go v0.37.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.37.0
	k8s.io/component-base => k8s.io/component-base v0.37.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.37.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.37.0
	k8s.io/cri-api => k8s.io/cri-api v0.37.0
	k8s.io/cri-client => k8s.io/cri-client v0.37.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.37.0
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.37.0
	k8s.io/endpointslice => k8s.io/endpointslice v0.37.0
	k8s.io/externaljwt => k8s.io/externaljwt v0.37.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.37.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.37.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.37.0
	k8s.io/kubectl => k8s.io/kubectl v0.37.0
	k8s.io/kubelet => k8s.io/kubelet v0.37.0
	k8s.io/kubernetes => k8s.io/kubernetes v1.37.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.37.0
	k8s.io/metrics => k8s.io/metrics v0.37.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.37.0
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.37.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.37.0
)

require (
	github.com/rancher/aks-operator v1.15.0
	github.com/rancher/ali-operator v1.15.0-rc.3
	github.com/rancher/eks-operator v1.15.0
	github.com/rancher/fleet/pkg/apis v0.17.0-alpha.1
	github.com/rancher/gke-operator v1.15.0
	github.com/rancher/norman v0.10.0
	github.com/rancher/rancher/pkg/plan v0.0.0-20260428222332-2696373f4152
	github.com/rancher/wrangler/v3 v3.7.1
	github.com/sirupsen/logrus v1.10.2
	github.com/stretchr/testify v1.12.1
	k8s.io/api v0.37.0
	k8s.io/apimachinery v0.37.0
	sigs.k8s.io/cluster-api/api v1.14.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/evanphx/json-patch v5.9.11+incompatible // indirect
	github.com/fxamacker/cbor/v2 v2.9.1 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-openapi/jsonpointer v1.0.0 // indirect
	github.com/go-openapi/jsonreference v1.0.0 // indirect
	github.com/go-openapi/swag v0.27.1 // indirect
	github.com/go-openapi/swag/cmdutils v0.27.1 // indirect
	github.com/go-openapi/swag/conv v0.27.1 // indirect
	github.com/go-openapi/swag/fileutils v0.27.1 // indirect
	github.com/go-openapi/swag/jsonutils v0.27.1 // indirect
	github.com/go-openapi/swag/loading v0.27.1 // indirect
	github.com/go-openapi/swag/mangling v0.27.1 // indirect
	github.com/go-openapi/swag/netutils v0.27.1 // indirect
	github.com/go-openapi/swag/pools v0.27.1 // indirect
	github.com/go-openapi/swag/stringutils v0.27.1 // indirect
	github.com/go-openapi/swag/typeutils v0.27.1 // indirect
	github.com/go-openapi/swag/yamlutils v0.27.1 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.24.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.0 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	github.com/rancher/lasso v0.2.9 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/apiextensions-apiserver v0.37.0 // indirect
	k8s.io/client-go v12.0.0+incompatible // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260721132016-d427ff9ee9ad // indirect
	k8s.io/utils v0.0.0-20260626114624-be93311217bd // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.4.2 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)
