package node

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
	"time"

	"encoding/json"

	"github.com/ghodss/yaml"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/encryptedstore"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/compose"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configKey = "extractedConfig"
)

type DriverHandlers struct {
	NodeDriverClient v3.NodeDriverInterface
}

func (h *DriverHandlers) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	m, err := h.NodeDriverClient.GetNamespaced("", apiContext.ID, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// passing nil as the resource only works because just namespace is grabbed from it and nodedriver is not namespaced
	if err := apiContext.AccessControl.CanDo(v3.NodeDriverGroupVersionKind.Group, v3.NodeDriverResource.Name, "update", apiContext, nil, apiContext.Schema); err != nil {
		return err
	}

	switch actionName {
	case "activate":
		m.Spec.Active = true
		v3.NodeDriverConditionActive.Unknown(m)
	case "deactivate":
		m.Spec.Active = false
		v3.NodeDriverConditionInactive.Unknown(m)
	}

	_, err = h.NodeDriverClient.Update(m)
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

// Formatter for NodeDriver
func (h *DriverHandlers) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	if err := apiContext.AccessControl.CanDo(v3.NodeDriverGroupVersionKind.Group, v3.NodeDriverResource.Name, "update", apiContext, resource.Values, apiContext.Schema); err == nil {
		resource.AddAction(apiContext, "activate")
		resource.AddAction(apiContext, "deactivate")
	}
	resource.Links["exportYaml"] = apiContext.URLBuilder.Link("exportYaml", resource)
}

type DriverHandler struct {
	SecretStore *encryptedstore.GenericEncryptedStore
}

func (h DriverHandler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	var node map[string]interface{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &node); err != nil {
		return err
	}

	if err := apiContext.AccessControl.CanDo(v3.NodeDriverGroupVersionKind.Group, v3.NodeDriverResource.Name, "update", apiContext, node, apiContext.Schema); err != nil {
		return err
	}

	nID, _ := node["id"].(string)
	nodeID := strings.Split(nID, ":")[1]
	secret, err := h.SecretStore.Get(nodeID)
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
	apiContext.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", node[client.NodeSpecFieldRequestedHostname]))
	apiContext.Response.Header().Set("Cache-Control", "private")
	apiContext.Response.Header().Set("Pragma", "private")
	apiContext.Response.Header().Set("Expires", "Wed 24 Feb 1982 18:42:00 GMT")
	_, err = apiContext.Response.Write(buf.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func (h DriverHandlers) ExportYamlHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "exportyaml":
		nodeDriver, err := h.NodeDriverClient.Get(apiContext.ID, metav1.GetOptions{})
		if err != nil {
			return err
		}
		topkey := compose.Config{}
		topkey.Version = "v3"
		nd := client.NodeDriver{}
		if err := convert.ToObj(nodeDriver.Spec, &nd); err != nil {
			return err
		}
		topkey.NodeDrivers = map[string]client.NodeDriver{}
		topkey.NodeDrivers[nodeDriver.Spec.DisplayName] = nd
		m, err := convert.EncodeToMap(topkey)
		if err != nil {
			return err
		}
		delete(m["nodeDrivers"].(map[string]interface{})[nodeDriver.Spec.DisplayName].(map[string]interface{}), "actions")
		delete(m["nodeDrivers"].(map[string]interface{})[nodeDriver.Spec.DisplayName].(map[string]interface{}), "links")
		data, err := json.Marshal(m)
		if err != nil {
			return err
		}

		buf, err := yaml.JSONToYAML(data)
		if err != nil {
			return err
		}
		reader := bytes.NewReader(buf)
		apiContext.Response.Header().Set("Content-Type", "text/yaml")
		http.ServeContent(apiContext.Response, apiContext.Request, "exportYaml", time.Now(), reader)
		return nil
	}
	return nil
}
