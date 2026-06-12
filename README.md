# 🗃️ wacli-pro — WhatsApp CLI: sync, search, send.

WhatsApp CLI built on top of [`whatsmeow`](https://github.com/tulir/whatsmeow), focused on:

- Best-effort local sync of message history + continuous capture
- Fast offline full-text search (SQLite FTS5)
- Sending messages, files, stickers, voice notes, polls, reactions, locations
- Contact + group + channel management
- Chat state management (archive, pin, mute, mark-read)
- Background sync daemon, webhooks, message export

This is a third-party tool that uses the WhatsApp Web protocol via `whatsmeow` and is not affiliated with WhatsApp.

See [`docs/commands.md`](docs/commands.md) for the full command reference and [`CHANGELOG.md`](CHANGELOG.md) for release notes.

## Install

Pick one of the options below. Pre-built binaries are available for macOS (universal), Linux (amd64/arm64), and Windows (amd64).

### Option A: Install script (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/UAEpro/wacli-pro/main/scripts/install.sh | bash
```

Installs the latest release binary to `/usr/local/bin` (or `~/.local/bin` if that isn't writable). Pin a version with `WACLI_PRO_VERSION=v1.3.0`.

### Option B: Download a release binary

Grab the archive for your platform from the [latest release](https://github.com/UAEpro/wacli-pro/releases/latest), extract it, and put `wacli-pro` somewhere on your `PATH`:

```bash
tar -xzf wacli-pro-macos-universal.tar.gz
sudo mv wacli-pro /usr/local/bin/
```

On Windows, download `wacli-pro-windows-amd64.zip` and add the extracted folder to your `PATH`.

### Option C: go install

Requires Go 1.25+ and a C compiler (CGO is needed for SQLite):

```bash
CGO_ENABLED=1 go install -tags sqlite_fts5 github.com/UAEpro/wacli-pro/cmd/wacli-pro@latest
```

### Option D: Build from source

Requires Go 1.25+ and a C compiler. With [pnpm](https://pnpm.io):

```bash
git clone https://github.com/UAEpro/wacli-pro.git
cd wacli-pro
pnpm build          # builds dist/wacli-pro
./dist/wacli-pro --help
```

Or with plain Go:

```bash
CGO_ENABLED=1 go build -tags sqlite_fts5 -o dist/wacli-pro ./cmd/wacli-pro
```

The `sqlite_fts5` build tag enables full-text search; without it, search falls back to slower `LIKE` queries.

### Option E: Docker (great for servers)

Run the sync daemon on a server with Docker Compose:

```bash
git clone https://github.com/UAEpro/wacli-pro.git
cd wacli-pro

# 1) Build the image and authenticate interactively (scan the QR code)
docker compose build
docker compose run --rm wacli-pro auth

# 2) Start the background sync service
docker compose up -d

# Use the CLI against the same data volume
docker compose run --rm wacli-pro messages search "meeting"
docker compose run --rm wacli-pro send text --to 1234567890 --message "hello"
```

All state is kept in the `wacli-data` named volume, so the container can be rebuilt or updated without re-authenticating.

## Quick start

Default store directory is `~/.wacli-pro` (override with `--store DIR` or `WACLI_PRO_STORE_DIR`).

```bash
# 1) Authenticate (shows QR), then bootstrap sync
wacli-pro auth

# Or pair with a phone number code instead of a QR
wacli-pro auth --phone 9715xxxxxxxx

# Check auth status / logout
wacli-pro auth status
wacli-pro auth logout

# 2) Keep syncing (never shows QR; requires prior auth)
wacli-pro sync --follow

# Sync once and exit
wacli-pro sync --once

# Sync with NDJSON event stream (for scripting)
wacli-pro sync --once --events

# Diagnostics
wacli-pro doctor
```

## Messages

```bash
# Search messages (uses FTS5 if available, falls back to LIKE)
wacli-pro messages search "meeting"

# List messages in a chat
wacli-pro messages list --chat 1234567890@s.whatsapp.net

# Show a single message with context
wacli-pro messages show --chat 1234567890@s.whatsapp.net --id <message-id>
wacli-pro messages context --chat 1234567890@s.whatsapp.net --id <message-id>

# Edit a sent message
wacli-pro messages edit --chat 1234567890@s.whatsapp.net --id <message-id> --message "corrected text"

# Delete (revoke) a sent message
wacli-pro messages delete --chat 1234567890@s.whatsapp.net --id <message-id>

# Export messages as JSON (writes to stdout)
wacli-pro messages export --chat 1234567890@s.whatsapp.net > export.json
```

## Sending

```bash
# Send a text message
wacli-pro send text --to 1234567890 --message "hello"

# Reply to a message (quoted reply)
wacli-pro send text --to 120363000000000000@g.us --reply-to <message-id> --message "replying"

# Send a file (auto-detects image/video/audio/document)
wacli-pro send file --to 1234567890 --file ./pic.jpg --caption "hi"
wacli-pro send file --to 1234567890 --file /tmp/abc123 --filename report.pdf

# Send a sticker (must be WebP)
wacli-pro send sticker --to 1234567890 --file ./sticker.webp

# Send a voice note (Push-To-Talk)
wacli-pro send voice --to 1234567890 --file ./recording.ogg

# React to a message
wacli-pro send reaction --to 1234567890 --id <message-id> --emoji "👍"

# Send a location
wacli-pro send location --to 1234567890 --lat 25.2 --lng 55.3 --name "Office"

# Send a poll
wacli-pro send poll --to 1234567890 --question "Lunch?" --option "Pizza" --option "Sushi"

# Forward a message
wacli-pro send forward --to 1234567890 --from-chat <source-chat> --id <message-id>
```

## Chat management

```bash
# List chats (with optional filters)
wacli-pro chats list
wacli-pro chats list --pinned
wacli-pro chats list --unread
wacli-pro chats list --archived

# Show a single chat
wacli-pro chats show --jid 1234567890@s.whatsapp.net

# Archive/unarchive, pin/unpin, mute/unmute, mark read/unread
wacli-pro chats archive --jid 1234567890@s.whatsapp.net
wacli-pro chats unarchive --jid 1234567890@s.whatsapp.net
wacli-pro chats pin --jid 1234567890@s.whatsapp.net
wacli-pro chats unpin --jid 1234567890@s.whatsapp.net
wacli-pro chats mute --jid 1234567890@s.whatsapp.net --duration 8h
wacli-pro chats unmute --jid 1234567890@s.whatsapp.net
wacli-pro chats mark-read --jid 1234567890@s.whatsapp.net
wacli-pro chats mark-unread --jid 1234567890@s.whatsapp.net
```

## Contacts

```bash
# Search contacts
wacli-pro contacts search "alice"

# Show a single contact
wacli-pro contacts show --jid 1234567890@s.whatsapp.net

# Set a local alias
wacli-pro contacts alias set --jid 1234567890@s.whatsapp.net --alias "Alice"
wacli-pro contacts alias rm --jid 1234567890@s.whatsapp.net

# Tag contacts
wacli-pro contacts tags add --jid 1234567890@s.whatsapp.net --tag "work"
wacli-pro contacts tags rm --jid 1234567890@s.whatsapp.net --tag "work"

# Refresh contacts from WhatsApp
wacli-pro contacts refresh
```

## Groups

```bash
# List groups
wacli-pro groups list

# Create a group
wacli-pro groups create --name "My group" --user 1234567890 --user 0987654321

# Get group info (live from WhatsApp, shows settings)
wacli-pro groups info --jid 123456789@g.us

# Refresh all groups from WhatsApp
wacli-pro groups refresh

# Rename a group / set description
wacli-pro groups rename --jid 123456789@g.us --name "New name"
wacli-pro groups topic --jid 123456789@g.us --topic "Group description here"

# Set or remove group photo (JPEG)
wacli-pro groups photo --jid 123456789@g.us --file photo.jpg
wacli-pro groups photo --jid 123456789@g.us --remove

# Admin settings
wacli-pro groups lock --jid 123456789@g.us        # only admins can edit group info
wacli-pro groups announce --jid 123456789@g.us    # only admins can send messages
wacli-pro groups join-approval --jid 123456789@g.us --on
wacli-pro groups member-add-mode --jid 123456789@g.us --mode admin

# Manage participants
wacli-pro groups participants add --jid 123456789@g.us --user 1234567890
wacli-pro groups participants remove --jid 123456789@g.us --user 1234567890
wacli-pro groups participants promote --jid 123456789@g.us --user 1234567890
wacli-pro groups participants demote --jid 123456789@g.us --user 1234567890

# Invite links
wacli-pro groups invite link get --jid 123456789@g.us
wacli-pro groups invite link revoke --jid 123456789@g.us
wacli-pro groups join --code <invite-code>

# Leave a group
wacli-pro groups leave --jid 123456789@g.us
```

## Channels, profile, media, presence, status

```bash
# Channels (newsletters)
wacli-pro channels list
wacli-pro channels follow --jid 120363000000000000@newsletter

# Profile
wacli-pro profile set-about --text "Available"
wacli-pro profile set-photo --file me.jpg

# Download media for a message
wacli-pro media download --chat 1234567890@s.whatsapp.net --id <message-id>

# Typing / recording / paused indicators
wacli-pro presence typing --to 1234567890
wacli-pro presence typing --to 1234567890 --media audio
wacli-pro presence paused --to 1234567890

# Post a status (story)
wacli-pro status text --text "Hello world"
wacli-pro status file --file photo.jpg --caption "Check this out"
```

## Daemon (background sync)

```bash
# Start background sync daemon
wacli-pro daemon start
wacli-pro daemon start --download-media --refresh-contacts --refresh-groups

# Check daemon status / view logs
wacli-pro daemon status
wacli-pro daemon logs -f

# Stop daemon
wacli-pro daemon stop
```

## Diagnostics

```bash
# Check store, auth, and search status
wacli-pro doctor
wacli-pro doctor --connect  # also test WhatsApp connection
```

## Backfilling older history

`wacli-pro sync` stores whatever WhatsApp Web sends opportunistically. To try to fetch *older* messages, use on-demand history sync requests to your **primary device** (your phone).

Important notes:

- This is **best-effort**: WhatsApp may not return full history.
- Your **primary device must be online**.
- Requests are **per chat** (DM or group). `wacli-pro` uses the *oldest locally stored message* in that chat as the anchor.
- Recommended `--count` is `50` per request.

```bash
# Backfill one chat
wacli-pro history backfill --chat 1234567890@s.whatsapp.net --requests 10 --count 50

# Backfill all chats known to the local DB
wacli-pro --json chats list --limit 100000 \
  | jq -r '.[].JID' \
  | while read -r jid; do
      wacli-pro history backfill --chat "$jid" --requests 3 --count 50
    done
```

## Global flags

| Flag | Description |
|------|-------------|
| `--store DIR` | Store directory (default: `~/.wacli-pro`) |
| `--json` | Output JSON instead of human-readable text |
| `--full` | Disable truncation in table output |
| `--events` | Emit NDJSON lifecycle events to stderr |
| `--timeout` | Command timeout (default: 5m, does not apply to sync) |

## Environment variables

- `WACLI_PRO_STORE_DIR`: override the store directory (default: `~/.wacli-pro`). Equivalent to `--store`.
- `WACLI_PRO_DEVICE_LABEL`: set the linked device label (shown in WhatsApp linked devices).
- `WACLI_PRO_DEVICE_PLATFORM`: override the linked device platform (defaults to `CHROME` if unset or invalid).

## Storage

Defaults to `~/.wacli-pro` (override with `--store DIR` or `WACLI_PRO_STORE_DIR`).

Two SQLite databases:
- `session.db` — whatsmeow session/auth state
- `wacli.db` — application data (messages, contacts, groups)

## Development

```bash
pnpm build              # Build binary to dist/wacli-pro (requires CGO)
pnpm test               # Run all tests (standard + FTS5 tagged)
pnpm lint               # go vet ./...
pnpm format             # gofmt -w .
```

Releases are automated: pushing a `vX.Y.Z` tag triggers the GoReleaser GitHub Actions workflow, which builds macOS/Linux/Windows archives and attaches them to the GitHub release. See [`docs/release.md`](docs/release.md).

## Prior art / credit

This project started as a fork of [`wacli`](https://github.com/steipete/wacli) by Peter Steinberger, which was itself inspired by [`whatsapp-cli`](https://github.com/vicentereig/whatsapp-cli) by Vicente Reig. Many thanks to both.

## License

MIT — see [`LICENSE`](LICENSE).
