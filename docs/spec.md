# wacli-pro specification (plan)

This document defines the v1 plan for `wacli-pro`: a WhatsApp CLI that syncs messages locally, supports fast search, sending, and contact/group management. Implementation will use `whatsmeow` under the hood.

## Goals

- **Explicit authentication step**: `wacli-pro auth` shows a QR code and completes login.
- **Auth starts syncing immediately**: after successful QR pairing, `wacli-pro auth` begins initial sync (history + metadata).
- **Non-interactive sync**: `wacli-pro sync` never displays a QR code; it fails with a clear error if not authenticated.
- **Fast offline message search**: local SQLite + FTS5 index.
- **Human-first output**: readable tables by default, `--json` opt-in for scripting.
- **Single-instance safety**: store locking to avoid multi-instance session conflicts (device/session replacement issues).
- **Group management**: list groups, inspect, rename, manage participants, invites.

## Non-goals (v1)

- Guaranteed full-history export (WhatsApp/WhatsApp Web history is best-effort).
- End-to-end “contact creation” in WhatsApp (we can manage local aliases/notes; WhatsApp contacts are sourced from the account/device).
- Full message-type parity (polls, reactions, ephemeral nuances, etc.) in v1.

## Terminology

- **JID**: WhatsApp Jabber ID, e.g. `1234567890@s.whatsapp.net` (user) or `123456789@g.us` (group).
- **Store directory**: directory containing all local state, default `~/.wacli-pro`.

## Storage layout

Default store: `~/.wacli-pro` (override with `--store DIR`).

Proposed files:

- `~/.wacli-pro/session.db` — `whatsmeow` SQL store (device identity, keys, app-state).
- `~/.wacli-pro/wacli.db` — our SQLite DB (messages/chats, FTS, local metadata).
- `~/.wacli-pro/media/...` — downloaded media (optional, on-demand or background).
- `~/.wacli-pro/LOCK` — store lock to prevent concurrent access.

Rationale for two SQLite files: reduce coupling and keep the `whatsmeow`-owned schema separate from `wacli-pro`’s local schema. It’s still “one store directory” for the user.

## Concurrency + locking

Every command that accesses the WhatsApp session must acquire an exclusive lock in the store dir.

Behavior:

- If lock is held: fail fast with a clear message (include PID and start time if available).
- This prevents running multiple `wacli-pro` instances against the same WhatsApp device identity, which can cause disconnects or “device replaced” style failures.

## Authentication model

### Commands

- `wacli-pro auth` (interactive)
  - If not authenticated: connect, show QR code, wait for success.
  - After success: start initial sync (bootstrap) immediately.
  - Exits after initial sync “goes idle” (configurable), unless `--follow` is set.

- `wacli-pro sync` (non-interactive)
  - Requires an existing authenticated session in `session.db`.
  - Never displays QR; if not authenticated, prints “run `wacli-pro auth`”.
  - `--once` performs a bounded sync and exits.
  - Default (or `--follow`) stays connected and continues capturing messages.

### UX principle

Only `wacli-pro auth` is expected to show a QR code. `wacli-pro sync` should be safe to run in scripts/daemons without surprising interactivity.

## Sync model (best-effort)

`wacli-pro` captures messages via `whatsmeow` event handlers:

- `events.HistorySync`: initial/batch history sync delivered by WhatsApp Web.
- `events.Message`: new incoming/outgoing messages while connected.
- Connection lifecycle events (`Connected`, `Disconnected`) for logging/reconnect.

### Bootstrap sync (after auth)

Immediately after QR pairing success, `wacli-pro auth` runs a bootstrap sync:

- Processes history sync events and stores message metadata.
- Updates chats, names, and contact-derived names as available.
- Optionally starts media download worker (off by default, behind a flag).
- Exits once “idle for N seconds” (no new history events) unless `--follow`.

### Continuous sync

`wacli-pro sync --follow` keeps running:

- persists new messages as they arrive
- performs safe reconnect with backoff on disconnect
- continues best-effort history catch-up when WhatsApp emits it

## Database schema (wacli.db)

### Tables (proposed)

- `chats`
  - `jid` (PK), `name`, `kind` (`dm|group|broadcast`), `last_message_ts`, `archived`, `pinned`, `muted_until`, `unread`
- `contacts`
  - `jid` (PK), `push_name`, `full_name`, `business_name`, `phone`, …
- `groups`
  - `jid` (PK), `name`, `owner_jid`, `created_ts`, …
- `messages`
  - `rowid` (PK), `chat_jid`, `msg_id`, `sender_jid`, `ts`, `from_me`, `text`, `media_type`, `media_caption`, `filename`, `mime_type`, `direct_path`, hashes/keys, …
  - unique constraint: (`chat_jid`, `msg_id`)
- `contact_aliases` (local management)
  - `jid` (PK/FK), `alias`, `notes`, `tags` (or join table)

### Message search (FTS5)

Use SQLite **FTS5** for fast full-text search.

Approach:

- Maintain canonical data in `messages`.
- Maintain an FTS5 virtual table `messages_fts` (external content) indexing:
  - message body text
  - media caption
  - document filename
  - (optionally) denormalized sender/chat names for convenience

Query behavior:

- Default: `MATCH` queries (FTS syntax) with ranking via `bm25`.
- Filters implemented in SQL: `--chat`, `--from`, `--after`, `--before`, `--has-media`, `--type`.
- Human output includes snippets/highlights; `--json` returns structured matches + offsets/snippet string.

Fallback:

- If FTS5 is unavailable, fall back to `LIKE` with an explicit warning (slower).

## CLI command surface (v1)

Global flags:

- `--store DIR` (default `~/.wacli-pro`)
- `--json` (default: human text)
- `--timeout DURATION` (non-sync commands; e.g. `5m`)
- `--version` (prints version and exits)

### Doctor

- `wacli-pro doctor [--connect]`

### Auth

- `wacli-pro auth [--follow] [--idle-exit 30s]`
- `wacli-pro auth status`
- `wacli-pro auth logout`

### Sync

- `wacli-pro sync [--once] [--follow] [--download-media]`

Notes:

- `sync` errors if not authenticated (never prints QR).
- `--download-media` runs a bounded/concurrent media downloader for messages that contain downloadable media metadata.

### History backfill (best-effort)

WhatsApp Web history is best-effort. If you want to try fetching *older* messages for a specific chat, `wacli-pro` can send an on-demand history request to your primary device:

- `wacli-pro history backfill --chat JID [--count 50] [--requests N]`

### Messages

- `wacli-pro messages list [--chat JID] [--limit N] [--before TS] [--after TS]`
- `wacli-pro messages search <query> [--chat JID] [--from JID] [--limit N] [--before TS] [--after TS] [--type text|image|video|audio|document]`
- `wacli-pro messages show --chat JID --id MSG_ID`
- `wacli-pro messages context --chat JID --id MSG_ID [--before N] [--after N]`

### Send

- `wacli-pro send text --to PHONE_OR_JID --message TEXT`
- `wacli-pro send file --to PHONE_OR_JID --file PATH [--caption TEXT] [--mime TYPE]`

### Contacts (read + local management)

- `wacli-pro contacts search <query>`
- `wacli-pro contacts show --jid JID`
- `wacli-pro contacts refresh`
- `wacli-pro contacts alias set --jid JID --alias "Name"`
- `wacli-pro contacts alias rm --jid JID`
- `wacli-pro contacts tags add|rm --jid JID --tag TAG`

### Chats

- `wacli-pro chats list [--query TEXT] [--limit N] [--archived] [--no-archived] [--pinned] [--no-pinned] [--muted] [--no-muted] [--unread] [--no-unread]`
- `wacli-pro chats show --jid JID`
- `wacli-pro chats archive --jid JID`
- `wacli-pro chats unarchive --jid JID`
- `wacli-pro chats pin --jid JID`
- `wacli-pro chats unpin --jid JID`
- `wacli-pro chats mute --jid JID [--duration DURATION]`
- `wacli-pro chats unmute --jid JID`
- `wacli-pro chats mark-read --jid JID`
- `wacli-pro chats mark-unread --jid JID`

### Groups

- `wacli-pro groups list [--query TEXT]`
- `wacli-pro groups refresh`
- `wacli-pro groups info --jid GROUP_JID`
- `wacli-pro groups rename --jid GROUP_JID --name "New Name"`
- `wacli-pro groups participants add|remove --jid GROUP_JID --user PHONE_OR_JID [--user ...]`
- `wacli-pro groups participants promote|demote --jid GROUP_JID --user PHONE_OR_JID [--user ...]`
- `wacli-pro groups invite link get|revoke --jid GROUP_JID`
- `wacli-pro groups join --code INVITE_CODE`
- `wacli-pro groups leave --jid GROUP_JID`

## Output formats

Default: human-readable text (tables / aligned columns; TTY-aware wrapping).

Optional:

- `--json` prints `{"success":true,"data":...,"error":null}`-style responses.

Recommendation:

- Write logs/progress (sync counters, reconnect notices) to stderr.
- Write primary command output to stdout.

## Reliability considerations

- **Session conflicts**: running multiple instances can cause disconnects or “device replaced” behavior; locking is mandatory.
- **Reconnect**: on disconnect, retry with exponential backoff and respect context cancellation.
- **Idempotency**: message inserts are upserts keyed by (`chat_jid`, `msg_id`) so replays/history sync don’t duplicate data.

## Security considerations

- Store contains encryption keys/session data; protect permissions:
  - store dir `0700`
  - DB files `0600`
- Avoid printing sensitive identifiers in logs unless needed for debugging (`--verbose`).

## Implementation milestones

### v0.1 (MVP)

- `auth` (QR + bootstrap sync)
- `sync` (non-interactive, follow mode)
- `messages list/search` with FTS5
- `send text`
- store locking, default `~/.wacli-pro`

### v0.2

- contacts: show + local alias/notes/tags
- chats list/show with better naming resolution
- groups list/info/rename/participants

### v0.3

- media download command + optional background downloader
- `messages show/context` polish

## Prior art / credit

This spec borrows ideas and lessons learned from:

- `https://github.com/vicentereig/whatsapp-cli`
