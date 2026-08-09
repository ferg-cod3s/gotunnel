//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
)

const launchdPlistPath = "/Library/LaunchDaemons/com.gotunnel.helper.plist"

const launchdPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.gotunnel.helper</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>port-forward</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/gotunnel-helper.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/gotunnel-helper.log</string>
</dict>
</plist>
`

// InstallHelper sets up the privileged port forwarder as a launchd daemon.
func InstallHelper() error {
	if os.Getuid() != 0 {
		return fmt.Errorf("install-helper requires root: run with sudo")
	}

	if err := copySelfToHelper(); err != nil {
		return err
	}

	plistContent := fmt.Sprintf(launchdPlist, helperTargetPath)
	if err := os.WriteFile(launchdPlistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("cannot write plist: %w", err)
	}

	// Unload if already loaded, then load fresh
	exec.Command("launchctl", "unload", launchdPlistPath).Run()
	if err := exec.Command("launchctl", "load", launchdPlistPath).Run(); err != nil {
		return fmt.Errorf("failed to load launchd service: %w", err)
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

	exec.Command("launchctl", "unload", launchdPlistPath).Run()
	os.Remove(launchdPlistPath)
	os.Remove(helperTargetPath)

	fmt.Println("gotunnel helper removed.")
	return nil
}
