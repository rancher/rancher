package metadata

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	mVersion "github.com/mcuadros/go-version"
	"github.com/rancher/rke/data"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/kdm"
)

const (
	RancherMetadataURLEnv = "RANCHER_METADATA_URL"
)

var (
	RKEVersion                  string
	DefaultK8sVersion           string
	K8sVersionToTemplates       map[string]map[string]string
	K8sVersionToRKESystemImages map[string]v3.RKESystemImages
	K8sVersionToServiceOptions  map[string]v3.KubernetesServicesOptions
	K8sVersionToDockerVersions  map[string][]string
	K8sVersionsCurrent          []string
	K8sBadVersions              = map[string]bool{}

	K8sVersionToWindowsServiceOptions map[string]v3.KubernetesServicesOptions

	c = http.Client{
		Timeout: time.Second * 30,
	}
	kdmMutex = sync.Mutex{}
)

func InitMetadata(ctx context.Context) error {
	kdmMutex.Lock()
	defer kdmMutex.Unlock()
	data, err := loadData()
	if err != nil {
		return fmt.Errorf("failed to load data.json, error: %v", err)
	}
	initK8sRKESystemImages(data)
	initAddonTemplates(data)
	initServiceOptions(data)
	initDockerOptions(data)
	return nil
}

// this method loads metadata, if RANCHER_METADATA_URL is provided then load data from specified location. Otherwise load data from bindata.
func loadData() (kdm.Data, error) {
	var b []byte
	var err error
	u := os.Getenv(RancherMetadataURLEnv)
	if u != "" {
		logrus.Debugf("Loading data.json from %s", u)
		b, err = readFile(u)
		if err != nil {
			return kdm.Data{}, err
		}
	} else {
		logrus.Debug("Loading data.json from local source")
		b, err = data.Asset("data/data.json")
		if err != nil {
			return kdm.Data{}, err
		}
	}
	logrus.Debugf("data.json SHA256 checksum: %x", sha256.Sum256(b))
	logrus.Tracef("data.json content: %v", string(b))
	return kdm.FromData(b)
}

func readFile(file string) ([]byte, error) {
	if strings.HasPrefix(file, "http") {
		resp, err := c.Get(file)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return ioutil.ReadAll(resp.Body)
	}
	return ioutil.ReadFile(file)
}

const RKEVersionDev = "v0.2.3"

func initAddonTemplates(data kdm.Data) {
	K8sVersionToTemplates = data.K8sVersionedTemplates
}

func initServiceOptions(data kdm.Data) {
	K8sVersionToServiceOptions = interface{}(data.K8sVersionServiceOptions).(map[string]v3.KubernetesServicesOptions)
	K8sVersionToWindowsServiceOptions = data.K8sVersionWindowsServiceOptions
}

func initDockerOptions(data kdm.Data) {
	K8sVersionToDockerVersions = data.K8sVersionDockerInfo
}

func initK8sRKESystemImages(data kdm.Data) {
	K8sVersionToRKESystemImages = map[string]v3.RKESystemImages{}
	rkeData := data
	// non released versions
	if RKEVersion == "" {
		RKEVersion = RKEVersionDev
	}
	DefaultK8sVersion = rkeData.RKEDefaultK8sVersions["default"]
	if defaultK8sVersion, ok := rkeData.RKEDefaultK8sVersions[RKEVersion[1:]]; ok {
		DefaultK8sVersion = defaultK8sVersion
	}
	maxVersionForMajorK8sVersion := map[string]string{}
	for k8sVersion, systemImages := range rkeData.K8sVersionRKESystemImages {
		rkeVersionInfo, ok := rkeData.K8sVersionInfo[k8sVersion]
		if ok {
			// RKEVersion = 0.2.4, DeprecateRKEVersion = 0.2.2
			if rkeVersionInfo.DeprecateRKEVersion != "" && mVersion.Compare(RKEVersion, rkeVersionInfo.DeprecateRKEVersion, ">=") {
				K8sBadVersions[k8sVersion] = true
				continue
			}
			// RKEVersion = 0.2.4, MinVersion = 0.2.5, don't store
			lowerThanMin := rkeVersionInfo.MinRKEVersion != "" && mVersion.Compare(RKEVersion, rkeVersionInfo.MinRKEVersion, "<")
			if lowerThanMin {
				continue
			}
		}
		// store all for upgrades
		K8sVersionToRKESystemImages[k8sVersion] = interface{}(systemImages).(v3.RKESystemImages)

		majorVersion := getTagMajorVersion(k8sVersion)
		maxVersionInfo, ok := rkeData.K8sVersionInfo[majorVersion]
		if ok {
			// RKEVersion = 0.2.4, MaxVersion = 0.2.3, don't use in current
			greaterThanMax := maxVersionInfo.MaxRKEVersion != "" && mVersion.Compare(RKEVersion, maxVersionInfo.MaxRKEVersion, ">")
			if greaterThanMax {
				continue
			}
		}
		if curr, ok := maxVersionForMajorK8sVersion[majorVersion]; !ok || mVersion.Compare(k8sVersion, curr, ">") {
			maxVersionForMajorK8sVersion[majorVersion] = k8sVersion
		}
	}
	for _, k8sVersion := range maxVersionForMajorK8sVersion {
		K8sVersionsCurrent = append(K8sVersionsCurrent, k8sVersion)
	}
}

func getTagMajorVersion(tag string) string {
	splitTag := strings.Split(tag, ".")
	if len(splitTag) < 2 {
		return ""
	}
	return strings.Join(splitTag[:2], ".")
}
