package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/out"
)

func newSendVoiceCmd(flags *rootFlags) *cobra.Command {
	var to string
	var filePath string

	cmd := &cobra.Command{
		Use:   "voice",
		Short: "Send a voice note (Push-To-Talk)",
		Long:  "Send an audio file as a WhatsApp voice note (PTT). This is a shortcut for 'send file --ptt'.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" || filePath == "" {
				return fmt.Errorf("--to and --file are required")
			}

			if data, err := tryDaemonCall(flags, "send.voice", map[string]any{
				"to": to, "file_path": filePath,
			}); err != nil {
				return err
			} else if data != nil {
				return outputIPCResult(flags, data, fmt.Sprintf("Sent voice note to %s (id %s)\n", data["to"], data["id"]))
			}

			ctx, cancel, a, lk, toJID, err := prepareSend(flags, to)
			if err != nil {
				return err
			}
			defer cancel()
			defer closeApp(a, lk)

			msgID, meta, err := sendFile(ctx, a, toJID, filePath, "", "", "", true)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"sent": true,
					"to":   toJID.String(),
					"id":   msgID,
					"file": meta,
				})
			}
			fmt.Fprintf(os.Stdout, "Sent voice note to %s (id %s)\n", toJID.String(), msgID)
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "recipient phone number or JID")
	cmd.Flags().StringVar(&filePath, "file", "", "path to audio file")
	return cmd
}
