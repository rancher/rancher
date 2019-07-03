package kontainerdrivermetadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/rancher/rancher/pkg/settings"

	"github.com/rancher/rancher/pkg/ticker"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"

	"github.com/rancher/types/config"
)

type MetadataController struct {
	SystemImagesLister        v3.RKEK8sSystemImageLister
	SystemImages              v3.RKEK8sSystemImageInterface
	ServiceOptionsLister      v3.RKEK8sServiceOptionLister
	ServiceOptions            v3.RKEK8sServiceOptionInterface
	AddonsLister              v3.RKEAddonLister
	Addons                    v3.RKEAddonInterface
	WindowsSystemImagesLister v3.RKEK8sWindowsSystemImageLister
	WindowsSystemImages       v3.RKEK8sWindowsSystemImageInterface
	ctx                       context.Context
}

type Data struct {
	K8sVersionServiceOptions  map[string]v3.KubernetesServicesOptions
	K8sVersionRKESystemImages map[string]v3.RKESystemImages
	K8sVersionedTemplates     map[string]map[string]string

	K8sVersionInfo            map[string]v3.K8sVersionInfo
	RancherDefaultK8sVersions map[string]string

	K8sVersionWindowsSystemImages   map[string]v3.WindowsSystemImages
	K8sVersionWindowsServiceOptions map[string]v3.KubernetesServicesOptions
}

type TickerData struct {
	cancelFunc context.CancelFunc
	url        string
	interval   time.Duration
}

var tickerData *TickerData

func Register(ctx context.Context, management *config.ManagementContext) {
	mgmt := management.Management

	m := &MetadataController{
		SystemImagesLister:        mgmt.RKEK8sSystemImages("").Controller().Lister(),
		SystemImages:              mgmt.RKEK8sSystemImages(""),
		ServiceOptionsLister:      mgmt.RKEK8sServiceOptions("").Controller().Lister(),
		ServiceOptions:            mgmt.RKEK8sServiceOptions(""),
		AddonsLister:              mgmt.RKEAddons("").Controller().Lister(),
		Addons:                    mgmt.RKEAddons(""),
		WindowsSystemImagesLister: mgmt.RKEK8sWindowsSystemImages("").Controller().Lister(),
		WindowsSystemImages:       mgmt.RKEK8sWindowsSystemImages(""),
		ctx:                       ctx,
	}

	// load values on startup and start ticker
	m.refresh(settings.RkeMetadataURL.Get(), true)
	m.initTicker(ctx)

	mgmt.Settings("").AddHandler(ctx, "rke-metadata-handler", m.sync)
}

func (m *MetadataController) sync(key string, test *v3.Setting) (runtime.Object, error) {
	if test == nil || (test.Name != "rke-metadata-url" && test.Name != "rke-metadata-refresh-interval-minutes") {
		return nil, nil
	}
	url, interval, err := getSettingValues()
	if err != nil {
		return nil, fmt.Errorf("driverMetadata error getting interval in int %s %v", settings.RkeMetadataRefreshIntervalMins.Get(), err)
	}
	if tickerData != nil {
		m.refresh(url, false)
		if tickerData.url != url || tickerData.interval != interval {
			tickerData.cancelFunc()

			logrus.Infof("driverMetadata: starting new %s %v", url, interval.Minutes())
			cctx, cancel := context.WithCancel(m.ctx)
			tickerData.interval = interval
			tickerData.url = url
			tickerData.cancelFunc = cancel

			go m.startTicker(cctx, tickerData, false)
		}
	}
	return test, nil
}

func (m *MetadataController) initTicker(ctx context.Context) {
	url, interval, err := getSettingValues()
	if err != nil {
		panic(fmt.Errorf("driverMetadata error getting interval in int %s %v", settings.RkeMetadataRefreshIntervalMins.Get(), err))
	}
	cctx, cancel := context.WithCancel(ctx)

	tickerData = &TickerData{cancelFunc: cancel, url: url, interval: interval}

	go m.startTicker(cctx, tickerData, true)
}

func (m *MetadataController) startTicker(ctx context.Context, tickerData *TickerData, init bool) {
	checkInterval, url := tickerData.interval, tickerData.url
	for range ticker.Context(ctx, checkInterval) {
		logrus.Infof("driverMetadata: checking rke-metadata-url every %v", checkInterval)
		m.refresh(url, false)
	}
}

func (m *MetadataController) refresh(url string, init bool) {
	data, err := loadData(url)
	if err != nil {
		if init {
			logrus.Errorf("error loading rke data, using stored defaults %v", err)
			if err := m.createOrUpdateMetadataDefaults(); err != nil {
				logrus.Errorf("driverMetadata %v", err)
			}
		}
		return
	}

	if err := m.createOrUpdateMetadata(data); err != nil {
		logrus.Errorf("driverMetadata %v", err)
	}
}

func getSettingValues() (string, time.Duration, error) {
	t := fmt.Sprintf("%sm", settings.RkeMetadataRefreshIntervalMins.Get())
	checkInterval, err := time.ParseDuration(t)
	if err != nil {
		return "", 0, err
	}
	return settings.RkeMetadataURL.Get(), checkInterval, nil
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
