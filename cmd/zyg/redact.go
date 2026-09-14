package main

import (
	"fmt"
	"os"

	"github.com/mindfire/zygote/pkg/redact"
	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
	"github.com/spf13/cobra"
)

var (
	redactPolicy string
	redactOut    string
)

var redactCmd = &cobra.Command{
	Use:   "redact [bundle.zip]",
	Short: "Post-process and redact an existing bundle",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		inPath := args[0]

		store := vfs.NewMemStore()
		bundle, err := trace.ImportFile(inPath, store)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading bundle: %v\n", err)
			os.Exit(2) // Invalid bundle
		}

		policy, err := redact.LoadPolicy(redactPolicy)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading policy: %v\n", err)
			os.Exit(3)
		}

		cleanBundle, err := trace.RedactBundle(&bundle, policy)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Redaction failed: %v\n", err)
			os.Exit(3)
		}

		if err := trace.ExportFile(redactOut, cleanBundle.Store, cleanBundle.Recording); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting redacted bundle: %v\n", err)
			os.Exit(3)
		}

		fmt.Printf("Successfully redacted %s into %s\n", inPath, redactOut)
	},
}

func init() {
	redactCmd.Flags().StringVar(&redactPolicy, "policy", ".zygote/redact.yaml", "Path to redaction policy")
	redactCmd.Flags().StringVarP(&redactOut, "out", "o", "clean.zip", "Output bundle file")
	rootCmd.AddCommand(redactCmd)
}
