# Command Reference

Complete reference for every `wacli-pro` command, subcommand, and flag.

---

## Table of Contents

- [Global Flags](#global-flags)
- [auth](#auth) - Authenticate with WhatsApp
- [sync](#sync) - Sync messages
- [messages](#messages) - List, search, and export messages
- [send](#send) - Send messages, files, reactions, locations, polls
- [chats](#chats) - List and manage chats
- [contacts](#contacts) - Search and manage contacts
- [groups](#groups) - Group management and admin settings
- [channels](#channels) - WhatsApp channels (newsletters)
- [profile](#profile) - Profile management
- [store](#store) - Local store management
- [media](#media) - Download media
- [history](#history) - Backfill older messages
- [presence](#presence) - Typing indicators
- [status](#status) - Post to WhatsApp Status (stories)
- [daemon](#daemon) - Background sync as a native OS service
- [doctor](#doctor) - Diagnostics
- [version](#version) - Print version

---

## Global Flags

These flags are available on all commands.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--store` | string | `~/.wacli-pro` | Store directory |
| `--json` | bool | `false` | Output JSON instead of human-readable text |
| `--full` | bool | `false` | Disable truncation in table output |
| `--events` | bool | `false` | Emit NDJSON lifecycle events to stderr (for scripting) |
| `--timeout` | duration | `5m` | Command timeout (does not apply to sync) |

---

## auth

Authenticate with WhatsApp (shows QR code) and bootstrap sync.

```bash
wacli-pro auth
wacli-pro auth --follow          # keep syncing after auth
wacli-pro auth --download-media  # download media during initial sync
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--follow` | bool | `false` | Keep syncing after auth |
| `--idle-exit` | duration | `30s` | Exit after being idle |
| `--download-media` | bool | `false` | Download media in the background during sync |

### auth status

Check authentication status.

```bash
wacli-pro auth status
```

### auth logout

Logout and invalidate the session.

```bash
wacli-pro auth logout
```

---

## sync

Sync messages from WhatsApp. Requires prior authentication (never shows QR).

```bash
wacli-pro sync                # default: --follow (keep syncing)
wacli-pro sync --once         # sync until idle and exit
wacli-pro sync --once --events  # NDJSON event stream for scripting
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--once` | bool | `false` | Sync until idle and exit |
| `--follow` | bool | `true` | Keep syncing until Ctrl+C |
| `--idle-exit` | duration | `30s` | Exit after being idle (once mode) |
| `--max-duration` | duration | `0` | Hard timeout for once mode (e.g. `5m`); 0 = no limit |
| `--download-media` | bool | `false` | Download media in the background during sync |
| `--refresh-contacts` | bool | `false` | Refresh contacts from session store into local DB |
| `--refresh-groups` | bool | `false` | Refresh joined groups (live) into local DB |

---

## messages

List and search messages from the local database.

### messages list

List messages, optionally filtered by chat and time range.

```bash
wacli-pro messages list --chat 1234567890@s.whatsapp.net
wacli-pro messages list --chat 1234567890@s.whatsapp.net --after 2025-01-01 --limit 100
wacli-pro messages list --names  # show sender names instead of JIDs
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--chat` | string | | Chat JID |
| `--limit` | int | `50` | Limit results |
| `--after` | string | | Only messages after time (RFC3339 or `YYYY-MM-DD`) |
| `--before` | string | | Only messages before time (RFC3339 or `YYYY-MM-DD`) |
| `--names` | bool | `false` | Show sender names instead of JIDs |

### messages search

Search messages using full-text search (FTS5 if available, falls back to LIKE).

```bash
wacli-pro messages search "meeting"
wacli-pro messages search "meeting" --chat 1234567890@s.whatsapp.net
wacli-pro messages search "report" --type document --after 2025-01-01
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--chat` | string | | Chat JID |
| `--from` | string | | Sender JID |
| `--limit` | int | `50` | Limit results |
| `--after` | string | | Only messages after time |
| `--before` | string | | Only messages before time |
| `--type` | string | | Media type filter: `image`, `video`, `audio`, `document` |
| `--names` | bool | `false` | Show sender names instead of JIDs |

### messages show

Show a single message.

```bash
wacli-pro messages show --chat 1234567890@s.whatsapp.net --id <message-id>
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--chat` | string | | Chat JID (required) |
| `--id` | string | | Message ID (required) |
| `--names` | bool | `false` | Show sender names instead of JIDs |

### messages context

Show messages surrounding a specific message.

```bash
wacli-pro messages context --chat 1234567890@s.whatsapp.net --id <message-id>
wacli-pro messages context --chat 1234567890@s.whatsapp.net --id <message-id> --before 10 --after 10
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--chat` | string | | Chat JID (required) |
| `--id` | string | | Message ID (required) |
| `--before` | int | `5` | Number of messages before |
| `--after` | int | `5` | Number of messages after |
| `--names` | bool | `false` | Show sender names instead of JIDs |

### messages edit

Edit a message you sent (within WhatsApp's edit window).

```bash
wacli-pro messages edit --chat 1234567890@s.whatsapp.net --id <message-id> --message "corrected text"
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--chat` | string | | Chat JID (required) |
| `--id` | string | | Message ID (required) |
| `--message` | string | | New message text (required) |

### messages delete

Delete (revoke) a message for everyone.

```bash
wacli-pro messages delete --chat 1234567890@s.whatsapp.net --id <message-id>
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--chat` | string | | Chat JID (required) |
| `--id` | string | | Message ID (required) |

### messages export

Export messages from a chat as JSON.

```bash
wacli-pro messages export --chat 1234567890@s.whatsapp.net
wacli-pro messages export --chat 1234567890@s.whatsapp.net --after 2025-01-01 --limit 1000
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--chat` | string | | Chat JID (required) |
| `--limit` | int | `0` | Limit results (0 = all) |
| `--after` | string | | Only messages after time (RFC3339 or `YYYY-MM-DD`) |
| `--before` | string | | Only messages before time (RFC3339 or `YYYY-MM-DD`) |

---

## send

Send messages, files, reactions, locations, and more.

### send text

Send a text message.

```bash
wacli-pro send text --to 1234567890 --message "hello"
wacli-pro send text --to 120363000000000000@g.us --reply-to <message-id> --message "replying"
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--to` | string | | Recipient phone number or JID (required) |
| `--message` | string | | Message text (required) |
| `--reply-to` | string | | Message ID to reply to (stanza id) |
| `--reply-chat` | string | | Chat JID where the reply-to message lives (defaults to `--to`) |

### send file

Send a file. Auto-detects type (image/video/audio/document).

```bash
wacli-pro send file --to 1234567890 --file ./pic.jpg --caption "check this out"
wacli-pro send file --to 1234567890 --file /tmp/report --filename report.pdf
wacli-pro send file --to 1234567890 --file recording.ogg --ptt  # as voice note
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--to` | string | | Recipient phone number or JID (required) |
| `--file` | string | | Path to file (required) |
| `--filename` | string | | Display name for the file (defaults to basename) |
| `--caption` | string | | Caption (images/videos/documents) |
| `--mime` | string | | Override detected MIME type |
| `--ptt` | bool | `false` | Send as voice note (Push-To-Talk) |

### send sticker

Send a sticker (must be WebP format).

```bash
wacli-pro send sticker --to 1234567890 --file ./sticker.webp
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--to` | string | | Recipient phone number or JID (required) |
| `--file` | string | | Path to WebP sticker file (required) |

### send voice

Send a voice note (shortcut for `send file --ptt`).

```bash
wacli-pro send voice --to 1234567890 --file ./recording.ogg
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--to` | string | | Recipient phone number or JID (required) |
| `--file` | string | | Path to audio file (required) |

### send reaction

React to a message with an emoji. Send an empty emoji to remove the reaction.

```bash
wacli-pro send reaction --to 1234567890 --id <message-id> --emoji "👍"
wacli-pro send reaction --to 1234567890 --id <message-id> --emoji ""  # remove
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--to` | string | | Chat JID or phone number (required) |
| `--id` | string | | Message ID to react to (required) |
| `--emoji` | string | | Emoji to react with (empty to remove) |

### send location

Send a location message.

```bash
wacli-pro send location --to 1234567890 --lat 25.2 --lng 55.3 --name "Office"
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--to` | string | | Recipient phone number or JID (required) |
| `--lat` | float | | Latitude (required) |
| `--lng` | float | | Longitude (required) |
| `--name` | string | | Location name |
| `--address` | string | | Location address |

### send forward

Forward a message to another chat.

```bash
wacli-pro send forward --to 1234567890 --from-chat <source-chat> --id <message-id>
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--to` | string | | Recipient phone number or JID (required) |
| `--from-chat` | string | | Source chat JID (required) |
| `--id` | string | | Message ID to forward (required) |

### send poll

Send a poll with options.

```bash
wacli-pro send poll --to 120363000000000000@g.us --question "Lunch?" --option "Pizza" --option "Sushi" --option "Tacos"
wacli-pro send poll --to 1234567890 --question "Pick two" --option "A" --option "B" --option "C" --max-selections 2
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--to` | string | | Recipient phone number or JID (required) |
| `--question` | string | | Poll question (required) |
| `--option` | string[] | | Poll option (repeatable, min 2 required) |
| `--max-selections` | int | `1` | Max selections allowed (0 = unlimited) |

---

## chats

List and manage chats.

### chats list

List chats with optional filters.

```bash
wacli-pro chats list
wacli-pro chats list --pinned
wacli-pro chats list --unread --no-archived
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--query` | string | | Search query |
| `--limit` | int | `50` | Limit results |
| `--archived` | bool | `false` | Show only archived chats |
| `--no-archived` | bool | `false` | Exclude archived chats |
| `--pinned` | bool | `false` | Show only pinned chats |
| `--no-pinned` | bool | `false` | Exclude pinned chats |
| `--muted` | bool | `false` | Show only muted chats |
| `--no-muted` | bool | `false` | Exclude muted chats |
| `--unread` | bool | `false` | Show only unread chats |
| `--no-unread` | bool | `false` | Exclude unread chats |

### chats show

Show details of a single chat.

```bash
wacli-pro chats show --jid 1234567890@s.whatsapp.net
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Chat JID (required) |

### chats archive / unarchive

```bash
wacli-pro chats archive --jid 1234567890@s.whatsapp.net
wacli-pro chats unarchive --jid 1234567890@s.whatsapp.net
```

### chats pin / unpin

```bash
wacli-pro chats pin --jid 1234567890@s.whatsapp.net
wacli-pro chats unpin --jid 1234567890@s.whatsapp.net
```

### chats mute / unmute

```bash
wacli-pro chats mute --jid 1234567890@s.whatsapp.net --duration 8h
wacli-pro chats mute --jid 1234567890@s.whatsapp.net  # mute forever
wacli-pro chats unmute --jid 1234567890@s.whatsapp.net
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Chat JID (required) |
| `--duration` | string | | Mute duration (e.g. `8h`, `24h`, `168h`); empty = forever |

### chats mark-read / mark-unread

```bash
wacli-pro chats mark-read --jid 1234567890@s.whatsapp.net
wacli-pro chats mark-unread --jid 1234567890@s.whatsapp.net
```

---

## contacts

Search and manage local contact metadata.

### contacts search

Search contacts from synced metadata.

```bash
wacli-pro contacts search "alice"
wacli-pro contacts search "work" --limit 100
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` | int | `50` | Limit results |

### contacts show

Show a single contact.

```bash
wacli-pro contacts show --jid 1234567890@s.whatsapp.net
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Contact JID (required) |

### contacts refresh

Import contacts from whatsmeow session store into local DB.

```bash
wacli-pro contacts refresh
```

### contacts alias set / rm

Manage local aliases for contacts.

```bash
wacli-pro contacts alias set --jid 1234567890@s.whatsapp.net --alias "Alice"
wacli-pro contacts alias rm --jid 1234567890@s.whatsapp.net
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Contact JID (required) |
| `--alias` | string | | Alias name (required for `set`) |

### contacts tags add / rm

Manage local tags for contacts.

```bash
wacli-pro contacts tags add --jid 1234567890@s.whatsapp.net --tag "work"
wacli-pro contacts tags rm --jid 1234567890@s.whatsapp.net --tag "work"
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Contact JID (required) |
| `--tag` | string | | Tag name (required) |

---

## groups

Group management and admin settings.

### groups list

List joined groups from local DB.

```bash
wacli-pro groups list
wacli-pro groups list --query "family"
wacli-pro groups list --all  # include groups you've left
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--query` | string | | Search by name or JID |
| `--limit` | int | `50` | Limit results |
| `--all` | bool | `false` | Include groups you have left |

### groups refresh

Fetch joined groups live from WhatsApp and update local DB.

```bash
wacli-pro groups refresh
```

### groups info

Fetch live group info from WhatsApp. Shows name, owner, participants, topic, locked/announce status, member add mode, and join approval.

```bash
wacli-pro groups info --jid 123456789@g.us
wacli-pro --json groups info --jid 123456789@g.us  # full JSON output
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Group JID (required) |

### groups create

Create a new group with participants.

```bash
wacli-pro groups create --name "Project Team" --user 1234567890 --user 9876543210
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | string | | Group name, max 25 characters (required) |
| `--user` | string[] | | Participant phone number or JID (repeatable, required) |

### groups rename

Rename a group.

```bash
wacli-pro groups rename --jid 123456789@g.us --name "New Group Name"
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Group JID (required) |
| `--name` | string | | New group name (required) |

### groups topic

Set or clear the group description/topic.

```bash
wacli-pro groups topic --jid 123456789@g.us --topic "Welcome to the group!"
wacli-pro groups topic --jid 123456789@g.us --topic ""  # clear description
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Group JID (required) |
| `--topic` | string | | New topic/description (empty to clear; required) |

### groups photo

Set or remove the group photo.

```bash
wacli-pro groups photo --jid 123456789@g.us --file photo.jpg
wacli-pro groups photo --jid 123456789@g.us --remove
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Group JID (required) |
| `--file` | string | | Path to photo file (JPEG) |
| `--remove` | bool | `false` | Remove group photo |

### groups lock / unlock

Lock or unlock group settings. When locked, only admins can edit group info (name, description, photo).

```bash
wacli-pro groups lock --jid 123456789@g.us    # admin-only editing
wacli-pro groups unlock --jid 123456789@g.us  # all participants can edit
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Group JID (required) |

### groups announce / unannounce

Enable or disable announce mode. When enabled, only admins can send messages.

```bash
wacli-pro groups announce --jid 123456789@g.us    # admin-only messages
wacli-pro groups unannounce --jid 123456789@g.us  # everyone can message
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Group JID (required) |

### groups join-approval

Toggle join approval mode. When enabled, new members require admin approval.

```bash
wacli-pro groups join-approval --jid 123456789@g.us --on
wacli-pro groups join-approval --jid 123456789@g.us --off
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Group JID (required) |
| `--on` | bool | `false` | Enable join approval |
| `--off` | bool | `false` | Disable join approval |

### groups member-add-mode

Set who can add new members to the group.

```bash
wacli-pro groups member-add-mode --jid 123456789@g.us --mode admin  # admins only
wacli-pro groups member-add-mode --jid 123456789@g.us --mode all    # all members
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Group JID (required) |
| `--mode` | string | | Who can add members: `admin` or `all` (required) |

### groups participants add / remove / promote / demote

Manage group participants. The `--user` flag can be repeated to act on multiple users.

```bash
wacli-pro groups participants add --jid 123456789@g.us --user 1234567890 --user 9876543210
wacli-pro groups participants remove --jid 123456789@g.us --user 1234567890
wacli-pro groups participants promote --jid 123456789@g.us --user 1234567890  # make admin
wacli-pro groups participants demote --jid 123456789@g.us --user 1234567890   # remove admin
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Group JID (required) |
| `--user` | string[] | | Phone number or JID (repeatable, required) |

### groups invite link get / revoke

Manage group invite links.

```bash
wacli-pro groups invite link get --jid 123456789@g.us
wacli-pro groups invite link revoke --jid 123456789@g.us  # revoke and generate new link
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Group JID (required) |

### groups join

Join a group by invite code.

```bash
wacli-pro groups join --code <invite-code>
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--code` | string | | Invite code from link (required) |

### groups requests list / approve / reject

Manage pending join requests (when join approval is enabled).

```bash
wacli-pro groups requests list --jid 123456789@g.us
wacli-pro groups requests approve --jid 123456789@g.us --user 1234567890
wacli-pro groups requests reject --jid 123456789@g.us --user 1234567890 --user 9876543210
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Group JID (required) |
| `--user` | string[] | | User phone number or JID (repeatable, required for approve/reject) |

### groups leave

Leave a group.

```bash
wacli-pro groups leave --jid 123456789@g.us
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Group JID (required) |

---

## channels

Manage WhatsApp channels (newsletters).

### channels list

List channels you're subscribed to.

```bash
wacli-pro channels list
```

### channels info

Get info about a channel.

```bash
wacli-pro channels info --jid 123456789@newsletter
wacli-pro channels info --invite "https://whatsapp.com/channel/..."
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Channel JID (required if no --invite) |
| `--invite` | string | | Channel invite link or code |

### channels follow / unfollow

Follow (join) or unfollow (leave) a channel.

```bash
wacli-pro channels follow --jid 123456789@newsletter
wacli-pro channels unfollow --jid 123456789@newsletter
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Channel JID (required) |

### channels mute / unmute

Mute or unmute a channel.

```bash
wacli-pro channels mute --jid 123456789@newsletter
wacli-pro channels unmute --jid 123456789@newsletter
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--jid` | string | | Channel JID (required) |

---

## profile

Manage your WhatsApp profile.

### profile set-about

Set your "About" text.

```bash
wacli-pro profile set-about --text "Available"
wacli-pro profile set-about --text ""  # clear
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--text` | string | | About text (required) |

### profile set-photo

Set your profile photo (JPEG).

```bash
wacli-pro profile set-photo --file photo.jpg
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--file` | string | | Path to JPEG photo file (required) |

### profile remove-photo

Remove your profile photo.

```bash
wacli-pro profile remove-photo
```

---

## store

Local store management.

### store stats

Show database statistics.

```bash
wacli-pro store stats
```

Output: store directory, DB file size, message count, chat count, contact count, group count.

---

## media

### media download

Download media for a message.

```bash
wacli-pro media download --chat 1234567890@s.whatsapp.net --id <message-id>
wacli-pro media download --chat 1234567890@s.whatsapp.net --id <message-id> --output ./downloads/
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--chat` | string | | Chat JID (required) |
| `--id` | string | | Message ID (required) |
| `--output` | string | | Output file or directory (default: store media dir) |

---

## history

### history backfill

Request older messages for a chat from your primary device (on-demand history sync).

Your primary device (phone) must be online. This is best-effort and WhatsApp may not return full history.

```bash
wacli-pro history backfill --chat 1234567890@s.whatsapp.net
wacli-pro history backfill --chat 1234567890@s.whatsapp.net --requests 10 --count 50
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--chat` | string | | Chat JID (required) |
| `--count` | int | `50` | Messages to request per sync |
| `--requests` | int | `1` | Number of on-demand requests to attempt |
| `--wait` | duration | `60s` | Time to wait for response per request |
| `--idle-exit` | duration | `5s` | Exit after being idle |

**Backfill all chats (script):**

```bash
wacli-pro --json chats list --limit 100000 \
  | jq -r '.[].JID' \
  | while read -r jid; do
      wacli-pro history backfill --chat "$jid" --requests 3 --count 50
    done
```

---

## presence

Send typing and recording indicators to a chat.

### presence typing

Send a "composing" (typing) indicator.

```bash
wacli-pro presence typing --to 1234567890
wacli-pro presence typing --to 1234567890 --media audio  # recording indicator
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--to` | string | | Recipient phone number or JID (required) |
| `--media` | string | | Media type: `audio` for recording indicator |

### presence paused

Send a "paused" indicator (stop typing).

```bash
wacli-pro presence paused --to 1234567890
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--to` | string | | Recipient phone number or JID (required) |

---

## status

Post to WhatsApp Status (stories).

### status text

Post a text status update.

```bash
wacli-pro status text --text "Hello world"
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--text` | string | | Status text (required) |

### status file

Post an image or video as a status update.

```bash
wacli-pro status file --file photo.jpg
wacli-pro status file --file video.mp4 --caption "Check this out"
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--file` | string | | Path to image/video file (required) |
| `--caption` | string | | Caption for the status |
| `--filename` | string | | Display name override |
| `--mime` | string | | Override detected MIME type |

---

## daemon

Run background sync as a **native OS service** that auto-starts on boot and
restarts on crash. `daemon start` registers wacli with the platform service
manager:

- **Linux / WSL** — a systemd *user* service (no root required; starts on boot
  when user lingering is enabled: `loginctl enable-linger $USER`).
- **macOS** — a launchd LaunchAgent.
- **Windows** — a Windows service (run from an elevated/admin prompt).

The service runs `sync --follow` against the configured store and keeps an IPC
socket open so other commands (e.g. `send`) delegate to the live connection.

### daemon start

Install (or refresh) the service and start it. Safe to re-run — it reinstalls
the unit so it always matches the current binary path and options.

```bash
wacli-pro daemon start
wacli-pro daemon start --download-media --refresh-contacts --refresh-groups
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--download-media` | bool | `false` | Download media during sync |
| `--refresh-contacts` | bool | `false` | Refresh contacts on each start |
| `--refresh-groups` | bool | `false` | Refresh groups on each start |

### daemon stop

Stop the service. It stays installed and will start again on next boot — use
`daemon uninstall` to remove it entirely.

```bash
wacli-pro daemon stop
```

### daemon restart

Restart the service.

```bash
wacli-pro daemon restart
```

### daemon status

Show whether the service is installed and running, plus its store and log path.

```bash
wacli-pro daemon status
```

### daemon uninstall

Stop the service and remove it from the OS (it will no longer start on boot).

```bash
wacli-pro daemon uninstall
```

### daemon logs

Show daemon log output.

```bash
wacli-pro daemon logs
wacli-pro daemon logs -f       # follow (like tail -f)
wacli-pro daemon logs -n 100   # last 100 lines
```

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--follow` | `-f` | bool | `false` | Follow log output (like tail -f) |
| `--lines` | `-n` | int | `50` | Number of lines to show |

---

## doctor

Diagnostics for store, auth, and search status.

```bash
wacli-pro doctor
wacli-pro doctor --connect  # also test WhatsApp connection
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--connect` | bool | `false` | Try connecting to WhatsApp (requires store lock) |

Output includes:
- Store directory path
- Lock status
- Authentication status
- Connection status (with `--connect`)
- FTS5 (full-text search) status

---

## version

Print the wacli-pro version.

```bash
wacli-pro version
```

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `WACLI_PRO_STORE_DIR` | Override the store directory (default: `~/.wacli-pro`). Equivalent to `--store` |
| `WACLI_PRO_DEVICE_LABEL` | Custom device label shown in WhatsApp linked devices |
| `WACLI_PRO_DEVICE_PLATFORM` | Device platform override (defaults to `CHROME`) |

---

## JID Formats

WhatsApp uses JIDs (Jabber IDs) to identify chats and contacts:

| Format | Description |
|--------|-------------|
| `1234567890@s.whatsapp.net` | Individual chat (phone number) |
| `123456789@g.us` | Group chat |
| `status@broadcast` | Status broadcast |

Most `--to` flags accept either a phone number (digits only) or a full JID. Phone numbers are automatically converted to `<number>@s.whatsapp.net`.
