package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/spf13/cobra"
	"go.mau.fi/whatsmeow/types"
)

func newGroupsTopicCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	var topic string
	cmd := &cobra.Command{
		Use:   "topic",
		Short: "Set or clear the group description/topic",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			if !cmd.Flags().Changed("topic") {
				return fmt.Errorf("--topic is required (use empty string to clear)")
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
			if err := a.WA().SetGroupTopic(ctx, gjid, topic); err != nil {
				return err
			}
			if info, err := a.WA().GetGroupInfo(ctx, gjid); err == nil && info != nil {
				warnOnErr(persistGroupInfo(a.DB(), info), "persist group info")
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"jid": gjid.String(), "topic": topic})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	cmd.Flags().StringVar(&topic, "topic", "", "new topic/description (empty to clear)")
	return cmd
}

func newGroupsPhotoCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	var filePath string
	var remove bool
	cmd := &cobra.Command{
		Use:   "photo",
		Short: "Set or remove the group photo",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			if !remove && strings.TrimSpace(filePath) == "" {
				return fmt.Errorf("--file is required (or use --remove to remove photo)")
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

			var avatar []byte
			if !remove {
				avatar, err = os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("read photo file: %w", err)
				}
			}

			pictureID, err := a.WA().SetGroupPhoto(ctx, gjid, avatar)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"jid": gjid.String(), "picture_id": pictureID})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	cmd.Flags().StringVar(&filePath, "file", "", "path to photo file (JPEG)")
	cmd.Flags().BoolVar(&remove, "remove", false, "remove group photo")
	return cmd
}

func newGroupsLockCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Lock group settings (only admins can edit group info)",
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
			if err := a.WA().SetGroupLocked(ctx, gjid, true); err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"jid": gjid.String(), "locked": true})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	return cmd
}

func newGroupsUnlockCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Unlock group settings (all participants can edit group info)",
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
			if err := a.WA().SetGroupLocked(ctx, gjid, false); err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"jid": gjid.String(), "locked": false})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	return cmd
}

func newGroupsAnnounceCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "announce",
		Short: "Enable announce mode (only admins can send messages)",
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
			if err := a.WA().SetGroupAnnounce(ctx, gjid, true); err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"jid": gjid.String(), "announce": true})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	return cmd
}

func newGroupsUnannounceCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "unannounce",
		Short: "Disable announce mode (all participants can send messages)",
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
			if err := a.WA().SetGroupAnnounce(ctx, gjid, false); err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"jid": gjid.String(), "announce": false})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	return cmd
}

func newGroupsJoinApprovalCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	var enable bool
	var disable bool
	cmd := &cobra.Command{
		Use:   "join-approval",
		Short: "Toggle join approval mode (require admin approval to join)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			if enable == disable {
				return fmt.Errorf("specify exactly one of --on or --off")
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
			if err := a.WA().SetGroupJoinApprovalMode(ctx, gjid, enable); err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"jid": gjid.String(), "join_approval": enable})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	cmd.Flags().BoolVar(&enable, "on", false, "enable join approval")
	cmd.Flags().BoolVar(&disable, "off", false, "disable join approval")
	return cmd
}

func newGroupsMemberAddModeCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	var mode string
	cmd := &cobra.Command{
		Use:   "member-add-mode",
		Short: "Set who can add members (admin or all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			var m types.GroupMemberAddMode
			switch strings.ToLower(strings.TrimSpace(mode)) {
			case "admin":
				m = types.GroupMemberAddModeAdmin
			case "all":
				m = types.GroupMemberAddModeAllMember
			default:
				return fmt.Errorf("--mode must be 'admin' or 'all'")
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
			if err := a.WA().SetGroupMemberAddMode(ctx, gjid, m); err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"jid": gjid.String(), "member_add_mode": string(m)})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	cmd.Flags().StringVar(&mode, "mode", "", "who can add members: 'admin' or 'all'")
	return cmd
}
