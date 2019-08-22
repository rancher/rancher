package kontainerdrivermetadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/catalog/git"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/rancher/rancher/pkg/settings"

	"github.com/rancher/rancher/pkg/ticker"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"

	"github.com/rancher/types/config"
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

const (
	rkeMetadataURL      = "rke-metadata-url"
	rkeMetadataInterval = "rke-metadata-refresh-interval-minutes"
)

var tickerData *TickerData
var prevURL string

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
	if setting == nil || (setting.Name != rkeMetadataURL && setting.Name != rkeMetadataInterval) {
		return nil, nil
	}

	var interval time.Duration
	if tickerData == nil {
		if err := m.initAndStartTicker(); err != nil {
			return nil, err
		}
		logrus.Infof("driverMetadata initialized successfully")
		return setting, nil
	}

	if setting.Name == rkeMetadataURL {
		currURL := settings.RkeMetadataURL.Get()
		if currURL != prevURL {
			if err := m.refresh(); err != nil {
				return nil, err
			}
			prevURL = currURL
		}
		return setting, nil
	}

	interval, err := getTimeSetting()
	if err != nil {
		if err.Error() != "refresh disabled" {
			return nil, err
		}
	}

	if tickerData.interval != interval {

		// don't refresh/sync if time is set to negative/zero
		if interval == 0 {
			logrus.Debugf("driverMetadata: canceled counter")
			tickerData.cancelFunc()
			tickerData.interval = 0
			return setting, nil
		}

		// refresh for valid interval change
		if err := m.refresh(); err != nil {
			return nil, err
		}

		tickerData.cancelFunc()

		logrus.Infof("driverMetadata: starting new counter every %v", interval)
		cctx, cancel := context.WithCancel(m.ctx)
		tickerData.interval = interval
		tickerData.cancelFunc = cancel

		go m.startTicker(cctx, tickerData)
	}

	return setting, nil
}

func (m *MetadataController) initAndStartTicker() error {
	urlData, err := getURLSetting()
	if err != nil {
		return err
	}
	// ignore error and proceed because init
	url, _ := generateURL(urlData)
	if err := m.Refresh(url, true); err != nil {
		return err
	}
	interval, err := getTimeSetting()
	if err != nil {
		if err.Error() == "refresh disabled" {
			return nil
		}
		return err
	}
	cctx, cancel := context.WithCancel(m.ctx)
	tickerData = &TickerData{cancelFunc: cancel, interval: interval}
	go m.startTicker(cctx, tickerData)
	return nil
}

func (m *MetadataController) startTicker(ctx context.Context, tickerData *TickerData) {
	checkInterval := tickerData.interval
	for range ticker.Context(ctx, checkInterval) {
		logrus.Infof("driverMetadata: checking rke-metadata-url every %v", checkInterval)
		url, err := GetURLSettingValue()
		if err != nil {
			logrus.Errorf("driverMetadata: error getting settings %v", err)
		}
		if err := m.Refresh(url, false); err != nil {
			logrus.Errorf("driverMetadata failed to refresh %v", err)
		}
	}
}

func (m *MetadataController) Refresh(url string, init bool) error {
	data, err := loadData(url)
	if err != nil {
		if init {
			logrus.Errorf("error loading rke data, using stored defaults %v", err)
			if err := m.createOrUpdateMetadataDefaults(); err != nil {
				return err
			}
			return nil
		}
		return err
	}

	logrus.Debug("driverMetadata: refresh data")
	if err := m.createOrUpdateMetadata(data); err != nil {
		return err
	}
	return nil
}

func (m *MetadataController) refresh() error {
	url, err := GetURLSettingValue()
	if err != nil {
		return err
	}
	if err := m.Refresh(url, false); err != nil {
		return fmt.Errorf("driverMetadata failed to refresh %v", err)
	}
	return nil
}

func getURLSetting() (map[string]interface{}, error) {
	urlData := map[string]interface{}{}
	if err := json.Unmarshal([]byte(settings.RkeMetadataURL.Get()), &urlData); err != nil {
		return nil, fmt.Errorf("unmarshal err %v", err)
	}
	if _, ok := urlData["url"]; !ok {
		return nil, fmt.Errorf("url not present in settings %s", settings.RkeMetadataURL.Get())
	}
	return urlData, nil
}

func generateURL(urlData map[string]interface{}) (string, error) {
	branch, ok := urlData["branch"]
	if !ok {
		return convert.ToString(urlData["url"]), nil
	}
	latestURL, err := generateURLHelper(convert.ToString(convert.ToString(urlData["url"])), convert.ToString(branch))
	if err != nil {
		return "", err
	}
	return latestURL, nil
}

func getTimeSetting() (time.Duration, error) {
	mins := settings.RkeMetadataRefreshIntervalMins.Get()
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

func GetURLSettingValue() (string, error) {
	urlData, err := getURLSetting()
	if err != nil {
		return "", err
	}
	url, err := generateURL(urlData)
	if err != nil {
		return "", err
	}
	return url, nil
}

func generateURLHelper(url, branch string) (string, error) {
	latestCommit, err := git.RemoteBranchHeadCommit(url, branch)
	if err != nil {
		return "", err
	}
	split := strings.Split(strings.TrimSuffix(url, ".git"), "/")
	n := len(split) - 1
	if n < 1 {
		return "", fmt.Errorf("couldn't extract repo from %s", url)
	}
	repo := fmt.Sprintf("%s/%s", split[n-1], split[n])
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/data/data.json", repo, latestCommit), nil
}

func loadData(url string) (Data, error) {
	var data Data
	resp, err := http.Get(url)
	if err != nil {
		return data, fmt.Errorf("driverMetadata err %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return data, fmt.Errorf("driverMetadata statusCode %v", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return data, fmt.Errorf("read response body error %v", err)
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return data, fmt.Errorf("driverMetadata %v", err)
	}
	return data, nil
}
