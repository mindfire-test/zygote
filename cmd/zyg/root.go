// Package main provides the CLI for Zygote.
package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "zyg",
	Short: "zygote — a flight recorder for AI agent runs",
}

func init() {
	// Global flags can be added here
}
