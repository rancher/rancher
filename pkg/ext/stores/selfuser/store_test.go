package selfuser

import (
	"context"
	"testing"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestCreate(t *testing.T) {
	fakeUserName := "fake-name"
	tests := map[string]struct {
		ctx     context.Context
		obj     *ext.SelfUser
		options *metav1.CreateOptions
		wantObj *ext.SelfUser
		wantErr string
	}{
		"valid request": {
			ctx:     request.WithUser(context.Background(), &user.DefaultInfo{Name: fakeUserName}),
			obj:     &ext.SelfUser{},
			options: &metav1.CreateOptions{},
			wantObj: &ext.SelfUser{
				Status: ext.SelfUserStatus{
					UserID: fakeUserName,
				},
			},
		},
		"dry run": {
			ctx: request.WithUser(context.Background(), &user.DefaultInfo{Name: fakeUserName}),
			obj: &ext.SelfUser{},
			options: &metav1.CreateOptions{
				DryRun: []string{metav1.DryRunAll},
			},
			wantObj: &ext.SelfUser{},
		},
		"context without user": {
			ctx:     context.TODO(),
			obj:     &ext.SelfUser{},
			options: &metav1.CreateOptions{},
			wantErr: "can't get user info from context",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := Store{}

			obj, err := store.Create(test.ctx, test.obj, nil, test.options)

			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.wantObj, obj)
			}
		})
	}
}
