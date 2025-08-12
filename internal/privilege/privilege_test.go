package privilege

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckPrivileges(t *testing.T) {
	err := CheckPrivileges()
	// Since we've disabled privilege checks to allow non-root execution,
	// CheckPrivileges should always return nil
	assert.NoError(t, err)
}

func TestHasRootPrivileges(t *testing.T) {
	isRoot := HasRootPrivileges()

	if runtime.GOOS == "windows" {
		// On Windows, check if running as admin
		// The actual result depends on how the test is run
		t.Logf("Running with admin privileges: %v", isRoot)
	} else {
		// On Unix systems, we can check the effective user ID
		expectedRoot := os.Geteuid() == 0
		assert.Equal(t, expectedRoot, isRoot)
	}
}

func TestCheckUnixPrivileges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix privilege test on Windows")
	}

	err := checkUnixPrivileges()
	// Since we've disabled privilege checks, this should always return nil
	assert.NoError(t, err)
}

func TestCheckWindowsPrivileges(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows privilege test on non-Windows platform")
	}

	err := checkWindowsPrivileges()
	// Since we've disabled privilege checks, this should always return nil
	assert.NoError(t, err)
}

func TestElevatePrivileges(t *testing.T) {
	// Skip if already running with privileges
	if HasRootPrivileges() {
		t.Skip("Skipping elevation test as already running with privileges")
	}

	// This is a potentially dangerous test as it might trigger UAC/sudo
	// Only run it in specific test environments
	if os.Getenv("GOTUNNEL_TEST_ELEVATION") != "1" {
		t.Skip("Skipping elevation test. Set GOTUNNEL_TEST_ELEVATION=1 to run")
	}

	err := ElevatePrivileges()
	// Just verify the function runs without panicking
	// The actual result depends on the environment
	t.Logf("Privilege elevation attempt result: %v", err)
}

func TestPrivilegeChecksIntegration(t *testing.T) {
	// Integration test combining multiple privilege checks
	isRoot := HasRootPrivileges()
	err := CheckPrivileges()

	// Since we've disabled privilege checks, this should always pass
	assert.NoError(t, err, "Privilege check should always pass now")
	
	// Just test that HasRootPrivileges doesn't panic
	t.Logf("Has root privileges: %v", isRoot)
}
