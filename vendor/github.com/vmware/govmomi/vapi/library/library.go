/*
Copyright (c) 2018 VMware, Inc. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package library

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/vmware/govmomi/vapi/internal"
	"github.com/vmware/govmomi/vapi/rest"
)

// StorageBackings for Content Libraries
type StorageBackings struct {
	DatastoreID string `json:"datastore_id,omitempty"`
	Type        string `json:"type,omitempty"`
}

// Library  provides methods to create, read, update, delete, and enumerate libraries.
type Library struct {
	CreationTime     *time.Time        `json:"creation_time,omitempty"`
	Description      string            `json:"description,omitempty"`
	ID               string            `json:"id,omitempty"`
	LastModifiedTime *time.Time        `json:"last_modified_time,omitempty"`
	Name             string            `json:"name,omitempty"`
	Storage          []StorageBackings `json:"storage_backings,omitempty"`
	Type             string            `json:"type,omitempty"`
	Version          string            `json:"version,omitempty"`
	Subscription     *Subscription     `json:"subscription_info,omitempty"`
}

// Subscription info
type Subscription struct {
	AuthenticationMethod string `json:"authentication_method"`
	AutomaticSyncEnabled *bool  `json:"automatic_sync_enabled,omitempty"`
	OnDemand             *bool  `json:"on_demand,omitempty"`
	Password             string `json:"password,omitempty"`
	SslThumbprint        string `json:"ssl_thumbprint,omitempty"`
	SubscriptionURL      string `json:"subscription_url,omitempty"`
	UserName             string `json:"user_name,omitempty"`
}

// Patch merges updates from the given src.
func (l *Library) Patch(src *Library) {
	if src.Name != "" {
		l.Name = src.Name
	}
	if src.Description != "" {
		l.Description = src.Description
	}
	if src.Version != "" {
		l.Version = src.Version
	}
}

// Manager extends rest.Client, adding content library related methods.
type Manager struct {
	*rest.Client
}

// NewManager creates a new Manager instance with the given client.
func NewManager(client *rest.Client) *Manager {
	return &Manager{
		Client: client,
	}
}

// Find is the search criteria for finding libraries.
type Find struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// FindLibrary returns one or more libraries that match the provided search
// criteria.
//
// The provided name is case-insensitive.
//
// Either the name or type of library may be set to empty values in order
// to search for all libraries, all libraries with a specific name, regardless
// of type, or all libraries of a specified type.
func (c *Manager) FindLibrary(ctx context.Context, search Find) ([]string, error) {
	url := c.Resource(internal.LibraryPath).WithAction("find")
	spec := struct {
		Spec Find `json:"spec"`
	}{search}
	var res []string
	return res, c.Do(ctx, url.Request(http.MethodPost, spec), &res)
}

// CreateLibrary creates a new library with the given Type, Name,
// Description, and CategoryID.
func (c *Manager) CreateLibrary(ctx context.Context, library Library) (string, error) {
	spec := struct {
		Library Library `json:"create_spec"`
	}{library}
	path := internal.LocalLibraryPath
	if library.Type == "SUBSCRIBED" {
		path = internal.SubscribedLibraryPath
	}
	url := c.Resource(path)
	var res string
	return res, c.Do(ctx, url.Request(http.MethodPost, spec), &res)
}

// DeleteLibrary deletes an existing library.
func (c *Manager) DeleteLibrary(ctx context.Context, library *Library) error {
	path := internal.LocalLibraryPath
	if library.Type == "SUBSCRIBED" {
		path = internal.SubscribedLibraryPath
	}
	url := c.Resource(path).WithID(library.ID)
	return c.Do(ctx, url.Request(http.MethodDelete), nil)
}

// ListLibraries returns a list of all content library IDs in the system.
func (c *Manager) ListLibraries(ctx context.Context) ([]string, error) {
	url := c.Resource(internal.LibraryPath)
	var res []string
	return res, c.Do(ctx, url.Request(http.MethodGet), &res)
}

// GetLibraryByID returns information on a library for the given ID.
func (c *Manager) GetLibraryByID(ctx context.Context, id string) (*Library, error) {
	url := c.Resource(internal.LibraryPath).WithID(id)
	var res Library
	return &res, c.Do(ctx, url.Request(http.MethodGet), &res)
}

// GetLibraryByName returns information on a library for the given name.
func (c *Manager) GetLibraryByName(ctx context.Context, name string) (*Library, error) {
	// Lookup by name
	libraries, err := c.GetLibraries(ctx)
	if err != nil {
		return nil, err
	}
	for i := range libraries {
		if libraries[i].Name == name {
			return &libraries[i], nil
		}
	}
	return nil, fmt.Errorf("library name (%s) not found", name)
}

// GetLibraries returns a list of all content library details in the system.
func (c *Manager) GetLibraries(ctx context.Context) ([]Library, error) {
	ids, err := c.ListLibraries(ctx)
	if err != nil {
		return nil, fmt.Errorf("get libraries failed for: %s", err)
	}

	var libraries []Library
	for _, id := range ids {
		library, err := c.GetLibraryByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get library %s failed for %s", id, err)
		}

		libraries = append(libraries, *library)

	}
	return libraries, nil
}
