module github.com/rancher/rancher/pkg/client

go 1.20

replace github.com/rancher/norman => github.com/chiukapoor/norman v0.0.0-20240124044239-602422347a40

replace k8s.io/client-go => github.com/rancher/client-go v1.28.6-rancher1

require (
	github.com/rancher/norman v0.0.0-20230831160711-5de27f66385d
	k8s.io/apimachinery v0.28.6
)

require (
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rancher/wrangler/v2 v2.1.3 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
)
