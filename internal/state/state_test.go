package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStateDir(t *testing.T) (string, func()) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gotunnel-state-test-*")
	require.NoError(t, err)

	// Override the state file location for testing
	originalStateFile := getStateFileFunc
	getStateFileFunc = func() string {
		return filepath.Join(tempDir, "tunnels.yaml")
	}

	cleanup := func() {
		getStateFileFunc = originalStateFile
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func TestSaveAndLoadTunnels(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	// Test data
	tunnels := []TunnelState{
		{Port: 8080, Domain: "test1.local", HTTPS: false},
		{Port: 8443, Domain: "test2.local", HTTPS: true},
	}

	// Save tunnels
	err := SaveTunnels(tunnels)
	require.NoError(t, err)

	// Load tunnels
	loadedTunnels, err := LoadTunnels()
	require.NoError(t, err)

	// Compare
	assert.Equal(t, tunnels, loadedTunnels)
}

func TestLoadTunnelsNoFile(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	tunnels, err := LoadTunnels()
	require.NoError(t, err)
	assert.Empty(t, tunnels)
}

func TestSaveTunnelsCreateDirectory(t *testing.T) {
	tempDir, cleanup := setupTestStateDir(t)
	defer cleanup()

	// Create a deeper path that doesn't exist
	getStateFileFunc = func() string {
		return filepath.Join(tempDir, "deep", "deeper", "tunnels.yaml")
	}

	tunnels := []TunnelState{
		{Port: 8080, Domain: "test.local", HTTPS: false},
	}

	err := SaveTunnels(tunnels)
	require.NoError(t, err)

	// Verify the file exists
	_, err = os.Stat(filepath.Join(tempDir, "deep", "deeper", "tunnels.yaml"))
	assert.NoError(t, err)
}

func TestSaveAndLoadEmptyTunnels(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	var tunnels []TunnelState

	err := SaveTunnels(tunnels)
	require.NoError(t, err)

	loadedTunnels, err := LoadTunnels()
	require.NoError(t, err)
	assert.Empty(t, loadedTunnels)
}

func TestTunnelStateSerialization(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	tunnels := []TunnelState{
		{
			Port:   8080,
			Domain: "test.local",
			HTTPS:  true,
		},
	}

	// Save and load to test serialization
	err := SaveTunnels(tunnels)
	require.NoError(t, err)

	loadedTunnels, err := LoadTunnels()
	require.NoError(t, err)

	// Check if all fields are preserved
	require.Len(t, loadedTunnels, 1)
	assert.Equal(t, 8080, loadedTunnels[0].Port)
	assert.Equal(t, "test.local", loadedTunnels[0].Domain)
	assert.Equal(t, true, loadedTunnels[0].HTTPS)
}
