package image

import (
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchImagesFromSystem(t *testing.T) {
	t.Run("linux images must contain kube-api-auth", func(t *testing.T) {
		imagesSet := make(map[string]map[string]struct{})
		system := System{
			Config: ExportConfig{
				OsType: Linux,
			},
		}
		err := system.FetchImages(imagesSet)
		require.NoError(t, err)

		kubeApiAuth := v32.ToolsSystemImages.AuthSystemImages.KubeAPIAuth
		require.Contains(t, imagesSet, kubeApiAuth)
		assert.Contains(t, imagesSet[kubeApiAuth], "system")
	})
	t.Run("windows images should be empty", func(t *testing.T) {
		imagesSet := make(map[string]map[string]struct{})
		system := System{
			Config: ExportConfig{
				OsType: Windows,
			},
		}
		err := system.FetchImages(imagesSet)
		require.NoError(t, err)

		assert.Empty(t, imagesSet)
	})
}
