# archwiki-tui

A terminal browser for the Arch Wiki. No bloat. No browser. Just the wiki.

Because if you're already in a TTY fixing your bootloader, you shouldn't have to launch X11 just to read about kernel parameters.

![archwiki-tui demo](assets/demo.gif)

**Note:** This is an initial beta release. Expect bugs. If it breaks your workflow, you get to keep the pieces. PRs welcome.


## Installation

### The AUR Way (Recommended for Arch Users)
```bash
yay -S archwiki-tui-git
```

### The Quick Way (Other Distros)
```bash
curl -sL https://raw.githubusercontent.com/Harshil-Anuwadia/archwiki-tui/master/install.sh | sudo bash
```

### The Arch Way (Manual)
Requires `go`, `make`, and `gcc`.

```bash
git clone https://github.com/Harshil-Anuwadia/archwiki-tui
cd archwiki-tui
make build
sudo cp bin/archwiki /usr/local/bin/
```

## Usage

```bash
archwiki          # Home screen
archwiki <query>  # Direct search
```

## Keybindings

* `/` : Search
* `j`/`k` : Navigate (Vim keys, obviously)
* `Enter` : Open page or follow link
* `c` : Copy code block to clipboard
* `b` : Go back in history
* `q` : Quit

## Why this exists

1. **KISS**: It does one thing and does it well.
2. **Efficiency**: Fuzzy searching titles is faster than navigating a website.
3. **Utility**: `c` to yank code blocks directly into your terminal saves time when you're lazy (and we all are).
4. **Offline**: It caches what you read, because sometimes you break your networking.

## Contributing

Contributions are welcome. If you want to help improve the project:

1. Check the [CONTRIBUTING.md](CONTRIBUTING.md) guide.
2. Use the [Issue Templates](https://github.com/Harshil-Anuwadia/archwiki-tui/issues/new/choose) for bugs and features.
3. Submit a PR.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for release history.

## License

[MIT](LICENSE). 

---
*I use Arch btw.*
