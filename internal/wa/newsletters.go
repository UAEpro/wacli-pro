package wa

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow/types"
)

func (c *Client) GetSubscribedNewsletters(ctx context.Context) ([]*types.NewsletterMetadata, error) {
	c.mu.Lock()
	cli := c.client
	c.mu.Unlock()
	if cli == nil || !cli.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	return cli.GetSubscribedNewsletters(ctx)
}

func (c *Client) GetNewsletterInfo(ctx context.Context, jid types.JID) (*types.NewsletterMetadata, error) {
	c.mu.Lock()
	cli := c.client
	c.mu.Unlock()
	if cli == nil || !cli.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	return cli.GetNewsletterInfo(ctx, jid)
}

func (c *Client) GetNewsletterInfoWithInvite(ctx context.Context, key string) (*types.NewsletterMetadata, error) {
	c.mu.Lock()
	cli := c.client
	c.mu.Unlock()
	if cli == nil || !cli.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	return cli.GetNewsletterInfoWithInvite(ctx, key)
}

func (c *Client) FollowNewsletter(ctx context.Context, jid types.JID) error {
	c.mu.Lock()
	cli := c.client
	c.mu.Unlock()
	if cli == nil || !cli.IsConnected() {
		return fmt.Errorf("not connected")
	}
	return cli.FollowNewsletter(ctx, jid)
}

func (c *Client) UnfollowNewsletter(ctx context.Context, jid types.JID) error {
	c.mu.Lock()
	cli := c.client
	c.mu.Unlock()
	if cli == nil || !cli.IsConnected() {
		return fmt.Errorf("not connected")
	}
	return cli.UnfollowNewsletter(ctx, jid)
}

func (c *Client) NewsletterToggleMute(ctx context.Context, jid types.JID, mute bool) error {
	c.mu.Lock()
	cli := c.client
	c.mu.Unlock()
	if cli == nil || !cli.IsConnected() {
		return fmt.Errorf("not connected")
	}
	return cli.NewsletterToggleMute(ctx, jid, mute)
}
