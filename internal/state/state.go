package state

import (
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type TunnelState struct {
	Domain    string `yaml:"domain"`
	Port      int    `yaml:"port"`      // backend target port (where user's app runs)
	HTTPPort  int    `yaml:"http_port"` // tunnel HTTP listen port
	HTTPSPort int    `yaml:"https_port"`
	HTTPS     bool   `yaml:"https"`
	PID       int    `yaml:"pid"`
	ProxyMode string `yaml:"proxy_mode"`
}

// For testing purposes
var getStateFileFunc = getStateFile

func getStateFile() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".gotunnel", "tunnels.yaml")
}

func SaveTunnels(tunnels []TunnelState) error {
	log.Println("Saving tunnel states...")
	data, err := yaml.Marshal(tunnels)
	if err != nil {
		return err
	}

	stateFile := getStateFileFunc() // Use the function variable for testing
	if err := os.MkdirAll(filepath.Dir(stateFile), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		log.Printf("Failed to write tunnel states to file: %v", err)
		return err
	}
	log.Println("Tunnel states saved successfully.")
	return nil
}

func LoadTunnels() ([]TunnelState, error) {
	log.Println("Loading tunnel states...")
	stateFile := getStateFileFunc() // Use the function variable for testing
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("No tunnel states found.")
			return nil, nil
		}
		log.Printf("Error reading tunnel states file: %v", err)
		return nil, err
	}

	var tunnels []TunnelState
	if err := yaml.Unmarshal(data, &tunnels); err != nil {
		log.Printf("Failed to unmarshal tunnel states: %v", err)
		return nil, err
	}

	log.Println("Tunnel states loaded successfully.")
	return tunnels, nil
}

// AddTunnel persists or replaces a tunnel entry by domain.
func AddTunnel(t TunnelState) error {
	tunnels, _ := LoadTunnels()
	found := false
	for i, existing := range tunnels {
		if existing.Domain == t.Domain {
			tunnels[i] = t
			found = true
			break
		}
	}
	if !found {
		tunnels = append(tunnels, t)
	}
	return SaveTunnels(tunnels)
}

// RemoveTunnel deletes a tunnel entry by domain.
func RemoveTunnel(domain string) error {
	tunnels, _ := LoadTunnels()
	kept := tunnels[:0]
	removed := false
	for _, t := range tunnels {
		if t.Domain == domain {
			removed = true
			continue
		}
		kept = append(kept, t)
	}
	if !removed {
		return nil
	}
	return SaveTunnels(kept)
}

// ClearTunnels removes all persisted tunnel entries.
func ClearTunnels() error {
	return SaveTunnels(nil)
}
