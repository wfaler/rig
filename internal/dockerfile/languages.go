package dockerfile

import (
	"fmt"

	"github.com/wfaler/rig/internal/config"
)

// GenerateLanguageInstall returns the Dockerfile RUN commands for installing a language
// Uses Mise for Go, Node, Python, Ruby, Rust
// Uses SDKMAN for Java
func GenerateLanguageInstall(lang string, cfg config.LanguageConfig) string {
	version := cfg.GetVersion()

	switch lang {
	case "go":
		return installWithMise("go", version)
	case "node":
		return installWithMise("node", version)
	case "python":
		return installWithMise("python", version)
	case "rust":
		return installWithMise("rust", version)
	case "ruby":
		return installWithMise("ruby", version)
	case "java":
		return installJavaWithSDKMAN(version)
	default:
		return fmt.Sprintf("# Unknown language: %s", lang)
	}
}

// GenerateBuildSystemInstall returns the Dockerfile RUN commands for installing a build system
// Deprecated: Use GenerateBuildSystemsInstall for multiple build systems support
func GenerateBuildSystemInstall(lang string, cfg config.LanguageConfig) string {
	return GenerateBuildSystemsInstall(lang, cfg)
}

// GenerateBuildSystemsInstall returns the Dockerfile RUN commands for installing all configured build systems
func GenerateBuildSystemsInstall(lang string, cfg config.LanguageConfig) string {
	buildSystems := cfg.GetBuildSystems()
	if len(buildSystems) == 0 {
		return ""
	}

	var result string
	for bs, version := range buildSystems {
		install := generateSingleBuildSystemInstall(lang, bs, version)
		if install != "" {
			if result != "" {
				result += "\n"
			}
			result += install
		}
	}
	return result
}

// generateSingleBuildSystemInstall returns the Dockerfile RUN command for a single build system
func generateSingleBuildSystemInstall(lang, buildSystem, version string) string {
	switch lang {
	case "node":
		return installNodeBuildSystem(buildSystem)
	case "python":
		return installPythonBuildSystem(buildSystem, version)
	case "java":
		return installJavaBuildSystemWithSDKMAN(buildSystem, version)
	case "ruby":
		return installRubyBuildSystem(buildSystem)
	default:
		return ""
	}
}

// installWithMise generates a Mise install command for a language
func installWithMise(lang, version string) string {
	// Map version aliases
	miseVersion := version
	if version == "latest" {
		miseVersion = "latest"
	} else if version == "lts" {
		// Mise uses "lts" for Node, but for others we use latest
		if lang == "node" {
			miseVersion = "lts"
		} else {
			miseVersion = "latest"
		}
	}

	return fmt.Sprintf(`# Install %s via Mise
RUN mise use --global %s@%s`, lang, lang, miseVersion)
}

// installJavaWithSDKMAN installs Java using SDKMAN
func installJavaWithSDKMAN(version string) string {
	// SDKMAN Java version format: version-distribution
	// Default to Temurin (Eclipse Adoptium) distribution
	sdkmanVersion := "21-tem" // Default to Java 21 Temurin

	if version != "" && version != "latest" && version != "lts" {
		// If user specified just a number like "21", use Temurin
		// If they specified full version like "21.0.2-tem", use as-is
		if len(version) <= 2 || (len(version) > 2 && version[2] != '.') {
			sdkmanVersion = version + "-tem"
		} else {
			sdkmanVersion = version
		}
	}

	return fmt.Sprintf(`# Install Java via SDKMAN
RUN bash -c "source ~/.sdkman/bin/sdkman-init.sh && sdk install java %s"`, sdkmanVersion)
}

// Build system installers

func installNodeBuildSystem(bs string) string {
	switch bs {
	case "yarn":
		return `# Install Yarn
RUN eval "$(~/.local/bin/mise activate bash)" && npm install -g yarn`
	case "pnpm":
		return `# Install pnpm
RUN eval "$(~/.local/bin/mise activate bash)" && npm install -g pnpm`
	case "npm":
		return "" // npm comes with Node
	default:
		return ""
	}
}

func installPythonBuildSystem(bs, version string) string {
	switch bs {
	case "poetry":
		if version != "" {
			return fmt.Sprintf(`# Install Poetry %s
RUN eval "$(~/.local/bin/mise activate bash)" && pip install poetry==%s`, version, version)
		}
		return `# Install Poetry
RUN eval "$(~/.local/bin/mise activate bash)" && pip install poetry`
	case "pipenv":
		return `# Install Pipenv
RUN eval "$(~/.local/bin/mise activate bash)" && pip install pipenv`
	case "pip":
		return "" // pip comes with Python
	default:
		return ""
	}
}

func installJavaBuildSystemWithSDKMAN(bs, version string) string {
	switch bs {
	case "gradle":
		gradleVersion := ""
		if version != "" {
			gradleVersion = version
		}
		if gradleVersion != "" {
			return fmt.Sprintf(`# Install Gradle %s via SDKMAN
RUN bash -c "source ~/.sdkman/bin/sdkman-init.sh && sdk install gradle %s"`, gradleVersion, gradleVersion)
		}
		return `# Install Gradle via SDKMAN
RUN bash -c "source ~/.sdkman/bin/sdkman-init.sh && sdk install gradle"`
	case "maven":
		if version != "" {
			return fmt.Sprintf(`# Install Maven %s via SDKMAN
RUN bash -c "source ~/.sdkman/bin/sdkman-init.sh && sdk install maven %s"`, version, version)
		}
		return `# Install Maven via SDKMAN
RUN bash -c "source ~/.sdkman/bin/sdkman-init.sh && sdk install maven"`
	case "sbt":
		if version != "" {
			return fmt.Sprintf(`# Install SBT %s via SDKMAN
RUN bash -c "source ~/.sdkman/bin/sdkman-init.sh && sdk install sbt %s"`, version, version)
		}
		return `# Install SBT via SDKMAN
RUN bash -c "source ~/.sdkman/bin/sdkman-init.sh && sdk install sbt"`
	case "ant":
		if version != "" {
			return fmt.Sprintf(`# Install Ant %s via SDKMAN
RUN bash -c "source ~/.sdkman/bin/sdkman-init.sh && sdk install ant %s"`, version, version)
		}
		return `# Install Ant via SDKMAN
RUN bash -c "source ~/.sdkman/bin/sdkman-init.sh && sdk install ant"`
	default:
		return ""
	}
}

func installRubyBuildSystem(bs string) string {
	switch bs {
	case "bundler":
		return `# Install Bundler
RUN eval "$(~/.local/bin/mise activate bash)" && gem install bundler`
	case "gem":
		return "" // gem comes with Ruby
	default:
		return ""
	}
}

// VSCodeExtensionsForLanguage maps languages to their recommended VS Code extensions
var VSCodeExtensionsForLanguage = map[string][]string{
	"go": {
		"golang.go", // Official Go extension
	},
	"node": {
		"dbaeumer.vscode-eslint",           // ESLint
		"esbenp.prettier-vscode",           // Prettier
		"ms-vscode.vscode-typescript-next", // TypeScript
	},
	"python": {
		"ms-python.python",         // Official Python extension
		"ms-python.vscode-pylance", // Pylance language server
		"ms-python.debugpy",        // Python debugger
	},
	"java": {
		"redhat.java",                    // Language Support for Java
		"vscjava.vscode-java-debug",      // Debugger for Java
		"vscjava.vscode-java-dependency", // Project Manager for Java
		"vscjava.vscode-maven",           // Maven support
		"vscjava.vscode-gradle",          // Gradle support
	},
	"rust": {
		"rust-lang.rust-analyzer", // Rust Analyzer
	},
	"ruby": {
		"shopify.ruby-lsp", // Ruby LSP
	},
}

// GetExtensionsForLanguages returns a deduplicated list of VS Code extensions for the given languages
func GetExtensionsForLanguages(languages []string) []string {
	extensionSet := make(map[string]bool)
	var extensions []string

	for _, lang := range languages {
		if exts, ok := VSCodeExtensionsForLanguage[lang]; ok {
			for _, ext := range exts {
				if !extensionSet[ext] {
					extensionSet[ext] = true
					extensions = append(extensions, ext)
				}
			}
		}
	}

	return extensions
}
