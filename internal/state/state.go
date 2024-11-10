package state

import (
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
	data, err := yaml.Marshal(tunnels)
	if err != nil {
		return err
	}

	stateFile := getStateFile()
	if err := os.MkdirAll(filepath.Dir(stateFile), 0755); err != nil {
		return err
	}

	return os.WriteFile(stateFile, data, 0644)
}

func LoadTunnels() ([]TunnelState, error) {
	stateFile := getStateFile()
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var tunnels []TunnelState
	if err := yaml.Unmarshal(data, &tunnels); err != nil {
		return nil, err
	}

	return tunnels, nil
}
