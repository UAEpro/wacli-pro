package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/UAEpro/wacli-pro/internal/app"
	"github.com/UAEpro/wacli-pro/internal/wa"
	"go.mau.fi/whatsmeow/types"
)

// This file holds the shared "operations" for every command that needs a live
// WhatsApp connection, plus their IPC handler wrappers. Each op runs against a
// connected *app.App and returns a serializable map. The command's direct path
// (via runLiveOrDelegate) and the daemon's IPC handler both call the same op,
// guaranteeing identical behaviour whether or not a daemon is running.

// ---------------------------------------------------------------------------
// groups
// ---------------------------------------------------------------------------

func opGroupRename(ctx context.Context, a *app.App, jidStr, name string) (map[string]any, error) {
	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	if err := a.WA().SetGroupName(ctx, gjid, name); err != nil {
		return nil, err
	}
	if info, err := a.WA().GetGroupInfo(ctx, gjid); err == nil && info != nil {
		warnOnErr(persistGroupInfo(a.DB(), info), "persist group info")
	}
	return map[string]any{"jid": gjid.String(), "name": name}, nil
}

func opGroupLeave(ctx context.Context, a *app.App, jidStr string) (map[string]any, error) {
	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	if err := a.WA().LeaveGroup(ctx, gjid); err != nil {
		return nil, err
	}
	warnOnErr(a.DB().MarkGroupLeft(gjid.String()), "mark group left")
	return map[string]any{"jid": gjid.String(), "left": true}, nil
}

func opGroupTopic(ctx context.Context, a *app.App, jidStr, topic string) (map[string]any, error) {
	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	if err := a.WA().SetGroupTopic(ctx, gjid, topic); err != nil {
		return nil, err
	}
	if info, err := a.WA().GetGroupInfo(ctx, gjid); err == nil && info != nil {
		warnOnErr(persistGroupInfo(a.DB(), info), "persist group info")
	}
	return map[string]any{"jid": gjid.String(), "topic": topic}, nil
}

func opGroupPhoto(ctx context.Context, a *app.App, jidStr, filePath string, remove bool) (map[string]any, error) {
	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	var avatar []byte
	if !remove {
		avatar, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read photo file: %w", err)
		}
	}
	pictureID, err := a.WA().SetGroupPhoto(ctx, gjid, avatar)
	if err != nil {
		return nil, err
	}
	return map[string]any{"jid": gjid.String(), "picture_id": pictureID}, nil
}

func opGroupLocked(ctx context.Context, a *app.App, jidStr string, locked bool) (map[string]any, error) {
	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	if err := a.WA().SetGroupLocked(ctx, gjid, locked); err != nil {
		return nil, err
	}
	return map[string]any{"jid": gjid.String(), "locked": locked}, nil
}

func opGroupAnnounce(ctx context.Context, a *app.App, jidStr string, announce bool) (map[string]any, error) {
	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	if err := a.WA().SetGroupAnnounce(ctx, gjid, announce); err != nil {
		return nil, err
	}
	return map[string]any{"jid": gjid.String(), "announce": announce}, nil
}

func opGroupJoinApproval(ctx context.Context, a *app.App, jidStr string, enable bool) (map[string]any, error) {
	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	if err := a.WA().SetGroupJoinApprovalMode(ctx, gjid, enable); err != nil {
		return nil, err
	}
	return map[string]any{"jid": gjid.String(), "join_approval": enable}, nil
}

func opGroupMemberAddMode(ctx context.Context, a *app.App, jidStr, mode string) (map[string]any, error) {
	var m types.GroupMemberAddMode
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "admin":
		m = types.GroupMemberAddModeAdmin
	case "all":
		m = types.GroupMemberAddModeAllMember
	default:
		return nil, fmt.Errorf("--mode must be 'admin' or 'all'")
	}
	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	if err := a.WA().SetGroupMemberAddMode(ctx, gjid, m); err != nil {
		return nil, err
	}
	return map[string]any{"jid": gjid.String(), "member_add_mode": string(m)}, nil
}

func opGroupParticipants(ctx context.Context, a *app.App, jidStr string, users []string, action string) (map[string]any, error) {
	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	var jids []types.JID
	for _, u := range users {
		j, err := wa.ParseUserOrJID(u)
		if err != nil {
			return nil, err
		}
		jids = append(jids, j)
	}
	updated, err := a.WA().UpdateGroupParticipants(ctx, gjid, jids, wa.GroupParticipantAction(action))
	if err != nil {
		return nil, err
	}
	if info, err := a.WA().GetGroupInfo(ctx, gjid); err == nil && info != nil {
		_ = persistGroupInfo(a.DB(), info)
	}
	parts := make([]any, 0, len(updated))
	for _, p := range updated {
		parts = append(parts, map[string]any{
			"jid":            p.JID.String(),
			"is_admin":       p.IsAdmin,
			"is_super_admin": p.IsSuperAdmin,
		})
	}
	return map[string]any{"jid": gjid.String(), "action": action, "participants": parts}, nil
}

func opGroupInviteLink(ctx context.Context, a *app.App, jidStr string, revoke bool) (map[string]any, error) {
	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	link, err := a.WA().GetGroupInviteLink(ctx, gjid, revoke)
	if err != nil {
		return nil, err
	}
	return map[string]any{"jid": gjid.String(), "link": link, "revoked": revoke}, nil
}

func opGroupJoin(ctx context.Context, a *app.App, code string) (map[string]any, error) {
	jid, err := a.WA().JoinGroupWithLink(ctx, code)
	if err != nil {
		return nil, err
	}
	if info, err := a.WA().GetGroupInfo(ctx, jid); err == nil && info != nil {
		_ = persistGroupInfo(a.DB(), info)
	}
	return map[string]any{"jid": jid.String(), "joined": true}, nil
}

func opGroupCreate(ctx context.Context, a *app.App, name string, users []string) (map[string]any, error) {
	var participants []types.JID
	for _, u := range users {
		jid, err := wa.ParseUserOrJID(u)
		if err != nil {
			return nil, fmt.Errorf("invalid user %q: %w", u, err)
		}
		participants = append(participants, jid)
	}
	info, err := a.WA().CreateGroup(ctx, name, participants)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf("no group info returned after create")
	}
	warnOnErr(persistGroupInfo(a.DB(), info), "persist group info")
	return map[string]any{"jid": info.JID.String(), "name": info.GroupName.Name}, nil
}

func opGroupRequestsList(ctx context.Context, a *app.App, jidStr string) (map[string]any, error) {
	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	requests, err := a.WA().GetGroupRequestParticipants(ctx, gjid)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0, len(requests))
	for _, r := range requests {
		items = append(items, map[string]any{
			"jid":          r.JID.String(),
			"requested_at": r.RequestedAt.Local().Format(time.RFC3339),
		})
	}
	return map[string]any{"jid": gjid.String(), "requests": items}, nil
}

func opGroupRequestsAction(ctx context.Context, a *app.App, jidStr string, users []string, approve bool) (map[string]any, error) {
	gjid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	var jids []types.JID
	for _, u := range users {
		jid, err := wa.ParseUserOrJID(u)
		if err != nil {
			return nil, fmt.Errorf("invalid user %q: %w", u, err)
		}
		jids = append(jids, jid)
	}
	if _, err := a.WA().UpdateGroupRequestParticipants(ctx, gjid, jids, approve); err != nil {
		return nil, err
	}
	action := "rejected"
	if approve {
		action = "approved"
	}
	return map[string]any{"jid": gjid.String(), "action": action, "users": users}, nil
}

func opGroupRefresh(ctx context.Context, a *app.App) (map[string]any, error) {
	gs, err := a.WA().GetJoinedGroups(ctx)
	if err != nil {
		return nil, err
	}
	for _, g := range gs {
		if g == nil {
			continue
		}
		warnOnErr(persistGroupInfo(a.DB(), g), "persist group info")
		warnOnErr(a.DB().UpsertChat(g.JID.String(), "group", g.GroupName.Name, time.Now()), "persist chat")
	}
	return map[string]any{"groups": len(gs)}, nil
}

// ---------------------------------------------------------------------------
// profile
// ---------------------------------------------------------------------------

func opProfileSetAbout(ctx context.Context, a *app.App, text string) (map[string]any, error) {
	if err := a.WA().SetStatusMessage(ctx, text); err != nil {
		return nil, err
	}
	return map[string]any{"about": text}, nil
}

func opProfileSetPhoto(ctx context.Context, a *app.App, filePath string) (map[string]any, error) {
	avatar, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read photo file: %w", err)
	}
	pictureID, err := a.WA().SetProfilePhoto(ctx, avatar)
	if err != nil {
		return nil, err
	}
	return map[string]any{"picture_id": pictureID}, nil
}

func opProfileRemovePhoto(ctx context.Context, a *app.App) (map[string]any, error) {
	if _, err := a.WA().SetProfilePhoto(ctx, nil); err != nil {
		return nil, err
	}
	return map[string]any{"removed": true}, nil
}

// ---------------------------------------------------------------------------
// channels (newsletters)
// ---------------------------------------------------------------------------

func opChannelsList(ctx context.Context, a *app.App) (map[string]any, error) {
	newsletters, err := a.WA().GetSubscribedNewsletters(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0, len(newsletters))
	for _, n := range newsletters {
		items = append(items, map[string]any{
			"jid":         n.ID.String(),
			"name":        n.ThreadMeta.Name.Text,
			"subscribers": n.ThreadMeta.SubscriberCount,
		})
	}
	return map[string]any{"channels": items}, nil
}

func opChannelsInfo(ctx context.Context, a *app.App, jidStr, inviteLink string) (map[string]any, error) {
	var info *types.NewsletterMetadata
	var err error
	if strings.TrimSpace(inviteLink) != "" {
		info, err = a.WA().GetNewsletterInfoWithInvite(ctx, inviteLink)
	} else {
		jid, parseErr := types.ParseJID(jidStr)
		if parseErr != nil {
			return nil, parseErr
		}
		info, err = a.WA().GetNewsletterInfo(ctx, jid)
	}
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, fmt.Errorf("no channel info returned")
	}
	return map[string]any{
		"jid":         info.ID.String(),
		"name":        info.ThreadMeta.Name.Text,
		"description": info.ThreadMeta.Description.Text,
		"subscribers": info.ThreadMeta.SubscriberCount,
	}, nil
}

func opChannelsFollow(ctx context.Context, a *app.App, jidStr string, follow bool) (map[string]any, error) {
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	if follow {
		err = a.WA().FollowNewsletter(ctx, jid)
	} else {
		err = a.WA().UnfollowNewsletter(ctx, jid)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"jid": jid.String(), "followed": follow}, nil
}

func opChannelsMute(ctx context.Context, a *app.App, jidStr string, mute bool) (map[string]any, error) {
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}
	if err := a.WA().NewsletterToggleMute(ctx, jid, mute); err != nil {
		return nil, err
	}
	return map[string]any{"jid": jid.String(), "muted": mute}, nil
}

// ---------------------------------------------------------------------------
// send
// ---------------------------------------------------------------------------

func opSendPoll(ctx context.Context, a *app.App, to, question string, options []string, maxSelections int) (map[string]any, error) {
	jid, err := wa.ParseUserOrJID(to)
	if err != nil {
		return nil, err
	}
	pollMsg := a.WA().BuildPollCreation(question, options, maxSelections)
	msgID, err := a.WA().SendProtoMessage(ctx, jid, pollMsg)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": string(msgID), "to": jid.String()}, nil
}

// ---------------------------------------------------------------------------
// IPC handlers — thin wrappers that translate params into op calls.
// ---------------------------------------------------------------------------

func handleGroupsRename(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupRename(ctx, a, paramString(p, "jid"), paramString(p, "name"))
}

func handleGroupsLeave(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupLeave(ctx, a, paramString(p, "jid"))
}

func handleGroupsTopic(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupTopic(ctx, a, paramString(p, "jid"), paramString(p, "topic"))
}

func handleGroupsPhoto(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupPhoto(ctx, a, paramString(p, "jid"), paramString(p, "file"), paramBool(p, "remove"))
}

func handleGroupsLock(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupLocked(ctx, a, paramString(p, "jid"), paramBool(p, "locked"))
}

func handleGroupsAnnounce(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupAnnounce(ctx, a, paramString(p, "jid"), paramBool(p, "announce"))
}

func handleGroupsJoinApproval(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupJoinApproval(ctx, a, paramString(p, "jid"), paramBool(p, "enable"))
}

func handleGroupsMemberAddMode(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupMemberAddMode(ctx, a, paramString(p, "jid"), paramString(p, "mode"))
}

func handleGroupsParticipants(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupParticipants(ctx, a, paramString(p, "jid"), paramStringSlice(p, "users"), paramString(p, "action"))
}

func handleGroupsInviteLink(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupInviteLink(ctx, a, paramString(p, "jid"), paramBool(p, "revoke"))
}

func handleGroupsJoin(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupJoin(ctx, a, paramString(p, "code"))
}

func handleGroupsCreate(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupCreate(ctx, a, paramString(p, "name"), paramStringSlice(p, "users"))
}

func handleGroupsRequestsList(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupRequestsList(ctx, a, paramString(p, "jid"))
}

func handleGroupsRequestsAction(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opGroupRequestsAction(ctx, a, paramString(p, "jid"), paramStringSlice(p, "users"), paramBool(p, "approve"))
}

func handleGroupsRefresh(ctx context.Context, a *app.App, _ map[string]any) (any, error) {
	return opGroupRefresh(ctx, a)
}

func handleProfileSetAbout(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opProfileSetAbout(ctx, a, paramString(p, "text"))
}

func handleProfileSetPhoto(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opProfileSetPhoto(ctx, a, paramString(p, "file"))
}

func handleProfileRemovePhoto(ctx context.Context, a *app.App, _ map[string]any) (any, error) {
	return opProfileRemovePhoto(ctx, a)
}

func handleChannelsList(ctx context.Context, a *app.App, _ map[string]any) (any, error) {
	return opChannelsList(ctx, a)
}

func handleChannelsInfo(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opChannelsInfo(ctx, a, paramString(p, "jid"), paramString(p, "invite"))
}

func handleChannelsFollow(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opChannelsFollow(ctx, a, paramString(p, "jid"), paramBool(p, "follow"))
}

func handleChannelsMute(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opChannelsMute(ctx, a, paramString(p, "jid"), paramBool(p, "mute"))
}

func handleSendPoll(ctx context.Context, a *app.App, p map[string]any) (any, error) {
	return opSendPoll(ctx, a, paramString(p, "to"), paramString(p, "question"),
		paramStringSlice(p, "options"), int(paramFloat64(p, "max_selections")))
}
