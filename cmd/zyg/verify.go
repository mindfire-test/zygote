// Package main provides the CLI for Zygote.
package main

import (
	"fmt"
	"os"

	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
	"github.com/spf13/cobra"
)

var (
	verifyKey string
)

var verifyCmd = &cobra.Command{
	Use:   "verify [run.zip]",
	Short: "Verify bundle cryptographic integrity and detached signature",
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		bundlePath := args[0]
		store := vfs.NewMemStore()

		var pubKey []byte
		if verifyKey != "" {
			k, err := trace.LoadPublicKey(verifyKey)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading public key: %v\n", err)
				os.Exit(2)
			}
			pubKey = k
		}

		bundle, err := trace.ImportVerifiedFile(bundlePath, store, pubKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Verification failed: %v\n", err)
			os.Exit(2)
		}

		if verifyKey != "" {
			fmt.Printf("Bundle verified successfully. Signature: valid (verified with %s)\n", verifyKey)
		} else if len(bundle.Signature) > 0 {
			fmt.Println("Bundle integrity valid. Signature: present (unverified, provide --verify-key to verify)")
		} else {
			fmt.Println("Bundle integrity valid. Signature: none")
		}
	},
}

func init() {
	verifyCmd.Flags().StringVar(&verifyKey, "verify-key", "", "Public key file to verify bundle signature")
	rootCmd.AddCommand(verifyCmd)
}
