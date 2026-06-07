package store

// StoreStats holds aggregate counts for the local database.
type StoreStats struct {
	Messages int64 `json:"messages"`
	Chats    int64 `json:"chats"`
	Contacts int64 `json:"contacts"`
	Groups   int64 `json:"groups"`
}

// Stats returns aggregate counts from the database.
func (d *DB) Stats() (StoreStats, error) {
	var s StoreStats
	row := d.sql.QueryRow("SELECT COUNT(*) FROM messages")
	if err := row.Scan(&s.Messages); err != nil {
		return s, err
	}
	row = d.sql.QueryRow("SELECT COUNT(*) FROM chats")
	if err := row.Scan(&s.Chats); err != nil {
		return s, err
	}
	row = d.sql.QueryRow("SELECT COUNT(*) FROM contacts")
	if err := row.Scan(&s.Contacts); err != nil {
		return s, err
	}
	row = d.sql.QueryRow("SELECT COUNT(*) FROM groups")
	if err := row.Scan(&s.Groups); err != nil {
		return s, err
	}
	return s, nil
}
