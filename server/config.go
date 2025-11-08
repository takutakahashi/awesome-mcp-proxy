package server

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadGatewayConfig loads gateway configuration from a YAML file
func LoadGatewayConfig(filename string) (*GatewayConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// Expand environment variables
	content := os.ExpandEnv(string(data))

	var config GatewayConfig
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, err
	}

	// Set defaults
	if config.Gateway.Host == "" {
		config.Gateway.Host = "0.0.0.0"
	}
	if config.Gateway.Port == 0 {
		config.Gateway.Port = 8080
	}
	if config.Gateway.Endpoint == "" {
		config.Gateway.Endpoint = "/mcp"
	}
	if config.Gateway.Timeout == "" {
		config.Gateway.Timeout = "30s"
	}

	// Expand environment variables in backend configurations
	for i := range config.Groups {
		for j := range config.Groups[i].Backends {
			backend := &config.Groups[i].Backends[j]
			
			// Expand environment variables in headers
			for key, value := range backend.Headers {
				backend.Headers[key] = expandEnvVars(value)
			}
			
			// Expand environment variables in env
			for key, value := range backend.Env {
				backend.Env[key] = expandEnvVars(value)
			}
			
			// Expand endpoint
			backend.Endpoint = expandEnvVars(backend.Endpoint)
			backend.Command = expandEnvVars(backend.Command)
		}
	}

	return &config, nil
}

// expandEnvVars expands environment variables in the format ${VAR_NAME}
func expandEnvVars(s string) string {
	return os.ExpandEnv(s)
}

// DefaultGatewayConfig returns a default gateway configuration
func DefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		Gateway: GatewaySettings{
			Host:     "0.0.0.0",
			Port:     8080,
			Endpoint: "/mcp",
			Timeout:  "30s",
		},
		Groups: []Group{
			{
				Name: "default",
				Backends: []Backend{
					{
						Name:      "local-server",
						Transport: "http",
						Endpoint:  "http://localhost:3000/mcp",
					},
				},
			},
		},
		Middleware: MiddlewareConfig{
			Logging: LoggingConfig{
				Enabled: true,
				Level:   "info",
			},
			CORS: CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*"},
			},
			Caching: CachingConfig{
				Enabled: true,
				TTL:     "300s",
			},
		},
	}
}