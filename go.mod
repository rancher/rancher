module github.com/rancher/rancher

go 1.24.0

toolchain go1.24.5

replace (
	github.com/crewjam/saml => github.com/rancher/saml v0.4.14-rancher3
	github.com/docker/distribution => github.com/docker/distribution v2.8.2+incompatible // rancher-machine requires a replace is set
	github.com/google/gnostic-models => github.com/google/gnostic-models v0.6.9
	github.com/knative/pkg => github.com/rancher/pkg v0.0.0-20190514055449-b30ab9de040e
	github.com/matryer/moq => github.com/rancher/moq v0.0.0-20200712062324-13d1f37d2d77
	github.com/rancher/rancher/pkg/apis => ./pkg/apis
	github.com/rancher/rancher/pkg/client => ./pkg/client
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc => go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.58.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp => go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.58.0
	go.opentelemetry.io/otel => go.opentelemetry.io/otel v1.33.0
	go.opentelemetry.io/otel/metric => go.opentelemetry.io/otel/metric v1.33.0
	go.opentelemetry.io/otel/sdk => go.opentelemetry.io/otel/sdk v1.33.0
	go.opentelemetry.io/otel/trace => go.opentelemetry.io/otel/trace v1.33.0
	go.opentelemetry.io/proto/otlp => go.opentelemetry.io/proto/otlp v1.4.0
	go.qase.io/client => github.com/rancher/qase-go/client v0.0.0-20231114201952-65195ec001fa

	helm.sh/helm/v3 => github.com/rancher/helm/v3 v3.18.3-rancher1
	k8s.io/api => k8s.io/api v0.33.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.33.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.33.2
	k8s.io/apiserver => k8s.io/apiserver v0.33.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.33.2
	k8s.io/client-go => k8s.io/client-go v0.33.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.33.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.33.2
	k8s.io/code-generator => k8s.io/code-generator v0.33.2
	k8s.io/component-base => k8s.io/component-base v0.33.2
	k8s.io/component-helpers => k8s.io/component-helpers v0.33.2
	k8s.io/controller-manager => k8s.io/controller-manager v0.33.2
	k8s.io/cri-api => k8s.io/cri-api v0.33.2
	k8s.io/cri-client => k8s.io/cri-client v0.33.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.33.2
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.33.2
	k8s.io/endpointslice => k8s.io/endpointslice v0.33.2
	k8s.io/externaljwt => k8s.io/externaljwt v0.33.2
	k8s.io/kms => k8s.io/kms v0.33.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.33.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.33.2
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20250318190949-c8a335a9a2ff
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.33.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.33.2
	k8s.io/kubectl => k8s.io/kubectl v0.33.2
	k8s.io/kubelet => k8s.io/kubelet v0.33.2
	k8s.io/kubernetes => k8s.io/kubernetes v1.33.4
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.33.2
	k8s.io/metrics => k8s.io/metrics v0.33.2
	k8s.io/mount-utils => k8s.io/mount-utils v0.33.2
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.33.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.33.2
	oras.land/oras-go => oras.land/oras-go v1.2.2 // for docker 20.10.x compatibility
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v1.10.2
)

require github.com/rancher/rancher/pkg/apis v0.0.0

require (
	github.com/Azure/azure-sdk-for-go v68.0.0+incompatible
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.18.2
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.10.1
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5 v5.6.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v5 v5.0.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v5 v5.1.1
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions v1.3.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription v1.2.0
	github.com/Azure/go-autorest/autorest v0.11.30
	github.com/Azure/go-autorest/autorest/adal v0.9.24
	github.com/Azure/go-autorest/autorest/to v0.4.1-0.20240104002855-4c0e21ca2bbb
	github.com/AzureAD/microsoft-authentication-library-for-go v1.4.2
	github.com/Masterminds/semver/v3 v3.3.0
	github.com/Masterminds/sprig/v3 v3.3.0
	github.com/alibabacloud-go/cs-20151215/v5 v5.9.7
	github.com/alibabacloud-go/darabonba-openapi/v2 v2.1.10
	github.com/alibabacloud-go/ecs-20140526/v7 v7.0.13
	github.com/alibabacloud-go/resourcemanager-20200331/v3 v3.3.0
	github.com/alibabacloud-go/tea v1.3.10
	github.com/alibabacloud-go/vpc-20160428/v6 v6.12.8
	github.com/aliyun/credentials-go v1.4.7
	github.com/aws/aws-sdk-go v1.55.8
	github.com/aws/aws-sdk-go-v2 v1.37.2
	github.com/aws/aws-sdk-go-v2/credentials v1.18.3
	github.com/aws/aws-sdk-go-v2/service/eks v1.67.1
	github.com/blang/semver v3.5.1+incompatible
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/coreos/go-oidc/v3 v3.9.0
	github.com/coreos/go-semver v0.3.1
	github.com/creasty/defaults v1.5.2
	github.com/crewjam/saml v0.0.0-00010101000000-000000000000
	github.com/docker/distribution v2.8.3+incompatible
	github.com/docker/docker v25.0.8+incompatible
	github.com/ehazlett/simplelog v0.0.0-20200226020431-d374894e92a4
	github.com/evanphx/json-patch v5.9.11+incompatible
	github.com/evanphx/json-patch/v5 v5.9.11
	github.com/ghodss/yaml v1.0.0
	github.com/go-git/go-git/v5 v5.13.0
	github.com/go-ldap/ldap/v3 v3.4.1
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/golang/protobuf v1.5.4
	github.com/google/gnostic-models v0.7.0
	github.com/google/go-containerregistry v0.19.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674
	github.com/hashicorp/golang-lru v1.0.2
	github.com/k3s-io/api v0.1.0
	github.com/mattn/go-colorable v0.1.13
	github.com/mcuadros/go-version v0.0.0-20190830083331-035f6764e8d2
	github.com/microsoft/kiota-abstractions-go v1.7.0
	github.com/microsoftgraph/msgraph-sdk-go v1.51.0
	github.com/microsoftgraph/msgraph-sdk-go-core v1.2.1
	github.com/mitchellh/mapstructure v1.5.0
	github.com/moby/locker v1.0.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/oracle/oci-go-sdk v18.0.0+incompatible
	github.com/pborman/uuid v1.2.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.52.0
	github.com/prometheus/client_golang v1.23.0
	github.com/prometheus/client_model v0.6.2
	github.com/rancher/aks-operator v1.12.1
	github.com/rancher/ali-operator v0.0.0-20250910043122-2aba32fbfe4c
	github.com/rancher/apiserver v0.7.3
	github.com/rancher/channelserver v0.7.0
	github.com/rancher/dynamiclistener v0.7.2-rc1
	github.com/rancher/eks-operator v1.12.1
	github.com/rancher/fleet/pkg/apis v0.14.0-alpha.3
	github.com/rancher/gke-operator v1.12.1
	github.com/rancher/jsonpath v0.0.0-20250620213443-ad24535cf0c1
	github.com/rancher/kubernetes-provider-detector v0.1.6-0.20240606163014-fcae75779379
	github.com/rancher/lasso v0.2.5-rc.1
	github.com/rancher/machine v0.15.0-rancher134
	github.com/rancher/norman v0.7.0
	github.com/rancher/rancher/pkg/client v0.0.0
	github.com/rancher/remotedialer v0.6.0-rc.1
	github.com/rancher/remotedialer-proxy v0.6.0-rc.4
	github.com/rancher/rke v1.8.0-rc.4
	github.com/rancher/shepherd v0.0.0-20251003202915-131297166ece
	github.com/rancher/steve v0.7.18
	github.com/rancher/system-upgrade-controller/pkg/apis v0.0.0-20250710162344-185ff9f785cd
	github.com/rancher/wrangler v1.1.2
	github.com/rancher/wrangler/v3 v3.2.4
	github.com/robfig/cron v1.2.0
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.11.1
	github.com/tomnomnom/linkheader v0.0.0-20180905144013-02ca5825eb80
	github.com/urfave/cli v1.22.16
	github.com/vmware/govmomi v0.42.0
	go.uber.org/mock v0.6.0
	golang.org/x/crypto v0.41.0
	golang.org/x/mod v0.27.0
	golang.org/x/net v0.43.0
	golang.org/x/oauth2 v0.30.0
	golang.org/x/sync v0.17.0
	golang.org/x/text v0.28.0
	google.golang.org/api v0.244.0
	google.golang.org/grpc v1.74.2
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	helm.sh/helm/v3 v3.18.3
	k8s.io/api v0.34.1
	k8s.io/apiextensions-apiserver v0.33.2
	k8s.io/apimachinery v0.34.1
	k8s.io/apiserver v0.34.1
	k8s.io/cli-runtime v0.33.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/helm v2.17.0+incompatible
	k8s.io/kube-aggregator v0.33.2
	k8s.io/kube-openapi v0.0.0-20250710124328-f3f2b991d03b
	k8s.io/kubectl v0.33.2
	k8s.io/kubernetes v1.33.4
	k8s.io/utils v0.0.0-20250604170112-4c0f3b243397
	oras.land/oras-go/v2 v2.6.0
	sigs.k8s.io/aws-iam-authenticator v0.6.17
	sigs.k8s.io/cluster-api v1.10.2
	sigs.k8s.io/controller-runtime v0.21.0
	sigs.k8s.io/yaml v1.6.0
)

require (
	cel.dev/expr v0.24.0 // indirect
	cloud.google.com/go/auth v0.16.3 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	dario.cat/mergo v1.0.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/operationalinsights/armoperationalinsights v1.2.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources v1.2.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/ProtonMail/go-crypto v1.1.3 // indirect
	github.com/alibabacloud-go/alibabacloud-gateway-spi v0.0.5 // indirect
	github.com/alibabacloud-go/debug v1.0.1 // indirect
	github.com/alibabacloud-go/endpoint-util v1.1.0 // indirect
	github.com/alibabacloud-go/openapi-util v0.1.1 // indirect
	github.com/alibabacloud-go/tea-utils/v2 v2.0.7 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.30.3 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.62.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.231.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/iam v1.45.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.27.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.32.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.36.0 // indirect
	github.com/aws/smithy-go v1.22.5 // indirect
	github.com/cjlapao/common-go v0.0.39 // indirect
	github.com/clbanning/mxj/v2 v2.7.0 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.0 // indirect
	github.com/go-jose/go-jose/v3 v3.0.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/google/cel-go v0.26.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/kubereboot/kured v1.13.1 // indirect
	github.com/microsoft/kiota-authentication-azure-go v1.1.0 // indirect
	github.com/microsoft/kiota-http-go v1.4.4 // indirect
	github.com/microsoft/kiota-serialization-form-go v1.0.0 // indirect
	github.com/microsoft/kiota-serialization-json-go v1.0.8 // indirect
	github.com/microsoft/kiota-serialization-multipart-go v1.0.0 // indirect
	github.com/microsoft/kiota-serialization-text-go v1.0.0 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/prometheus/common v0.65.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/skeema/knownhosts v1.3.0 // indirect
	github.com/std-uritemplate/std-uritemplate/go v0.0.57 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/tjfoc/gmsm v1.4.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.61.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.34.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b // indirect
	golang.org/x/tools/go/packages/packagestest v0.1.1-deprecated // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250728155136-f173205681a0 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/controller-manager v0.0.0 // indirect
	k8s.io/gengo v0.0.0-20250130153323-76c5745d3511 // indirect
	k8s.io/gengo/v2 v2.0.0-20250207200755-1244d31929d7 // indirect
	k8s.io/kms v0.34.1 // indirect
	modernc.org/libc v1.66.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.39.0 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.12 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.6 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.2-0.20210111195520-9fc88b15294e // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/Azure/go-ntlmssp v0.0.0-20200615164410-66371956d46c // indirect
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/squirrel v1.5.4 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/beevik/etree v1.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/containerd/containerd v1.7.27 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/cyphar/filepath-securejoin v0.4.1
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/docker/cli v25.0.3+incompatible // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/exponent-io/jsonpath v0.0.0-20210407135951-1de76d718b3f // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.3 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-gorp/gorp/v3 v3.1.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/gobuffalo/flect v1.0.3 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gofrs/flock v0.12.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-github/v29 v29.0.3 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/gorilla/handlers v1.5.2 // indirect
	github.com/gosuri/uitable v0.0.4 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-version v1.6.0
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jmoiron/sqlx v1.4.0 // indirect
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matryer/moq v0.5.2 // indirect
	github.com/mattermost/xml-roundtrip-validator v0.1.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/onsi/gomega v1.38.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/rubenv/sql-migrate v1.8.0 // indirect
	github.com/russellhaering/goxmldsig v1.4.0
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	go.etcd.io/etcd/api/v3 v3.6.4 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.6.4 // indirect
	go.etcd.io/etcd/client/v3 v3.6.4 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/term v0.34.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	golang.org/x/tools v0.36.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.5.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/cluster-bootstrap v0.32.3 // indirect
	k8s.io/code-generator v0.33.2 // indirect
	k8s.io/component-base v0.34.1 // indirect
	k8s.io/component-helpers v0.33.2 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.31.2 // indirect
	sigs.k8s.io/cli-utils v0.37.2 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/kustomize/api v0.19.0 // indirect
	sigs.k8s.io/kustomize/kyaml v0.19.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.6.0 // indirect
)
