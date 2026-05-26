# archwiki-tui

`archwiki-tui` is a terminal client for searching and reading the Arch Wiki. It is built for users who want to stay in their terminal workflow and avoid the context-switching that comes with opening a web browser for documentation.

It doesn't just wrap `curl`; it uses a dedicated rendering engine to convert wiki HTML into structured, readable documents with support for tables, callout boxes, and code block extraction.

## Features

- **Fast Search**: Uses a local title index for instant fuzzy-finding, with live API enrichment.
- **Technical Rendering**: Specifically handles MediaWiki tables and styled callouts (Warnings, Notes, Tips) for the terminal.
- **Offline Reading**: Support for local page caching and downloading page bundles for offline survival.
- **Developer Workflow**: Single-key (`c`) extraction of code blocks to your clipboard.
- **Breadcrumbs**: A visual navigation tree (`Ctrl+Y`) to track your browsing history across pages.
- **Deep Links**: Tab-based access to related articles and all external links found in the text.

## Installation

### The Quick Way
This script clones the repo, builds the binary, and moves it to `/usr/local/bin`.

```bash
curl -sL https://raw.githubusercontent.com/Harshil-Anuwadia/arch-wiki-tui/master/install.sh | sudo bash
```

### From Source
Requires Go 1.25+.

```bash
git clone https://github.com/Harshil-Anuwadia/arch-wiki-tui.git
cd arch-wiki-tui
make build
./bin/archwiki
```

## Usage

Start the interactive UI:
```bash
archwiki
```

Pass a query directly:
```bash
archwiki systemd
```

### Essential Keys

- `/` : Search
- `j` / `k` : Scroll article
- `Tab` : Cycle through tabs (Article / Related / Links)
- `c` : Copy next code block to clipboard
- `o` : Open current page in browser
- `Ctrl+P` : Jump to section (TOC)
- `Ctrl+Y` : Navigation map
- `q` : Back / Quit

## Contributing

The project is in beta. We are particularly interested in fixes for the HTML-to-Markdown engine and feedback on terminal emulator compatibility. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

MIT
