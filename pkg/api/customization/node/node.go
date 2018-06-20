package node

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/encryptedstore"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Formatter for Node
func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	etcd := convert.ToBool(resource.Values[client.NodeFieldEtcd])
	cp := convert.ToBool(resource.Values[client.NodeFieldControlPlane])
	worker := convert.ToBool(resource.Values[client.NodeFieldWorker])
	if !etcd && !cp && !worker {
		resource.Values[client.NodeFieldWorker] = true
	}

	// add nodeConfig link
	if err := apiContext.AccessControl.CanDo(v3.NodeGroupVersionKind.Group, v3.NodeResource.Name, "update", apiContext, resource.Values, apiContext.Schema); err == nil {
		resource.Links["nodeConfig"] = apiContext.URLBuilder.Link("nodeConfig", resource)
	}

	// remove link
	nodeTemplateID := resource.Values["nodeTemplateId"]
	customConfig := resource.Values["customConfig"]
	if nodeTemplateID == nil {
		delete(resource.Links, "nodeConfig")
	}

	if nodeTemplateID == nil && customConfig == nil {
		delete(resource.Links, "remove")
	}

	if convert.ToBool(resource.Values["unschedulable"]) {
		resource.AddAction(apiContext, "uncordon")
	} else {
		resource.AddAction(apiContext, "cordon")
	}
}

type ActionWrapper struct{}

func (a ActionWrapper) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case "cordon":
		return cordonUncordonNode(actionName, apiContext, true)

	case "uncordon":
		return cordonUncordonNode(actionName, apiContext, false)
	}

	return nil
}

func cordonUncordonNode(actionName string, apiContext *types.APIContext, cordon bool) error {
	var node map[string]interface{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &node); err != nil {
		return httperror.NewAPIError(httperror.InvalidReference, "Error accessing node")
	}
	schema := apiContext.Schemas.Schema(&managementschema.Version, client.NodeType)
	unschedulable := convert.ToBool(values.GetValueN(node, "unschedulable"))
	if cordon == unschedulable {
		return httperror.NewAPIError(httperror.InvalidAction, fmt.Sprintf("Node %s already %sed", apiContext.ID, actionName))
	}
	values.PutValue(node, convert.ToString(!unschedulable), "desiredNodeUnschedulable")
	if _, err := schema.Store.Update(apiContext, schema, node, apiContext.ID); err != nil && apierrors.IsNotFound(err) {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("Error updating node %s by %s : %s", apiContext.ID, actionName, err.Error()))
	}
	return nil
}

type Handler struct {
	SecretStore *encryptedstore.GenericEncryptedStore
}

func (h Handler) LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	var node map[string]interface{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &node); err != nil {
		return err
	}

	if err := apiContext.AccessControl.CanDo(v3.NodeGroupVersionKind.Group, v3.NodeResource.Name, "update", apiContext, node, apiContext.Schema); err != nil {
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
