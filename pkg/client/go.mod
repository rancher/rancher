module github.com/rancher/rancher/pkg/client

go 1.24.0

toolchain go1.24.6

replace golang.org/x/net => golang.org/x/net v0.36.0 // CVE-2025-22870

require (
	github.com/rancher/norman v0.0.0-20240708202514-a0127673d1b9
	k8s.io/apimachinery v0.33.4
)

require (
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rancher/wrangler/v3 v3.0.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
)
