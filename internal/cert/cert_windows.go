//go:build windows

package cert

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func runAsUser(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{}

	originalUser, err := getCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HOME=%s", originalUser.HomeDir),
		fmt.Sprintf("USER=%s", originalUser.Username),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}
