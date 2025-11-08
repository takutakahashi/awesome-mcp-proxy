package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Gateway    GatewayConfig    `yaml:"gateway" mapstructure:"gateway"`
	Groups     []Group          `yaml:"groups" mapstructure:"groups"`
	Middleware MiddlewareConfig `yaml:"middleware" mapstructure:"middleware"`
}

type GatewayConfig struct {
	Host     string        `yaml:"host" mapstructure:"host"`
	Port     int           `yaml:"port" mapstructure:"port"`
	Endpoint string        `yaml:"endpoint" mapstructure:"endpoint"`
	Timeout  time.Duration `yaml:"timeout" mapstructure:"timeout"`
}

type Group struct {
	Name     string             `yaml:"name" mapstructure:"name"`
	Backends map[string]Backend `yaml:"backends" mapstructure:"backends"`
}

type Backend struct {
	Name      string            `yaml:"name" mapstructure:"name"`
	Transport string            `yaml:"transport" mapstructure:"transport"`
	Command   string            `yaml:"command,omitempty" mapstructure:"command"`
	Args      []string          `yaml:"args,omitempty" mapstructure:"args"`
	Endpoint  string            `yaml:"endpoint,omitempty" mapstructure:"endpoint"`
	Headers   map[string]string `yaml:"headers,omitempty" mapstructure:"headers"`
	Env       map[string]string `yaml:"env,omitempty" mapstructure:"env"`
}

type MiddlewareConfig struct {
	Logging LoggingConfig `yaml:"logging" mapstructure:"logging"`
	CORS    CORSConfig    `yaml:"cors" mapstructure:"cors"`
	Caching CachingConfig `yaml:"caching" mapstructure:"caching"`
}

type LoggingConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Level   string `yaml:"level" mapstructure:"level"`
}

type CORSConfig struct {
	Enabled        bool     `yaml:"enabled" mapstructure:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins" mapstructure:"allowed_origins"`
}

type CachingConfig struct {
	Enabled bool          `yaml:"enabled" mapstructure:"enabled"`
	TTL     time.Duration `yaml:"ttl" mapstructure:"ttl"`
}

func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// 設定ファイルの場所を設定
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// デフォルトの設定ファイル検索パス
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("$HOME/.mcp-proxy")
		v.AddConfigPath("/etc/mcp-proxy")
	}

	// 環境変数の読み込み設定
	v.AutomaticEnv()
	v.SetEnvPrefix("MCP_PROXY")

	// デフォルト値の設定
	setDefaults(v)

	// 設定ファイルの読み込み
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 設定ファイルが見つからない場合はデフォルト値を使用
			fmt.Printf("Warning: Config file not found, using defaults\n")
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 環境変数の展開 (Unmarshal後に実行)
	expandConfigEnvVars(&config)

	// 設定の検証
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

func setDefaults(v *viper.Viper) {
	// Gateway defaults
	v.SetDefault("gateway.host", "0.0.0.0")
	v.SetDefault("gateway.port", 8080)
	v.SetDefault("gateway.endpoint", "/mcp")
	v.SetDefault("gateway.timeout", "30s")

	// Middleware defaults
	v.SetDefault("middleware.logging.enabled", true)
	v.SetDefault("middleware.logging.level", "info")
	v.SetDefault("middleware.cors.enabled", true)
	v.SetDefault("middleware.cors.allowed_origins", []string{"*"})
	v.SetDefault("middleware.caching.enabled", true)
	v.SetDefault("middleware.caching.ttl", "300s")
}

func expandConfigEnvVars(config *Config) {
	// Groups内のBackendsの環境変数を展開
	for i := range config.Groups {
		for name, backend := range config.Groups[i].Backends {
			// Command, Endpoint, Args, Env, Headersの環境変数を展開
			backend.Command = os.ExpandEnv(backend.Command)
			backend.Endpoint = os.ExpandEnv(backend.Endpoint)

			// Argsの展開
			for j, arg := range backend.Args {
				backend.Args[j] = os.ExpandEnv(arg)
			}

			// Envの展開
			for key, value := range backend.Env {
				backend.Env[key] = os.ExpandEnv(value)
			}

			// Headersの展開
			for key, value := range backend.Headers {
				backend.Headers[key] = os.ExpandEnv(value)
			}

			// 更新されたbackendを戻す
			config.Groups[i].Backends[name] = backend
		}
	}
}

func validateConfig(config *Config) error {
	// Gateway設定の検証
	if config.Gateway.Port <= 0 || config.Gateway.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", config.Gateway.Port)
	}

	if config.Gateway.Endpoint == "" {
		return fmt.Errorf("gateway endpoint cannot be empty")
	}

	// Groups設定の検証
	if len(config.Groups) == 0 {
		return fmt.Errorf("at least one group must be defined")
	}

	groupNames := make(map[string]bool)
	for _, group := range config.Groups {
		if group.Name == "" {
			return fmt.Errorf("group name cannot be empty")
		}

		if groupNames[group.Name] {
			return fmt.Errorf("duplicate group name: %s", group.Name)
		}
		groupNames[group.Name] = true

		if len(group.Backends) == 0 {
			return fmt.Errorf("group %s must have at least one backend", group.Name)
		}

		// Backend設定の検証
		backendNames := make(map[string]bool)
		for _, backend := range group.Backends {
			if backend.Name == "" {
				return fmt.Errorf("backend name cannot be empty in group %s", group.Name)
			}

			if backendNames[backend.Name] {
				return fmt.Errorf("duplicate backend name %s in group %s", backend.Name, group.Name)
			}
			backendNames[backend.Name] = true

			if err := validateBackend(&backend, group.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateBackend(backend *Backend, groupName string) error {
	switch backend.Transport {
	case "stdio":
		if backend.Command == "" {
			return fmt.Errorf("command is required for stdio transport in backend %s (group %s)", backend.Name, groupName)
		}
	case "http":
		if backend.Endpoint == "" {
			return fmt.Errorf("endpoint is required for http transport in backend %s (group %s)", backend.Name, groupName)
		}
	default:
		return fmt.Errorf("unsupported transport type %s in backend %s (group %s)", backend.Transport, backend.Name, groupName)
	}

	return nil
}

// GetConfigPath returns the path to the config file being used
func GetConfigPath(configPath string) (string, error) {
	if configPath != "" {
		if _, err := os.Stat(configPath); err != nil {
			return "", fmt.Errorf("config file not found: %s", configPath)
		}
		return configPath, nil
	}

	// デフォルトパスを検索
	searchPaths := []string{
		"./config.yaml",
		"./config/config.yaml",
		filepath.Join(os.Getenv("HOME"), ".mcp-proxy", "config.yaml"),
		"/etc/mcp-proxy/config.yaml",
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no config file found in default locations")
}
