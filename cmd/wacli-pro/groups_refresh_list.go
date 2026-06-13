package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/UAEpro/wacli-pro/internal/app"
	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/spf13/cobra"
)

func newGroupsRefreshCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Fetch joined groups (live) and update local DB",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := runLiveOrDelegate(flags, "groups.refresh", map[string]any{},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupRefresh(ctx, a)
				})
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, data)
			}
			fmt.Fprintf(os.Stdout, "Imported %v groups.\n", data["groups"])
			return nil
		},
	}
	return cmd
}

func newGroupsListCmd(flags *rootFlags) *cobra.Command {
	var query string
	var limit int
	var all bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List joined groups (from local DB; run sync to populate)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			gs, err := a.DB().ListGroups(query, limit, all)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, gs)
			}

			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tJID\tCREATED")
			for _, g := range gs {
				name := g.Name
				if name == "" {
					name = g.JID
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", truncate(name, 40), g.JID, g.CreatedAt.Local().Format("2006-01-02"))
			}
			_ = w.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "search query")
	cmd.Flags().IntVar(&limit, "limit", 50, "limit")
	cmd.Flags().BoolVar(&all, "all", false, "include groups you have left")
	return cmd
}
