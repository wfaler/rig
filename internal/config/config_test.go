package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func boolPtr(b bool) *bool { return &b }

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
    build_systems:
      npm: "true"
  python:
    version: "3.12"
    build_systems:
      poetry: "1.7.0"
ports:
  - "8080:8080"
  - "3000"
env:
  API_KEY: "test-key"
`,
			want: &Config{
				Languages: map[string]LanguageConfig{
					"node": {
						Version:      "lts",
						BuildSystems: map[string]string{"npm": "true"},
					},
					"python": {
						Version:      "3.12",
						BuildSystems: map[string]string{"poetry": "1.7.0"},
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
					"node": {Version: "lts", BuildSystems: map[string]string{"npm": "true"}},
				},
			},
			wantErr: false,
		},
		{
			name: "valid multi-language config",
			config: Config{
				Languages: map[string]LanguageConfig{
					"node":   {Version: "20", BuildSystems: map[string]string{"yarn": "true"}},
					"python": {Version: "3.12", BuildSystems: map[string]string{"poetry": "1.8.0"}},
					"java":   {Version: "21", BuildSystems: map[string]string{"gradle": "8.5"}},
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
					"node": {BuildSystems: map[string]string{"gradle": "true"}},
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
		name   string
		config LanguageConfig
		want   string
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

func TestGetBuildSystems(t *testing.T) {
	tests := []struct {
		name   string
		config LanguageConfig
		want   map[string]string
	}{
		{
			name:   "empty config returns nil",
			config: LanguageConfig{},
			want:   nil,
		},
		{
			name: "build_systems with versions",
			config: LanguageConfig{
				BuildSystems: map[string]string{
					"gradle": "8.5",
					"maven":  "3.9",
				},
			},
			want: map[string]string{"gradle": "8.5", "maven": "3.9"},
		},
		{
			name: "build_systems with true normalizes to empty",
			config: LanguageConfig{
				BuildSystems: map[string]string{
					"gradle": "true",
					"maven":  "latest",
					"sbt":    "1.9.8",
				},
			},
			want: map[string]string{"gradle": "", "maven": "", "sbt": "1.9.8"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetBuildSystems()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseWithBuildSystems(t *testing.T) {
	yaml := `
languages:
  java:
    version: "21"
    build_systems:
      gradle: "8.5"
      maven: "true"
      sbt: "1.9.8"
  python:
    version: "3.12"
    build_systems:
      poetry: "1.8.0"
      pip: "latest"
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)

	// Check Java build systems
	javaCfg := cfg.Languages["java"]
	assert.Equal(t, "21", javaCfg.Version)
	assert.Equal(t, map[string]string{
		"gradle": "8.5",
		"maven":  "true",
		"sbt":    "1.9.8",
	}, javaCfg.BuildSystems)

	// Test normalized GetBuildSystems
	javaBuildSystems := javaCfg.GetBuildSystems()
	assert.Equal(t, "8.5", javaBuildSystems["gradle"])
	assert.Equal(t, "", javaBuildSystems["maven"])
	assert.Equal(t, "1.9.8", javaBuildSystems["sbt"])

	// Check Python build systems
	pythonCfg := cfg.Languages["python"]
	pythonBuildSystems := pythonCfg.GetBuildSystems()
	assert.Equal(t, "1.8.0", pythonBuildSystems["poetry"])
	assert.Equal(t, "", pythonBuildSystems["pip"])
}

func TestMarkdownServerDefaults(t *testing.T) {
	tests := []struct {
		name        string
		mdServer    *MarkdownServerConfig
		wantEnabled bool
		wantPort    int
	}{
		{"absent defaults to disabled", nil, false, 3030},
		{"port without enabled stays disabled", &MarkdownServerConfig{Port: 4040}, false, 4040},
		{"explicitly disabled", &MarkdownServerConfig{Enabled: false}, false, 3030},
		{"explicitly enabled uses default port", &MarkdownServerConfig{Enabled: true}, true, 3030},
		{"explicitly enabled with custom port", &MarkdownServerConfig{Enabled: true, Port: 4040}, true, 4040},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{MarkdownServer: tt.mdServer}
			assert.Equal(t, tt.wantEnabled, cfg.IsMarkdownServerEnabled())
			assert.Equal(t, tt.wantPort, cfg.GetMarkdownServerPort())
		})
	}
}

func TestParseWithMarkdownServer(t *testing.T) {
	yaml := `
markdown_server:
  enabled: true
  port: 4000
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	assert.True(t, cfg.IsMarkdownServerEnabled())
	assert.Equal(t, 4000, cfg.GetMarkdownServerPort())
}

func TestParseWithMarkdownServerDisabled(t *testing.T) {
	yaml := `
markdown_server:
  enabled: false
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	assert.False(t, cfg.IsMarkdownServerEnabled())
}

func TestGetAllPortsIncludesMarkdownServer(t *testing.T) {
	// Default config (markdown server disabled)
	cfg := &Config{
		Ports: []string{"8080"},
	}
	ports := cfg.GetAllPorts()
	assert.NotContains(t, ports, "3030")
	assert.Contains(t, ports, "8080")

	// Markdown server enabled
	cfg2 := &Config{
		Ports:          []string{"8080"},
		MarkdownServer: &MarkdownServerConfig{Enabled: true},
	}
	ports2 := cfg2.GetAllPorts()
	assert.Contains(t, ports2, "3030")
	assert.Contains(t, ports2, "8080")
}

func TestValidateMultipleBuildSystems(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid multiple build systems",
			config: Config{
				Languages: map[string]LanguageConfig{
					"java": {
						Version: "21",
						BuildSystems: map[string]string{
							"gradle": "8.5",
							"maven":  "true",
							"sbt":    "1.9.8",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid build system in multiple",
			config: Config{
				Languages: map[string]LanguageConfig{
					"java": {
						Version: "21",
						BuildSystems: map[string]string{
							"gradle": "8.5",
							"npm":    "true", // npm is not valid for Java
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid build system",
		},
		{
			name: "valid python multiple build systems",
			config: Config{
				Languages: map[string]LanguageConfig{
					"python": {
						Version: "3.12",
						BuildSystems: map[string]string{
							"poetry": "1.8.0",
							"pip":    "true",
						},
					},
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

func TestHerdrDefaults(t *testing.T) {
	// Default: enabled, agent "claude"
	cfg := &Config{}
	assert.True(t, cfg.IsHerdrEnabled())
	assert.Equal(t, "claude", cfg.GetHerdrAgent())

	// Explicitly disabled
	disabled := false
	cfg2 := &Config{Herdr: &HerdrConfig{Enabled: &disabled}}
	assert.False(t, cfg2.IsHerdrEnabled())

	// Explicitly enabled with custom agent
	enabled := true
	cfg3 := &Config{Herdr: &HerdrConfig{Enabled: &enabled, Agent: "gemini"}}
	assert.True(t, cfg3.IsHerdrEnabled())
	assert.Equal(t, "gemini", cfg3.GetHerdrAgent())

	// Present but enabled unset -> still enabled by default
	cfg4 := &Config{Herdr: &HerdrConfig{Agent: "codex"}}
	assert.True(t, cfg4.IsHerdrEnabled())
	assert.Equal(t, "codex", cfg4.GetHerdrAgent())
}

func TestParseWithHerdr(t *testing.T) {
	yaml := `
herdr:
  enabled: false
  agent: gemini
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	assert.False(t, cfg.IsHerdrEnabled())
	assert.Equal(t, "gemini", cfg.GetHerdrAgent())
}

func TestIsClaudeStatePersisted(t *testing.T) {
	enabled := true
	disabled := false
	tests := []struct {
		name     string
		claude   *ClaudeConfig
		expected bool
	}{
		{"absent defaults to enabled", nil, true},
		{"present but unset defaults to enabled", &ClaudeConfig{}, true},
		{"explicitly enabled", &ClaudeConfig{PersistState: &enabled}, true},
		{"explicitly disabled", &ClaudeConfig{PersistState: &disabled}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Claude: tt.claude}
			assert.Equal(t, tt.expected, cfg.IsClaudeStatePersisted())
		})
	}
}

func TestParseWithClaude(t *testing.T) {
	yaml := `
claude:
  persist_state: false
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	assert.False(t, cfg.IsClaudeStatePersisted())
}

func TestHerdrAgentValidation(t *testing.T) {
	tests := []struct {
		name    string
		agent   string
		wantErr bool
	}{
		{"claude ok", "claude", false},
		{"gemini ok", "gemini", false},
		{"codex ok", "codex", false},
		{"empty ok (defaults)", "", false},
		{"unknown rejected", "bogus", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Herdr: &HerdrConfig{Agent: tt.agent}}
			err := cfg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unsupported herdr agent")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNeovimDefaults(t *testing.T) {
	repo := &NeovimConfigRepo{Repo: "https://github.com/example/dotfiles.git"}
	tests := []struct {
		name        string
		neovim      *NeovimConfig
		wantEnabled bool
		wantRepo    *NeovimConfigRepo
	}{
		{"absent: enabled with Neovim's defaults", nil, true, nil},
		{"enabled unset: enabled", &NeovimConfig{Config: repo}, true, repo},
		{"explicitly enabled without config", &NeovimConfig{Enabled: boolPtr(true)}, true, nil},
		{"disabled ignores the config", &NeovimConfig{Enabled: boolPtr(false), Config: repo}, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Neovim: tt.neovim}
			assert.Equal(t, tt.wantEnabled, cfg.IsNeovimEnabled())
			assert.Equal(t, tt.wantRepo, cfg.GetNeovimConfigRepo())
		})
	}
}

func TestParseWithNeovim(t *testing.T) {
	yaml := `
neovim:
  config:
    repo: https://github.com/wfaler/arch-dotfiles.git
    path: .config/nvim
    ref: main
`
	cfg, err := Parse([]byte(yaml))
	require.NoError(t, err)
	require.NoError(t, cfg.Validate())
	assert.True(t, cfg.IsNeovimEnabled())
	assert.Equal(t, &NeovimConfigRepo{
		Repo: "https://github.com/wfaler/arch-dotfiles.git",
		Path: ".config/nvim",
		Ref:  "main",
	}, cfg.GetNeovimConfigRepo())
}

func TestNeovimConfigRepoValidation(t *testing.T) {
	tests := []struct {
		name    string
		repo    NeovimConfigRepo
		wantErr string
	}{
		{"repo only", NeovimConfigRepo{Repo: "https://github.com/a/b"}, ""},
		{"repo, path and ref", NeovimConfigRepo{Repo: "https://github.com/a/b.git", Path: ".config/nvim", Ref: "v1.2"}, ""},
		{"nested ref", NeovimConfigRepo{Repo: "https://github.com/a/b", Ref: "feature/nvim"}, ""},
		{"empty repo", NeovimConfigRepo{}, "neovim.config.repo"},
		{"ssh repo", NeovimConfigRepo{Repo: "git@github.com:a/b.git"}, "neovim.config.repo"},
		{"http repo", NeovimConfigRepo{Repo: "http://github.com/a/b"}, "neovim.config.repo"},
		{"file repo", NeovimConfigRepo{Repo: "file:///etc"}, "neovim.config.repo"},
		{"repo with quote", NeovimConfigRepo{Repo: "https://github.com/a/b'; rm -rf /"}, "neovim.config.repo"},
		{"absolute path", NeovimConfigRepo{Repo: "https://github.com/a/b", Path: "/etc"}, "neovim.config.path"},
		{"path escaping the repo", NeovimConfigRepo{Repo: "https://github.com/a/b", Path: "nvim/../../x"}, "neovim.config.path"},
		{"path as an option", NeovimConfigRepo{Repo: "https://github.com/a/b", Path: "--no-cone"}, "neovim.config.path"},
		{"path with shell syntax", NeovimConfigRepo{Repo: "https://github.com/a/b", Path: "nvim;id"}, "neovim.config.path"},
		{"ref as an option", NeovimConfigRepo{Repo: "https://github.com/a/b", Ref: "-u"}, "neovim.config.ref"},
		{"ref with dots", NeovimConfigRepo{Repo: "https://github.com/a/b", Ref: "a..b"}, "neovim.config.ref"},
		{"ref with shell syntax", NeovimConfigRepo{Repo: "https://github.com/a/b", Ref: "$(id)"}, "neovim.config.ref"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Neovim: &NeovimConfig{Config: &tt.repo}}
			err := cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestNeovimConfigRepoValidationProperties checks that any path or ref that
// passes validation is a plain word: no shell syntax, no option, no escape.
func TestNeovimConfigRepoValidationProperties(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		path := rapid.String().Draw(rt, "path")
		ref := rapid.String().Draw(rt, "ref")
		cfg := &Config{Neovim: &NeovimConfig{Config: &NeovimConfigRepo{
			Repo: "https://github.com/a/b",
			Path: path,
			Ref:  ref,
		}}}
		if cfg.Validate() != nil {
			return
		}
		assert.NotContains(rt, ref, "..")
		assert.NotContains(rt, strings.Split(path, "/"), "..", "%q escapes the repository", path)
		for _, s := range []string{path, ref} {
			assert.False(rt, strings.HasPrefix(s, "-"), "%q starts with -", s)
			assert.False(rt, strings.ContainsAny(s, " \t\n'\"`$;&|<>(){}\\*?![]~#"), "%q has shell syntax", s)
		}
		assert.False(rt, strings.HasPrefix(path, "/"), "%q is absolute", path)
	})
}
