package common

import (
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
)

func Test_injectDefaultRegistry(t *testing.T) {
	testRegistry := "test.registry.com"
	err := settings.SystemDefaultRegistry.Set(testRegistry)
	assert.Nil(t, err, "failed to set system default registry settings")

	testCases := []struct {
		app  *v3.App
		want bool
	}{
		{
			app: &v3.App{
				Spec: v32.AppSpec{
					ExternalID: "catalog://?catalog=library&template=wordpress&version=2.1.11",
				},
			},
			want: false,
		},
		{
			app: &v3.App{
				Spec: v32.AppSpec{
					ExternalID: "catalog://?catalog=system-library&template=rancher-external-dns&version=0.1.0",
				},
			},
			want: true,
		},
	}

	for _, testCase := range testCases {
		testApp := testCase.app
		injectMap := injectDefaultRegistry(testApp)
		if !testCase.want {
			assert.Nilf(t, injectMap, "catalog id %s should not get default registry parameters", testApp.Spec.ExternalID)
		} else {
			v, _ := injectMap["systemDefaultRegistry"]
			assert.Equalf(t, testRegistry, v, "catalog id %s should not get default registry parameters", testApp.Spec.ExternalID)
		}
	}
}
