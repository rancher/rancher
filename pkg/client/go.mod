module github.com/rancher/rancher/pkg/client

go 1.25.0

toolchain go1.25.4

replace github.com/rancher/norman => github.com/bhartigautam156/norman v0.0.0-20251230120334-71f332fee56c

require (
	github.com/rancher/norman v0.8.1
	k8s.io/apimachinery v0.35.0
)

require (
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rancher/wrangler/v3 v3.3.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
)
