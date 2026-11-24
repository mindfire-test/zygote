// Package main provides the CLI for Zygote.
package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mindfire/zygote/vfs"
	"github.com/spf13/cobra"
)

var (
	gcStorePath    string
	gcKeep         []string
	gcDryRun       bool
	gcGracePeriod  time.Duration
)

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Garbage-collect unreferenced objects from the store",
	Run: func(cmd *cobra.Command, _ []string) {
		var keepSnaps []vfs.Snapshot

		for _, item := range gcKeep {
			for _, part := range strings.Split(item, ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				hb, err := hex.DecodeString(part)
				if err != nil || len(hb) != 32 {
					fmt.Fprintf(os.Stderr, "Error: invalid snapshot hash %q: expected 64-character hex BLAKE3 hash\n", part)
					os.Exit(3)
				}
				var h vfs.Hash
				copy(h[:], hb)
				keepSnaps = append(keepSnaps, vfs.Snapshot{Root: h})
			}
		}

		if len(keepSnaps) == 0 && !gcDryRun {
			fmt.Fprintln(os.Stderr, "Warning: no --keep snapshots specified; all objects older than grace period will be pruned.")
		}

		store, err := vfs.NewDiskStore(gcStorePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening store at %q: %v\n", gcStorePath, err)
			os.Exit(2)
		}

		opts := vfs.GCOptions{
			DryRun:      gcDryRun,
			GracePeriod: gcGracePeriod,
		}

		res, err := vfs.GC(cmd.Context(), store, keepSnaps, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Garbage collection failed: %v\n", err)
			os.Exit(2)
		}

		if gcDryRun {
			fmt.Println("[dry-run] Garbage collection summary:")
			fmt.Printf("  Store: %s\n", gcStorePath)
			fmt.Printf("  Reachable objects: %d\n", res.ReachableObjects)
			fmt.Printf("  Candidates for deletion: %d\n", res.CandidateObjects)
			fmt.Printf("  Reclaimable space: %s (%d bytes)\n", formatBytes(res.ReclaimedBytes), res.ReclaimedBytes)
			if len(res.Candidates) > 0 {
				fmt.Println("Candidates:")
				for _, c := range res.Candidates {
					fmt.Printf("  - %x (%s)\n", c.Hash, formatBytes(c.Size))
				}
			}
		} else {
			fmt.Println("Garbage collection complete:")
			fmt.Printf("  Store: %s\n", gcStorePath)
			fmt.Printf("  Reachable objects: %d\n", res.ReachableObjects)
			fmt.Printf("  Deleted objects: %d\n", res.DeletedObjects)
			fmt.Printf("  Reclaimed space: %s (%d bytes)\n", formatBytes(res.ReclaimedBytes), res.ReclaimedBytes)
		}
	},
}

func init() {
	gcCmd.Flags().StringVar(&gcStorePath, "store", ".zygote/store", "Path to on-disk object store directory")
	gcCmd.Flags().StringSliceVar(&gcKeep, "keep", nil, "Comma-separated list of snapshot root hashes to keep")
	gcCmd.Flags().BoolVar(&gcDryRun, "dry-run", false, "Show candidates for deletion and reclaimed bytes without mutating disk")
	gcCmd.Flags().DurationVar(&gcGracePeriod, "grace-period", vfs.DefaultGCGracePeriod, "Grace period window protecting recently written objects from deletion")
	rootCmd.AddCommand(gcCmd)
}
