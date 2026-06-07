package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/UAEpro/wacli-pro/internal/wa"
	"go.mau.fi/whatsmeow/types"
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

			var participants []types.JID
			for _, u := range users {
				jid, err := wa.ParseUserOrJID(u)
				if err != nil {
					return fmt.Errorf("invalid user %q: %w", u, err)
				}
				participants = append(participants, jid)
			}

			info, err := a.WA().CreateGroup(ctx, name, participants)
			if err != nil {
				return err
			}
			if info != nil {
				warnOnErr(persistGroupInfo(a.DB(), info), "persist group info")
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, info)
			}
			fmt.Fprintf(os.Stdout, "Created: %s (%s)\n", info.GroupName.Name, info.JID.String())
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

			gjid, err := types.ParseJID(jidStr)
			if err != nil {
				return err
			}
			requests, err := a.WA().GetGroupRequestParticipants(ctx, gjid)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, requests)
			}
			if len(requests) == 0 {
				fmt.Fprintln(os.Stdout, "No pending requests")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "JID\tREQUESTED AT")
			for _, r := range requests {
				fmt.Fprintf(w, "%s\t%s\n", r.JID.String(), r.RequestedAt.Local().Format("2006-01-02 15:04"))
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

	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return err
	}
	var jids []types.JID
	for _, u := range users {
		jid, err := wa.ParseUserOrJID(u)
		if err != nil {
			return fmt.Errorf("invalid user %q: %w", u, err)
		}
		jids = append(jids, jid)
	}

	_, err = a.WA().UpdateGroupRequestParticipants(ctx, gjid, jids, approve)
	if err != nil {
		return err
	}
	action := "rejected"
	if approve {
		action = "approved"
	}
	if flags.asJSON {
		return out.WriteJSON(os.Stdout, map[string]any{"jid": gjid.String(), "action": action, "users": users})
	}
	fmt.Fprintln(os.Stdout, "OK")
	return nil
}
