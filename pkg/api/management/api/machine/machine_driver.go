package machine

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/rancher/machine-controller/store"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configKey = "extractedConfig"
)

type DriverHandlers struct {
	MachineDriverClient v3.MachineDriverInterface
}

func (h *DriverHandlers) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	m, err := h.MachineDriverClient.GetNamespaced("", apiContext.ID, metav1.GetOptions{})
	if err != nil {
		return err
	}

	switch actionName {
	case "activate":
		m.Spec.Active = true
		v3.MachineDriverConditionActive.Unknown(m)
	case "deactivate":
		m.Spec.Active = false
		v3.MachineDriverConditionInactive.Unknown(m)
	}

	_, err = h.MachineDriverClient.Update(m)
	if err != nil {
		return err
	}

	data := map[string]interface{}{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &data); err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

// Formatter for MachineDriver
func (h *DriverHandlers) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "activate")
	resource.AddAction(apiContext, "deactivate")
}

type Handler struct {
	SecretStore *store.GenericEncryptedStore
}

func (h Handler) LinkHandler(apiContext *types.APIContext) error {
	var machine client.Machine
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &machine); err != nil {
		return err
	}
	machineID := strings.Split(machine.ID, ":")[1]
	secret, err := h.SecretStore.Get(machineID)
	if err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(secret[configKey])
	if err != nil {
		return err
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(gzipReader)

	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reinitializing config (tarRead.Next): %v", err)
		}
		parts := strings.Split(header.Name, "/")
		if len(parts) != 4 {
			continue
		}

		if parts[3] == "config.json" {
			continue
		}
		fh := &zip.FileHeader{}
		fh.Name = fmt.Sprintf("%s/%s", parts[2], parts[3])
		fh.SetMode(0400)
		file, err := w.CreateHeader(fh)
		if err != nil {
			return err
		}
		buf := &bytes.Buffer{}
		_, err = io.Copy(buf, tarReader)
		if err != nil {
			return err
		}
		_, err = file.Write(buf.Bytes())
		if err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}
	apiContext.Response.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))
	apiContext.Response.Header().Set("Content-Type", "application/octet-stream")
	apiContext.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", machine.Name))
	apiContext.Response.Header().Set("Cache-Control", "private")
	apiContext.Response.Header().Set("Pragma", "private")
	apiContext.Response.Header().Set("Expires", "Wed 24 Feb 1982 18:42:00 GMT")
	_, err = apiContext.Response.Write(buf.Bytes())
	if err != nil {
		return err
	}
	return nil
}

// Formatter for Machine
func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	roles := convert.ToStringSlice(resource.Values[client.MachineFieldRole])
	if len(roles) == 0 {
		resource.Values[client.MachineFieldRole] = []string{"worker"}
	}

	// add machineConfig action
	resource.Links["machineConfig"] = apiContext.URLBuilder.Link("machineConfig", resource)

	// remove link
	machineTemplateID, ok := resource.Values["machineTemplateId"]
	if !ok || machineTemplateID == nil {
		delete(resource.Links, "remove")
		delete(resource.Links, "machineConfig")
	}
}
