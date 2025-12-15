package cmd

import (
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start and enter the rig container",
	Long: `Starts the rig container and attaches to it with the configured shell.

If the container doesn't exist, it will be created from the .rig.yml configuration.
If the configuration has changed, the container will be rebuilt.
If the container is stopped, it will be started.
If the container is already running, it will attach to it.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSession(nil) // Uses configured shell from .rig.yml
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
}
