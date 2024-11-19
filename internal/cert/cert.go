package cert

import (
	"crypto/tls"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

type CertManager struct {
	certsDir string
}

func New(certsDir string) *CertManager {
	return &CertManager{
		certsDir: certsDir,
	}
}

func getCurrentUser() (*user.User, error) {
	return user.Current()
}

func (m *CertManager) EnsureMkcertInstalled() error {
	// Check if mkcert is already installed
	_, err := exec.LookPath("mkcert")
	if err == nil {
		return nil
	}

	// Install mkcert based on the platform
	var installCmd string
	switch runtime.GOOS {
	case "darwin":
		installCmd = "brew install mkcert"
	case "linux":
		// Add more package managers as needed
		if _, err := exec.LookPath("apt-get"); err == nil {
			installCmd = "apt-get install -y mkcert"
		} else if _, err := exec.LookPath("yum"); err == nil {
			installCmd = "yum install -y mkcert"
		} else {
			return fmt.Errorf("no supported package manager found")
		}
	case "windows":
		if _, err := exec.LookPath("choco"); err == nil {
			installCmd = "choco install mkcert"
		} else {
			return fmt.Errorf("chocolatey package manager not found")
		}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmdParts := strings.Fields(installCmd)
	if err := runAsUser(cmdParts[0], cmdParts[1:]...); err != nil {
		return fmt.Errorf("failed to install mkcert: %w", err)
	}

	// Initialize mkcert
	if err := runAsUser("mkcert", "-install"); err != nil {
		return fmt.Errorf("failed to initialize mkcert: %w", err)
	}

	return nil
}

func (m *CertManager) EnsureCert(domain string) (*tls.Certificate, error) {
	if err := os.MkdirAll(m.certsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create certs directory: %w", err)
	}

	certFile := filepath.Join(m.certsDir, domain+".pem")
	keyFile := filepath.Join(m.certsDir, domain+"-key.pem")

	// Check if certificate already exists
	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			// Both files exist, load and return the certificate
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load existing certificate: %w", err)
			}
			return &cert, nil
		}
	}

	// Generate new certificate
	if err := runAsUser("mkcert", "-cert-file", certFile, "-key-file", keyFile, domain); err != nil {
		return nil, fmt.Errorf("failed to generate certificate: %w", err)
	}

	// Load and return the new certificate
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load new certificate: %w", err)
	}

	return &cert, nil
}
