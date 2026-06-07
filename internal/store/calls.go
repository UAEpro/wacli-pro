package store

import "time"

type CallEvent struct {
	ChatJID   string    `json:"chat_jid"`
	CallerJID string    `json:"caller_jid"`
	CallID    string    `json:"call_id"`
	Type      string    `json:"type"` // "voice" or "video"
	Timestamp time.Time `json:"timestamp"`
}

func (d *DB) InsertCallEvent(c CallEvent) error {
	_, err := d.sql.Exec(
		`INSERT OR IGNORE INTO call_events (chat_jid, caller_jid, call_id, type, ts) VALUES (?, ?, ?, ?, ?)`,
		c.ChatJID, c.CallerJID, c.CallID, c.Type, c.Timestamp.UTC().Unix(),
	)
	return err
}

func (d *DB) ListCallEvents(limit int) ([]CallEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.sql.Query(`SELECT chat_jid, caller_jid, call_id, type, ts FROM call_events ORDER BY ts DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []CallEvent
	for rows.Next() {
		var c CallEvent
		var ts int64
		if err := rows.Scan(&c.ChatJID, &c.CallerJID, &c.CallID, &c.Type, &ts); err != nil {
			return nil, err
		}
		c.Timestamp = time.Unix(ts, 0).UTC()
		events = append(events, c)
	}
	return events, rows.Err()
}
