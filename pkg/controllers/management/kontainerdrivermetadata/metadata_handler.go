package kontainerdrivermetadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	m.refresh(settings.RkeMetadataURL.Get())
	m.initTicker(ctx)

	mgmt.Settings("").AddHandler(ctx, "rke-metadata-handler", m.sync)
}

func (m *MetadataController) sync(key string, test *v3.Setting) (runtime.Object, error) {
	if test == nil || (test.Name != "rke-metadata-url" && test.Name != "rke-metadata-refresh-interval") {
		return nil, nil
	}
	url, interval, err := getSettingValues()
	if err != nil {
		return nil, fmt.Errorf("driverMetadata error getting interval in int %s %v", settings.RkeMetadataRefreshInterval.Get(), err)
	}
	if tickerData != nil {
		if tickerData.url != url || tickerData.interval != interval {
			tickerData.cancelFunc()

			logrus.Infof("driverMetadata: starting new %s %v", url, interval)
			cctx, cancel := context.WithCancel(m.ctx)
			tickerData.interval = interval
			tickerData.url = url
			tickerData.cancelFunc = cancel

			go m.startTicker(cctx, tickerData)
		}
	}
	return test, nil
}

func (m *MetadataController) initTicker(ctx context.Context) {
	url, interval, err := getSettingValues()
	if err != nil {
		panic(fmt.Errorf("driverMetadata error getting interval in int %s %v", settings.RkeMetadataRefreshInterval.Get(), err))
	}
	cctx, cancel := context.WithCancel(ctx)

	tickerData = &TickerData{cancelFunc: cancel, url: url, interval: interval}

	go m.startTicker(cctx, tickerData)
}

func (m *MetadataController) startTicker(ctx context.Context, tickerData *TickerData) {
	checkInterval, url := tickerData.interval, tickerData.url
	for range ticker.Context(ctx, checkInterval) {
		logrus.Infof("driverMetadata: checking rke-metadata-url every %v", checkInterval)
		m.refresh(url)
	}
}

func (m *MetadataController) refresh(url string) {
	resp, err := http.Get(url)
	if err != nil {
		logrus.Errorf("driverMetadata err %v", err)
		return
	}
	if resp.StatusCode != 200 {
		logrus.Errorf("driverMetadata statusCode %v", resp.StatusCode)
	}
	defer resp.Body.Close()

	var testData Data
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respByte := buf.Bytes()

	if err := json.Unmarshal(respByte, &testData); err != nil {
		logrus.Errorf("driverMetadata %v", err)
		return
	}

	if err := m.createOrUpdateMetadata(testData); err != nil {
		logrus.Errorf("driverMetadata %v", err)
	}
}

func getSettingValues() (string, time.Duration, error) {
	n, err := strconv.Atoi(settings.RkeMetadataRefreshInterval.Get())
	if err != nil {
		return "", 0, err
	}
	checkInterval := time.Duration(n) * time.Minute
	return settings.RkeMetadataURL.Get(), checkInterval, nil
}
