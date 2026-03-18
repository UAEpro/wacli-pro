package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/UAEpro/wacli-pro/internal/store"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// TestIntegrationFullLifecycle exercises the core lifecycle:
// create App → inject events → verify DB → send → verify stored.
func TestIntegrationFullLifecycle(t *testing.T) {
	dir := t.TempDir()
	fw := newFakeWA()

	a, err := New(Options{
		StoreDir: dir,
		Version:  "test",
		Events:   out.NewEventWriter(nil, false),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer a.Close()

	// Inject fake WA client.
	a.wa = fw
	a.waOnce.Do(func() {})

	// Verify DB opens correctly.
	dbPath := filepath.Join(dir, "wacli.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open DB: %v", err)
	}
	_ = db.Close()

	// Set up contacts and groups in the fake client.
	contactJID := types.JID{User: "111", Server: types.DefaultUserServer}
	fw.contacts[contactJID] = types.ContactInfo{
		Found:    true,
		PushName: "Alice",
		FullName: "Alice Smith",
	}
	groupJID := types.JID{User: "group1", Server: types.GroupServer}
	fw.groups[groupJID] = &types.GroupInfo{
		JID:          groupJID,
		GroupName:     types.GroupName{Name: "Test Group"},
		GroupCreated:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Participants:  []types.GroupParticipant{{JID: contactJID}},
	}

	// Schedule events to be emitted on connect.
	text1 := "hello world"
	text2 := "group message"
	fw.connectEvents = []interface{}{
		&events.Message{
			Info: types.MessageInfo{
				ID:        "msg1",
				Timestamp: time.Now().Add(-5 * time.Minute),
				MessageSource: types.MessageSource{
					Chat:   contactJID,
					Sender: contactJID,
				},
				PushName: "Alice",
			},
			Message: &waProto.Message{Conversation: proto.String(text1)},
		},
		&events.Message{
			Info: types.MessageInfo{
				ID:        "msg2",
				Timestamp: time.Now().Add(-3 * time.Minute),
				MessageSource: types.MessageSource{
					Chat:     groupJID,
					Sender:   contactJID,
					IsGroup:  true,
				},
				PushName: "Alice",
			},
			Message: &waProto.Message{Conversation: proto.String(text2)},
		},
	}

	// Run sync in once mode with short idle.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := a.Sync(ctx, SyncOptions{
		Mode:     SyncModeOnce,
		AllowQR:  false,
		IdleExit: 1 * time.Second,
	})
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if result.MessagesStored < 2 {
		t.Fatalf("expected at least 2 messages stored, got %d", result.MessagesStored)
	}

	// Verify messages in DB.
	m, err := a.DB().GetMessage(contactJID.String(), "msg1")
	if err != nil {
		t.Fatalf("GetMessage msg1: %v", err)
	}
	if m.Text != text1 {
		t.Fatalf("expected text %q, got %q", text1, m.Text)
	}

	m2, err := a.DB().GetMessage(groupJID.String(), "msg2")
	if err != nil {
		t.Fatalf("GetMessage msg2: %v", err)
	}
	if m2.Text != text2 {
		t.Fatalf("expected text %q, got %q", text2, m2.Text)
	}

	// Verify chat was created.
	c, err := a.DB().GetChat(contactJID.String())
	if err != nil {
		t.Fatalf("GetChat: %v", err)
	}
	if c.Kind != "dm" {
		t.Fatalf("expected chat kind dm, got %q", c.Kind)
	}

	// Test sending a message.
	msgID, err := a.WA().SendText(ctx, contactJID, "test reply")
	if err != nil {
		t.Fatalf("SendText: %v", err)
	}
	if msgID == "" {
		t.Fatal("expected non-empty message ID")
	}

	// Test sending a reaction.
	err = a.WA().SendReaction(ctx, contactJID, "msg1", "👍")
	if err != nil {
		t.Fatalf("SendReaction: %v", err)
	}

	// Test sending a location.
	locID, err := a.WA().SendLocation(ctx, contactJID, 25.2, 55.3, "Office", "Dubai")
	if err != nil {
		t.Fatalf("SendLocation: %v", err)
	}
	if locID == "" {
		t.Fatal("expected non-empty location message ID")
	}
}

// TestIntegrationRefreshContactsAndGroups verifies bootstrap imports.
func TestIntegrationRefreshContactsAndGroups(t *testing.T) {
	dir := t.TempDir()
	fw := newFakeWA()

	a, err := New(Options{
		StoreDir: dir,
		Version:  "test",
		Events:   out.NewEventWriter(nil, false),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	a.wa = fw
	a.waOnce.Do(func() {})

	jid1 := types.JID{User: "100", Server: types.DefaultUserServer}
	jid2 := types.JID{User: "200", Server: types.DefaultUserServer}
	fw.contacts[jid1] = types.ContactInfo{Found: true, PushName: "Bob"}
	fw.contacts[jid2] = types.ContactInfo{Found: true, PushName: "Charlie"}

	gid := types.JID{User: "g1", Server: types.GroupServer}
	fw.groups[gid] = &types.GroupInfo{
		JID:          gid,
		GroupName:     types.GroupName{Name: "Dev"},
		GroupCreated:  time.Now(),
		Participants:  []types.GroupParticipant{{JID: jid1}},
	}

	ctx := context.Background()
	if err := a.refreshContacts(ctx); err != nil {
		t.Fatalf("refreshContacts: %v", err)
	}
	if err := a.refreshGroups(ctx); err != nil {
		t.Fatalf("refreshGroups: %v", err)
	}

	// Verify contacts stored.
	found, err := a.DB().SearchContacts("Bob", 10)
	if err != nil {
		t.Fatalf("SearchContacts: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(found))
	}

	// Verify group stored.
	groups, err := a.DB().ListGroups("Dev", 10, false)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
}
