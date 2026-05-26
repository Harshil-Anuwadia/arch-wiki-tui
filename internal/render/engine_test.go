package render

import (
	"regexp"
	"strings"
	"testing"
)

var osc8RE = regexp.MustCompile(`\x1b]8;;[^\x1b]*\x1b\\`)
var csiRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func extractTextCodeBlockLines(md string) []string {
	lines := strings.Split(md, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "```text" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil
	}

	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "```" {
			return lines[start:i]
		}
	}

	return nil
}

func TestBuildMarkdownFromHTML(t *testing.T) {
	engine := NewEngine()

	input := strings.Join([]string{
		"<!doctype html>",
		"<html>",
		"<head><title>Pacman</title></head>",
		"<body>",
		"<nav>navigation</nav>",
		"<article>",
		"<h1>Pacman</h1>",
		"<p>Pacman is Arch Linux package manager used to install and update software safely.</p>",
		"<pre><code>pacman -Syu</code></pre>",
		"</article>",
		"</body>",
		"</html>",
	}, "")

	md, codeBlocks, err := engine.BuildMarkdownFromHTML(input)
	if err != nil {
		t.Fatalf("BuildMarkdownFromHTML error: %v", err)
	}
	if !strings.Contains(md, "Pacman") {
		t.Fatalf("expected markdown to include article title/content, got:\n%s", md)
	}
	if !strings.Contains(md, "package manager") {
		t.Fatalf("expected markdown to include article body, got:\n%s", md)
	}
	if len(codeBlocks) != 1 || codeBlocks[0] != "pacman -Syu" {
		t.Fatalf("expected one extracted code block, got %#v", codeBlocks)
	}
}

func TestBuildMarkdownFromHTMLEmpty(t *testing.T) {
	engine := NewEngine()
	_, _, err := engine.BuildMarkdownFromHTML("   ")
	if err == nil {
		t.Fatalf("expected error for empty html input")
	}
}

func TestBuildMarkdownFromHTMLRendersTable(t *testing.T) {
	engine := NewEngine()

	html := strings.Join([]string{
		"<div class=\"mw-parser-output\">",
		"<h2>Packages</h2>",
		"<table class=\"wikitable\">",
		"<tr><th>Name</th><th>Role</th></tr>",
		"<tr><td>pacman</td><td>Package manager</td></tr>",
		"</table>",
		"</div>",
	}, "")

	md, _, err := engine.BuildMarkdownFromHTML(html)
	if err != nil {
		t.Fatalf("BuildMarkdownFromHTML error: %v", err)
	}
	if !strings.Contains(md, "```text") {
		t.Fatalf("expected fixed-grid table to render in text code block, got:\n%s", md)
	}
	if !strings.Contains(md, "| Name") || !strings.Contains(md, "Role") {
		t.Fatalf("expected rendered table header row, got:\n%s", md)
	}
	if !strings.Contains(md, "| pacman") || !strings.Contains(md, "Package manager") {
		t.Fatalf("expected rendered table data row, got:\n%s", md)
	}

	grid := extractTextCodeBlockLines(md)
	if len(grid) < 4 {
		t.Fatalf("expected rendered fixed-grid table lines, got:\n%s", md)
	}
	width := len([]rune(grid[0]))
	for _, line := range grid {
		if len([]rune(line)) != width {
			t.Fatalf("expected consistent fixed-width grid lines, got line %q with width %d (want %d)", line, len([]rune(line)), width)
		}
	}
}

func TestBuildMarkdownFromHTMLTableAddsHeaderWhenMissing(t *testing.T) {
	engine := NewEngine()

	html := strings.Join([]string{
		"<div class=\"mw-parser-output\">",
		"<table class=\"wikitable\">",
		"<tr><td>Satellite L30</td><td>2017-01-31</td><td>Yes</td></tr>",
		"<tr><td>Satellite C650</td><td>2017-03-02</td><td>No</td></tr>",
		"</table>",
		"</div>",
	}, "")

	md, _, err := engine.BuildMarkdownFromHTML(html)
	if err != nil {
		t.Fatalf("BuildMarkdownFromHTML error: %v", err)
	}

	if !strings.Contains(md, "Column 1") || !strings.Contains(md, "Column 2") {
		t.Fatalf("expected synthetic column headers for tables without <th>, got:\n%s", md)
	}
	if !strings.Contains(md, "Satellite L30") {
		t.Fatalf("expected data row retained in table output, got:\n%s", md)
	}
}

func TestBuildMarkdownFromHTMLTableNormalizesMultilineAndClampsWidth(t *testing.T) {
	engine := NewEngine()

	html := strings.Join([]string{
		"<div class=\"mw-parser-output\">",
		"<table class=\"wikitable\">",
		"<tr><th>Model</th><th>Date</th><th>WiFi</th><th>Bluetooth</th><th>Audio</th><th>Status</th></tr>",
		"<tr><td>Satellite<br>L30 very long model variant for alignment verification</td><td>2017-01-31</td><td>Yes</td><td>Yes</td><td>Yes</td><td>Partial support with additional setup required</td></tr>",
		"</table>",
		"</div>",
	}, "")

	md, _, err := engine.BuildMarkdownFromHTML(html)
	if err != nil {
		t.Fatalf("BuildMarkdownFromHTML error: %v", err)
	}

	if !strings.Contains(md, "Satellite L30") {
		t.Fatalf("expected multiline cell content normalized into a single row, got:\n%s", md)
	}

	grid := extractTextCodeBlockLines(md)
	if len(grid) == 0 {
		t.Fatalf("expected text code block table output, got:\n%s", md)
	}

	for _, line := range grid {
		if len([]rune(line)) > tableMaxWidth {
			t.Fatalf("expected table line width <= %d, got %d for line %q", tableMaxWidth, len([]rune(line)), line)
		}
	}
}

func TestExtractCodeBlocks(t *testing.T) {
	html := `<div><pre><code>pacman -Syu</code></pre><pre>systemctl restart</pre><pre><code>pacman -Syu</code></pre></div>`
	blocks := extractCodeBlocks(html)

	if len(blocks) != 2 {
		t.Fatalf("expected 2 unique code blocks, got %d (%v)", len(blocks), blocks)
	}
	if blocks[0] != "pacman -Syu" {
		t.Fatalf("unexpected first code block: %q", blocks[0])
	}
	if blocks[1] != "systemctl restart" {
		t.Fatalf("unexpected second code block: %q", blocks[1])
	}
}

func TestBuildDocumentRemovesRelatedSectionFromMainContent(t *testing.T) {
	engine := NewEngine()

	input := strings.Join([]string{
		"Related articles",
		"",
		"- [Foo](https://wiki.archlinux.org/title/Foo)",
		"- [Bar](https://wiki.archlinux.org/title/Bar)",
		"",
		"## Installation",
		"Install instructions here.",
	}, "\n")

	doc := engine.BuildDocument(input)
	if strings.Contains(strings.ToLower(doc.WikiMarkdown), "related articles") {
		t.Fatalf("expected related section removed from main markdown, got %q", doc.WikiMarkdown)
	}
	if !strings.Contains(doc.WikiMarkdown, "## 1 Installation") {
		t.Fatalf("expected main markdown to keep article body, got %q", doc.WikiMarkdown)
	}
	if len(doc.RelatedLinks) != 2 {
		t.Fatalf("expected 2 related links, got %d", len(doc.RelatedLinks))
	}
	if len(doc.AllLinks) != 0 {
		t.Fatalf("expected links tab data to exclude related-section links, got %d", len(doc.AllLinks))
	}
}

func TestBuildDocumentSeeAlsoFeedsSemanticRelatedOnly(t *testing.T) {
	engine := NewEngine()

	input := strings.Join([]string{
		"## See also",
		"- [Pacman](https://wiki.archlinux.org/title/Pacman)",
		"- [Anchor](https://wiki.archlinux.org/title/Pacman#Configuration)",
		"- [External](https://example.com/docs)",
		"",
		"## Usage",
		"Use [Systemd](https://wiki.archlinux.org/title/Systemd).",
	}, "\n")

	doc := engine.BuildDocument(input)

	if strings.Contains(strings.ToLower(doc.WikiMarkdown), "see also") {
		t.Fatalf("expected see also section removed from wiki markdown, got %q", doc.WikiMarkdown)
	}
	if len(doc.RelatedLinks) != 1 {
		t.Fatalf("expected only internal non-anchor related links, got %d (%+v)", len(doc.RelatedLinks), doc.RelatedLinks)
	}
	if doc.RelatedLinks[0].Text != "Pacman" {
		t.Fatalf("expected Pacman as semantic related link, got %+v", doc.RelatedLinks[0])
	}
	if len(doc.AllLinks) != 1 || doc.AllLinks[0].Text != "Systemd" {
		t.Fatalf("expected links tab to contain main-content raw links only, got %+v", doc.AllLinks)
	}
}

func TestAddHierarchicalHeadingNumbers(t *testing.T) {
	input := strings.Join([]string{
		"# Intro",
		"",
		"## Install",
		"",
		"### Pacman",
		"",
		"## Usage",
		"",
		"```bash",
		"# do-not-touch",
		"```",
	}, "\n")

	out := addHierarchicalHeadingNumbers(input)

	for _, want := range []string{"# 1 Intro", "## 1.1 Install", "### 1.1.1 Pacman", "## 1.2 Usage"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected numbered heading %q in output:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "# do-not-touch") {
		t.Fatalf("expected fenced code content to remain unchanged")
	}
}

func TestPrepareWikiMarkdownForDisplayStripsVisibleURLs(t *testing.T) {
	engine := NewEngine()

	input := strings.Join([]string{
		"From [Wikipedia](https://en.wikipedia.org/wiki/Device_file):",
		"On Arch Linux, device nodes are managed by udev https://wiki.archlinux.org/title/Udev.",
		"Reference <https://example.com/docs>.",
		"",
		"```bash",
		"echo https://example.com/keep",
		"```",
	}, "\n")

	out, links := engine.PrepareWikiMarkdownForDisplay(input)

	if len(links) != 1 {
		t.Fatalf("expected one extracted markdown link, got %d", len(links))
	}
	if links[0].Label != "Wikipedia" || links[0].URL != "https://en.wikipedia.org/wiki/Device_file" {
		t.Fatalf("unexpected extracted link: %+v", links[0])
	}
	if !strings.Contains(out, "From "+links[0].Token+":") {
		t.Fatalf("expected markdown link token in output, got:\n%s", out)
	}
	if !strings.Contains(out, "managed by udev.") {
		t.Fatalf("expected prose URL to be removed, got:\n%s", out)
	}
	if !strings.Contains(out, "Reference.") {
		t.Fatalf("expected autolink URL to be removed, got:\n%s", out)
	}
	if strings.Contains(out, "https://en.wikipedia.org/wiki/Device_file") {
		t.Fatalf("expected markdown URL to be hidden, got:\n%s", out)
	}
	if strings.Contains(out, "https://wiki.archlinux.org/title/Udev") {
		t.Fatalf("expected inline URL to be hidden, got:\n%s", out)
	}
	if strings.Contains(out, "https://example.com/docs") {
		t.Fatalf("expected autolink URL to be hidden, got:\n%s", out)
	}
	if !strings.Contains(out, "echo https://example.com/keep") {
		t.Fatalf("expected code fence URLs to stay unchanged")
	}
}

func TestPrepareWikiMarkdownForDisplayClassifiesLinks(t *testing.T) {
	engine := NewEngine()

	input := strings.Join([]string{
		"See [Kernel](https://wiki.archlinux.org/title/Kernel), [Boot issues](#Troubleshooting) and [Search](https://duckduckgo.com).",
	}, "\n")

	out, links := engine.PrepareWikiMarkdownForDisplay(input)
	if len(links) != 3 {
		t.Fatalf("expected 3 links, got %d (%+v)", len(links), links)
	}

	if links[0].Type != LinkTypeInternal {
		t.Fatalf("expected internal link type, got %q", links[0].Type)
	}
	if links[1].Type != LinkTypeSection {
		t.Fatalf("expected section link type, got %q", links[1].Type)
	}
	if links[2].Type != LinkTypeExternal {
		t.Fatalf("expected external link type, got %q", links[2].Type)
	}

	for i, link := range links {
		if link.ID != i+1 {
			t.Fatalf("expected link ID %d got %d", i+1, link.ID)
		}
		if link.State != LinkStateNormal {
			t.Fatalf("expected default normal state, got %q", link.State)
		}
	}

	if !strings.Contains(out, links[0].Token) || !strings.Contains(out, links[1].Token) || !strings.Contains(out, links[2].Token) {
		t.Fatalf("expected all link tokens in output, got:\n%s", out)
	}
}

func TestBuildMarkdownFromHTMLKeepsPipeSeparatedLinks(t *testing.T) {
	engine := NewEngine()

	html := strings.Join([]string{
		`<div class="mw-parser-output">`,
		`<dl><dd>`,
		`<a class="external free" href="https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git">https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git</a> || `,
		`<span class="plainlinks archwiki-template-pkg"><a class="external text" href="https://aur.archlinux.org/packages/linux-git/">linux-git</a></span><sup><small>AUR</small></sup>`,
		`</dd></dl>`,
		`</div>`,
	}, "")

	md, _, err := engine.BuildMarkdownFromHTML(html)
	if err != nil {
		t.Fatalf("BuildMarkdownFromHTML error: %v", err)
	}

	out, links := engine.PrepareWikiMarkdownForDisplay(md)
	if len(links) < 2 {
		t.Fatalf("expected at least 2 links in pipe row, got %d (%+v) with markdown:\n%s", len(links), links, md)
	}

	rendered := engine.ApplyWikiRenderLinks(out, links)
	visible := csiRE.ReplaceAllString(osc8RE.ReplaceAllString(rendered, ""), "")
	if !strings.Contains(visible, "linux-git") {
		t.Fatalf("expected package link label present, got:\n%s", visible)
	}
	if strings.Contains(visible, "https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git") {
		t.Fatalf("expected raw source URL hidden in visible output, got:\n%s", visible)
	}
	if !strings.Contains(visible, "||") {
		t.Fatalf("expected pipe separator to remain visible, got:\n%s", visible)
	}
	if !strings.Contains(visible, "linux-git AUR") {
		t.Fatalf("expected spacing between package and AUR marker, got:\n%s", visible)
	}
}

func TestBuildMarkdownFromHTMLRendersCalloutBoxes(t *testing.T) {
	engine := NewEngine()

	html := strings.Join([]string{
		`<div class="mw-parser-output">`,
		`<div class="archwiki-template-box warning">Do not force power-off during package transactions.</div>`,
		`<table class="ambox caution"><tr><td>Backup configuration files before changing bootloader settings.</td></tr></table>`,
		`</div>`,
	}, "")

	md, _, err := engine.BuildMarkdownFromHTML(html)
	if err != nil {
		t.Fatalf("BuildMarkdownFromHTML error: %v", err)
	}

	if !strings.Contains(md, "```text") {
		t.Fatalf("expected callout to render as boxed text block, got:\n%s", md)
	}
	if !strings.Contains(md, "WARNING") {
		t.Fatalf("expected warning callout kind in rendered markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "┌─") || !strings.Contains(md, "┐") || !strings.Contains(md, "└") || !strings.Contains(md, "┘") {
		t.Fatalf("expected complete box corners in callout markdown, got:\n%s", md)
	}
	if !strings.Contains(md, " │") {
		t.Fatalf("expected right-side box border in callout markdown, got:\n%s", md)
	}
}

func TestPrepareWikiMarkdownForDisplayClassifiesReferenceLinks(t *testing.T) {
	engine := NewEngine()

	input := "See [ref](#cite_note-1), [internal](https://wiki.archlinux.org/title/Pacman) and [section](#Usage)."
	out, links := engine.PrepareWikiMarkdownForDisplay(input)

	if len(links) != 3 {
		t.Fatalf("expected 3 links, got %d", len(links))
	}
	if links[0].Type != LinkTypeReference {
		t.Fatalf("expected first link to be reference, got %q", links[0].Type)
	}
	if links[1].Type != LinkTypeInternal {
		t.Fatalf("expected second link to be internal, got %q", links[1].Type)
	}
	if links[2].Type != LinkTypeSection {
		t.Fatalf("expected third link to be section, got %q", links[2].Type)
	}
	if !strings.Contains(out, links[0].Token) {
		t.Fatalf("expected reference token in output, got:\n%s", out)
	}
}

func TestApplyCalloutStylesColorsKinds(t *testing.T) {
	engine := NewEngine()

	input := strings.Join([]string{
		"before",
		"┌─ NOTE ────────────┐",
		"│ Keep mirrors synchronized. │",
		"└────────────────────┘",
		"",
		"┌─ TIP ─────────────┐",
		"│ Use pacman -Syu first.    │",
		"└────────────────────┘",
		"",
		"┌─ WARNING ─────────┐",
		"│ Do not interrupt upgrades. │",
		"└────────────────────┘",
		"after",
	}, "\n")

	out := engine.ApplyCalloutStyles(input)

	if !strings.Contains(out, "\x1b[38;5;117m") {
		t.Fatalf("expected NOTE callout to use blue color")
	}
	if !strings.Contains(out, "\x1b[38;5;78m") {
		t.Fatalf("expected TIP callout to use green color")
	}
	if !strings.Contains(out, "\x1b[38;5;203m") {
		t.Fatalf("expected WARNING callout to use red color")
	}
	if !strings.Contains(out, "before") || !strings.Contains(out, "after") {
		t.Fatalf("expected non-callout lines to stay present")
	}
}

func TestBuildMarkdownFromHTMLCalloutStripsLongLinkTargetsAndKeepsClosedBox(t *testing.T) {
	engine := NewEngine()

	html := strings.Join([]string{
		`<div class="mw-parser-output">`,
		`<div class="archwiki-template-box note">`,
		`Arch Linux installation images do not support Secure Boot.`,
		`You will need to <a href="https://wiki.archlinux.org/title/Unified_Extensible_Firmware_Interface/Secure_Boot#Disabling_Secure_Boot">disable Secure Boot</a>`,
		`for installation media.`,
		`</div>`,
		`</div>`,
	}, "")

	md, _, err := engine.BuildMarkdownFromHTML(html)
	if err != nil {
		t.Fatalf("BuildMarkdownFromHTML error: %v", err)
	}

	if strings.Contains(md, "https://wiki.archlinux.org/title/Unified_Extensible_Firmware_Interface/Secure_Boot") {
		t.Fatalf("expected callout markdown to hide long URL targets, got:\n%s", md)
	}
	if !strings.Contains(md, "disable Secure Boot") {
		t.Fatalf("expected callout markdown to keep link label, got:\n%s", md)
	}
	if !strings.Contains(md, "┌─ NOTE") || !strings.Contains(md, "┐") || !strings.Contains(md, "└") || !strings.Contains(md, "┘") {
		t.Fatalf("expected closed callout box corners, got:\n%s", md)
	}
	if !strings.Contains(md, " │") {
		t.Fatalf("expected right-side vertical border in callout box, got:\n%s", md)
	}
}

func TestArchWikiTitleFromURL(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		title string
		ok    bool
	}{
		{name: "title path", raw: "https://wiki.archlinux.org/title/Pacman", title: "Pacman", ok: true},
		{name: "title query", raw: "https://wiki.archlinux.org/index.php?title=PipeWire", title: "PipeWire", ok: true},
		{name: "relative title path", raw: "/title/Systemd", title: "Systemd", ok: true},
		{name: "relative index query", raw: "/index.php?title=NetworkManager", title: "NetworkManager", ok: true},
		{name: "encoded title", raw: "https://wiki.archlinux.org/title/NVIDIA_%28%C4%8Ce%C5%A1tina%29", title: "NVIDIA (Čeština)", ok: true},
		{name: "external host", raw: "https://en.wikipedia.org/wiki/Pacman", ok: false},
		{name: "empty", raw: "", ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			title, ok := ArchWikiTitleFromURL(tc.raw)
			if ok != tc.ok {
				t.Fatalf("expected ok=%v got ok=%v title=%q", tc.ok, ok, title)
			}
			if title != tc.title {
				t.Fatalf("expected title=%q got %q", tc.title, title)
			}
		})
	}
}
