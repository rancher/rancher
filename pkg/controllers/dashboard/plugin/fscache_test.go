package plugin

import (
	"io/fs"
	"os"
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
)

func TestSyncWithIndex(t *testing.T) {
	testCases := []struct {
		Name            string
		CurrentEntries  map[string]*v1.UIPluginEntry
		ExpectedEntries map[string]*v1.UIPluginEntry
		FsCacheFiles    []string
		ShouldDelete    bool
	}{
		{
			Name: "Sync index with FS cache no new entries",
			CurrentEntries: map[string]*v1.UIPluginEntry{
				"test-plugin": {
					Name:     "test-plugin",
					Version:  "0.1.0",
					Endpoint: "https://test.endpoint.svc",
					NoCache:  false,
					Metadata: map[string]string{
						"test": "data",
					},
				},
			},
			ExpectedEntries: map[string]*v1.UIPluginEntry{
				"test-plugin": {
					Name:     "test-plugin",
					Version:  "0.1.0",
					Endpoint: "https://test.endpoint.svc",
					NoCache:  false,
					Metadata: map[string]string{
						"test": "data",
					},
				},
			},
			FsCacheFiles: []string{
				FSCacheRootDir + "/test-plugin/0.1.0",
			},
		},
		{
			Name: "Sync index with FS cache delete old test-plugin-2 entry",
			CurrentEntries: map[string]*v1.UIPluginEntry{
				"test-plugin": {
					Name:     "test-plugin",
					Version:  "0.1.0",
					Endpoint: "https://test.endpoint.svc",
					NoCache:  false,
					Metadata: map[string]string{
						"test": "data",
					},
				},
			},
			ExpectedEntries: map[string]*v1.UIPluginEntry{
				"test-plugin": {
					Name:     "test-plugin",
					Version:  "0.1.0",
					Endpoint: "https://test.endpoint.svc",
					NoCache:  false,
					Metadata: map[string]string{
						"test": "data",
					},
				},
			},
			FsCacheFiles: []string{
				FSCacheRootDir + "/test-plugin/0.1.0",
				FSCacheRootDir + "/test-plugin-2/0.1.0",
			},
			ShouldDelete: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			var (
				osRemoveAllCalled   bool
				actualPluginDeleted string
			)
			if tc.ShouldDelete {
				osRemoveAll = func(path string) error {
					osRemoveAllCalled = true
					actualPluginDeleted = path
					return nil
				}
			}
			fsCache := FSCache{}
			index := SafeIndex{
				Entries: tc.CurrentEntries,
			}
			err := fsCache.SyncWithIndex(&index, tc.FsCacheFiles)
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, index.Entries, tc.CurrentEntries)
			if tc.ShouldDelete {
				assert.Equal(t, tc.ShouldDelete, osRemoveAllCalled)
				expectedPluginDeleted := FSCacheRootDir + "/test-plugin-2"
				assert.Equal(t, expectedPluginDeleted, actualPluginDeleted)
			}
		})
	}
}

func Test_isCached(t *testing.T) {
	testCases := []struct {
		Name          string
		PluginName    string
		PluginVersion string
		Expected      bool
		OsStatErr     error
		IsDirEmptyVal bool
	}{
		{
			Name:          "Test cached plugin",
			PluginName:    "test-plugin",
			PluginVersion: "0.0.1",
			Expected:      true,
			OsStatErr:     nil,
			IsDirEmptyVal: false,
		},
		{
			Name:          "Test non cached plugin",
			PluginName:    "test-plugin",
			PluginVersion: "0.0.1",
			Expected:      false,
			OsStatErr:     os.ErrNotExist,
			IsDirEmptyVal: true,
		},
		{
			Name:          "Test cached plugin but dir is empty",
			PluginName:    "test-plugin",
			PluginVersion: "0.0.1",
			Expected:      false,
			OsStatErr:     nil,
			IsDirEmptyVal: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			osStat = func(name string) (fs.FileInfo, error) {
				return nil, tc.OsStatErr
			}
			isDirEmpty = func(path string) (bool, error) {
				return tc.IsDirEmptyVal, nil
			}
			fsCache := FSCache{}
			actual, err := fsCache.isCached(tc.PluginName, tc.PluginVersion)
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, tc.Expected, actual)
		})
	}
}
