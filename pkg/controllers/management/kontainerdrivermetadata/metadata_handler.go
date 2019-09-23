package kontainerdrivermetadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/catalog/git"
	"github.com/rancher/rancher/pkg/randomtoken"
	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

type MetadataController struct {
	SystemImagesLister   v3.RKEK8sSystemImageLister
	SystemImages         v3.RKEK8sSystemImageInterface
	ServiceOptionsLister v3.RKEK8sServiceOptionLister
	ServiceOptions       v3.RKEK8sServiceOptionInterface
	AddonsLister         v3.RKEAddonLister
	Addons               v3.RKEAddonInterface
	ctx                  context.Context
}

type Data struct {
	K8sVersionServiceOptions  map[string]v3.KubernetesServicesOptions
	K8sVersionRKESystemImages map[string]v3.RKESystemImages
	K8sVersionedTemplates     map[string]map[string]string

	K8sVersionInfo            map[string]v3.K8sVersionInfo
	RancherDefaultK8sVersions map[string]string

	K8sVersionWindowsServiceOptions map[string]v3.KubernetesServicesOptions
}

type TickerData struct {
	cancelFunc context.CancelFunc
	interval   time.Duration
}

type URL struct {
	//http path
	path string
	// branch set if .git path by user
	branch string
	// latestHash, isGit set in parseURL
	latestHash string
	isGit      bool
}

const (
	rkeMetadataConfig = "rke-metadata-config"
	refreshInterval   = "refresh-interval-minutes"
	fileLoc           = "data/data.json"
)

var (
	httpClient = &http.Client{
		Timeout: time.Second * 30,
	}
	dataPath    = filepath.Join("./management-state", "driver-metadata", "rke")
	tickerData  *TickerData
	prevHash    string
	fileMapLock = sync.Mutex{}
	fileMapData = map[string]bool{}
)

func Register(ctx context.Context, management *config.ManagementContext) {
	mgmt := management.Management

	m := &MetadataController{
		SystemImagesLister:   mgmt.RKEK8sSystemImages("").Controller().Lister(),
		SystemImages:         mgmt.RKEK8sSystemImages(""),
		ServiceOptionsLister: mgmt.RKEK8sServiceOptions("").Controller().Lister(),
		ServiceOptions:       mgmt.RKEK8sServiceOptions(""),
		AddonsLister:         mgmt.RKEAddons("").Controller().Lister(),
		Addons:               mgmt.RKEAddons(""),
		ctx:                  ctx,
	}

	mgmt.Settings("").AddHandler(ctx, "rke-metadata-handler", m.sync)
}

func (m *MetadataController) sync(key string, setting *v3.Setting) (runtime.Object, error) {
	if setting == nil || (setting.Name != rkeMetadataConfig) {
		return nil, nil
	}

	settingValues, err := getSettingValues()
	if err != nil {
		return nil, err
	}

	interval, err := parseTime(settingValues[refreshInterval])
	if err != nil {
		if err.Error() != "refresh disabled" {
			return nil, err
		}
	}

	// don't sync if time is set to negative/zero
	if interval == 0 && tickerData != nil && tickerData.interval != 0 {
		logrus.Infof("driverMetadata: canceled counter")
		tickerData.cancelFunc()
		tickerData.interval = 0
		return setting, nil
	}

	url, urlErr := parseURL(settingValues)

	if tickerData == nil {
		// load from stored metadata on error
		if interval == 0 || urlErr != nil {
			logrus.Errorf("driverMetadata: error loading data, using stored defaults: interval %v, error %v", interval, urlErr)
			if err := m.createOrUpdateMetadataDefaults(); err != nil {
				return nil, err
			}
			return setting, nil
		}

		if err := m.refresh(url, true); err != nil {
			return nil, err
		}

		cctx, cancel := context.WithCancel(m.ctx)
		tickerData = &TickerData{cancelFunc: cancel, interval: interval}
		go m.startTicker(cctx, tickerData)

		logrus.Infof("driverMetadata initialized successfully")
		return setting, nil
	}

	// update ticker if required
	if tickerData.interval != interval {
		tickerData.cancelFunc()

		logrus.Infof("driverMetadata: starting new counter every %v", interval)
		cctx, cancel := context.WithCancel(m.ctx)
		tickerData.interval = interval
		tickerData.cancelFunc = cancel

		go m.startTicker(cctx, tickerData)
	}

	if urlErr != nil {
		return nil, urlErr
	}

	if err := m.refresh(url, false); err != nil {
		return nil, err
	}

	return setting, nil
}

func (m *MetadataController) startTicker(ctx context.Context, tickerData *TickerData) {
	checkInterval := tickerData.interval
	tryTicker := time.NewTicker(checkInterval)

	for {
		select{
		case <-ctx.Done():
			return
		case <-tryTicker.C:
			logrus.Infof("driverMetadata: checking rke-metadata-url every %v", checkInterval)
			settingValues, err := getSettingValues()
			if err != nil {
				logrus.Errorf("driverMetadata: error getting settings %v %v", settingValues, err)
				return
			}
			url, err := parseURL(settingValues)
			if err != nil {
				logrus.Errorf("driverMetadata: error parsing url %v %v", url, err)
				return
			}
			if err := m.refresh(url, false); err != nil {
				logrus.Errorf("driverMetadata failed to refresh %v", err)
			}
		}
	}
}

func (m *MetadataController) refresh(url *URL, init bool) error {
	if !toSync(url) {
		logrus.Debugf("driverMetadata: skip sync, hash up to date %v", url.latestHash)
		return nil
	}
	if !storeMap(url) {
		logrus.Debugf("driverMetadata: already in progress")
		return nil
	}
	defer deleteMap(url)
	if err := m.Refresh(url, true); err != nil {
		return err
	}
	setFinalPath(url)
	return nil
}

func parseURL(rkeData map[string]interface{}) (*URL, error) {
	url := &URL{}
	path, ok := rkeData["url"]
	if !ok {
		return nil, fmt.Errorf("url not present in settings %s", settings.RkeMetadataConfig.Get())
	}
	url.path = convert.ToString(path)
	branch, ok := rkeData["branch"]
	if !ok {
		return url, nil
	}
	url.branch = convert.ToString(branch)
	latestHash, err := git.RemoteBranchHeadCommit(url.path, url.branch)
	if err != nil {
		return nil, fmt.Errorf("error getting latest commit %s %s %v", url.path, url.branch, err)
	}
	url.latestHash = latestHash
	url.isGit = true
	return url, nil
}

func parseTime(interval interface{}) (time.Duration, error) {
	mins := convert.ToString(interval)
	if strings.HasPrefix(mins, "-") || strings.HasPrefix(mins, "0") {
		return 0, fmt.Errorf("refresh disabled")
	}
	t := fmt.Sprintf("%sm", mins)
	checkInterval, err := time.ParseDuration(t)
	if err != nil {
		return 0, err
	}
	return checkInterval, nil
}

func loadData(url *URL) (Data, error) {
	if url.isGit {
		return getDataGit(url.path, url.branch)
	}
	return getDataHTTP(url.path)
}

func getDataHTTP(url string) (Data, error) {
	var data Data
	resp, err := httpClient.Get(url)
	if err != nil {
		return data, fmt.Errorf("driverMetadata err %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return data, fmt.Errorf("driverMetadata statusCode %v", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return data, fmt.Errorf("driverMetadata read response body error %v", err)
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return data, fmt.Errorf("driverMetadata %v", err)
	}
	return data, nil
}

func getDataGit(urlPath, branch string) (Data, error) {
	var data Data

	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dataPath, 0755); err != nil {
			return data, fmt.Errorf("error creating directory %v", err)
		}
	}

	name, err := randomtoken.Generate()
	if err != nil {
		return data, fmt.Errorf("error generating metadata dirName %v", err)
	}

	path := fmt.Sprintf("%s/%s", dataPath, fmt.Sprintf("data-%s", name))
	if err := git.CloneWithDepth(path, urlPath, branch, 1); err != nil {
		return data, fmt.Errorf("error cloning repo %s %s: %v", urlPath, branch, err)
	}

	filePath := fmt.Sprintf("%s/%s", path, fileLoc)
	file, err := os.Open(filePath)
	if err != nil {
		return data, fmt.Errorf("error opening file %s %v", filePath, err)
	}
	defer file.Close()

	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return data, fmt.Errorf("error reading file %s %v", filePath, err)
	}

	if err := json.Unmarshal(buf, &data); err != nil {
		return data, fmt.Errorf("error unmarshaling metadata contents %v", err)
	}

	if err := os.RemoveAll(path); err != nil {
		logrus.Errorf("error removing metadata path %s %v", path, err)
	}
	return data, nil
}

func getSettingValues() (map[string]interface{}, error) {
	urlData := map[string]interface{}{}
	if err := json.Unmarshal([]byte(settings.RkeMetadataConfig.Get()), &urlData); err != nil {
		return nil, fmt.Errorf("unmarshal err %v", err)
	}
	return urlData, nil
}

func setFinalPath(url *URL) {
	if url.isGit {
		prevHash = url.latestHash
	}
}

func toSync(url *URL) bool {
	// check if hash changed for Git, can't do much for normal url
	if url.isGit {
		return prevHash != url.latestHash
	}
	return true
}

func deleteMap(url *URL) {
	key := getKey(url)
	fileMapLock.Lock()
	delete(fileMapData, key)
	fileMapLock.Unlock()
}

func storeMap(url *URL) bool {
	key := getKey(url)
	fileMapLock.Lock()
	defer fileMapLock.Unlock()
	if _, ok := fileMapData[key]; ok {
		return false
	}
	fileMapData[key] = true
	return true
}

func getKey(url *URL) string {
	if url.isGit {
		return url.latestHash
	}
	return url.path
}

func (m *MetadataController) Refresh(url *URL, init bool) error {
	data, err := loadData(url)
	if err != nil {
		if init {
			return m.createOrUpdateMetadataDefaults()
		}
		return err
	}
	logrus.Info("driverMetadata: refresh data")
	return m.createOrUpdateMetadata(data)
}

func GetURLSettingValue() (*URL, error) {
	settingValues, err := getSettingValues()
	if err != nil {
		return nil, err
	}
	url, err := parseURL(settingValues)
	if err != nil {
		return nil, fmt.Errorf("error parsing url %v %v", url, err)
	}
	return url, nil
}
