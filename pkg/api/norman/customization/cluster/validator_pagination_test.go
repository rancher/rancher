package cluster

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	aksv1 "github.com/rancher/aks-operator/pkg/apis/aks.cattle.io/v1"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	gkev1 "github.com/rancher/gke-operator/pkg/apis/gke.cattle.io/v1"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/pagination"
	"github.com/rancher/rancher/pkg/pagination/paginationtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	pgTargetName = "target"
	pgOtherName  = "other"
	pgRegion     = "region-1"
)

// pgStep is the response of the fake client for one continue token.
type pgStep struct {
	items []v32.Cluster
	cont  string
	err   error
}

// pgFakeClusterClient serves list responses keyed by the continue token of the request.
type pgFakeClusterClient struct {
	v3.ClusterInterface

	t     *testing.T
	steps map[string]pgStep

	mu     sync.Mutex
	tokens []string
	limits []int64
}

func (f *pgFakeClusterClient) List(opts metav1.ListOptions) (*v32.ClusterList, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.tokens = append(f.tokens, opts.Continue)
	f.limits = append(f.limits, opts.Limit)

	step, ok := f.steps[opts.Continue]
	if !ok {
		f.t.Errorf("unexpected list call with continue token %q", opts.Continue)
		return nil, errors.New("unexpected continue token")
	}
	if step.err != nil {
		return nil, step.err
	}
	return &v32.ClusterList{
		ListMeta: metav1.ListMeta{Continue: step.cont},
		Items:    step.items,
	}, nil
}

// pgProvider adapts one cluster name validator to a common signature.
type pgProvider struct {
	name string
	// newCluster returns a cluster of the provider type.
	newCluster func(name, region string) v32.Cluster
	// validate runs the validator under test.
	validate func(client v3.ClusterInterface, name, region string) error
	// emptyRegionMatchesAny is true if an empty requested region matches any region.
	emptyRegionMatchesAny bool
}

func pgProviders() []pgProvider {
	return []pgProvider{
		{
			name: "AKS",
			newCluster: func(name, region string) v32.Cluster {
				return v32.Cluster{Spec: v32.ClusterSpec{AKSConfig: &aksv1.AKSClusterConfigSpec{
					ClusterName:      name,
					ResourceLocation: region,
				}}}
			},
			validate: func(client v3.ClusterInterface, name, region string) error {
				return validateAKSClusterName(client, &v32.ClusterSpec{AKSConfig: &aksv1.AKSClusterConfigSpec{
					ClusterName:      name,
					ResourceLocation: region,
				}})
			},
			emptyRegionMatchesAny: true,
		},
		{
			name: "GKE",
			newCluster: func(name, region string) v32.Cluster {
				return v32.Cluster{Spec: v32.ClusterSpec{GKEConfig: &gkev1.GKEClusterConfigSpec{
					ClusterName: name,
					Region:      region,
				}}}
			},
			validate: func(client v3.ClusterInterface, name, region string) error {
				return validateGKEClusterName(client, &v32.ClusterSpec{GKEConfig: &gkev1.GKEClusterConfigSpec{
					ClusterName: name,
					Region:      region,
				}})
			},
			emptyRegionMatchesAny: true,
		},
		{
			name: "Ali",
			newCluster: func(name, region string) v32.Cluster {
				return v32.Cluster{Spec: v32.ClusterSpec{AliConfig: &aliv1.AliClusterConfigSpec{
					ClusterName: name,
					RegionID:    region,
				}}}
			},
			validate: func(client v3.ClusterInterface, name, region string) error {
				return validateAliConfigClusterName(client, &v32.ClusterSpec{AliConfig: &aliv1.AliClusterConfigSpec{
					ClusterName: name,
					RegionID:    region,
				}})
			},
			emptyRegionMatchesAny: true,
		},
		{
			// The EKS validator reads the Rancher cluster list through Validator.ClusterClient.
			// Without a SecretLister, eksAccountID returns "" and makes no network call.
			name: "EKS",
			newCluster: func(name, region string) v32.Cluster {
				return v32.Cluster{Spec: v32.ClusterSpec{EKSConfig: &eksv1.EKSClusterConfigSpec{
					DisplayName: name,
					Region:      region,
				}}}
			},
			validate: func(client v3.ClusterInterface, name, region string) error {
				return pgValidateEKS(client, name, region)
			},
			emptyRegionMatchesAny: false,
		},
	}
}

// pgValidateEKS runs validateEKSConfig for an imported EKS cluster create request.
// The imported flag skips the validation steps that are not part of the name check.
// The request has no amazonCredentialSecret, so the cloud credential access check is skipped.
func pgValidateEKS(client v3.ClusterInterface, name, region string) error {
	v := &Validator{ClusterClient: client}
	request := &types.APIContext{Method: http.MethodPost}
	cluster := map[string]interface{}{
		"eksConfig": map[string]interface{}{
			"displayName": name,
			"region":      region,
			"imported":    true,
		},
	}
	return v.validateEKSConfig(request, cluster, &v32.ClusterSpec{})
}

func pgRequireAPIError(t *testing.T, err error, want httperror.ErrorCode, msgContains string) {
	t.Helper()
	require.Error(t, err)
	var apiErr *httperror.APIError
	require.True(t, errors.As(err, &apiErr), "error %v is not an *httperror.APIError", err)
	assert.Equal(t, want, apiErr.Code)
	assert.Contains(t, apiErr.Message, msgContains)
}

func TestClusterNameValidatorsPagination(t *testing.T) {
	t.Parallel()

	type scenario struct {
		name string
		// steps builds the list responses. filler and match build clusters of the provider type.
		steps    func(filler, match v32.Cluster) map[string]pgStep
		wantCode *httperror.ErrorCode
		wantMsg  string
		// wantCause is text from the list error. The API error message must include it.
		wantCause  string
		wantTokens []string
	}

	conflictCode := httperror.InvalidBodyContent
	serverCode := httperror.ServerError

	scenarios := []scenario{
		{
			name: "match on page 2 is detected",
			steps: func(filler, match v32.Cluster) map[string]pgStep {
				return map[string]pgStep{
					"":   {items: []v32.Cluster{filler}, cont: "t1"},
					"t1": {items: []v32.Cluster{filler, match}, cont: "t2"},
				}
			},
			wantCode:   &conflictCode,
			wantMsg:    "cluster already exists for",
			wantTokens: []string{"", "t1"},
		},
		{
			name: "no match across 3 pages returns nil",
			steps: func(filler, match v32.Cluster) map[string]pgStep {
				return map[string]pgStep{
					"":   {items: []v32.Cluster{filler, {}}, cont: "t1"},
					"t1": {items: []v32.Cluster{filler}, cont: "t2"},
					"t2": {items: []v32.Cluster{filler}},
				}
			},
			wantTokens: []string{"", "t1", "t2"},
		},
		{
			name: "no match on a single empty page returns nil",
			steps: func(filler, match v32.Cluster) map[string]pgStep {
				return map[string]pgStep{"": {}}
			},
			wantTokens: []string{""},
		},
		{
			name: "match on page 1 stops paging",
			steps: func(filler, match v32.Cluster) map[string]pgStep {
				return map[string]pgStep{
					"": {items: []v32.Cluster{match}, cont: "t1"},
				}
			},
			wantCode:   &conflictCode,
			wantMsg:    "cluster already exists for",
			wantTokens: []string{""},
		},
		{
			name: "match on last page is detected",
			steps: func(filler, match v32.Cluster) map[string]pgStep {
				return map[string]pgStep{
					"":   {items: []v32.Cluster{filler}, cont: "t1"},
					"t1": {items: []v32.Cluster{filler}, cont: "t2"},
					"t2": {items: []v32.Cluster{match}},
				}
			},
			wantCode:   &conflictCode,
			wantMsg:    "cluster already exists for",
			wantTokens: []string{"", "t1", "t2"},
		},
		{
			name: "list error on page 1 returns a server error",
			steps: func(filler, match v32.Cluster) map[string]pgStep {
				return map[string]pgStep{
					"": {err: errors.New("boom")},
				}
			},
			wantCode:   &serverCode,
			wantMsg:    "failed to confirm",
			wantCause:  "boom",
			wantTokens: []string{""},
		},
		{
			name: "list error on page 2 returns a server error",
			steps: func(filler, match v32.Cluster) map[string]pgStep {
				return map[string]pgStep{
					"":   {items: []v32.Cluster{filler}, cont: "t1"},
					"t1": {err: errors.New("boom")},
				}
			},
			wantCode:   &serverCode,
			wantMsg:    "failed to confirm",
			wantCause:  "boom",
			wantTokens: []string{"", "t1"},
		},
		{
			name: "list error hides a match on a later page",
			steps: func(filler, match v32.Cluster) map[string]pgStep {
				return map[string]pgStep{
					"":   {items: []v32.Cluster{filler}, cont: "t1"},
					"t1": {err: errors.New("boom")},
					"t2": {items: []v32.Cluster{match}},
				}
			},
			wantCode:   &serverCode,
			wantMsg:    "failed to confirm",
			wantCause:  "boom",
			wantTokens: []string{"", "t1"},
		},
		{
			name: "expired error without token on page 2 returns a server error",
			steps: func(filler, match v32.Cluster) map[string]pgStep {
				return map[string]pgStep{
					"":   {items: []v32.Cluster{filler}, cont: "t1"},
					"t1": {err: paginationtest.ExpiredError()},
				}
			},
			wantCode:   &serverCode,
			wantMsg:    "failed to confirm",
			wantCause:  "continue token expired",
			wantTokens: []string{"", "t1"},
		},
		{
			name: "expired error on page 1 returns a server error",
			steps: func(filler, match v32.Cluster) map[string]pgStep {
				return map[string]pgStep{
					"": {err: paginationtest.ExpiredError()},
				}
			},
			wantCode:   &serverCode,
			wantMsg:    "failed to confirm",
			wantCause:  "continue token expired",
			wantTokens: []string{""},
		},
		{
			name: "expired error on page 2 returns a server error and does not resume",
			steps: func(filler, match v32.Cluster) map[string]pgStep {
				return map[string]pgStep{
					"":          {items: []v32.Cluster{filler}, cont: "t1"},
					"t1":        {err: paginationtest.ExpiredError()},
					"new-token": {items: []v32.Cluster{match}},
				}
			},
			wantCode:   &serverCode,
			wantMsg:    "failed to confirm",
			wantCause:  "continue token expired",
			wantTokens: []string{"", "t1"},
		},
	}

	for _, p := range pgProviders() {
		for _, sc := range scenarios {
			t.Run(p.name+"/"+sc.name, func(t *testing.T) {
				t.Parallel()

				filler := p.newCluster(pgOtherName, pgRegion)
				match := p.newCluster(pgTargetName, pgRegion)
				client := &pgFakeClusterClient{t: t, steps: sc.steps(filler, match)}

				err := p.validate(client, pgTargetName, pgRegion)

				if sc.wantCode == nil {
					assert.NoError(t, err)
				} else {
					pgRequireAPIError(t, err, *sc.wantCode, sc.wantMsg)
					assert.Contains(t, err.Error(), sc.wantCause)
				}
				assert.Equal(t, sc.wantTokens, client.tokens, "continue tokens of the list calls")
				for i, limit := range client.limits {
					assert.Equal(t, pagination.PageSize, limit, "limit of list call %d", i)
				}
			})
		}
	}
}

func TestClusterNameValidatorsFieldMatching(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		existingName   string
		existingRegion string
		reqName        string
		reqRegion      string
		wantConflict   bool
		// emptyRegionOnly marks cases that require emptyRegionMatchesAny semantics.
		emptyRegionOnly bool
	}{
		{
			name:         "same name and region collide",
			existingName: pgTargetName, existingRegion: pgRegion,
			reqName: pgTargetName, reqRegion: pgRegion,
			wantConflict: true,
		},
		{
			name:         "same name and different region do not collide",
			existingName: pgTargetName, existingRegion: "region-2",
			reqName: pgTargetName, reqRegion: pgRegion,
		},
		{
			name:         "different name and same region do not collide",
			existingName: pgOtherName, existingRegion: pgRegion,
			reqName: pgTargetName, reqRegion: pgRegion,
		},
		{
			name:         "different name and different region do not collide",
			existingName: pgOtherName, existingRegion: "region-2",
			reqName: pgTargetName, reqRegion: pgRegion,
		},
		{
			name:         "different name with empty region does not collide",
			existingName: pgOtherName, existingRegion: pgRegion,
			reqName: pgTargetName, reqRegion: "",
		},
		{
			name:         "same name with empty region collides by name only",
			existingName: pgTargetName, existingRegion: pgRegion,
			reqName: pgTargetName, reqRegion: "",
			wantConflict: true, emptyRegionOnly: true,
		},
	}

	for _, p := range pgProviders() {
		for _, tt := range tests {
			t.Run(p.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				client := &pgFakeClusterClient{t: t, steps: map[string]pgStep{
					"": {items: []v32.Cluster{p.newCluster(tt.existingName, tt.existingRegion)}},
				}}

				err := p.validate(client, tt.reqName, tt.reqRegion)

				wantConflict := tt.wantConflict
				if tt.emptyRegionOnly && !p.emptyRegionMatchesAny {
					wantConflict = false
				}
				if wantConflict {
					pgRequireAPIError(t, err, httperror.InvalidBodyContent, "cluster already exists for")
					assert.Contains(t, err.Error(), tt.reqName)
				} else {
					assert.NoError(t, err)
				}
				assert.Equal(t, []string{""}, client.tokens)
			})
		}
	}
}

func TestClusterNameValidatorsIgnoreOtherProviders(t *testing.T) {
	t.Parallel()

	providers := pgProviders()
	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()

			var items []v32.Cluster
			for _, other := range providers {
				if other.name != p.name {
					items = append(items, other.newCluster(pgTargetName, pgRegion))
				}
			}
			items = append(items, v32.Cluster{})
			client := &pgFakeClusterClient{t: t, steps: map[string]pgStep{"": {items: items}}}

			assert.NoError(t, p.validate(client, pgTargetName, pgRegion))
		})
	}
}

func TestValidateGKEClusterNameZone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		existing     gkev1.GKEClusterConfigSpec
		request      gkev1.GKEClusterConfigSpec
		wantConflict bool
		wantMsg      string
	}{
		{
			name:         "same name and zone with empty regions collide",
			existing:     gkev1.GKEClusterConfigSpec{ClusterName: pgTargetName, Zone: "zone-a"},
			request:      gkev1.GKEClusterConfigSpec{ClusterName: pgTargetName, Zone: "zone-a"},
			wantConflict: true,
			wantMsg:      "in zone [zone-a]",
		},
		{
			name:     "same name and different zone do not collide",
			existing: gkev1.GKEClusterConfigSpec{ClusterName: pgTargetName, Zone: "zone-b"},
			request:  gkev1.GKEClusterConfigSpec{ClusterName: pgTargetName, Zone: "zone-a"},
		},
		{
			name:         "request without zone collides by name and region",
			existing:     gkev1.GKEClusterConfigSpec{ClusterName: pgTargetName, Region: pgRegion, Zone: "zone-a"},
			request:      gkev1.GKEClusterConfigSpec{ClusterName: pgTargetName, Region: pgRegion},
			wantConflict: true,
			wantMsg:      "in region [region-1]",
		},
		{
			name:     "same name and zone with different region do not collide",
			existing: gkev1.GKEClusterConfigSpec{ClusterName: pgTargetName, Region: "region-2", Zone: "zone-a"},
			request:  gkev1.GKEClusterConfigSpec{ClusterName: pgTargetName, Region: pgRegion, Zone: "zone-a"},
		},
		{
			name:         "same name and region with different zone do not collide",
			existing:     gkev1.GKEClusterConfigSpec{ClusterName: pgTargetName, Region: pgRegion, Zone: "zone-b"},
			request:      gkev1.GKEClusterConfigSpec{ClusterName: pgTargetName, Region: pgRegion, Zone: "zone-a"},
			wantConflict: false,
		},
		{
			name:     "different name and same zone do not collide",
			existing: gkev1.GKEClusterConfigSpec{ClusterName: pgOtherName, Zone: "zone-a"},
			request:  gkev1.GKEClusterConfigSpec{ClusterName: pgTargetName, Zone: "zone-a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			existing := tt.existing
			request := tt.request
			client := &pgFakeClusterClient{t: t, steps: map[string]pgStep{
				"": {items: []v32.Cluster{{Spec: v32.ClusterSpec{GKEConfig: &existing}}}},
			}}

			err := validateGKEClusterName(client, &v32.ClusterSpec{GKEConfig: &request})

			if tt.wantConflict {
				pgRequireAPIError(t, err, httperror.InvalidBodyContent, "cluster already exists for GKE cluster")
				assert.True(t, strings.Contains(err.Error(), tt.wantMsg), "error %q does not contain %q", err.Error(), tt.wantMsg)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestValidateEKSConfigClusterNameCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		existingCredential string
		wantConflict       bool
	}{
		{
			name:               "same credential collides",
			existingCredential: "",
			wantConflict:       true,
		},
		{
			// The new request has no credential. With no SecretLister, both account IDs
			// resolve to "", so a different credential does not collide.
			name:               "different credential without account ID does not collide",
			existingCredential: "cattle-global-data:cc-existing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			existing := v32.Cluster{Spec: v32.ClusterSpec{EKSConfig: &eksv1.EKSClusterConfigSpec{
				DisplayName:            pgTargetName,
				Region:                 pgRegion,
				AmazonCredentialSecret: tt.existingCredential,
			}}}
			client := &pgFakeClusterClient{t: t, steps: map[string]pgStep{
				"":   {items: []v32.Cluster{{}}, cont: "t1"},
				"t1": {items: []v32.Cluster{existing}},
			}}

			err := pgValidateEKS(client, pgTargetName, pgRegion)

			if tt.wantConflict {
				pgRequireAPIError(t, err, httperror.InvalidBodyContent, "cluster already exists for EKS cluster")
			} else {
				assert.NoError(t, err)
			}
			wantTokens := []string{"", "t1"}
			assert.Equal(t, wantTokens, client.tokens)
		})
	}
}
