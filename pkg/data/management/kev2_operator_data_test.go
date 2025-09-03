package management

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var fakeKontinerDriverLister = fakes.KontainerDriverListerMock{}

func Test_syncKEv2OperatorsSetting(t *testing.T) {
	defaultNotFoundErr := errors.NewNotFound(schema.GroupResource{}, "")
	genericErr := fmt.Errorf("generic error")

	defaultOps := make([]KEv2OperatorInfo, len(defaultKEv2Operators))
	copy(defaultOps, defaultKEv2Operators)
	defaultJSON := getSortedOperatorsJSON(t, defaultOps)

	tests := []struct {
		name            string
		initialSetting  string
		driversLister   *fakes.KontainerDriverListerMock
		expectedSetting string
		wantErr         bool
	}{
		{
			name:           "empty setting",
			initialSetting: "",
			driversLister: &fakes.KontainerDriverListerMock{
				GetFunc: func(namespace, name string) (*v3.KontainerDriver, error) {
					return nil, defaultNotFoundErr
				},
			},
			expectedSetting: defaultJSON,
		},
		{
			name:           "invalid json setting",
			initialSetting: `{"invalid": "json"`,
			driversLister: &fakes.KontainerDriverListerMock{
				GetFunc: func(namespace, name string) (*v3.KontainerDriver, error) {
					return nil, defaultNotFoundErr
				},
			},
			expectedSetting: defaultJSON,
		},
		{
			name: "missing default operator",
			initialSetting: func() string {
				b, _ := json.Marshal([]KEv2OperatorInfo{{Name: "eks", Active: true}})
				return string(b)
			}(),
			driversLister: &fakes.KontainerDriverListerMock{
				GetFunc: func(namespace, name string) (*v3.KontainerDriver, error) {
					if name == "amazonelasticcontainerservice" {
						return &v3.KontainerDriver{Spec: v3.KontainerDriverSpec{Active: true}}, nil
					}
					return nil, defaultNotFoundErr
				},
			},
			expectedSetting: func() string {
				ops := []KEv2OperatorInfo{
					{Name: AKSOperator, Active: true},
					{Name: EKSOperator, Active: true},
					{Name: GKEOperator, Active: true},
					{Name: AlibabaOperator, Active: false},
				}
				return getSortedOperatorsJSON(t, ops)
			}(),
		},
		{
			name:           "update active from old driver (active)",
			initialSetting: "",
			driversLister: &fakes.KontainerDriverListerMock{
				GetFunc: func(namespace, name string) (*v3.KontainerDriver, error) {
					if name == "amazonelasticcontainerService" {
						return &v3.KontainerDriver{Spec: v3.KontainerDriverSpec{Active: true}}, nil
					}
					return nil, defaultNotFoundErr
				},
			},
			expectedSetting: func() string {
				ops := make([]KEv2OperatorInfo, len(defaultKEv2Operators))
				copy(ops, defaultKEv2Operators)
				for i := range ops {
					if ops[i].Name == EKSOperator {
						ops[i].Active = true
					}
				}
				return getSortedOperatorsJSON(t, ops)
			}(),
		},
		{
			name:           "update active from old driver (inactive)",
			initialSetting: `[{"name":"aks","active":true},{"name":"eks","active":true},{"name":"gke","active":true},{"name":"alibaba","active":false}]`,
			driversLister: &fakes.KontainerDriverListerMock{
				GetFunc: func(namespace, name string) (*v3.KontainerDriver, error) {
					if name == "azurekubernetesservice" {
						return &v3.KontainerDriver{Spec: v3.KontainerDriverSpec{Active: false}}, nil
					}
					return nil, defaultNotFoundErr
				},
			},
			expectedSetting: func() string {
				ops := make([]KEv2OperatorInfo, len(defaultKEv2Operators))
				copy(ops, defaultKEv2Operators)
				for i := range ops {
					if ops[i].Name == AKSOperator {
						ops[i].Active = false
					}
				}
				return getSortedOperatorsJSON(t, ops)
			}(),
		},
		{
			name:           "lister returns error",
			initialSetting: "",
			driversLister: &fakes.KontainerDriverListerMock{
				GetFunc: func(namespace, name string) (*v3.KontainerDriver, error) {
					return nil, genericErr
				},
			},
			wantErr: true,
		},
		{
			name:           "no changes needed",
			initialSetting: defaultJSON,
			driversLister: &fakes.KontainerDriverListerMock{
				GetFunc: func(namespace, name string) (*v3.KontainerDriver, error) {
					return nil, defaultNotFoundErr
				},
			},
			expectedSetting: defaultJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalSetting := settings.KEv2Operators.Get()
			settings.KEv2Operators.Set(tt.initialSetting)
			t.Cleanup(func() {
				settings.KEv2Operators.Set(originalSetting)
			})

			err := syncKEv2OperatorsSetting(tt.driversLister)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.JSONEq(t, tt.expectedSetting, settings.KEv2Operators.Get())
			}
		})
	}
}

func getSortedOperatorsJSON(t *testing.T, ops []KEv2OperatorInfo) string {
	t.Helper()
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].Name < ops[j].Name
	})
	b, err := json.Marshal(ops)
	require.NoError(t, err)
	return string(b)
}
