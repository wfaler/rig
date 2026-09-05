package dockerfile

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wfaler/rig/internal/config"
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
				"curl -fsSL https://claude.ai/install.sh | bash",
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
					"node": {Version: "lts", BuildSystems: map[string]string{"npm": "true"}},
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
					"node":   {Version: "20", BuildSystems: map[string]string{"yarn": "true"}},
					"python": {Version: "3.12", BuildSystems: map[string]string{"poetry": "true"}},
					"go":     {Version: "1.22"},
				},
				Env: map[string]string{},
			},
			wantContains: []string{
				"mise use --global node@20",
				"mise use --global python@3.12",
				"mise where python",
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
					"java": {Version: "21", BuildSystems: map[string]string{"gradle": "8.5"}},
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
			buildCtx, err := Generate(tt.config)
			require.NoError(t, err)

			for _, want := range tt.wantContains {
				assert.Contains(t, buildCtx.Dockerfile, want, "expected Dockerfile to contain: %s", want)
			}

			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, buildCtx.Dockerfile, notWant, "expected Dockerfile NOT to contain: %s", notWant)
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

	buildCtx, err := Generate(cfg)
	require.NoError(t, err)

	// Verify basic structure
	lines := strings.Split(buildCtx.Dockerfile, "\n")

	// Should start with FROM
	assert.True(t, strings.HasPrefix(lines[0], "FROM"), "Dockerfile should start with FROM")

	// Should contain USER developer
	assert.Contains(t, buildCtx.Dockerfile, "USER developer")

	// Should contain WORKDIR /workspace
	assert.Contains(t, buildCtx.Dockerfile, "WORKDIR /workspace")

	// Should end with CMD (default shell is zsh)
	assert.Contains(t, buildCtx.Dockerfile, `CMD ["/bin/zsh"]`)

	// Should install Mise
	assert.Contains(t, buildCtx.Dockerfile, "curl https://mise.run")
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
				Languages: map[string]config.LanguageConfig{"go": {Version: "1.22"}},
				Env:       map[string]string{},
			},
			wantNotContain: []string{
				"code-server.dev/install.sh",
				"--install-extension",
			},
		},
		{
			name: "code-server enabled without extensions",
			config: &config.Config{
				Languages:  map[string]config.LanguageConfig{"go": {Version: "1.22"}},
				Env:        map[string]string{},
				CodeServer: &config.CodeServerConfig{Enabled: true},
			},
			wantContains: []string{
				"code-server.dev/install.sh",
			},
			wantNotContain: []string{
				"--install-extension", // No auto-install of language extensions
			},
		},
		{
			name: "code-server enabled with explicit extensions",
			config: &config.Config{
				Languages:  map[string]config.LanguageConfig{"go": {Version: "1.22"}},
				Env:        map[string]string{},
				CodeServer: &config.CodeServerConfig{Enabled: true, Extensions: []string{"golang.go", "github.copilot"}},
			},
			wantContains: []string{
				"code-server.dev/install.sh",
				"--install-extension golang.go",
				"--install-extension github.copilot",
			},
		},
		{
			name: "code-server with custom theme",
			config: &config.Config{
				Languages:  map[string]config.LanguageConfig{},
				Env:        map[string]string{},
				CodeServer: &config.CodeServerConfig{Enabled: true, Theme: "Monokai"},
			},
			wantContains: []string{
				"code-server.dev/install.sh",
				`"workbench.colorTheme": "Monokai"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildCtx, err := Generate(tt.config)
			require.NoError(t, err)

			for _, want := range tt.wantContains {
				assert.Contains(t, buildCtx.Dockerfile, want, "expected Dockerfile to contain: %s", want)
			}

			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, buildCtx.Dockerfile, notWant, "expected Dockerfile NOT to contain: %s", notWant)
			}
		})
	}
}

func TestGenerateWithMarkdownServer(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "markdown server enabled by default",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{},
				Env:       map[string]string{},
			},
			wantContains: []string{
				"npm install -g marked@latest",
				"rig-md-server.js",
				"RIG_MD_PORT=3030",
			},
		},
		{
			name: "markdown server with custom port",
			config: &config.Config{
				Languages:      map[string]config.LanguageConfig{},
				Env:            map[string]string{},
				MarkdownServer: &config.MarkdownServerConfig{Enabled: true, Port: 4040},
			},
			wantContains: []string{
				"npm install -g marked@latest",
				"RIG_MD_PORT=4040",
			},
		},
		{
			name: "markdown server explicitly disabled",
			config: &config.Config{
				Languages:      map[string]config.LanguageConfig{},
				Env:            map[string]string{},
				MarkdownServer: &config.MarkdownServerConfig{Enabled: false},
			},
			wantNotContain: []string{
				"npm install -g marked",
				"ENV RIG_MD_PORT",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildCtx, err := Generate(tt.config)
			require.NoError(t, err)

			for _, want := range tt.wantContains {
				assert.Contains(t, buildCtx.Dockerfile, want, "expected Dockerfile to contain: %s", want)
			}

			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, buildCtx.Dockerfile, notWant, "expected Dockerfile NOT to contain: %s", notWant)
			}
		})
	}
}

func TestGenerateWithJavaIncludesSDKMAN(t *testing.T) {
	cfg := &config.Config{
		Languages: map[string]config.LanguageConfig{
			"java": {Version: "17", BuildSystems: map[string]string{"maven": "true"}},
		},
		Env: map[string]string{},
	}

	buildCtx, err := Generate(cfg)
	require.NoError(t, err)

	// Should include SDKMAN installation
	assert.Contains(t, buildCtx.Dockerfile, "get.sdkman.io")
	assert.Contains(t, buildCtx.Dockerfile, "sdkman-init.sh")

	// Should install Java and Maven via SDKMAN
	assert.Contains(t, buildCtx.Dockerfile, "sdk install java")
	assert.Contains(t, buildCtx.Dockerfile, "sdk install maven")
}

func TestGenerateWithShellConfiguration(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "default zsh shell with oh-my-zsh",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{},
				Env:       map[string]string{},
			},
			wantContains: []string{
				"useradd -m -s /bin/zsh developer",
				`CMD ["/bin/zsh"]`,
				`mise activate zsh`,
				"ohmyzsh",
			},
			wantNotContain: []string{
				"fish",
			},
		},
		{
			name: "bash shell explicitly set",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{},
				Env:       map[string]string{},
				Shell:     "bash",
			},
			wantContains: []string{
				"useradd -m -s /bin/bash developer",
				`CMD ["/bin/bash"]`,
				`mise activate bash`,
			},
			wantNotContain: []string{
				"oh-my-zsh",
				"fish",
			},
		},
		{
			name: "zsh with oh-my-zsh",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{},
				Env:       map[string]string{},
				Shell:     "zsh",
			},
			wantContains: []string{
				"useradd -m -s /bin/zsh developer",
				`CMD ["/bin/zsh"]`,
				"ohmyzsh",
				"zsh",
				`mise activate zsh`,
				">> ~/.zshrc",
			},
		},
		{
			name: "fish shell",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{},
				Env:       map[string]string{},
				Shell:     "fish",
			},
			wantContains: []string{
				"useradd -m -s /bin/fish developer",
				`CMD ["/bin/fish"]`,
				"fish",
				"mise activate fish | source",
				"~/.config/fish/config.fish",
			},
			wantNotContain: []string{
				"oh-my-zsh",
			},
		},
		{
			name: "zsh with java and sdkman",
			config: &config.Config{
				Languages: map[string]config.LanguageConfig{
					"java": {Version: "21"},
				},
				Env:   map[string]string{},
				Shell: "zsh",
			},
			wantContains: []string{
				"useradd -m -s /bin/zsh developer",
				`mise activate zsh`,
				"sdkman-init.sh",
				">> ~/.zshrc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildCtx, err := Generate(tt.config)
			require.NoError(t, err)

			for _, want := range tt.wantContains {
				assert.Contains(t, buildCtx.Dockerfile, want, "expected Dockerfile to contain: %s", want)
			}

			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, buildCtx.Dockerfile, notWant, "expected Dockerfile NOT to contain: %s", notWant)
			}
		})
	}
}

func TestGenerateHerdr(t *testing.T) {
	enabled := true
	disabled := false

	herdrMarkers := []string{
		"releases/latest/download/herdr-linux-",
		"/usr/local/bin/herdr",
		"skills/herdr/SKILL.md",
		"Fix herdr socket permissions",
		"/run/herdr/herdr.sock",
		"socat",
		"RIG_HERDR_PROXY_PORT",
		"rig-sandbox-skill.md",
		"skills/rig-sandbox/SKILL.md",
	}

	t.Run("enabled by default", func(t *testing.T) {
		cfg := &config.Config{
			Languages: map[string]config.LanguageConfig{},
			Env:       map[string]string{},
		}
		buildCtx, err := Generate(cfg)
		require.NoError(t, err)
		for _, m := range herdrMarkers {
			assert.Contains(t, buildCtx.Dockerfile, m, "default config should install herdr: %s", m)
		}
		assert.Contains(t, buildCtx.ExtraFiles, "rig-sandbox-skill.md")
		assert.Contains(t, string(buildCtx.ExtraFiles["rig-sandbox-skill.md"]), "RIG_HOST_WORKDIR")
	})

	t.Run("explicitly enabled", func(t *testing.T) {
		cfg := &config.Config{
			Languages: map[string]config.LanguageConfig{},
			Env:       map[string]string{},
			Herdr:     &config.HerdrConfig{Enabled: &enabled},
		}
		buildCtx, err := Generate(cfg)
		require.NoError(t, err)
		for _, m := range herdrMarkers {
			assert.Contains(t, buildCtx.Dockerfile, m)
		}
	})

	t.Run("disabled omits all herdr content", func(t *testing.T) {
		cfg := &config.Config{
			Languages: map[string]config.LanguageConfig{},
			Env:       map[string]string{},
			Herdr:     &config.HerdrConfig{Enabled: &disabled},
		}
		buildCtx, err := Generate(cfg)
		require.NoError(t, err)
		for _, m := range herdrMarkers {
			assert.NotContains(t, buildCtx.Dockerfile, m, "disabled config must not install herdr: %s", m)
		}
		assert.NotContains(t, buildCtx.ExtraFiles, "rig-sandbox-skill.md")
		// Structural invariants still hold.
		assert.True(t, strings.HasPrefix(buildCtx.Dockerfile, "FROM "))
		assert.Contains(t, buildCtx.Dockerfile, "CMD [")
	})
}
