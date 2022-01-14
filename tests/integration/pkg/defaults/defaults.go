package defaults

import "os"

var (
	PodTestImage        = "rancher/systemd-node:v0.0.2"
	SomeK8sVersion      = os.Getenv("SOME_K8S_VERSION")
	WatchTimeoutSeconds = int64(60 * 30)
	CommonClusterConfig = map[string]interface{}{
		"service-cidr": "10.45.0.0/16",
		"cluster-cidr": "10.44.0.0/16",
		"garbage":      "value",
	}

	One   = int32(1)
	Two   = int32(2)
	Three = int32(3)
)
