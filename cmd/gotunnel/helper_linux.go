//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
)

const systemdServicePath = "/etc/systemd/system/gotunnel-helper.service"

const systemdService = `[Unit]
Description=gotunnel port forwarder (80->8080, 443->8443)
After=network.target

[Service]
ExecStart=%s port-forward
Restart=always

[Install]
WantedBy=multi-user.target
`

// InstallHelper sets up the privileged port forwarder as a systemd service.
func InstallHelper() error {
	if os.Getuid() != 0 {
		return fmt.Errorf("install-helper requires root: run with sudo")
	}

	if err := copySelfToHelper(); err != nil {
		return err
	}

	serviceContent := fmt.Sprintf(systemdService, helperTargetPath)
	if err := os.WriteFile(systemdServicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("cannot write service file: %w", err)
	}

	exec.Command("systemctl", "daemon-reload").Run()
	if err := exec.Command("systemctl", "enable", "--now", "gotunnel-helper").Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	fmt.Println("gotunnel helper installed.")
	fmt.Println("  Ports: :80 -> :8080, :443 -> :8443")
	fmt.Println("  Starts on boot. Run 'sudo gotunnel uninstall-helper' to remove.")
	return nil
}

// UninstallHelper removes the privileged port forwarder.
func UninstallHelper() error {
	if os.Getuid() != 0 {
		return fmt.Errorf("uninstall-helper requires root: run with sudo")
	}

	exec.Command("systemctl", "disable", "--now", "gotunnel-helper").Run()
	os.Remove(systemdServicePath)
	exec.Command("systemctl", "daemon-reload").Run()
	os.Remove(helperTargetPath)

	fmt.Println("gotunnel helper removed.")
	return nil
}
