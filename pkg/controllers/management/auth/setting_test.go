package auth

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_syncWithEmptyObjValue(t *testing.T) {
	t.Run("Test azure auth sync with empty string obj.Value", func(t *testing.T) {
		obj := &v3.Setting{
			ObjectMeta: metav1.ObjectMeta{
				Name: "azure-group-cache-size",
			},
			Value: "",
		}
		sc := SettingController{}
		_, err := sc.sync("", obj)
		assert.NoError(t, err)
	})
}
