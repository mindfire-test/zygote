package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	recordAgent string
	recordDir   string
	recordOut   string
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record an agent run",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stderr, "Not implemented: requires M1 Harness protocol")
		os.Exit(3)
	},
}

func init() {
	recordCmd.Flags().StringVar(&recordAgent, "agent", "", "Agent command to run")
	recordCmd.Flags().StringVar(&recordDir, "dir", ".", "Directory to record")
	recordCmd.Flags().StringVarP(&recordOut, "out", "o", "run.zip", "Output bundle file")
	rootCmd.AddCommand(recordCmd)
}
