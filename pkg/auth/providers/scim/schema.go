package scim

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// Well known SCIM Schema URNs.
const (
	ListSchemaID     = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	GroupSchemaID    = "urn:ietf:params:scim:schemas:core:2.0:Group"
	UserSchemaID     = "urn:ietf:params:scim:schemas:core:2.0:User"
	ErrorSchemaID    = "urn:ietf:params:scim:api:messages:2.0:Error"
	ResourceSchemaID = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	SchemaSchemaID   = "urn:ietf:params:scim:schemas:core:2.0:Schema"
)

// Schema defines a SCIM schema.
type Schema struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Attributes  []SchemaAttribute `json:"attributes"`
	Schemas     []string          `json:"schemas"`
	Meta        Meta              `json:"meta"`
}

// SchemaAttribute defines an attribute in a SCIM schema.
type SchemaAttribute struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	CaseExact     bool              `json:"caseExact"`
	MultiValued   bool              `json:"multiValued"`
	Description   string            `json:"description"`
	Required      bool              `json:"required"`
	Mutability    string            `json:"mutability"`
	Returned      string            `json:"returned"`
	Uniqueness    string            `json:"uniqueness,omitempty"`
	SubAttributes []SchemaAttribute `json:"subAttributes,omitempty"`
}

// UserSchema defines the SCIM schema for User resources.
var UserSchema = Schema{
	Schemas:     []string{SchemaSchemaID},
	ID:          UserSchemaID,
	Name:        UserResource,
	Description: "User Account",
	Attributes: []SchemaAttribute{
		{
			Name:        "userName",
			Type:        "string",
			MultiValued: false,
			Required:    true,
			Mutability:  "readWrite",
			Returned:    "default",
		},
		{
			Name:        "active",
			Type:        "boolean",
			MultiValued: false,
			Required:    true,
			Mutability:  "readWrite",
			Returned:    "default",
		},
		{
			Name:        "emails",
			Type:        "complex",
			MultiValued: true,
			Mutability:  "readWrite",
			Returned:    "default",
			SubAttributes: []SchemaAttribute{
				{
					Name:        "value",
					Type:        "string",
					MultiValued: false,
					Required:    false,
					Mutability:  "readWrite",
					Returned:    "default",
				},
				{
					Name:        "primary",
					Type:        "boolean",
					MultiValued: false,
					Mutability:  "readWrite",
					Returned:    "default",
				},
			},
		},
	},
	Meta: Meta{
		ResourceType: SchemaResource,
	},
}

// GroupSchema defines the SCIM schema for Group resources.
var GroupSchema = Schema{
	Schemas:     []string{SchemaSchemaID},
	ID:          GroupSchemaID,
	Name:        GroupResource,
	Description: "Group",
	Attributes: []SchemaAttribute{
		{
			Name:        "displayName",
			Type:        "string",
			MultiValued: false,
			Required:    true,
			Mutability:  "readWrite",
			Returned:    "default",
		},
		{
			Name:        "members",
			Type:        "complex",
			MultiValued: true,
			Mutability:  "readWrite",
			Returned:    "default",
			SubAttributes: []SchemaAttribute{
				{
					Name:        "value",
					Type:        "string",
					MultiValued: false,
					Mutability:  "readWrite",
					Returned:    "default"},
				{
					Name:        "$ref",
					Type:        "reference",
					MultiValued: false,
					Mutability:  "readOnly",
					Returned:    "default",
				},
			},
		},
	},
	Meta: Meta{
		ResourceType: SchemaResource,
	},
}

var schemaRegistry = map[string]Schema{
	UserSchema.ID:  UserSchema,
	GroupSchema.ID: GroupSchema,
}

// ListSchemas lists supported SCIM schemas.
func (s *SCIMServer) ListSchemas(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::ListSchemas: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]

	response := &ListResponse{Schemas: []string{ListSchemaID}}
	for _, schema := range schemaRegistry {
		schema.Meta.Location = locationURL(r, provider, SchemasEndpoint, schema.ID)
		response.Resources = append(response.Resources, schema)
	}

	response.TotalResults = len(response.Resources)
	response.ItemsPerPage = response.TotalResults
	if response.TotalResults > 0 {
		response.StartIndex = 1
	}

	writeResponse(w, response)
}

// GetSchema retrieves a specific SCIM schema by ID.
func (s *SCIMServer) GetSchema(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::GetSchemas: url %s", r.URL.String())

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	schema, ok := schemaRegistry[id]
	if !ok {
		writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("Schema %s not found", id)))
		return
	}
	schema.Meta.Location = locationURL(r, provider, SchemasEndpoint, schema.ID)

	writeResponse(w, schema)
}
