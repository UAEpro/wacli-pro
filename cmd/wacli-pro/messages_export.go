package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/UAEpro/wacli-pro/internal/store"
	"github.com/spf13/cobra"
)

func newMessagesExportCmd(flags *rootFlags) *cobra.Command {
	var chatJID string
	var limit int
	var afterStr string
	var beforeStr string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export messages as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(chatJID) == "" {
				return fmt.Errorf("--chat is required")
			}
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, true)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)
			_ = ctx

			var after *time.Time
			var before *time.Time
			if afterStr != "" {
				t, err := parseTime(afterStr)
				if err != nil {
					return fmt.Errorf("invalid --after: %w", err)
				}
				after = &t
			}
			if beforeStr != "" {
				t, err := parseTime(beforeStr)
				if err != nil {
					return fmt.Errorf("invalid --before: %w", err)
				}
				before = &t
			}

			effectiveLimit := limit
			if effectiveLimit <= 0 {
				effectiveLimit = 1000000 // effectively unlimited
			}

			msgs, err := a.DB().ListMessages(store.ListMessagesParams{
				ChatJID: chatJID,
				Limit:   effectiveLimit,
				After:   after,
				Before:  before,
			})
			if err != nil {
				return err
			}

			return out.WriteJSON(os.Stdout, msgs)
		},
	}
	cmd.Flags().StringVar(&chatJID, "chat", "", "chat JID (required)")
	cmd.Flags().IntVar(&limit, "limit", 0, "limit results (0 = all)")
	cmd.Flags().StringVar(&afterStr, "after", "", "only messages after time (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&beforeStr, "before", "", "only messages before time (RFC3339 or YYYY-MM-DD)")
	return cmd
}
