# archwiki-tui

A fast, keyboard-driven terminal client for searching and reading the Arch Wiki. 

Built for users who live in the terminal and want to access the wiki's wealth of knowledge without the context-switch of opening a web browser.

## Features

- **Hybrid Search**: Instant results via local title index, supplemented by live API hits.
- **Clean Rendering**: Articles are formatted for the terminal with clear headers, lists, and callouts (Warnings, Notes, Tips).
- **Offline First**: Cache articles as you read, or sync the entire wiki for offline use.
- **Copy-Paste Flow**: One-key copy (`c`) for code blocks.
- **Navigation Map**: Track your rabbit hole with a visual history tree (`Ctrl+Y`).
- **Table of Contents**: Jump between sections quickly (`Ctrl+P`).

## Installation

### Using the installer (Recommended)
You can install `archwiki` directly to `/usr/local/bin` using the provided script:

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

Or jump straight to a search query:
```bash
archwiki pacman
```

### Essential Keybindings

- `/` : Search
- `j/k` or `Arrows` : Scroll
- `Tab` : Cycle between Article, Related Articles, and Links
- `c` : Copy the next code block to clipboard
- `Ctrl+P` : Open Table of Contents
- `Ctrl+Y` : Open Navigation Map
- `q` or `Esc` : Back / Quit

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details on how to get started.

## License

MIT - see [LICENSE](LICENSE) for details.
