module github.com/rancher/rancher/pkg/client

go 1.19

replace k8s.io/client-go => github.com/doflamingo721/client-go v1.26.5-rancher2

replace github.com/rancher/norman => github.com/doflamingo721/norman v1.26.5-rancher1

require (
	github.com/rancher/norman v0.0.0-20230426211126-d3552b018687
	k8s.io/apimachinery v0.26.5
)

require (
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rancher/wrangler v1.1.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.80.1 // indirect
)
