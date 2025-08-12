package privilege

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
)

func CheckPrivileges() error {
	// log.Println("Checking privileges...")
	switch runtime.GOOS {
	case "windows":
		return checkWindowsPrivileges()
	default: // Linux, macOS, BSD, etc.
		return checkUnixPrivileges()
	}
}

func checkUnixPrivileges() error {
	// Remove the privilege check to allow non-root execution
	return nil
}

func checkWindowsPrivileges() error {
	// Remove the privilege check to allow non-admin execution
	return nil
}

func ElevatePrivileges() error {
	log.Println("Attempting to elevate privileges...")
	if runtime.GOOS == "windows" {
		return elevateWindows()
	}
	return elevateSudo()
}

func elevateWindows() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command("powershell.exe", "Start-Process", exe, "-Verb", "RunAs")
	return cmd.Run()
}

func elevateSudo() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command("sudo", exe)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// HasRootPrivileges checks if the current process has root privileges
func HasRootPrivileges() bool {
	if runtime.GOOS == "windows" {
		return hasWindowsAdminPrivileges()
	}
	return os.Geteuid() == 0
}

// hasWindowsAdminPrivileges checks if running as admin on Windows
func hasWindowsAdminPrivileges() bool {
	// On Windows, we'll check if we can write to a system directory
	// This is a simple heuristic, not perfect but works for most cases
	_, err := os.Stat("C:\\Windows\\System32")
	if err != nil {
		return false
	}
	
	// Try to create a temp file in system32 (will fail if not admin)
	testFile := "C:\\Windows\\System32\\gotunnel_admin_test.tmp"
	file, err := os.Create(testFile)
	if err != nil {
		return false
	}
	file.Close()
	os.Remove(testFile) // Clean up
	return true
}
