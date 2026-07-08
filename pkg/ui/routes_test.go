package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestRoutes_IndexFileOnNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCache := fake.NewMockCacheInterface[*apimgmtv3.ClusterRegistrationToken](ctrl)
	// cacerts.Handler calls AddIndexer
	mockCache.EXPECT().AddIndexer(gomock.Any(), gomock.Any()).AnyTimes()

	tempDir, err := os.MkdirTemp("", "ui-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create index.html
	indexContent := "index file content"
	err = os.WriteFile(filepath.Join(tempDir, "index.html"), []byte(indexContent), 0644)
	assert.NoError(t, err)

	// Create assets directory
	assetsDir := filepath.Join(tempDir, "assets")
	err = os.Mkdir(assetsDir, 0755)
	assert.NoError(t, err)

	// Create an asset file
	assetContent := "asset file content"
	err = os.WriteFile(filepath.Join(assetsDir, "test.js"), []byte(assetContent), 0644)
	assert.NoError(t, err)

	// Create translations directory
	translationsDir := filepath.Join(tempDir, "translations")
	err = os.Mkdir(translationsDir, 0755)
	assert.NoError(t, err)

	// Set settings
	origUIPath := settings.UIPath.Get()
	origUIOffline := settings.UIOfflinePreferred.Get()

	// We use Set to override the defaults for the test
	settings.UIPath.Set(tempDir)
	settings.UIOfflinePreferred.Set("true")
	defer func() {
		settings.UIPath.Set(origUIPath)
		settings.UIOfflinePreferred.Set(origUIOffline)
	}()

	// New returns the router
	handler := New(nil, mockCache)

	tests := []struct {
		name           string
		path           string
		expectedBody   string
		expectedStatus int
	}{
		{
			name:           "Access assets directory",
			path:           "/assets",
			expectedBody:   indexContent,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Access assets directory with trailing slash",
			path:           "/assets/",
			expectedBody:   indexContent,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Access assets file",
			path:           "/assets/test.js",
			expectedBody:   assetContent,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Access translations directory",
			path:           "/translations",
			expectedBody:   indexContent,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Access translations directory with trailing slash",
			path:           "/translations/",
			expectedBody:   indexContent,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Access translations file (missing)",
			path:           "/translations/en-us.json",
			expectedBody:   indexContent,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Access missing asset",
			path:           "/assets/missing.js",
			expectedBody:   indexContent,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, tt.expectedBody, rec.Body.String())
		})
	}
}
