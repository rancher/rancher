// Package supportconfigs provides a HTTPHandler to serve supportconfigs. This handler should be registered at Endpoint
package supportconfigs

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/rancher/rancher/pkg/auth/util"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/managedcharts/cspadapter"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/release"
	authzv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/endpoints/request"
	authv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

const (
	// Endpoint The endpoint that this URL is accessible at - used for routing and for SARs
	Endpoint = "/v1/generateSUSERancherSupportConfig"
	// NOTE: the name of the configmap will be the same for both Bring-Your-Own (BOY) and
	// Pay-As-You-Go (PAYG) offerings. However, they will be in different namespaces to avoid
	// clashing.
	cspAdapterConfigmap = "csp-config"
	// Metering archive for CSP marketplace Pay-As-You-Go (PAYG
	// offerings. Used for auditing purposes.
	cspMeteringArchiveConfigmap = "metering-archive"
	tarContentType              = "application/x-tar"
	logPrefix                   = "support-config-generator"
)

var errNotFound = errors.New("not implemented")

type cspAdapterInterface interface {
	GetRelease(chartNamespace string, chartName string) (*release.Release, error)
}

// Handler implements http.Handler - and serves supportconfigs (tar file which contains support relevant information)
type Handler struct {
	ConfigMaps           v1.ConfigMapInterface
	SubjectAccessReviews authv1.SubjectAccessReviewInterface
	adapterUtil          cspAdapterInterface
}

// NewHandler creates a handler using the clients defined in scaledContext
func NewHandler(scaledContext *config.ScaledContext) Handler {
	return Handler{
		ConfigMaps:           scaledContext.Core.ConfigMaps(metav1.NamespaceAll),
		SubjectAccessReviews: scaledContext.K8sClient.AuthorizationV1().SubjectAccessReviews(),
		adapterUtil:          cspadapter.NewChartUtil(scaledContext.Wrangler.RESTClientGetter),
	}
}

// Check to see if user have access to the configmaps needed to create
// the supportconfig tarball. Return false if user is not authorize or error
// in checking authorization, true otherwise.
func (h *Handler) checkAuthorization(cspNamespace string, cspConfigmap string, writer http.ResponseWriter, request *http.Request) bool {
	authorized, err := h.authorize(cspNamespace, cspConfigmap, request)
	if err != nil {
		util.ReturnHTTPError(writer, request, http.StatusForbidden, http.StatusText(http.StatusForbidden))
		logrus.Errorf("[%s] Failed to authorize user with error: %s", logPrefix, err.Error())
		return false
	}
	if !authorized {
		util.ReturnHTTPError(writer, request, http.StatusForbidden, http.StatusText(http.StatusForbidden))
		return false
	}

	return true
}

// ServerHTTP implements http.Handler - attempts to authenticate/authorize the user. Returns a tar of support information
// if the user can get the backing configmap in the adapter namespace
func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	var cspChartNamespace string
	var cspChartName string

	if usePAYG := request.URL.Query()["usePAYG"]; len(usePAYG) > 0 && usePAYG[0] == "true" {
		// if usePAYG query parameter exist that means we are using the new Pay-As-You-Go offering CSP billing adapter
		cspChartNamespace = cspadapter.PAYGChartNamespace
		cspChartName = cspadapter.PAYGChartName
		if !h.checkAuthorization(cspChartNamespace, cspMeteringArchiveConfigmap, writer, request) {
			return
		}
	} else {
		// if userPAYG query parameter does exist that means we are using the old Managed License offering CSP adapter
		cspChartNamespace = cspadapter.MLOChartNamespace
		cspChartName = cspadapter.MLOChartName
	}
	if !h.checkAuthorization(cspChartNamespace, cspAdapterConfigmap, writer, request) {
		return
	}
	_, err := h.adapterUtil.GetRelease(cspChartNamespace, cspChartName)
	if err != nil {
		if errors.Is(err, cspadapter.ErrNotFound) {
			// If neither adapter is installed, return a 501, so the
			// user knows to install the adapter
			util.ReturnHTTPError(writer, request, http.StatusNotImplemented, cspChartName+" must be installed to generate supportconfigs")
			return
		}
		logrus.Errorf("[%s] Error when attempting to determine if adapter is installed, %s", logPrefix, err)
		util.ReturnHTTPError(writer, request, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	logrus.Infof("[%s] Generating supportconfig", logPrefix)
	archive, err := h.generateSupportConfig(cspChartNamespace)
	logrus.Infof("[%s] Done Generating supportconfig", logPrefix)
	if err != nil {
		if errors.Is(err, errNotFound) {
			util.ReturnHTTPError(writer, request, http.StatusServiceUnavailable, "supportconfig not yet generated, try again later")
			return
		}
		logrus.Errorf("[%s] Error when generating supportconfig %v", logPrefix, err)
		util.ReturnHTTPError(writer, request, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	writer.Header().Set("Content-Type", tarContentType)
	writer.Header().Set("Content-Disposition", "attachment; filename=\"supportconfig_rancher.tar\"")
	n, err := io.Copy(writer, archive)
	if err != nil {
		logrus.Warnf("set archive on http response writer: %v", err)
		util.ReturnHTTPError(writer, request, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	logrus.Debugf("[%s] wrote %v bytes in archive response", logPrefix, n)
}

// authorize checks to see if the user can get the given csp adapter configmap. Returns a bool (if the user is authorized)
// and optionally an error
func (h *Handler) authorize(cspNamespace string, cspConfigmap string, r *http.Request) (bool, error) {
	userInfo, ok := request.UserFrom(r.Context())
	if !ok {
		return false, fmt.Errorf("unable to extract user info from context")
	}
	response, err := h.SubjectAccessReviews.Create(r.Context(), &authzv1.SubjectAccessReview{
		Spec: authzv1.SubjectAccessReviewSpec{
			ResourceAttributes: &authzv1.ResourceAttributes{
				Resource:  "configmap",
				Verb:      "get",
				Name:      cspConfigmap,
				Namespace: cspNamespace,
			},
			User:   userInfo.GetName(),
			Groups: userInfo.GetGroups(),
			Extra:  convertExtra(userInfo.GetExtra()),
			UID:    userInfo.GetUID(),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to create a SubjectAccessReview: %w", err)
	}
	if !response.Status.Allowed {
		return false, nil
	}
	return true, nil
}

// getCSPConfig gets the configmap produced by the csp-adapter returns an error if not able to produce the map. Will return
// an errNotFound if the map is not found at all
func (h *Handler) getCSPConfig(cspNamespace string) (map[string]interface{}, error) {
	cspConfigMap, err := h.ConfigMaps.GetNamespaced(cspNamespace, cspAdapterConfigmap, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errNotFound
		}
		return nil, err
	}
	retVal := map[string]interface{}{}
	err = json.Unmarshal([]byte(cspConfigMap.Data["data"]), &retVal)
	if err != nil {
		return nil, err
	}

	return retVal, nil
}

// getMeteringArchive gets th metering-archive configmap produced by CSP billing adapter returns an error if not able to produce the map. Will return
// an errNotFound if the map is not found at all
func (h *Handler) getMeteringArchive(cspNamespace string) ([]interface{}, error) {
	cspMeteringArchive, err := h.ConfigMaps.GetNamespaced(cspNamespace, cspMeteringArchiveConfigmap, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errNotFound
		}
		return nil, err
	}
	var retVal []interface{}
	err = json.Unmarshal([]byte(cspMeteringArchive.Data["archive"]), &retVal)
	if err != nil {
		return nil, err
	}

	return retVal, nil
}

// generateSupportConfig produces an io.Reader with the supportconfig ready to be returned using a http.ResponseWriter
// Returns an err if it can't get the support config
func (h *Handler) generateSupportConfig(cspNamespace string) (io.Reader, error) {
	cspConfig, err := h.getCSPConfig(cspNamespace)
	// csp adapter won't always be installed
	if err != nil {
		return nil, err
	}
	configData, err := json.MarshalIndent(cspConfig, "", "  ")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	files := map[string][]byte{
		"rancher/config.json": configData,
	}

	if cspNamespace == cspadapter.PAYGChartNamespace {
		// For PAYG we also need to include the metering archive
		// for auditing purposes
		cspMeteringArchive, err := h.getMeteringArchive(cspNamespace)

		// NOTE: metering archive may not have generated yet so don't include it
		// if not found
		if err == nil {
			meteringArchiveData, err := json.MarshalIndent(cspMeteringArchive, "", "  ")
			if err != nil {
				return nil, err
			}
			files["rancher/metering_archive.json"] = meteringArchiveData
		} else {
			if !errors.Is(err, errNotFound) {
				return nil, err
			}
		}
	}

	for name, body := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(body); err != nil {
			return nil, err
		}
	}

	err = tw.Close()
	return &buf, err
}

func convertExtra(extra map[string][]string) map[string]authzv1.ExtraValue {
	result := map[string]authzv1.ExtraValue{}
	for k, v := range extra {
		result[k] = authzv1.ExtraValue(v)
	}
	return result
}
