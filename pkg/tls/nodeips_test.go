package tls

import (
	"testing"

	"github.com/rancher/rancher/pkg/pagination/paginationtest"
	corev1controllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// pagedNodeController serves nodes from a pager.
type pagedNodeController struct {
	corev1controllers.NodeController
	pager *paginationtest.Pager[corev1.Node]
}

func (f *pagedNodeController) List(opts metav1.ListOptions) (*corev1.NodeList, error) {
	items, next, err := f.pager.Serve(opts)
	if err != nil {
		return nil, err
	}
	return &corev1.NodeList{ListMeta: metav1.ListMeta{Continue: next}, Items: items}, nil
}

func TestCollectNodeIPs_Pagination(t *testing.T) {
	t.Parallel()

	node1 := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
				{Type: corev1.NodeHostName, Address: "hostname-ignored"},
			},
		},
	}
	node2 := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node2"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeExternalIP, Address: "198.51.100.1"},
			},
		},
	}
	node3 := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node3"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.3"},
				{Type: corev1.NodeExternalIP, Address: "198.51.100.3"},
			},
		},
	}

	tests := []struct {
		name       string
		nodes      []corev1.Node
		pageSize   int
		failPage   int
		failErr    error
		wantTokens []string
		wantIPs    []string
		wantErr    bool
	}{
		{
			name:       "two-page traversal collects all internal and external IPs",
			nodes:      []corev1.Node{node1, node2, node3},
			pageSize:   2,
			wantTokens: []string{"", "token-page-1"},
			wantIPs:    []string{"10.0.0.1", "198.51.100.1", "10.0.0.3", "198.51.100.3"},
		},
		{
			name:       "empty node list completes in one page and returns an empty slice",
			nodes:      []corev1.Node{},
			pageSize:   2,
			wantTokens: []string{""},
			wantIPs:    []string{},
		},
		{
			name:       "error on page 2 aborts and returns error",
			nodes:      []corev1.Node{node1, node2, node3},
			pageSize:   2,
			failPage:   2,
			wantTokens: []string{"", "token-page-1"},
			wantErr:    true,
		},
		{
			name:       "expired token on page 2 returns an error and does not resume",
			nodes:      []corev1.Node{node1, node2, node3},
			pageSize:   2,
			failPage:   2,
			failErr:    paginationtest.ExpiredError("resume-token"),
			wantTokens: []string{"", "token-page-1"},
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pager := paginationtest.NewPager(tc.nodes, tc.pageSize).FailOnPage(tc.failPage, tc.failErr)
			pager.ExtraTokens = map[string]int{"resume-token": 1}
			ips, err := collectNodeIPs(&pagedNodeController{pager: pager})
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantIPs, ips)
			}

			assert.Equal(t, tc.wantTokens, pager.Tokens())
			pager.AssertLimit(t)
		})
	}
}
