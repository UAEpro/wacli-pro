package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/out"
	"github.com/steipete/wacli/internal/store"
)

func newSendLocationCmd(flags *rootFlags) *cobra.Command {
	var to string
	var lat, lng float64
	var name, address string

	cmd := &cobra.Command{
		Use:   "location",
		Short: "Send a location message",
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" {
				return fmt.Errorf("--to is required")
			}
			if lat == 0 && lng == 0 {
				return fmt.Errorf("--lat and --lng are required")
			}

			if data, err := tryDaemonCall(flags, "send.location", map[string]any{
				"to": to, "lat": lat, "lng": lng, "name": name, "address": address,
			}); err != nil {
				return err
			} else if data != nil {
				return outputIPCResult(flags, data, fmt.Sprintf("Sent location to %s (id %s)\n", data["to"], data["id"]))
			}

			ctx, cancel, a, lk, toJID, err := prepareSend(flags, to)
			if err != nil {
				return err
			}
			defer cancel()
			defer closeApp(a, lk)

			msgID, err := a.WA().SendLocation(ctx, toJID, lat, lng, name, address)
			if err != nil {
				return err
			}

			now := time.Now().UTC()
			chatName := a.WA().ResolveChatName(ctx, toJID, "")
			kind := chatKindFromJID(toJID)
			warnOnErr(a.DB().UpsertChat(toJID.String(), kind, chatName, now), "persist chat")
			displayText := fmt.Sprintf("Location: %.6f, %.6f", lat, lng)
			if name != "" {
				displayText = fmt.Sprintf("Location: %s (%.6f, %.6f)", name, lat, lng)
			}
			warnOnErr(a.DB().UpsertMessage(store.UpsertMessageParams{
				ChatJID:     toJID.String(),
				ChatName:    chatName,
				MsgID:       string(msgID),
				SenderName:  "me",
				Timestamp:   now,
				FromMe:      true,
				Text:        displayText,
				DisplayText: displayText,
				MediaType:   "location",
			}), "persist message")

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"sent": true,
					"to":   toJID.String(),
					"id":   msgID,
					"lat":  lat,
					"lng":  lng,
				})
			}
			fmt.Fprintf(os.Stdout, "Sent location to %s (id %s)\n", toJID.String(), msgID)
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "recipient phone number or JID")
	cmd.Flags().Float64Var(&lat, "lat", 0, "latitude")
	cmd.Flags().Float64Var(&lng, "lng", 0, "longitude")
	cmd.Flags().StringVar(&name, "name", "", "location name")
	cmd.Flags().StringVar(&address, "address", "", "location address")
	return cmd
}
