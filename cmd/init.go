package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wfaler/devbox/internal/project"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new workspace with an empty .assistant.yml",
	Long: `Creates an empty .assistant.yml configuration file in the current directory.

Example:
  devbox init

This creates a template configuration file that you can edit to specify:
  - Programming languages and versions (Go, Node, Python, Java, Rust, Ruby)
  - Build systems (npm, yarn, gradle, poetry, etc.)
  - Port mappings
  - Environment variables`,
	RunE: runInit,
}

const emptyConfig = `# Devbox configuration
# See: https://github.com/wfaler/devbox for documentation

languages:
  # Example configurations:
  # node:
  #   version: "lts"           # "lts", "latest", or specific version like "20"
  #   build_system: npm        # npm, yarn, or pnpm
  # python:
  #   version: "3.12"
  #   build_system: poetry
  #   build_system_version: "1.7.0"
  # java:
  #   version: "21"
  #   build_system: gradle
  # go:
  #   version: "1.22"
  # rust:
  #   version: "latest"
  # ruby:
  #   version: "3.3"
  #   build_system: bundler

ports: []
  # Port mappings in "host:container" or "port" format:
  # - "8080:8080"
  # - "3000"

env: {}
  # Environment variables (supports ${VAR} expansion from host):
  # API_KEY: "${API_KEY}"
  # DATABASE_URL: "postgres://localhost:5432/dev"

# code_server: true
  # Install code-server (VS Code in browser) with language-specific extensions
  # Access at http://localhost:8080 (add "8080" to ports if needed)
`

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	configPath := project.ConfigPath(cwd)

	if project.ConfigExists(cwd) {
		return fmt.Errorf("%s already exists", project.ConfigFileName)
	}

	if err := os.WriteFile(configPath, []byte(emptyConfig), 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Printf("Created %s\n", project.ConfigFileName)
	fmt.Println("Edit this file to configure your development environment, then run:")
	fmt.Println("  devbox claude   # or gemini, codex, gh, bash")
	return nil
}
