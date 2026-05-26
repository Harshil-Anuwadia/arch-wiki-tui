package app

import (
	"strings"
	"testing"

	"archwiki-tui/internal/wiki"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestViewDoesNotOverflowWindowHeight(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 34})
	m = model.(*Model)

	m.currentPage = &wiki.Page{Title: "Pacman", Markdown: strings.Repeat("line with content\n", 300)}
	m.currentMarkdown = m.currentPage.Markdown
	m.renderPage()

	header := m.viewHeader()
	status := m.viewStatusBar()
	bodyTarget := max(6, m.height-lipgloss.Height(header)-lipgloss.Height(status))
	body := m.viewBrowse(bodyTarget)

	view := m.View()
	if h := lipgloss.Height(view); h > 34 {
		t.Fatalf(
			"view overflow: got=%d window=%d header=%d status=%d body=%d bodyTarget=%d viewport=%d contentLines=%d",
			h,
			34,
			lipgloss.Height(header),
			lipgloss.Height(status),
			lipgloss.Height(body),
			bodyTarget,
			m.viewport.Height,
			m.contentLines,
		)
	}
}

func TestViewShowsContentTabsWhenPageOpen(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 34})
	m = model.(*Model)

	md := strings.Join([]string{
		"Related articles",
		"- [Foo](https://example.com/foo)",
		"",
		"# Main",
		"Body",
	}, "\n")

	m.currentPage = &wiki.Page{Title: "Test", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.renderPage()

	view := m.View()
	if !strings.Contains(view, "Wiki") || !strings.Contains(view, "Related") || !strings.Contains(view, "Links") {
		t.Fatalf("expected tab strip in view")
	}
}

func TestHomeViewShowsSearchFirstLayout(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	m = model.(*Model)

	view := m.View()
	if !strings.Contains(view, "Terminal Wiki Reader") {
		t.Fatalf("expected subtitle in home view")
	}
	if !strings.Contains(view, "/ Search Arch Wiki") {
		t.Fatalf("expected primary search prompt in home view")
	}
	if !strings.Contains(view, "Suggested:") {
		t.Fatalf("expected single suggestion section in home view")
	}
	if !strings.Contains(view, "Recent:") {
		t.Fatalf("expected recent section in home view")
	}
	if strings.Contains(view, "Quick Keys") {
		t.Fatalf("did not expect quick-keys block in home view")
	}
}

func TestHomeViewUsesLoadedTopSearchTopic(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	m = model.(*Model)

	updated, _ := m.Update(topSearchLoadedMsg{topic: "systemd"})
	m2 := updated.(*Model)

	view := strings.ToLower(m2.View())
	if !strings.Contains(view, "suggested:") {
		t.Fatalf("expected suggested section in home view")
	}
	if !strings.Contains(view, "systemd") {
		t.Fatalf("expected loaded top search topic to be shown, got: %s", view)
	}
}

func TestHomeViewShowsRecentHistory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	m = model.(*Model)

	if m.store == nil {
		t.Skip("store unavailable in test environment")
	}

	m.homeTopSearch = "installation guide"
	if err := m.store.AddHistory("bootloader", "https://wiki.archlinux.org/title/Boot_loader"); err != nil {
		t.Fatalf("AddHistory failed: %v", err)
	}
	if err := m.store.AddHistory("pacman", "https://wiki.archlinux.org/title/Pacman"); err != nil {
		t.Fatalf("AddHistory failed: %v", err)
	}
	if err := m.store.AddHistory("systemd", "https://wiki.archlinux.org/title/Systemd"); err != nil {
		t.Fatalf("AddHistory failed: %v", err)
	}

	view := strings.ToLower(m.View())
	if !strings.Contains(view, "recent:") {
		t.Fatalf("expected recent section in home view")
	}
	for _, want := range []string{"systemd", "pacman", "bootloader"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected recent history title %q in home view", want)
		}
	}
}

func TestStatusBarShowsOnlyActionHints(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	m = model.(*Model)

	m.status = "UNIQUE_LEFT_STATUS_SHOULD_NOT_RENDER"
	bar := m.viewStatusBar()

	if strings.Contains(bar, m.status) {
		t.Fatalf("expected left status text to be hidden from status bar")
	}
	if !strings.Contains(bar, "/ search | ? help | q quit") {
		t.Fatalf("expected simplified home action hints to remain in status bar")
	}
	if strings.Contains(strings.ToLower(bar), "clear") {
		t.Fatalf("did not expect clear-cache action in home status bar")
	}
}

func TestStatusBarShowsLinkTabHints(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	m = model.(*Model)

	m.currentPage = &wiki.Page{Title: "DNSSEC", URL: "https://wiki.archlinux.org/title/DNSSEC", Markdown: "# DNSSEC"}
	m.activeTab = tabLinks

	bar := m.viewStatusBar()
	if !strings.Contains(bar, "Enter open") || !strings.Contains(bar, "c copy link") {
		t.Fatalf("expected link-tab action hints in status bar, got %q", bar)
	}
}

func TestSearchViewShowsUsefulEmptyState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	m = model.(*Model)

	m.mode = modeSearch
	m.searchInput.Focus()
	m.searchInput.SetValue("")
	m.searchResults = nil

	view := strings.ToLower(m.View())
	if !strings.Contains(view, "search arch wiki") {
		t.Fatalf("expected search panel title")
	}
	if !strings.Contains(view, "start typing to search") {
		t.Fatalf("expected explicit empty-query guidance")
	}
	if !strings.Contains(view, "example:") {
		t.Fatalf("expected example query under prompt")
	}
	if !strings.Contains(view, "popular:") {
		t.Fatalf("expected popular section in search empty state")
	}
	if !strings.Contains(view, "recent:") {
		t.Fatalf("expected recent section in search empty state")
	}
	if strings.Contains(view, "no results yet") {
		t.Fatalf("did not expect placeholder empty-state message")
	}
}

func TestSearchViewShowsQueryAndResultCount(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	m = model.(*Model)

	m.mode = modeSearch
	m.searchInput.Focus()
	m.searchInput.SetValue("how to")
	m.searchResults = []wiki.SearchResult{
		{Title: "How to install NVIDIA driver", Snippet: "indexed title match"},
		{Title: "How to use mouse in console", Snippet: "title prefix match"},
	}
	m.searchCursor = 0

	view := strings.ToLower(m.View())
	if !strings.Contains(view, "results for: \"how to\"") {
		t.Fatalf("expected explicit query feedback in search view")
	}
	if !strings.Contains(view, "results (2)") {
		t.Fatalf("expected result count in search view")
	}
}

func TestHomeViewShowsOnlyCacheBuildMessageWhenIndexRefreshing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	m = model.(*Model)

	m.indexRefreshing = true
	view := m.View()

	if !strings.Contains(view, "Building search cache") {
		t.Fatalf("expected cache-building message while index refresh is active")
	}
	if strings.Contains(view, "Suggested:") {
		t.Fatalf("did not expect home widgets while cache is building")
	}
	if strings.Contains(view, "Recent:") {
		t.Fatalf("did not expect recent list while cache is building")
	}
}

func TestReadableWrapWidthClampsForWideTerminal(t *testing.T) {
	if got := readableWrapWidth(180); got != maxReadableArticleWidth {
		t.Fatalf("expected wrap width clamp at %d, got %d", maxReadableArticleWidth, got)
	}

	if got := readableWrapWidth(64); got != 60 {
		t.Fatalf("expected viewport width 64 to map to wrap width 60, got %d", got)
	}

	if got := readableWrapWidth(20); got != 30 {
		t.Fatalf("expected minimum wrap width 30, got %d", got)
	}
}

func TestCenteredLeftPaddingUsesTrueCentering(t *testing.T) {
	if got := centeredLeftPadding(160, 60); got != 50 {
		t.Fatalf("expected true centered left padding of 50, got %d", got)
	}

	if got := centeredLeftPadding(100, 92); got != 4 {
		t.Fatalf("expected left padding of 4, got %d", got)
	}

	if got := centeredLeftPadding(80, 92); got != 0 {
		t.Fatalf("expected no padding when content width exceeds viewport, got %d", got)
	}
}

func TestViewShowsTOCPopupWhenEnabled(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	m := NewModel(Config{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	m = model.(*Model)

	md := strings.Join([]string{
		"# Intro",
		"Body",
		"",
		"## Installation",
		"Steps",
	}, "\n")

	m.currentPage = &wiki.Page{Title: "Test", Markdown: md}
	m.setArticleContent(md)
	m.activeTab = tabWiki
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()
	m.tocItems = buildTOCItems(m.wikiMarkdown)
	m.showTOC = true

	view := m.View()
	if !strings.Contains(view, "Table of Contents") {
		t.Fatalf("expected TOC popup title in view")
	}
	if !strings.Contains(strings.ToLower(view), "intro") {
		t.Fatalf("expected TOC popup to list visible heading titles")
	}
	if !strings.Contains(view, "▸") {
		t.Fatalf("expected TOC popup to show collapsed marker for headings with children")
	}
}
