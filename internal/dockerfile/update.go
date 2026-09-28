package dockerfile

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/wfaler/rig/internal/config"
)

// Install commands shared by the Dockerfile and the update script, so that an
// update always refreshes exactly what the image installed.
const (
	// agentNPMPackages are the npm-distributed AI agents installed globally.
	agentNPMPackages = "@google/gemini-cli openai"

	// markdownNPMPackage is the markdown parser the markdown server needs.
	markdownNPMPackage = "marked@latest"

	// herdrBinaryURL is the herdr CLI release asset; HERDR_ARCH must hold
	// the `uname -m` machine name, which matches the asset names.
	herdrBinaryURL = "https://github.com/herdrdev/herdr/releases/latest/download/herdr-linux-${HERDR_ARCH}"

	// herdrSkillURL is the herdr agent skill Claude Code auto-discovers.
	herdrSkillURL = "https://raw.githubusercontent.com/herdrdev/herdr/master/skills/herdr/SKILL.md"

	// codeServerInstall installs, or re-run upgrades, code-server.
	codeServerInstall = "curl -fsSL https://code-server.dev/install.sh | sh"
)

// UpdateScriptPath is where the update script is installed in the image.
const UpdateScriptPath = "/usr/local/bin/rig-update"

// updateScriptFile is the update script's name in the build context.
const updateScriptFile = "rig-update"

// updateStep is one independently runnable part of the update script.
type updateStep struct {
	Name        string // name used on the rig-update command line
	Description string // one line shown by rig-update --list
	Body        string // bash run in a subshell; the step fails if it does
}

// FuncName is the bash function implementing the step.
func (s updateStep) FuncName() string {
	return "step_" + strings.ReplaceAll(s.Name, "-", "_")
}

// GenerateUpdateScript renders the in-container update script for a config.
func GenerateUpdateScript(cfg *config.Config) (string, error) {
	tmpl, err := template.New("rig-update").Funcs(template.FuncMap{"shellQuote": shellQuote}).Parse(updateScriptTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing update script template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Steps []updateStep }{updateSteps(cfg)}); err != nil {
		return "", fmt.Errorf("executing update script template: %w", err)
	}
	return buf.String(), nil
}

// shellQuote quotes s as a single bash word.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// updateSteps lists the update steps for a config, in the order they must run:
// runtimes are upgraded before the packages installed into them are
// reinstalled, since a new runtime version starts without them.
func updateSteps(cfg *config.Config) []updateStep {
	steps := []updateStep{
		{
			Name:        "system",
			Description: "Debian packages, including the docker CLI and GitHub CLI",
			Body: `sudo DEBIAN_FRONTEND=noninteractive apt-get update \
    && sudo DEBIAN_FRONTEND=noninteractive apt-get upgrade -y -o Dpkg::Options::=--force-confold \
    && sudo apt-get clean`,
		},
		{
			Name:        "mise",
			Description: "mise and the runtimes it manages, within their pinned versions",
			Body:        miseUpdateBody(cfg),
		},
	}
	if cfg.HasLanguage("java") {
		steps = append(steps, updateStep{
			Name:        "sdkman",
			Description: "SDKMAN and its unpinned build systems (Java stays at its pinned version)",
			Body:        sdkmanUpdateBody(cfg),
		})
	}
	steps = append(steps,
		updateStep{
			Name:        "claude",
			Description: "Claude Code",
			Body:        `"$HOME/.local/bin/claude" update`,
		},
		updateStep{
			Name:        "npm",
			Description: "global npm packages: " + strings.Join(npmPackages(cfg), ", "),
			Body:        "mx npm install -g " + strings.Join(npmPackages(cfg), " "),
		},
	)
	if reqs := languageBuildSystems(cfg, "python", pythonBuildSystemRequirement); len(reqs) > 0 {
		steps = append(steps, updateStep{
			Name:        "pip",
			Description: "Python build systems: " + strings.Join(reqs, ", "),
			Body:        "mx pip install --upgrade " + strings.Join(reqs, " "),
		})
	}
	if gems := languageBuildSystems(cfg, "ruby", func(bs, _ string) string { return rubyBuildSystemGem(bs) }); len(gems) > 0 {
		steps = append(steps, updateStep{
			Name:        "gem",
			Description: "Ruby build systems: " + strings.Join(gems, ", "),
			Body:        "mx gem install " + strings.Join(gems, " "),
		})
	}
	if cfg.IsHerdrEnabled() {
		steps = append(steps, updateStep{
			Name:        "herdr",
			Description: "herdr CLI and its agent skill",
			Body: `HERDR_ARCH="$(uname -m)" \
    && tmp="$(mktemp)" \
    && trap 'rm -f "$tmp"' EXIT \
    && curl -fsSL "` + herdrBinaryURL + `" -o "$tmp" \
    && sudo install -m 755 "$tmp" /usr/local/bin/herdr \
    && mkdir -p "$HOME/.claude/skills/herdr" \
    && curl -fsSL ` + herdrSkillURL + ` -o "$HOME/.claude/skills/herdr/SKILL.md"`,
		})
	}
	if cfg.IsCodeServerEnabled() {
		body := codeServerInstall
		for _, ext := range cfg.GetCodeServerExtensions() {
			body += " \\\n    && code-server --install-extension " + ext + " --force"
		}
		steps = append(steps, updateStep{
			Name:        "code-server",
			Description: "code-server and its extensions",
			Body:        body,
		})
	}
	if cfg.GetShell() == "zsh" {
		steps = append(steps, updateStep{
			Name:        "oh-my-zsh",
			Description: "Oh My Zsh",
			Body:        `git -C "$HOME/.oh-my-zsh" pull --ff-only --quiet`,
		})
	}
	return steps
}

// miseUpdateBody upgrades mise itself and every runtime it manages. mise
// upgrade stays within each runtime's requested version, so a runtime pinned
// to 1.22 only moves within 1.22.x.
func miseUpdateBody(cfg *config.Config) string {
	body := `"$MISE" self-update --yes && "$MISE" upgrade`
	if cfg.HasLanguage("python") {
		body += " \\\n    && " + pythonLibFix
	}
	return body
}

// sdkmanUpdateBody refreshes SDKMAN and upgrades the Java build systems that
// are not pinned to a version. Java itself is always pinned (see
// installJavaWithSDKMAN), so it is left alone.
func sdkmanUpdateBody(cfg *config.Config) string {
	body := `source "$HOME/.sdkman/bin/sdkman-init.sh" \
    && sdkman_auto_answer=true \
    && sdk selfupdate \
    && sdk update`
	unpinned := languageBuildSystems(cfg, "java", func(bs, version string) string {
		if version != "" {
			return ""
		}
		return bs
	})
	for _, bs := range unpinned {
		body += " \\\n    && sdk upgrade " + bs
	}
	return body
}

// npmPackages lists the global npm packages to reinstall: the AI agents plus
// whatever the config's features and Node build systems need.
func npmPackages(cfg *config.Config) []string {
	packages := strings.Fields(agentNPMPackages)
	if cfg.IsMarkdownServerEnabled() {
		packages = append(packages, markdownNPMPackage)
	}
	return append(packages, languageBuildSystems(cfg, "node", func(bs, _ string) string {
		return nodeBuildSystemPackage(bs)
	})...)
}

// languageBuildSystems maps a language's configured build systems through
// toPackage, dropping empty results, in a stable order.
func languageBuildSystems(cfg *config.Config, lang string, toPackage func(bs, version string) string) []string {
	langCfg, ok := cfg.Languages[lang]
	if !ok {
		return nil
	}
	buildSystems := langCfg.GetBuildSystems()
	names := make([]string, 0, len(buildSystems))
	for bs := range buildSystems {
		names = append(names, bs)
	}
	sort.Strings(names)

	var packages []string
	for _, bs := range names {
		if pkg := toPackage(bs, buildSystems[bs]); pkg != "" {
			packages = append(packages, pkg)
		}
	}
	return packages
}
