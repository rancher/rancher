package kontainerdrivermetadata

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/convert"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/channelserver"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MetadataController struct {
	NamespacesController     v1.NamespaceController
	SystemImagesController   mgmtcontrollers.RkeK8sSystemImageController
	ServiceOptionsController mgmtcontrollers.RkeK8sServiceOptionController
	Addons                   mgmtcontrollers.RkeAddonController
	Settings                 mgmtcontrollers.SettingController
	url                      *MetadataURL
	wranglerContext          *wrangler.Context
}

type MetadataURL struct {
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
	prevHash    string
	fileMapLock = sync.Mutex{}
	fileMapData = map[string]bool{}
)

func Register(ctx context.Context, wCtx *wrangler.Context) {

	m := &MetadataController{
		SystemImagesController:   wCtx.Mgmt.RkeK8sSystemImage(),
		ServiceOptionsController: wCtx.Mgmt.RkeK8sServiceOption(),
		NamespacesController:     wCtx.Core.Namespace(),
		Addons:                   wCtx.Mgmt.RkeAddon(),
		Settings:                 wCtx.Mgmt.Setting(),
		wranglerContext:          wCtx,
	}

	wCtx.Mgmt.Setting().OnChange(ctx, "rke-metadata-handler", m.sync)
	wCtx.Mgmt.Setting().Enqueue(rkeMetadataConfig)
}

func (m *MetadataController) sync(_ string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil || (setting.Name != rkeMetadataConfig) {
		return nil, nil
	}

	if _, err := m.NamespacesController.Get(namespace.GlobalNamespace, metav1.GetOptions{}); err != nil {
		return nil, fmt.Errorf("failed to get %s namespace", namespace.GlobalNamespace)
	}

	value := setting.Value
	if value == "" {
		value = setting.Default
	}
	settingValues, err := getSettingValues(value)
	if err != nil {
		return nil, fmt.Errorf("error getting setting values: %v", err)
	}

	metadata, err := parseURL(settingValues)
	if err != nil {
		return nil, err
	}
	m.url = metadata

	interval, err := convert.ToNumber(settingValues[refreshInterval])
	if err != nil {
		return nil, fmt.Errorf("invalid number %v", interval)
	}

	if interval > 0 {
		logrus.Infof("Refreshing driverMetadata in %v minutes", interval)
		m.Settings.EnqueueAfter(setting.Name, time.Minute*time.Duration(interval))
	}

	// refresh to sync k3s/rke2 releases
	channelserver.Refresh()
	return setting, m.refresh()
}

func (m *MetadataController) refresh() error {
	if !toSync(m.url) {
		logrus.Infof("driverMetadata: skip sync, hash up to date %v", m.url.latestHash)
		return nil
	}
	if !storeMap(m.url) {
		logrus.Infof("driverMetadata: already in progress")
		return nil
	}
	defer deleteMap(m.url)
	if err := m.Refresh(m.url); err != nil {
		logrus.Warnf("%v, Fallback to refresh from local file path %v", err, DataJSONLocation)
		return errors.Wrapf(m.createOrUpdateMetadataFromLocal(), "failed to refresh from local file path: %s", DataJSONLocation)
	}
	setFinalPath(m.url)
	return nil
}

func (m *MetadataController) Refresh(url *MetadataURL) error {
	data, err := loadData(url)
	if err != nil {
		return errors.Wrapf(err, "failed to refresh data from upstream %v", url.path)
	}
	logrus.Infof("driverMetadata: refreshing data from upstream %v", url.path)
	return errors.Wrap(m.createOrUpdateMetadata(data), "failed to create or update driverMetadata")
}

func GetURLSettingValue() (*MetadataURL, error) {
	settingValues, err := getSettingValues(settings.RkeMetadataConfig.Get())
	if err != nil {
		return nil, err
	}
	url, err := parseURL(settingValues)
	if err != nil {
		return nil, fmt.Errorf("error parsing url %v %v", url, err)
	}
	return url, nil
}
