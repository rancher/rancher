package defaults

import "os"

var (
	PodTestImage        = "rancher/systemd-node:v0.0.2"
	SomeRKEVersion      = "v1.21.1-rc2+rke2r1"
	SomeK3SVersion      = "v1.21.1+k3s1"
	SomeK8sVersion      = SomeK3SVersion
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

func init() {
	if os.Getenv("TEST_RKE") == "true" {
		SomeK8sVersion = SomeRKEVersion
	}
}
