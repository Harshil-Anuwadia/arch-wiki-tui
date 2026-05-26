# archwiki-tui

Minimalist terminal browser for the Arch Wiki. No browser overhead, no context switching. Just the documentation you need, formatted for the terminal you live in.

I use Arch btw, and I want my wiki access to be just as efficient as my workflow.

### Features

- **Fast**: Local title index for instant fuzzy search.
- **Readable**: Dedicated rendering engine for MediaWiki tables and callouts.
- **Clipboard**: Hit `c` to yank code blocks instantly.
- **Offline**: Cache pages for when you're troubleshooting without a connection.
- **Keyboard-driven**: Vim-like navigation (mostly).

### Installation

```bash
curl -sL https://raw.githubusercontent.com/Harshil-Anuwadia/arch-wiki-tui/master/install.sh | sudo bash
```

*Note: Requires `go`, `make`, and `gcc` to build from source.*

### Usage

```bash
archwiki          # Open home screen
archwiki <query>  # Search directly
```

### Keybindings

- `/`: Search
- `j/k` or `arrows`: Navigate
- `Enter`: Open page / Follow link
- `c`: Copy code block under cursor
- `b`: Back in history
- `q` or `Esc`: Quit

### Why?

Because opening a Firefox tab to check a kernel parameter feels wrong when you're already in the TTY.

---

*Built with Bubble Tea. Keep it simple.*
