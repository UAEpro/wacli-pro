package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/steipete/wacli/internal/app"
	"github.com/steipete/wacli/internal/ipc"
	"github.com/steipete/wacli/internal/store"
	"github.com/steipete/wacli/internal/wa"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
)

func registerIPCHandlers(s *ipc.Server) {
	s.Handle("send.text", handleSendText)
	s.Handle("send.file", handleSendFile)
	s.Handle("send.sticker", handleSendSticker)
	s.Handle("send.voice", handleSendVoice)
	s.Handle("send.reaction", handleSendReaction)
	s.Handle("send.location", handleSendLocation)
	s.Handle("send.forward", handleSendForward)
	s.Handle("status.text", handleStatusText)
	s.Handle("status.file", handleStatusFile)
	s.Handle("messages.delete", handleMessagesDelete)
	s.Handle("messages.edit", handleMessagesEdit)
	s.Handle("presence.typing", handlePresenceTyping)
	s.Handle("presence.paused", handlePresencePaused)
	s.Handle("media.download", handleMediaDownload)
	s.Handle("chats.archive", handleChatsArchive)
	s.Handle("chats.unarchive", handleChatsUnarchive)
	s.Handle("chats.pin", handleChatsPin)
	s.Handle("chats.unpin", handleChatsUnpin)
	s.Handle("chats.mute", handleChatsMute)
	s.Handle("chats.unmute", handleChatsUnmute)
	s.Handle("chats.mark-read", handleChatsMarkRead)
	s.Handle("chats.mark-unread", handleChatsMarkUnread)
}

// --- param helpers ---

func paramString(params map[string]any, key string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func paramFloat64(params map[string]any, key string) float64 {
	if v, ok := params[key]; ok {
		switch f := v.(type) {
		case float64:
			return f
		case int:
			return float64(f)
		}
	}
	return 0
}

func paramBool(params map[string]any, key string) bool {
	if v, ok := params[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// --- send handlers ---

func handleSendText(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	to := paramString(params, "to")
	message := paramString(params, "message")
	replyTo := paramString(params, "reply_to")
	replyChat := paramString(params, "reply_chat")

	if to == "" || message == "" {
		return nil, fmt.Errorf("--to and --message are required")
	}

	toJID, err := wa.ParseUserOrJID(to)
	if err != nil {
		return nil, err
	}

	var msgID types.MessageID
	if strings.TrimSpace(replyTo) != "" {
		chatJID := toJID.String()
		if strings.TrimSpace(replyChat) != "" {
			cj, err := wa.ParseUserOrJID(replyChat)
			if err != nil {
				return nil, fmt.Errorf("invalid --reply-chat: %w", err)
			}
			chatJID = cj.String()
		}
		qm, err := a.DB().GetMessage(chatJID, replyTo)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("reply-to message not found in local DB (chat=%s id=%s)", chatJID, replyTo)
			}
			return nil, err
		}
		var participant *types.JID
		if wa.IsGroupJID(toJID) {
			if strings.TrimSpace(qm.SenderJID) == "" {
				return nil, fmt.Errorf("reply-to in groups requires sender JID in local DB")
			}
			pj, err := types.ParseJID(qm.SenderJID)
			if err != nil {
				return nil, fmt.Errorf("invalid sender JID %q: %v", qm.SenderJID, err)
			}
			participant = &pj
		}
		quotedText := strings.TrimSpace(qm.Text)
		if quotedText == "" {
			quotedText = strings.TrimSpace(qm.DisplayText)
		}
		quotedMsg := &waProto.Message{Conversation: &quotedText}
		id, err := a.WA().SendTextReply(ctx, toJID, message, replyTo, participant, quotedMsg)
		if err != nil {
			return nil, err
		}
		msgID = id
	} else {
		id, err := a.WA().SendText(ctx, toJID, message)
		if err != nil {
			return nil, err
		}
		msgID = id
	}

	now := time.Now().UTC()
	chatName := a.WA().ResolveChatName(ctx, toJID, "")
	kind := chatKindFromJID(toJID)
	warnOnErr(a.DB().UpsertChat(toJID.String(), kind, chatName, now), "persist chat")
	warnOnErr(a.DB().UpsertMessage(store.UpsertMessageParams{
		ChatJID:    toJID.String(),
		ChatName:   chatName,
		MsgID:      string(msgID),
		SenderJID:  "",
		SenderName: "me",
		Timestamp:  now,
		FromMe:     true,
		Text:       message,
	}), "persist message")

	return map[string]any{
		"sent": true,
		"to":   toJID.String(),
		"id":   msgID,
	}, nil
}

func handleSendFile(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	to := paramString(params, "to")
	filePath := paramString(params, "file_path")
	filename := paramString(params, "filename")
	caption := paramString(params, "caption")
	mimeOverride := paramString(params, "mime")
	ptt := paramBool(params, "ptt")

	if to == "" || filePath == "" {
		return nil, fmt.Errorf("--to and --file are required")
	}

	toJID, err := wa.ParseUserOrJID(to)
	if err != nil {
		return nil, err
	}

	msgID, meta, err := sendFile(ctx, a, toJID, filePath, filename, caption, mimeOverride, ptt)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"sent": true,
		"to":   toJID.String(),
		"id":   msgID,
		"file": meta,
	}, nil
}

func handleSendSticker(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	to := paramString(params, "to")
	filePath := paramString(params, "file_path")

	if to == "" || filePath == "" {
		return nil, fmt.Errorf("--to and --file are required")
	}

	toJID, err := wa.ParseUserOrJID(to)
	if err != nil {
		return nil, err
	}

	msgID, err := sendSticker(ctx, a, toJID, filePath)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"sent": true,
		"to":   toJID.String(),
		"id":   msgID,
		"type": "sticker",
	}, nil
}

func handleSendVoice(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	to := paramString(params, "to")
	filePath := paramString(params, "file_path")

	if to == "" || filePath == "" {
		return nil, fmt.Errorf("--to and --file are required")
	}

	toJID, err := wa.ParseUserOrJID(to)
	if err != nil {
		return nil, err
	}

	msgID, meta, err := sendFile(ctx, a, toJID, filePath, "", "", "", true)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"sent": true,
		"to":   toJID.String(),
		"id":   msgID,
		"file": meta,
	}, nil
}

func handleSendReaction(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	to := paramString(params, "to")
	id := paramString(params, "id")
	emoji := paramString(params, "emoji")

	if to == "" || id == "" {
		return nil, fmt.Errorf("--to and --id are required")
	}

	toJID, err := wa.ParseUserOrJID(to)
	if err != nil {
		return nil, err
	}

	if err := a.WA().SendReaction(ctx, toJID, types.MessageID(id), emoji); err != nil {
		return nil, err
	}

	return map[string]any{
		"reacted": true,
		"to":      toJID.String(),
		"id":      id,
		"emoji":   emoji,
	}, nil
}

func handleSendLocation(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	to := paramString(params, "to")
	lat := paramFloat64(params, "lat")
	lng := paramFloat64(params, "lng")
	name := paramString(params, "name")
	address := paramString(params, "address")

	if to == "" {
		return nil, fmt.Errorf("--to is required")
	}

	toJID, err := wa.ParseUserOrJID(to)
	if err != nil {
		return nil, err
	}

	msgID, err := a.WA().SendLocation(ctx, toJID, lat, lng, name, address)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	chatName := a.WA().ResolveChatName(ctx, toJID, "")
	kind := chatKindFromJID(toJID)
	warnOnErr(a.DB().UpsertChat(toJID.String(), kind, chatName, now), "persist chat")
	displayText := fmt.Sprintf("Location: %.6f, %.6f", lat, lng)
	if name != "" {
		displayText = fmt.Sprintf("Location: %s (%.6f, %.6f)", name, lat, lng)
	}
	warnOnErr(a.DB().UpsertMessage(store.UpsertMessageParams{
		ChatJID:     toJID.String(),
		ChatName:    chatName,
		MsgID:       string(msgID),
		SenderName:  "me",
		Timestamp:   now,
		FromMe:      true,
		Text:        displayText,
		DisplayText: displayText,
		MediaType:   "location",
	}), "persist message")

	return map[string]any{
		"sent": true,
		"to":   toJID.String(),
		"id":   msgID,
		"lat":  lat,
		"lng":  lng,
	}, nil
}

func handleSendForward(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	to := paramString(params, "to")
	fromChat := paramString(params, "from_chat")
	id := paramString(params, "id")

	if to == "" || fromChat == "" || id == "" {
		return nil, fmt.Errorf("--to, --from-chat, and --id are required")
	}

	toJID, err := wa.ParseUserOrJID(to)
	if err != nil {
		return nil, err
	}

	if _, err := wa.ParseUserOrJID(fromChat); err != nil {
		return nil, fmt.Errorf("invalid --from-chat: %w", err)
	}

	m, err := a.DB().GetMessage(fromChat, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("message not found in local DB (chat=%s id=%s)", fromChat, id)
		}
		return nil, err
	}

	text := m.Text
	if text == "" {
		text = m.DisplayText
	}
	if text == "" {
		return nil, fmt.Errorf("only text messages can be forwarded (message has no text content)")
	}

	isForwarded := true
	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: &text,
			ContextInfo: &waProto.ContextInfo{
				IsForwarded: &isForwarded,
			},
		},
	}

	resp, err := a.WA().SendProtoMessage(ctx, toJID, msg)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	chatName := a.WA().ResolveChatName(ctx, toJID, "")
	kind := chatKindFromJID(toJID)
	warnOnErr(a.DB().UpsertChat(toJID.String(), kind, chatName, now), "persist chat")
	warnOnErr(a.DB().UpsertMessage(store.UpsertMessageParams{
		ChatJID:     toJID.String(),
		ChatName:    chatName,
		MsgID:       string(resp),
		SenderName:  "me",
		Timestamp:   now,
		FromMe:      true,
		Text:        text,
		DisplayText: "Forwarded: " + text,
	}), "persist message")

	return map[string]any{
		"forwarded": true,
		"to":        toJID.String(),
		"id":        resp,
		"from_chat": fromChat,
		"from_id":   id,
	}, nil
}

// --- status handlers ---

func handleStatusText(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	text := paramString(params, "text")
	if text == "" {
		return nil, fmt.Errorf("--text is required")
	}

	msgID, err := a.WA().SendText(ctx, types.StatusBroadcastJID, text)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	warnOnErr(a.DB().UpsertChat(types.StatusBroadcastJID.String(), "broadcast", "My Status", now), "persist chat")
	warnOnErr(a.DB().UpsertMessage(store.UpsertMessageParams{
		ChatJID:     types.StatusBroadcastJID.String(),
		ChatName:    "My Status",
		MsgID:       string(msgID),
		SenderName:  "me",
		Timestamp:   now,
		FromMe:      true,
		Text:        text,
		DisplayText: text,
	}), "persist message")

	return map[string]any{
		"posted": true,
		"id":     msgID,
		"type":   "text",
	}, nil
}

func handleStatusFile(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	filePath := paramString(params, "file_path")
	caption := paramString(params, "caption")
	filename := paramString(params, "filename")
	mimeOverride := paramString(params, "mime")

	if filePath == "" {
		return nil, fmt.Errorf("--file is required")
	}

	msgID, meta, err := sendFile(ctx, a, types.StatusBroadcastJID, filePath, filename, caption, mimeOverride, false)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"posted": true,
		"id":     msgID,
		"type":   "file",
		"file":   meta,
	}, nil
}

// --- messages handlers ---

func handleMessagesDelete(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	chat := paramString(params, "chat")
	id := paramString(params, "id")

	if chat == "" || id == "" {
		return nil, fmt.Errorf("--chat and --id are required")
	}

	m, err := a.DB().GetMessage(chat, id)
	if err != nil {
		return nil, err
	}
	if !m.FromMe {
		return nil, fmt.Errorf("can only delete your own messages")
	}

	chatJID, err := wa.ParseUserOrJID(chat)
	if err != nil {
		return nil, err
	}

	if err := a.WA().RevokeMessage(ctx, chatJID, types.MessageID(id)); err != nil {
		return nil, err
	}
	if err := a.DB().MarkRevoked(chat, id); err != nil {
		return nil, err
	}

	return map[string]any{
		"revoked": true,
		"chat":    chat,
		"id":      id,
	}, nil
}

func handleMessagesEdit(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	chat := paramString(params, "chat")
	id := paramString(params, "id")
	message := paramString(params, "message")

	if chat == "" || id == "" || message == "" {
		return nil, fmt.Errorf("--chat, --id and --message are required")
	}

	m, err := a.DB().GetMessage(chat, id)
	if err != nil {
		return nil, err
	}
	if !m.FromMe {
		return nil, fmt.Errorf("can only edit your own messages")
	}

	chatJID, err := wa.ParseUserOrJID(chat)
	if err != nil {
		return nil, err
	}

	if _, err := a.WA().EditMessage(ctx, chatJID, types.MessageID(id), message); err != nil {
		return nil, err
	}

	chatName := m.ChatName
	if chatName == "" {
		chatName = a.WA().ResolveChatName(ctx, chatJID, "")
	}
	warnOnErr(a.DB().UpsertMessage(store.UpsertMessageParams{
		ChatJID:     chat,
		ChatName:    chatName,
		MsgID:       id,
		SenderJID:   "",
		SenderName:  "me",
		Timestamp:   m.Timestamp,
		FromMe:      true,
		Text:        message,
		DisplayText: message,
	}), "persist edited message")

	return map[string]any{
		"edited": true,
		"chat":   chat,
		"id":     id,
	}, nil
}

// --- presence handlers ---

func handlePresenceTyping(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	to := paramString(params, "to")
	media := paramString(params, "media")

	if to == "" {
		return nil, fmt.Errorf("--to is required")
	}

	toJID, err := wa.ParseUserOrJID(to)
	if err != nil {
		return nil, err
	}

	var chatMedia types.ChatPresenceMedia
	if strings.EqualFold(media, "audio") {
		chatMedia = types.ChatPresenceMediaAudio
	}

	if err := a.WA().SendChatPresence(ctx, toJID, types.ChatPresenceComposing, chatMedia); err != nil {
		return nil, err
	}

	return map[string]any{
		"sent":  true,
		"to":    toJID.String(),
		"state": string(types.ChatPresenceComposing),
	}, nil
}

func handlePresencePaused(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	to := paramString(params, "to")

	if to == "" {
		return nil, fmt.Errorf("--to is required")
	}

	toJID, err := wa.ParseUserOrJID(to)
	if err != nil {
		return nil, err
	}

	if err := a.WA().SendChatPresence(ctx, toJID, types.ChatPresencePaused, ""); err != nil {
		return nil, err
	}

	return map[string]any{
		"sent":  true,
		"to":    toJID.String(),
		"state": string(types.ChatPresencePaused),
	}, nil
}

// --- media handler ---

func handleMediaDownload(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	chat := paramString(params, "chat")
	id := paramString(params, "id")
	outputPath := paramString(params, "output")

	if chat == "" || id == "" {
		return nil, fmt.Errorf("--chat and --id are required")
	}

	info, err := a.DB().GetMediaDownloadInfo(chat, id)
	if err != nil {
		return nil, err
	}
	if info.MediaType == "" || info.DirectPath == "" || len(info.MediaKey) == 0 {
		return nil, fmt.Errorf("message has no downloadable media metadata (run `wacli sync` first)")
	}

	target, err := a.ResolveMediaOutputPath(info, outputPath)
	if err != nil {
		return nil, err
	}

	bytes, err := a.WA().DownloadMediaToFile(ctx, info.DirectPath, info.FileEncSHA256, info.FileSHA256, info.MediaKey, info.FileLength, info.MediaType, "", target)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	warnOnErr(a.DB().MarkMediaDownloaded(info.ChatJID, info.MsgID, target, now), "mark media downloaded")

	return map[string]any{
		"chat":          info.ChatJID,
		"id":            info.MsgID,
		"path":          target,
		"bytes":         bytes,
		"media_type":    info.MediaType,
		"mime_type":     info.MimeType,
		"downloaded":    true,
		"downloaded_at": now.Format(time.RFC3339Nano),
	}, nil
}

// --- chat state handlers ---

func handleChatsArchive(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	return handleChatState(ctx, a, params, "archive", func(ctx context.Context, a *app.App, jid types.JID) error {
		return a.ArchiveChat(ctx, jid, true)
	})
}

func handleChatsUnarchive(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	return handleChatState(ctx, a, params, "unarchive", func(ctx context.Context, a *app.App, jid types.JID) error {
		return a.ArchiveChat(ctx, jid, false)
	})
}

func handleChatsPin(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	return handleChatState(ctx, a, params, "pin", func(ctx context.Context, a *app.App, jid types.JID) error {
		return a.PinChat(ctx, jid, true)
	})
}

func handleChatsUnpin(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	return handleChatState(ctx, a, params, "unpin", func(ctx context.Context, a *app.App, jid types.JID) error {
		return a.PinChat(ctx, jid, false)
	})
}

func handleChatsMute(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	durStr := paramString(params, "duration")
	var dur time.Duration
	if strings.TrimSpace(durStr) != "" {
		var err error
		dur, err = time.ParseDuration(durStr)
		if err != nil {
			return nil, fmt.Errorf("invalid --duration: %w", err)
		}
	}

	return handleChatState(ctx, a, params, "mute", func(ctx context.Context, a *app.App, jid types.JID) error {
		return a.MuteChat(ctx, jid, true, dur)
	})
}

func handleChatsUnmute(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	return handleChatState(ctx, a, params, "unmute", func(ctx context.Context, a *app.App, jid types.JID) error {
		return a.MuteChat(ctx, jid, false, 0)
	})
}

func handleChatsMarkRead(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	return handleChatState(ctx, a, params, "mark-read", func(ctx context.Context, a *app.App, jid types.JID) error {
		return a.MarkChatRead(ctx, jid, true)
	})
}

func handleChatsMarkUnread(ctx context.Context, a *app.App, params map[string]any) (any, error) {
	return handleChatState(ctx, a, params, "mark-unread", func(ctx context.Context, a *app.App, jid types.JID) error {
		return a.MarkChatRead(ctx, jid, false)
	})
}

func handleChatState(ctx context.Context, a *app.App, params map[string]any, action string, fn func(context.Context, *app.App, types.JID) error) (any, error) {
	jidStr := paramString(params, "jid")
	if strings.TrimSpace(jidStr) == "" {
		return nil, fmt.Errorf("--jid is required")
	}

	jid, err := wa.ParseUserOrJID(jidStr)
	if err != nil {
		return nil, err
	}

	if err := fn(ctx, a, jid); err != nil {
		return nil, err
	}

	return map[string]any{
		"jid":    jid.String(),
		"action": action,
		"ok":     true,
	}, nil
}
