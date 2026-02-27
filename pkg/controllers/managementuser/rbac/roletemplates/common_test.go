package roletemplates

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_isRoleTemplateExternal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		rtName  string
		getFunc func() (*v3.RoleTemplate, error)
		want    bool
		wantErr bool
	}{
		{
			name:   "error getting role template",
			rtName: "test-rt",
			getFunc: func() (*v3.RoleTemplate, error) {
				return nil, errDefault
			},
			want:    false,
			wantErr: true,
		},
		{
			name:   "role template is nil",
			rtName: "test-rt",
			getFunc: func() (*v3.RoleTemplate, error) {
				return nil, nil
			},
			want:    false,
			wantErr: true,
		},
		{
			name:   "role template not found",
			rtName: "test-rt",
			getFunc: func() (*v3.RoleTemplate, error) {
				return nil, errNotFound
			},
			want:    false,
			wantErr: false,
		},
		{
			name:   "external role template found",
			rtName: "test-rt",
			getFunc: func() (*v3.RoleTemplate, error) {
				return &v3.RoleTemplate{External: true}, nil
			},
			want:    true,
			wantErr: false,
		},
		{
			name:   "non-external role template found",
			rtName: "test-rt",
			getFunc: func() (*v3.RoleTemplate, error) {
				return &v3.RoleTemplate{External: false}, nil
			},
			want:    false,
			wantErr: false,
		},
	}
	ctrl := gomock.NewController(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rtClient := fake.NewMockNonNamespacedControllerInterface[*v3.RoleTemplate, *v3.RoleTemplateList](ctrl)
			rtClient.EXPECT().Get(tt.rtName, metav1.GetOptions{}).Return(tt.getFunc())
			got, err := isRoleTemplateExternal(tt.rtName, rtClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("isRoleTemplateExternal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isRoleTemplateExternal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddAggregationFeatureLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		inputLabels    map[string]string
		expectedLabels map[string]string
	}{
		{
			name:        "adds label to object with no labels",
			inputLabels: nil,
			expectedLabels: map[string]string{
				AggregationFeatureLabel: "true",
			},
		},
		{
			name: "adds label to object with existing labels",
			inputLabels: map[string]string{
				"existing-label": "value",
			},
			expectedLabels: map[string]string{
				"existing-label":        "value",
				AggregationFeatureLabel: "true",
			},
		},
		{
			name: "updates label when already present",
			inputLabels: map[string]string{
				AggregationFeatureLabel: "false",
				"other-label":           "value",
			},
			expectedLabels: map[string]string{
				AggregationFeatureLabel: "true",
				"other-label":           "value",
			},
		},
		{
			name: "label already set to true",
			inputLabels: map[string]string{
				AggregationFeatureLabel: "true",
			},
			expectedLabels: map[string]string{
				AggregationFeatureLabel: "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Create a test object (using ClusterRoleTemplateBinding as an example)
			obj := &v3.ClusterRoleTemplateBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-crtb",
					Labels: tt.inputLabels,
				},
			}

			result := AddAggregationFeatureLabel(obj)

			// Verify the returned object is the same as input
			if result != obj {
				t.Errorf("AddAggregationFeatureLabel() should return the same object")
			}

			// Verify labels are set correctly
			resultLabels := result.GetLabels()
			if len(resultLabels) != len(tt.expectedLabels) {
				t.Errorf("AddAggregationFeatureLabel() labels count = %v, want %v", len(resultLabels), len(tt.expectedLabels))
			}

			for key, expectedValue := range tt.expectedLabels {
				if actualValue, ok := resultLabels[key]; !ok {
					t.Errorf("AddAggregationFeatureLabel() missing label %s", key)
				} else if actualValue != expectedValue {
					t.Errorf("AddAggregationFeatureLabel() label %s = %v, want %v", key, actualValue, expectedValue)
				}
			}
		})
	}
}
