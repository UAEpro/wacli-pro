package app

import (
	"context"
	"sync"
	"time"

	"github.com/UAEpro/wacli-pro/internal/store"
	"github.com/UAEpro/wacli-pro/internal/wa"
	"go.mau.fi/whatsmeow/types"
)

// Name/metadata resolution used to happen per message: every group message
// triggered two GetGroupInfo network IQ queries (whatsmeow does not cache its
// public GetGroupInfo) plus a full participant-table rewrite, and every DM did
// uncached contact lookups + writes. During history syncs or busy groups this
// blocked whatsmeow's event loop, delaying receipts/acks and making the phone
// retry and re-encrypt messages. These TTL caches bound that work to one
// lookup + persist per chat/contact per TTL window.
const (
	groupCacheTTL    = 30 * time.Minute
	contactCacheTTL  = 15 * time.Minute
	negativeCacheTTL = 5 * time.Minute
)

type nameCacheEntry struct {
	name      string
	ok        bool // lookup succeeded (a failure is cached for negativeCacheTTL)
	fetchedAt time.Time
}

type resolveCache struct {
	mu sync.Mutex
	// gen increments on every invalidation. A fill started before an
	// invalidation carries a stale gen and is discarded on put, so a rename
	// or refresh can never be overwritten by an in-flight lookup that read
	// the pre-change state.
	gen      uint64
	groups   map[string]nameCacheEntry
	contacts map[string]nameCacheEntry
}

func freshEntry(m map[string]nameCacheEntry, key string, ttl time.Duration) (nameCacheEntry, bool) {
	e, found := m[key]
	if !found {
		return nameCacheEntry{}, false
	}
	maxAge := ttl
	if !e.ok {
		maxAge = negativeCacheTTL
	}
	if time.Since(e.fetchedAt) > maxAge {
		return nameCacheEntry{}, false
	}
	return e, true
}

func (c *resolveCache) getGroup(key string) (nameCacheEntry, bool, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := freshEntry(c.groups, key, groupCacheTTL)
	return e, ok, c.gen
}

func (c *resolveCache) putGroup(key string, e nameCacheEntry, gen uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gen != gen {
		return
	}
	if c.groups == nil {
		c.groups = map[string]nameCacheEntry{}
	}
	c.groups[key] = e
}

func (c *resolveCache) getContact(key string) (nameCacheEntry, bool, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := freshEntry(c.contacts, key, contactCacheTTL)
	return e, ok, c.gen
}

func (c *resolveCache) putContact(key string, e nameCacheEntry, gen uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gen != gen {
		return
	}
	if c.contacts == nil {
		c.contacts = map[string]nameCacheEntry{}
	}
	c.contacts[key] = e
}

// InvalidateNameCache drops the cached resolved name for a chat/contact JID so
// the next message re-fetches fresh metadata (e.g. after a group rename
// through the live daemon connection).
func (a *App) InvalidateNameCache(jid types.JID) {
	key := jid.ToNonAD().String()
	a.names.mu.Lock()
	delete(a.names.groups, key)
	delete(a.names.contacts, key)
	a.names.gen++
	a.names.mu.Unlock()
}

// InvalidateAllNameCaches clears every cached resolved name (e.g. after a bulk
// contacts/groups refresh).
func (a *App) InvalidateAllNameCaches() {
	a.names.mu.Lock()
	a.names.groups = nil
	a.names.contacts = nil
	a.names.gen++
	a.names.mu.Unlock()
}

// cachedGroupName returns the group's name, fetching group metadata over the
// network at most once per groupCacheTTL. On a successful fetch it also
// persists the group row and its participant list.
func (a *App) cachedGroupName(ctx context.Context, chat types.JID) string {
	key := chat.String()
	e, ok, gen := a.names.getGroup(key)
	if ok {
		return e.name
	}

	entry := nameCacheEntry{fetchedAt: time.Now()}
	gi, err := a.wa.GetGroupInfo(ctx, chat)
	if err != nil && ctx.Err() != nil {
		// Failure caused by shutdown, not by the group: don't poison the
		// cache with it.
		return ""
	}
	if err == nil && gi != nil {
		entry.ok = true
		entry.name = gi.GroupName.Name
		if chat.Server == types.GroupServer {
			_ = a.db.UpsertGroup(key, gi.GroupName.Name, gi.OwnerJID.String(), gi.GroupCreated)
			ps := make([]store.GroupParticipant, 0, len(gi.Participants))
			for _, p := range gi.Participants {
				role := "member"
				if p.IsSuperAdmin {
					role = "superadmin"
				} else if p.IsAdmin {
					role = "admin"
				}
				ps = append(ps, store.GroupParticipant{
					GroupJID: key,
					UserJID:  p.JID.ToNonAD().String(),
					Role:     role,
				})
			}
			_ = a.db.ReplaceGroupParticipants(key, ps)
		}
	}

	a.names.putGroup(key, entry, gen)
	return entry.name
}

// cachedContactName returns the contact's best display name, reading the
// whatsmeow contact store at most once per contactCacheTTL. On a successful
// lookup it also persists the contact row. jid must be normalized (ToNonAD).
func (a *App) cachedContactName(ctx context.Context, jid types.JID) string {
	key := jid.String()
	e, ok, gen := a.names.getContact(key)
	if ok {
		return e.name
	}

	entry := nameCacheEntry{fetchedAt: time.Now()}
	info, err := a.wa.GetContact(ctx, jid)
	if err != nil && ctx.Err() != nil {
		// Failure caused by shutdown, not by the contact: don't poison the
		// cache with it.
		return ""
	}
	if err == nil {
		entry.ok = true
		entry.name = wa.BestContactName(info)
		// Upsert even when the contact isn't in the address book (Found ==
		// false): the base row (JID + phone) is what `contacts search/show`
		// needs to list the sender at all.
		_ = a.db.UpsertContact(
			key,
			jid.User,
			info.PushName,
			info.FullName,
			info.FirstName,
			info.BusinessName,
		)
	}

	a.names.putContact(key, entry, gen)
	return entry.name
}
