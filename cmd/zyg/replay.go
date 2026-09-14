// Package main provides the CLI for Zygote.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mindfire/zygote/harness"
	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
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
	Run: func(_ *cobra.Command, args []string) {
		if replayAgent == "" {
			fmt.Fprintln(os.Stderr, "Error: --agent is required")
			os.Exit(3)
		}

		bundlePath := args[0]
		store := vfs.NewMemStore()

		bundle, err := trace.ImportFile(bundlePath, store)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error importing bundle: %v\n", err)
			os.Exit(2) // bundle invalid
		}

		r, err := trace.Replay(store, &bundle)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing replay: %v\n", err)
			os.Exit(2)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cmd := exec.CommandContext(ctx, "sh", "-c", replayAgent) //nolint:gosec

		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating stdout pipe: %v\n", err)
			os.Exit(3)
		}

		stdinPipe, err := cmd.StdinPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating stdin pipe: %v\n", err)
			os.Exit(3)
		}

		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting agent: %v\n", err)
			os.Exit(3)
		}

		srv := harness.NewServer(stdoutPipe, stdinPipe, harness.NewReplayHandler(r))

		srvErr := srv.Serve()
		cancel()
		cmdErr := cmd.Wait()

		if srvErr != nil {
			// Check if it's a divergence error
			if strings.Contains(srvErr.Error(), "divergence") {
				if replayJSON {
					fmt.Printf("{\"status\": \"diverged\", \"error\": %q}\n", srvErr.Error())
				} else {
					fmt.Fprintf(os.Stderr, "[!] DIVERGENCE DETECTED: %v\n", srvErr)
				}
				os.Exit(1) // Divergence
			}
			fmt.Fprintf(os.Stderr, "Harness protocol error: %v\n", srvErr)
			os.Exit(3)
		}

		if cmdErr != nil {
			fmt.Fprintf(os.Stderr, "Agent exited with error: %v\n", cmdErr)
			os.Exit(3)
		}

		if replayJSON {
			fmt.Println(`{"status": "match"}`)
		} else {
			fmt.Println("Replay matched all steps perfectly.")
		}
	},
}

func init() {
	replayCmd.Flags().StringVar(&replayAgent, "agent", "", "Agent command to run")
	replayCmd.Flags().BoolVar(&replayJSON, "json", false, "Output machine-readable JSON")
	rootCmd.AddCommand(replayCmd)
}
