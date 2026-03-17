package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/out"
)

func newSendFileCmd(flags *rootFlags) *cobra.Command {
	var to string
	var filePath string
	var filename string
	var caption string
	var mimeOverride string
	var ptt bool

	cmd := &cobra.Command{
		Use:   "file",
		Short: "Send a file (image/video/audio/document)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" || filePath == "" {
				return fmt.Errorf("--to and --file are required")
			}

			if data, err := tryDaemonCall(flags, "send.file", map[string]any{
				"to": to, "file_path": filePath, "filename": filename, "caption": caption, "mime": mimeOverride, "ptt": ptt,
			}); err != nil {
				return err
			} else if data != nil {
				return outputIPCResult(flags, data, fmt.Sprintf("Sent %s to %s (id %s)\n", ipcFileName(data), data["to"], data["id"]))
			}

			ctx, cancel, a, lk, toJID, err := prepareSend(flags, to)
			if err != nil {
				return err
			}
			defer cancel()
			defer closeApp(a, lk)

			msgID, meta, err := sendFile(ctx, a, toJID, filePath, filename, caption, mimeOverride, ptt)
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
			fmt.Fprintf(os.Stdout, "Sent %s to %s (id %s)\n", meta["name"], toJID.String(), msgID)
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "recipient phone number or JID")
	cmd.Flags().StringVar(&filePath, "file", "", "path to file")
	cmd.Flags().StringVar(&filename, "filename", "", "display name for the file (defaults to basename of --file)")
	cmd.Flags().StringVar(&caption, "caption", "", "caption (images/videos/documents)")
	cmd.Flags().StringVar(&mimeOverride, "mime", "", "override detected mime type")
	cmd.Flags().BoolVar(&ptt, "ptt", false, "send as voice note (Push-To-Talk)")
	return cmd
}
