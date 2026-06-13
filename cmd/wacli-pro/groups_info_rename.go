package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/UAEpro/wacli-pro/internal/app"
	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/spf13/cobra"
	"go.mau.fi/whatsmeow/types"
)

func newGroupsInfoCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Fetch group info (live) and update local DB",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}

			// Delegate to the running daemon (which holds the lock + live
			// connection) if one is available; otherwise fall back to a
			// direct, locked live fetch.
			if data, err := tryDaemonCall(flags, "groups.info", map[string]any{"jid": jidStr}); err != nil {
				return err
			} else if data != nil {
				return printGroupInfo(flags, data)
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
			data, err := fetchGroupInfo(ctx, a, gjid)
			if err != nil {
				return err
			}
			return printGroupInfo(flags, data)
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	return cmd
}

// fetchGroupInfo performs the live group-info fetch on an already-connected
// app, persists it to the local DB, and returns a serializable map. Shared by
// the `groups info` command (direct path) and the `groups.info` IPC handler.
func fetchGroupInfo(ctx context.Context, a *app.App, gjid types.JID) (map[string]any, error) {
	info, err := a.WA().GetGroupInfo(ctx, gjid)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf("no group info returned for %s", gjid.String())
	}
	warnOnErr(persistGroupInfo(a.DB(), info), "persist group info")
	return groupInfoToMap(info), nil
}

// groupInfoToMap converts a whatsmeow GroupInfo into a stable, serializable map
// so the direct and IPC paths produce identical output.
func groupInfoToMap(info *types.GroupInfo) map[string]any {
	parts := make([]map[string]any, 0, len(info.Participants))
	for _, p := range info.Participants {
		parts = append(parts, map[string]any{
			"jid":            p.JID.String(),
			"is_admin":       p.IsAdmin,
			"is_super_admin": p.IsSuperAdmin,
		})
	}
	return map[string]any{
		"jid":                    info.JID.String(),
		"name":                   info.GroupName.Name,
		"owner":                  info.OwnerJID.String(),
		"created":                info.GroupCreated.Local().Format(time.RFC3339),
		"participant_count":      len(info.Participants),
		"participants":           parts,
		"topic":                  info.GroupTopic.Topic,
		"locked":                 info.IsLocked,
		"announce":               info.IsAnnounce,
		"member_add_mode":        string(info.MemberAddMode),
		"join_approval_required": info.IsJoinApprovalRequired,
	}
}

// printGroupInfo renders a group-info map as JSON or human-readable text.
func printGroupInfo(flags *rootFlags, data map[string]any) error {
	if flags.asJSON {
		return out.WriteJSON(os.Stdout, data)
	}
	fmt.Fprintf(os.Stdout, "JID: %v\nName: %v\nOwner: %v\nCreated: %v\nParticipants: %v\n",
		data["jid"], data["name"], data["owner"], data["created"], data["participant_count"])
	if topic, _ := data["topic"].(string); topic != "" {
		fmt.Fprintf(os.Stdout, "Topic: %s\n", topic)
	}
	fmt.Fprintf(os.Stdout, "Locked: %v\nAnnounce: %v\n", data["locked"], data["announce"])
	if mam, _ := data["member_add_mode"].(string); mam != "" {
		fmt.Fprintf(os.Stdout, "Member add mode: %s\n", mam)
	}
	if jar, _ := data["join_approval_required"].(bool); jar {
		fmt.Fprintln(os.Stdout, "Join approval: required")
	}
	return nil
}

func newGroupsRenameCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	var name string
	cmd := &cobra.Command{
		Use:   "rename",
		Short: "Rename group",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" || strings.TrimSpace(name) == "" {
				return fmt.Errorf("--jid and --name are required")
			}
			data, err := runLiveOrDelegate(flags, "groups.rename", map[string]any{"jid": jidStr, "name": name},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupRename(ctx, a, jidStr, name)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	cmd.Flags().StringVar(&name, "name", "", "new name")
	return cmd
}

func newGroupsLeaveCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "leave",
		Short: "Leave a group",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			data, err := runLiveOrDelegate(flags, "groups.leave", map[string]any{"jid": jidStr},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupLeave(ctx, a, jidStr)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	return cmd
}
