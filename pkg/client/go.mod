module github.com/rancher/rancher/pkg/client

go 1.24.6

replace (
	github.com/rancher/norman => github.com/swastik959/norman v0.0.0-20250908065538-4a8ca43cc36b
	github.com/rancher/wrangler/v3 => github.com/krunalhinguu/wrangler/v3 v3.3.1-0.20250905101441-b1b29faa8ccd

)

require (
	github.com/rancher/norman v0.7.0
	k8s.io/apimachinery v0.34.0
)

require (
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rancher/wrangler/v3 v3.2.2 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
)
