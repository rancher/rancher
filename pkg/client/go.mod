module github.com/rancher/rancher/pkg/client

go 1.22

replace k8s.io/client-go => github.com/rancher/client-go v1.28.6-rancher1

require (
	github.com/rancher/norman v0.0.0-20240503193601-9f5f6586bb5b
	k8s.io/apimachinery v0.29.3
)

require (
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rancher/wrangler/v2 v2.2.0-rc6 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.110.1 // indirect
)
