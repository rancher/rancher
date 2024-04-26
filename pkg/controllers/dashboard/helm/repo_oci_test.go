package helm

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/wrangler/v2/pkg/generic/fake"
	"github.com/rancher/wrangler/v2/pkg/genericcondition"
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

func TestGetRetryPolicy(t *testing.T) {
	testCases := []struct {
		name                string
		backOffValues       *catalog.ExponentialBackOffValues
		expectedRetryPolicy retryPolicy
	}{
		{
			name:          "Should return default values if values are not present",
			backOffValues: nil,
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * time.Second,
				MaxWait:  5 * time.Second,
				MaxRetry: 5,
			},
		},
		{
			name: "Should get max retries values from clusterRepo",
			backOffValues: &catalog.ExponentialBackOffValues{
				MaxRetries: 10,
			},
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * time.Second,
				MaxWait:  5 * time.Second,
				MaxRetry: 10,
			},
		},
		{
			name: "Should get max wait from clusterRepo",
			backOffValues: &catalog.ExponentialBackOffValues{
				MaxWait: "1h",
			},
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * time.Second,
				MaxWait:  1 * time.Hour,
				MaxRetry: 5,
			},
		},
		{
			name: "Should get min wait from clusterRepo",
			backOffValues: &catalog.ExponentialBackOffValues{
				MinWait: "1m",
			},
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * 1 * time.Minute,
				MaxWait:  5 * time.Second,
				MaxRetry: 5,
			},
		},
		{
			name: "minWait should be at least 1 second",
			backOffValues: &catalog.ExponentialBackOffValues{
				MinWait: "150ms",
			},
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * time.Second,
				MaxWait:  5 * time.Second,
				MaxRetry: 5,
			},
		},
		{
			name: "minWait cant be less than maxWait",
			backOffValues: &catalog.ExponentialBackOffValues{
				MinWait: "1m",
				MaxWait: "5s",
			},
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * 1 * time.Minute,
				MaxWait:  1 * 1 * time.Minute,
				MaxRetry: 5,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			clusterRepo := &catalog.ClusterRepo{
				Spec: catalog.RepoSpec{
					ExponentialBackOffValues: testCase.backOffValues,
				},
			}
			assert.Equal(t, testCase.expectedRetryPolicy, getRetryPolicy(clusterRepo))
		})
	}
}

func TestShouldResetRetries(t *testing.T) {
	interval = 1 * time.Hour
	testCases := []struct {
		name               string
		ociDownloadedTime  time.Time
		lastStatusUpdate   *metav1.Time
		forceUpdate        *metav1.Time
		timeNow            func() time.Time
		generation         int64
		observedGeneration int64
		expected           bool
	}{
		{
			name:              "Should reset retries if interval has passed and status was not updated after the interval",
			ociDownloadedTime: time.Date(2024, 04, 23, 9, 0, 0, 0, time.UTC),
			lastStatusUpdate:  &metav1.Time{Time: time.Date(2024, 04, 23, 9, 0, 0, 0, time.UTC)},
			forceUpdate:       nil,
			timeNow: func() time.Time {
				return time.Date(2024, 04, 23, 10, 1, 0, 0, time.UTC)
			},
			expected: true,
		},
		{
			name:              "Should NOT reset retries if interval has passed but status was updated after the interval",
			ociDownloadedTime: time.Date(2024, 04, 23, 10, 0, 0, 0, time.UTC),
			lastStatusUpdate:  &metav1.Time{Time: time.Date(2024, 04, 23, 11, 1, 0, 0, time.UTC)},
			forceUpdate:       nil,
			timeNow: func() time.Time {
				return time.Date(2024, 04, 23, 10, 1, 0, 0, time.UTC)
			},
			expected: false,
		},
		{
			name:              "Should reset retries if generation has changed",
			ociDownloadedTime: time.Date(2024, 04, 23, 10, 0, 0, 0, time.UTC),
			lastStatusUpdate:  &metav1.Time{Time: time.Date(2024, 04, 23, 11, 1, 0, 0, time.UTC)},
			forceUpdate:       nil,
			timeNow: func() time.Time {
				return time.Date(2024, 04, 23, 10, 1, 0, 0, time.UTC)
			},
			generation:         2,
			observedGeneration: 3,
			expected:           false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			timeNow = testCase.timeNow
			clusterRepo := &catalog.ClusterRepo{
				ObjectMeta: metav1.ObjectMeta{
					Generation: testCase.generation,
					ManagedFields: []metav1.ManagedFieldsEntry{
						{
							Subresource: "status",
							Operation:   metav1.ManagedFieldsOperationUpdate,
							Time:        testCase.lastStatusUpdate,
						},
					}},
				Spec: catalog.RepoSpec{
					ForceUpdate: testCase.forceUpdate,
				},
				Status: catalog.RepoStatus{
					ObservedGeneration: testCase.observedGeneration,
					Conditions: []genericcondition.GenericCondition{
						{
							Type:           string(catalog.OCIDownloaded),
							LastUpdateTime: testCase.ociDownloadedTime.Format(time.RFC3339),
						},
					}},
			}
			assert.Equal(t, testCase.expected, shouldResetRetries(clusterRepo))
		})
	}
}
