package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	replayAgent string
	replayJson  bool
)

var replayCmd = &cobra.Command{
	Use:   "replay [run.zip]",
	Short: "Replay an agent run from a bundle",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stderr, "Not implemented: requires M1 Harness protocol")
		os.Exit(3)
	},
}

func init() {
	replayCmd.Flags().StringVar(&replayAgent, "agent", "", "Agent command to run")
	replayCmd.Flags().BoolVar(&replayJson, "json", false, "Output machine-readable JSON")
	rootCmd.AddCommand(replayCmd)
}
