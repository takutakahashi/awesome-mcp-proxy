package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")

	// テスト用設定ファイルを作成
	configContent := `
gateway:
  host: "127.0.0.1"
  port: 9090
  endpoint: "/test-mcp"
  timeout: 45s

groups:
  - name: "test-group"
    backends:
      test-backend:
        name: "test-backend"
        transport: "http"
        endpoint: "http://localhost:3000/mcp"
        headers:
          Authorization: "Bearer test-token"

middleware:
  logging:
    enabled: false
    level: "debug"
  cors:
    enabled: false
  caching:
    enabled: false
    ttl: 600s
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// 設定ファイルを読み込み
	config, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Gateway設定の検証
	if config.Gateway.Host != "127.0.0.1" {
		t.Errorf("Expected host '127.0.0.1', got '%s'", config.Gateway.Host)
	}
	if config.Gateway.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", config.Gateway.Port)
	}
	if config.Gateway.Endpoint != "/test-mcp" {
		t.Errorf("Expected endpoint '/test-mcp', got '%s'", config.Gateway.Endpoint)
	}
	if config.Gateway.Timeout != 45*time.Second {
		t.Errorf("Expected timeout 45s, got %v", config.Gateway.Timeout)
	}

	// Groups設定の検証
	if len(config.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(config.Groups))
	}
	if config.Groups[0].Name != "test-group" {
		t.Errorf("Expected group name 'test-group', got '%s'", config.Groups[0].Name)
	}

	// Backend設定の検証
	if len(config.Groups[0].Backends) != 1 {
		t.Errorf("Expected 1 backend, got %d", len(config.Groups[0].Backends))
	}
	backend := config.Groups[0].Backends["test-backend"]
	if backend.Name != "test-backend" {
		t.Errorf("Expected backend name 'test-backend', got '%s'", backend.Name)
	}
	if backend.Transport != "http" {
		t.Errorf("Expected transport 'http', got '%s'", backend.Transport)
	}
	if backend.Endpoint != "http://localhost:3000/mcp" {
		t.Errorf("Expected endpoint 'http://localhost:3000/mcp', got '%s'", backend.Endpoint)
	}

	// Middleware設定の検証
	if config.Middleware.Logging.Enabled != false {
		t.Errorf("Expected logging enabled false, got %v", config.Middleware.Logging.Enabled)
	}
	if config.Middleware.Logging.Level != "debug" {
		t.Errorf("Expected logging level 'debug', got '%s'", config.Middleware.Logging.Level)
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	// 最小限の設定ファイルでデフォルト値をテスト
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "minimal-config.yaml")

	configContent := `
groups:
  - name: "minimal-group"
    backends:
      minimal-backend:
        name: "minimal-backend"
        transport: "stdio"
        command: "test-command"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	config, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// デフォルト値の検証
	if config.Gateway.Host != "0.0.0.0" {
		t.Errorf("Expected default host '0.0.0.0', got '%s'", config.Gateway.Host)
	}
	if config.Gateway.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Gateway.Port)
	}
	if config.Gateway.Endpoint != "/mcp" {
		t.Errorf("Expected default endpoint '/mcp', got '%s'", config.Gateway.Endpoint)
	}
	if config.Gateway.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.Gateway.Timeout)
	}

	if config.Middleware.Logging.Enabled != true {
		t.Errorf("Expected default logging enabled true, got %v", config.Middleware.Logging.Enabled)
	}
	if config.Middleware.CORS.Enabled != true {
		t.Errorf("Expected default CORS enabled true, got %v", config.Middleware.CORS.Enabled)
	}
	if config.Middleware.Caching.Enabled != true {
		t.Errorf("Expected default caching enabled true, got %v", config.Middleware.Caching.Enabled)
	}
}

func TestEnvVarExpansion(t *testing.T) {
	// 環境変数のテスト
	_ = os.Setenv("TEST_TOKEN", "secret-token-123")
	_ = os.Setenv("TEST_PORT", "8888")
	defer func() {
		_ = os.Unsetenv("TEST_TOKEN")
		_ = os.Unsetenv("TEST_PORT")
	}()

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "env-config.yaml")

	configContent := `
gateway:
  port: 8888

groups:
  - name: "env-group"
    backends:
      env-backend:
        name: "env-backend"
        transport: "http"
        endpoint: "http://localhost:3000/mcp"
        headers:
          Authorization: "Bearer ${TEST_TOKEN}"
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	config, err := LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 環境変数の展開を検証
	if config.Gateway.Port != 8888 {
		t.Errorf("Expected port 8888 from env var, got %d", config.Gateway.Port)
	}

	backend := config.Groups[0].Backends["env-backend"]
	expectedAuth := "Bearer secret-token-123"
	// mapstructureはYAMLキーを小文字にする
	actualAuth := backend.Headers["authorization"]

	if actualAuth != expectedAuth {
		t.Errorf("Expected Authorization '%s', got '%s'", expectedAuth, actualAuth)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
	}{
		{
			name: "valid config",
			config: `
groups:
  - name: "valid-group"
    backends:
      valid-backend:
        name: "valid-backend"
        transport: "stdio"
        command: "test-command"
`,
			expectError: false,
		},
		{
			name: "invalid port",
			config: `
gateway:
  port: 99999
groups:
  - name: "test-group"
    backends:
      test-backend:
        name: "test-backend"
        transport: "stdio"
        command: "test-command"
`,
			expectError: true,
		},
		{
			name: "empty group name",
			config: `
groups:
  - name: ""
    backends:
      test-backend:
        name: "test-backend"
        transport: "stdio"
        command: "test-command"
`,
			expectError: true,
		},
		{
			name: "missing command for stdio",
			config: `
groups:
  - name: "test-group"
    backends:
      test-backend:
        name: "test-backend"
        transport: "stdio"
`,
			expectError: true,
		},
		{
			name: "missing endpoint for http",
			config: `
groups:
  - name: "test-group"
    backends:
      test-backend:
        name: "test-backend"
        transport: "http"
`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configFile := filepath.Join(tempDir, "test-config.yaml")

			if err := os.WriteFile(configFile, []byte(tt.config), 0644); err != nil {
				t.Fatalf("Failed to write test config file: %v", err)
			}

			_, err := LoadConfig(configFile)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestGetConfigPath(t *testing.T) {
	// テスト用の一時ディレクトリを作成
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")

	// テスト用設定ファイルを作成
	if err := os.WriteFile(configFile, []byte("test: true"), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// 指定されたパスが正しく返されるかテスト
	path, err := GetConfigPath(configFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if path != configFile {
		t.Errorf("Expected path '%s', got '%s'", configFile, path)
	}

	// 存在しないファイルのテスト
	_, err = GetConfigPath("/nonexistent/path/config.yaml")
	if err == nil {
		t.Errorf("Expected error for nonexistent file")
	}
}
