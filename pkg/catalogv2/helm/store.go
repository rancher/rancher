package helm

import (
	"strings"

	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/stores/partition"
	"github.com/rancher/steve/pkg/stores/selector"
	"github.com/rancher/steve/pkg/stores/switchschema"
	"github.com/rancher/wrangler/v2/pkg/schemas/validation"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	// configMap2 target representing a configmap v2
	configMap2 = target{
		schemaType: "configmap",
		version:    "2",
		selector: labels.SelectorFromSet(labels.Set{
			"OWNER": "TILLER",
		}),
	}
	// secret2 target representing a secret v2
	secret2 = target{
		schemaType: "secret",
		version:    "2",
		selector: labels.SelectorFromSet(labels.Set{
			"OWNER": "TILLER",
		}),
	}
	// secret3 target representing a secret v3
	secret3 = target{
		schemaType: "secret",
		version:    "3",
		selector: labels.SelectorFromSet(labels.Set{
			"owner": "helm",
		}),
	}
	// all Slice of partition.Partition containing configMap2, secret2 and secret3
	all = []partition.Partition{
		configMap2,
		secret2,
		secret3,
	}
)

// target represents a schema. Implementation of the partition.Partition interface
type target struct {
	schemaType string          // the type of the schema
	version    string          // the version of the schema
	selector   labels.Selector // A selector that matches labels
}

// Name returns the name of the target
func (t target) Name() string {
	return t.schemaType + t.version
}

// partitioner
type partitioner struct {
}

// Lookup validates and return the partition.Partition for the received id
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

// All returns a list of partition.Partition containing the schemas for
// a configmap v2, secret v2 and secret v3
func (p *partitioner) All(apiOp *types.APIRequest, schema *types.APISchema, verb, id string) ([]partition.Partition, error) {
	return all, nil
}

// Store returns an implementation of types.Store for the received apiOp and partition
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

// stripIDPrefix Implementation of the types.Store interface
type stripIDPrefix struct {
	types.Store
}

// stripPrefix removes the prefixes c: and s: from the given string
func stripPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "c:"), "s:")
}

// Delete removes from the store the object with the given id
func (s *stripIDPrefix) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	return s.Store.Delete(apiOp, schema, stripPrefix(id))
}

// ByID gets from the store the object with the given id
func (s *stripIDPrefix) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	return s.Store.ByID(apiOp, schema, stripPrefix(id))
}

// Update updates an object in the store with the given id using the given data
func (s *stripIDPrefix) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	return s.Store.Update(apiOp, schema, data, stripPrefix(id))
}
