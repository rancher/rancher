package auth

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPRTBControllerFindsObjectsWithServiceAccountFieldAndSavesItAsAnnotation(t *testing.T) {
	t.Parallel()
	req := require.New(t)

	cache := []v3.ProjectRoleTemplateBinding{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sa-prtb",
			},
			ProjectName:      "rke:p1",
			RoleTemplateName: "project-member",
			ServiceAccount:   "p1:default",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sa-prtb-processed",
				Annotations: map[string]string{
					"management.cattle.io/serviceAccount": "p1:sa1",
				},
			},
			ProjectName:      "rke:p1",
			RoleTemplateName: "project-member",
			ServiceAccount:   "p1:sa1",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "user-prtb",
			},
			ProjectName:      "rke:p1",
			RoleTemplateName: "project-member",
			UserName:         "user1",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "group-prtb",
			},
			ProjectName:      "rke:p1",
			RoleTemplateName: "project-member",
			GroupName:        "group1",
		},
	}

	ctrl := gomock.NewController(t)
	client := fake.NewMockClientInterface[*v3.ProjectRoleTemplateBinding, *v3.ProjectRoleTemplateBindingList](ctrl)
	client.EXPECT().Update(gomock.Any()).DoAndReturn(func(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
		for i := range cache {
			if cache[i].Name == obj.Name {
				cache[i] = *obj
				break
			}
		}
		return obj, nil
	}).Times(1) // Only one object must be updated.

	controller := prtbServiceAccountController{
		prtbClient: client,
	}
	for _, prtb := range cache {
		upd, err := controller.sync("", &prtb)
		req.NoError(err)
		req.NotNil(upd)
	}

	req.Len(cache, 4)
	req.Equal(map[string]string{"management.cattle.io/serviceAccount": "p1:default"}, cache[0].Annotations)
	req.Equal(map[string]string{"management.cattle.io/serviceAccount": "p1:sa1"}, cache[1].Annotations)
	req.Nil(cache[2].Annotations)
	req.Nil(cache[3].Annotations)
}
