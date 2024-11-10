package privilege

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func CheckPrivileges() error {
	switch runtime.GOOS {
	case "windows":
		return checkWindowsPrivileges()
	default: // Linux, macOS, BSD, etc.
		return checkUnixPrivileges()
	}
}

func checkUnixPrivileges() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("this program must be run with sudo or as root")
	}
	return nil
}

func checkWindowsPrivileges() error {
	cmd := exec.Command("net", "session")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("this program must be run as Administrator")
	}
	return nil
}

func ElevatePrivileges() error {
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
