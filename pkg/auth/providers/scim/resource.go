package scim

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// SCIM resource types.
const (
	userResource         = "User"
	groupResource        = "Group"
	resourceTypeResource = "ResourceType"
	schemaResource       = "Schema"
)

// SCIM resource endpoints.
const (
	userEndpoint         = "Users"
	groupEndpoint        = "Groups"
	resourceTypeEndpoint = "ResourceTypes"
	schemaEndpoint       = "Schemas"
)

// Meta represents the resource metadata.
type Meta struct {
	// Location is the URI of the resource being returned.
	// This value must be the same as the "Content-Location" HTTP response header
	Location string `json:"location,omitempty"`
	// ResourceType is the name of the resource type of the resource.
	ResourceType string `json:"resourceType,omitempty"`
	// Created is the the "DateTime" that the resource was added to the service
	// provider.  This attribute must be a DateTime.
	Created *time.Time `json:"created,omitempty"`
	// LastModified is the most recent DateTime that the details of this
	// resource were updated at the service provider. If this
	// resource has never been modified since its initial creation,
	// the value MUST be the same as the value of "created".
	LastModified *time.Time `json:"lastModified,omitempty"`
	// Version is the version of the resource being returned.
	// This value must be the same as the entity-tag (ETag) HTTP response header.
	Version string `json:"version,omitempty"`
}

// ResourceType specifies the metadata about a resource type.
type ResourceType struct {
	// ID is the resource type's server unique id. This is often the same value as the "name" attribute.
	ID string `json:"id"`
	// Name is the resource type name. This name is referenced by the "meta.resourceType" attribute in all resources.
	Name string `json:"name"`
	// Description is the resource type's human-readable description.
	Description string `json:"description"`
	// Endpoint is the resource type's HTTP-addressable endpoint relative to the Base URL of the service provider,
	// e.g., "/Users".
	Endpoint string `json:"endpoint"`
	// Schema is the resource type's primary/base schema.
	Schema string `json:"schema"`
	// Schemas is a list of the resource type's supported schemas.
	Schemas []string `json:"schemas"`
	// SchemaExtensions is a list of the resource type's schema extensions.
	// SchemaExtensions []SchemaExtension
	Meta Meta `json:"meta"`
}

// userResourceType defines the SCIM User resource type.
var userResourceType = ResourceType{
	ID:          userResource,
	Name:        userResource,
	Description: "User Account",
	Endpoint:    "/" + userEndpoint,
	Schema:      userSchemaID,
	Schemas:     []string{resourceSchemaID},
	Meta: Meta{
		ResourceType: resourceTypeResource,
	},
}

// groupResourceType defines the SCIM Group resource type.
var groupResourceType = ResourceType{
	ID:          groupResource,
	Name:        groupResource,
	Description: "Group of users",
	Endpoint:    "/" + groupEndpoint,
	Schema:      groupSchemaID,
	Schemas:     []string{resourceSchemaID},
	Meta: Meta{
		ResourceType: resourceTypeResource,
	},
}

// resourceTypeRegistry maps SCIM resource type IDs to their definitions.
var resourceTypeRegistry = map[string]ResourceType{
	userResource:  userResourceType,
	groupResource: groupResourceType,
}

// ListResourceTypes lists supported SCIM resource types.
func (s *SCIMServer) ListResourceTypes(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::ListResourceTypes: url %s", r.URL)

	provider := mux.Vars(r)["provider"]

	response := &listResponse{Schemas: []string{listSchemaID}}
	for _, resourceType := range resourceTypeRegistry {
		resourceType := resourceType
		resourceType.Meta.Location = locationURL(r, provider, resourceTypeEndpoint, resourceType.ID)
		response.Resources = append(response.Resources, resourceType)
	}

	response.TotalResults = len(response.Resources)
	response.ItemsPerPage = response.TotalResults
	if response.TotalResults > 0 {
		response.StartIndex = 1
	}

	writeResponse(w, response)
}

// GetResourceType gets a SCIM resource type by ID.
func (s *SCIMServer) GetResourceType(w http.ResponseWriter, r *http.Request) {
	logrus.Tracef("scim::GetResourceType: url %s", r.URL)

	provider := mux.Vars(r)["provider"]
	id := mux.Vars(r)["id"]

	resourceType, ok := resourceTypeRegistry[id]
	if !ok {
		writeError(w, NewError(http.StatusNotFound, fmt.Sprintf("ResourceType %s not found", id)))
		return
	}
	resourceType.Meta.Location = locationURL(r, provider, resourceTypeEndpoint, resourceType.ID)

	writeResponse(w, resourceType)
}
