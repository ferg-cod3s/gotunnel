package state

import (
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type TunnelState struct {
	Port   int    `yaml:"port"`
	Domain string `yaml:"domain"`
	HTTPS  bool   `yaml:"https"`
}

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

	stateFile := getStateFile()
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
	stateFile := getStateFile()
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
