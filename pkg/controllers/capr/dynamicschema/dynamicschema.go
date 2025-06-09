package dynamicschema

import (
	"context"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/capr/dynamicschema/sample"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/rancher/wrangler/v3/pkg/data/convert"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/schemas"
	"github.com/rancher/wrangler/v3/pkg/schemas/openapi"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	machineAPIGroup       = "rke-machine.cattle.io"
	MachineConfigAPIGroup = "rke-machine-config.cattle.io"
)

type handler struct {
	schemaCache       mgmtcontrollers.DynamicSchemaCache
	schemasController mgmtcontrollers.DynamicSchemaController
	sampleProps       *apiextv1.JSONSchemaProps
}

func Register(ctx context.Context, clients *wrangler.Context) error {
	sampleProps, err := sample.GetSampleProps()
	if err != nil {
		return err
	}

	h := handler{
		schemaCache:       clients.Mgmt.DynamicSchema().Cache(),
		schemasController: clients.Mgmt.DynamicSchema(),
		sampleProps:       sampleProps,
	}

	mgmtcontrollers.RegisterDynamicSchemaGeneratingHandler(ctx,
		clients.Mgmt.DynamicSchema(),
		clients.Apply.WithCacheTypes(clients.CRD.CustomResourceDefinition()),
		"",
		"dynamic-driver-crd",
		h.OnChange,
		&generic.GeneratingHandlerOptions{
			AllowClusterScoped: true,
		})

	return nil
}

func getStatusSchema(allSchemas *schemas.Schemas) (*schemas.Schema, error) {
	return allSchemas.Import(rkev1.RKEMachineStatus{})
}

func addConfigSchema(name string, specSchema *schemas.Schema, allSchemas *schemas.Schemas) (string, error) {
	nodeConfigFields := removeKey(specSchema.ResourceFields, "common")
	nodeConfigFields = removeKey(nodeConfigFields, "providerID")

	// check if the infra provider supports Windows
	// and add the OS field to an infra provider node's config
	if capr.WindowsCheck(name) {
		nodeConfigFields = addField(specSchema.ResourceFields, name, "os")
	}

	id := name + "Config"
	return id, allSchemas.AddSchema(schemas.Schema{
		ID:             id,
		ResourceFields: nodeConfigFields,
		Description: fmt.Sprintf(
			"%s represents the desired configuration of each machine provisioned within a machine pool associated with a provisioning.cattle.io cluster.",
			convert.Capitalize(id)),
	})
}

func addMachineSchema(name string, specSchema, statusSchema *schemas.Schema, allSchemas *schemas.Schemas) (string, error) {
	id := name + "Machine"
	return id, allSchemas.AddSchema(schemas.Schema{
		ID: id,
		ResourceFields: map[string]schemas.Field{
			"spec": {
				Type:        specSchema.ID,
				Description: "Desired state of the machine, generated from the pool configuration.",
			},
			"status": {
				Type: statusSchema.ID,
			},
		},
		Description: fmt.Sprintf("%s is a Rancher CAPI infrastructure provider InfrastructureMachine.", convert.Capitalize(id)),
	})
}

func addMachineTemplateSchema(name string, specSchema *schemas.Schema, allSchemas *schemas.Schemas) (string, error) {
	templateTemplateSpecSchemaID := name + "MachineTemplateTemplateSpec"
	err := allSchemas.AddSchema(schemas.Schema{
		ID: templateTemplateSpecSchemaID,
		ResourceFields: map[string]schemas.Field{
			"spec": {
				Type:        specSchema.ID,
				Description: "Specification of the template object.",
			},
		},
	})
	if err != nil {
		return "", err
	}

	templateSpecSchemaID := name + "MachineTemplateTemplate"
	err = allSchemas.AddSchema(schemas.Schema{
		ID: templateSpecSchemaID,
		ResourceFields: map[string]schemas.Field{
			"template": {
				Type:        templateTemplateSpecSchemaID,
				Description: "Template for creating new machines.",
			},
			"clusterName": {
				Type:        "string",
				Description: "Name of the provisioning.cattle.io cluster that generated this template.",
			},
		},
	})
	if err != nil {
		return "", err
	}

	id := name + "MachineTemplate"
	return id, allSchemas.AddSchema(schemas.Schema{
		ID: id,
		ResourceFields: map[string]schemas.Field{
			"spec": {
				Type:        templateSpecSchemaID,
				Description: "Specification of the machines in the template, generated from the pool configuration.",
			},
		},
		Description: fmt.Sprintf("%s is a Rancher CAPI infrastructure provider InfrastructureMachineTemplate.",
			convert.Capitalize(id)),
	})
}

func getSchemas(name string, spec *v3.DynamicSchemaSpec) (string, string, string, *schemas.Schemas, error) {
	allSchemas, err := schemas.NewSchemas()
	if err != nil {
		return "", "", "", nil, err
	}

	configSpecSchema, err := getConfigSchemas(name, allSchemas, spec)
	if err != nil {
		return "", "", "", nil, err
	}

	specSchema, err := getSpecSchemas(name, allSchemas, spec)
	if err != nil {
		return "", "", "", nil, err
	}

	statusSchema, err := getStatusSchema(allSchemas)
	if err != nil {
		return "", "", "", nil, err
	}

	nodeConfigID, err := addConfigSchema(name, configSpecSchema, allSchemas)
	if err != nil {
		return "", "", "", nil, err
	}

	templateID, err := addMachineTemplateSchema(name, specSchema, allSchemas)
	if err != nil {
		return "", "", "", nil, err
	}

	machineID, err := addMachineSchema(name, specSchema, statusSchema, allSchemas)
	if err != nil {
		return "", "", "", nil, err
	}

	return nodeConfigID, templateID, machineID, allSchemas, nil
}

func removeKey(fields map[string]schemas.Field, key string) map[string]schemas.Field {
	result := map[string]schemas.Field{}
	for k, v := range fields {
		if k != key {
			result[k] = v
		}
	}
	return result
}

func addField(rFields map[string]schemas.Field, name, newField string) map[string]schemas.Field {
	newf := rFields
	if _, ok := newf[newField]; !ok {
		newf[newField] = schemas.Field{
			Type:   "string",
			Create: true,
			Update: true,
		}
	}
	return newf
}

func getConfigSchemas(name string, allSchemas *schemas.Schemas, spec *v3.DynamicSchemaSpec) (*schemas.Schema, error) {
	specSchema := schemas.Schema{}
	if err := convert.ToObj(spec, &specSchema); err != nil {
		return nil, err
	}
	specSchema.ID = name + "ConfigSpec"

	commonField, err := allSchemas.Import(rkev1.RKECommonNodeConfig{})
	if err != nil {
		return nil, err
	}

	if specSchema.ResourceFields == nil {
		specSchema.ResourceFields = map[string]schemas.Field{}
	}

	specSchema.ResourceFields["common"] = schemas.Field{
		Type: commonField.ID,
	}

	specSchema.ResourceFields["providerID"] = schemas.Field{
		Type: "string",
	}

	for name, field := range specSchema.ResourceFields {
		defMap, ok := field.Default.(map[string]interface{})
		if !ok {
			continue
		}

		// set to nil because if map is len() == 0
		field.Default = nil

		// Only add defaults for config objects, defaults will be handled for machines and machine templates based on config
		switch field.Type {
		case "string", "password":
			field.Default = defMap["stringValue"]
		case "int":
			field.Default = defMap["intValue"]
		case "boolean":
			field.Default = defMap["boolValue"]
		case "array[string]":
			field.Default = defMap["stringSliceValue"]
		}

		specSchema.ResourceFields[name] = field
	}

	if err := allSchemas.AddSchema(specSchema); err != nil {
		return nil, err
	}

	return allSchemas.Schema(specSchema.ID), nil
}

func getSpecSchemas(name string, allSchemas *schemas.Schemas, spec *v3.DynamicSchemaSpec) (*schemas.Schema, error) {
	specSchema := schemas.Schema{}
	if err := convert.ToObj(spec, &specSchema); err != nil {
		return nil, err
	}
	specSchema.ID = name + "Spec"

	commonField, err := allSchemas.Import(rkev1.RKECommonNodeConfig{})
	if err != nil {
		return nil, err
	}

	if specSchema.ResourceFields == nil {
		specSchema.ResourceFields = map[string]schemas.Field{}
	}

	specSchema.ResourceFields["common"] = schemas.Field{
		Type: commonField.ID,
	}

	specSchema.ResourceFields["providerID"] = schemas.Field{
		Type:        "string",
		Description: "Identifier for the machine, corresponds to the node object providerID.",
	}

	for name, field := range specSchema.ResourceFields {
		// Clear all defaults, defaults will be handled for machines and machine templates based on config objects
		field.Default = nil
		specSchema.ResourceFields[name] = field
	}

	if err := allSchemas.AddSchema(specSchema); err != nil {
		return nil, err
	}

	return allSchemas.Schema(specSchema.ID), nil
}

func getCRD(props *apiextv1.JSONSchemaProps, id, group string) *crd.CRD {
	_, ok := props.Properties["status"]

	return &crd.CRD{
		GVK: schema.GroupVersionKind{
			Group:   group,
			Version: rkev1.SchemeGroupVersion.Version,
			Kind:    convert.Capitalize(id),
		},
		Schema: props,
		Labels: map[string]string{
			"cluster.x-k8s.io/v1beta1":       "v1",
			"auth.cattle.io/cluster-indexed": "true",
		},
		Status: ok,
	}
}

func getConfigCRD(schemas *schemas.Schemas, id string) (runtime.Object, error) {
	props, err := openapi.ToOpenAPI(id, schemas)
	if err != nil {
		return nil, err
	}

	crd := getCRD(props, id, MachineConfigAPIGroup)

	crdObj, err := crd.ToCustomResourceDefinition()
	if err != nil {
		return nil, err
	}

	return crdObj, nil
}

func getMachineTemplateCRD(schemas *schemas.Schemas, sampleProps *apiextv1.JSONSchemaProps, id string) (runtime.Object, error) {
	props, err := openapi.ToOpenAPI(id, schemas)
	if err != nil {
		return nil, err
	}

	// Substitute the "common" field for the sample to get the descriptions that
	// were generated for it.
	props.Properties["spec"].Properties["template"].Properties["spec"].Properties["common"] =
		sampleProps.Properties["spec"].Properties["common"]

	crd := getCRD(props, id, machineAPIGroup)

	crdObj, err := crd.ToCustomResourceDefinition()
	if err != nil {
		return nil, err
	}

	return crdObj, nil
}

func getMachineCRD(schemas *schemas.Schemas, sampleProps *apiextv1.JSONSchemaProps, id string) (runtime.Object, error) {
	props, err := openapi.ToOpenAPI(id, schemas)
	if err != nil {
		return nil, err
	}

	// Substitute the "common" and "status" fields for the sample to get the descriptions that
	// were generated for it.
	props.Properties["spec"].Properties["common"] = sampleProps.Properties["spec"].Properties["common"]
	props.Properties["status"] = sampleProps.Properties["status"]

	crd := getCRD(props, id, machineAPIGroup)

	crdObj, err := crd.ToCustomResourceDefinition()
	if err != nil {
		return nil, err
	}

	return crdObj, nil
}

func (h *handler) OnChange(obj *v3.DynamicSchema, status v3.DynamicSchemaStatus) ([]runtime.Object, v3.DynamicSchemaStatus, error) {
	if obj.Name == "nodetemplateconfig" {
		all, err := h.schemaCache.List(labels.Everything())
		if err != nil {
			return nil, status, err
		}
		for _, schema := range all {
			if schema.Name == "nodetemplateconfig" {
				continue
			}
			h.schemasController.Enqueue(schema.Name)
		}
	}

	name, node, _, err := h.getStyle(obj.Name)
	if err != nil {
		return nil, status, err
	}

	if !node { // only support nodes right now  && !cluster {
		return nil, status, nil
	}

	nodeConfigID, templateID, machineID, schemas, err := getSchemas(name, &obj.Spec)
	if err != nil {
		return nil, status, err
	}

	var result []runtime.Object

	crdObj, err := getConfigCRD(schemas, nodeConfigID)
	if err != nil {
		return nil, status, err
	}

	result = append(result, crdObj)

	crdObj, err = getMachineTemplateCRD(schemas, h.sampleProps, templateID)
	if err != nil {
		return nil, status, err
	}

	result = append(result, crdObj)

	crdObj, err = getMachineCRD(schemas, h.sampleProps, machineID)
	if err != nil {
		return nil, status, err
	}

	result = append(result, crdObj)

	return result, status, nil
}

func (h *handler) getStyle(name string) (string, bool, bool, error) {
	if !strings.HasSuffix(name, "config") {
		return "", false, false, nil
	}

	for _, typeName := range []string{"nodetemplateconfig", "cluster"} {
		schema, err := h.schemaCache.Get(typeName)
		if apierror.IsNotFound(err) {
			continue
		} else if err != nil {
			return "", false, false, err
		}
		for key := range schema.Spec.ResourceFields {
			if strings.EqualFold(key, name) {
				return strings.TrimSuffix(key, "Config"),
					typeName == "nodetemplateconfig",
					typeName == "cluster",
					nil
			}
		}
	}

	return "", false, false, nil
}
