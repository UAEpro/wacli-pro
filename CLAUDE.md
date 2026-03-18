# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

wacli is a Go CLI for WhatsApp built on the whatsmeow library. It syncs messages to a local SQLite database, provides offline full-text search (FTS5), and supports sending messages/files and managing contacts/groups.

## Build & Development Commands

All commands use `pnpm` as the task runner:

```bash
pnpm build              # Build binary to dist/wacli (requires CGO_ENABLED=1)
pnpm test               # Run all tests (standard + FTS5 tagged)
pnpm test:go            # Go tests without FTS5
pnpm test:fts           # Go tests with -tags sqlite_fts5
pnpm lint               # go vet ./...
pnpm format             # gofmt -w .
pnpm format:check       # Check formatting (CI uses this)
```

To run a single test: `go test -run TestName ./internal/store/...`

The build requires CGO (for SQLite via mattn/go-sqlite3). The FTS5 build tag (`-tags sqlite_fts5`) enables full-text search support.

## Architecture

```
cmd/wacli/          CLI entry point and cobra commands
internal/
  app/              Core application logic (App struct coordinates everything)
  wa/               WhatsApp client wrapper around whatsmeow
  store/            SQLite database layer (wacli.db)
  out/              Output formatting (human-readable text + JSON envelope)
  lock/             File-based exclusive locking (prevents concurrent instances)
  config/           Configuration management
  pathutil/         Path sanitization utilities
```

**Key abstractions:**

- `App` (`internal/app/app.go`) — central coordinator that holds a `WAClient` interface and `store.DB`. All command implementations receive an `App`.
- `WAClient` interface (`internal/app/app.go`) — abstraction over whatsmeow, used for testing with mocks.
- `store.DB` (`internal/store/store.go`) — SQLite wrapper with WAL mode. Tables: chats, contacts, groups, group_participants, contact_aliases, contact_tags, messages, messages_fts (FTS5 virtual table). Schema at migration v8.
- `out` package — provides consistent JSON envelope output (`--json` flag) and tabwriter-based text output.

**Data flow:** CLI commands → `App` methods in `internal/app/` → `WAClient` (network) + `store.DB` (persistence).

**Two SQLite databases** live in the store directory (default `~/.wacli`):
- `session.db` — whatsmeow session/auth state
- `wacli.db` — application data (messages, contacts, groups)

## Commands Overview

- `auth` — interactive login (QR code), then initial sync
- `sync` — non-interactive sync (`--once`, `--follow`, `--events` for JSON event stream)
- `messages` — list, search, show, context, edit, delete
- `send text` — send text messages (with optional `--reply-to` for quoted replies)
- `send file` — send files (image/video/audio/document, with optional `--ptt` for voice notes)
- `send sticker` — send WebP stickers
- `send voice` — shortcut for `send file --ptt`
- `send reaction` — react to a message with an emoji
- `send location` — send GPS coordinates
- `send forward` — forward a message to another chat
- `chats list` — list chats with filters (`--archived`, `--pinned`, `--muted`, `--unread`)
- `chats archive/pin/mute/mark-read` — manage chat state
- `contacts` — search, show, refresh, alias, tags
- `groups` — list, refresh, info, rename, leave, participants, invite, join
- `media download` — download media for a message
- `history backfill` — request older messages from primary device
- `presence` — send typing/paused indicators

## Testing Patterns

Tests use Go's standard `testing` package with table-driven tests. Database tests use `openTestDB()` helpers that create temp directories via `t.TempDir()`. The `WAClient` interface enables mock-based testing of app logic without network calls.

## Environment Variables

- `WACLI_STORE_DIR` — override the store directory (default: `~/.wacli`). Equivalent to `--store`
- `WACLI_DEVICE_LABEL` — custom device label shown in WhatsApp linked devices
- `WACLI_DEVICE_PLATFORM` — device platform override (defaults to CHROME)
