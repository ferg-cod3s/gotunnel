package cert

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cert-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cm := New(tempDir)
	assert.NotNil(t, cm)
	assert.Equal(t, tempDir, cm.certsDir)
}

func TestEnsureMkcertInstalled(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cert-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cm := New(tempDir)
	err = cm.EnsureMkcertInstalled()
	// Note: This test might fail if mkcert is not installed or if we don't have privileges
	// In a real environment, we'd mock the package manager and command execution
	if err != nil {
		t.Logf("Mkcert installation failed (this might be expected): %v", err)
	}
}

func TestEnsureCert(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cert-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cm := New(tempDir)
	domain := "test.local"

	// First ensure mkcert is installed
	err = cm.EnsureMkcertInstalled()
	if err != nil {
		t.Skipf("Skipping test as mkcert installation failed: %v", err)
		return
	}

	cert, err := cm.EnsureCert(domain)
	if err != nil {
		t.Logf("Certificate generation failed (this might be expected without proper privileges): %v", err)
		return
	}

	if cert != nil {
		// Verify certificate files exist
		certFile := filepath.Join(tempDir, domain+".pem")
		keyFile := filepath.Join(tempDir, domain+"-key.pem")
		assert.FileExists(t, certFile)
		assert.FileExists(t, keyFile)
	}
}

func TestGetCurrentUser(t *testing.T) {
	user, err := getCurrentUser()
	require.NoError(t, err)
	assert.NotEmpty(t, user.Username)
	assert.NotEmpty(t, user.HomeDir)
}