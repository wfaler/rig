package dockerfile

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wfaler/rig/internal/config"
	"pgregory.net/rapid"
)

func boolPtr(b bool) *bool { return &b }

func stepNames(steps []updateStep) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name
	}
	return names
}

func stepBody(t *testing.T, steps []updateStep, name string) string {
	t.Helper()
	for _, s := range steps {
		if s.Name == name {
			return s.Body
		}
	}
	require.Failf(t, "step not found", "no %q step in %v", name, stepNames(steps))
	return ""
}

func TestUpdateStepsSelection(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.Config
		expected []string
	}{
		{
			name:     "defaults: zsh and herdr on",
			config:   &config.Config{},
			expected: []string{"system", "mise", "claude", "npm", "herdr", "oh-my-zsh"},
		},
		{
			name: "bash without herdr has only the core steps",
			config: &config.Config{
				Shell: "bash",
				Herdr: &config.HerdrConfig{Enabled: boolPtr(false)},
			},
			expected: []string{"system", "mise", "claude", "npm"},
		},
		{
			name: "every optional step, runtimes before their packages",
			config: &config.Config{
				Shell: "zsh",
				Languages: map[string]config.LanguageConfig{
					"java":   {BuildSystems: map[string]string{"gradle": "true"}},
					"python": {BuildSystems: map[string]string{"uv": "true"}},
					"ruby":   {BuildSystems: map[string]string{"bundler": "true"}},
				},
				CodeServer: &config.CodeServerConfig{Enabled: true},
			},
			expected: []string{"system", "mise", "sdkman", "claude", "npm", "pip", "gem", "herdr", "code-server", "oh-my-zsh"},
		},
		{
			name: "build systems that ship with their runtime add no step",
			config: &config.Config{
				Shell: "bash",
				Herdr: &config.HerdrConfig{Enabled: boolPtr(false)},
				Languages: map[string]config.LanguageConfig{
					"python": {BuildSystems: map[string]string{"pip": "true"}},
					"ruby":   {BuildSystems: map[string]string{"gem": "true"}},
				},
			},
			expected: []string{"system", "mise", "claude", "npm"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, stepNames(updateSteps(tt.config)))
		})
	}
}

func TestUpdateStepBodies(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		step           string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:           "npm reinstalls the agents, without marked when the markdown server is off",
			config:         &config.Config{MarkdownServer: &config.MarkdownServerConfig{Enabled: false}},
			step:           "npm",
			wantContains:   []string{"mx npm install -g " + agentNPMPackages},
			wantNotContain: []string{"marked"},
		},
		{
			name: "npm adds marked and node build systems in a stable order",
			config: &config.Config{Languages: map[string]config.LanguageConfig{
				"node": {BuildSystems: map[string]string{"yarn": "true", "pnpm": "true", "npm": "true"}},
			}},
			step:         "npm",
			wantContains: []string{"mx npm install -g " + agentNPMPackages + " " + markdownNPMPackage + " pnpm yarn"},
		},
		{
			name: "pip upgrades unpinned tools and holds pinned ones",
			config: &config.Config{Languages: map[string]config.LanguageConfig{
				"python": {BuildSystems: map[string]string{"poetry": "1.8.0", "uv": "latest", "pipenv": "true"}},
			}},
			step:         "pip",
			wantContains: []string{"mx pip install --upgrade pipenv poetry==1.8.0 uv"},
		},
		{
			name: "mise reapplies the python lib fix when python is configured",
			config: &config.Config{Languages: map[string]config.LanguageConfig{
				"python": {Version: "3.12"},
			}},
			step:         "mise",
			wantContains: []string{`"$MISE" self-update --yes`, `"$MISE" upgrade`, pythonLibFix},
		},
		{
			name:           "mise skips the python lib fix without python",
			config:         &config.Config{},
			step:           "mise",
			wantNotContain: []string{"mise where python"},
		},
		{
			name: "sdkman upgrades only unpinned build systems, never java",
			config: &config.Config{Languages: map[string]config.LanguageConfig{
				"java": {Version: "21", BuildSystems: map[string]string{"gradle": "true", "maven": "3.9.6", "sbt": "latest"}},
			}},
			step:           "sdkman",
			wantContains:   []string{"sdk selfupdate", "sdk update", "sdk upgrade gradle", "sdk upgrade sbt", "sdkman_auto_answer=true"},
			wantNotContain: []string{"maven", "sdk upgrade java"},
		},
		{
			name: "code-server force-reinstalls each extension",
			config: &config.Config{CodeServer: &config.CodeServerConfig{
				Enabled:    true,
				Extensions: []string{"golang.go", "rust-lang.rust-analyzer"},
			}},
			step: "code-server",
			wantContains: []string{
				codeServerInstall,
				"code-server --install-extension golang.go --force",
				"code-server --install-extension rust-lang.rust-analyzer --force",
			},
		},
		{
			name:         "herdr refreshes the binary and the skill from the Dockerfile's sources",
			config:       &config.Config{},
			step:         "herdr",
			wantContains: []string{herdrBinaryURL, herdrSkillURL, "sudo install -m 755"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := stepBody(t, updateSteps(tt.config), tt.step)
			for _, want := range tt.wantContains {
				assert.Contains(t, body, want)
			}
			for _, unwanted := range tt.wantNotContain {
				assert.NotContains(t, body, unwanted)
			}
		})
	}
}

func TestShellQuote(t *testing.T) {
	requireBash(t)
	tests := []struct {
		name  string
		input string
	}{
		{"plain", "Claude Code"},
		{"empty", ""},
		{"single quote", "it's"},
		{"shell metacharacters", "$HOME `id` \"x\" ; | & \\"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := exec.Command("bash", "-c", "printf %s "+shellQuote(tt.input)).Output()
			require.NoError(t, err)
			assert.Equal(t, tt.input, string(out))
		})
	}
}

func requireBash(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
}

// updateScriptHarness writes the update script for a minimal config into a
// fake home whose mise and claude are stubs that log their arguments, so the
// runner can be exercised without touching the real toolchain.
type updateScriptHarness struct {
	home   string
	script string
	log    string
}

func newUpdateScriptHarness(t *testing.T) *updateScriptHarness {
	t.Helper()
	requireBash(t)
	home := t.TempDir()
	bin := filepath.Join(home, ".local", "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))

	h := &updateScriptHarness{
		home:   home,
		script: filepath.Join(home, "rig-update"),
		log:    filepath.Join(home, "calls.log"),
	}
	// Each stub logs "name args" and exits with $FAIL_<NAME> (default 0).
	for _, name := range []string{"mise", "claude"} {
		stub := "#!/usr/bin/env bash\necho \"" + name + " $*\" >> \"$CALLS_LOG\"\nexit \"${FAIL_" + strings.ToUpper(name) + ":-0}\"\n"
		require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte(stub), 0o755))
	}

	script, err := GenerateUpdateScript(&config.Config{
		Shell: "bash",
		Herdr: &config.HerdrConfig{Enabled: boolPtr(false)},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(h.script, []byte(script), 0o755))
	return h
}

func (h *updateScriptHarness) run(t *testing.T, env []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{h.script}, args...)...)
	cmd.Env = append([]string{"HOME=" + h.home, "PATH=/usr/bin:/bin", "CALLS_LOG=" + h.log}, env...)
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else {
		require.NoError(t, err)
	}
	return out.String(), errOut.String(), exitCode
}

func (h *updateScriptHarness) calls(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(h.log)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestUpdateScriptRunner(t *testing.T) {
	t.Run("selected steps run in canonical order, not argument order", func(t *testing.T) {
		h := newUpdateScriptHarness(t)
		stdout, _, code := h.run(t, nil, "claude", "mise")
		assert.Equal(t, 0, code)
		assert.Equal(t, []string{"mise self-update --yes", "mise upgrade", "claude update"}, h.calls(t))
		assert.Contains(t, stdout, "==> mise: done")
		assert.Contains(t, stdout, "==> claude: done")
		assert.Contains(t, stdout, "all steps succeeded")
	})

	t.Run("a failing step is reported and the rest still run", func(t *testing.T) {
		h := newUpdateScriptHarness(t)
		_, stderr, code := h.run(t, []string{"FAIL_MISE=1"}, "mise", "claude")
		assert.Equal(t, 1, code)
		assert.Equal(t, []string{"mise self-update --yes", "claude update"}, h.calls(t))
		assert.Contains(t, stderr, "==> mise: FAILED")
		assert.Contains(t, stderr, "1 step(s) failed: mise")
		assert.Contains(t, stderr, "rig-update mise")
	})

	t.Run("list shows every step without running any", func(t *testing.T) {
		h := newUpdateScriptHarness(t)
		stdout, _, code := h.run(t, nil, "--list")
		assert.Equal(t, 0, code)
		for _, name := range []string{"system", "mise", "claude", "npm"} {
			assert.Contains(t, stdout, "  "+name+" ")
		}
		assert.Nil(t, h.calls(t))
	})

	t.Run("help exits cleanly", func(t *testing.T) {
		h := newUpdateScriptHarness(t)
		stdout, _, code := h.run(t, nil, "--help")
		assert.Equal(t, 0, code)
		assert.Contains(t, stdout, "Usage: rig-update")
	})

	t.Run("unknown step is rejected before anything runs", func(t *testing.T) {
		h := newUpdateScriptHarness(t)
		_, stderr, code := h.run(t, nil, "mise", "sdkman")
		assert.Equal(t, 2, code)
		assert.Contains(t, stderr, "unknown step: sdkman")
		assert.Nil(t, h.calls(t))
	})

	t.Run("unknown option is rejected", func(t *testing.T) {
		h := newUpdateScriptHarness(t)
		_, stderr, code := h.run(t, nil, "--yes")
		assert.Equal(t, 2, code)
		assert.Contains(t, stderr, "unknown option: --yes")
	})
}

// TestGenerateUpdateScriptProperties checks invariants over arbitrary valid
// configs: the script is valid bash, its steps are unique and each is wired to
// a function, and runtimes are always upgraded before their packages.
func TestGenerateUpdateScriptProperties(t *testing.T) {
	requireBash(t)
	buildSystemVersion := rapid.SampledFrom([]string{"true", "latest", "", "1.2.3"})

	rapid.Check(t, func(rt *rapid.T) {
		cfg := &config.Config{
			Shell:     rapid.SampledFrom([]string{"", "bash", "zsh", "fish"}).Draw(rt, "shell"),
			Languages: map[string]config.LanguageConfig{},
			Herdr:     &config.HerdrConfig{Enabled: boolPtr(rapid.Bool().Draw(rt, "herdr"))},
			CodeServer: &config.CodeServerConfig{
				Enabled:    rapid.Bool().Draw(rt, "codeServer"),
				Extensions: rapid.SliceOfN(rapid.StringMatching(`[a-z]+\.[a-z-]+`), 0, 3).Draw(rt, "extensions"),
			},
			MarkdownServer: &config.MarkdownServerConfig{Enabled: rapid.Bool().Draw(rt, "markdown")},
		}
		langs := make([]string, 0, len(config.BuildSystemsForLanguage))
		for lang := range config.BuildSystemsForLanguage {
			langs = append(langs, lang)
		}
		sort.Strings(langs) // map order would make draws unreproducible
		for _, lang := range langs {
			systems := config.BuildSystemsForLanguage[lang]
			if !rapid.Bool().Draw(rt, "has-"+lang) {
				continue
			}
			langCfg := config.LanguageConfig{BuildSystems: map[string]string{}}
			for _, bs := range systems {
				if rapid.Bool().Draw(rt, "has-"+lang+"-"+bs) {
					langCfg.BuildSystems[bs] = buildSystemVersion.Draw(rt, lang+"-"+bs+"-version")
				}
			}
			cfg.Languages[lang] = langCfg
		}

		script, err := GenerateUpdateScript(cfg)
		require.NoError(rt, err)
		require.True(rt, strings.HasPrefix(script, "#!/usr/bin/env bash\n"))

		syntax := exec.Command("bash", "-n")
		syntax.Stdin = strings.NewReader(script)
		out, err := syntax.CombinedOutput()
		require.NoError(rt, err, "bash -n: %s\n%s", out, script)

		steps := updateSteps(cfg)
		position := map[string]int{}
		for i, s := range steps {
			_, dup := position[s.Name]
			require.False(rt, dup, "duplicate step %q", s.Name)
			position[s.Name] = i
			require.Contains(rt, script, s.FuncName()+"() {")
			require.Contains(rt, script, "["+s.Name+"]="+s.FuncName())
			require.NotEmpty(rt, strings.TrimSpace(s.Body), "step %q has an empty body", s.Name)
		}
		for _, pkgStep := range []string{"npm", "pip", "gem"} {
			if i, ok := position[pkgStep]; ok {
				require.Less(rt, position["mise"], i, "mise must run before %s", pkgStep)
			}
		}
	})
}
