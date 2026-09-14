// Package main provides the CLI for Zygote.
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/mindfire/zygote/source/localdir"
	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
	"github.com/spf13/cobra"
)

var forkInto string

var forkCmd = &cobra.Command{
	Use:   "fork [run.zip] [step]",
	Short: "Materialise a step to a directory",
	Args:  cobra.ExactArgs(2),
	Run: func(_ *cobra.Command, args []string) {
		bundlePath := args[0]
		stepArg := args[1]

		if forkInto == "" {
			fmt.Fprintln(os.Stderr, "Error: --into must be specified")
			os.Exit(1)
		}

		store := vfs.NewMemStore()
		bundle, err := trace.ImportFile(bundlePath, store)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error importing bundle: %v\n", err)
			os.Exit(2)
		}

		var targetStep *trace.Step

		// check if stepArg is an integer index
		if idx, err := strconv.Atoi(stepArg); err == nil && idx >= 0 && idx < len(bundle.Recording.Steps) {
			targetStep = &bundle.Recording.Steps[idx]
		} else {
			// fallback to name matching
			for i, s := range bundle.Recording.Steps {
				if s.Name == stepArg {
					targetStep = &bundle.Recording.Steps[i]
					break
				}
			}
		}

		if targetStep == nil {
			fmt.Fprintf(os.Stderr, "Error: step '%s' not found\n", stepArg)
			os.Exit(1)
		}

		hBytes, err := hex.DecodeString(targetStep.Root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid step hash\n")
			os.Exit(1)
		}

		var h vfs.Hash
		copy(h[:], hBytes)

		// Create the fork
		forkWorld := vfs.Fork(store, vfs.Snapshot{Root: h})

		// Ensure directory exists
		if err := os.MkdirAll(forkInto, 0755); err != nil { //nolint:gosec
			fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
			os.Exit(1)
		}

		// Apply the world to the directory
		adapter := localdir.New(forkInto)
		if err := adapter.Apply(context.Background(), forkWorld); err != nil {
			fmt.Fprintf(os.Stderr, "Error applying world: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully forked step '%s' into %s\n", targetStep.Name, forkInto)
	},
}

func init() {
	forkCmd.Flags().StringVar(&forkInto, "into", "", "Directory to materialise the world into")
	rootCmd.AddCommand(forkCmd)
}
