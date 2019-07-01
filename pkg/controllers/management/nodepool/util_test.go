package nodepool

import "testing"

func TestCompare(t *testing.T) {

	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		// TODO: Add test cases.
		{
			name: "basic",
			a:    "nodepool9",
			b:    "nodepool10",
			want: true, // a should come first
		},
		{
			"basic2",
			"nodepool09",
			"nodepool10",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Compare(tt.a, tt.b); got != tt.want {
				t.Errorf("Compare() = %v, want %v", got, tt.want)
			}
		})
	}
}
