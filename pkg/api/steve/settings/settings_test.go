package settings

import (
	"errors"
	"testing"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
)

type fakeStore struct {
	byIDObject   types.APIObject
	byIDErr      error
	updateObject types.APIObject
	updateErr    error
	updateCalled bool
	deleteObject types.APIObject
	deleteErr    error
	createObject types.APIObject
	createErr    error
	listObject   types.APIObjectList
	listErr      error
	watchChan    chan types.APIEvent
	watchErr     error
}

func (f *fakeStore) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	return f.byIDObject, f.byIDErr
}

func (f *fakeStore) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	return f.listObject, f.listErr
}

func (f *fakeStore) Create(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	return f.createObject, f.createErr
}

func (f *fakeStore) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	f.updateCalled = true
	return f.updateObject, f.updateErr
}

func (f *fakeStore) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	return f.deleteObject, f.deleteErr
}

func (f *fakeStore) Watch(apiOp *types.APIRequest, schema *types.APISchema, w types.WatchRequest) (chan types.APIEvent, error) {
	return f.watchChan, f.watchErr
}

func TestSettingsStoreUpdateValidation(t *testing.T) {
	tests := []struct {
		name           string
		id             string
		current        types.APIObject
		update         types.APIObject
		expectedCode   validation.ErrorCode
		expectedCalled bool
	}{
		{
			name: "env backed setting is read only",
			id:   "server-url",
			current: types.APIObject{Object: map[string]interface{}{
				"source": "env",
			}},
			update:         types.APIObject{Object: map[string]interface{}{"value": "https://example.com"}},
			expectedCode:   validation.MethodNotAllowed,
			expectedCalled: false,
		},
		{
			name: "read only setting is rejected",
			id:   "cacerts",
			current: types.APIObject{Object: map[string]interface{}{
				"source": "db",
			}},
			update:         types.APIObject{Object: map[string]interface{}{"value": "new-value"}},
			expectedCode:   validation.MethodNotAllowed,
			expectedCalled: false,
		},
		{
			name: "invalid max age is rejected",
			id:   "auth-user-info-max-age-seconds",
			current: types.APIObject{Object: map[string]interface{}{
				"source": "db",
			}},
			update:         types.APIObject{Object: map[string]interface{}{"value": "not-a-duration"}},
			expectedCode:   validation.InvalidBodyContent,
			expectedCalled: false,
		},
		{
			name: "invalid cron is rejected",
			id:   "auth-user-info-resync-cron",
			current: types.APIObject{Object: map[string]interface{}{
				"source": "db",
			}},
			update:         types.APIObject{Object: map[string]interface{}{"value": "invalid cron"}},
			expectedCode:   validation.InvalidBodyContent,
			expectedCalled: false,
		},
		{
			name: "valid update is delegated",
			id:   "auth-user-info-max-age-seconds",
			current: types.APIObject{Object: map[string]interface{}{
				"source": "db",
			}},
			update:         types.APIObject{Object: map[string]interface{}{"value": "3600"}},
			expectedCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{
				byIDObject:   tt.current,
				updateObject: types.APIObject{Object: map[string]interface{}{"value": "ok"}},
			}
			settingsStore := &settingsStore{Store: store}

			_, err := settingsStore.Update(nil, nil, tt.update, tt.id)
			if tt.expectedCode == (validation.ErrorCode{}) {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			} else {
				var apiErr *apierror.APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected APIError, got %v", err)
				}
				if apiErr.Code != tt.expectedCode {
					t.Fatalf("expected code %v, got %v", tt.expectedCode, apiErr.Code)
				}
			}

			if store.updateCalled != tt.expectedCalled {
				t.Fatalf("expected updateCalled=%v, got %v", tt.expectedCalled, store.updateCalled)
			}
		})
	}
}
