package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    *Config
		wantErr bool
	}{
		{
			name: "valid config with all fields",
			yaml: `
languages:
  node:
    version: "lts"
    build_system: npm
  python:
    version: "3.12"
    build_system: poetry
    build_system_version: "1.7.0"
ports:
  - "8080:8080"
  - "3000"
env:
  API_KEY: "test-key"
`,
			want: &Config{
				Languages: map[string]LanguageConfig{
					"node": {
						Version:     "lts",
						BuildSystem: "npm",
					},
					"python": {
						Version:            "3.12",
						BuildSystem:        "poetry",
						BuildSystemVersion: "1.7.0",
					},
				},
				Ports: []string{"8080:8080", "3000"},
				Env: map[string]string{
					"API_KEY": "test-key",
				},
			},
			wantErr: false,
		},
		{
			name: "empty config",
			yaml: ``,
			want: &Config{
				Languages: map[string]LanguageConfig{},
				Ports:     nil,
				Env:       map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "minimal language config",
			yaml: `
languages:
  go:
    version: "1.22"
`,
			want: &Config{
				Languages: map[string]LanguageConfig{
					"go": {Version: "1.22"},
				},
				Ports: nil,
				Env:   map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "config with code_server enabled",
			yaml: `
languages:
  node:
    version: "lts"
code_server:
  enabled: true
  port: 9000
  theme: "Monokai"
  extensions:
    - github.copilot
`,
			want: &Config{
				Languages: map[string]LanguageConfig{
					"node": {Version: "lts"},
				},
				Ports: nil,
				Env:   map[string]string{},
				CodeServer: &CodeServerConfig{
					Enabled:    true,
					Port:       9000,
					Theme:      "Monokai",
					Extensions: []string{"github.copilot"},
				},
			},
			wantErr: false,
		},
		{
			name: "config with code_server disabled",
			yaml: `
code_server:
  enabled: false
`,
			want: &Config{
				Languages: map[string]LanguageConfig{},
				Ports:     nil,
				Env:       map[string]string{},
				CodeServer: &CodeServerConfig{
					Enabled: false,
				},
			},
			wantErr: false,
		},
		{
			name:    "invalid yaml",
			yaml:    `languages: [invalid`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.yaml))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid node config",
			config: Config{
				Languages: map[string]LanguageConfig{
					"node": {Version: "lts", BuildSystem: "npm"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid multi-language config",
			config: Config{
				Languages: map[string]LanguageConfig{
					"node":   {Version: "20", BuildSystem: "yarn"},
					"python": {Version: "3.12", BuildSystem: "poetry"},
					"java":   {Version: "21", BuildSystem: "gradle"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid language",
			config: Config{
				Languages: map[string]LanguageConfig{
					"cobol": {},
				},
			},
			wantErr: true,
			errMsg:  "unsupported language: cobol",
		},
		{
			name: "invalid build system for language",
			config: Config{
				Languages: map[string]LanguageConfig{
					"node": {BuildSystem: "gradle"},
				},
			},
			wantErr: true,
			errMsg:  "invalid build system",
		},
		{
			name: "valid port - single",
			config: Config{
				Ports: []string{"8080"},
			},
			wantErr: false,
		},
		{
			name: "valid port - mapping",
			config: Config{
				Ports: []string{"8080:3000"},
			},
			wantErr: false,
		},
		{
			name: "invalid port - non-numeric",
			config: Config{
				Ports: []string{"abc"},
			},
			wantErr: true,
			errMsg:  "invalid port",
		},
		{
			name: "invalid port - bad format",
			config: Config{
				Ports: []string{"8080:3000:1234"},
			},
			wantErr: true,
			errMsg:  "invalid port",
		},
		{
			name: "language without build system is valid",
			config: Config{
				Languages: map[string]LanguageConfig{
					"go": {Version: "1.22"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestExpandEnvVars(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_API_KEY", "secret-value")
	defer os.Unsetenv("TEST_API_KEY")

	cfg := &Config{
		Env: map[string]string{
			"API_KEY":  "${TEST_API_KEY}",
			"STATIC":   "static-value",
			"COMBINED": "prefix-${TEST_API_KEY}-suffix",
		},
	}

	cfg.ExpandEnvVars()

	assert.Equal(t, "secret-value", cfg.Env["API_KEY"])
	assert.Equal(t, "static-value", cfg.Env["STATIC"])
	assert.Equal(t, "prefix-secret-value-suffix", cfg.Env["COMBINED"])
}

func TestLanguageConfigGetVersion(t *testing.T) {
	tests := []struct {
		name    string
		config  LanguageConfig
		want    string
	}{
		{
			name:   "explicit version",
			config: LanguageConfig{Version: "20.10.0"},
			want:   "20.10.0",
		},
		{
			name:   "lts version",
			config: LanguageConfig{Version: "lts"},
			want:   "lts",
		},
		{
			name:   "empty defaults to latest",
			config: LanguageConfig{},
			want:   "latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.config.GetVersion())
		})
	}
}

func TestHasLanguage(t *testing.T) {
	cfg := &Config{
		Languages: map[string]LanguageConfig{
			"node": {Version: "lts"},
		},
	}

	assert.True(t, cfg.HasLanguage("node"))
	assert.False(t, cfg.HasLanguage("python"))
}

func TestGetShell(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name:   "empty defaults to zsh",
			config: Config{},
			want:   "zsh",
		},
		{
			name:   "bash explicitly set",
			config: Config{Shell: "bash"},
			want:   "bash",
		},
		{
			name:   "zsh explicitly set",
			config: Config{Shell: "zsh"},
			want:   "zsh",
		},
		{
			name:   "fish",
			config: Config{Shell: "fish"},
			want:   "fish",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.config.GetShell())
		})
	}
}

func TestShellValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid bash",
			config:  Config{Shell: "bash"},
			wantErr: false,
		},
		{
			name:    "valid zsh",
			config:  Config{Shell: "zsh"},
			wantErr: false,
		},
		{
			name:    "valid fish",
			config:  Config{Shell: "fish"},
			wantErr: false,
		},
		{
			name:    "empty shell is valid (defaults to bash)",
			config:  Config{},
			wantErr: false,
		},
		{
			name:    "invalid shell",
			config:  Config{Shell: "csh"},
			wantErr: true,
			errMsg:  "unsupported shell",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParseWithShell(t *testing.T) {
	yaml := `
shell: zsh
languages:
  node:
    version: "lts"
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	assert.Equal(t, "zsh", cfg.Shell)
	assert.Equal(t, "zsh", cfg.GetShell())
}
