package dockerfile

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wfaler/rig/internal/config"
)

const (
	// neovimInstall installs Neovim through mise, so rig-update's mise step
	// upgrades it along with the language runtimes.
	neovimInstall = "mise use --global neovim@latest"

	// nvimInitScriptFile is the pre-initialisation helper's name in the build
	// context, and NvimInitScriptPath is where it is installed in the image.
	nvimInitScriptFile = "rig-nvim-init.lua"
	NvimInitScriptPath = "/usr/local/share/rig/rig-nvim-init.lua"

	// NvimConfigCheckout is where the Neovim config repository is cloned;
	// ~/.config/nvim links to the config directory within it.
	NvimConfigCheckout = "/home/developer/.local/share/rig/nvim-config"

	// NvimBinPath is the Neovim binary through mise's "latest" link, which
	// mise moves when it upgrades Neovim, so it stays valid after rig-update.
	NvimBinPath = "/home/developer/.local/share/mise/installs/neovim/latest/bin/nvim"

	// vimAliasDir holds the vi and vim wrappers. It comes before /usr/bin on
	// PATH, where Debian's Vim stays available.
	vimAliasDir = "/usr/local/bin"
)

// neovimAliases returns the command that makes vi and vim open Neovim: a vim
// wrapper in binDir that runs nvimPath, falling back to nvim on PATH, and vi
// linked to it. Wrappers, unlike shell aliases, also cover scripts, git's
// editor and sudo.
func neovimAliases(binDir, nvimPath string) string {
	lines := []string{
		"#!/bin/sh",
		"# Installed by rig because Neovim is enabled: vi and vim open Neovim.",
		"# Debian's Vim is still available as /usr/bin/vim.",
		"nvim=" + shellQuote(nvimPath),
		`[ -x "$nvim" ] || nvim=nvim`,
		`exec "$nvim" "$@"`,
	}
	quoted := make([]string, 0, len(lines))
	for _, line := range lines {
		quoted = append(quoted, shellQuote(line))
	}
	vim := binDir + "/vim"
	return "printf '%s\\n' " + strings.Join(quoted, " ") + " > " + vim +
		" \\\n    && chmod 755 " + vim +
		" \\\n    && ln -sfn vim " + binDir + "/vi"
}

// masonLanguages maps rig languages to the Mason registry languages whose
// tools a project in that language needs. Node covers the web languages its
// projects typically contain; Java covers the JVM languages rig builds with
// SDKMAN.
var masonLanguages = map[string][]string{
	"go":     {"Go"},
	"node":   {"JavaScript", "TypeScript", "HTML", "CSS", "Vue", "Svelte"},
	"python": {"Python"},
	"java":   {"Java", "Kotlin", "Scala"},
	"rust":   {"Rust"},
	"ruby":   {"Ruby"},
}

// MasonLanguages returns the Mason registry languages to install tools for:
// those of the configured languages, plus Lua for editing the Neovim config
// itself. The result is sorted and has no duplicates.
func MasonLanguages(cfg *config.Config) []string {
	seen := map[string]bool{"Lua": true}
	for lang := range cfg.Languages {
		for _, mason := range masonLanguages[lang] {
			seen[mason] = true
		}
	}
	langs := make([]string, 0, len(seen))
	for lang := range seen {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return langs
}

// neovimConfigClone returns the commands that clone the config repository and
// link ~/.config/nvim to it. A config in a subdirectory is fetched with a
// sparse checkout, so only that directory is downloaded.
func neovimConfigClone(repo *config.NeovimConfigRepo) string {
	args := []string{"git clone --depth 1"}
	if repo.Ref != "" {
		args = append(args, "--branch "+shellQuote(repo.Ref))
	}
	if repo.Path != "" {
		args = append(args, "--filter=blob:none --sparse")
	}
	args = append(args, shellQuote(repo.Repo), NvimConfigCheckout)

	cmds := []string{strings.Join(args, " ")}
	configDir := NvimConfigCheckout
	if repo.Path != "" {
		cmds = append(cmds, "git -C "+NvimConfigCheckout+" sparse-checkout set "+shellQuote(repo.Path))
		configDir += "/" + strings.TrimSuffix(repo.Path, "/")
	}
	cmds = append(cmds,
		fmt.Sprintf("test -d %s", shellQuote(configDir)),
		"mkdir -p /home/developer/.config",
		fmt.Sprintf("ln -sfn %s /home/developer/.config/nvim", shellQuote(configDir)),
	)
	return strings.Join(cmds, " \\\n    && ")
}

// neovimSetup returns the commands that pre-initialise the config: its
// plugins, the Mason tools for the project's languages, and its treesitter
// parsers, each in a fresh headless Neovim (see rig-nvim-init.lua). With
// update set, installed parsers are updated too.
func neovimSetup(cfg *config.Config, update bool) string {
	quoted := make([]string, 0)
	for _, lang := range MasonLanguages(cfg) {
		quoted = append(quoted, "'"+lang+"'")
	}
	parsers := "RigNvimParsers()"
	if update {
		parsers = "RigNvimParsers({ update = true })"
	}
	calls := []string{
		"RigNvimPlugins()",
		"RigNvimMason({" + strings.Join(quoted, ", ") + "})",
		parsers,
	}
	cmds := make([]string, 0, len(calls))
	for _, call := range calls {
		cmds = append(cmds, fmt.Sprintf(`"$HOME/.local/bin/mise" exec -- nvim --headless --cmd "luafile %s" -c "lua %s" -c qa`, NvimInitScriptPath, call))
	}
	return strings.Join(cmds, " \\\n    && ")
}

// neovimUpdateBody pulls the config repository and re-runs the
// pre-initialisation, which moves plugins to the pulled lazy-lock.json and
// upgrades the Mason tools and parsers.
func neovimUpdateBody(cfg *config.Config) string {
	return "git -C " + NvimConfigCheckout + " pull --ff-only --quiet \\\n    && " + neovimSetup(cfg, true)
}
