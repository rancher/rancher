package defaults

var (
	PodTestImage        = "rancher/systemd-node:v0.0.1"
	SomeK8sVersion      = "v1.21.1-rc1+k3s1"
	WatchTimeoutSeconds = int64(60 * 10)
	CommonClusterConfig = map[string]interface{}{
		"service-cidr": "10.45.0.0/16",
		"cluster-cidr": "10.44.0.0/16",
		"garbage":      "value",
	}

	One   = int32(1)
	Two   = int32(2)
	Three = int32(3)
)
