<div align="center">

# 🏛️ archwiki-tui

**The definitive terminal browser for the Arch Wiki.**

[![Go Report Card](https://goreportcard.com/badge/github.com/Harshil-Anuwadia/arch-wiki-tui)](https://goreportcard.com/report/github.com/Harshil-Anuwadia/arch-wiki-tui)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Harshil-Anuwadia/arch-wiki-tui)](https://go.dev/)
[![Beta](https://img.shields.io/badge/status-beta-orange.svg)]()

---

`archwiki-tui` is a performance-focused, keyboard-first terminal client for the Arch Wiki. 
Stop breaking your flow by jumping to a browser—access the world's best Linux documentation directly in your multiplexer.

[Features](#-key-features) • [Installation](#-installation) • [Usage](#-quick-start) • [Contributing](#-contributing)

</div>

## 💡 Why archwiki-tui?

Most TUI wiki clients are simple wrappers around `curl`. `archwiki-tui` is different:
- It **understands** the wiki structure, extracting code blocks and tables specifically for terminal width.
- It **remembers** your journey, building a navigation tree as you browse.
- It **works offline**, so you can fix your system even when your network is down.

---

## 🚀 Installation

### The One-Liner (Standard Linux)
```bash
curl -sL https://raw.githubusercontent.com/Harshil-Anuwadia/arch-wiki-tui/master/install.sh | sudo bash
```

### Manual Build
```bash
git clone https://github.com/Harshil-Anuwadia/arch-wiki-tui.git
cd arch-wiki-tui
make build
# Binary is in ./bin/archwiki
```

---

## ✨ Key Features

- 🔍 **Hybrid Search Engine** — Uses a local title index for instant fuzzy-finding, then pulls live snippets from the API.
- 🎨 **Glamour Rendering** — Beautifully formatted Markdown with terminal-optimized colors and styles.
- 📦 **Offline Bundles** — Press `D` on any page to download it and all its internal links for offline survival.
- 📋 **Code Extraction** — Hit `c` to instantly copy the next command block to your clipboard.
- 🗺️ **Navigation Map** — `Ctrl+Y` shows your browsing history as a visual tree.
- 📑 **Integrated TOC** — `Ctrl+P` lets you jump to any section without scrolling.

---

## ⌨️ Essential Controls

| Key | Action |
|-----|--------|
| `/` | Start Searching |
| `j` / `k` | Scroll Article |
| `Tab` | Next Tab (Article / Related / Links) |
| `c` | Copy Code Block |
| `o` | Open in Browser (External links) |
| `Ctrl+P` | Table of Contents |
| `Ctrl+Y` | Navigation History |
| `q` | Quit / Back |

---

## 🤝 Contributing

This project is in **Beta**. We need help with:
- Testing on different terminal emulators.
- Improving the HTML-to-Markdown conversion engine.
- Adding support for other MediaWiki-based wikis (Gentoo, Debian, etc).

Check out [CONTRIBUTING.md](CONTRIBUTING.md) to get started!

---

## ⚖️ License

Distributed under the MIT License. See `LICENSE` for more information.

<div align="center">
  <sub>Built with ❤️ for the Linux Community</sub>
</div>
