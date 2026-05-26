package app

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"archwiki-tui/internal/render"
	"archwiki-tui/internal/wiki"
	tea "github.com/charmbracelet/bubbletea"
)

var osc8RE = regexp.MustCompile(`\x1b]8;;[^\x1b]*\x1b\\`)

func TestEnterFromSearchStartsPageOpenImmediately(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	m.mode = modeSearch
	m.searchInput.SetValue("pacman")
	m.searchQuery = "pacman"
	m.searchResults = []wiki.SearchResult{{Title: "Pacman"}}
	m.searchCursor = 0

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*Model)

	if m2.mode != modeBrowse {
		t.Fatalf("expected modeBrowse after pressing enter, got %v", m2.mode)
	}
	if !m2.pageLoading {
		t.Fatalf("expected pageLoading to be true after enter")
	}
	if !m2.loading {
		t.Fatalf("expected loading to be true after enter")
	}
	if cmd == nil {
		t.Fatalf("expected open-page command to be returned")
	}
}

func TestEnterFromHomeOpensSuggestedPage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	m.homeTopSearch = "Installation guide"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*Model)

	if m2.mode != modeBrowse {
		t.Fatalf("expected browse mode after Enter from home, got %v", m2.mode)
	}
	if !m2.pageLoading {
		t.Fatalf("expected pageLoading to be true after Enter from home")
	}
	if !m2.loading {
		t.Fatalf("expected loading to be true after Enter from home")
	}
	if cmd == nil {
		t.Fatalf("expected open-page command to be returned from home Enter")
	}
	if !strings.Contains(m2.status, "Opening: Installation guide") {
		t.Fatalf("expected status to show opening suggestion, got %q", m2.status)
	}
}

func TestCtrlPOpensTOCPopupInWikiReading(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	md := strings.Join([]string{
		"# Intro",
		"Body",
		"",
		"## Install",
		"Steps",
	}, "\n")

	m.currentPage = &wiki.Page{Title: "Test", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m2 := updated.(*Model)

	if cmd != nil {
		t.Fatalf("did not expect command while opening TOC popup")
	}
	if !m2.showTOC {
		t.Fatalf("expected TOC popup to be visible after Ctrl+P")
	}
	if len(m2.tocItems) < 2 {
		t.Fatalf("expected TOC items to be extracted from headings, got %d", len(m2.tocItems))
	}
}

func TestCtrlYOpensNavigationMapPopup(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	md := "## Leaf\ncontent\n"
	m.backStack = []string{"PipeWire", "Audio"}
	m.currentPage = &wiki.Page{Title: "ALSA clients", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()
	m.ensureNavigationPath()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	m2 := updated.(*Model)

	if cmd != nil {
		t.Fatalf("did not expect command while opening navigation map")
	}
	if !m2.showNav {
		t.Fatalf("expected navigation map popup to open on Ctrl+Y")
	}
	if len(m2.navNodes) < 3 {
		t.Fatalf("expected navigation tree nodes to be present, got %d", len(m2.navNodes))
	}
}

func TestNavigationMapEnterJumpUpdatesPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	md := "## Node\ncontent\n"

	m.backStack = []string{"PipeWire", "Audio"}
	m.currentPage = &wiki.Page{Title: "ALSA clients", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()
	m.ensureNavigationPath()

	m.backStack = []string{"PipeWire", "Video"}
	m.currentPage = &wiki.Page{Title: "WebRTC", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()
	m.ensureNavigationPath()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	m2 := updated.(*Model)
	if !m2.showNav {
		t.Fatalf("expected navigation map popup to open")
	}

	target := -1
	for i, node := range m2.navNodes {
		if strings.EqualFold(node.Title, "ALSA clients") {
			target = i
			break
		}
	}
	if target < 0 {
		t.Fatalf("expected ALSA clients node in navigation tree")
	}

	m2.navCursor = target
	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(*Model)

	if m3.showNav {
		t.Fatalf("expected navigation map popup to close after Enter jump")
	}
	if cmd == nil {
		t.Fatalf("expected open-page command after navigation jump")
	}

	if len(m3.backStack) != 2 || !strings.EqualFold(m3.backStack[0], "PipeWire") || !strings.EqualFold(m3.backStack[1], "Audio") {
		t.Fatalf("expected back path [PipeWire, Audio], got %v", m3.backStack)
	}
}

func TestTOCPopupEnterJumpsToSelectedHeading(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	md := "# Intro\n\n" + strings.Repeat("filler line for scrolling\n", 80) + "\n## Target Section\nTarget body\n"

	m.currentPage = &wiki.Page{Title: "Test", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m2 := updated.(*Model)
	if !m2.showTOC || len(m2.tocItems) < 2 {
		t.Fatalf("expected TOC popup with headings")
	}

	m2.tocCursor = len(m2.tocItems) - 1
	targetLine := m2.tocItems[m2.tocCursor].MarkdownLine
	before := m2.viewport.YOffset
	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(*Model)

	if m3.showTOC {
		t.Fatalf("expected TOC popup to close after Enter jump")
	}
	if m3.viewport.YOffset <= before {
		t.Fatalf("expected viewport to jump forward, before=%d after=%d", before, m3.viewport.YOffset)
	}
	if cmd == nil {
		t.Fatalf("expected transient status command after TOC jump")
	}
	if !m3.tocJumpHighlight {
		t.Fatalf("expected jump highlight to be active after TOC jump")
	}
	if m3.tocJumpLine != targetLine {
		t.Fatalf("expected jump highlight line %d, got %d", targetLine, m3.tocJumpLine)
	}
}

func TestNearestTOCIndexUsesRenderedOffsetsForWrappedContent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 72, Height: 36})
	m = model.(*Model)

	md := strings.Join([]string{
		"## First",
		strings.Repeat("This is a long wrapped paragraph to stress TOC offset mapping. ", 180),
		"## Second",
		"short",
		"## Third",
	}, "\n")

	m.currentPage = &wiki.Page{Title: "Wrapped", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()
	m.tocItems = buildTOCItems(m.wikiMarkdown)

	if len(m.tocItems) < 3 {
		t.Fatalf("expected at least 3 headings for mapping test, got %d", len(m.tocItems))
	}

	secondOffset := m.renderedOffsetForMarkdownLine(m.tocItems[1].MarkdownLine)
	if secondOffset <= 0 {
		t.Fatalf("expected second heading rendered offset > 0, got %d", secondOffset)
	}

	m.viewport.YOffset = secondOffset
	if got := m.nearestTOCIndexForViewport(); got != 1 {
		t.Fatalf("expected nearest TOC index 1 at second heading offset, got %d", got)
	}
}

func TestScrollContentSyncsTOCCursorToViewport(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 36})
	m = model.(*Model)

	md := strings.Join([]string{
		"## Intro",
		strings.Repeat("filler line for scroll sync\n", 90),
		"## Target",
		"target body",
		strings.Repeat("tail line for scroll sync\n", 60),
	}, "\n")

	m.currentPage = &wiki.Page{Title: "Sync", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()
	m.tocItems = buildTOCItems(m.wikiMarkdown)

	if len(m.tocItems) < 2 {
		t.Fatalf("expected at least 2 headings for sync test, got %d", len(m.tocItems))
	}

	targetOffset := m.renderedOffsetForMarkdownLine(m.tocItems[1].MarkdownLine)
	if targetOffset <= 0 {
		t.Fatalf("expected target heading offset > 0, got %d", targetOffset)
	}

	m.tocCursor = 0
	m.scrollContent(targetOffset)
	if m.tocCursor != 1 {
		t.Fatalf("expected TOC cursor to sync to second heading after scroll, got %d", m.tocCursor)
	}
}

func TestTOCPopupDefaultsToCollapsedTree(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	md := strings.Join([]string{
		"## Alpha",
		"alpha text",
		"### Alpha Child",
		strings.Repeat("filler\n", 60),
		"## Beta",
		"beta text",
		"### Beta Child",
	}, "\n")

	m.currentPage = &wiki.Page{Title: "Test", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()
	m.viewport.YOffset = m.maxViewportOffset()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m2 := updated.(*Model)

	if !m2.showTOC {
		t.Fatalf("expected TOC popup to open")
	}
	if len(m2.tocItems) < 4 {
		t.Fatalf("expected multiple TOC headings for collapse test")
	}

	visible := m2.visibleTOCIndices()
	if len(visible) >= len(m2.tocItems) {
		t.Fatalf("expected collapsed tree to hide some headings: visible=%d total=%d", len(visible), len(m2.tocItems))
	}

	for _, idx := range m2.tocPathToRoot(m2.tocCursor) {
		if !m2.tocExpanded[idx] {
			t.Fatalf("expected active heading path to be expanded, index=%d", idx)
		}
	}
}

func TestTOCPopupArrowKeysExpandCollapseAndParentNavigation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	md := strings.Join([]string{
		"# Parent",
		"body",
		"## Child",
		"child body",
		"### Grandchild",
		"grand body",
		"# Other",
	}, "\n")

	m.currentPage = &wiki.Page{Title: "Test", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m2 := updated.(*Model)
	if !m2.showTOC {
		t.Fatalf("expected TOC popup to open")
	}

	m2.tocExpanded = map[int]bool{}
	m2.tocCursor = 0
	beforeVisible := len(m2.visibleTOCIndices())

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRight})
	m3 := updated.(*Model)
	if !m3.tocExpanded[0] {
		t.Fatalf("expected right arrow to expand selected heading")
	}
	afterExpand := len(m3.visibleTOCIndices())
	if afterExpand <= beforeVisible {
		t.Fatalf("expected visible headings to increase after expand, before=%d after=%d", beforeVisible, afterExpand)
	}

	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyDown})
	m4 := updated.(*Model)
	if m4.tocCursor == 0 {
		t.Fatalf("expected down arrow to move cursor to child heading")
	}

	updated, _ = m4.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m5 := updated.(*Model)
	if m5.tocCursor != 0 {
		t.Fatalf("expected left arrow on child to move cursor to parent, got %d", m5.tocCursor)
	}

	updated, _ = m5.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m6 := updated.(*Model)
	if m6.tocExpanded[0] {
		t.Fatalf("expected second left arrow on expanded parent to collapse it")
	}
	if len(m6.visibleTOCIndices()) != beforeVisible {
		t.Fatalf("expected collapse to restore visible count to %d, got %d", beforeVisible, len(m6.visibleTOCIndices()))
	}
}

func TestBuildTOCItemsSkipsDeepNoisyH1Lines(t *testing.T) {
	md := strings.Join([]string{
		"## Installation",
		"text",
		"### Usage",
		"text",
		strings.Repeat("filler\n", 25),
		"# We do not start with dmix, but with an input device.",
		"# Do not forget to add an input device.",
		"### Troubleshooting",
	}, "\n")

	items := buildTOCItems(md)
	if len(items) != 3 {
		t.Fatalf("expected only real headings in TOC, got %d items", len(items))
	}

	if items[0].Title != "Installation" || items[0].Level != 1 {
		t.Fatalf("expected normalized top heading Installation at level 1, got title=%q level=%d", items[0].Title, items[0].Level)
	}
	if items[1].Title != "Usage" || items[1].Level != 2 {
		t.Fatalf("expected Usage subheading at level 2, got title=%q level=%d", items[1].Title, items[1].Level)
	}
	if items[2].Title != "Troubleshooting" || items[2].Level != 2 {
		t.Fatalf("expected Troubleshooting subheading at level 2, got title=%q level=%d", items[2].Title, items[2].Level)
	}
}

func TestTOCJumpHighlightClearsOnKeyPress(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	md := strings.Join([]string{
		"## Intro",
		"body",
		"## Target",
		"target body",
	}, "\n")

	m.currentPage = &wiki.Page{Title: "Test", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()
	m.tocItems = buildTOCItems(m.wikiMarkdown)

	_ = m.jumpToTOCItem(1)
	if !m.tocJumpHighlight {
		t.Fatalf("expected jump highlight to be active after jump")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m2 := updated.(*Model)
	if m2.tocJumpHighlight {
		t.Fatalf("expected jump highlight to clear on key press")
	}
}

func TestTOCJumpHighlightClearsOnMouseActivity(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	md := strings.Join([]string{
		"## Intro",
		"body",
		"## Target",
		"target body",
	}, "\n")

	m.currentPage = &wiki.Page{Title: "Test", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()
	m.tocItems = buildTOCItems(m.wikiMarkdown)

	_ = m.jumpToTOCItem(1)
	if !m.tocJumpHighlight {
		t.Fatalf("expected jump highlight to be active after jump")
	}

	updated, _ := m.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 0, Y: 0})
	m2 := updated.(*Model)
	if m2.tocJumpHighlight {
		t.Fatalf("expected jump highlight to clear on mouse activity")
	}
}

func TestCtrlQQuitsApplication(t *testing.T) {
	m := NewModel(Config{})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	_ = updated.(*Model)

	if cmd == nil {
		t.Fatalf("expected quit command for Ctrl+Q")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from Ctrl+Q command")
	}
}

func TestSearchModeAllowsTypingQAndK(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	m.mode = modeSearch
	m.searchInput.Focus()
	m.searchResults = []wiki.SearchResult{{Title: "One"}, {Title: "Two"}, {Title: "Three"}}
	m.searchCursor = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m2 := updated.(*Model)
	if m2.searchInput.Value() != "k" {
		t.Fatalf("expected search input to contain typed k, got %q", m2.searchInput.Value())
	}
	if m2.searchCursor != 1 {
		t.Fatalf("expected search cursor to remain unchanged, got %d", m2.searchCursor)
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m3 := updated.(*Model)
	if m3.searchInput.Value() != "kq" {
		t.Fatalf("expected search input to contain typed q, got %q", m3.searchInput.Value())
	}
	if m3.mode != modeSearch {
		t.Fatalf("expected to stay in search mode after typing q")
	}
}

func TestSearchModeEscClearsThenCloses(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	m.mode = modeSearch
	m.searchInput.Focus()
	m.searchInput.SetValue("how to")
	m.searchResults = []wiki.SearchResult{{Title: "How to install"}}
	m.searchCursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(*Model)

	if m2.mode != modeSearch {
		t.Fatalf("expected first Esc to keep search mode open")
	}
	if m2.searchInput.Value() != "" {
		t.Fatalf("expected first Esc to clear query, got %q", m2.searchInput.Value())
	}
	if len(m2.searchResults) != 0 {
		t.Fatalf("expected first Esc to clear results")
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := updated.(*Model)
	if m3.mode != modeBrowse {
		t.Fatalf("expected second Esc to close search mode")
	}
}

func TestOpenPageCancelsSearchLoadingAndResetsTabState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	m.searchLoading = true
	m.loading = true
	m.activeSearchReq = 4
	m.currentPage = &wiki.Page{Title: "Current"}
	m.wikiMarkdown = "# Current"
	m.relatedMarkdown = "## Related"
	m.linksMarkdown = "## Links"
	m.relatedLinks = []articleLink{{Text: "A", URL: "https://example.com/a"}}
	m.allLinks = []articleLink{{Text: "B", URL: "https://example.com/b"}}
	m.activeTab = tabLinks

	cmd := m.openPage("Pacman", false)
	if cmd == nil {
		t.Fatalf("expected open-page command")
	}
	if m.searchLoading {
		t.Fatalf("expected search loading to be canceled")
	}
	if m.activeSearchReq != 5 {
		t.Fatalf("expected activeSearchReq to increment, got %d", m.activeSearchReq)
	}
	if !m.pageLoading || !m.loading {
		t.Fatalf("expected page loading state to be active")
	}
	if m.currentPage == nil {
		t.Fatalf("expected currentPage to remain visible until new page loads")
	}
	if m.activeTab != tabLinks {
		t.Fatalf("expected activeTab to remain unchanged while loading")
	}
	if len(m.relatedLinks) == 0 || len(m.allLinks) == 0 {
		t.Fatalf("expected tab link state to remain intact while loading")
	}
}

func TestOfflineDownloadTitlesFiltersAndLimits(t *testing.T) {
	m := NewModel(Config{})
	m.currentPage = &wiki.Page{Title: "Pacman"}
	m.allLinks = []articleLink{
		{Text: "Systemd", URL: "https://wiki.archlinux.org/title/Systemd"},
		{Text: "External", URL: "https://example.com/docs"},
		{Text: "Duplicate", URL: "/title/Systemd"},
		{Text: "PipeWire", URL: "/index.php?title=PipeWire"},
		{Text: "GRUB", URL: "https://wiki.archlinux.org/title/GRUB"},
	}

	titles, skipped := m.offlineDownloadTitles(3)
	if len(titles) != 3 {
		t.Fatalf("expected 3 titles (including current page), got %d: %#v", len(titles), titles)
	}
	if titles[0] != "Pacman" || titles[1] != "Systemd" || titles[2] != "PipeWire" {
		t.Fatalf("unexpected title selection order: %#v", titles)
	}
	if skipped != 1 {
		t.Fatalf("expected one skipped title due to limit, got %d", skipped)
	}
}

func TestBrowseModeXClearsCacheAndResetsSearchState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	if m.store == nil {
		t.Fatalf("expected store to be initialized for cache-clear test")
	}

	if err := m.store.WritePageCache("Pacman", "# Pacman"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}
	m.searchResults = []wiki.SearchResult{{Title: "Pacman"}}
	m.searchQuery = "pac"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m2 := updated.(*Model)
	if cmd != nil {
		t.Fatalf("did not expect clear-cache command before confirmation")
	}
	if m2.confirmDialog == nil {
		t.Fatalf("expected confirmation dialog after pressing x")
	}
	if m2.confirmDialog.action != confirmActionClearCache {
		t.Fatalf("expected clear-cache confirmation action, got %v", m2.confirmDialog.action)
	}

	updated, cmd = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m3 := updated.(*Model)
	if m3.confirmDialog != nil {
		t.Fatalf("expected confirmation dialog to close after yes")
	}
	if cmd == nil {
		t.Fatalf("expected clear-cache command after confirming yes")
	}

	msg := cmd()
	clearMsg, ok := msg.(cacheClearedMsg)
	if !ok {
		t.Fatalf("expected cacheClearedMsg, got %T", msg)
	}
	if clearMsg.err != nil {
		t.Fatalf("clear cache command returned error: %v", clearMsg.err)
	}

	updated, _ = m3.Update(clearMsg)
	m4 := updated.(*Model)
	if len(m4.searchResults) != 0 || m4.searchQuery != "" {
		t.Fatalf("expected search state reset after cache clear")
	}
}

func TestBrowseModeDownloadRequiresConfirmation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	m.currentPage = &wiki.Page{Title: "Pacman", Markdown: "# Pacman"}
	m.allLinks = []articleLink{{Text: "Systemd", URL: "https://wiki.archlinux.org/title/Systemd"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	m2 := updated.(*Model)
	if cmd != nil {
		t.Fatalf("did not expect download command before confirmation")
	}
	if m2.confirmDialog == nil {
		t.Fatalf("expected download confirmation dialog")
	}
	if m2.confirmDialog.action != confirmActionDownloadOffline {
		t.Fatalf("expected download confirmation action, got %v", m2.confirmDialog.action)
	}

	updated, cmd = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(*Model)
	if m3.confirmDialog != nil {
		t.Fatalf("expected dialog to close after confirmation")
	}
	if !m3.downloadLoading {
		t.Fatalf("expected downloadLoading true after confirmation")
	}
	if cmd == nil {
		t.Fatalf("expected download command after confirmation")
	}
}

func TestOfflineDownloadProgressUpdatesStatusAndContinues(t *testing.T) {
	m := NewModel(Config{})
	m.downloadLoading = true
	m.syncLoadingState()

	updated, cmd := m.Update(offlineDownloadProgressMsg{
		requested: 5,
		cached:    2,
		failed:    1,
		remaining: []string{"next"},
	})
	m2 := updated.(*Model)

	if !m2.downloadLoading || !m2.loading {
		t.Fatalf("expected download loading state to remain active during progress")
	}
	if !strings.Contains(m2.status, "3/5") || !strings.Contains(m2.status, "2 cached") {
		t.Fatalf("expected progress status with counts, got %q", m2.status)
	}
	if cmd == nil {
		t.Fatalf("expected next-step command while download queue remains")
	}
}

func TestOfflineDownloadProgressCompletionStopsLoading(t *testing.T) {
	m := NewModel(Config{})
	m.downloadLoading = true
	m.syncLoadingState()

	updated, cmd := m.Update(offlineDownloadProgressMsg{
		requested: 4,
		cached:    3,
		failed:    1,
		skipped:   2,
		remaining: nil,
	})
	m2 := updated.(*Model)

	if m2.downloadLoading || m2.loading {
		t.Fatalf("expected download loading state to stop on completion")
	}
	if !strings.Contains(m2.status, "Offline cache updated: 3/4 pages") {
		t.Fatalf("expected completion status, got %q", m2.status)
	}
	if cmd == nil {
		t.Fatalf("expected clear-status command from transient completion status")
	}
}

func TestEscInSearchCancelsPendingDebounce(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	m.mode = modeSearch
	m.searchInput.Focus()
	m.pendingSearchSeq = 7
	m.activeSearchReq = 3

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(*Model)

	if m2.mode != modeBrowse {
		t.Fatalf("expected browse mode after Esc, got %v", m2.mode)
	}
	if m2.pendingSearchSeq != 8 {
		t.Fatalf("expected pendingSearchSeq increment after Esc, got %d", m2.pendingSearchSeq)
	}

	reqAfterEsc := m2.activeSearchReq
	updated, cmd := m2.Update(debounceSearchMsg{seq: m2.pendingSearchSeq, query: "pacman"})
	m3 := updated.(*Model)

	if cmd != nil {
		t.Fatalf("expected debounce to be ignored outside search mode")
	}
	if m3.activeSearchReq != reqAfterEsc {
		t.Fatalf("expected no new search request, got %d -> %d", reqAfterEsc, m3.activeSearchReq)
	}
	if m3.searchLoading {
		t.Fatalf("expected searchLoading to remain false")
	}
}

func TestBackOpenFailureRestoresBackStackAndKeepsCurrentPage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	m.currentPage = &wiki.Page{Title: "Current", Markdown: "# Current"}
	m.currentMarkdown = "# Current"
	m.wikiMarkdown = "# Current"
	m.backStack = []string{"Previous"}
	m.mode = modeBrowse

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m2 := updated.(*Model)

	if cmd == nil {
		t.Fatalf("expected back navigation to schedule page open")
	}
	if m2.currentPage == nil || m2.currentPage.Title != "Current" {
		t.Fatalf("expected current page to remain during loading")
	}
	if len(m2.backStack) != 0 {
		t.Fatalf("expected back stack entry to be tentatively popped")
	}

	updated, _ = m2.Update(pageLoadedMsg{requestID: m2.activePageReq, err: errors.New("fetch failed")})
	m3 := updated.(*Model)

	if m3.currentPage == nil || m3.currentPage.Title != "Current" {
		t.Fatalf("expected current page to remain after failed back open")
	}
	if len(m3.backStack) != 1 || m3.backStack[0] != "Previous" {
		t.Fatalf("expected back stack to be restored, got %#v", m3.backStack)
	}
}

func TestResizeComponentsWithoutWindowSize(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	m.currentMarkdown = "# Pacman\n\nInstall packages with pacman"
	m.resizeComponents()

	if m.viewport.Width < 20 {
		t.Fatalf("expected viewport width to be initialized, got %d", m.viewport.Width)
	}
	if m.viewport.Height < 3 {
		t.Fatalf("expected viewport height to be initialized, got %d", m.viewport.Height)
	}
}

func TestSplitRelatedArticlesRemovesSectionFromMainContent(t *testing.T) {
	engine := render.NewEngine()

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
}

func TestAddHierarchicalHeadingNumbers(t *testing.T) {
	engine := render.NewEngine()

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

	out := engine.BuildDocument(input).WikiMarkdown

	for _, want := range []string{"# 1 Intro", "## 1.1 Install", "### 1.1.1 Pacman", "## 1.2 Usage"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected numbered heading %q in output:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "# do-not-touch") {
		t.Fatalf("expected fenced code content to remain unchanged")
	}
}

func TestAddHierarchicalHeadingNumbersSkipsDeepHeadings(t *testing.T) {
	engine := render.NewEngine()

	input := strings.Join([]string{
		"# Intro",
		"",
		"## Install",
		"",
		"### Command Flags",
		"",
		"#### Virtual packages",
	}, "\n")

	out := engine.BuildDocument(input).WikiMarkdown

	if !strings.Contains(out, "#### Virtual packages") {
		t.Fatalf("expected deep heading to stay unnumbered, got:\n%s", out)
	}
	if strings.Contains(out, "#### 1.1.1.1 Virtual packages") {
		t.Fatalf("expected deep heading numbering to be skipped, got:\n%s", out)
	}
}

func TestSetArticleContentBuildsRelatedAndLinksTabs(t *testing.T) {
	m := NewModel(Config{})

	md := strings.Join([]string{
		"Related articles",
		"- [Foo](https://wiki.archlinux.org/title/Foo)",
		"",
		"# Main",
		"Use [Pacman](https://wiki.archlinux.org/title/Pacman) here.",
	}, "\n")

	m.setArticleContent(md)

	if strings.Contains(strings.ToLower(m.wikiMarkdown), "related articles") {
		t.Fatalf("expected wiki tab markdown to exclude related section")
	}
	if len(m.relatedLinks) != 1 {
		t.Fatalf("expected 1 related link, got %d", len(m.relatedLinks))
	}
	if len(m.allLinks) != 1 {
		t.Fatalf("expected links tab to exclude related links and keep main-content links only, got %d", len(m.allLinks))
	}
	if !strings.Contains(m.relatedMarkdown, "1 Related Articles") {
		t.Fatalf("expected related tab markdown heading")
	}
	if !strings.Contains(m.linksMarkdown, "1 Links in This Article") {
		t.Fatalf("expected links tab markdown heading")
	}
}

func TestTabSwitchingChangesRenderedContent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	md := strings.Join([]string{
		"Related articles",
		"- [Foo](https://wiki.archlinux.org/title/Foo)",
		"",
		"# Main",
		"Use [Pacman](https://wiki.archlinux.org/title/Pacman) here.",
	}, "\n")

	m.currentPage = &wiki.Page{Title: "Test", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.renderPage()

	if strings.Contains(strings.ToLower(m.currentMarkdown), "related articles") {
		t.Fatalf("expected wiki tab to hide related section")
	}

	if !m.activateTab(tabRelated) {
		t.Fatalf("expected related tab activation to succeed")
	}
	if !strings.Contains(m.currentMarkdown, "Related Articles") {
		t.Fatalf("expected related tab markdown content")
	}

	if !m.activateTab(tabLinks) {
		t.Fatalf("expected links tab activation to succeed")
	}
	if !strings.Contains(m.currentMarkdown, "Links in This Article") {
		t.Fatalf("expected links tab markdown content")
	}
}

func TestSimplifyWikiMarkdownForDisplayStripsVisibleURLs(t *testing.T) {
	engine := render.NewEngine()

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

func TestRenderPageHidesURLsInWikiTabOnly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	md := strings.Join([]string{
		"# Main",
		"From [Wikipedia](https://en.wikipedia.org/wiki/Device_file):",
	}, "\n")

	m.currentPage = &wiki.Page{Title: "Test", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.renderPage()

	wikiView := osc8RE.ReplaceAllString(m.viewport.View(), "")
	if strings.Contains(wikiView, "https://en.wikipedia.org/wiki/Device_file") {
		t.Fatalf("expected wiki view to hide visible URL")
	}
	if !strings.Contains(m.viewport.View(), "\x1b]8;;https://en.wikipedia.org/wiki/Device_file\x1b\\") {
		t.Fatalf("expected wiki view to include terminal hyperlink target")
	}

	if !m.activateTab(tabLinks) {
		t.Fatalf("expected links tab activation to succeed")
	}
	if !strings.Contains(m.viewport.View(), "https://en.wikipedia.org/wiki/Device_file") {
		t.Fatalf("expected links tab view to retain URL")
	}
}

func TestBuildTabLinkItemsGroupsDedupeAndTitleCleanup(t *testing.T) {
	links := []articleLink{
		{Text: "DANE", URL: "https://wiki.archlinux.org/title/DANE"},
		{Text: "Duplicate DANE", URL: "https://wiki.archlinux.org/title/DANE"},
		{Text: "resolv.conf(5)", URL: "https://man.archlinux.org/man/resolv.conf.5.en"},
		{Text: "Domain name resolution#DNS_servers", URL: "https://wiki.archlinux.org/title/Domain_name_resolution#DNS_servers"},
		{Text: "https://dnscheck.tools/", URL: "https://dnscheck.tools/"},
	}

	items := buildTabLinkItems(links)
	if len(items) != 4 {
		t.Fatalf("expected 4 unique grouped links, got %d", len(items))
	}

	if items[0].Group != linkGroupArchWiki || items[0].Title != "DANE" {
		t.Fatalf("expected first item to be ArchWiki DANE, got %+v", items[0])
	}
	if items[1].Group != linkGroupManPages {
		t.Fatalf("expected second item to be Man Pages, got %+v", items[1])
	}
	if items[2].Group != linkGroupAnchors || items[2].Title != "Domain name resolution (DNS servers)" {
		t.Fatalf("expected cleaned anchor title and group, got %+v", items[2])
	}
	if items[3].Group != linkGroupExternal || items[3].Title != "dnscheck.tools" {
		t.Fatalf("expected external host-only title, got %+v", items[3])
	}
}

func TestLinksTabEnterOpensSelectedArchWikiLink(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	m.currentPage = &wiki.Page{Title: "DNSSEC", URL: "https://wiki.archlinux.org/title/DNSSEC", Markdown: "# DNSSEC"}
	m.activeTab = tabLinks
	m.allLinks = []articleLink{
		{Text: "DANE", URL: "https://wiki.archlinux.org/title/DANE"},
		{Text: "dnscheck.tools", URL: "https://dnscheck.tools/"},
	}
	m.linksTabItems = buildTabLinkItems(m.allLinks)
	m.linksCursor = 0
	m.renderPage()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*Model)

	if cmd == nil {
		t.Fatalf("expected open-page command when pressing Enter on selected ArchWiki link")
	}
	if !m2.pageLoading || !m2.loading {
		t.Fatalf("expected page loading state after Enter on selected ArchWiki link")
	}
	if !strings.Contains(m2.status, "Opening: DANE") {
		t.Fatalf("expected opening status for selected link, got %q", m2.status)
	}
}

func TestLinksTabLongListCollapseExpand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	m.currentPage = &wiki.Page{Title: "DNSSEC", URL: "https://wiki.archlinux.org/title/DNSSEC", Markdown: "# DNSSEC"}
	m.activeTab = tabLinks
	m.linksCursor = 0

	links := make([]articleLink, 0, 16)
	for i := 1; i <= 16; i++ {
		title := "Topic " + strconv.Itoa(i)
		links = append(links, articleLink{Text: title, URL: "https://wiki.archlinux.org/title/Topic_" + strconv.Itoa(i)})
	}
	m.allLinks = links
	m.linksTabItems = buildTabLinkItems(m.allLinks)
	m.renderPage()

	collapsed := m.viewport.View()
	if !strings.Contains(collapsed, "... more (") {
		t.Fatalf("expected collapsed list to show hidden-count hint, got:\n%s", collapsed)
	}
	if strings.Contains(collapsed, "Topic 12") {
		t.Fatalf("expected collapsed list to hide deeper rows, got:\n%s", collapsed)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m2 := updated.(*Model)
	expanded := m2.viewport.View()
	if strings.Contains(expanded, "... more (") {
		t.Fatalf("expected expanded list to remove hidden-count hint, got:\n%s", expanded)
	}
	if !strings.Contains(expanded, "Topic 12") {
		t.Fatalf("expected expanded list to include deeper rows, got:\n%s", expanded)
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m3 := updated.(*Model)
	recollapsed := m3.viewport.View()
	if !strings.Contains(recollapsed, "... more (") {
		t.Fatalf("expected left arrow to collapse group again, got:\n%s", recollapsed)
	}
}

func TestLinksTabPgDownJumpUsesVisibleWindow(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	m.currentPage = &wiki.Page{Title: "DNSSEC", URL: "https://wiki.archlinux.org/title/DNSSEC", Markdown: "# DNSSEC"}
	m.activeTab = tabLinks
	m.linksCursor = 0

	links := make([]articleLink, 0, 24)
	for i := 1; i <= 24; i++ {
		title := "Jump " + strconv.Itoa(i)
		links = append(links, articleLink{Text: title, URL: "https://wiki.archlinux.org/title/Jump_" + strconv.Itoa(i)})
	}
	m.allLinks = links
	m.linksTabItems = buildTabLinkItems(m.allLinks)
	m.renderPage()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m2 := updated.(*Model)

	if m2.linksCursor != tabLinkCollapsedVisible-1 {
		t.Fatalf("expected PgDown jump to land on last visible row in collapsed group (%d), got %d", tabLinkCollapsedVisible-1, m2.linksCursor)
	}
}

func TestCtrlDOpensOfflineLibraryPopup(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	if m.store == nil {
		t.Fatalf("expected store to be initialized")
	}
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	if err := m.store.WritePageCache("Pacman", "# Pacman"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}
	if err := m.store.WritePageCache("Systemd", "# Systemd"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m2 := updated.(*Model)
	if cmd != nil {
		t.Fatalf("did not expect command while opening offline library popup")
	}
	if !m2.showOfflineLibrary {
		t.Fatalf("expected offline library popup to open on Ctrl+D")
	}
	if len(m2.offlineTitles) < 2 {
		t.Fatalf("expected offline titles to be loaded, got %v", m2.offlineTitles)
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m3 := updated.(*Model)
	if m3.showOfflineLibrary {
		t.Fatalf("expected Esc to close offline library popup")
	}
}

func TestOfflineLibraryPopupSlashEnablesFilterAndFiltersResults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	if m.store == nil {
		t.Fatalf("expected store to be initialized")
	}
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	if err := m.store.WritePageCache("Pacman", "# Pacman"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}
	if err := m.store.WritePageCache("Systemd", "# Systemd"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m2 := updated.(*Model)
	if !m2.showOfflineLibrary {
		t.Fatalf("expected popup to open")
	}

	updated, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m3 := updated.(*Model)
	if !m3.offlineFilterActive {
		t.Fatalf("expected / to activate offline filter input")
	}

	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m4 := updated.(*Model)
	if len(m4.offlineVisibleTitles) != 1 || !strings.EqualFold(m4.offlineVisibleTitles[0], "Systemd") {
		t.Fatalf("expected filter to narrow list to Systemd, got %v", m4.offlineVisibleTitles)
	}

	updated, _ = m4.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m5 := updated.(*Model)
	if !m5.offlineFilterActive {
		t.Fatalf("expected first Esc to clear filter text but keep input active")
	}
	if strings.TrimSpace(m5.offlineFilterInput.Value()) != "" {
		t.Fatalf("expected filter text cleared on first Esc")
	}
}

func TestOfflineLibraryPopupEnterOpensCachedPage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	if m.store == nil {
		t.Fatalf("expected store to be initialized")
	}
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	if err := m.store.WritePageCache("Pacman", "# Pacman\ncontent"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m2 := updated.(*Model)
	if !m2.showOfflineLibrary {
		t.Fatalf("expected popup to open")
	}

	updated, cmd := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(*Model)
	if cmd == nil {
		t.Fatalf("expected open cached page command when pressing Enter in popup")
	}
	if m3.showOfflineLibrary {
		t.Fatalf("expected popup to close when opening cached page")
	}
	if !m3.pageLoading || !m3.loading {
		t.Fatalf("expected page loading state after opening cached page")
	}
}

func TestBrowseModeFullArchiveSyncRequiresConfirmation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	m2 := updated.(*Model)
	if cmd != nil {
		t.Fatalf("did not expect full archive command before confirmation")
	}
	if m2.confirmDialog == nil {
		t.Fatalf("expected full archive confirmation dialog")
	}
	if m2.confirmDialog.action != confirmActionDownloadFullArchive {
		t.Fatalf("expected full archive confirmation action, got %v", m2.confirmDialog.action)
	}

	updated, cmd = m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated.(*Model)
	if m3.confirmDialog != nil {
		t.Fatalf("expected dialog to close after full archive confirmation")
	}
	if !m3.archiveSyncLoading || !m3.loading {
		t.Fatalf("expected archive sync loading state after confirmation")
	}
	if cmd == nil {
		t.Fatalf("expected archive sync command after confirmation")
	}
}

func TestArchiveSyncProgressAndCompletionState(t *testing.T) {
	m := NewModel(Config{})
	m.archiveSyncLoading = true
	m.syncLoadingState()

	updated, cmd := m.Update(archiveSyncProgressMsg{
		phase:     "sync",
		processed: 10,
		total:     100,
		cached:    9,
		failed:    1,
		remaining: []string{"next"},
	})
	m2 := updated.(*Model)

	if !m2.archiveSyncLoading || !m2.loading {
		t.Fatalf("expected archive sync loading state to remain active during progress")
	}
	if !strings.Contains(m2.status, "10/100") || !strings.Contains(m2.status, "9 cached") {
		t.Fatalf("expected archive progress status with counts, got %q", m2.status)
	}
	if cmd == nil {
		t.Fatalf("expected next-step command while archive queue remains")
	}

	updated, cmd = m2.Update(archiveSyncDoneMsg{total: 100, cached: 95, failed: 5})
	m3 := updated.(*Model)
	if m3.archiveSyncLoading || m3.loading {
		t.Fatalf("expected archive sync loading state to stop on completion")
	}
	if !strings.Contains(m3.status, "Full archive sync complete: 95/100 pages") {
		t.Fatalf("expected archive completion status, got %q", m3.status)
	}
	if cmd == nil {
		t.Fatalf("expected clear-status command from archive completion")
	}
}

func TestOpenCachedPageFallsBackToArchiveStore(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	if m.store == nil {
		t.Fatalf("expected store to be initialized")
	}

	index, err := m.store.LoadArchiveIndex()
	if err != nil {
		t.Fatalf("LoadArchiveIndex failed: %v", err)
	}
	if err := m.store.WriteArchivePage("Pacman", "https://wiki.archlinux.org/title/Pacman", "# Pacman\narchive", &index); err != nil {
		t.Fatalf("WriteArchivePage failed: %v", err)
	}
	index.TotalTitles = 1
	if err := m.store.SaveArchiveIndex(index); err != nil {
		t.Fatalf("SaveArchiveIndex failed: %v", err)
	}

	cmd := m.openCachedPage("Pacman", false)
	if cmd == nil {
		t.Fatalf("expected open-cached-page command")
	}

	msg := cmd()
	loaded := pageLoadedMsg{}
	found := false

	switch value := msg.(type) {
	case pageLoadedMsg:
		loaded = value
		found = true

	case tea.BatchMsg:
		for _, sub := range value {
			if sub == nil {
				continue
			}

			if pl, ok := sub().(pageLoadedMsg); ok {
				loaded = pl
				found = true
				break
			}
		}
	}

	if !found {
		t.Fatalf("expected pageLoadedMsg, got %T", msg)
	}
	if loaded.err != nil {
		t.Fatalf("expected archive fallback to succeed, got %v", loaded.err)
	}
	if !loaded.fromCache {
		t.Fatalf("expected archive fallback to report fromCache")
	}
	if !strings.Contains(loaded.page.Markdown, "Pacman") {
		t.Fatalf("expected archived markdown content, got %q", loaded.page.Markdown)
	}
}

func TestOfflineTitlesFromStoreIncludesArchiveEntries(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	if m.store == nil {
		t.Fatalf("expected store to be initialized")
	}

	if err := m.store.WritePageCache("Systemd", "# Systemd"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}

	index, err := m.store.LoadArchiveIndex()
	if err != nil {
		t.Fatalf("LoadArchiveIndex failed: %v", err)
	}
	if err := m.store.WriteArchivePage("Pacman", "https://wiki.archlinux.org/title/Pacman", "# Pacman", &index); err != nil {
		t.Fatalf("WriteArchivePage failed: %v", err)
	}
	index.TotalTitles = 1
	if err := m.store.SaveArchiveIndex(index); err != nil {
		t.Fatalf("SaveArchiveIndex failed: %v", err)
	}

	titles := m.offlineTitlesFromStore()
	if len(titles) < 2 {
		t.Fatalf("expected combined offline title list to include cache and archive titles, got %v", titles)
	}

	hasSystemd := false
	hasPacman := false
	for _, title := range titles {
		if strings.EqualFold(title, "Systemd") {
			hasSystemd = true
		}
		if strings.EqualFold(title, "Pacman") {
			hasPacman = true
		}
	}
	if !hasSystemd || !hasPacman {
		t.Fatalf("expected Systemd and Pacman in combined offline titles, got %v", titles)
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
			title, ok := render.ArchWikiTitleFromURL(tc.raw)
			if ok != tc.ok {
				t.Fatalf("expected ok=%v got ok=%v title=%q", tc.ok, ok, title)
			}
			if title != tc.title {
				t.Fatalf("expected title=%q got %q", tc.title, title)
			}
		})
	}
}

func TestLinkURLAtDisplayColumn(t *testing.T) {
	url := "https://wiki.archlinux.org/title/Pacman"
	line := "Go to " + render.MakeTerminalHyperlink("Pacman", url) + " now"

	if got := linkURLAtDisplayColumn(line, 0); got != "" {
		t.Fatalf("expected no link at col 0, got %q", got)
	}

	if got := linkURLAtDisplayColumn(line, 6); got != url {
		t.Fatalf("expected link at label start, got %q", got)
	}

	if got := linkURLAtDisplayColumn(line, 11); got == "" {
		t.Fatalf("expected link inside label")
	}
}

func TestHandleMouseClickOpensArchWikiLinkInTUI(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = model.(*Model)

	md := strings.Join([]string{
		"# Main",
		"Use [Pacman](https://wiki.archlinux.org/title/Pacman) here.",
	}, "\n")

	m.currentPage = &wiki.Page{Title: "Start", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.mode = modeBrowse
	m.renderPage()

	view := m.View()
	lines := strings.Split(view, "\n")
	x, y := -1, -1
	for row, raw := range lines {
		plain := stripANSISequences(raw)
		idx := strings.Index(plain, "Pacman")
		if idx >= 0 {
			x = idx
			y = row
			break
		}
	}
	if x < 0 || y < 0 {
		t.Fatalf("failed to find Pacman label in rendered view")
	}

	cmd := m.handleMouseClick(tea.MouseMsg{
		X:      x,
		Y:      y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})

	if cmd == nil {
		t.Fatalf("expected open-page command from ArchWiki link click")
	}
	if !m.pageLoading {
		t.Fatalf("expected pageLoading after click-open")
	}
	if !strings.Contains(m.status, "Opening: Pacman") {
		t.Fatalf("expected opening status for Pacman, got %q", m.status)
	}
}
