# Changelog

## 0.5.0 - Unreleased

### Added

- Send: `wacli send sticker` command for sending WebP sticker files. (#27 — thanks @fm1randa)
- Presence: `wacli presence typing` and `wacli presence paused` commands for sending chat state indicators, with `--media audio` for recording indicator. (#76 — thanks @redemerco)
- Messages: `--full` global flag to disable truncation in table output; message IDs are shown in full when piped or with `--full`. (#13 — thanks @rickhallett)
- Messages: store structured reaction data (`reaction_to_id`, `reaction_emoji`) in the messages table. (#67 — thanks @vlassance)
- Sync: `--events` flag emits NDJSON lifecycle events to stderr (`connected`, `disconnected`, `new_message`, `qr_code`, etc.) for machine-readable scripting and automation. (#85 — thanks @nextbysam)
- Config: `WACLI_STORE_DIR` environment variable to override the store directory (equivalent to `--store`). (#37 — thanks @mia-mouret)

### Fixed

- JIDs: normalize all JIDs to non-AD form before storage, preventing duplicate entries for the same contact. (#32 — thanks @terry-li-hm)
- Phone numbers: strip leading `+` prefix in `ParseUserOrJID` so `+49123456789` is accepted. (#74 — thanks @FrederickStempfle)

### Security

- SQL injection: escape `%` and `_` metacharacters in LIKE queries; wrap FTS5 MATCH queries to prevent operator injection.
- Input validation: phone number regex validation (`^\d{1,15}$`), DB path sanitization, table name validation in migrations.
- Resource limits: 100 MB file size cap for uploads and downloads; bounded media download queue.
- File permissions: set `0600` on the SQLite database file.
- Concurrency: `sync.Once` for WhatsApp client initialization; panic recovery in event handlers and media workers.
- Sync: force-exit on double SIGINT during `sync --follow`. (#63 — thanks @alexander-morris)

### Changed

- Internal architecture: split store and groups command logic into focused modules for cleaner maintenance and safer follow-up changes.

### Build

- CI: extract a shared setup action and reuse it across CI and release workflows.
- Release: install arm64 libc headers in release workflow to improve ARM build reliability.

### Docs

- README: update usage/docs for the 0.2.0 release baseline.
- Changelog: roll unreleased tracking from `0.2.1` to `0.5.0`.
- Add CLAUDE.md for Claude Code guidance with project overview, architecture, build commands, and testing patterns. (#82 — thanks @dojanjanjan)

### Chore

- Version: bump CLI version string to `0.5.0` (unreleased).
- Dependencies: bump `filippo.io/edwards25519` from 1.1.0 to 1.1.1. (#69)

## 0.2.0 - 2026-01-23

### Added

- Messages: store display text for reactions, replies, and media; include in search output.
- Send: `wacli send file --filename` to override display name for uploads. (#7 — thanks @plattenschieber)
- Auth: allow `WACLI_DEVICE_LABEL` and `WACLI_DEVICE_PLATFORM` overrides for linked device identity. (#4 — thanks @zats)

### Fixed

- Build: preserve existing `CGO_CFLAGS` when adding GCC 15+ workaround. (#8 — thanks @ramarivera)
- Messages: keep captions in list/search output.

### Build

- Release: multi-OS GoReleaser configs and workflow for macOS, linux, and windows artifacts.

## 0.1.0 - 2026-01-01

### Added

- Auth: `wacli auth` QR login, bootstrap sync, optional follow, idle-exit, background media download, contacts/groups refresh.
- Sync: non-interactive `wacli sync` once/follow, never shows QR, idle-exit, background media download, optional contacts/groups refresh.
- Messages: list/search/show/context with chat/sender/time/media filters; FTS5 search with LIKE fallback and snippets.
- Send: text and file (image/video/audio/document) with caption and MIME override.
- Media: download by chat/id, resolves output paths, and records downloaded media in the DB.
- History: on-demand backfill per chat with request count, wait, and idle-exit.
- Contacts: search/show; import from WhatsApp store; local alias and tag management.
- Chats: list/show with kind and last message timestamp.
- Groups: list/refresh/info/rename; participants add/remove/promote/demote; invite link get/revoke; join/leave.
- Diagnostics: `wacli doctor` for store path, lock status/info, auth/connection check, and FTS status.
- CLI UX: human-readable output by default with `--json`, global `--store`/`--timeout`, plus `wacli version`.
- Storage: default `~/.wacli`, lock file for single-instance safety, SQLite DB with FTS5, WhatsApp session store, and media directory.
