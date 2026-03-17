package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/out"
	"github.com/steipete/wacli/internal/wa"
	"go.mau.fi/whatsmeow/types"
)

func newPresenceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "presence",
		Short: "Send presence indicators (typing, paused)",
	}
	cmd.AddCommand(newPresenceTypingCmd(flags))
	cmd.AddCommand(newPresencePausedCmd(flags))
	return cmd
}

func newPresenceTypingCmd(flags *rootFlags) *cobra.Command {
	var to string
	var media string

	cmd := &cobra.Command{
		Use:   "typing",
		Short: "Send a 'composing' (typing) indicator to a chat",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPresence(flags, to, types.ChatPresenceComposing, media)
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "recipient phone number or JID")
	cmd.Flags().StringVar(&media, "media", "", "media type: 'audio' for recording indicator (default: typing text)")
	return cmd
}

func newPresencePausedCmd(flags *rootFlags) *cobra.Command {
	var to string

	cmd := &cobra.Command{
		Use:   "paused",
		Short: "Send a 'paused' indicator (stop typing) to a chat",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPresence(flags, to, types.ChatPresencePaused, "")
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "recipient phone number or JID")
	return cmd
}

func runPresence(flags *rootFlags, to string, state types.ChatPresence, media string) error {
	if to == "" {
		return fmt.Errorf("--to is required")
	}

	ipcCmd := "presence.paused"
	ipcParams := map[string]any{"to": to}
	if state == types.ChatPresenceComposing {
		ipcCmd = "presence.typing"
		ipcParams["media"] = media
	}
	if data, err := tryDaemonCall(flags, ipcCmd, ipcParams); err != nil {
		return err
	} else if data != nil {
		return outputIPCResult(flags, data, fmt.Sprintf("Presence '%s' sent to %s\n", state, data["to"]))
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

	toJID, err := wa.ParseUserOrJID(to)
	if err != nil {
		return err
	}

	var chatMedia types.ChatPresenceMedia
	if strings.EqualFold(media, "audio") {
		chatMedia = types.ChatPresenceMediaAudio
	}

	if err := a.WA().SendChatPresence(ctx, toJID, state, chatMedia); err != nil {
		return err
	}

	if flags.asJSON {
		return out.WriteJSON(os.Stdout, map[string]any{
			"sent":  true,
			"to":    toJID.String(),
			"state": string(state),
		})
	}
	fmt.Fprintf(os.Stdout, "Presence '%s' sent to %s\n", state, toJID.String())
	return nil
}
