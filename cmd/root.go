package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rig",
	Short: "Create dockerized development sandboxes for AI agents",
	Long: `Rig creates isolated Docker containers configured with language
runtimes, build tools, and AI agent CLIs (Claude, Gemini, Codex, GitHub CLI).

Containers are persistent between sessions and automatically rebuild
when configuration changes.

Commands:
  rig up        Enter the container (uses configured shell)
  rig down      Stop the container (preserves state)
  rig destroy   Stop container and remove images
  rig list      List running rig containers
  rig init      Initialize a new workspace with .rig.yml
  rig rebuild   Force a clean rebuild of the image`,
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
