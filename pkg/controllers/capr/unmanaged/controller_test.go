package unmanaged

import (
	"errors"
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func provisionedCustomMachine() *rkev1.CustomMachine {
	customMachine := &rkev1.CustomMachine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fleet-default",
			Name:      "custom-0123456789ab",
			Labels:    map[string]string{capi.ClusterNameLabel: "test-cluster"},
		},
		Spec: rkev1.CustomMachineSpec{
			ProviderID: "aws:///us-west-2a/i-0123456789abcdef0",
		},
	}
	customMachine.Status.Initialization.Provisioned = ptr.To(true)
	capr.Ready.SetStatus(customMachine, "True")
	return customMachine
}

func TestOnUnmanagedMachineChange(t *testing.T) {
	tests := []struct {
		name       string
		getErr     error
		wantErr    bool
		rkeCluster *rkev1.RKECluster
	}{
		{
			// A cluster whose rkeConfig.infrastructureRef points to a non-Rancher
			// infrastructure cluster has no RKECluster.
			name:   "missing RKECluster is not an error",
			getErr: apierror.NewNotFound(schema.GroupResource{Group: "rke.cattle.io", Resource: "rkeclusters"}, "test-cluster"),
		},
		{
			name:    "other errors are returned",
			getErr:  errors.New("cache not synced"),
			wantErr: true,
		},
		{
			name:       "RKECluster without the deletion annotation",
			rkeCluster: &rkev1.RKECluster{ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "test-cluster"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rkeClusterCache := ctrlfake.NewMockCacheInterface[*rkev1.RKECluster](ctrl)
			rkeClusterCache.EXPECT().Get("fleet-default", "test-cluster").Return(tt.rkeCluster, tt.getErr)

			h := &handler{rkeClusterCache: rkeClusterCache}
			customMachine := provisionedCustomMachine()

			got, err := h.onUnmanagedMachineChange("", customMachine)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Same(t, customMachine, got)
		})
	}
}
