package plugin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
)

func TestSyncWithIndex(t *testing.T) {
	testCases := []struct {
		Name                  string
		CurrentEntries        map[string]*UIPlugin
		ExpectedEntries       map[string]*UIPlugin
		FsCacheFiles          []string
		ShouldDelete          bool
		ExpectedPluginDeleted string
	}{
		{
			Name: "Sync index with FS cache no new entries",
			CurrentEntries: map[string]*UIPlugin{
				"test-plugin": {
					UIPluginEntry: v1.UIPluginEntry{
						Name:     "test-plugin",
						Version:  "0.1.0",
						Endpoint: "https://test.endpoint.svc",
						NoCache:  false,
						Metadata: map[string]string{
							"test": "data",
						},
					},
				},
			},
			ExpectedEntries: map[string]*UIPlugin{
				"test-plugin": {
					UIPluginEntry: v1.UIPluginEntry{
						Name:     "test-plugin",
						Version:  "0.1.0",
						Endpoint: "https://test.endpoint.svc",
						NoCache:  false,
						Metadata: map[string]string{
							"test": "data",
						},
					},
				},
			},
			FsCacheFiles: []string{
				FSCacheRootDir + "/test-plugin/0.1.0",
			},
		},
		{
			Name: "Sync index with FS cache delete old test-plugin-2 entry",
			CurrentEntries: map[string]*UIPlugin{
				"test-plugin": {
					UIPluginEntry: v1.UIPluginEntry{
						Name:     "test-plugin",
						Version:  "0.1.0",
						Endpoint: "https://test.endpoint.svc",
						NoCache:  false,
						Metadata: map[string]string{
							"test": "data",
						},
					},
				},
			},
			ExpectedEntries: map[string]*UIPlugin{
				"test-plugin": {
					UIPluginEntry: v1.UIPluginEntry{
						Name:     "test-plugin",
						Version:  "0.1.0",
						Endpoint: "https://test.endpoint.svc",
						NoCache:  false,
						Metadata: map[string]string{
							"test": "data",
						},
					},
				},
			},
			FsCacheFiles: []string{
				FSCacheRootDir + "/test-plugin/0.1.0",
				FSCacheRootDir + "/test-plugin-2/0.1.0",
			},
			ShouldDelete:          true,
			ExpectedPluginDeleted: FSCacheRootDir + "/test-plugin-2",
		},
		{
			Name: "Sync with nil entry does not panic",
			CurrentEntries: map[string]*UIPlugin{
				"test-plugin": nil,
			},
			ExpectedEntries: map[string]*UIPlugin{
				"test-plugin": nil,
			},
			FsCacheFiles: []string{
				FSCacheRootDir + "/test-plugin/0.1.0",
			},
			ShouldDelete:          true,
			ExpectedPluginDeleted: FSCacheRootDir + "/test-plugin",
		},
		{
			Name:            "Sync with nil UIPluginEntry does not panic",
			CurrentEntries:  map[string]*UIPlugin{},
			ExpectedEntries: map[string]*UIPlugin{},
			FsCacheFiles: []string{
				FSCacheRootDir + "/test-plugin/0.1.0",
			},
			ShouldDelete:          true,
			ExpectedPluginDeleted: FSCacheRootDir + "/test-plugin",
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
				if tc.ExpectedPluginDeleted != "" {
					assert.Equal(t, tc.ExpectedPluginDeleted, actualPluginDeleted)
				}
			}
		})
	}
}

func TestSyncWithIndexConcurrentAccess(t *testing.T) {
	testCases := []struct {
		Name       string
		NumPlugins int
		Iterations int
	}{
		{
			Name:       "SyncWithIndex is safe during concurrent index generation",
			NumPlugins: 20,
			Iterations: 300,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			originalRemoveAll := osRemoveAll
			osRemoveAll = func(path string) error {
				return nil
			}
			defer func() {
				osRemoveAll = originalRemoveAll
			}()

			plugins := make([]*v1.UIPlugin, tc.NumPlugins)
			fsCacheFiles := make([]string, tc.NumPlugins)

			for i := 0; i < tc.NumPlugins; i++ {
				name := fmt.Sprintf("test-plugin-%d", i)
				plugins[i] = &v1.UIPlugin{
					Spec: v1.UIPluginSpec{
						Plugin: v1.UIPluginEntry{
							Name:    name,
							Version: "0.1.0",
						},
					},
				}

				fsCacheFiles[i] = fmt.Sprintf("%s/%s/0.1.0/plugin.js", FSCacheRootDir, name)
			}

			index := &SafeIndex{}
			fsCache := FSCache{}

			assert.NoError(t, index.Generate(plugins))
			ready := make(chan struct{})

			var wg sync.WaitGroup
			wg.Add(2)

			go func() {
				defer wg.Done()
				<-ready

				for i := 0; i < tc.Iterations; i++ {
					_ = fsCache.SyncWithIndex(index, fsCacheFiles)
				}
			}()

			go func() {
				defer wg.Done()
				<-ready

				for i := 0; i < tc.Iterations; i++ {
					_ = index.Generate(plugins)
				}
			}()

			close(ready)
			wg.Wait()
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

func Test_getChartNameAndVersion(t *testing.T) {
	testCases := []struct {
		name                 string
		input                string
		expectedChartName    string
		expectedChartVersion string
		expectedError        error
	}{
		{
			name:                 "chart rooted at FSCacheRootDir",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "istio/v2.1.0"),
			expectedChartName:    "istio",
			expectedChartVersion: "v2.1.0",
			expectedError:        nil,
		},
		{
			name:                 "chart rooted at FSCacheRootDir with rc version",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "istio/v2.1.0-rc1"),
			expectedChartName:    "istio",
			expectedChartVersion: "v2.1.0-rc1",
			expectedError:        nil,
		},
		{
			name:                 "file rooted at FSCacheRootDir",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "istio/2.1.0/Chart.yaml"),
			expectedChartName:    "istio",
			expectedChartVersion: "2.1.0",
			expectedError:        nil,
		},
		{
			name:                 "chart rooted at FSCacheRootDir without version",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "istio"),
			expectedChartName:    "",
			expectedChartVersion: "",
			expectedError:        fmt.Errorf("file path is not valid"),
		},
		{
			name:                 "chart rooted at FSCacheRootDir with invalid version",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "istio/invalid-version"),
			expectedChartName:    "",
			expectedChartVersion: "",
			expectedError:        fmt.Errorf("invalid chart version"),
		},
		{
			name:                 "chart not rooted at FSCacheRootDir",
			input:                "/home/wrong-path/bad-chart/v1.0.0",
			expectedChartName:    "",
			expectedChartVersion: "",
			expectedError:        fmt.Errorf("path root is not the root cache path"),
		},
		{
			name:                 "chart not rooted at FSCacheRootDir with ../",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "../bad-chart/v2.1.0"),
			expectedChartName:    "",
			expectedChartVersion: "",
			expectedError:        fmt.Errorf("path root is not the root cache path"),
		},
		{
			name:                 "chart not rooted at FSCacheRootDir 2",
			input:                fmt.Sprintf("%s/%s", FSCacheRootDir, "/istio/../../bad-chart/v2.1.0"),
			expectedChartName:    "",
			expectedChartVersion: "",
			expectedError:        fmt.Errorf("path root is not the root cache path"),
		},
	}

	for _, testCase := range testCases {

		chartName, chartVersion, err := getChartNameAndVersion(testCase.input)
		assert.Equal(t, testCase.expectedChartName, chartName, testCase.name)
		assert.Equal(t, testCase.expectedChartVersion, chartVersion, testCase.name)
		if testCase.expectedError != nil {
			assert.ErrorContains(t, err, testCase.expectedError.Error(), testCase.name)
		} else {
			assert.NoError(t, err, testCase.name)
		}
	}
}

func TestUntar(t *testing.T) {
	testCases := []struct {
		Name          string
		Files         []string
		ExpectedFiles []string
	}{
		{
			Name: "Extract files with explicit directory entries",
			Files: []string{
				"1.0.0/",
				"1.0.0/files.txt",
			},
			ExpectedFiles: []string{
				"1.0.0/files.txt",
			},
		},
		{
			Name: "Extract files without explicit directory entries",
			Files: []string{
				"1.0.0/files.txt",
			},
			ExpectedFiles: []string{
				"1.0.0/files.txt",
			},
		},
		{
			Name: "Extract files from nested directories without directory entries",
			Files: []string{
				"1.0.0/assets/plugin.js",
			},
			ExpectedFiles: []string{
				"1.0.0/assets/plugin.js",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			dst := t.TempDir()
			archive := createTarGz(t, tc.Files)

			err := Untar(dst, bytes.NewReader(archive))
			assert.NoError(t, err)

			for _, expectedFile := range tc.ExpectedFiles {
				filePath := filepath.Join(dst, expectedFile)

				info, err := os.Stat(filePath)
				assert.NoError(t, err)
				assert.False(t, info.IsDir())
			}
		})
	}
}

func createTarGz(t *testing.T, files []string) []byte {
	t.Helper()

	var buffer bytes.Buffer

	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)

	for _, file := range files {
		if strings.HasSuffix(file, "/") {
			err := tarWriter.WriteHeader(&tar.Header{
				Name:     file,
				Mode:     0755,
				Typeflag: tar.TypeDir,
			})
			assert.NoError(t, err)

			continue
		}

		content := []byte(file)

		err := tarWriter.WriteHeader(&tar.Header{
			Name:     file,
			Mode:     0644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		})
		assert.NoError(t, err)

		_, err = tarWriter.Write(content)
		assert.NoError(t, err)
	}

	assert.NoError(t, tarWriter.Close())
	assert.NoError(t, gzipWriter.Close())

	return buffer.Bytes()
}

func Test_validateFilesTxtEntries(t *testing.T) {
	testCases := []struct {
		Name      string
		Files     []string
		ShouldErr bool
	}{
		{
			Name:      "valid files",
			Files:     []string{"plugin/file.js", "plugin/folder/file.min.js"},
			ShouldErr: false,
		},
		{
			Name:      "valid encoded files",
			Files:     []string{"plugins%2Ffile.js", "plugin%2Ffolder%2Ffile.min.js"},
			ShouldErr: false,
		},
		{
			Name:      "invalid file starting with /",
			Files:     []string{"/plugin/file.js", "plugin/folder/file.js"},
			ShouldErr: true,
		},
		{
			Name:      "invalid encoded file starting with /",
			Files:     []string{"%2Fplugin%2Ffile.js", "plugin/folder/file.js"},
			ShouldErr: true,
		},
		{
			Name:      "invalid file starting with \\",
			Files:     []string{"\\plugin/file.js", "plugin/folder/file.js"},
			ShouldErr: true,
		},
		{
			Name:      "invalid file starting with ..",
			Files:     []string{"plugin/file.js", "../plugin/folder/file.js"},
			ShouldErr: true,
		},
		{
			Name:      "invalid file starting with .",
			Files:     []string{"plugin/file.js", ".plugin/folder/file.js"},
			ShouldErr: true,
		},
		{
			Name:      "invalid file containing ..",
			Files:     []string{"plugin/file.js", "plugin/folder/../file.js"},
			ShouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			err := validateFilesTxtEntries(tc.Files)
			if tc.ShouldErr {
				assert.ErrorContains(t, err, "invalid file entry")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
