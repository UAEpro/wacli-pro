package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/UAEpro/wacli-pro/internal/app"
	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/spf13/cobra"
)

func newSendPollCmd(flags *rootFlags) *cobra.Command {
	var to string
	var question string
	var options []string
	var maxSelections int
	cmd := &cobra.Command{
		Use:   "poll",
		Short: "Send a poll",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(to) == "" {
				return fmt.Errorf("--to is required")
			}
			if strings.TrimSpace(question) == "" {
				return fmt.Errorf("--question is required")
			}
			if len(options) < 2 {
				return fmt.Errorf("at least 2 --option values are required")
			}
			data, err := runLiveOrDelegate(flags, "send.poll", map[string]any{
				"to": to, "question": question, "options": options, "max_selections": maxSelections,
			}, func(ctx context.Context, a *app.App) (map[string]any, error) {
				return opSendPoll(ctx, a, to, question, options, maxSelections)
			})
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, data)
			}
			fmt.Fprintf(os.Stdout, "Sent: %v\n", data["id"])
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "recipient phone number or JID")
	cmd.Flags().StringVar(&question, "question", "", "poll question")
	cmd.Flags().StringSliceVar(&options, "option", nil, "poll option (repeatable, min 2)")
	cmd.Flags().IntVar(&maxSelections, "max-selections", 1, "max selections allowed (0 = unlimited)")
	return cmd
}
