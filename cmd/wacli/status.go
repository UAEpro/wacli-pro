package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/out"
	"github.com/steipete/wacli/internal/store"
	"go.mau.fi/whatsmeow/types"
)

func newStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Post to WhatsApp Status (stories)",
	}
	cmd.AddCommand(newStatusPostTextCmd(flags))
	cmd.AddCommand(newStatusPostFileCmd(flags))
	return cmd
}

func newStatusPostTextCmd(flags *rootFlags) *cobra.Command {
	var text string

	cmd := &cobra.Command{
		Use:   "text",
		Short: "Post a text status update",
		RunE: func(cmd *cobra.Command, args []string) error {
			if text == "" {
				return fmt.Errorf("--text is required")
			}

			if data, err := tryDaemonCall(flags, "status.text", map[string]any{
				"text": text,
			}); err != nil {
				return err
			} else if data != nil {
				return outputIPCResult(flags, data, fmt.Sprintf("Status posted (id %s)\n", data["id"]))
			}

			ctx, cancel, a, lk, _, err := prepareSend(flags, types.StatusBroadcastJID.String())
			if err != nil {
				return err
			}
			defer cancel()
			defer closeApp(a, lk)

			msgID, err := a.WA().SendText(ctx, types.StatusBroadcastJID, text)
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			warnOnErr(a.DB().UpsertChat(types.StatusBroadcastJID.String(), "broadcast", "My Status", now), "persist chat")
			warnOnErr(a.DB().UpsertMessage(store.UpsertMessageParams{
				ChatJID:     types.StatusBroadcastJID.String(),
				ChatName:    "My Status",
				MsgID:       string(msgID),
				SenderName:  "me",
				Timestamp:   now,
				FromMe:      true,
				Text:        text,
				DisplayText: text,
			}), "persist message")

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"posted": true,
					"id":     msgID,
					"type":   "text",
				})
			}
			fmt.Fprintf(os.Stdout, "Status posted (id %s)\n", msgID)
			return nil
		},
	}

	cmd.Flags().StringVar(&text, "text", "", "status text")
	return cmd
}

func newStatusPostFileCmd(flags *rootFlags) *cobra.Command {
	var filePath string
	var caption string
	var filename string
	var mimeOverride string

	cmd := &cobra.Command{
		Use:   "file",
		Short: "Post a file (image/video) as a status update",
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return fmt.Errorf("--file is required")
			}

			if data, err := tryDaemonCall(flags, "status.file", map[string]any{
				"file_path": filePath, "caption": caption, "filename": filename, "mime": mimeOverride,
			}); err != nil {
				return err
			} else if data != nil {
				return outputIPCResult(flags, data, fmt.Sprintf("Status posted: %s (id %s)\n", ipcFileName(data), data["id"]))
			}

			ctx, cancel, a, lk, _, err := prepareSend(flags, types.StatusBroadcastJID.String())
			if err != nil {
				return err
			}
			defer cancel()
			defer closeApp(a, lk)

			msgID, meta, err := sendFile(ctx, a, types.StatusBroadcastJID, filePath, filename, caption, mimeOverride, false)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"posted": true,
					"id":     msgID,
					"type":   "file",
					"file":   meta,
				})
			}
			fmt.Fprintf(os.Stdout, "Status posted: %s (id %s)\n", meta["name"], msgID)
			return nil
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "path to image/video file")
	cmd.Flags().StringVar(&caption, "caption", "", "caption for the status")
	cmd.Flags().StringVar(&filename, "filename", "", "display name override")
	cmd.Flags().StringVar(&mimeOverride, "mime", "", "override detected mime type")
	return cmd
}
