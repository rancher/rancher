package management

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// mockMgmtInterface embeds the real interface and only overrides ProxyEndpoint()
type mockMgmtInterface struct {
	managementv3.Interface
	proxyEndpointController *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]
}

func (m *mockMgmtInterface) ProxyEndpoint() managementv3.ProxyEndpointController {
	return m.proxyEndpointController
}

// setupMockContext creates a mock wrangler.Context with the provided ProxyEndpointController
func setupMockContext(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) *wrangler.Context {
	return &wrangler.Context{
		Mgmt: &mockMgmtInterface{
			proxyEndpointController: controller,
		},
	}
}

func TestAddProxyEndpointData(t *testing.T) {
	tests := []struct {
		name                     string
		disabledEndpointsSetting string
		setup                    func(*fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList])
		wantErr                  bool
	}{
		{
			name:                     "empty setting creates all endpoints",
			disabledEndpointsSetting: "",
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				// All endpoints don't exist, so they should be created
				controller.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errors.NewNotFound(schema.GroupResource{}, "")).Times(3)
				controller.EXPECT().Create(gomock.Any()).Return(&v3.ProxyEndpoint{}, nil).Times(3)
			},
		},
		{
			name:                     "all disabled creates no endpoints",
			disabledEndpointsSetting: "all",
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				// All endpoints exist, so they should be deleted
				controller.EXPECT().Get(gomock.Any(), gomock.Any()).Return(&v3.ProxyEndpoint{}, nil).Times(3)
				controller.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil).Times(3)
			},
		},
		{
			name:                     "specific endpoint disabled",
			disabledEndpointsSetting: "rancher-aws-endpoints",
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				// AWS endpoint exists and should be deleted
				controller.EXPECT().Get("rancher-aws-endpoints", gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
				controller.EXPECT().Delete("rancher-aws-endpoints", gomock.Any()).Return(nil)
				// Other endpoints don't exist and should be created
				controller.EXPECT().Get("rancher-digitalocean-endpoints", gomock.Any()).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				controller.EXPECT().Create(gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
				controller.EXPECT().Get("rancher-linode-endpoints", gomock.Any()).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				controller.EXPECT().Create(gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
			},
		},
		{
			name:                     "comma-separated disabled list",
			disabledEndpointsSetting: "rancher-aws-endpoints,rancher-digitalocean-endpoints",
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				// AWS and DigitalOcean endpoints exist and should be deleted
				controller.EXPECT().Get("rancher-aws-endpoints", gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
				controller.EXPECT().Delete("rancher-aws-endpoints", gomock.Any()).Return(nil)
				controller.EXPECT().Get("rancher-digitalocean-endpoints", gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
				controller.EXPECT().Delete("rancher-digitalocean-endpoints", gomock.Any()).Return(nil)
				// Linode endpoint doesn't exist and should be created
				controller.EXPECT().Get("rancher-linode-endpoints", gomock.Any()).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				controller.EXPECT().Create(gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
			},
		},
		{
			name:                     "comma-separated disabled list with invalid entry",
			disabledEndpointsSetting: "rancher-aws-endpoints,not-a-endpoint,nonsense,rancher-digitalocean-endpoints",
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				// AWS and DigitalOcean endpoints exist and should be deleted
				controller.EXPECT().Get("rancher-aws-endpoints", gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
				controller.EXPECT().Delete("rancher-aws-endpoints", gomock.Any()).Return(nil)
				controller.EXPECT().Get("rancher-digitalocean-endpoints", gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
				controller.EXPECT().Delete("rancher-digitalocean-endpoints", gomock.Any()).Return(nil)
				// Linode endpoint doesn't exist and should be created
				controller.EXPECT().Get("rancher-linode-endpoints", gomock.Any()).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				controller.EXPECT().Create(gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			controller := fake.NewMockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList](ctrl)
			if tt.setup != nil {
				tt.setup(controller)
			}

			ctx := setupMockContext(controller)
			err := AddProxyEndpointData(tt.disabledEndpointsSetting, ctx)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateOrDisableEndpoint(t *testing.T) {
	testEndpoint := v3.ProxyEndpoint{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-endpoint",
		},
		Spec: v3.ProxyEndpointSpec{
			Routes: []v3.ProxyEndpointRoute{
				{Domain: "test.example.com"},
			},
		},
	}

	tests := []struct {
		name     string
		endpoint v3.ProxyEndpoint
		disabled bool
		setup    func(*fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList])
		wantErr  bool
	}{
		{
			name:     "creates endpoint when disabled=false and not exists",
			endpoint: testEndpoint,
			disabled: false,
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				controller.EXPECT().Get("test-endpoint", gomock.Any()).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				controller.EXPECT().Create(gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
			},
		},
		{
			name:     "skips creation when endpoint exists",
			endpoint: testEndpoint,
			disabled: false,
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				controller.EXPECT().Get("test-endpoint", gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
			},
		},
		{
			name:     "deletes endpoint when disabled=true and exists",
			endpoint: testEndpoint,
			disabled: true,
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				controller.EXPECT().Get("test-endpoint", gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
				controller.EXPECT().Delete("test-endpoint", gomock.Any()).Return(nil)
			},
		},
		{
			name:     "skips deletion when endpoint not exists",
			endpoint: testEndpoint,
			disabled: true,
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				controller.EXPECT().Get("test-endpoint", gomock.Any()).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
			},
		},
		{
			name:     "handles AlreadyExists error gracefully on create",
			endpoint: testEndpoint,
			disabled: false,
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				controller.EXPECT().Get("test-endpoint", gomock.Any()).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				controller.EXPECT().Create(gomock.Any()).Return(nil, errors.NewAlreadyExists(schema.GroupResource{}, "test-endpoint"))
			},
		},
		{
			name:     "handles NotFound error gracefully on delete",
			endpoint: testEndpoint,
			disabled: true,
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				controller.EXPECT().Get("test-endpoint", gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
				controller.EXPECT().Delete("test-endpoint", gomock.Any()).Return(errors.NewNotFound(schema.GroupResource{}, "test-endpoint"))
			},
		},
		{
			name:     "returns error when Get fails",
			endpoint: testEndpoint,
			disabled: false,
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				controller.EXPECT().Get("test-endpoint", gomock.Any()).Return(nil, fmt.Errorf("get error"))
			},
			wantErr: true,
		},
		{
			name:     "returns error when Create fails with unexpected error",
			endpoint: testEndpoint,
			disabled: false,
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				controller.EXPECT().Get("test-endpoint", gomock.Any()).Return(nil, errors.NewNotFound(schema.GroupResource{}, ""))
				controller.EXPECT().Create(gomock.Any()).Return(nil, fmt.Errorf("create error"))
			},
			wantErr: true,
		},
		{
			name:     "returns error when Delete fails with unexpected error",
			endpoint: testEndpoint,
			disabled: true,
			setup: func(controller *fake.MockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList]) {
				controller.EXPECT().Get("test-endpoint", gomock.Any()).Return(&v3.ProxyEndpoint{}, nil)
				controller.EXPECT().Delete("test-endpoint", gomock.Any()).Return(fmt.Errorf("delete error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			controller := fake.NewMockNonNamespacedControllerInterface[*v3.ProxyEndpoint, *v3.ProxyEndpointList](ctrl)
			if tt.setup != nil {
				tt.setup(controller)
			}

			ctx := setupMockContext(controller)
			err := createOrDisableEndpoint(tt.endpoint, tt.disabled, ctx)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
