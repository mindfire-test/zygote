// Package main provides the CLI for Zygote.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/mindfire/zygote/harness"
	"github.com/mindfire/zygote/source/localdir"
	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
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
	Run: func(_ *cobra.Command, _ []string) {
		if recordAgent == "" {
			fmt.Fprintln(os.Stderr, "Error: --agent is required")
			os.Exit(3)
		}

		ctx := context.Background()
		store := vfs.NewMemStore()
		r := trace.Record(store)

		adapter := localdir.New(recordDir)
		if err := adapter.Load(ctx, r.World()); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading directory: %v\n", err)
			os.Exit(3)
		}
		r.Step("init")

		// shell is required because the agent command might be "python main.py"
		cmd := exec.CommandContext(ctx, "sh", "-c", recordAgent)

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

		srv := harness.NewServer(stdoutPipe, stdinPipe, harness.NewRecordHandler(r))

		srvErr := srv.Serve()

		cmdErr := cmd.Wait()

		if srvErr != nil {
			fmt.Fprintf(os.Stderr, "Harness protocol error: %v\n", srvErr)
			os.Exit(3)
		}

		if cmdErr != nil {
			fmt.Fprintf(os.Stderr, "Agent exited with error: %v\n", cmdErr)
			os.Exit(3) // Harness Error covers agent crash
		}

		if err := trace.ExportFile(recordOut, store, r.Recording()); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting bundle: %v\n", err)
			os.Exit(3)
		}

		fmt.Printf("Successfully recorded run to %s\n", recordOut)
	},
}

func init() {
	recordCmd.Flags().StringVar(&recordAgent, "agent", "", "Agent command to run")
	recordCmd.Flags().StringVar(&recordDir, "dir", ".", "Directory to record")
	recordCmd.Flags().StringVarP(&recordOut, "out", "o", "run.zip", "Output bundle file")
	rootCmd.AddCommand(recordCmd)
}
