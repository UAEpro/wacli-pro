package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/UAEpro/wacli-pro/internal/app"
	"github.com/spf13/cobra"
)

func newGroupsParticipantsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "participants",
		Short: "Manage group participants",
	}
	cmd.AddCommand(newGroupsParticipantsActionCmd(flags, "add"))
	cmd.AddCommand(newGroupsParticipantsActionCmd(flags, "remove"))
	cmd.AddCommand(newGroupsParticipantsActionCmd(flags, "promote"))
	cmd.AddCommand(newGroupsParticipantsActionCmd(flags, "demote"))
	return cmd
}

func newGroupsParticipantsActionCmd(flags *rootFlags, action string) *cobra.Command {
	var group string
	var users []string
	cmd := &cobra.Command{
		Use:   action,
		Short: action + " participants",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(group) == "" || len(users) == 0 {
				return fmt.Errorf("--jid and at least one --user are required")
			}
			data, err := runLiveOrDelegate(flags, "groups.participants",
				map[string]any{"jid": group, "users": users, "action": action},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupParticipants(ctx, a, group, users, action)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
		},
	}
	cmd.Flags().StringVar(&group, "jid", "", "group JID (…@g.us)")
	cmd.Flags().StringSliceVar(&users, "user", nil, "user phone number or JID (repeatable)")
	return cmd
}
