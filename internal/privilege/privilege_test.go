package privilege

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckPrivileges(t *testing.T) {
	// Skip if running tests as root/admin to avoid false positives
	if HasRootPrivileges() {
		t.Skip("Skipping test as it's running with root/admin privileges")
	}

	err := CheckPrivileges()
	if runtime.GOOS == "windows" {
		// On Windows, the test behavior depends on whether it's run as admin
		if HasRootPrivileges() {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
	} else {
		// On Unix systems, should fail when not root
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be run with sudo or as root")
	}
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
	if os.Geteuid() == 0 {
		assert.NoError(t, err)
	} else {
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be run with sudo or as root")
	}
}

func TestCheckWindowsPrivileges(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows privilege test on non-Windows platform")
	}

	err := checkWindowsPrivileges()
	// Log the result since it depends on how the test is run
	t.Logf("Windows privilege check result: %v", err)
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

	if isRoot {
		assert.NoError(t, err, "Privilege check should pass when running as root/admin")
	} else {
		assert.Error(t, err, "Privilege check should fail when not running as root/admin")
	}
}
