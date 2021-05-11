package defaults

var (
	PodTestImage = "ibuildthecloud/systemd-node"
	//RKE2Version         = "v1.21.0-alpha2+rke2r1"

	SomeK8sVersion      = "v1.20.5+k3s1"
	WatchTimeoutSeconds = int64(60 * 10)
	CommonClusterConfig = map[string]interface{}{
		"service-cidr": "10.45.0.0/16",
		"cluster-cidr": "10.44.0.0/16",
	}

	One   = int32(1)
	Two   = int32(2)
	Three = int32(3)
)
