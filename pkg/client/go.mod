module github.com/rancher/rancher/pkg/client

go 1.17

replace k8s.io/client-go => k8s.io/client-go v0.18.8

require (
	github.com/rancher/norman v0.0.0-20240205164525-bd13c653293b
	k8s.io/apimachinery v0.24.0
)

require (
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.2.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rancher/wrangler v0.6.2-0.20200820173016-2068de651106 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	golang.org/x/sys v0.0.0-20220721230656-c6bc011c0c49 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.60.1 // indirect
)
