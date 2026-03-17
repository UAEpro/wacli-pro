package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/out"
	"go.mau.fi/whatsmeow/types"
)

func newSendReactionCmd(flags *rootFlags) *cobra.Command {
	var to string
	var id string
	var emoji string

	cmd := &cobra.Command{
		Use:   "reaction",
		Short: "React to a message with an emoji (empty emoji removes reaction)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" || id == "" {
				return fmt.Errorf("--to and --id are required")
			}

			if data, err := tryDaemonCall(flags, "send.reaction", map[string]any{
				"to": to, "id": id, "emoji": emoji,
			}); err != nil {
				return err
			} else if data != nil {
				action := "Reacted"
				if emoji == "" {
					action = "Removed reaction from"
				}
				return outputIPCResult(flags, data, fmt.Sprintf("%s %s to message %s in %s\n", action, emoji, id, data["to"]))
			}

			ctx, cancel, a, lk, toJID, err := prepareSend(flags, to)
			if err != nil {
				return err
			}
			defer cancel()
			defer closeApp(a, lk)

			if err := a.WA().SendReaction(ctx, toJID, types.MessageID(id), emoji); err != nil {
				return err
			}

			action := "Reacted"
			if emoji == "" {
				action = "Removed reaction from"
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"reacted": true,
					"to":      toJID.String(),
					"id":      id,
					"emoji":   emoji,
				})
			}
			fmt.Fprintf(os.Stdout, "%s %s to message %s in %s\n", action, emoji, id, toJID.String())
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "chat JID or phone number")
	cmd.Flags().StringVar(&id, "id", "", "message ID to react to")
	cmd.Flags().StringVar(&emoji, "emoji", "", "emoji to react with (empty to remove)")
	return cmd
}
