module github.com/rancher/rancher

go 1.13

replace (
	git.apache.org/thrift.git => github.com/apache/thrift v0.12.0
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.2.0+incompatible
	github.com/crewjam/saml => github.com/rancher/saml v0.0.0-20180713225824-ce1532152fde
	github.com/knative/pkg => github.com/rancher/pkg v0.0.0-20190514055449-b30ab9de040e
	github.com/matryer/moq => github.com/rancher/moq v0.0.0-20200712062324-13d1f37d2d77
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.3

	k8s.io/api => k8s.io/api v0.18.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.0
	k8s.io/apiserver => k8s.io/apiserver v0.18.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.0
	k8s.io/client-go => github.com/rancher/client-go v1.18.0-rancher.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.0
	k8s.io/code-generator => k8s.io/code-generator v0.18.0
	k8s.io/component-base => k8s.io/component-base v0.18.0
	k8s.io/cri-api => k8s.io/cri-api v0.18.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.0
	k8s.io/kubectl => k8s.io/kubectl v0.18.0
	k8s.io/kubelet => k8s.io/kubelet v0.18.0
	k8s.io/kubernetes => k8s.io/kubernetes v1.18.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.0
	k8s.io/metrics => k8s.io/metrics v0.18.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.0
)

require (
	github.com/Azure/azure-sdk-for-go v36.1.0+incompatible
	github.com/Azure/go-autorest/autorest v0.9.6
	github.com/Azure/go-autorest/autorest/adal v0.8.2
	github.com/Azure/go-autorest/autorest/to v0.3.1-0.20191028180845-3492b2aff503
	github.com/DataDog/zstd v1.4.5 // indirect
	github.com/aws/aws-sdk-go v1.33.5
	github.com/beevik/etree v1.1.0 // indirect
	github.com/bep/debounce v1.2.0
	github.com/blang/semver v3.5.1+incompatible
	github.com/coreos/etcd v3.3.17+incompatible
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/prometheus-operator v0.36.0
	github.com/crewjam/saml v0.0.0-00010101000000-000000000000
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.4.2-0.20200203170920-46ec8731fbce
	github.com/docker/go-connections v0.4.0
	github.com/ehazlett/simplelog v0.0.0-20200226020431-d374894e92a4
	github.com/ghodss/yaml v1.0.0
	github.com/golang/protobuf v1.3.2
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v1.0.0
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/websocket v1.4.1
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.3
	github.com/heptio/authenticator v0.0.0-20180409043135-d282f87a1972
	github.com/knative/pkg v0.0.0-20190817231834-12ee58e32cc8
	github.com/mattn/go-colorable v0.1.2
	github.com/mcuadros/go-version v0.0.0-20180611085657-6d5863ca60fa
	github.com/minio/minio-go v0.0.0-20190523192347-c6c2912aa552
	github.com/mitchellh/mapstructure v1.1.2
	github.com/mrjones/oauth v0.0.0-20180629183705-f4e24b6d100c
	github.com/oracle/oci-go-sdk v18.0.0+incompatible
	github.com/pborman/uuid v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.4.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.9.1
	github.com/rancher/apiserver v0.0.0-20200730050206-780f0e4c5f48
	github.com/rancher/dynamiclistener v0.2.1-0.20200728054858-bafb05165617
	github.com/rancher/eks-operator v0.1.0-rc22
	github.com/rancher/lasso v0.0.0-20200728055126-961184571d1b
	github.com/rancher/machine v0.15.0-rancher25
	github.com/rancher/norman v0.0.0-20200728060519-1697d43171e1
	github.com/rancher/rdns-server v0.0.0-20180802070304-bf662911db6a
	github.com/rancher/remotedialer v0.2.6-0.20200403185207-c326f2656545
	github.com/rancher/rke v1.2.0-rc4
	github.com/rancher/security-scan v0.1.7-0.20200222041501-f7377f127168
	github.com/rancher/steve v0.0.0-20200730044803-afb90310a200
	github.com/rancher/system-upgrade-controller v0.6.2
	github.com/rancher/wrangler v0.6.2-0.20200729234100-2cb57bf36360
	github.com/robfig/cron v1.1.0
	github.com/russellhaering/goxmldsig v0.0.0-20180430223755-7acd5e4a6ef7 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/segmentio/kafka-go v0.0.0-20190411192201-218fd49cff39
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.5.1
	github.com/tomnomnom/linkheader v0.0.0-20180905144013-02ca5825eb80
	github.com/urfave/cli v1.22.2
	github.com/vmihailenco/msgpack v4.0.1+incompatible
	github.com/vmware/govmomi v0.21.1-0.20191006164024-1d61d1ba0200
	github.com/vmware/kube-fluentd-operator v0.0.0-20190307154903-bf9de7e79eaf
	github.com/xanzy/go-gitlab v0.0.0-20180830102804-feb856f4760f
	github.com/xdg/scram v0.0.0-20180814205039-7eeb5667e42c // indirect
	github.com/xdg/stringprep v1.0.0 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/net v0.0.0-20200625001655-4c5254603344
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	google.golang.org/api v0.14.0
	google.golang.org/grpc v1.27.0
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	gopkg.in/ldap.v2 v2.5.1
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v2 v2.3.0
	helm.sh/helm/v3 v3.2.0
	k8s.io/api v0.18.0
	k8s.io/apiextensions-apiserver v0.18.0
	k8s.io/apimachinery v0.18.5
	k8s.io/apiserver v0.18.0
	k8s.io/cli-runtime v0.18.0
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/gengo v0.0.0-20200710205751-c0d492a0f3ca
	k8s.io/helm v2.16.7+incompatible
	k8s.io/kube-aggregator v0.18.0
	k8s.io/kubectl v0.18.0
	k8s.io/kubernetes v1.18.0
	k8s.io/utils v0.0.0-20200324210504-a9aa75ae1b89
	sigs.k8s.io/aws-iam-authenticator v0.5.1
	sigs.k8s.io/yaml v1.2.0
)
