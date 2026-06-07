package wa

import (
	"context"
	"fmt"
)

// SetStatusMessage updates the user's "About" text.
func (c *Client) SetStatusMessage(ctx context.Context, msg string) error {
	c.mu.Lock()
	cli := c.client
	c.mu.Unlock()
	if cli == nil || !cli.IsConnected() {
		return fmt.Errorf("not connected")
	}
	return cli.SetStatusMessage(ctx, msg)
}

// SetProfilePhoto sets the user's profile picture. Pass nil to remove.
func (c *Client) SetProfilePhoto(ctx context.Context, avatar []byte) (string, error) {
	c.mu.Lock()
	cli := c.client
	c.mu.Unlock()
	if cli == nil || !cli.IsConnected() {
		return "", fmt.Errorf("not connected")
	}
	ownJID := cli.Store.ID.ToNonAD()
	return cli.SetGroupPhoto(ctx, ownJID, avatar)
}
