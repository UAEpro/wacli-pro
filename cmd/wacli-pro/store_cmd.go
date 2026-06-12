package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/spf13/cobra"
)

func newStoreCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store",
		Short: "Local store management",
	}
	cmd.AddCommand(newStoreStatsCmd(flags))
	return cmd
}

func newStoreStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show store statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, true)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)
			_ = ctx

			stats, err := a.DB().Stats()
			if err != nil {
				return err
			}

			// Get DB file size
			dbPath := filepath.Join(a.StoreDir(), "wacli.db")
			var dbSize int64
			if fi, err := os.Stat(dbPath); err == nil {
				dbSize = fi.Size()
			}

			result := map[string]any{
				"store_dir":     a.StoreDir(),
				"db_size_bytes": dbSize,
				"messages":      stats.Messages,
				"chats":         stats.Chats,
				"contacts":      stats.Contacts,
				"groups":        stats.Groups,
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, result)
			}

			fmt.Fprintf(os.Stdout, "Store: %s\n", a.StoreDir())
			fmt.Fprintf(os.Stdout, "DB size: %s\n", formatBytes(dbSize))
			fmt.Fprintf(os.Stdout, "Messages: %d\n", stats.Messages)
			fmt.Fprintf(os.Stdout, "Chats: %d\n", stats.Chats)
			fmt.Fprintf(os.Stdout, "Contacts: %d\n", stats.Contacts)
			fmt.Fprintf(os.Stdout, "Groups: %d\n", stats.Groups)
			return nil
		},
	}
	return cmd
}

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
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
