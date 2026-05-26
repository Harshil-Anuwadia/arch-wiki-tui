# archwiki-tui

A production-ready terminal UI application for searching and reading Arch Wiki pages without leaving your terminal.

## Highlights

- Keyboard-first TUI built with Bubble Tea
- Hybrid search: instant local title index + live API enrichment
- Multi-source API search (prefix, title-only, text) with deduped ranking
- Background title-index refresh with on-disk cache reuse
- Full article reading pane with Markdown rendering via Glamour
- Dedicated render engine for HTML cleanup, conversion, and readable document structuring
- Extracted code blocks with one-key copy (`c`)
- History and page cache persisted on disk
- Browser fallback (`o`) for any page
- CLI query invocation (`archwiki pacman`)
- Local page cache fallback when network requests fail
- Responsive single-pane reading layout
- Simple single-pane reading mode

## Tech Stack

- Go
- Bubble Tea + Bubbles + Lip Gloss
- Glamour
- goquery for structured HTML parsing and block classification
- Arch Wiki MediaWiki API

## Install

### 1) Install Go

Arch Linux:

```bash
sudo pacman -S --needed go
```

If you cannot use sudo, install Go in user space:

```bash
curl -fsSL https://go.dev/dl/go1.25.0.linux-amd64.tar.gz -o /tmp/go.tar.gz
mkdir -p ~/.local
tar -C ~/.local -xzf /tmp/go.tar.gz
export PATH="$HOME/.local/go/bin:$PATH"
```

### 2) Build

```bash
make build
```

### 3) Run

```bash
./bin/archwiki
```

Run directly with a query:

```bash
./bin/archwiki pacman
```

Use offline-only mode (cache required):

```bash
./bin/archwiki --offline
```

Clear local cache and title index:

```bash
./bin/archwiki --clear-cache
```

Download one article into local cache for offline reading:

```bash
./bin/archwiki --download "Pacman"
```

## Keybindings

Global:

- `q` / `Ctrl+Q` / `Ctrl+C`: Quit
- `/`: Search
- `?`: Help

Browse:

- `↑` `↓` / `j` `k`: Scroll
- `Tab` / `Shift+Tab`: Cycle Wiki / Related / Links tabs
- `1` / `2` / `3`: Jump to Wiki / Related / Links tab
- `PgUp` / `PgDn` or `Ctrl+B` / `Ctrl+F`: Page scroll
- `Ctrl+U` / `Ctrl+D`: Half-page scroll
- `g` / `G`: Top / bottom
- `Esc` / `b` / `h`: Back
- `o`: Open page in browser
- `c`: Copy next code block
- `D`: Download current page + internal links into local offline cache
- `x`: Clear local page cache + title index
- `y` / `n` or `Enter` / `Esc`: Confirm or cancel action dialogs

Tabs:

- `Wiki`: Main article content (the "Related articles" section is removed from this tab)
- `Related`: Related article links extracted from the page
- `Links`: All links with text found in the current article

Search:

- Type to search (debounced)
- `↑` `↓` or `Ctrl+N` / `Ctrl+P`: Move results
- `Enter`: Open selected result
- `Esc`: Close search

## Search Indexing

- On first run, archwiki-tui builds a local title index from Arch Wiki article titles.
- The index is cached at `~/.cache/archwiki-tui/title-index.json` and refreshed in the background periodically.
- While offline, title search continues to work from the local index and cached pages.
- Use `D` in browse mode to pre-download the current page bundle for offline reading.
- Use `--download "Title"` to pre-cache an article without launching the TUI.

## Project Layout

- `cmd/archwiki`: CLI entrypoint
- `internal/app`: Bubble Tea model, update loop, rendering
- `internal/render`: Dedicated HTML/text formatting and document rendering engine
- `internal/wiki`: Arch Wiki API client
- `internal/storage`: History/cache persistence
- `internal/ui`: Reusable style system

## Quality Gates

Run all checks:

```bash
make test
make build
```

## License

MIT
