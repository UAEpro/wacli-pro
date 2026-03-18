package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/steipete/wacli/internal/store"
	"github.com/steipete/wacli/internal/wa"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type SyncMode string

const (
	SyncModeBootstrap SyncMode = "bootstrap"
	SyncModeOnce      SyncMode = "once"
	SyncModeFollow    SyncMode = "follow"
)

type SyncOptions struct {
	Mode            SyncMode
	AllowQR         bool
	OnQRCode        func(string)
	AfterConnect    func(context.Context) error
	DownloadMedia   bool
	RefreshContacts bool
	RefreshGroups   bool
	IdleExit        time.Duration // only used for bootstrap/once
	MaxDuration     time.Duration // hard timeout for bootstrap/once (0 = no limit)
	Verbosity       int           // future
}

type SyncResult struct {
	MessagesStored int64
}

func (a *App) Sync(ctx context.Context, opts SyncOptions) (SyncResult, error) {
	if opts.Mode == "" {
		opts.Mode = SyncModeFollow
	}
	if (opts.Mode == SyncModeBootstrap || opts.Mode == SyncModeOnce) && opts.IdleExit <= 0 {
		opts.IdleExit = 30 * time.Second
	}

	// Hard timeout for bootstrap/once prevents hanging forever when
	// WhatsApp keeps sending events and idle never triggers (#87).
	if opts.MaxDuration > 0 && opts.Mode != SyncModeFollow {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.MaxDuration)
		defer cancel()
	}

	if err := a.OpenWA(); err != nil {
		return SyncResult{}, err
	}

	var messagesStored atomic.Int64
	lastEvent := atomic.Int64{}
	lastEvent.Store(time.Now().UTC().UnixNano())

	disconnected := make(chan struct{}, 1)

	var stopMedia func()
	var mediaJobs chan mediaJob
	enqueueMedia := func(chatJID, msgID string) {}
	if opts.DownloadMedia {
		mediaJobs = make(chan mediaJob, 512)
		enqueueMedia = func(chatJID, msgID string) {
			if strings.TrimSpace(chatJID) == "" || strings.TrimSpace(msgID) == "" {
				return
			}
			select {
			case mediaJobs <- mediaJob{chatJID: chatJID, msgID: msgID}:
			default:
				// Drop jobs when the channel is full rather than spawning
				// unbounded goroutines that could leak during large syncs.
				fmt.Fprintf(os.Stderr, "media job queue full, dropping %s/%s\n", chatJID, msgID)
			}
		}
	}

	handlerID := a.wa.AddEventHandler(func(evt interface{}) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "recovered panic in event handler: %v\n", r)
			}
		}()
		lastEvent.Store(time.Now().UTC().UnixNano())

		switch v := evt.(type) {
		case *events.Message:
			if chatJID, msgID, isRevoke := wa.RevokeInfo(v); isRevoke {
				if err := a.db.MarkRevoked(chatJID, msgID); err == nil {
					messagesStored.Add(1)
				}
			} else if editPM, isEdit := wa.ParseEditEvent(v); isEdit {
				if err := a.storeParsedMessage(ctx, editPM); err == nil {
					messagesStored.Add(1)
				}
			} else {
				pm := wa.ParseLiveMessage(v)
				if pm.ReactionToID != "" && pm.ReactionEmoji == "" && v.Message != nil && v.Message.GetEncReactionMessage() != nil {
					if reaction, err := a.wa.DecryptReaction(ctx, v); err == nil && reaction != nil {
						pm.ReactionEmoji = reaction.GetText()
						if pm.ReactionToID == "" {
							if key := reaction.GetKey(); key != nil {
								pm.ReactionToID = key.GetID()
							}
						}
					}
				}
				if err := a.storeParsedMessage(ctx, pm); err == nil {
					messagesStored.Add(1)
				}
				if a.events.Enabled() {
					data := map[string]interface{}{
						"id":        pm.ID,
						"chat":      pm.Chat.String(),
						"sender":    pm.SenderJID,
						"timestamp": pm.Timestamp.Unix(),
						"from_me":   pm.FromMe,
						"is_group":  pm.Chat.Server == types.GroupServer,
					}
					if pm.PushName != "" {
						data["push_name"] = pm.PushName
					}
					if pm.Text != "" {
						data["text"] = pm.Text
					}
					if pm.Media != nil {
						data["media_type"] = pm.Media.Type
						if pm.Media.Caption != "" {
							data["caption"] = pm.Media.Caption
						}
						if pm.Media.MimeType != "" {
							data["mime_type"] = pm.Media.MimeType
						}
						if pm.Media.Filename != "" {
							data["filename"] = pm.Media.Filename
						}
					}
					if pm.ReactionEmoji != "" {
						data["reaction_emoji"] = pm.ReactionEmoji
						data["reaction_to_id"] = pm.ReactionToID
					}
					if pm.ReplyToID != "" {
						data["reply_to_id"] = pm.ReplyToID
					}
					a.events.Emit("new_message", data)
				}
				if opts.DownloadMedia && pm.Media != nil && pm.ID != "" {
					enqueueMedia(pm.Chat.String(), pm.ID)
				}
			}
			if messagesStored.Load()%25 == 0 {
				n := messagesStored.Load()
				if a.events.Enabled() {
					a.events.Emit("progress", map[string]interface{}{"messages_synced": n})
				} else {
					fmt.Fprintf(os.Stderr, "\rSynced %d messages...", n)
				}
			}
		case *events.HistorySync:
			nConv := len(v.Data.Conversations)
			if a.events.Enabled() {
				a.events.Emit("history_sync", map[string]interface{}{"conversations": nConv})
			} else {
				fmt.Fprintf(os.Stderr, "\nProcessing history sync (%d conversations)...\n", nConv)
			}
			for _, conv := range v.Data.Conversations {
				lastEvent.Store(time.Now().UTC().UnixNano())
				chatID := strings.TrimSpace(conv.GetID())
				if chatID == "" {
					continue
				}
				for _, m := range conv.Messages {
					lastEvent.Store(time.Now().UTC().UnixNano())
					if m.Message == nil {
						continue
					}
					pm := wa.ParseHistoryMessage(chatID, m.Message)
					if pm.ID == "" || pm.Chat.IsEmpty() {
						continue
					}
					if err := a.storeParsedMessage(ctx, pm); err == nil {
						messagesStored.Add(1)
					}
					if opts.DownloadMedia && pm.Media != nil && pm.ID != "" {
						enqueueMedia(pm.Chat.String(), pm.ID)
					}
				}
			}
			n := messagesStored.Load()
			if a.events.Enabled() {
				a.events.Emit("progress", map[string]interface{}{"messages_synced": n})
			} else {
				fmt.Fprintf(os.Stderr, "\rSynced %d messages...", n)
			}
		case *events.Connected:
			// Mark as unavailable so the user doesn't appear "online" 24/7
			// and the phone keeps receiving push notifications.
			_ = a.wa.SendPresence(ctx, types.PresenceUnavailable)
			if a.events.Enabled() {
				a.events.Emit("connected", nil)
			} else {
				fmt.Fprintln(os.Stderr, "\nConnected.")
			}
		case *events.Disconnected:
			if a.events.Enabled() {
				a.events.Emit("disconnected", nil)
			} else {
				fmt.Fprintln(os.Stderr, "\nDisconnected.")
			}
			select {
			case disconnected <- struct{}{}:
			default:
			}
		// Chat state events are best-effort metadata persistence inside the event
		// handler goroutine. Errors here must not interrupt the sync loop, so we
		// intentionally discard them with _ =.
		case *events.Archive:
			if v.Action != nil {
				_ = a.db.SetChatArchived(v.JID.String(), v.Action.GetArchived())
				if v.Action.GetArchived() {
					_ = a.db.SetChatPinned(v.JID.String(), false)
				}
			}
		case *events.Pin:
			if v.Action != nil {
				_ = a.db.SetChatPinned(v.JID.String(), v.Action.GetPinned())
			}
		case *events.Mute:
			if v.Action != nil {
				var mu int64
				if v.Action.GetMuted() {
					ms := v.Action.GetMuteEndTimestamp()
					switch {
					case ms == -1:
						mu = -1
					case ms > 0:
						mu = ms
					default:
						mu = -1
					}
				}
				_ = a.db.SetChatMutedUntil(v.JID.String(), mu)
			}
		case *events.MarkChatAsRead:
			if v.Action != nil {
				_ = a.db.SetChatUnread(v.JID.String(), !v.Action.GetRead())
			}
		}
	})
	defer a.wa.RemoveEventHandler(handlerID)

	if err := a.Connect(ctx, opts.AllowQR, opts.OnQRCode); err != nil {
		return SyncResult{}, err
	}

	if opts.DownloadMedia {
		var err error
		stopMedia, err = a.runMediaWorkers(ctx, mediaJobs, 4)
		if err != nil {
			return SyncResult{}, err
		}
		defer stopMedia()
	}

	// Optional: bootstrap imports (helps contacts/groups management without waiting for events).
	if opts.RefreshContacts {
		_ = a.refreshContacts(ctx)
	}
	if opts.RefreshGroups {
		_ = a.refreshGroups(ctx)
	}
	if opts.AfterConnect != nil {
		if err := opts.AfterConnect(ctx); err != nil {
			return SyncResult{MessagesStored: messagesStored.Load()}, err
		}
	}

	if opts.Mode == SyncModeFollow {
		for {
			select {
			case <-ctx.Done():
				if a.events.Enabled() {
					a.events.Emit("stopping", map[string]interface{}{"messages_synced": messagesStored.Load()})
				} else {
					fmt.Fprintln(os.Stderr, "\nStopping sync.")
				}
				return SyncResult{MessagesStored: messagesStored.Load()}, nil
			case <-disconnected:
				if a.events.Enabled() {
					a.events.Emit("reconnecting", nil)
				} else {
					fmt.Fprintln(os.Stderr, "Reconnecting...")
				}
				if err := a.wa.ReconnectWithBackoff(ctx, 2*time.Second, 30*time.Second); err != nil {
					return SyncResult{MessagesStored: messagesStored.Load()}, err
				}
			}
		}
	}

	// Bootstrap/once: exit after idle.
	poll := 250 * time.Millisecond
	if opts.IdleExit >= 2*time.Second {
		poll = 1 * time.Second
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if a.events.Enabled() {
				a.events.Emit("stopping", map[string]interface{}{"messages_synced": messagesStored.Load()})
			} else {
				fmt.Fprintln(os.Stderr, "\nStopping sync.")
			}
			return SyncResult{MessagesStored: messagesStored.Load()}, nil
		case <-disconnected:
			if a.events.Enabled() {
				a.events.Emit("reconnecting", nil)
			} else {
				fmt.Fprintln(os.Stderr, "Reconnecting...")
			}
			if err := a.wa.ReconnectWithBackoff(ctx, 2*time.Second, 30*time.Second); err != nil {
				return SyncResult{MessagesStored: messagesStored.Load()}, err
			}
		case <-ticker.C:
			last := time.Unix(0, lastEvent.Load())
			if time.Since(last) >= opts.IdleExit {
				if a.events.Enabled() {
					a.events.Emit("idle_exit", map[string]interface{}{
						"idle_duration":  opts.IdleExit.String(),
						"messages_synced": messagesStored.Load(),
					})
				} else {
					fmt.Fprintf(os.Stderr, "\nIdle for %s, exiting.\n", opts.IdleExit)
				}
				return SyncResult{MessagesStored: messagesStored.Load()}, nil
			}
		}
	}
}

func chatKind(chat types.JID) string {
	if chat.Server == types.GroupServer {
		return "group"
	}
	if chat.IsBroadcastList() {
		return "broadcast"
	}
	if chat.Server == types.DefaultUserServer {
		return "dm"
	}
	return "unknown"
}

func (a *App) storeParsedMessage(ctx context.Context, pm wa.ParsedMessage) error {
	// Resolve LID chat JIDs to Phone Number JIDs so messages are stored
	// under the canonical PN-based chat instead of a separate LID chat.
	pm.Chat = a.wa.ResolveLIDToPN(ctx, pm.Chat)

	chatJID := pm.Chat.ToNonAD().String()
	chatName := a.wa.ResolveChatName(ctx, pm.Chat, pm.PushName)
	if err := a.db.UpsertChat(chatJID, chatKind(pm.Chat), chatName, pm.Timestamp); err != nil {
		return err
	}

	// Best-effort: store contact info for DMs. Errors are intentionally
	// discarded (_ =) because this runs inside the sync event loop and
	// must not block message storage.
	if pm.Chat.Server == types.DefaultUserServer {
		normalizedChat := pm.Chat.ToNonAD()
		if info, err := a.wa.GetContact(ctx, normalizedChat); err == nil {
			_ = a.db.UpsertContact(
				normalizedChat.String(),
				normalizedChat.User,
				info.PushName,
				info.FullName,
				info.FirstName,
				info.BusinessName,
			)
		}
	}

	senderName := ""
	senderJID := ""
	if pm.FromMe {
		senderName = "me"
	} else if s := strings.TrimSpace(pm.PushName); s != "" && s != "-" {
		senderName = s
	}
	if pm.SenderJID != "" {
		if jid, err := types.ParseJID(pm.SenderJID); err == nil {
			normalizedJID := jid.ToNonAD()
			senderJID = normalizedJID.String()
			if info, err := a.wa.GetContact(ctx, normalizedJID); err == nil {
				if name := wa.BestContactName(info); name != "" {
					senderName = name
				}
				_ = a.db.UpsertContact(
					normalizedJID.String(),
					normalizedJID.User,
					info.PushName,
					info.FullName,
					info.FirstName,
					info.BusinessName,
				)
			}
		}
	}

	// Best-effort: store group metadata (and participants) when available.
	if pm.Chat.Server == types.GroupServer {
		if gi, err := a.wa.GetGroupInfo(ctx, pm.Chat); err == nil && gi != nil {
			normalizedChat := pm.Chat.ToNonAD()
			_ = a.db.UpsertGroup(normalizedChat.String(), gi.GroupName.Name, gi.OwnerJID.String(), gi.GroupCreated)
			var ps []store.GroupParticipant
			for _, p := range gi.Participants {
				role := "member"
				if p.IsSuperAdmin {
					role = "superadmin"
				} else if p.IsAdmin {
					role = "admin"
				}
				ps = append(ps, store.GroupParticipant{
					GroupJID: normalizedChat.String(),
					UserJID:  p.JID.ToNonAD().String(),
					Role:     role,
				})
			}
			_ = a.db.ReplaceGroupParticipants(normalizedChat.String(), ps)
		}
	}

	var mediaType, caption, filename, mimeType, directPath string
	var mediaKey, fileSha, fileEncSha []byte
	var fileLen uint64
	if pm.Media != nil {
		mediaType = pm.Media.Type
		caption = pm.Media.Caption
		filename = pm.Media.Filename
		mimeType = pm.Media.MimeType
		directPath = pm.Media.DirectPath
		mediaKey = pm.Media.MediaKey
		fileSha = pm.Media.FileSHA256
		fileEncSha = pm.Media.FileEncSHA256
		fileLen = pm.Media.FileLength
	}

	displayText := a.buildDisplayText(ctx, pm)

	return a.db.UpsertMessage(store.UpsertMessageParams{
		ChatJID:       chatJID,
		ChatName:      chatName,
		MsgID:         pm.ID,
		SenderJID:     senderJID,
		SenderName:    senderName,
		Timestamp:     pm.Timestamp,
		FromMe:        pm.FromMe,
		Text:          pm.Text,
		DisplayText:   displayText,
		MediaType:     mediaType,
		MediaCaption:  caption,
		Filename:      filename,
		MimeType:      mimeType,
		DirectPath:    directPath,
		MediaKey:      mediaKey,
		FileSHA256:    fileSha,
		FileEncSHA256: fileEncSha,
		FileLength:    fileLen,
		ReactionToID:  pm.ReactionToID,
		ReactionEmoji: pm.ReactionEmoji,
	})
}

func (a *App) buildDisplayText(ctx context.Context, pm wa.ParsedMessage) string {
	base := baseDisplayText(pm)

	if pm.ReactionToID != "" || strings.TrimSpace(pm.ReactionEmoji) != "" {
		target := strings.TrimSpace(pm.ReactionToID)
		display := ""
		if target != "" {
			display = a.lookupMessageDisplayText(pm.Chat.String(), target)
		}
		if display == "" {
			display = "message"
		}
		emoji := strings.TrimSpace(pm.ReactionEmoji)
		if emoji != "" {
			return fmt.Sprintf("Reacted %s to %s", emoji, display)
		}
		return fmt.Sprintf("Reacted to %s", display)
	}

	if pm.ReplyToID != "" {
		quoted := strings.TrimSpace(pm.ReplyToDisplay)
		if quoted == "" {
			quoted = a.lookupMessageDisplayText(pm.Chat.String(), pm.ReplyToID)
		}
		if quoted == "" {
			quoted = "message"
		}
		if base == "" {
			base = "(message)"
		}
		return fmt.Sprintf("> %s\n%s", quoted, base)
	}

	if base == "" {
		base = "(message)"
	}
	return base
}

func baseDisplayText(pm wa.ParsedMessage) string {
	if pm.Media != nil {
		return "Sent " + mediaLabel(pm.Media.Type)
	}
	if text := strings.TrimSpace(pm.Text); text != "" {
		return text
	}
	return ""
}

func (a *App) lookupMessageDisplayText(chatJID, msgID string) string {
	if strings.TrimSpace(chatJID) == "" || strings.TrimSpace(msgID) == "" {
		return ""
	}
	msg, err := a.db.GetMessage(chatJID, msgID)
	if err != nil {
		return ""
	}
	if text := strings.TrimSpace(msg.DisplayText); text != "" {
		return text
	}
	if text := strings.TrimSpace(msg.Text); text != "" {
		return text
	}
	if strings.TrimSpace(msg.MediaType) != "" {
		return "Sent " + mediaLabel(msg.MediaType)
	}
	return ""
}

func mediaLabel(mediaType string) string {
	mt := strings.ToLower(strings.TrimSpace(mediaType))
	switch mt {
	case "gif":
		return "gif"
	case "image":
		return "image"
	case "video":
		return "video"
	case "audio":
		return "audio"
	case "sticker":
		return "sticker"
	case "document":
		return "document"
	case "location":
		return "location"
	case "contact":
		return "contact"
	case "contacts":
		return "contacts"
	case "":
		return "message"
	default:
		return mt
	}
}
