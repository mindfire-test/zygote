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
	redactPolicy   string
	redactOut      string
	redactSign     bool
	redactKey      string
	redactCompress bool
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

		var privKey []byte
		if redactSign || redactKey != "" {
			if redactKey == "" {
				fmt.Fprintln(os.Stderr, "Error: --key <private-key-file> is required when --sign is specified")
				os.Exit(3)
			}
			pk, err := trace.LoadPrivateKey(redactKey)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading private key: %v\n", err)
				os.Exit(3)
			}
			privKey = pk
		}

		opts := trace.ExportOptions{
			PrivateKey: privKey,
			Compress:   redactCompress,
		}
		if err := trace.ExportBundle(redactOut, cleanBundle.Store, cleanBundle.Recording, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting redacted bundle: %v\n", err)
			os.Exit(3)
		}

		fmt.Printf("Successfully redacted %s into %s\n", inPath, redactOut)
	},
}

func init() {
	redactCmd.Flags().StringVar(&redactPolicy, "policy", ".zygote/redact.yaml", "Path to redaction policy")
	redactCmd.Flags().StringVarP(&redactOut, "out", "o", "clean.zip", "Output bundle file")
	redactCmd.Flags().BoolVar(&redactSign, "sign", false, "Cryptographically sign bundle with an Ed25519 private key")
	redactCmd.Flags().StringVar(&redactKey, "key", "", "Path to Ed25519 private key file")
	redactCmd.Flags().BoolVar(&redactCompress, "compress", false, "Enable Zstandard (zstd) compression for bundle objects (FR-4.10)")
	rootCmd.AddCommand(redactCmd)
}
