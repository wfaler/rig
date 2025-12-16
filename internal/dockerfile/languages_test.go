package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wfaler/rig/internal/config"
)

func TestGenerateLanguageInstall(t *testing.T) {
	tests := []struct {
		name         string
		lang         string
		cfg          config.LanguageConfig
		wantContains []string
	}{
		{
			name:         "go latest",
			lang:         "go",
			cfg:          config.LanguageConfig{Version: "latest"},
			wantContains: []string{"mise use --global go@latest"},
		},
		{
			name:         "go specific version",
			lang:         "go",
			cfg:          config.LanguageConfig{Version: "1.22"},
			wantContains: []string{"mise use --global go@1.22"},
		},
		{
			name:         "node lts",
			lang:         "node",
			cfg:          config.LanguageConfig{Version: "lts"},
			wantContains: []string{"mise use --global node@lts"},
		},
		{
			name:         "node latest",
			lang:         "node",
			cfg:          config.LanguageConfig{Version: "latest"},
			wantContains: []string{"mise use --global node@latest"},
		},
		{
			name:         "node specific version",
			lang:         "node",
			cfg:          config.LanguageConfig{Version: "20.10.0"},
			wantContains: []string{"mise use --global node@20.10.0"},
		},
		{
			name:         "python latest",
			lang:         "python",
			cfg:          config.LanguageConfig{Version: "latest"},
			wantContains: []string{"mise use --global python@latest"},
		},
		{
			name:         "python specific version",
			lang:         "python",
			cfg:          config.LanguageConfig{Version: "3.12"},
			wantContains: []string{"mise use --global python@3.12"},
		},
		{
			name:         "java default uses SDKMAN",
			lang:         "java",
			cfg:          config.LanguageConfig{},
			wantContains: []string{"sdkman-init.sh", "sdk install java", "21-tem"},
		},
		{
			name:         "java specific version",
			lang:         "java",
			cfg:          config.LanguageConfig{Version: "17"},
			wantContains: []string{"sdk install java 17-tem"},
		},
		{
			name:         "rust latest",
			lang:         "rust",
			cfg:          config.LanguageConfig{Version: "latest"},
			wantContains: []string{"mise use --global rust@latest"},
		},
		{
			name:         "rust specific version",
			lang:         "rust",
			cfg:          config.LanguageConfig{Version: "1.75.0"},
			wantContains: []string{"mise use --global rust@1.75.0"},
		},
		{
			name:         "ruby latest",
			lang:         "ruby",
			cfg:          config.LanguageConfig{Version: "latest"},
			wantContains: []string{"mise use --global ruby@latest"},
		},
		{
			name:         "unknown language",
			lang:         "cobol",
			cfg:          config.LanguageConfig{},
			wantContains: []string{"Unknown language: cobol"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateLanguageInstall(tt.lang, tt.cfg)
			for _, want := range tt.wantContains {
				assert.Contains(t, result, want)
			}
		})
	}
}

func TestGenerateBuildSystemInstall(t *testing.T) {
	tests := []struct {
		name         string
		lang         string
		cfg          config.LanguageConfig
		want         string
		wantContains []string
	}{
		{
			name:         "node yarn",
			lang:         "node",
			cfg:          config.LanguageConfig{BuildSystems: map[string]string{"yarn": "true"}},
			wantContains: []string{"npm install -g yarn"},
		},
		{
			name:         "node pnpm",
			lang:         "node",
			cfg:          config.LanguageConfig{BuildSystems: map[string]string{"pnpm": "true"}},
			wantContains: []string{"npm install -g pnpm"},
		},
		{
			name: "node npm returns empty",
			lang: "node",
			cfg:  config.LanguageConfig{BuildSystems: map[string]string{"npm": "true"}},
			want: "",
		},
		{
			name:         "python poetry",
			lang:         "python",
			cfg:          config.LanguageConfig{BuildSystems: map[string]string{"poetry": "true"}},
			wantContains: []string{"pip install poetry"},
		},
		{
			name:         "python poetry with version",
			lang:         "python",
			cfg:          config.LanguageConfig{BuildSystems: map[string]string{"poetry": "1.7.0"}},
			wantContains: []string{"pip install poetry==1.7.0"},
		},
		{
			name:         "python pipenv",
			lang:         "python",
			cfg:          config.LanguageConfig{BuildSystems: map[string]string{"pipenv": "true"}},
			wantContains: []string{"pip install pipenv"},
		},
		{
			name:         "java gradle via SDKMAN",
			lang:         "java",
			cfg:          config.LanguageConfig{BuildSystems: map[string]string{"gradle": "true"}},
			wantContains: []string{"sdkman-init.sh", "sdk install gradle"},
		},
		{
			name:         "java gradle with version via SDKMAN",
			lang:         "java",
			cfg:          config.LanguageConfig{BuildSystems: map[string]string{"gradle": "8.5"}},
			wantContains: []string{"sdk install gradle 8.5"},
		},
		{
			name:         "java maven via SDKMAN",
			lang:         "java",
			cfg:          config.LanguageConfig{BuildSystems: map[string]string{"maven": "true"}},
			wantContains: []string{"sdk install maven"},
		},
		{
			name:         "java sbt via SDKMAN",
			lang:         "java",
			cfg:          config.LanguageConfig{BuildSystems: map[string]string{"sbt": "true"}},
			wantContains: []string{"sdk install sbt"},
		},
		{
			name:         "ruby bundler",
			lang:         "ruby",
			cfg:          config.LanguageConfig{BuildSystems: map[string]string{"bundler": "true"}},
			wantContains: []string{"gem install bundler"},
		},
		{
			name: "no build system",
			lang: "node",
			cfg:  config.LanguageConfig{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateBuildSystemInstall(tt.lang, tt.cfg)

			if tt.want != "" || len(tt.wantContains) == 0 {
				assert.Equal(t, tt.want, result)
			}

			for _, want := range tt.wantContains {
				assert.Contains(t, result, want)
			}
		})
	}
}

func TestGetExtensionsForLanguages(t *testing.T) {
	tests := []struct {
		name      string
		languages []string
		want      []string
	}{
		{
			name:      "no languages",
			languages: []string{},
			want:      nil,
		},
		{
			name:      "go only",
			languages: []string{"go"},
			want:      []string{"golang.go"},
		},
		{
			name:      "python only",
			languages: []string{"python"},
			want:      []string{"ms-python.python", "ms-python.vscode-pylance", "ms-python.debugpy"},
		},
		{
			name:      "multiple languages",
			languages: []string{"go", "rust"},
			want:      []string{"golang.go", "rust-lang.rust-analyzer"},
		},
		{
			name:      "unknown language ignored",
			languages: []string{"cobol", "go"},
			want:      []string{"golang.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetExtensionsForLanguages(tt.languages)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVSCodeExtensionsMapping(t *testing.T) {
	// Verify all supported languages have extensions defined
	supportedLanguages := []string{"go", "node", "python", "java", "rust", "ruby"}

	for _, lang := range supportedLanguages {
		t.Run(lang, func(t *testing.T) {
			exts, ok := VSCodeExtensionsForLanguage[lang]
			assert.True(t, ok, "language %s should have extensions defined", lang)
			assert.NotEmpty(t, exts, "language %s should have at least one extension", lang)
		})
	}
}

func TestGenerateBuildSystemsInstall(t *testing.T) {
	tests := []struct {
		name         string
		lang         string
		cfg          config.LanguageConfig
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:      "no build systems returns empty",
			lang:      "java",
			cfg:       config.LanguageConfig{},
			wantEmpty: true,
		},
		{
			name: "single build system via new format",
			lang: "java",
			cfg: config.LanguageConfig{
				BuildSystems: map[string]string{
					"gradle": "8.5",
				},
			},
			wantContains: []string{"sdk install gradle 8.5"},
		},
		{
			name: "multiple java build systems",
			lang: "java",
			cfg: config.LanguageConfig{
				BuildSystems: map[string]string{
					"gradle": "8.5",
					"maven":  "true",
					"sbt":    "1.9.8",
				},
			},
			wantContains: []string{
				"sdk install gradle 8.5",
				"sdk install maven",
				"sdk install sbt 1.9.8",
			},
		},
		{
			name: "multiple python build systems",
			lang: "python",
			cfg: config.LanguageConfig{
				BuildSystems: map[string]string{
					"poetry": "1.8.0",
					"pipenv": "latest",
				},
			},
			wantContains: []string{
				"pip install poetry==1.8.0",
				"pip install pipenv",
			},
		},
		{
			name: "multiple node build systems",
			lang: "node",
			cfg: config.LanguageConfig{
				BuildSystems: map[string]string{
					"yarn": "true",
					"pnpm": "true",
				},
			},
			wantContains: []string{
				"npm install -g yarn",
				"npm install -g pnpm",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateBuildSystemsInstall(tt.lang, tt.cfg)

			if tt.wantEmpty {
				assert.Empty(t, result)
				return
			}

			for _, want := range tt.wantContains {
				assert.Contains(t, result, want)
			}
		})
	}
}
