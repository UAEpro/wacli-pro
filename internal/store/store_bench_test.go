package store

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkUpsertMessage(b *testing.B) {
	db := openBenchDB(b)
	chat := "bench@s.whatsapp.net"
	if err := db.UpsertChat(chat, "dm", "Bench", time.Now()); err != nil {
		b.Fatal(err)
	}
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := db.UpsertMessage(UpsertMessageParams{
			ChatJID:    chat,
			ChatName:   "Bench",
			MsgID:      fmt.Sprintf("msg-%d", i),
			SenderJID:  "sender@s.whatsapp.net",
			SenderName: "Alice",
			Timestamp:  base.Add(time.Duration(i) * time.Second),
			FromMe:     i%2 == 0,
			Text:       fmt.Sprintf("benchmark message number %d with some content", i),
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchMessages(b *testing.B) {
	db := openBenchDB(b)
	chat := "bench@s.whatsapp.net"
	if err := db.UpsertChat(chat, "dm", "Bench", time.Now()); err != nil {
		b.Fatal(err)
	}
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Seed 10K messages.
	for i := 0; i < 10000; i++ {
		_ = db.UpsertMessage(UpsertMessageParams{
			ChatJID:    chat,
			ChatName:   "Bench",
			MsgID:      fmt.Sprintf("msg-%d", i),
			SenderJID:  "sender@s.whatsapp.net",
			SenderName: "Alice",
			Timestamp:  base.Add(time.Duration(i) * time.Second),
			Text:       fmt.Sprintf("message %d about project meeting", i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.SearchMessages(SearchMessagesParams{
			Query: "meeting",
			Limit: 50,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListChats(b *testing.B) {
	db := openBenchDB(b)

	for i := 0; i < 1000; i++ {
		jid := fmt.Sprintf("%d@s.whatsapp.net", 1000000000+i)
		if err := db.UpsertChat(jid, "dm", fmt.Sprintf("User %d", i), time.Now().Add(-time.Duration(i)*time.Hour)); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.ListChats(ChatListFilter{Limit: 50})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestPerformanceIndexesExist(t *testing.T) {
	db := openTestDB(t)

	indexes := []string{
		"idx_chats_kind",
		"idx_contacts_push_name",
		"idx_groups_name",
	}
	for _, idx := range indexes {
		exists, err := db.indexExists(idx)
		if err != nil {
			t.Fatalf("check index %s: %v", idx, err)
		}
		if !exists {
			t.Fatalf("expected index %s to exist after migration v8", idx)
		}
	}
}

func (d *DB) indexExists(name string) (bool, error) {
	row := d.sql.QueryRow(`SELECT 1 FROM sqlite_master WHERE type='index' AND name=?`, name)
	var one int
	err := row.Scan(&one)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func openBenchDB(b *testing.B) *DB {
	b.Helper()
	dir := b.TempDir()
	db, err := Open(dir + "/bench.db")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = db.Close() })
	return db
}
