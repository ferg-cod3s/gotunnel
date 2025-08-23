//go:build !windows

package cert

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func runAsUser(name string, arg ...string) error {
	originalUser, err := getCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	uid, err := strconv.Atoi(originalUser.Uid)
	if err != nil {
		return fmt.Errorf("failed to parse user ID: %w", err)
	}
	gid, err := strconv.Atoi(originalUser.Gid)
	if err != nil {
		return fmt.Errorf("failed to parse group ID: %w", err)
	}

	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uint32(uid),
			Gid: uint32(gid),
		},
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
