package rbac

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	fakes "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var roles = map[string]*v3.RoleTemplate{
	"recursive1": {
		RoleTemplateNames: []string{"recursive2"},
	},
	"recursive2": {
		RoleTemplateNames: []string{"recursive1"},
	},
	"non-recursive": {},
	"inherit non-recursive": {
		RoleTemplateNames: []string{"non-recursive"},
	},
}

func Test_gatherRoles(t *testing.T) {
	manager := &manager{
		rtLister: &fakes.RoleTemplateListerMock{
			GetFunc: roleListerGetFunc,
		},
	}
	emptyRoleTemplates := make(map[string]*v3.RoleTemplate)
	type args struct {
		rt            *v3.RoleTemplate
		roleTemplates map[string]*v3.RoleTemplate
		depthCounter  int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Non-recursive role, none inherited",
			args: args{
				rt:            roles["non-recursive"],
				roleTemplates: emptyRoleTemplates,
				depthCounter:  0,
			},
			wantErr: false,
		},
		{
			name: "Non-recursive role, inherits another",
			args: args{
				rt:            roles["inherit non-recursive"],
				roleTemplates: emptyRoleTemplates,
				depthCounter:  0,
			},
			wantErr: false,
		},
		{
			name: "Recursive role",
			args: args{
				rt:            roles["recursive1"],
				roleTemplates: emptyRoleTemplates,
				depthCounter:  0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.gatherRoles(tt.args.rt, tt.args.roleTemplates, tt.args.depthCounter)
			if tt.wantErr {
				assert.Error(t, err, "expected an error, received none")
			} else {
				assert.NoError(t, err, fmt.Sprintf("expected no err, got %v", err))
			}
		})
	}
}

func roleListerGetFunc(ns, name string) (*v3.RoleTemplate, error) {
	role, ok := roles[name]
	if !ok {
		return nil, errors.NewNotFound(schema.GroupResource{
			Group:    v3.RoleTemplateGroupVersionKind.Group,
			Resource: v3.RoleTemplateGroupVersionResource.Resource,
		}, name)
	}
	return role, nil
}
