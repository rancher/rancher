package k3sbasedupgrade

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_upgradingMessage(t *testing.T) {

	tests := []struct {
		name        string
		concurrency int
		nodes       []string
		want        string
	}{
		// concurrency cannot be negative, min is set to 1 via Norman struct tag
		{
			name:        "base",
			concurrency: 2,
			nodes:       []string{"node1", "node2"},
			want:        "node1, node2",
		},
		{
			name:        "single",
			concurrency: 1,
			nodes:       []string{"node1", "node2"},
			want:        "node1",
		},
		{
			name:        "high concurrency",
			concurrency: 50000,
			nodes:       []string{"n1", "n2", "n3"},
			want:        "n1, n2, n3",
		},
		{
			name:        "max + 1",
			concurrency: MaxDisplayNodes + 1,
			nodes:       []string{"n1", "n2", "n3", "n3", "n4", "n5", "n6", "n7", "n8", "n9", "n10", "n11"},
			want:        "n1, n2, n3, n3, n4, n5, n6, n7, n8, n9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := upgradingMessage(tt.concurrency, tt.nodes)
			assert.Equal(t, tt.want, got)
		})
	}
}
