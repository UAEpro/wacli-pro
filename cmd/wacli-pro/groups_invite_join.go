package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/UAEpro/wacli-pro/internal/app"
	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/spf13/cobra"
)

func newGroupsInviteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invite",
		Short: "Manage group invite links",
	}
	cmd.AddCommand(newGroupsInviteLinkCmd(flags))
	return cmd
}

func newGroupsInviteLinkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Get or revoke invite links",
	}
	cmd.AddCommand(newGroupsInviteLinkGetCmd(flags))
	cmd.AddCommand(newGroupsInviteLinkRevokeCmd(flags))
	return cmd
}

func newGroupsInviteLinkGetCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get invite link",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			data, err := runLiveOrDelegate(flags, "groups.invite-link", map[string]any{"jid": jidStr, "revoke": false},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupInviteLink(ctx, a, jidStr, false)
				})
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, data)
			}
			fmt.Fprintln(os.Stdout, data["link"])
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	return cmd
}

func newGroupsInviteLinkRevokeCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke/reset invite link",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			data, err := runLiveOrDelegate(flags, "groups.invite-link", map[string]any{"jid": jidStr, "revoke": true},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupInviteLink(ctx, a, jidStr, true)
				})
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, data)
			}
			fmt.Fprintln(os.Stdout, data["link"])
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	return cmd
}

func newGroupsJoinCmd(flags *rootFlags) *cobra.Command {
	var code string
	cmd := &cobra.Command{
		Use:   "join",
		Short: "Join group by invite code",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(code) == "" {
				return fmt.Errorf("--code is required")
			}
			data, err := runLiveOrDelegate(flags, "groups.join", map[string]any{"code": code},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupJoin(ctx, a, code)
				})
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, data)
			}
			fmt.Fprintf(os.Stdout, "Joined: %v\n", data["jid"])
			return nil
		},
	}
	cmd.Flags().StringVar(&code, "code", "", "invite code (from link)")
	return cmd
}
