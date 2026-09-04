package projects

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
	steveschema "github.com/rancher/steve/pkg/schema"
	"github.com/rancher/wrangler/v3/pkg/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// projectSchemaID is what GVKToSchemaID yields for the project schema built in newSchemas.
const projectSchemaID = "management.cattle.io.project"

type fakeFactory struct {
	schemas *types.APISchemas
	err     error
	gotUser user.Info
}

func (f *fakeFactory) Schemas(u user.Info) (*types.APISchemas, error) {
	f.gotUser = u
	return f.schemas, f.err
}
func (f *fakeFactory) ByGVR(k8sschema.GroupVersionResource) string { return "" }
func (f *fakeFactory) ByGVK(k8sschema.GroupVersionKind) string     { return "" }
func (f *fakeFactory) OnChange(context.Context, func())            {}
func (f *fakeFactory) AddTemplate(...steveschema.Template)         {}

// fakeStore records the schema steveStore resolved before delegating.
type fakeStore struct {
	gotSchema *types.APISchema
}

func (f *fakeStore) ByID(_ *types.APIRequest, s *types.APISchema, _ string) (types.APIObject, error) {
	f.gotSchema = s
	return types.APIObject{}, nil
}
func (f *fakeStore) List(_ *types.APIRequest, s *types.APISchema) (types.APIObjectList, error) {
	f.gotSchema = s
	return types.APIObjectList{}, nil
}
func (f *fakeStore) Create(_ *types.APIRequest, s *types.APISchema, _ types.APIObject) (types.APIObject, error) {
	f.gotSchema = s
	return types.APIObject{}, nil
}
func (f *fakeStore) Update(_ *types.APIRequest, s *types.APISchema, _ types.APIObject, _ string) (types.APIObject, error) {
	f.gotSchema = s
	return types.APIObject{}, nil
}
func (f *fakeStore) Delete(_ *types.APIRequest, s *types.APISchema, _ string) (types.APIObject, error) {
	f.gotSchema = s
	return types.APIObject{}, nil
}
func (f *fakeStore) Watch(_ *types.APIRequest, s *types.APISchema, _ types.WatchRequest) (chan types.APIEvent, error) {
	f.gotSchema = s
	return nil, nil
}

// localSchema is the schema steve hands the store: it carries the GVK but no store of its own.
func localSchema() *types.APISchema {
	s := &types.APISchema{Schema: &schemas.Schema{ID: "project"}}
	attributes.SetGroup(s, "management.cattle.io")
	attributes.SetVersion(s, "v3")
	attributes.SetKind(s, "Project")
	return s
}

func steveSchemas(id string, store types.Store) *types.APISchemas {
	all := types.EmptyAPISchemas()
	all.Schemas[id] = &types.APISchema{Schema: &schemas.Schema{ID: id}, Store: store}
	return all
}

func apiRequest(withUser bool) *types.APIRequest {
	req := httptest.NewRequest("GET", "/v1/management.cattle.io.projects", nil)
	if withUser {
		req = req.WithContext(request.WithUser(req.Context(), &user.DefaultInfo{Name: "alice"}))
	}
	return &types.APIRequest{Request: req}
}

func TestSteveStoreSchemaFor(t *testing.T) {
	tests := []struct {
		name     string
		withUser bool
		factory  *fakeFactory
		wantErr  string
	}{
		{
			name:     "no user in request",
			withUser: false,
			factory:  &fakeFactory{schemas: steveSchemas(projectSchemaID, &fakeStore{})},
			wantErr:  "could not find user in request",
		},
		{
			name:     "factory error is propagated",
			withUser: true,
			factory:  &fakeFactory{err: assert.AnError},
			wantErr:  assert.AnError.Error(),
		},
		{
			name:     "steve has no schema for the gvk",
			withUser: true,
			factory:  &fakeFactory{schemas: types.EmptyAPISchemas()},
			wantErr:  "no store for " + projectSchemaID,
		},
		{
			name:     "steve schema has no store",
			withUser: true,
			factory:  &fakeFactory{schemas: steveSchemas(projectSchemaID, nil)},
			wantErr:  "no store for " + projectSchemaID,
		},
		{
			name:     "resolves the steve schema for the gvk",
			withUser: true,
			factory:  &fakeFactory{schemas: steveSchemas(projectSchemaID, &fakeStore{})},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newSteveStore(tt.factory).schemaFor(apiRequest(tt.withUser), localSchema())

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.factory.schemas.LookupSchema(projectSchemaID), got)
			// The schemas must be the ones the requesting user is allowed to see.
			assert.Equal(t, "alice", tt.factory.gotUser.GetName())
		})
	}
}

// TestSteveStoreDelegates covers every store method: on success it forwards to the
// steve schema's store, and on a schemaFor failure it returns the error instead.
func TestSteveStoreDelegates(t *testing.T) {
	calls := map[string]func(*steveStore, *types.APIRequest, *types.APISchema) error{
		"ByID": func(st *steveStore, op *types.APIRequest, s *types.APISchema) error {
			_, err := st.ByID(op, s, "p-1")
			return err
		},
		"List": func(st *steveStore, op *types.APIRequest, s *types.APISchema) error {
			_, err := st.List(op, s)
			return err
		},
		"Create": func(st *steveStore, op *types.APIRequest, s *types.APISchema) error {
			_, err := st.Create(op, s, types.APIObject{})
			return err
		},
		"Update": func(st *steveStore, op *types.APIRequest, s *types.APISchema) error {
			_, err := st.Update(op, s, types.APIObject{}, "p-1")
			return err
		},
		"Delete": func(st *steveStore, op *types.APIRequest, s *types.APISchema) error {
			_, err := st.Delete(op, s, "p-1")
			return err
		},
		"Watch": func(st *steveStore, op *types.APIRequest, s *types.APISchema) error {
			_, err := st.Watch(op, s, types.WatchRequest{})
			return err
		},
	}

	for name, call := range calls {
		t.Run(name+" forwards to the steve store", func(t *testing.T) {
			backing := &fakeStore{}
			factory := &fakeFactory{schemas: steveSchemas(projectSchemaID, backing)}

			require.NoError(t, call(newSteveStore(factory), apiRequest(true), localSchema()))
			// The backing store is called with steve's schema, not the local one.
			assert.Equal(t, factory.schemas.LookupSchema(projectSchemaID), backing.gotSchema)
		})

		t.Run(name+" returns schema lookup errors", func(t *testing.T) {
			factory := &fakeFactory{err: assert.AnError}

			err := call(newSteveStore(factory), apiRequest(true), localSchema())
			assert.ErrorIs(t, err, assert.AnError)
		})
	}
}
