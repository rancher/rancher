package kontainerdriver

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mVersion "github.com/mcuadros/go-version"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	"github.com/rancher/rancher/pkg/catalog/utils"
	kd "github.com/rancher/rancher/pkg/controllers/management/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/image"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rke/util"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	img "github.com/rancher/types/image"
)

const (
	linuxImages            = "rancher-images"
	windowsImages          = "rancher-windows-images"
	rkeMetadataConfig      = "rke-metadata-config"
	forceRefreshAnnotation = "field.cattle.io/lastForceRefresh"
)

type ActionHandler struct {
	KontainerDrivers      v3.KontainerDriverInterface
	KontainerDriverLister v3.KontainerDriverLister
	MetadataHandler       kd.MetadataController
}

type ListHandler struct {
	SysImageLister v3.RKEK8sSystemImageLister
	SysImages      v3.RKEK8sSystemImageInterface
	CatalogLister  v3.CatalogLister
}

func (a ActionHandler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	// passing nil as the resource only works because just namespace is grabbed from it and nodedriver is not namespaced
	if err := apiContext.AccessControl.CanDo(v3.KontainerDriverGroupVersionKind.Group, v3.KontainerDriverResource.Name, "update", apiContext, nil, apiContext.Schema); err != nil {
		return err
	}

	switch actionName {
	case "activate":
		return a.activate(apiContext)
	case "deactivate":
		return a.deactivate(apiContext)
	case "refresh":
		return a.refresh(apiContext)
	}
	return httperror.NewAPIError(httperror.NotFound, "not found")
}

func (a ActionHandler) activate(apiContext *types.APIContext) error {
	return a.setDriverActiveStatus(apiContext, true)
}

func (a ActionHandler) deactivate(apiContext *types.APIContext) error {
	return a.setDriverActiveStatus(apiContext, false)
}

func (a ActionHandler) refresh(apiContext *types.APIContext) error {
	response := map[string]interface{}{}
	url, err := kd.GetURLSettingValue()
	if err != nil {
		msg := fmt.Sprintf("failed to get settings %v", err)
		return httperror.WrapAPIError(err, httperror.ServerError, msg)
	}
	if err := a.MetadataHandler.Refresh(url); err != nil {
		msg := fmt.Sprintf("failed to refresh %v", err)
		return httperror.WrapAPIError(err, httperror.ServerError, msg)
	}

	setting, err := a.MetadataHandler.SettingLister.Get("", rkeMetadataConfig)
	if err != nil {
		return err
	}

	if setting.Annotations == nil {
		setting.Annotations = make(map[string]string)
	}

	setting.Annotations[forceRefreshAnnotation] = strconv.FormatInt(time.Now().Unix(), 10)
	_, err = a.MetadataHandler.Settings.Update(setting)
	apiContext.WriteResponse(http.StatusOK, response)
	return nil
}

func (a ActionHandler) setDriverActiveStatus(apiContext *types.APIContext, status bool) error {
	driver, err := a.KontainerDriverLister.Get("", apiContext.ID)
	if err != nil {
		return err
	}

	driver.Spec.Active = status

	_, err = a.KontainerDrivers.Update(driver)

	return err
}

func (lh ListHandler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	k8sCurr := strings.Split(settings.KubernetesVersionsCurrent.Get(), ",")
	rkeSysImages := map[string]v3.RKESystemImages{}
	if apiContext.ID != linuxImages && apiContext.ID != windowsImages {
		return httperror.NewAPIError(httperror.NotFound, "link does not exist")
	}
	for _, k8sVersion := range k8sCurr {
		rkeSysImg, err := kd.GetRKESystemImages(k8sVersion, lh.SysImageLister, lh.SysImages)
		if err != nil {
			return err
		}
		rkeSysImgCopy := rkeSysImg.DeepCopy()
		switch apiContext.ID {
		case linuxImages:
			rkeSysImgCopy.WindowsPodInfraContainer = ""
			// removing weave images since it's not supported
			rkeSysImgCopy.WeaveNode = ""
			rkeSysImgCopy.WeaveCNI = ""
		case windowsImages:
			majorVersion := util.GetTagMajorVersion(k8sVersion)
			if mVersion.Compare(majorVersion, "v1.13", "<=") {
				continue
			}
			windowsSysImages := v3.RKESystemImages{
				Kubernetes:                rkeSysImg.Kubernetes,
				WindowsPodInfraContainer:  rkeSysImg.WindowsPodInfraContainer,
				NginxProxy:                rkeSysImg.NginxProxy,
				KubernetesServicesSidecar: rkeSysImg.KubernetesServicesSidecar,
				CertDownloader:            rkeSysImg.CertDownloader,
			}
			rkeSysImgCopy = &windowsSysImages
		}
		rkeSysImages[k8sVersion] = *rkeSysImgCopy
	}

	// get system charts path
	systemCatalog, err := lh.CatalogLister.Get("", utils.SystemLibraryName)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, "error getting system catalog")
	}
	systemCatalogHash := helmlib.CatalogSHA256Hash(systemCatalog)
	systemCatalogChartPath := filepath.Join(helmlib.CatalogCache, systemCatalogHash)

	var targetImages []string
	switch apiContext.ID {
	case linuxImages:
		targetImages, err = image.GetImages(systemCatalogChartPath, []string{}, []string{}, rkeSysImages, image.Linux)
		if err != nil {
			return httperror.WrapAPIError(err, httperror.ServerError, "error getting image list for linux platform")
		}
	case windowsImages:
		targetImages, err = image.GetImages(systemCatalogChartPath, []string{}, []string{}, rkeSysImages, image.Windows)
		if err != nil {
			return httperror.WrapAPIError(err, httperror.ServerError, "error getting image list for windows platform")
		}
	}

	agentImage := settings.AgentImage.Get()
	targetImages = append(targetImages, img.Mirror(agentImage))

	b := []byte(strings.Join(targetImages, "\n"))
	apiContext.Response.Header().Set("Content-Length", strconv.Itoa(len(b)))
	apiContext.Response.Header().Set("Content-Type", "application/octet-stream")
	apiContext.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.txt", apiContext.ID))
	apiContext.Response.Header().Set("Cache-Control", "private")
	apiContext.Response.Header().Set("Pragma", "private")
	apiContext.Response.Header().Set("Expires", "Wed 24 Feb 1982 18:42:00 GMT")
	apiContext.Response.WriteHeader(http.StatusOK)
	_, err = apiContext.Response.Write(b)
	return err
}

func (lh ListHandler) ListHandler(request *types.APIContext, next types.RequestHandler) error {
	if request.ID == linuxImages || request.ID == windowsImages {
		return lh.LinkHandler(request, next)
	}
	return handler.ListHandler(request, next)
}
