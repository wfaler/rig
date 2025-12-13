package dockerfile

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wfaler/devbox/internal/config"
)

func TestGenerate(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "empty config",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{},
				Env:       map[string]string{},
			},
			wantContains: []string{
				"FROM debian:bookworm-slim",
				"docker-ce-cli",
				"mise use --global node@lts", // Node LTS installed for AI agents
				"npm install -g @anthropic-ai/claude-code",
				"curl https://mise.run", // Mise installed
			},
			wantNotContain: []string{
				"sdkman", // No SDKMAN if no Java
			},
		},
		{
			name: "with node configured",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{
					"node": {Version: "lts", BuildSystem: "npm"},
				},
				Env: map[string]string{},
			},
			wantContains: []string{
				"FROM debian:bookworm-slim",
				"mise use --global node@lts",
			},
			wantNotContain: []string{
				"Node.js LTS for AI agents", // Should NOT have fallback Node install
			},
		},
		{
			name: "with multiple languages",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{
					"node":   {Version: "20", BuildSystem: "yarn"},
					"python": {Version: "3.12", BuildSystem: "poetry"},
					"go":     {Version: "1.22"},
				},
				Env: map[string]string{},
			},
			wantContains: []string{
				"mise use --global node@20",
				"mise use --global python@3.12",
				"mise use --global go@1.22",
				"npm install -g yarn",
				"pip install poetry",
			},
		},
		{
			name: "with environment variables",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{},
				Env: map[string]string{
					"API_KEY":      "secret123",
					"DATABASE_URL": "postgres://localhost",
				},
			},
			wantContains: []string{
				`ENV API_KEY="secret123"`,
				`ENV DATABASE_URL="postgres://localhost"`,
			},
		},
		{
			name: "with java and gradle",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{
					"java": {Version: "21", BuildSystem: "gradle", BuildSystemVersion: "8.5"},
				},
				Env: map[string]string{},
			},
			wantContains: []string{
				"get.sdkman.io",           // SDKMAN installed
				"sdk install java 21-tem", // Java via SDKMAN
				"sdk install gradle 8.5",  // Gradle via SDKMAN
			},
		},
		{
			name: "with rust",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{
					"rust": {Version: "1.75.0"},
				},
				Env: map[string]string{},
			},
			wantContains: []string{
				"mise use --global rust@1.75.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dockerfile, err := Generate(tt.config)
			require.NoError(t, err)

			for _, want := range tt.wantContains {
				assert.Contains(t, dockerfile, want, "expected Dockerfile to contain: %s", want)
			}

			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, dockerfile, notWant, "expected Dockerfile NOT to contain: %s", notWant)
			}
		})
	}
}

func TestGenerateDockerfileStructure(t *testing.T) {
	cfg := &config.Config{
		Languages: map[string]config.LanguageConfig{
			"node": {Version: "lts"},
		},
		Env: map[string]string{},
	}

	dockerfile, err := Generate(cfg)
	require.NoError(t, err)

	// Verify basic structure
	lines := strings.Split(dockerfile, "\n")

	// Should start with FROM
	assert.True(t, strings.HasPrefix(lines[0], "FROM"), "Dockerfile should start with FROM")

	// Should contain USER developer
	assert.Contains(t, dockerfile, "USER developer")

	// Should contain WORKDIR /workspace
	assert.Contains(t, dockerfile, "WORKDIR /workspace")

	// Should end with CMD
	assert.Contains(t, dockerfile, `CMD ["/bin/bash"]`)

	// Should install Mise
	assert.Contains(t, dockerfile, "curl https://mise.run")
}

func TestGenerateWithCodeServer(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "code-server disabled (default)",
			config: &config.Config{
				Languages:  map[string]config.LanguageConfig{"go": {Version: "1.22"}},
				Env:        map[string]string{},
				CodeServer: false,
			},
			wantNotContain: []string{
				"code-server.dev/install.sh",
				"--install-extension",
			},
		},
		{
			name: "code-server enabled with go",
			config: &config.Config{
				Languages:  map[string]config.LanguageConfig{"go": {Version: "1.22"}},
				Env:        map[string]string{},
				CodeServer: true,
			},
			wantContains: []string{
				"code-server.dev/install.sh",
				"--install-extension golang.go",
			},
		},
		{
			name: "code-server enabled with multiple languages",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{
					"node":   {Version: "lts"},
					"python": {Version: "3.12"},
				},
				Env:        map[string]string{},
				CodeServer: true,
			},
			wantContains: []string{
				"code-server.dev/install.sh",
				"--install-extension ms-python.python",
				"--install-extension dbaeumer.vscode-eslint",
			},
		},
		{
			name: "code-server enabled with no languages",
			config: &config.Config{
				Languages:  map[string]config.LanguageConfig{},
				Env:        map[string]string{},
				CodeServer: true,
			},
			wantContains: []string{
				"code-server.dev/install.sh",
			},
			wantNotContain: []string{
				"--install-extension", // No extensions if no languages
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dockerfile, err := Generate(tt.config)
			require.NoError(t, err)

			for _, want := range tt.wantContains {
				assert.Contains(t, dockerfile, want, "expected Dockerfile to contain: %s", want)
			}

			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, dockerfile, notWant, "expected Dockerfile NOT to contain: %s", notWant)
			}
		})
	}
}

func TestGenerateWithJavaIncludesSDKMAN(t *testing.T) {
	cfg := &config.Config{
		Languages: map[string]config.LanguageConfig{
			"java": {Version: "17", BuildSystem: "maven"},
		},
		Env: map[string]string{},
	}

	dockerfile, err := Generate(cfg)
	require.NoError(t, err)

	// Should include SDKMAN installation
	assert.Contains(t, dockerfile, "get.sdkman.io")
	assert.Contains(t, dockerfile, "sdkman-init.sh")

	// Should install Java and Maven via SDKMAN
	assert.Contains(t, dockerfile, "sdk install java")
	assert.Contains(t, dockerfile, "sdk install maven")
}
