// Package main provides the CLI for Zygote.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	replayAgent string
	replayJSON  bool
)

var replayCmd = &cobra.Command{
	Use:   "replay [run.zip]",
	Short: "Replay an agent run from a bundle",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Fprintln(os.Stderr, "Not implemented: requires M1 Harness protocol")
		os.Exit(3)
	},
}

func init() {
	replayCmd.Flags().StringVar(&replayAgent, "agent", "", "Agent command to run")
	replayCmd.Flags().BoolVar(&replayJSON, "json", false, "Output machine-readable JSON")
	rootCmd.AddCommand(replayCmd)
}
