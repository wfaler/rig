package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "devbox",
	Short: "Create dockerized development sandboxes for AI agents",
	Long: `Devbox creates isolated Docker containers configured with language
runtimes, build tools, and AI agent CLIs (Claude, Gemini, Codex, GitHub CLI).

Containers are persistent between sessions and automatically rebuild
when configuration changes.

Usage:
  devbox           Enter the container with bash
  devbox init      Initialize a new workspace with .assistant.yml
  devbox rebuild   Force a clean rebuild of the image`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSession(nil) // Uses configured shell from .assistant.yml
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
}
