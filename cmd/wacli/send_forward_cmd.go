package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/out"
	"github.com/steipete/wacli/internal/store"
	"github.com/steipete/wacli/internal/wa"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

func newSendForwardCmd(flags *rootFlags) *cobra.Command {
	var to string
	var fromChat string
	var id string

	cmd := &cobra.Command{
		Use:   "forward",
		Short: "Forward a message to another chat",
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" || fromChat == "" || id == "" {
				return fmt.Errorf("--to, --from-chat, and --id are required")
			}

			if data, err := tryDaemonCall(flags, "send.forward", map[string]any{
				"to": to, "from_chat": fromChat, "id": id,
			}); err != nil {
				return err
			} else if data != nil {
				return outputIPCResult(flags, data, fmt.Sprintf("Forwarded message to %s (id %s)\n", data["to"], data["id"]))
			}

			ctx, cancel, a, lk, toJID, err := prepareSend(flags, to)
			if err != nil {
				return err
			}
			defer cancel()
			defer closeApp(a, lk)

			// Parse from-chat JID for validation.
			_, err = wa.ParseUserOrJID(fromChat)
			if err != nil {
				return fmt.Errorf("invalid --from-chat: %w", err)
			}

			m, err := a.DB().GetMessage(fromChat, id)
			if err != nil {
				if err == sql.ErrNoRows {
					return fmt.Errorf("message not found in local DB (chat=%s id=%s)", fromChat, id)
				}
				return err
			}

			// Build forwarded message proto. Currently supports text-only forwarding.
			text := m.Text
			if text == "" {
				text = m.DisplayText
			}
			if text == "" {
				return fmt.Errorf("only text messages can be forwarded (message has no text content)")
			}

			isForwarded := true
			msg := &waProto.Message{
				ExtendedTextMessage: &waProto.ExtendedTextMessage{
					Text: proto.String(text),
					ContextInfo: &waProto.ContextInfo{
						IsForwarded: &isForwarded,
					},
				},
			}

			resp, err := a.WA().SendProtoMessage(ctx, toJID, msg)
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			chatName := a.WA().ResolveChatName(ctx, toJID, "")
			kind := chatKindFromJID(toJID)
			warnOnErr(a.DB().UpsertChat(toJID.String(), kind, chatName, now), "persist chat")
			warnOnErr(a.DB().UpsertMessage(store.UpsertMessageParams{
				ChatJID:     toJID.String(),
				ChatName:    chatName,
				MsgID:       string(resp),
				SenderName:  "me",
				Timestamp:   now,
				FromMe:      true,
				Text:        text,
				DisplayText: "Forwarded: " + text,
			}), "persist message")

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"forwarded":  true,
					"to":         toJID.String(),
					"id":         resp,
					"from_chat":  fromChat,
					"from_id":    id,
				})
			}
			fmt.Fprintf(os.Stdout, "Forwarded message to %s (id %s)\n", toJID.String(), resp)
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "recipient phone number or JID")
	cmd.Flags().StringVar(&fromChat, "from-chat", "", "source chat JID")
	cmd.Flags().StringVar(&id, "id", "", "message ID to forward")
	return cmd
}
