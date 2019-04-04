package helm

import (
	"testing"

	v3 "github.com/rancher/types/apis/project.cattle.io/v3"
)

func Test_isSame(t *testing.T) {

	tests := []struct {
		name string
		obj  v3.App
		rev  v3.AppRevision
		want bool
	}{
		{
			name: "ExternalID-same",
			obj: v3.App{
				Spec: v3.AppSpec{
					ExternalID: "catalog",
					Answers:    map[string]string{"ans1": "hi"},
					ValuesYaml: "values",
				},
			},
			rev: v3.AppRevision{
				Status: v3.AppRevisionStatus{
					ExternalID: "catalog",
					Answers:    map[string]string{"ans1": "hi"},
					ValuesYaml: "values",
				},
			},
			want: true,
		},
		{
			name: "ExternalID-different",
			obj: v3.App{
				Spec: v3.AppSpec{
					ExternalID: "catalogFromSomewhereElse",
					Answers:    map[string]string{"ans1": "hi"},
					ValuesYaml: "values",
				},
			},
			rev: v3.AppRevision{
				Status: v3.AppRevisionStatus{
					ExternalID: "catalog",
					Answers:    map[string]string{"ans1": "hi"},
					ValuesYaml: "values",
				},
			},
			want: false,
		},
	}
	// TODO: Add test cases.

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSame(&tt.obj, &tt.rev); got != tt.want {
				t.Errorf("isDifferent() = %v, want %v", got, tt.want)
			}
		})
	}
}
