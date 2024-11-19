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
	originalStateFile := getStateFile
	getStateFile = func() string {
		return filepath.Join(tempDir, "tunnels.yaml")
	}

	cleanup := func() {
		getStateFile = originalStateFile
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func TestSaveAndLoadTunnels(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	// Create test tunnel states
	tunnels := []TunnelState{
		{
			Port:   8080,
			Domain: "test1.local",
			HTTPS:  false,
		},
		{
			Port:   8443,
			Domain: "test2.local",
			HTTPS:  true,
		},
	}

	// Test saving tunnels
	err := SaveTunnels(tunnels)
	require.NoError(t, err)

	// Test loading tunnels
	loadedTunnels, err := LoadTunnels()
	require.NoError(t, err)

	// Verify loaded tunnels match saved tunnels
	assert.Equal(t, len(tunnels), len(loadedTunnels))
	for i, tunnel := range tunnels {
		assert.Equal(t, tunnel.Port, loadedTunnels[i].Port)
		assert.Equal(t, tunnel.Domain, loadedTunnels[i].Domain)
		assert.Equal(t, tunnel.HTTPS, loadedTunnels[i].HTTPS)
	}
}

func TestLoadTunnelsNoFile(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	// Try loading when no file exists
	tunnels, err := LoadTunnels()
	require.NoError(t, err)
	assert.Empty(t, tunnels)
}

func TestSaveTunnelsCreateDirectory(t *testing.T) {
	tempDir, cleanup := setupTestStateDir(t)
	defer cleanup()

	// Remove the directory to test creation
	os.RemoveAll(tempDir)

	tunnels := []TunnelState{
		{
			Port:   8080,
			Domain: "test.local",
			HTTPS:  false,
		},
	}

	// Save should create the directory
	err := SaveTunnels(tunnels)
	require.NoError(t, err)

	// Verify directory was created
	_, err = os.Stat(tempDir)
	assert.NoError(t, err)
}

func TestSaveAndLoadEmptyTunnels(t *testing.T) {
	_, cleanup := setupTestStateDir(t)
	defer cleanup()

	// Save empty tunnel list
	err := SaveTunnels([]TunnelState{})
	require.NoError(t, err)

	// Load empty tunnel list
	tunnels, err := LoadTunnels()
	require.NoError(t, err)
	assert.Empty(t, tunnels)
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

	// Save tunnels
	err := SaveTunnels(tunnels)
	require.NoError(t, err)

	// Read the raw file contents
	stateFile := getStateFile()
	content, err := os.ReadFile(stateFile)
	require.NoError(t, err)

	// Verify YAML format
	expectedContent := `- port: 8080
  domain: test.local
  https: true
`
	assert.Equal(t, expectedContent, string(content))
}
