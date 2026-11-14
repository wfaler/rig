package dockerfile

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wfaler/rig/internal/config"
)

var testNeovimRepo = &config.NeovimConfigRepo{
	Repo: "https://github.com/wfaler/arch-dotfiles.git",
	Path: ".config/nvim",
	Ref:  "main",
}

func TestMasonLanguages(t *testing.T) {
	tests := []struct {
		name      string
		languages []string
		expected  []string
	}{
		{"no languages: Lua only, for the config itself", nil, []string{"Lua"}},
		{"go", []string{"go"}, []string{"Go", "Lua"}},
		{"node brings the web languages", []string{"node"}, []string{"CSS", "HTML", "JavaScript", "Lua", "Svelte", "TypeScript", "Vue"}},
		{"java brings the JVM languages", []string{"java"}, []string{"Java", "Kotlin", "Lua", "Scala"}},
		{"several, sorted", []string{"rust", "python", "ruby"}, []string{"Lua", "Python", "Ruby", "Rust"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Languages: map[string]config.LanguageConfig{}}
			for _, lang := range tt.languages {
				cfg.Languages[lang] = config.LanguageConfig{}
			}
			assert.Equal(t, tt.expected, MasonLanguages(cfg))
		})
	}
}

func TestMasonLanguagesCoverSupportedLanguages(t *testing.T) {
	for lang := range config.SupportedLanguages {
		assert.NotEmpty(t, masonLanguages[lang], "no Mason languages for %s", lang)
	}
}

func TestNeovimConfigClone(t *testing.T) {
	tests := []struct {
		name           string
		repo           *config.NeovimConfigRepo
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "repo root on the default branch",
			repo: &config.NeovimConfigRepo{Repo: "https://github.com/a/nvim.git"},
			wantContains: []string{
				"git clone --depth 1 'https://github.com/a/nvim.git' " + NvimConfigCheckout,
				"ln -sfn '" + NvimConfigCheckout + "' /home/developer/.config/nvim",
			},
			wantNotContain: []string{"--branch", "--sparse", "sparse-checkout"},
		},
		{
			name: "subdirectory at a ref uses a sparse checkout",
			repo: testNeovimRepo,
			wantContains: []string{
				"git clone --depth 1 --branch 'main' --filter=blob:none --sparse 'https://github.com/wfaler/arch-dotfiles.git' " + NvimConfigCheckout,
				"git -C " + NvimConfigCheckout + " sparse-checkout set '.config/nvim'",
				"test -d '" + NvimConfigCheckout + "/.config/nvim'",
				"ln -sfn '" + NvimConfigCheckout + "/.config/nvim' /home/developer/.config/nvim",
			},
		},
		{
			name:         "trailing slash on the path",
			repo:         &config.NeovimConfigRepo{Repo: "https://github.com/a/b", Path: "nvim/"},
			wantContains: []string{"ln -sfn '" + NvimConfigCheckout + "/nvim' /home/developer/.config/nvim"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clone := neovimConfigClone(tt.repo)
			for _, want := range tt.wantContains {
				assert.Contains(t, clone, want)
			}
			for _, notWant := range tt.wantNotContain {
				assert.NotContains(t, clone, notWant)
			}
			assertBashSyntax(t, clone)
		})
	}
}

func TestNeovimSetup(t *testing.T) {
	cfg := &config.Config{Languages: map[string]config.LanguageConfig{"go": {}}}
	nvim := `"$HOME/.local/bin/mise" exec -- nvim --headless --cmd "luafile ` + NvimInitScriptPath + `" -c "lua `

	build := neovimSetup(cfg, false)
	assert.Equal(t, nvim+`RigNvimPlugins()" -c qa \
    && `+nvim+`RigNvimMason({'Go', 'Lua'})" -c qa \
    && `+nvim+`RigNvimParsers()" -c qa`, build)
	assertBashSyntax(t, build)

	update := neovimSetup(cfg, true)
	assert.Contains(t, update, `RigNvimParsers({ update = true })`)
	assertBashSyntax(t, update)
}

func TestGenerateNeovim(t *testing.T) {
	plainMarkers := []string{
		"RUN " + neovimInstall,
		"RUN " + neovimAliases(vimAliasDir, NvimBinPath),
	}
	configMarkers := []string{
		"COPY rig-nvim-init.lua " + NvimInitScriptPath,
		"sparse-checkout set '.config/nvim'",
		"RigNvimPlugins()",
		"RigNvimMason({'Lua'})",
		"RigNvimParsers()",
	}
	tests := []struct {
		name       string
		neovim     *config.NeovimConfig
		wantPlain  bool
		wantConfig bool
	}{
		{"installed by default, with Neovim's defaults", nil, true, false},
		{"with a config repository", &config.NeovimConfig{Config: testNeovimRepo}, true, true},
		{"disabled", &config.NeovimConfig{Enabled: boolPtr(false), Config: testNeovimRepo}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Languages: map[string]config.LanguageConfig{},
				Env:       map[string]string{},
				Neovim:    tt.neovim,
			}
			buildCtx, err := Generate(cfg)
			require.NoError(t, err)
			for _, m := range plainMarkers {
				assert.Equal(t, tt.wantPlain, strings.Contains(buildCtx.Dockerfile, m), m)
			}
			for _, m := range configMarkers {
				assert.Equal(t, tt.wantConfig, strings.Contains(buildCtx.Dockerfile, m), m)
			}
			_, hasInit := buildCtx.ExtraFiles[nvimInitScriptFile]
			assert.Equal(t, tt.wantConfig, hasInit)
			if tt.wantConfig {
				assert.Equal(t, nvimInitScript, string(buildCtx.ExtraFiles[nvimInitScriptFile]))
			}
		})
	}
}

func TestGenerateNeovimAfterRuntimes(t *testing.T) {
	cfg := &config.Config{
		Languages: map[string]config.LanguageConfig{"go": {}},
		Env:       map[string]string{},
		Neovim:    &config.NeovimConfig{Config: testNeovimRepo},
	}
	buildCtx, err := Generate(cfg)
	require.NoError(t, err)
	df := buildCtx.Dockerfile
	// Mason builds Go and npm tools, so the runtimes must already be there.
	setup := strings.Index(df, "RigNvimMason")
	require.Positive(t, setup)
	assert.Less(t, strings.Index(df, "mise use --global go@"), setup)
	assert.Less(t, strings.Index(df, "mise use --global node@lts"), setup)
}

func TestNeovimUpdateStep(t *testing.T) {
	tests := []struct {
		name     string
		neovim   *config.NeovimConfig
		wantStep bool
	}{
		{"no step without a config: mise upgrades Neovim", nil, false},
		{"step with a config", &config.NeovimConfig{Config: testNeovimRepo}, true},
		{"no step when disabled", &config.NeovimConfig{Enabled: boolPtr(false), Config: testNeovimRepo}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := updateSteps(&config.Config{Neovim: tt.neovim})
			assert.Equal(t, tt.wantStep, contains(stepNames(steps), "neovim"))
		})
	}

	t.Run("pulls, then re-runs setup with parser updates, after npm", func(t *testing.T) {
		steps := updateSteps(&config.Config{Neovim: &config.NeovimConfig{Config: testNeovimRepo}})
		body := stepBody(t, steps, "neovim")
		assert.True(t, strings.HasPrefix(body, "git -C "+NvimConfigCheckout+" pull --ff-only"))
		assert.Contains(t, body, "RigNvimParsers({ update = true })")
		names := stepNames(steps)
		assert.Less(t, indexOf(names, "npm"), indexOf(names, "neovim"))
	})
}

func assertBashSyntax(t *testing.T, script string) {
	t.Helper()
	requireBash(t)
	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err, "bash -n: %s\n%s", out, script)
}

func indexOf(items []string, item string) int {
	for i, s := range items {
		if s == item {
			return i
		}
	}
	return -1
}

func contains(items []string, item string) bool {
	return indexOf(items, item) >= 0
}

func TestNeovimAliases(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skipf("sh not available: %v", err)
	}
	// fakeNvim reports which binary ran and the arguments it received, one per line.
	fakeNvim := func(label string) string {
		return "#!/bin/sh\necho " + label + "\nfor a in \"$@\"; do echo \"$a\"; done\n"
	}
	tests := []struct {
		name         string
		installed    bool // whether the mise "latest" Neovim exists
		onPath       bool // whether an nvim is on PATH
		wantOutput   string
		wantExitCode int
	}{
		{"runs mise's latest Neovim", true, true, "latest\nfile one.txt\n+42\n", 0},
		{"falls back to nvim on PATH", false, true, "path\nfile one.txt\n+42\n", 0},
		{"fails when Neovim is missing", false, false, "", 127},
	}
	for _, tt := range tests {
		for _, name := range []string{"vim", "vi"} {
			t.Run(tt.name+" as "+name, func(t *testing.T) {
				dir := t.TempDir()
				binDir := filepath.Join(dir, "bin")
				pathDir := filepath.Join(dir, "path")
				require.NoError(t, os.MkdirAll(binDir, 0o755))
				require.NoError(t, os.MkdirAll(pathDir, 0o755))
				nvimPath := filepath.Join(dir, "latest", "nvim")
				if tt.installed {
					require.NoError(t, os.MkdirAll(filepath.Dir(nvimPath), 0o755))
					writeExecutable(t, filepath.Dir(nvimPath), "nvim", fakeNvim("latest"))
				}
				if tt.onPath {
					writeExecutable(t, pathDir, "nvim", fakeNvim("path"))
				}

				install := exec.Command("sh", "-c", neovimAliases(binDir, nvimPath))
				out, err := install.CombinedOutput()
				require.NoError(t, err, string(out))

				// Only the fake nvim and sh's own utilities may be found.
				cmd := exec.Command(filepath.Join(binDir, name), "file one.txt", "+42")
				cmd.Env = []string{"PATH=" + pathDir + ":/usr/bin:/bin"}
				var stdout strings.Builder
				cmd.Stdout = &stdout
				err = cmd.Run()
				if tt.wantExitCode == 0 {
					require.NoError(t, err)
				} else {
					var exitErr *exec.ExitError
					require.ErrorAs(t, err, &exitErr)
					assert.Equal(t, tt.wantExitCode, exitErr.ExitCode())
				}
				assert.Equal(t, tt.wantOutput, stdout.String())
			})
		}
	}
}

func TestNeovimAliasesQuoteThePath(t *testing.T) {
	aliases := neovimAliases("/usr/local/bin", "/opt/it's here/nvim")
	assert.Contains(t, aliases, "ln -sfn vim /usr/local/bin/vi")
	assert.Contains(t, aliases, "> /usr/local/bin/vim")

	// Running the printf alone yields the wrapper, with the path quoted for sh.
	printf := strings.SplitN(aliases, " > ", 2)[0]
	out, err := exec.Command("sh", "-c", printf).Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "\nnvim='/opt/it'\\''s here/nvim'\n")
	assert.True(t, strings.HasPrefix(string(out), "#!/bin/sh\n"))
}
