package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/UAEpro/wacli-pro/internal/app"
	"github.com/spf13/cobra"
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
			data, err := runLiveOrDelegate(flags, "groups.topic", map[string]any{"jid": jidStr, "topic": topic},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupTopic(ctx, a, jidStr, topic)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
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
			data, err := runLiveOrDelegate(flags, "groups.photo", map[string]any{"jid": jidStr, "file": filePath, "remove": remove},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupPhoto(ctx, a, jidStr, filePath, remove)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
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
			data, err := runLiveOrDelegate(flags, "groups.lock", map[string]any{"jid": jidStr, "locked": true},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupLocked(ctx, a, jidStr, true)
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

func newGroupsUnlockCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Unlock group settings (all participants can edit group info)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			data, err := runLiveOrDelegate(flags, "groups.lock", map[string]any{"jid": jidStr, "locked": false},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupLocked(ctx, a, jidStr, false)
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

func newGroupsAnnounceCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "announce",
		Short: "Enable announce mode (only admins can send messages)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			data, err := runLiveOrDelegate(flags, "groups.announce", map[string]any{"jid": jidStr, "announce": true},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupAnnounce(ctx, a, jidStr, true)
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

func newGroupsUnannounceCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	cmd := &cobra.Command{
		Use:   "unannounce",
		Short: "Disable announce mode (all participants can send messages)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}
			data, err := runLiveOrDelegate(flags, "groups.announce", map[string]any{"jid": jidStr, "announce": false},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupAnnounce(ctx, a, jidStr, false)
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
			data, err := runLiveOrDelegate(flags, "groups.join-approval", map[string]any{"jid": jidStr, "enable": enable},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupJoinApproval(ctx, a, jidStr, enable)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
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
			data, err := runLiveOrDelegate(flags, "groups.member-add-mode", map[string]any{"jid": jidStr, "mode": mode},
				func(ctx context.Context, a *app.App) (map[string]any, error) {
					return opGroupMemberAddMode(ctx, a, jidStr, mode)
				})
			if err != nil {
				return err
			}
			return outputOK(flags, data)
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "group JID (…@g.us)")
	cmd.Flags().StringVar(&mode, "mode", "", "who can add members: 'admin' or 'all'")
	return cmd
}
