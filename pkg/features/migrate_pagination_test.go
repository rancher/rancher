package features

import (
	"errors"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/pagination/paginationtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// pagedClusterClientFake serves clusters from a pager.
type pagedClusterClientFake struct {
	managementv3.ClusterClient

	pager *paginationtest.Pager[v3.Cluster]
}

func newPagedClusterClientFake(errAtCall map[int]error, pages ...[]v3.Cluster) *pagedClusterClientFake {
	pager := paginationtest.NewPagesPager(pages...)
	pager.ListErrs = errAtCall
	return &pagedClusterClientFake{pager: pager}
}

func (f *pagedClusterClientFake) List(opts metav1.ListOptions) (*v3.ClusterList, error) {
	items, next, err := f.pager.Serve(opts)
	if err != nil {
		return nil, err
	}
	return &v3.ClusterList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}, nil
}

type featureClientFake struct {
	managementv3.FeatureClient

	features  []v3.Feature
	updateErr error
	updated   []*v3.Feature
	created   []*v3.Feature
}

func (f *featureClientFake) List(_ metav1.ListOptions) (*v3.FeatureList, error) {
	return &v3.FeatureList{Items: f.features}, nil
}

func (f *featureClientFake) Update(feature *v3.Feature) (*v3.Feature, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.updated = append(f.updated, feature)
	return feature, nil
}

func (f *featureClientFake) Create(feature *v3.Feature) (*v3.Feature, error) {
	f.created = append(f.created, feature)
	return feature, nil
}

func paginationTestCluster(name string, administrated bool) v3.Cluster {
	c := v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if administrated {
		c.Annotations = map[string]string{"provisioning.cattle.io/administrated": "true"}
	}
	return c
}

func paginationTestFeature(value *bool) *v3.Feature {
	return &v3.Feature{
		ObjectMeta: metav1.ObjectMeta{Name: RKE2.Name()},
		Spec:       v3.FeatureSpec{Value: value},
	}
}

func TestEnableRKE2IfClustersExistPagination(t *testing.T) {
	t.Parallel()

	listErr := errors.New("list failed")
	updateErr := errors.New("update failed")
	expiredErr := paginationtest.ExpiredError("token-page-2")
	plain := []v3.Cluster{paginationTestCluster("a", false)}
	match := []v3.Cluster{paginationTestCluster("b", false), paginationTestCluster("match", true)}

	tests := []struct {
		name           string
		featureValue   *bool
		pages          [][]v3.Cluster
		errAtCall      map[int]error
		updateErr      error
		wantErr        error
		wantUpdates    int
		wantListTokens []string
	}{
		{
			name:           "match only on page 2",
			featureValue:   &[]bool{false}[0],
			pages:          [][]v3.Cluster{plain, match, plain},
			wantUpdates:    1,
			wantListTokens: []string{"", "token-page-1"},
		},
		{
			name:           "match on page 1 stops paging",
			featureValue:   &[]bool{false}[0],
			pages:          [][]v3.Cluster{match, plain, plain},
			wantUpdates:    1,
			wantListTokens: []string{""},
		},
		{
			name:           "no match across 3 pages",
			featureValue:   &[]bool{false}[0],
			pages:          [][]v3.Cluster{plain, plain, plain},
			wantUpdates:    0,
			wantListTokens: []string{"", "token-page-1", "token-page-2"},
		},
		{
			name:           "update error is returned",
			featureValue:   &[]bool{false}[0],
			pages:          [][]v3.Cluster{plain, match, plain},
			updateErr:      updateErr,
			wantErr:        updateErr,
			wantListTokens: []string{"", "token-page-1"},
		},
		{
			name:           "list error on page 2 is returned",
			featureValue:   &[]bool{false}[0],
			pages:          [][]v3.Cluster{plain, match, plain},
			errAtCall:      map[int]error{1: listErr},
			wantErr:        listErr,
			wantListTokens: []string{"", "token-page-1"},
		},
		{
			name:           "expired token is returned and the list does not resume",
			featureValue:   &[]bool{false}[0],
			pages:          [][]v3.Cluster{plain, plain, match},
			errAtCall:      map[int]error{1: expiredErr},
			wantErr:        expiredErr,
			wantListTokens: []string{"", "token-page-1"},
		},
		{
			name:           "feature value nil does not list",
			featureValue:   nil,
			pages:          [][]v3.Cluster{match},
			wantListTokens: nil,
		},
		{
			name:           "feature already enabled does not list",
			featureValue:   &[]bool{true}[0],
			pages:          [][]v3.Cluster{match},
			wantListTokens: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			clusters := newPagedClusterClientFake(tt.errAtCall, tt.pages...)
			featuresClient := &featureClientFake{updateErr: tt.updateErr}

			err := enableRKE2IfClustersExist(paginationTestFeature(tt.featureValue), featuresClient, clusters)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantListTokens, clusters.pager.Tokens())
			clusters.pager.AssertLimit(t)
			require.Len(t, featuresClient.updated, tt.wantUpdates)
			for _, f := range featuresClient.updated {
				require.NotNil(t, f.Spec.Value)
				assert.True(t, *f.Spec.Value)
				require.NotNil(t, f.Status.LockedValue)
				assert.True(t, *f.Status.LockedValue)
			}
		})
	}
}

func TestMigrateFeaturesRKE2Pagination(t *testing.T) {
	t.Parallel()

	disabled := false
	clusters := newPagedClusterClientFake(nil,
		[]v3.Cluster{paginationTestCluster("a", false)},
		[]v3.Cluster{paginationTestCluster("match", true)},
		[]v3.Cluster{paginationTestCluster("c", false)},
	)
	featuresClient := &featureClientFake{features: []v3.Feature{
		{ObjectMeta: metav1.ObjectMeta{Name: Legacy.Name()}},
		*paginationTestFeature(&disabled),
	}}

	require.NoError(t, MigrateFeatures(featuresClient, nil, clusters))

	assert.Equal(t, []string{"", "token-page-1"}, clusters.pager.Tokens())
	require.Len(t, featuresClient.updated, 1)
	assert.Equal(t, RKE2.Name(), featuresClient.updated[0].Name)
	require.NotNil(t, featuresClient.updated[0].Spec.Value)
	assert.True(t, *featuresClient.updated[0].Spec.Value)
	assert.Empty(t, featuresClient.created)
}
