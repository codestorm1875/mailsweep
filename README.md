# mailsweep

`mailsweep` is a terminal UI for finding the heaviest senders in your Gmail account and bulk-moving mail to Trash safely.

It is optimized for large inboxes:

- incremental Gmail sync
- local mailbox cache
- lazy message drilldown loading
- background sender prefetch
- sortable and filterable sender leaderboard
- safer bulk trash with age filters

## What It Does

`mailsweep` scans your Gmail mailbox, groups messages by sender, and ranks senders by storage usage.

From the UI, you can:

- see which senders take the most space
- sort by total size, message count, or sender name
- filter senders quickly
- drill into a sender’s messages
- select individual messages to move to Trash
- bulk-move all messages from a sender to Trash
- restrict bulk actions to messages older than 30, 90, 180, or 365 days

Messages are moved to Gmail Trash, not hard-deleted immediately.

## Requirements

- Go `1.25+`
- a Gmail account
- a Google Cloud OAuth desktop client with Gmail API enabled

## Setup

### 1. Create Google OAuth credentials

`mailsweep` expects `credentials.json` in one of these locations:

- `~/.mailsweep/credentials.json`
- `./credentials.json`

You can create it like this:

1. Open `https://console.cloud.google.com`
2. Create a Google Cloud project
3. Enable the Gmail API
4. Open `APIs & Services -> OAuth consent screen`
5. Create an `External` app
6. Add your Gmail address as a test user
7. Open `APIs & Services -> Credentials`
8. Create an `OAuth client ID`
9. Choose `Desktop app`
10. Download the JSON credentials file

Move it into place:

```bash
mkdir -p ~/.mailsweep
mv ~/Downloads/client_secret_*.json ~/.mailsweep/credentials.json
```

### 2. Run the app

```bash
go run .
```

Or build a binary first:

```bash
go build -o mailsweep .
./mailsweep
```

On first run, `mailsweep` opens a browser-based OAuth flow and stores your token locally in:

- `~/.mailsweep/token.json`

It also stores:

- mailbox cache in `~/.mailsweep/cache.json`
- UI preferences in `~/.mailsweep/prefs.json`

## How To Use

### Leaderboard

The first screen is the sender leaderboard.

It shows:

- sender email
- message count
- total mailbox space used
- a relative usage bar
- age-preview counts once sender details are prefetched

Keys:

- `↑↓` / `j k`: move selection
- `enter`: open sender drilldown
- `D`: bulk-move the selected sender’s mail to Trash
- `/`: start filtering senders
- `s`: cycle sort mode
- `r`: refresh mailbox
- `q`: quit

Sorting cycles through:

- total size
- message count
- sender name

Filtering matches sender email or sender display text.

### Drilldown

The drilldown screen shows messages for one sender.

Keys:

- `↑↓` / `j k`: move selection
- `space`: toggle a message
- `a`: select all / clear all
- `d`: move selected messages to Trash
- `esc`: go back

### Confirm Dialog

Before a trash action runs, `mailsweep` shows a confirmation step.

Keys:

- `y`: confirm
- `n` / `esc`: cancel
- `←→` / `h l`: switch Yes/No
- `[` / `]`: change the age filter

Age filter options:

- all messages
- older than 30 days
- older than 90 days
- older than 180 days
- older than 365 days

## Performance Notes

The app avoids expensive rescans where possible:

- refresh uses Gmail history-based incremental sync when available
- sender message bodies are not fetched during the initial scan
- drilldown data is loaded only when needed
- selected/nearby senders are prefetched in the background
- prefetch cache is bounded in memory

## Development

Format and build:

```bash
gofmt -w .
go build ./...
```

## Safety Notes

- `mailsweep` uses Gmail modify scope
- bulk actions move messages to `TRASH`
- messages may remain recoverable through Gmail Trash depending on Gmail retention behavior

## License

See [LICENSE](LICENSE).
