package defaults

import (
	"os"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	PodTestImage           = "rancher/systemd-node:v0.0.5-rc2"
	ObjectStoreServerImage = "rancher/mirrored-minio-minio:RELEASE.2022-12-12T19-27-27Z"
	ObjectStoreUtilImage   = "rancher/mirrored-minio-mc:RELEASE.2022-12-13T00-23-28Z"
	SomeK8sVersion         = os.Getenv("SOME_K8S_VERSION")
	DistroDataDir          = os.Getenv("V2PROV_DISTRO_DATA_DIR")
	ProvisioningDataDir    = os.Getenv("V2PROV_PROVISIONING_DATA_DIR")
	SystemAgentDataDir     = os.Getenv("V2PROV_SYSTEM_AGENT_DATA_DIR")
	WatchTimeoutSeconds    = int64(2700) // 45 minutes.
	CommonClusterConfig    = map[string]interface{}{
		"service-cidr": "10.45.0.0/16",
		"cluster-cidr": "10.44.0.0/16",
		"garbage":      "value",
	}

	One             = int32(1)
	Two             = int32(2)
	Three           = int32(3)
	DownstreamRetry = wait.Backoff{
		Steps:    10,
		Duration: 30 * time.Second,
		Factor:   1.0,
		Jitter:   0.1,
	}
)
