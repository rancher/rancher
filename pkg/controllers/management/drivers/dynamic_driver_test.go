package drivers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDynamicDriver(t *testing.T) {
	// Builtin driver without prefix
	d, err := NewDynamicDriver(true, "amazonec2", "", "")
	require.NoError(t, err)
	assert.True(t, d.Builtin)
	assert.Equal(t, "docker-machine-driver-amazonec2", d.DriverName)

	// Custom driver already having prefix
	d, err = NewDynamicDriver(false, "docker-machine-driver-custom", "https://example.com/custom.zip", fmt.Sprintf("%x", sha256.Sum256([]byte("helloworld"))))
	require.NoError(t, err)
	assert.False(t, d.Builtin)
	assert.Equal(t, "docker-machine-driver-custom", d.DriverName)

	// Custom driver with invalid hash format
	_, err = NewDynamicDriver(false, "custom", "https://example.com/custom.zip", "invalid-hash")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hash format")
}

func TestDynamicDriverAssetPath(t *testing.T) {
	d, err := NewDynamicDriver(false, "linode", "https://example.com/linode.zip", "")
	require.NoError(t, err)
	expected := filepath.Join(settings.UIDashboardPath.Get(), "assets", "docker-machine-driver-linode")
	assert.Equal(t, expected, d.assetPath())

	// With AssetDir override
	d.AssetDir = "/custom/assets"
	assert.Equal(t, "/custom/assets/docker-machine-driver-linode", d.assetPath())
}

func TestDynamicDriverValid(t *testing.T) {
	// Builtin driver is always valid
	builtin, err := NewDynamicDriver(true, "amazonec2", "", "")
	require.NoError(t, err)
	assert.True(t, builtin.Valid())

	// Non-existent driver is not valid
	nonExistent, err := NewDynamicDriver(false, "nonexistent", "https://example.com/nonexistent", "")
	require.NoError(t, err)
	assert.False(t, nonExistent.Valid())

	// Staged driver
	tempDir := t.TempDir()
	driver, err := NewDynamicDriver(false, "testdriver", "https://example.com/test", "")
	require.NoError(t, err)
	driver.CacheDir = filepath.Join(tempDir, "cache")
	driver.BinDir = filepath.Join(tempDir, "bin")
	driver.AssetDir = filepath.Join(tempDir, "assets")

	binaryContent := []byte("#!/bin/sh\necho test\n")
	hasher := sha256.New()
	hasher.Write(binaryContent)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	// Create srcBinName (the binary)
	require.NoError(t, os.MkdirAll(filepath.Dir(driver.srcBinName()), 0755))
	require.NoError(t, os.WriteFile(driver.srcBinName(), binaryContent, 0755))

	// Create srcBinHashFile (the hash)
	require.NoError(t, os.WriteFile(driver.srcBinHashFile(), []byte(expectedHash), 0644))

	// Create binName (in bin directory)
	require.NoError(t, os.MkdirAll(filepath.Dir(driver.binName()), 0755))
	require.NoError(t, os.WriteFile(driver.binName(), binaryContent, 0755))

	// Before assetPath exists, Valid() should be false
	assert.False(t, driver.Valid(), "expected false when assetPath does not exist")

	// Create assetPath with matching content
	require.NoError(t, os.MkdirAll(filepath.Dir(driver.assetPath()), 0755))
	require.NoError(t, os.WriteFile(driver.assetPath(), binaryContent, 0755))
	assert.True(t, driver.Valid(), "expected true when all files exist and hash matches")

	// Modifying assetPath content should cause hash mismatch
	require.NoError(t, os.WriteFile(driver.assetPath(), []byte("corrupted-content"), 0755))
	assert.False(t, driver.Valid(), "expected false when assetPath hash does not match")
}

func TestDynamicDriverInstall(t *testing.T) {
	tempDir := t.TempDir()
	driver, err := NewDynamicDriver(false, "installdriver", "https://example.com/install", "")
	require.NoError(t, err)
	driver.CacheDir = filepath.Join(tempDir, "cache")
	driver.BinDir = filepath.Join(tempDir, "bin")
	driver.AssetDir = filepath.Join(tempDir, "assets")

	content := []byte("driver-binary-data")
	require.NoError(t, os.MkdirAll(filepath.Dir(driver.srcBinName()), 0755))
	require.NoError(t, os.WriteFile(driver.srcBinName(), content, 0755))
	require.NoError(t, os.MkdirAll(filepath.Dir(driver.binName()), 0755))
	require.NoError(t, os.MkdirAll(filepath.Dir(driver.assetPath()), 0755))

	// In DevMode=true, copies to binName but skips assetPath
	driver.DevMode = true
	require.NoError(t, driver.Install())

	binData, err := os.ReadFile(driver.binName())
	require.NoError(t, err)
	assert.Equal(t, content, binData)

	_, err = os.Stat(driver.assetPath())
	assert.True(t, os.IsNotExist(err), "expected assetPath to be skipped in dev mode")

	// In production mode (DevMode=false), copies to both binName and assetPath (creating parent dir)
	driver.DevMode = false
	require.NoError(t, driver.Install())

	assetData, err := os.ReadFile(driver.assetPath())
	require.NoError(t, err)
	assert.Equal(t, content, assetData)
}

func TestHashFile(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "test-file")
	content := []byte("hello world")
	require.NoError(t, os.WriteFile(tempFile, content, 0644))

	hash, err := sha256sum(tempFile)
	require.NoError(t, err)

	expectedSum := sha256.Sum256(content)
	assert.Equal(t, hex.EncodeToString(expectedSum[:]), hash)
}
