package auth

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

func Test_checkReferencedRoles(t *testing.T) {
	manager := &manager{
		rtLister: &fakes.RoleTemplateListerMock{
			GetFunc: roleListerGetFunc,
		},
	}

	type args struct {
		rtName       string
		rtContext    string
		depthCounter int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Non-recursive role, none inherited",
			args: args{
				rtName:       "non-recursive",
				rtContext:    "",
				depthCounter: 0,
			},
			wantErr: false,
		},
		{
			name: "Non-recursive role, inherits another",
			args: args{
				rtName:       "inherit non-recursive",
				rtContext:    "",
				depthCounter: 0,
			},
			wantErr: false,
		},
		{
			name: "Recursive role",
			args: args{
				rtName:       "recursive1",
				rtContext:    "",
				depthCounter: 0,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.checkReferencedRoles(tt.args.rtName, tt.args.rtContext, tt.args.depthCounter)
			if tt.wantErr {
				assert.Error(t, err, "expected an error, got none")
			} else {
				assert.NoError(t, err, fmt.Sprintf("expected no error, got: %v", err))
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
