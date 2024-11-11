package privilege

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
)

func CheckPrivileges() error {
	log.Println("Checking privileges...")
	switch runtime.GOOS {
	case "windows":
		return checkWindowsPrivileges()
	default: // Linux, macOS, BSD, etc.
		return checkUnixPrivileges()
	}
}

func checkUnixPrivileges() error {
	if os.Geteuid() != 0 {
		log.Println("Privilege check failed: must be run with sudo or as root")
		return fmt.Errorf("this program must be run with sudo or as root")
	}
	log.Println("Privilege check passed for Unix")
	return nil
}

func checkWindowsPrivileges() error {
	cmd := exec.Command("net", "session")
	if err := cmd.Run(); err != nil {
		log.Println("Privilege check failed: must be run as Administrator")
		return fmt.Errorf("this program must be run as Administrator")
	}
	log.Println("Privilege check passed for Windows")
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
