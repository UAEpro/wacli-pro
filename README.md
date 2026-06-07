# 🗃️ wacli — WhatsApp CLI: sync, search, send.

WhatsApp CLI built on top of `whatsmeow`, focused on:

- Best-effort local sync of message history + continuous capture
- Fast offline search (FTS5)
- Sending messages, files, stickers, voice notes, reactions, locations
- Contact + group management
- Chat state management (archive, pin, mute, mark-read)

This is a third-party tool that uses the WhatsApp Web protocol via `whatsmeow` and is not affiliated with WhatsApp.

## Status

Core implementation is in place. See `docs/spec.md` for the full design notes.

## Recent updates (0.7.0)

- Channels: list, info, follow/unfollow, mute/unmute WhatsApp channels (newsletters)
- Profile: set-about, set-photo, remove-photo commands
- Groups: create new groups, manage join requests (list/approve/reject)
- Send: poll messages with configurable options and max selections
- Messages: export command for JSON dumps
- Store: stats command showing DB size and record counts
- Full command reference at `docs/commands.md`

## Previous (0.6.0)

- Group admin settings: topic/description, photo, lock/unlock, announce mode, join approval, member add mode
- Enhanced `groups info` output with group settings (locked, announce, topic, member add mode)
- Status/stories: post text and image/video status updates
- Background sync daemon (`daemon start/stop/status/logs`)
- Diagnostics (`doctor`) command

## Previous (0.5.0)

- Chat state: archive, pin, mute, mark-read commands
- Message edit/delete (revoke) support
- Send reactions, locations, and forward messages
- Send stickers (WebP) and voice notes (PTT)
- Presence indicators (typing/paused)
- Contact aliases and tags for local metadata
- Sync: `--events` flag for NDJSON event stream, `--full` flag for untruncated output
- Performance indexes on chats, contacts, and groups tables
- Dependencies updated (go-sqlite3 v1.14.37, whatsmeow latest)

## Install / Build

Choose **one** of the following options.
If you install via Homebrew, you can skip the local build step.

### Option A: Install via Homebrew (tap)

- `brew install steipete/tap/wacli`

### Option B: Build locally

- `go build -tags sqlite_fts5 -o ./dist/wacli ./cmd/wacli`

Run (local build only):

- `./dist/wacli --help`

## Quick start

Default store directory is `~/.wacli` (override with `--store DIR` or `WACLI_STORE_DIR`).

```bash
# 1) Authenticate (shows QR), then bootstrap sync
wacli auth

# Check auth status
wacli auth status

# Logout (invalidate session)
wacli auth logout

# 2) Keep syncing (never shows QR; requires prior auth)
wacli sync --follow

# Sync once and exit
wacli sync --once

# Sync with NDJSON event stream (for scripting)
wacli sync --once --events

# Diagnostics
wacli doctor
```

## Messages

```bash
# Search messages (uses FTS5 if available, falls back to LIKE)
wacli messages search "meeting"

# List messages in a chat
wacli messages list --chat 1234567890@s.whatsapp.net

# Show a single message with context
wacli messages show --chat 1234567890@s.whatsapp.net --id <message-id>
wacli messages context --chat 1234567890@s.whatsapp.net --id <message-id>

# Edit a sent message
wacli messages edit --chat 1234567890@s.whatsapp.net --id <message-id> --message "corrected text"

# Delete (revoke) a sent message
wacli messages delete --chat 1234567890@s.whatsapp.net --id <message-id>
```

## Sending

```bash
# Send a text message
wacli send text --to 1234567890 --message "hello"

# Reply to a message (quoted reply)
wacli send text --to 120363000000000000@g.us --reply-to <message-id> --message "replying"

# Send a file (auto-detects image/video/audio/document)
wacli send file --to 1234567890 --file ./pic.jpg --caption "hi"
wacli send file --to 1234567890 --file /tmp/abc123 --filename report.pdf

# Send a sticker (must be WebP)
wacli send sticker --to 1234567890 --file ./sticker.webp

# Send a voice note (Push-To-Talk)
wacli send voice --to 1234567890 --file ./recording.ogg

# React to a message
wacli send reaction --to 1234567890 --id <message-id> --emoji "👍"

# Send a location
wacli send location --to 1234567890 --lat 25.2 --lng 55.3 --name "Office"

# Forward a message
wacli send forward --to 1234567890 --from-chat <source-chat> --id <message-id>
```

## Chat management

```bash
# List chats (with optional filters)
wacli chats list
wacli chats list --pinned
wacli chats list --unread
wacli chats list --archived

# Show a single chat
wacli chats show --jid 1234567890@s.whatsapp.net

# Archive/unarchive, pin/unpin, mute/unmute, mark read/unread
wacli chats archive --jid 1234567890@s.whatsapp.net
wacli chats unarchive --jid 1234567890@s.whatsapp.net
wacli chats pin --jid 1234567890@s.whatsapp.net
wacli chats unpin --jid 1234567890@s.whatsapp.net
wacli chats mute --jid 1234567890@s.whatsapp.net --duration 8h
wacli chats unmute --jid 1234567890@s.whatsapp.net
wacli chats mark-read --jid 1234567890@s.whatsapp.net
wacli chats mark-unread --jid 1234567890@s.whatsapp.net
```

## Contacts

```bash
# Search contacts
wacli contacts search "alice"

# Show a single contact
wacli contacts show --jid 1234567890@s.whatsapp.net

# Set a local alias
wacli contacts alias set --jid 1234567890@s.whatsapp.net --alias "Alice"
wacli contacts alias rm --jid 1234567890@s.whatsapp.net

# Tag contacts
wacli contacts tags add --jid 1234567890@s.whatsapp.net --tag "work"
wacli contacts tags rm --jid 1234567890@s.whatsapp.net --tag "work"

# Refresh contacts from WhatsApp
wacli contacts refresh
```

## Groups

```bash
# List groups
wacli groups list

# Get group info (live from WhatsApp, shows settings)
wacli groups info --jid 123456789@g.us

# Refresh all groups from WhatsApp
wacli groups refresh

# Rename a group
wacli groups rename --jid 123456789@g.us --name "New name"

# Set group description/topic
wacli groups topic --jid 123456789@g.us --topic "Group description here"
wacli groups topic --jid 123456789@g.us --topic ""  # clear description

# Set or remove group photo (JPEG)
wacli groups photo --jid 123456789@g.us --file photo.jpg
wacli groups photo --jid 123456789@g.us --remove

# Lock/unlock group settings (admin-only editing)
wacli groups lock --jid 123456789@g.us    # only admins can edit group info
wacli groups unlock --jid 123456789@g.us  # all participants can edit

# Announce mode (admin-only messaging)
wacli groups announce --jid 123456789@g.us    # only admins can send messages
wacli groups unannounce --jid 123456789@g.us  # all participants can send

# Join approval (require admin approval to join)
wacli groups join-approval --jid 123456789@g.us --on
wacli groups join-approval --jid 123456789@g.us --off

# Who can add members
wacli groups member-add-mode --jid 123456789@g.us --mode admin  # admins only
wacli groups member-add-mode --jid 123456789@g.us --mode all    # all members

# Manage participants
wacli groups participants add --jid 123456789@g.us --user 1234567890
wacli groups participants remove --jid 123456789@g.us --user 1234567890
wacli groups participants promote --jid 123456789@g.us --user 1234567890
wacli groups participants demote --jid 123456789@g.us --user 1234567890

# Invite links
wacli groups invite link get --jid 123456789@g.us
wacli groups invite link revoke --jid 123456789@g.us
wacli groups join --code <invite-code>

# Leave a group
wacli groups leave --jid 123456789@g.us
```

## Media

```bash
# Download media for a message
wacli media download --chat 1234567890@s.whatsapp.net --id <message-id>
wacli media download --chat 1234567890@s.whatsapp.net --id <message-id> --output ./downloads/
```

## Presence

```bash
# Send typing indicator
wacli presence typing --to 1234567890

# Send recording (audio) indicator
wacli presence typing --to 1234567890 --media audio

# Send paused indicator
wacli presence paused --to 1234567890
```

## Status (stories)

```bash
# Post a text status
wacli status text --text "Hello world"

# Post an image/video status
wacli status file --file photo.jpg --caption "Check this out"
```

## Daemon (background sync)

```bash
# Start background sync daemon
wacli daemon start
wacli daemon start --download-media --refresh-contacts --refresh-groups

# Check daemon status
wacli daemon status

# View daemon logs
wacli daemon logs
wacli daemon logs -f     # follow (like tail -f)
wacli daemon logs -n 100 # last 100 lines

# Stop daemon
wacli daemon stop
```

## Diagnostics

```bash
# Check store, auth, and search status
wacli doctor
wacli doctor --connect  # also test WhatsApp connection
```

## Backfilling older history

`wacli sync` stores whatever WhatsApp Web sends opportunistically. To try to fetch *older* messages, use on-demand history sync requests to your **primary device** (your phone).

Important notes:

- This is **best-effort**: WhatsApp may not return full history.
- Your **primary device must be online**.
- Requests are **per chat** (DM or group). `wacli` uses the *oldest locally stored message* in that chat as the anchor.
- Recommended `--count` is `50` per request.

### Backfill one chat

```bash
wacli history backfill --chat 1234567890@s.whatsapp.net --requests 10 --count 50
```

### Backfill all chats (script)

This loops through chats already known in your local DB:

```bash
wacli --json chats list --limit 100000 \
  | jq -r '.[].JID' \
  | while read -r jid; do
      wacli history backfill --chat "$jid" --requests 3 --count 50
    done
```

## Global flags

| Flag | Description |
|------|-------------|
| `--store DIR` | Store directory (default: `~/.wacli`) |
| `--json` | Output JSON instead of human-readable text |
| `--full` | Disable truncation in table output |
| `--events` | Emit NDJSON lifecycle events to stderr |
| `--timeout` | Command timeout (default: 5m, does not apply to sync) |

## Environment overrides

- `WACLI_STORE_DIR`: override the store directory (default: `~/.wacli`). Equivalent to `--store`.
- `WACLI_DEVICE_LABEL`: set the linked device label (shown in WhatsApp).
- `WACLI_DEVICE_PLATFORM`: override the linked device platform (defaults to `CHROME` if unset or invalid).

## Prior Art / Credit

This project is heavily inspired by (and learns from) the excellent `whatsapp-cli` by Vicente Reig:

- [`whatsapp-cli`](https://github.com/vicentereig/whatsapp-cli)

## High-level UX

- `wacli auth`: interactive login (shows QR code), then immediately performs initial data sync.
- `wacli sync`: non-interactive sync loop (never shows QR; errors if not authenticated).
- Output is human-readable by default; pass `--json` for machine-readable output.

## Storage

Defaults to `~/.wacli` (override with `--store DIR` or `WACLI_STORE_DIR`).

Two SQLite databases:
- `session.db` — whatsmeow session/auth state
- `wacli.db` — application data (messages, contacts, groups)

## License

See `LICENSE`.
