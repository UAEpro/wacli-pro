package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/UAEpro/wacli-pro/internal/app"
	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/spf13/cobra"
)

func newChannelsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "Manage WhatsApp channels (newsletters)",
	}
	cmd.AddCommand(newChannelsListCmd(flags))
	cmd.AddCommand(newChannelsInfoCmd(flags))
	cmd.AddCommand(newChannelsFollowCmd(flags))
	cmd.AddCommand(newChannelsUnfollowCmd(flags))
	cmd.AddCommand(newChannelsMuteCmd(flags))
	cmd.AddCommand(newChannelsUnmuteCmd(flags))
	return cmd
}

func newChannelsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List subscribed channels",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := runLiveOrDelegate(flags, "channels.list", map[string]any{},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opChannelsList(ctx, a)
				})
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, data)
			}
			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tJID\tSUBSCRIBERS")
			if list, ok := data["channels"].([]any); ok {
				for _, item := range list {
					m, _ := item.(map[string]any)
					fmt.Fprintf(w, "%s\t%v\t%v\n", truncate(fmt.Sprintf("%v", m["name"]), 40), m["jid"], m["subscribers"])
				}
			}
			_ = w.Flush()
			return nil
		},
	}
	return cmd
}

func newChannelsInfoCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	var inviteLink string
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Get channel info",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" && strings.TrimSpace(inviteLink) == "" {
				return fmt.Errorf("--jid or --invite is required")
			}
			data, err := runLiveOrDelegate(flags, "channels.info", map[string]any{"jid": jidStr, "invite": inviteLink},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opChannelsInfo(ctx, a, jidStr, inviteLink)
				})
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, data)
			}
			fmt.Fprintf(os.Stdout, "JID: %v\nName: %v\nDescription: %v\n", data["jid"], data["name"], data["description"])
			if subs, _ := data["subscribers"].(float64); subs > 0 {
				fmt.Fprintf(os.Stdout, "Subscribers: %v\n", data["subscribers"])
			} else if subsInt, _ := data["subscribers"].(int); subsInt > 0 {
				fmt.Fprintf(os.Stdout, "Subscribers: %d\n", subsInt)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "channel JID (…@newsletter)")
	cmd.Flags().StringVar(&inviteLink, "invite", "", "channel invite link or code")
	return cmd
}

func newChannelsFollowCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "follow",
		Short: "Follow (join) a channel",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			data, err := runLiveOrDelegate(flags, "channels.follow", map[string]any{"jid": jidStr, "follow": true},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opChannelsFollow(ctx, a, jidStr, true)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "channel JID (…@newsletter)")
	return cmd
}

func newChannelsUnfollowCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "unfollow",
		Short: "Unfollow (leave) a channel",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			data, err := runLiveOrDelegate(flags, "channels.unfollow", map[string]any{"jid": jidStr, "follow": false},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opChannelsFollow(ctx, a, jidStr, false)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "channel JID (…@newsletter)")
	return cmd
}

func newChannelsMuteCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "mute",
		Short: "Mute a channel",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			data, err := runLiveOrDelegate(flags, "channels.mute", map[string]any{"jid": jidStr, "mute": true},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opChannelsMute(ctx, a, jidStr, true)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "channel JID (…@newsletter)")
	return cmd
}

func newChannelsUnmuteCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "unmute",
		Short: "Unmute a channel",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			data, err := runLiveOrDelegate(flags, "channels.unmute", map[string]any{"jid": jidStr, "mute": false},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opChannelsMute(ctx, a, jidStr, false)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "channel JID (…@newsletter)")
	return cmd
}
