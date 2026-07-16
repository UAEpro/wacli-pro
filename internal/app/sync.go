package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/UAEpro/wacli-pro/internal/store"
	"github.com/UAEpro/wacli-pro/internal/wa"
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
	IdleExit        time.Duration                                                        // only used for bootstrap/once
	MaxDuration     time.Duration                                                        // hard timeout for bootstrap/once (0 = no limit)
	Verbosity       int                                                                  // future
	MaxMessages     int64                                                                // stop syncing if total messages exceed this (0 = unlimited)
	MaxDBSize       int64                                                                // stop syncing if DB file exceeds this size in bytes (0 = unlimited)
	OnMessage       func(ctx context.Context, event string, data map[string]interface{}) // optional callback for each new message
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

	// Wrap context with a cancel so we can stop on limits.
	ctx, limitCancel := context.WithCancel(ctx)
	defer limitCancel()

	if err := a.OpenWA(); err != nil {
		return SyncResult{}, err
	}

	var messagesStored atomic.Int64
	lastEvent := atomic.Int64{}
	lastEvent.Store(time.Now().UTC().UnixNano())

	disconnected := make(chan struct{}, 1)

	// checkLimits returns true if any storage limit has been exceeded.
	checkLimits := func() bool {
		if opts.MaxMessages > 0 && messagesStored.Load() >= opts.MaxMessages {
			if a.events.Enabled() {
				a.events.Emit("limit_reached", map[string]interface{}{"reason": "max_messages", "count": messagesStored.Load()})
			} else {
				fmt.Fprintf(os.Stderr, "\nMax messages limit reached (%d), stopping.\n", opts.MaxMessages)
			}
			limitCancel()
			return true
		}
		if opts.MaxDBSize > 0 {
			if info, err := os.Stat(a.db.Path()); err == nil {
				if info.Size() >= opts.MaxDBSize {
					if a.events.Enabled() {
						a.events.Emit("limit_reached", map[string]interface{}{"reason": "max_db_size", "size": info.Size()})
					} else {
						fmt.Fprintf(os.Stderr, "\nMax DB size limit reached (%d bytes), stopping.\n", info.Size())
					}
					limitCancel()
					return true
				}
			}
		}
		return false
	}

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

	// procCtx backs message processing (decryption, LID/name resolution, DB
	// writes). It deliberately survives ctx cancellation: whatsmeow has
	// already acked queued events to the server, so the shutdown drain must
	// still process them with full enrichment or their data is degraded
	// permanently. To keep shutdown itself bounded, cancellation of ctx arms
	// a grace period after which procCtx is canceled too — network enrichment
	// then fails fast while the remaining drain reduces to local DB writes.
	procCtx, procCancel := context.WithCancel(context.WithoutCancel(ctx))
	defer procCancel()
	go func() {
		select {
		case <-procCtx.Done():
			return
		case <-ctx.Done():
		}
		select {
		case <-procCtx.Done():
		case <-time.After(15 * time.Second):
			procCancel()
		}
	}()

	// countStored bumps the counter and emits progress once per 25 messages.
	countStored := func() {
		n := messagesStored.Add(1)
		if n%25 == 0 {
			if a.events.Enabled() {
				a.events.Emit("progress", map[string]interface{}{"messages_synced": n})
			} else {
				fmt.Fprintf(os.Stderr, "\rSynced %d messages...", n)
			}
		}
	}

	processMessage := func(v *events.Message) {
		if chatJID, msgID, isRevoke := wa.RevokeInfo(v); isRevoke {
			if err := a.db.MarkRevoked(chatJID, msgID); err == nil {
				countStored()
			}
		} else if editPM, isEdit := wa.ParseEditEvent(v); isEdit {
			if err := a.storeParsedMessage(procCtx, editPM); err == nil {
				countStored()
			}
		} else {
			pm := wa.ParseLiveMessage(v)
			if pm.ReactionToID != "" && pm.ReactionEmoji == "" && v.Message != nil && v.Message.GetEncReactionMessage() != nil {
				if reaction, err := a.wa.DecryptReaction(procCtx, v); err == nil && reaction != nil {
					pm.ReactionEmoji = reaction.GetText()
					if pm.ReactionToID == "" {
						if key := reaction.GetKey(); key != nil {
							pm.ReactionToID = key.GetID()
						}
					}
				}
			}
			if err := a.storeParsedMessage(procCtx, pm); err == nil {
				countStored()
			}
			{
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
				if a.events.Enabled() {
					a.events.Emit("new_message", data)
				}
				if opts.OnMessage != nil {
					opts.OnMessage(procCtx, "new_message", data)
				}
			}
			if opts.DownloadMedia && pm.Media != nil && pm.ID != "" {
				enqueueMedia(pm.Chat.String(), pm.ID)
			}
		}
		checkLimits()
	}

	processHistorySync := func(v *events.HistorySync) {
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
				if err := a.storeParsedMessage(procCtx, pm); err == nil {
					countStored()
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
		checkLimits()
	}

	// Messages and history syncs are processed on a dedicated worker goroutine
	// instead of inside the whatsmeow event handler. whatsmeow dispatches events
	// from a single node-handling loop; blocking it (DB writes, decrypts,
	// metadata fetches) delays receipts/acks for every subsequent message, which
	// makes the phone retry, re-encrypt, and resend — the main source of phone
	// lag. The handler now only enqueues, so the event loop stays drained.
	processQueued := func(evt interface{}) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "recovered panic in sync worker: %v\n", r)
			}
		}()
		switch v := evt.(type) {
		case *events.Message:
			processMessage(v)
		case *events.HistorySync:
			processHistorySync(v)
		case *events.CallOffer:
			callType := "voice"
			_ = a.db.InsertCallEvent(store.CallEvent{
				ChatJID:   v.CallCreator.String(),
				CallerJID: v.CallCreator.String(),
				CallID:    v.CallID,
				Type:      callType,
				Timestamp: v.Timestamp,
			})
		// Chat state updates ride the same queue as messages so they cannot
		// run before the message that creates their chat row. Errors are
		// best-effort metadata persistence and intentionally discarded.
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
		lastEvent.Store(time.Now().UTC().UnixNano())
	}

	// Trade-offs, chosen deliberately:
	//   - Queued events are processed even past MaxMessages/MaxDBSize: those
	//     limits are soft bounds, and whatsmeow has already acked the events,
	//     so dropping them would lose them permanently. Overshoot is bounded
	//     by the queue.
	//   - A crash between ack and commit loses whatever is queued. whatsmeow
	//     acks before event handlers run even in fully synchronous mode, so
	//     this window predates the queue — the queue only widens it. A
	//     durable event journal would close it entirely.
	queue := make(chan interface{}, 4096)
	drainReq := make(chan struct{})
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		// Queued events are ALWAYS processed, even on a hard cancel
		// (Ctrl-C, MaxDuration, storage limits): whatsmeow has already acked
		// them to the server, so dropping them here would lose them
		// permanently. With a canceled ctx the network enrichment inside
		// storeParsedMessage fails fast, so a shutdown drain reduces to
		// bounded local DB writes.
		drain := func() {
			for {
				select {
				case evt := <-queue:
					processQueued(evt)
				default:
					return
				}
			}
		}
		for {
			select {
			case evt := <-queue:
				processQueued(evt)
			case <-ctx.Done():
				drain()
				return
			case <-drainReq:
				// Graceful exits (idle exit, connect errors) drain with the
				// context still live so enrichment stays intact.
				drain()
				return
			}
		}
	}()

	var handlerWG sync.WaitGroup
	handlerID := a.wa.AddEventHandler(func(evt interface{}) {
		handlerWG.Add(1)
		defer handlerWG.Done()
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "recovered panic in event handler: %v\n", r)
			}
		}()
		lastEvent.Store(time.Now().UTC().UnixNano())

		switch evt.(type) {
		case *events.Message, *events.HistorySync, *events.CallOffer,
			*events.Archive, *events.Pin, *events.Mute, *events.MarkChatAsRead:
			// Never drop: whatsmeow acks the event once this handler returns,
			// so a dropped event is lost permanently. If the queue is full
			// this blocks (backpressure); if the worker has already exited
			// (shutdown drain finished), process inline like the old
			// synchronous path did.
			select {
			case queue <- evt:
			case <-workerDone:
				processQueued(evt)
			}
		case *events.Connected:
			// Mark as unavailable so the user doesn't appear "online" 24/7
			// and the phone keeps receiving push notifications.
			if err := a.wa.SendPresence(ctx, types.PresenceUnavailable); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to set presence unavailable: %v\n", err)
			} else {
				fmt.Fprintln(os.Stderr, "Presence set to unavailable.")
			}
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
		}
	})
	defer a.wa.RemoveEventHandler(handlerID)

	// finish stops the pipeline in order — producer first (unregister, then
	// wait out in-flight handler invocations), then the worker, which drains
	// the queue — so no acked event can slip in behind the drain and be
	// silently dropped. It must be called exactly once, on every return path.
	finish := func(err error) (SyncResult, error) {
		a.wa.RemoveEventHandler(handlerID)
		handlerWG.Wait()
		close(drainReq)
		<-workerDone
		// Belt and braces: consume anything a pathologically late handler
		// invocation buffered after the worker's final empty-check.
		for {
			select {
			case evt := <-queue:
				processQueued(evt)
			default:
				limitCancel()
				return SyncResult{MessagesStored: messagesStored.Load()}, err
			}
		}
	}

	if err := a.Connect(ctx, opts.AllowQR, opts.OnQRCode); err != nil {
		return finish(err)
	}

	if opts.DownloadMedia {
		var err error
		stopMedia, err = a.runMediaWorkers(ctx, mediaJobs, 4)
		if err != nil {
			return finish(err)
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
			return finish(err)
		}
	}

	emitStopping := func(res SyncResult) {
		if a.events.Enabled() {
			a.events.Emit("stopping", map[string]interface{}{"messages_synced": res.MessagesStored})
		} else {
			fmt.Fprintln(os.Stderr, "\nStopping sync.")
		}
	}

	if opts.Mode == SyncModeFollow {
		for {
			select {
			case <-ctx.Done():
				res, err := finish(nil)
				emitStopping(res)
				return res, err
			case <-disconnected:
				if a.events.Enabled() {
					a.events.Emit("reconnecting", nil)
				} else {
					fmt.Fprintln(os.Stderr, "Reconnecting...")
				}
				if err := a.wa.ReconnectWithBackoff(ctx, 2*time.Second, 30*time.Second); err != nil {
					return finish(err)
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
			res, err := finish(nil)
			emitStopping(res)
			return res, err
		case <-disconnected:
			if a.events.Enabled() {
				a.events.Emit("reconnecting", nil)
			} else {
				fmt.Fprintln(os.Stderr, "Reconnecting...")
			}
			if err := a.wa.ReconnectWithBackoff(ctx, 2*time.Second, 30*time.Second); err != nil {
				return finish(err)
			}
		case <-ticker.C:
			last := time.Unix(0, lastEvent.Load())
			if time.Since(last) >= opts.IdleExit {
				res, err := finish(nil)
				if a.events.Enabled() {
					a.events.Emit("idle_exit", map[string]interface{}{
						"idle_duration":   opts.IdleExit.String(),
						"messages_synced": res.MessagesStored,
					})
				} else {
					fmt.Fprintf(os.Stderr, "\nIdle for %s, exiting.\n", opts.IdleExit)
				}
				return res, err
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

	normalizedChat := pm.Chat.ToNonAD()
	chatJID := normalizedChat.String()

	// Group/contact metadata is fetched and persisted through TTL caches so a
	// burst of messages in one chat costs a single lookup, not one per message.
	chatName := ""
	switch {
	case pm.Chat.Server == types.GroupServer || pm.Chat.IsBroadcastList():
		chatName = a.cachedGroupName(ctx, normalizedChat)
	case pm.Chat.Server == types.DefaultUserServer:
		chatName = a.cachedContactName(ctx, normalizedChat)
	}
	if chatName == "" {
		// The push name belongs to the sender, so it only names a DM chat
		// when the peer (not us) sent the message. Everything else falls back
		// to the JID so first-time chats stay labeled (and searchable by
		// number) in chat_name.
		if s := strings.TrimSpace(pm.PushName); s != "" && s != "-" && !pm.FromMe && pm.Chat.Server == types.DefaultUserServer {
			chatName = s
		} else {
			chatName = chatJID
		}
	}
	if err := a.db.UpsertChat(chatJID, chatKind(pm.Chat), chatName, pm.Timestamp); err != nil {
		return err
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
			if !pm.FromMe {
				if name := a.cachedContactName(ctx, normalizedJID); name != "" {
					senderName = name
				}
			}
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
