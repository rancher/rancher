module github.com/rancher/rancher/pkg/client

go 1.26.0

toolchain go1.26.6

replace (
	github.com/rancher/norman => github.com/Abhishek-Valaboju/norman v0.10.1-0.20260828094032-75fa12a9114f
	github.com/rancher/wrangler/v3 => github.com/Abhishek-Valaboju/wrangler/v3 v3.5.1-rc.1.0.20260828064927-bed3da9cc545
)

require (
	github.com/rancher/norman v0.0.0-00010101000000-000000000000
	k8s.io/apimachinery v0.37.0
)

require (
	github.com/fxamacker/cbor/v2 v2.9.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/rancher/wrangler/v3 v3.7.0 // indirect
	github.com/sirupsen/logrus v1.10.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)
