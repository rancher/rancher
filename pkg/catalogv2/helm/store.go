package helm

import (
	"strings"

	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/stores/partition"
	"github.com/rancher/steve/pkg/stores/selector"
	"github.com/rancher/steve/pkg/stores/switchschema"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	configMap2 = target{
		schemaType: "configmap",
		version:    "2",
		selector: labels.SelectorFromSet(labels.Set{
			"OWNER": "TILLER",
		}),
	}
	secret2 = target{
		schemaType: "secret",
		version:    "2",
		selector: labels.SelectorFromSet(labels.Set{
			"OWNER": "TILLER",
		}),
	}
	secret3 = target{
		schemaType: "secret",
		version:    "3",
		selector: labels.SelectorFromSet(labels.Set{
			"owner": "helm",
		}),
	}
	all = []partition.Partition{
		configMap2,
		secret2,
		secret3,
	}
)

type target struct {
	schemaType string
	version    string
	selector   labels.Selector
}

func (t target) Name() string {
	return t.schemaType + t.version
}

type partitioner struct {
}

func (p *partitioner) Lookup(apiOp *types.APIRequest, schema *types.APISchema, verb, id string) (partition.Partition, error) {
	if id == "" {
		return nil, validation.Unauthorized
	}
	t := strings.SplitN(id, ":", 2)[0]
	if t == "c" {
		return configMap2, nil
	} else if t == "s" {
		return secret2, nil
	}
	return nil, validation.NotFound
}

func (p *partitioner) All(apiOp *types.APIRequest, schema *types.APISchema, verb, id string) ([]partition.Partition, error) {
	return all, nil
}

func (p *partitioner) Store(apiOp *types.APIRequest, partition partition.Partition) (types.Store, error) {
	target := partition.(target)
	schema := apiOp.Schemas.LookupSchema(target.schemaType)
	if schema == nil {
		return &empty.Store{}, nil
	}
	return &stripIDPrefix{
		Store: &selector.Store{
			Selector: target.selector,
			Store: &switchschema.Store{
				Schema: schema,
			},
		},
	}, nil
}

type stripIDPrefix struct {
	types.Store
}

func stripPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "c:"), "s:")
}

func (s *stripIDPrefix) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	return s.Store.Delete(apiOp, schema, stripPrefix(id))
}

func (s *stripIDPrefix) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	return s.Store.ByID(apiOp, schema, stripPrefix(id))
}

func (s *stripIDPrefix) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	return s.Store.Update(apiOp, schema, data, stripPrefix(id))
}
