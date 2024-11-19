package cert

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return user.Lookup(sudoUser)
	}
	return user.Current()
}

func runAsUser(name string, arg ...string) error {
	if os.Getuid() != 0 {
		// Not running as root, just execute normally
		cmd := exec.Command(name, arg...)
		return cmd.Run()
	}

	// Get the original user
	originalUser, err := getCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	uid, _ := strconv.Atoi(originalUser.Uid)
	gid, _ := strconv.Atoi(originalUser.Gid)

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
		return fmt.Errorf("command failed: %w\nOutput: %s", err, output)
	}
	return nil
}

func (cm *CertManager) EnsureMkcertInstalled() error {
	// Check if mkcert is installed
	if _, err := exec.LookPath("mkcert"); err != nil {
		log.Println("mkcert not found, attempting to install...")

		switch runtime.GOOS {
		case "darwin":
			if err := runAsUser("brew", "install", "mkcert"); err != nil {
				return fmt.Errorf("failed to install mkcert: %w", err)
			}
		case "windows":
			// Try Chocolatey first
			if _, err := exec.LookPath("choco"); err == nil {
				cmd := exec.Command("choco", "install", "-y", "mkcert")
				if output, err := cmd.CombinedOutput(); err != nil {
					return fmt.Errorf("failed to install mkcert: %w\nOutput: %s", err, output)
				}
			} else if _, err := exec.LookPath("scoop"); err == nil {
				// Try Scoop if Chocolatey isn't available
				cmd := exec.Command("scoop", "bucket", "add", "extras")
				if err := cmd.Run(); err == nil {
					cmd = exec.Command("scoop", "install", "mkcert")
					if output, err := cmd.CombinedOutput(); err != nil {
						return fmt.Errorf("failed to install mkcert: %w\nOutput: %s", err, output)
					}
				}
			} else {
				return fmt.Errorf("please install Chocolatey (https://chocolatey.org) or Scoop (https://scoop.sh) to automatically install mkcert, " +
					"or install mkcert manually from: https://github.com/FiloSottile/mkcert#windows")
			}
		case "linux":
			if _, err := exec.LookPath("apt-get"); err == nil {
				cmd := exec.Command("sudo", "apt-get", "install", "-y", "mkcert")
				if output, err := cmd.CombinedOutput(); err != nil {
					return fmt.Errorf("failed to install mkcert: %w\nOutput: %s", err, output)
				}
			} else {
				return fmt.Errorf("please install mkcert manually: https://github.com/FiloSottile/mkcert#installation")
			}
		default:
			return fmt.Errorf("automatic mkcert installation not supported on %s", runtime.GOOS)
		}
	}

	// Install the local CA
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// On Windows, we need to run as admin for CA installation
		cmd = exec.Command("powershell", "Start-Process", "mkcert", "-ArgumentList", "-install", "-Verb", "RunAs")
	} else {
		cmd = exec.Command("mkcert", "-install")
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install local CA: %w\nOutput: %s", err, output)
	}

	return nil
}

func (cm *CertManager) EnsureCert(domain string) (*tls.Certificate, error) {
	// Ensure mkcert is installed
	if err := cm.EnsureMkcertInstalled(); err != nil {
		return nil, fmt.Errorf("failed to ensure mkcert is installed: %w", err)
	}

	// Ensure certs directory exists
	if err := os.MkdirAll(cm.certsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create certs directory: %w", err)
	}

	certPath := filepath.Join(cm.certsDir, domain+".crt")
	keyPath := filepath.Join(cm.certsDir, domain+".key")

	log.Printf("Generating certificate for domain: %s", domain)

	// Get local IP addresses
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Warning: Failed to get interface addresses: %v", err)
		addrs = []net.Addr{}
	}

	// Collect all local IPs
	var localIPs []string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				localIPs = append(localIPs, ipnet.IP.String())
			}
		}
	}

	// Prepare mkcert arguments with all possible local names
	mkcertArgs := []string{
		"-cert-file", certPath,
		"-key-file", keyPath,
		domain,                               // example.local
		"*." + domain,                        // *.example.local
		strings.TrimSuffix(domain, ".local"), // example (without .local)
		"localhost",
		"127.0.0.1",
		"::1",
	}
	mkcertArgs = append(mkcertArgs, localIPs...)

	// Use mkcert to generate the certificate
	cmd := exec.Command("mkcert", mkcertArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to generate certificate: %w\nOutput: %s", err, output)
	}

	// Load the generated certificate
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	log.Printf("Successfully generated certificate for domain: %s", domain)
	return &cert, nil
}
