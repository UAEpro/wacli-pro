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

func newGroupsCreateCmd(flags *rootFlags) *cobra.Command {
	var name string
	var users []string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new group",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("--name is required")
			}
			if len(users) == 0 {
				return fmt.Errorf("at least one --user is required")
			}
			data, err := runLiveOrDelegate(flags, "groups.create", map[string]any{"name": name, "users": users},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupCreate(ctx, a, name, users)
				})
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, data)
			}
			fmt.Fprintf(os.Stdout, "Created: %v (%v)\n", data["name"], data["jid"])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "group name (max 25 characters)")
	cmd.Flags().StringSliceVar(&users, "user", nil, "participant phone number or JID (repeatable)")
	return cmd
}

func newGroupsRequestsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "requests",
		Short: "Manage group join requests",
	}
	cmd.AddCommand(newGroupsRequestsListCmd(flags))
	cmd.AddCommand(newGroupsRequestsApproveCmd(flags))
	cmd.AddCommand(newGroupsRequestsRejectCmd(flags))
	return cmd
}

func newGroupsRequestsListCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pending join requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			data, err := runLiveOrDelegate(flags, "groups.requests-list", map[string]any{"jid": jidStr},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupRequestsList(ctx, a, jidStr)
				})
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, data)
			}
			list, _ := data["requests"].([]any)
			if len(list) == 0 {
				fmt.Fprintln(os.Stdout, "No pending requests")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "JID\tREQUESTED AT")
			for _, item := range list {
				m, _ := item.(map[string]any)
				fmt.Fprintf(w, "%v\t%v\n", m["jid"], m["requested_at"])
			}
			_ = w.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	return cmd
}

func newGroupsRequestsApproveCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	var users []string
	cmd := &cobra.Command{
		Use:   "approve",
		Short: "Approve join requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleGroupRequestAction(flags, jidStr, users, true)
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	cmd.Flags().StringSliceVar(&users, "user", nil, "user phone number or JID (repeatable)")
	return cmd
}

func newGroupsRequestsRejectCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	var users []string
	cmd := &cobra.Command{
		Use:   "reject",
		Short: "Reject join requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			return handleGroupRequestAction(flags, jidStr, users, false)
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	cmd.Flags().StringSliceVar(&users, "user", nil, "user phone number or JID (repeatable)")
	return cmd
}

func handleGroupRequestAction(flags *rootFlags, jidStr string, users []string, approve bool) error {
	if strings.TrimSpace(jidStr) == "" {
		return fmt.Errorf("--jid is required")
	}
	if len(users) == 0 {
		return fmt.Errorf("at least one --user is required")
	}
	data, err := runLiveOrDelegate(flags, "groups.requests-action",
		map[string]any{"jid": jidStr, "users": users, "approve": approve},
		func(ctx context.Context, a *app.App) (map[string]any, error) {
			return opGroupRequestsAction(ctx, a, jidStr, users, approve)
		})
	if err != nil {
		return err
	}
	return outputOK(flags, data)
}
