package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/spf13/cobra"
)

func newCallsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calls",
		Short: "List call events",
	}
	cmd.AddCommand(newCallsListCmd(flags))
	return cmd
}

func newCallsListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent call events",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, true)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			calls, err := a.DB().ListCallEvents(limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, calls)
			}
			if len(calls) == 0 {
				fmt.Fprintln(os.Stdout, "No call events")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "TIME\tTYPE\tCALLER\tCALL ID")
			for _, c := range calls {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.Timestamp.Local().Format("2006-01-02 15:04"), c.Type, c.CallerJID, c.CallID)
			}
			_ = w.Flush()
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "limit results")
	return cmd
}
