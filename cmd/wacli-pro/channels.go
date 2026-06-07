package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/UAEpro/wacli-pro/internal/out"
	"go.mau.fi/whatsmeow/types"
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

			newsletters, err := a.WA().GetSubscribedNewsletters(ctx)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, newsletters)
			}

			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tJID\tSUBSCRIBERS")
			for _, n := range newsletters {
				name := n.ThreadMeta.Name.Text
				fmt.Fprintf(w, "%s\t%s\t%d\n", truncate(name, 40), n.ID.String(), n.ThreadMeta.SubscriberCount)
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

			var info *types.NewsletterMetadata
			if strings.TrimSpace(inviteLink) != "" {
				info, err = a.WA().GetNewsletterInfoWithInvite(ctx, inviteLink)
			} else {
				jid, parseErr := types.ParseJID(jidStr)
				if parseErr != nil {
					return parseErr
				}
				info, err = a.WA().GetNewsletterInfo(ctx, jid)
			}
			if err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, info)
			}

			fmt.Fprintf(os.Stdout, "JID: %s\nName: %s\nDescription: %s\n",
				info.ID.String(),
				info.ThreadMeta.Name.Text,
				info.ThreadMeta.Description.Text,
			)
			if info.ThreadMeta.SubscriberCount > 0 {
				fmt.Fprintf(os.Stdout, "Subscribers: %d\n", info.ThreadMeta.SubscriberCount)
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

			jid, err := types.ParseJID(jidStr)
			if err != nil {
				return err
			}
			if err := a.WA().FollowNewsletter(ctx, jid); err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"jid": jid.String(), "followed": true})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
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

			jid, err := types.ParseJID(jidStr)
			if err != nil {
				return err
			}
			if err := a.WA().UnfollowNewsletter(ctx, jid); err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"jid": jid.String(), "followed": false})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
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

			jid, err := types.ParseJID(jidStr)
			if err != nil {
				return err
			}
			if err := a.WA().NewsletterToggleMute(ctx, jid, true); err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"jid": jid.String(), "muted": true})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
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

			jid, err := types.ParseJID(jidStr)
			if err != nil {
				return err
			}
			if err := a.WA().NewsletterToggleMute(ctx, jid, false); err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"jid": jid.String(), "muted": false})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "channel JID (…@newsletter)")
	return cmd
}
