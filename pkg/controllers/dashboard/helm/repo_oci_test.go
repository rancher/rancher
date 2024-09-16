package helm

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"testing"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
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
		expectedErr         string
	}{
		{
			name:          "Should return default values if exponentailBackOffValues is empty",
			backOffValues: &catalog.ExponentialBackOffValues{},
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * time.Second,
				MaxWait:  5 * time.Second,
				MaxRetry: 5,
			},
			expectedErr: "",
		},
		{
			name:          "Should return default values if values are not present",
			backOffValues: nil,
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * time.Second,
				MaxWait:  5 * time.Second,
				MaxRetry: 5,
			},
			expectedErr: "",
		},
		{
			name: "Should get max retries values from clusterRepo",
			backOffValues: &catalog.ExponentialBackOffValues{
				MinWait:    1,
				MaxWait:    5,
				MaxRetries: 10,
			},
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * time.Second,
				MaxWait:  5 * time.Second,
				MaxRetry: 10,
			},
			expectedErr: "",
		},
		{
			name: "Should get max wait from clusterRepo",
			backOffValues: &catalog.ExponentialBackOffValues{
				MinWait: 1,
				MaxWait: 3600,
			},
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * time.Second,
				MaxWait:  1 * time.Hour,
				MaxRetry: 5,
			},
			expectedErr: "",
		},
		{
			name: "Should get min wait from clusterRepo",
			backOffValues: &catalog.ExponentialBackOffValues{
				MinWait: 60,
				MaxWait: 120,
			},
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * time.Minute,
				MaxWait:  2 * time.Minute,
				MaxRetry: 5,
			},
			expectedErr: "",
		},
		{
			name: "minWait should be atleast 1 second",
			backOffValues: &catalog.ExponentialBackOffValues{
				MinWait: -1,
				MaxWait: 5,
			},
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * time.Second,
				MaxWait:  5 * time.Second,
				MaxRetry: 5,
			},
			expectedErr: "minWait must be at least 1 second",
		},
		{
			name: "maxWait cant be less than minWait",
			backOffValues: &catalog.ExponentialBackOffValues{
				MinWait: 60,
				MaxWait: 20,
			},
			expectedRetryPolicy: retryPolicy{
				MinWait:  1 * time.Minute,
				MaxWait:  20 * time.Second,
				MaxRetry: 5,
			},
			expectedErr: "maxWait must be greater than or equal to minWait",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			clusterRepo := &catalog.ClusterRepo{
				Spec: catalog.RepoSpec{
					URL:                      "oci://dp.apps.rancher.io",
					ExponentialBackOffValues: testCase.backOffValues,
				},
			}
			retryPolicy, err := getRetryPolicy(clusterRepo)
			if testCase.expectedErr != "" {
				assert.Contains(t, err.Error(), testCase.expectedErr)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, testCase.expectedRetryPolicy, retryPolicy)
		})
	}
}

func TestShouldSkip(t *testing.T) {
	ociInterval := 1 * time.Hour
	testCases := []struct {
		name                     string
		ociDownloadedTime        time.Time
		timeNow                  func() time.Time
		nextRetryAt              metav1.Time
		newClusterRepoController func(ctrl *gomock.Controller) *fake.MockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList]
		generation               int64
		observedGeneration       int64
		maxRetries               int
		numberOfRetries          int
		shouldNotSkip            bool
		expected                 bool
	}{
		{
			name: "Should skip if resourceVersion don't match",
			newClusterRepoController: func(ctrl *gomock.Controller) *fake.MockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList] {
				mockController := fake.NewMockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList](ctrl)
				mockController.EXPECT().Get("clusterRepo", metav1.GetOptions{}).Return(&catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"}}, nil)
				return mockController
			},
			expected: true,
		},
		{
			name: "Should skip if nextRetryAt is after time.now()",
			timeNow: func() time.Time {
				return time.Date(2024, 04, 23, 10, 1, 0, 0, time.UTC)
			},
			nextRetryAt: metav1.NewTime(time.Date(2024, 04, 23, 10, 2, 0, 0, time.UTC)),
			newClusterRepoController: func(ctrl *gomock.Controller) *fake.MockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList] {
				mockController := fake.NewMockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList](ctrl)
				mockController.EXPECT().Get("clusterRepo", metav1.GetOptions{}).Return(&catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}, nil)
				return mockController
			},
			expected: true,
		},
		{
			name: "Should NOT skip if status.ShouldNotSkip is true",
			timeNow: func() time.Time {
				return time.Date(2024, 04, 23, 10, 1, 0, 0, time.UTC)
			},
			newClusterRepoController: func(ctrl *gomock.Controller) *fake.MockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList] {
				mockController := fake.NewMockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList](ctrl)
				mockController.EXPECT().Get("clusterRepo", metav1.GetOptions{}).Return(&catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}, nil)
				return mockController
			},
			shouldNotSkip: true,
			expected:      false,
		},
		{
			name: "Should NOT skip if the handler is retrying",
			timeNow: func() time.Time {
				return time.Date(2024, 04, 23, 10, 1, 0, 0, time.UTC)
			},
			newClusterRepoController: func(ctrl *gomock.Controller) *fake.MockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList] {
				mockController := fake.NewMockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList](ctrl)
				mockController.EXPECT().Get("clusterRepo", metav1.GetOptions{}).Return(&catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}, nil)
				return mockController
			},
			maxRetries:      5,
			numberOfRetries: 1,
			expected:        false,
		},
		{
			name: "Should NOT skip if generation has changed",
			timeNow: func() time.Time {
				return time.Date(2024, 04, 23, 10, 1, 0, 0, time.UTC)
			},
			newClusterRepoController: func(ctrl *gomock.Controller) *fake.MockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList] {
				mockController := fake.NewMockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList](ctrl)
				mockController.EXPECT().Get("clusterRepo", metav1.GetOptions{}).Return(&catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}, nil)
				return mockController
			},
			generation:         1,
			observedGeneration: 0,
			maxRetries:         5,
			numberOfRetries:    0,
			expected:           false,
		},
		{
			name:              "Should NOT skip if interval has not passed",
			ociDownloadedTime: time.Date(2024, 04, 23, 10, 0, 0, 0, time.UTC),
			timeNow: func() time.Time {
				return time.Date(2024, 04, 23, 10, 1, 0, 0, time.UTC)
			},
			newClusterRepoController: func(ctrl *gomock.Controller) *fake.MockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList] {
				mockController := fake.NewMockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList](ctrl)
				mockController.EXPECT().Get("clusterRepo", metav1.GetOptions{}).Return(&catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}, nil)
				return mockController
			},
			generation:         1,
			observedGeneration: 0,
			maxRetries:         5,
			numberOfRetries:    0,
			expected:           false,
		},
		{
			name:              "Should skip if handler is done retrying, generation didn't change and interval has not passed",
			ociDownloadedTime: time.Date(2024, 04, 23, 10, 0, 0, 0, time.UTC),
			timeNow: func() time.Time {
				return time.Date(2024, 04, 23, 10, 5, 0, 0, time.UTC)
			},
			newClusterRepoController: func(ctrl *gomock.Controller) *fake.MockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList] {
				mockController := fake.NewMockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList](ctrl)
				mockController.EXPECT().Get("clusterRepo", metav1.GetOptions{}).Return(&catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}, nil)
				mockController.EXPECT().EnqueueAfter("clusterRepo", ociInterval).Return()
				return mockController
			},
			generation:         1,
			observedGeneration: 1,
			maxRetries:         5,
			numberOfRetries:    6,
			expected:           true,
		},
		{
			name:              "Should NOT skip if handler is done retrying, generation didn't change but interval has passed",
			ociDownloadedTime: time.Date(2024, 04, 23, 10, 0, 0, 0, time.UTC),
			timeNow: func() time.Time {
				return time.Date(2024, 04, 24, 11, 0, 0, 0, time.UTC)
			},
			newClusterRepoController: func(ctrl *gomock.Controller) *fake.MockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList] {
				mockController := fake.NewMockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList](ctrl)
				mockController.EXPECT().Get("clusterRepo", metav1.GetOptions{}).Return(&catalog.ClusterRepo{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "1"}}, nil)
				return mockController
			},
			generation:         1,
			observedGeneration: 1,
			maxRetries:         5,
			numberOfRetries:    5,
			expected:           false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			policy := retryPolicy{MaxRetry: testCase.maxRetries}
			ctrl := gomock.NewController(t)
			mockController := testCase.newClusterRepoController(ctrl)
			handler := OCIRepohandler{clusterRepoController: mockController}
			timeNow = testCase.timeNow
			clusterRepo := &catalog.ClusterRepo{
				ObjectMeta: metav1.ObjectMeta{
					Generation:      testCase.generation,
					ResourceVersion: "1",
					Name:            "clusterRepo",
				},
				Status: catalog.RepoStatus{
					ObservedGeneration: testCase.observedGeneration,
					NumberOfRetries:    testCase.numberOfRetries,
					NextRetryAt:        testCase.nextRetryAt,
					ShouldNotSkip:      testCase.shouldNotSkip,
					IndexConfigMapName: "indexConfigMap",
					Conditions: []genericcondition.GenericCondition{
						{
							Type:           string(catalog.OCIDownloaded),
							LastUpdateTime: testCase.ociDownloadedTime.Format(time.RFC3339),
						},
					}},
			}
			assert.Equal(t, testCase.expected, shouldSkip(clusterRepo, policy, catalog.OCIDownloaded, ociInterval, handler.clusterRepoController, &clusterRepo.Status))
		})
	}
}
