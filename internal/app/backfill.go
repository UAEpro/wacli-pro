package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type BackfillOptions struct {
	ChatJID        string
	Count          int
	Requests       int
	WaitPerRequest time.Duration
	IdleExit       time.Duration
}

type BackfillResult struct {
	ChatJID        string
	RequestsSent   int
	ResponsesSeen  int
	MessagesAdded  int64
	MessagesSynced int64
}

type onDemandResponse struct {
	conversations int
	messages      int
	endType       waHistorySync.Conversation_EndOfHistoryTransferType
}

// prepBackfill validates options, applies defaults, and ensures the client is
// authed and initialized. Returns the parsed chat JID and its canonical string.
func (a *App) prepBackfill(opts *BackfillOptions) (types.JID, string, error) {
	chatStr := strings.TrimSpace(opts.ChatJID)
	if chatStr == "" {
		return types.JID{}, "", fmt.Errorf("--chat is required")
	}
	chat, err := types.ParseJID(chatStr)
	if err != nil {
		return types.JID{}, "", fmt.Errorf("parse chat JID: %w", err)
	}
	chatStr = chat.String()

	if opts.Count <= 0 {
		opts.Count = 50
	}
	if opts.Requests <= 0 {
		opts.Requests = 1
	}
	if opts.WaitPerRequest <= 0 {
		opts.WaitPerRequest = 60 * time.Second
	}
	if opts.IdleExit <= 0 {
		opts.IdleExit = 5 * time.Second
	}

	if err := a.EnsureAuthed(); err != nil {
		return types.JID{}, "", err
	}
	if err := a.OpenWA(); err != nil {
		return types.JID{}, "", err
	}
	return chat, chatStr, nil
}

// runBackfillRequests registers a temporary on-demand history handler and issues
// the configured number of on-demand requests over the *already-connected*
// client, waiting for each response. It does not connect or disconnect, so it is
// safe to run both inside a one-shot Sync (BackfillHistory) and against a live
// connection held by the sync daemon (BackfillOnConnected).
func (a *App) runBackfillRequests(ctx context.Context, chat types.JID, chatStr string, opts BackfillOptions) (int, int, error) {
	var mu sync.Mutex
	var waitCh chan onDemandResponse
	handlerID := a.wa.AddEventHandler(func(evt interface{}) {
		hs, ok := evt.(*events.HistorySync)
		if !ok || hs == nil || hs.Data == nil {
			return
		}
		if hs.Data.GetSyncType() != waHistorySync.HistorySync_ON_DEMAND {
			return
		}

		for _, conv := range hs.Data.GetConversations() {
			if strings.TrimSpace(conv.GetID()) != chatStr {
				continue
			}
			mu.Lock()
			ch := waitCh
			mu.Unlock()
			if ch == nil {
				return
			}
			resp := onDemandResponse{
				conversations: len(hs.Data.GetConversations()),
				messages:      len(conv.GetMessages()),
				endType:       conv.GetEndOfHistoryTransferType(),
			}
			select {
			case ch <- resp:
			default:
			}
			return
		}
	})
	defer a.wa.RemoveEventHandler(handlerID)

	var requestsSent, responsesSeen int
	for i := 0; i < opts.Requests; i++ {
		oldest, err := a.db.GetOldestMessageInfo(chatStr)
		if err != nil {
			if err == sql.ErrNoRows {
				return requestsSent, responsesSeen, fmt.Errorf("no messages for %s in local DB; run `wacli sync` first", chatStr)
			}
			return requestsSent, responsesSeen, err
		}

		reqInfo := types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat:     chat,
				IsFromMe: oldest.FromMe,
			},
			ID:        types.MessageID(oldest.MsgID),
			Timestamp: oldest.Timestamp,
		}

		ch := make(chan onDemandResponse, 4)
		mu.Lock()
		waitCh = ch
		mu.Unlock()

		requestsSent++
		fmt.Fprintf(os.Stderr, "Requesting %d older messages for %s...\n", opts.Count, chatStr)
		if _, err := a.wa.RequestHistorySyncOnDemand(ctx, reqInfo, opts.Count); err != nil {
			return requestsSent, responsesSeen, err
		}

		var resp onDemandResponse
		select {
		case <-ctx.Done():
			return requestsSent, responsesSeen, ctx.Err()
		case resp = <-ch:
			responsesSeen++
		case <-time.After(opts.WaitPerRequest):
			return requestsSent, responsesSeen, fmt.Errorf("timed out waiting for on-demand history sync response")
		}

		mu.Lock()
		if waitCh == ch {
			waitCh = nil
		}
		mu.Unlock()

		fmt.Fprintf(os.Stderr, "On-demand history sync: %d conversations, %d messages.\n", resp.conversations, resp.messages)

		newOldest, err := a.db.GetOldestMessageInfo(chatStr)
		if err == nil && newOldest.MsgID == oldest.MsgID {
			fmt.Fprintln(os.Stderr, "No older messages were added (stopping).")
			return requestsSent, responsesSeen, nil
		}
		if resp.messages <= 0 {
			fmt.Fprintln(os.Stderr, "No messages returned (stopping).")
			return requestsSent, responsesSeen, nil
		}
		if resp.endType == waHistorySync.Conversation_COMPLETE_AND_NO_MORE_MESSAGE_REMAIN_ON_PRIMARY {
			fmt.Fprintln(os.Stderr, "Reached start of chat history (stopping).")
			return requestsSent, responsesSeen, nil
		}
	}
	return requestsSent, responsesSeen, nil
}

// BackfillHistory requests older messages for a chat. It establishes its own
// one-shot connection (Sync) and is used when no sync daemon is running.
func (a *App) BackfillHistory(ctx context.Context, opts BackfillOptions) (BackfillResult, error) {
	chat, chatStr, err := a.prepBackfill(&opts)
	if err != nil {
		return BackfillResult{}, err
	}

	beforeCount, _ := a.db.CountMessages()

	var requestsSent, responsesSeen int
	syncRes, err := a.Sync(ctx, SyncOptions{
		Mode:     SyncModeOnce,
		AllowQR:  false,
		IdleExit: opts.IdleExit,
		AfterConnect: func(ctx context.Context) error {
			var e error
			requestsSent, responsesSeen, e = a.runBackfillRequests(ctx, chat, chatStr, opts)
			return e
		},
	})
	if err != nil {
		return BackfillResult{}, err
	}

	afterCount, _ := a.db.CountMessages()

	return BackfillResult{
		ChatJID:        chatStr,
		RequestsSent:   requestsSent,
		ResponsesSeen:  responsesSeen,
		MessagesAdded:  afterCount - beforeCount,
		MessagesSynced: syncRes.MessagesStored,
	}, nil
}

// BackfillOnConnected requests older messages using an already-live connection
// (e.g. the one held by the running sync daemon). It issues the on-demand
// requests directly without starting a second Sync; incoming messages are
// persisted by the daemon's existing sync handlers.
func (a *App) BackfillOnConnected(ctx context.Context, opts BackfillOptions) (BackfillResult, error) {
	chat, chatStr, err := a.prepBackfill(&opts)
	if err != nil {
		return BackfillResult{}, err
	}

	beforeCount, _ := a.db.CountMessages()

	requestsSent, responsesSeen, err := a.runBackfillRequests(ctx, chat, chatStr, opts)
	if err != nil {
		return BackfillResult{}, err
	}

	afterCount, _ := a.db.CountMessages()

	return BackfillResult{
		ChatJID:        chatStr,
		RequestsSent:   requestsSent,
		ResponsesSeen:  responsesSeen,
		MessagesAdded:  afterCount - beforeCount,
		MessagesSynced: afterCount - beforeCount,
	}, nil
}
