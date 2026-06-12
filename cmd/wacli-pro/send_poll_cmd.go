package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/UAEpro/wacli-pro/internal/wa"
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
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, true, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			if err := a.EnsureAuthed(); err != nil {
				return err
			}
			if err := a.Connect(ctx, false, nil); err != nil {
				return err
			}

			jid, err := wa.ParseUserOrJID(to)
			if err != nil {
				return err
			}

			pollMsg := a.WA().BuildPollCreation(question, options, maxSelections)
			msgID, err := a.WA().SendProtoMessage(ctx, jid, pollMsg)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"id": string(msgID), "to": jid.String()})
			}
			fmt.Fprintf(os.Stdout, "Sent: %s\n", msgID)
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "recipient phone number or JID")
	cmd.Flags().StringVar(&question, "question", "", "poll question")
	cmd.Flags().StringSliceVar(&options, "option", nil, "poll option (repeatable, min 2)")
	cmd.Flags().IntVar(&maxSelections, "max-selections", 1, "max selections allowed (0 = unlimited)")
	return cmd
}
