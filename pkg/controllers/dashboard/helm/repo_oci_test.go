package helm

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/wrangler/v2/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGetIndexFile(t *testing.T) {
	indexFile := repo.NewIndexFile()
	indexFile.Entries["testingchart"] = repo.ChartVersions{
		&repo.ChartVersion{
			Metadata: &chart.Metadata{
				Name:    "testingchart",
				Version: "0.1.0",
			},
		},
	}

	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	err := json.NewEncoder(gz).Encode(indexFile)
	assert.NoError(t, err)

	err = gz.Close()
	assert.NoError(t, err)

	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "repoName-0-unique",
			Namespace: "cattle-system",
		},
		BinaryData: map[string][]byte{
			"content": buf.Bytes(),
		},
	}

	tests := []struct {
		name                string
		clusterRepo         *catalog.ClusterRepo
		newConfigController func(ctrl *gomock.Controller) *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList]
		expectedResult      bool
		namespace           string
		indexFile           *repo.IndexFile
		expectedErrMsg      string
	}{
		{
			"no error is returned if configfile is not found when fetched through configmap name generation",
			&catalog.ClusterRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repoName",
					UID:  "unique",
				},
				Spec: catalog.RepoSpec{
					URL: "www.example.com",
				},
				Status: catalog.RepoStatus{
					URL: "www.example.com",
				},
			},
			func(ctrl *gomock.Controller) *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList] {
				configMapControllerFake := fake.NewMockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
				// Set the configMapController Expectations

				configMapControllerFake.EXPECT().Get("cattle-system", "repoName-0-unique", metav1.GetOptions{}).Return(nil, apierrors.NewNotFound(schema.GroupResource{
					Group:    corev1.GroupName,
					Resource: corev1.ResourceConfigMaps.String(),
				}, "repoName-0-unique"))

				return configMapControllerFake
			},
			false,
			"",
			repo.NewIndexFile(),
			"",
		},
		{
			"no error is returned if configfile is not found when fetched through configmap name generation",
			&catalog.ClusterRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repoName",
					UID:  "unique",
				},
				Spec: catalog.RepoSpec{
					URL: "www.example.com",
				},
				Status: catalog.RepoStatus{
					URL: "www.example.com",
				},
			},
			func(ctrl *gomock.Controller) *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList] {
				configMapControllerFake := fake.NewMockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
				// Set the configMapController Expectations

				configMapControllerFake.EXPECT().Get("cattle-system", "repoName-0-unique", metav1.GetOptions{}).Return(nil, apierrors.NewInternalError(errors.New("internal")))

				return configMapControllerFake
			},
			false,
			"",
			repo.NewIndexFile(),
			"failed to fetch the index configmap",
		},
		{
			"index File is fetched through the configmap name generation",
			&catalog.ClusterRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repoName",
					UID:  "unique",
				},
				Spec: catalog.RepoSpec{
					URL: "www.example.com",
				},
				Status: catalog.RepoStatus{
					URL: "www.example.com",
				},
			},
			func(ctrl *gomock.Controller) *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList] {
				configMapControllerFake := fake.NewMockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
				// Set the configMapController Expectations

				configMapControllerFake.EXPECT().Get("cattle-system", "repoName-0-unique", metav1.GetOptions{}).Return(&configMap, nil)

				return configMapControllerFake
			},
			false,
			"",
			indexFile,
			"",
		},
		{
			"error is returned if configmap from status field is failed to fetch",
			&catalog.ClusterRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repoName",
					UID:  "unique",
				},
				Spec: catalog.RepoSpec{
					URL: "www.example.com",
				},
				Status: catalog.RepoStatus{
					URL:                     "www.example.com",
					IndexConfigMapName:      "repoName-0-unique",
					IndexConfigMapNamespace: "cattle-system",
				},
			},
			func(ctrl *gomock.Controller) *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList] {
				configMapControllerFake := fake.NewMockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
				// Set the configMapController Expectations

				configMapControllerFake.EXPECT().Get("cattle-system", "repoName-0-unique", metav1.GetOptions{}).Return(nil, errors.New("kube api server not working"))

				return configMapControllerFake
			},
			false,
			"",
			repo.NewIndexFile(),
			"failed to fetch the index configmap",
		},
		{
			"index File is fetched through the status field values",
			&catalog.ClusterRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repoName",
					UID:  "unique",
				},
				Spec: catalog.RepoSpec{
					URL: "www.example.com",
				},
				Status: catalog.RepoStatus{
					URL:                     "www.example.com",
					IndexConfigMapName:      "repoName-0-unique",
					IndexConfigMapNamespace: "cattle-system",
				},
			},
			func(ctrl *gomock.Controller) *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList] {
				configMapControllerFake := fake.NewMockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
				// Set the configMapController Expectations

				configMapControllerFake.EXPECT().Get("cattle-system", "repoName-0-unique", metav1.GetOptions{}).Return(&configMap, nil)

				return configMapControllerFake
			},
			false,
			"",
			indexFile,
			"",
		},
		{
			"when spec URL and status URL differ, empty indexFile is returned",
			&catalog.ClusterRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name: "repoName",
					UID:  "unique",
				},
				Spec: catalog.RepoSpec{
					URL: "www.example.com",
				},
				Status: catalog.RepoStatus{
					URL: "www.different.com",
				},
			},
			func(ctrl *gomock.Controller) *fake.MockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList] {
				configMapControllerFake := fake.NewMockControllerInterface[*corev1.ConfigMap, *corev1.ConfigMapList](ctrl)
				return configMapControllerFake
			},
			false,
			"",
			repo.NewIndexFile(),
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner := metav1.OwnerReference{
				APIVersion: catalog.SchemeGroupVersion.Group + "/" + catalog.SchemeGroupVersion.Version,
				Kind:       "ClusterRepo",
				Name:       tt.clusterRepo.Name,
				UID:        tt.clusterRepo.UID,
			}

			ctrl := gomock.NewController(t)
			configMapController := tt.newConfigController(ctrl)

			indexFile, err := getIndexfile(tt.clusterRepo.Status, tt.clusterRepo.Spec, configMapController, owner, tt.namespace)
			assert.Equal(t, indexFile.Entries, tt.indexFile.Entries)
			if tt.expectedErrMsg != "" {
				assert.Equal(t, tt.expectedErrMsg, "failed to fetch the index configmap")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
