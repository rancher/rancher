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

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/encryptedstore"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
)

var toIgnoreErrs = []string{"--ignore-daemonsets", "--delete-emptydir-data", "--force", "did not complete within"}
var allowedStates = map[string]bool{"active": true, "cordoned": true, "draining": true, "drained": true}

// Formatter for Node
func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	etcd := convert.ToBool(resource.Values[client.NodeFieldEtcd])
	cp := convert.ToBool(resource.Values[client.NodeFieldControlPlane])
	worker := convert.ToBool(resource.Values[client.NodeFieldWorker])
	if !etcd && !cp && !worker {
		resource.Values[client.NodeFieldWorker] = true
	}

	// add nodeConfig link
	canUpdateNode := false
	if err := apiContext.AccessControl.CanDo(v3.NodeGroupVersionKind.Group, v3.NodeResource.Name, "update", apiContext, resource.Values, apiContext.Schema); err == nil {
		canUpdateNode = true
		resource.Links["nodeConfig"] = apiContext.URLBuilder.Link("nodeConfig", resource)
	}

	// remove link
	nodeTemplateID := resource.Values["nodeTemplateId"]
	customConfig := resource.Values["customConfig"]
	if nodeTemplateID == nil {
		delete(resource.Links, "nodeConfig")
	} else if canUpdateNode {
		resource.AddAction(apiContext, "scaledown")
	}

	if nodeTemplateID == nil && customConfig == nil {
		delete(resource.Links, "remove")
	}
	if resource.Values[client.NodeFieldWorker] != true {
		return
	}
	state := convert.ToString(resource.Values["state"])
	if _, ok := allowedStates[state]; !ok {
		return
	}
	if state == "draining" {
		override := false
		for _, cond := range convert.ToMapSlice(resource.Values["conditions"]) {
			if cond["type"] == "Drained" && cond["status"] == "False" {
				if ignoreErr(convert.ToString(cond["message"])) {
					override = true
					resource.Values["state"] = "cordoned"
					break
				}
			}
		}
		if !override {
			resource.AddAction(apiContext, "stopDrain")
			return
		}
	}
	if state != "drained" {
		resource.AddAction(apiContext, "drain")
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
	case "drain":
		return drainNode(actionName, apiContext, false)
	case "stopDrain":
		return drainNode(actionName, apiContext, true)
	case "scaledown":
		return scaledownNode(actionName, apiContext)
	}
	return nil
}

func scaledownNode(actionName string, apiContext *types.APIContext) error {
	node, schema, err := getNodeAndSchema(apiContext)
	if err != nil {
		return err
	}

	values.PutValue(node, time.Now().Format(time.RFC3339), "scaledownTime")
	err = updateNode(apiContext, node, schema, actionName)
	if err != nil {
		return err
	}
	apiContext.WriteResponse(http.StatusOK, map[string]interface{}{})
	return nil
}

func cordonUncordonNode(actionName string, apiContext *types.APIContext, cordon bool) error {
	node, schema, err := getNodeAndSchema(apiContext)
	if err != nil {
		return err
	}
	unschedulable := convert.ToBool(values.GetValueN(node, "unschedulable"))
	if cordon == unschedulable {
		return httperror.NewAPIError(httperror.InvalidAction, fmt.Sprintf("Node %s already %sed", apiContext.ID, actionName))
	}
	values.PutValue(node, convert.ToString(!unschedulable), "desiredNodeUnschedulable")
	err = updateNode(apiContext, node, schema, actionName)
	if err != nil {
		return err
	}
	apiContext.WriteResponse(http.StatusOK, map[string]interface{}{})
	return nil
}

func drainNode(actionName string, apiContext *types.APIContext, stop bool) error {
	node, schema, err := getNodeAndSchema(apiContext)
	if err != nil {
		return err
	}
	if !stop {
		drainInput, err := validateDrainInput(apiContext)
		if err != nil {
			return err
		}
		if drainInput != nil && drainInput.IgnoreDaemonSets == nil {
			trueVal := true
			drainInput.IgnoreDaemonSets = &trueVal
		}
		values.PutValue(node, drainInput, "nodeDrainInput")
	}
	values.PutValue(node, actionName, "desiredNodeUnschedulable")
	err = updateNode(apiContext, node, schema, actionName)
	if err != nil {
		return err
	}
	apiContext.WriteResponse(http.StatusOK, map[string]interface{}{})
	return nil
}

func validateDrainInput(apiContext *types.APIContext) (*v32.NodeDrainInput, error) {
	input, err := handler.ParseAndValidateActionBody(apiContext, apiContext.Schemas.Schema(&managementschema.Version,
		client.NodeDrainInputType))
	if err != nil {
		return nil, httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse action body: %v", err))
	}
	drainInput := &v32.NodeDrainInput{}
	if err := mapstructure.Decode(input, drainInput); err != nil {
		return nil, httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("Failed to parse body: %v", err))
	}
	return drainInput, nil
}

func getNodeAndSchema(apiContext *types.APIContext) (map[string]interface{}, *types.Schema, error) {
	var node map[string]interface{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, &node); err != nil {
		return nil, nil, httperror.NewAPIError(httperror.InvalidReference, "Error accessing node")
	}
	schema := apiContext.Schemas.Schema(&managementschema.Version, client.NodeType)
	return node, schema, nil
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
	apiContext.Response.WriteHeader(http.StatusOK)
	_, err = apiContext.Response.Write(buf.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func updateNode(apiContext *types.APIContext, node map[string]interface{}, schema *types.Schema, actionName string) error {
	if _, err := schema.Store.Update(apiContext, schema, node, apiContext.ID); err != nil {
		return err
	}
	return nil
}

func ignoreErr(msg string) bool {
	for _, val := range toIgnoreErrs {
		if strings.Contains(msg, val) {
			return true
		}
	}
	return false
}
