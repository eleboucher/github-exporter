package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Success(t *testing.T) {
	content := `
requests:
  - api_path: "/users/{{ .GITHUB_USER }}"
    metrics:
      - name: github_followers
        path: "followers"
        help: "Total followers"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath, "testuser")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.GithubAPIURL != DefaultGitHubAPIURL {
		t.Errorf("Expected default GitHub API URL, got %s", cfg.GithubAPIURL)
	}

	if len(cfg.Requests) != 1 {
		t.Fatalf("Expected 1 request, got %d", len(cfg.Requests))
	}

	if cfg.Requests[0].ApiPath != "/users/testuser" {
		t.Errorf("Expected '/users/testuser', got '%s'", cfg.Requests[0].ApiPath)
	}

	if len(cfg.Requests[0].Metrics) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(cfg.Requests[0].Metrics))
	}

	if cfg.Requests[0].Metrics[0].Name != "github_followers" {
		t.Errorf("Expected 'github_followers', got '%s'", cfg.Requests[0].Metrics[0].Name)
	}
}

func TestLoad_WithEnvToken(t *testing.T) {
	content := `
requests:
  - api_path: "/users/test"
    metrics:
      - name: github_followers
        path: "followers"
        help: "Total followers"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	if err := os.Setenv("GITHUB_TOKEN", "test-token-123"); err != nil {
		t.Fatalf("Failed to set GITHUB_TOKEN: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("GITHUB_TOKEN"); err != nil {
			t.Errorf("Failed to unset GITHUB_TOKEN: %v", err)
		}
	}()

	cfg, err := Load(configPath, "")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Token != "test-token-123" {
		t.Errorf("Expected token 'test-token-123', got '%s'", cfg.Token)
	}
}

func TestLoad_WithCustomAPIURL(t *testing.T) {
	content := `
github_api_url: "https://github.example.com/api/v3"
requests:
  - api_path: "/users/test"
    metrics:
      - name: github_followers
        path: "followers"
        help: "Total followers"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath, "")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.GithubAPIURL != "https://github.example.com/api/v3" {
		t.Errorf("Expected custom API URL, got %s", cfg.GithubAPIURL)
	}
}

func TestLoad_TrailingSlashRemoval(t *testing.T) {
	content := `
github_api_url: "https://api.github.com/"
requests:
  - api_path: "/users/test"
    metrics:
      - name: github_followers
        path: "followers"
        help: "Total followers"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath, "")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.GithubAPIURL != "https://api.github.com" {
		t.Errorf("Expected trailing slash to be removed, got %s", cfg.GithubAPIURL)
	}
}

func TestLoad_MetricWithLabels(t *testing.T) {
	content := `
requests:
  - api_path: "/users/test/events"
    metrics:
      - name: github_last_push_info
        path: '#(type=="PushEvent").created_at'
        help: "Last push event"
        value_type: "date"
        labels:
          repo: '#(type=="PushEvent").repo.name'
          type: '#(type=="PushEvent").type'
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath, "")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(cfg.Requests) != 1 {
		t.Fatalf("Expected 1 request, got %d", len(cfg.Requests))
	}

	metric := cfg.Requests[0].Metrics[0]
	if metric.ValueType != TypeDate {
		t.Errorf("Expected value_type 'date', got '%s'", metric.ValueType)
	}

	if len(metric.Labels) != 2 {
		t.Fatalf("Expected 2 labels, got %d", len(metric.Labels))
	}

	if metric.Labels["repo"] != `#(type=="PushEvent").repo.name` {
		t.Errorf("Unexpected repo label path: %s", metric.Labels["repo"])
	}

	if metric.Labels["type"] != `#(type=="PushEvent").type` {
		t.Errorf("Unexpected type label path: %s", metric.Labels["type"])
	}
}

func TestLoad_MetricWithAggregate(t *testing.T) {
	content := `
requests:
  - api_path: "/users/test/repos"
    metrics:
      - name: github_stars_total
        path: "#.stargazers_count"
        help: "Sum of all stars"
        aggregate: "sum"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath, "")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	metric := cfg.Requests[0].Metrics[0]
	if metric.Aggregate != AggregateSum {
		t.Errorf("Expected aggregate 'sum', got '%s'", metric.Aggregate)
	}
}

func TestLoad_POSTRequest(t *testing.T) {
	content := `
requests:
  - api_path: "/graphql"
    method: "POST"
    body: |
      { "query": "{ user { name } }" }
    metrics:
      - name: github_user_name
        path: "data.user.name"
        help: "User name"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath, "")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	req := cfg.Requests[0]
	if req.Method != "POST" {
		t.Errorf("Expected method 'POST', got '%s'", req.Method)
	}

	if req.Body == "" {
		t.Error("Expected body to be set")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	content := `invalid: yaml: content:`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := Load(configPath, "")
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml", "")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestGetEnvMap(t *testing.T) {
	if err := os.Setenv("TEST_VAR", "test_value"); err != nil {
		t.Fatalf("Failed to set TEST_VAR: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_VAR"); err != nil {
			t.Errorf("Failed to unset TEST_VAR: %v", err)
		}
	}()

	envMap := getEnvMap("testuser")

	if envMap["GITHUB_USER"] != "testuser" {
		t.Errorf("Expected GITHUB_USER to be 'testuser', got '%s'", envMap["GITHUB_USER"])
	}

	if envMap["TEST_VAR"] != "test_value" {
		t.Errorf("Expected TEST_VAR to be 'test_value', got '%s'", envMap["TEST_VAR"])
	}
}

func TestGetEnvMap_NoGitHubUser(t *testing.T) {
	envMap := getEnvMap("")

	if _, exists := envMap["GITHUB_USER"]; exists {
		t.Error("Expected GITHUB_USER to not be set when empty string provided")
	}
}
