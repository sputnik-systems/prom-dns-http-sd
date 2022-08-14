package storage

import (
	"os"

	"github.com/kubernetes-sigs/yaml"
)

type Config struct {
	Provider ProviderConfig `json:"provider"`
	Zones    []string       `json:"zones"`
	Rules    []RuleConfig   `json:"rules"`
}

type ProviderConfig struct {
	Type     string                 `json:"type"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type RuleConfig struct {
	Path    string            `json:"path"`
	Port    int64             `json:"port"`
	Filters []string          `json:"filters"`
	Labels  map[string]string `json:"labels,omitempty"`
}

func GetConfig(filepath string) (*Config, error) {
	b, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}

	return &c, nil
}
