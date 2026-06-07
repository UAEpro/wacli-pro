package main

import "github.com/spf13/cobra"

func newGroupsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "Group management",
	}
	cmd.AddCommand(newGroupsListCmd(flags))
	cmd.AddCommand(newGroupsRefreshCmd(flags))
	cmd.AddCommand(newGroupsInfoCmd(flags))
	cmd.AddCommand(newGroupsRenameCmd(flags))
	cmd.AddCommand(newGroupsTopicCmd(flags))
	cmd.AddCommand(newGroupsPhotoCmd(flags))
	cmd.AddCommand(newGroupsLockCmd(flags))
	cmd.AddCommand(newGroupsUnlockCmd(flags))
	cmd.AddCommand(newGroupsAnnounceCmd(flags))
	cmd.AddCommand(newGroupsUnannounceCmd(flags))
	cmd.AddCommand(newGroupsJoinApprovalCmd(flags))
	cmd.AddCommand(newGroupsMemberAddModeCmd(flags))
	cmd.AddCommand(newGroupsParticipantsCmd(flags))
	cmd.AddCommand(newGroupsInviteCmd(flags))
	cmd.AddCommand(newGroupsJoinCmd(flags))
	cmd.AddCommand(newGroupsLeaveCmd(flags))
	return cmd
}
