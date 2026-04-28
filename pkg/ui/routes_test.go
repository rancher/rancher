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

func TestRoutes_Vue(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCache := fake.NewMockCacheInterface[*apimgmtv3.ClusterRegistrationToken](ctrl)
	// cacerts.Handler calls AddIndexer
	mockCache.EXPECT().AddIndexer(gomock.Any(), gomock.Any()).AnyTimes()

	tempDir, err := os.MkdirTemp("", "ui-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create index.html at the dashboard path root (served by IndexFile()).
	indexContent := "index file content"
	err = os.WriteFile(filepath.Join(tempDir, "index.html"), []byte(indexContent), 0644)
	assert.NoError(t, err)

	// Create a root-level asset served by ServeAsset().
	robotsContent := "User-agent: *"
	err = os.WriteFile(filepath.Join(tempDir, "robots.txt"), []byte(robotsContent), 0644)
	assert.NoError(t, err)

	// Create the dashboard subdirectory used by dashboard asset routes and
	// ServeFaviconDashboard().
	dashboardDir := filepath.Join(tempDir, "dashboard")
	err = os.Mkdir(dashboardDir, 0755)
	assert.NoError(t, err)

	dashboardAssetContent := "dashboard asset content"
	err = os.WriteFile(filepath.Join(dashboardDir, "test.js"), []byte(dashboardAssetContent), 0644)
	assert.NoError(t, err)

	faviconContent := "favicon bytes"
	err = os.WriteFile(filepath.Join(dashboardDir, "favicon.png"), []byte(faviconContent), 0644)
	assert.NoError(t, err)

	origPath := settings.UIDashboardPath.Get()
	origOffline := settings.UIOfflinePreferred.Get()

	settings.UIDashboardPath.Set(tempDir)
	settings.UIOfflinePreferred.Set("true")
	defer func() {
		settings.UIDashboardPath.Set(origPath)
		settings.UIOfflinePreferred.Set(origOffline)
	}()

	handler := New(nil, mockCache)

	tests := []struct {
		name           string
		path           string
		userAgent      string
		expectedBody   string
		expectedStatus int
	}{
		{
			name:           "Root redirects to dashboard",
			path:           "/",
			expectedStatus: http.StatusFound,
		},
		{
			name:           "Dashboard redirects to dashboard with trailing slash",
			path:           "/dashboard",
			expectedStatus: http.StatusFound,
		},
		{
			name:           "Dashboard serves existing asset",
			path:           "/dashboard/test.js",
			expectedBody:   dashboardAssetContent,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Dashboard missing asset falls back to index",
			path:           "/dashboard/missing.js",
			expectedBody:   indexContent,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Root-level robots.txt is served",
			path:           "/robots.txt",
			expectedBody:   robotsContent,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Favicon served from dashboard subdir",
			path:           "/favicon.png",
			expectedBody:   faviconContent,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Unknown path returns index for browser",
			path:           "/some/spa/route",
			userAgent:      "Mozilla/5.0",
			expectedBody:   indexContent,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Unknown path returns 404 for non-browser",
			path:           "/some/spa/route",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
				req.Header.Set("Accept", "*/*")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedBody != "" {
				assert.Equal(t, tt.expectedBody, rec.Body.String())
			}
		})
	}
}
