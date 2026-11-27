package cluster

import (
	"fmt"
	"testing"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"
)

type networkPolicyStore struct {
	types.Store
	data map[string]interface{}
}

func (s *networkPolicyStore) ByID(_ *types.APIContext, _ *types.Schema, _ string) (map[string]interface{}, error) {
	return s.data, nil
}

func (s *networkPolicyStore) Update(_ *types.APIContext, _ *types.Schema, data map[string]interface{}, _ string) (map[string]interface{}, error) {
	s.data = data
	return data, nil
}

func TestStoreUpdateNetworkPolicy(t *testing.T) {
	for _, config := range []string{"rke2Config", "k3sConfig", ""} {
		t.Run(config, func(t *testing.T) {
			backend := &networkPolicyStore{data: map[string]interface{}{"name": "imported"}}
			store := &Store{
				Store: backend,
				KontainerDriverLister: &fakes.KontainerDriverListerMock{
					ListFunc: func(string, labels.Selector) ([]*v3.KontainerDriver, error) {
						return nil, nil
					},
				},
			}
			// Imported K3s/RKE2 clusters acquire these config fields after detection.
			data := map[string]interface{}{"name": "imported"}
			if config != "" {
				data[config] = map[string]interface{}{"version": "v1.34.1"}
			}
			for _, enabled := range []bool{true, true, false} {
				data["enableNetworkPolicy"] = enabled
				out, err := store.Update(nil, nil, data, "c-imported")
				require.NoError(t, err)
				assert.Equal(t, enabled, out["enableNetworkPolicy"])
				assert.Equal(t, data, backend.data)
				if config != "" {
					assert.Equal(t, map[string]interface{}{"version": "v1.34.1"}, out[config])
				}
			}
		})
	}
}

func TestValidateNetworkFlag(t *testing.T) {
	for _, config := range []string{"rke2Config", "k3sConfig", ""} {
		for _, create := range []bool{true, false} {
			for _, flag := range []interface{}{nil, false, true} {
				t.Run(fmt.Sprintf("config=%s/create=%t/flag=%v", config, create, flag), func(t *testing.T) {
					data := map[string]interface{}{}
					if config != "" {
						data[config] = map[string]interface{}{}
					}
					if flag != nil {
						data["enableNetworkPolicy"] = flag
					}
					require.NoError(t, validateNetworkFlag(data, create))
					want := flag
					if flag == nil && create {
						want = false
					}
					assert.Equal(t, want, data["enableNetworkPolicy"])
					_, exists := data["enableNetworkPolicy"]
					assert.Equal(t, flag != nil || create, exists)
				})
			}
		}
	}
}
