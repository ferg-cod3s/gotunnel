//go:build !darwin && !linux

package main

import "fmt"

func InstallHelper() error {
	return fmt.Errorf("privileged helper not supported on this platform")
}

func UninstallHelper() error {
	return fmt.Errorf("privileged helper not supported on this platform")
}
