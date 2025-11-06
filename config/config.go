package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Gateways []Gateway `yaml:"gateways"`
}

// Gateway represents a remote MCP server configuration
type Gateway struct {
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	Prefix string `yaml:"prefix"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	for i, gw := range config.Gateways {
		if gw.Name == "" {
			return nil, fmt.Errorf("gateway[%d]: name is required", i)
		}
		if gw.URL == "" {
			return nil, fmt.Errorf("gateway[%d]: url is required", i)
		}
		if gw.Prefix == "" {
			return nil, fmt.Errorf("gateway[%d]: prefix is required", i)
		}
	}

	return &config, nil
}
