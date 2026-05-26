package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"archwiki-tui/internal/wiki"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) View() string {
	if !m.ready {
		return "Loading archwiki-tui..."
	}

	if m.width < 60 || m.height < 16 {
		return "Terminal too small. Resize to at least 60x16."
	}

	header := m.viewHeader()
	status := m.viewStatusBar()
	bodyHeight := max(6, m.height-lipgloss.Height(header)-lipgloss.Height(status))

	body := ""
	if m.confirmDialog != nil {
		body = m.viewConfirmDialog(bodyHeight)
	} else if m.showHelp {
		body = m.viewHelp(bodyHeight)
	} else if m.showOfflineLibrary {
		body = m.viewOfflineLibraryPopup(bodyHeight)
	} else if m.showNav {
		body = m.viewNavPopup(bodyHeight)
	} else if m.showTOC {
		body = m.viewTOCPopup(bodyHeight)
	} else if m.mode == modeSearch {
		body = m.viewSearch(bodyHeight)
	} else {
		body = m.viewBrowse(bodyHeight)
	}

	full := lipgloss.JoinVertical(lipgloss.Left, header, body, status)
	return m.styles.App.Width(m.width).Height(m.height).Render(full)
}

func (m *Model) viewHeader() string {
	left := m.styles.HeaderTitle.Render("[W] archwiki-tui")
	if m.currentPage == nil {
		rightText := m.homeHeaderStatus()
		if progress := m.activeProgressStatus(); progress != "" {
			rightText = progress
		}
		right := m.styles.HeaderMeta.Render(rightText)
		line := distributeRow(left, "", right, max(20, m.width-2))
		return m.styles.Header.Width(max(20, m.width)).Render(line)
	}

	modeText := "online"
	if m.offline {
		modeText = "offline"
	}
	stateParts := make([]string, 0, 2)
	if m.loading {
		stateParts = append(stateParts, m.spinner.View()+" loading")
	}
	if m.indexRefreshing {
		stateParts = append(stateParts, m.spinner.View()+" indexing")
	}
	if len(stateParts) > 0 {
		modeText = strings.Join(stateParts, " ") + " | " + modeText
	}
	if progress := m.activeProgressStatus(); progress != "" {
		modeText = progress
	} else {
		modeText = truncate(m.currentPage.Title, 28) + " | " + modeText
	}
	right := m.styles.HeaderMeta.Render(modeText)

	line := distributeRow(left, "", right, max(20, m.width-2))
	return m.styles.Header.Width(max(20, m.width)).Render(line)
}

func (m *Model) viewStatusBar() string {
	center := m.statusCenterText()

	line := distributeRow(
		"",
		m.styles.StatusCenter.Render(truncate(center, max(10, m.width-6))),
		"",
		max(20, m.width-2),
	)
	return m.styles.Status.Width(max(20, m.width)).Render(line)
}

func (m *Model) activeProgressStatus() string {
	if m.archiveSyncLoading {
		if m.archiveSyncTotal > 0 {
			processed := min(m.archiveSyncProcessed, m.archiveSyncTotal)
			if processed < 0 {
				processed = 0
			}
			cached := max(0, m.archiveSyncCached)
			failed := max(0, m.archiveSyncFailed)
			eta := estimateETA(m.archiveSyncStartedAt, processed, m.archiveSyncTotal)
			if eta == "" {
				eta = "estimating"
			}
			return fmt.Sprintf("Full sync: %d/%d | %d cached | %d failed | ETA %s", processed, m.archiveSyncTotal, cached, failed, eta)
		}
		return "Full sync: preparing title list..."
	}

	if m.downloadLoading {
		if m.downloadTotal > 0 {
			processed := min(m.downloadProcessed, m.downloadTotal)
			if processed < 0 {
				processed = 0
			}
			eta := estimateETA(m.downloadStartedAt, processed, m.downloadTotal)
			if eta == "" {
				eta = "estimating"
			}

			progress := fmt.Sprintf("Bundle: %d/%d | %d cached | %d failed", processed, m.downloadTotal, max(0, m.downloadCached), max(0, m.downloadFailed))
			if m.downloadSkipped > 0 {
				progress += fmt.Sprintf(" | %d skipped", m.downloadSkipped)
			}
			progress += " | ETA " + eta
			return progress
		}

		if status := strings.TrimSpace(m.status); status != "" {
			return status
		}
		return "Downloading offline bundle..."
	}

	return ""
}

func (m *Model) statusCenterText() string {
	if m.confirmDialog != nil {
		return "Confirm action | Enter yes | Esc no"
	}

	if m.showHelp {
		return "Esc close help"
	}

	if m.showOfflineLibrary {
		return "↑↓ move | Enter open offline | / filter | Esc close"
	}

	if m.showNav {
		return "↑↓ move | Enter open | → expand | ← collapse | Esc close"
	}

	if m.showTOC {
		return "↑↓ move | Enter jump | → expand | ← collapse"
	}

	if progress := m.activeProgressStatus(); progress != "" {
		return "Please wait, this may take time | " + progress
	}

	if m.mode == modeSearch {
		return "↑↓/Ctrl+N Ctrl+P move | Enter open | Esc clear"
	}

	if m.currentPage == nil {
		return "/ search | ? help | q quit"
	}

	if m.activeTab == tabRelated || m.activeTab == tabLinks {
		return "↑↓ move | Enter open | l/→ expand | ← collapse | Ctrl+U/PgDn jump | c copy link"
	}

	return "o browser | c copy | D bundle | A full archive | x clear"
}

func (m *Model) tabName(tab contentTab) string {
	switch tab {
	case tabRelated:
		return "Related"
	case tabLinks:
		return "Links"
	default:
		return "Wiki"
	}
}

func (m *Model) viewContentTabs(width int) string {
	if m.currentPage == nil {
		return ""
	}
	m.ensureTabLinkItemsBuilt()

	relatedCount := len(m.relatedTabItems)
	if relatedCount == 0 {
		relatedCount = len(m.relatedLinks)
	}

	linksCount := len(m.linksTabItems)
	if linksCount == 0 {
		linksCount = len(m.allLinks)
	}

	items := []struct {
		key       string
		tab       contentTab
		label     string
		count     int
		withCount bool
	}{
		{key: "1", tab: tabWiki, label: "Wiki", count: 0, withCount: false},
		{key: "2", tab: tabRelated, label: "Related", count: relatedCount, withCount: true},
		{key: "3", tab: tabLinks, label: "Links", count: linksCount, withCount: true},
	}

	parts := make([]string, 0, len(items))
	for _, item := range items {
		label := item.label
		if item.withCount {
			label += fmt.Sprintf(" (%d)", item.count)
		}

		style := m.styles.Dim
		if item.tab == m.activeTab {
			style = m.styles.PanelTitle
		}
		parts = append(parts, style.Render(label))
	}

	line := strings.Join(parts, m.styles.Dim.Render(" | "))
	if lipgloss.Width(line) <= width {
		return line
	}

	compact := fmt.Sprintf("%s | %s | %s", m.tabName(tabWiki), m.tabName(tabRelated), m.tabName(tabLinks))
	return truncate(compact, width)
}

func (m *Model) viewBrowse(bodyHeight int) string {
	panelWidth := max(20, m.width)
	panelHeight := max(6, bodyHeight)
	panel := m.viewMainPanel(panelWidth, panelHeight)
	return lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Top, panel)
}

func (m *Model) viewTOCPopup(bodyHeight int) string {
	panelWidth := min(74, max(44, m.width-16))
	maxRows := max(4, min(16, bodyHeight-12))

	lines := []string{
		m.styles.PanelTitle.Render("Table of Contents"),
		m.styles.Dim.Render("Enter jump | → expand | ← collapse"),
		"",
	}

	visible := m.visibleTOCIndices()
	if len(visible) == 0 {
		lines = append(lines, m.styles.Dim.Render("No headings found in this page."))
	} else {
		selected := indexOfInt(visible, m.tocCursor)
		if selected < 0 {
			selected = 0
			m.tocCursor = visible[0]
		}

		start, end := clampWindow(selected, len(visible), maxRows)
		for i := start; i < end; i++ {
			itemIndex := visible[i]
			item := m.tocItems[itemIndex]
			prefix := "  "
			style := m.styles.Item
			if itemIndex == m.tocCursor {
				prefix = "> "
				style = m.styles.ItemSelected
			}

			indent := strings.Repeat("  ", max(0, item.Level-1))
			toggle := "  "
			if m.tocHasChildren(itemIndex) {
				if m.tocExpanded[itemIndex] {
					toggle = "▾ "
				} else {
					toggle = "▸ "
				}
			}

			titleWidth := max(10, panelWidth-14-lipgloss.Width(indent)-lipgloss.Width(toggle))
			line := prefix + indent + toggle + truncate(cleanTOCTitle(item.Title), titleWidth)
			lines = append(lines, style.Render(line))
		}
	}

	lines = append(lines, "", m.styles.Dim.Render("Esc/Ctrl+P close"))
	panel := m.styles.Modal.Width(panelWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, panel)
}

func (m *Model) viewNavPopup(bodyHeight int) string {
	panelWidth := min(84, max(50, m.width-12))
	maxRows := max(5, min(20, bodyHeight-12))

	lines := []string{
		m.styles.PanelTitle.Render("Navigation Map"),
		m.styles.Dim.Render("Enter open | → expand | ← collapse"),
		"",
	}

	visible := m.visibleNavIndices()
	if len(visible) == 0 {
		lines = append(lines, m.styles.Dim.Render("No pages in navigation history yet."))
	} else {
		selected := indexOfInt(visible, m.navCursor)
		if selected < 0 {
			selected = 0
			m.navCursor = visible[0]
		}

		start, end := clampWindow(selected, len(visible), maxRows)
		for i := start; i < end; i++ {
			idx := visible[i]
			node := m.navNodes[idx]

			prefix := "  "
			style := m.styles.Item
			if idx == m.navCursor {
				prefix = "> "
				style = m.styles.ItemSelected
			}

			indent := strings.Repeat("  ", max(0, m.navDepth(idx)))
			toggle := "  "
			if len(node.Children) > 0 {
				if m.navExpanded != nil && m.navExpanded[idx] {
					toggle = "▾ "
				} else {
					toggle = "▸ "
				}
			}

			current := "  "
			if idx == m.navCurrent {
				current = "• "
			}

			titleWidth := max(10, panelWidth-16-lipgloss.Width(indent)-lipgloss.Width(toggle)-lipgloss.Width(current))
			line := prefix + indent + toggle + current + truncate(node.Title, titleWidth)
			lines = append(lines, style.Render(line))
		}
	}

	lines = append(lines, "", m.styles.Dim.Render("Esc/Ctrl+Y close"))
	panel := m.styles.Modal.Width(panelWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, panel)
}

func (m *Model) viewOfflineLibraryPopup(bodyHeight int) string {
	panelWidth := min(90, max(54, m.width-10))
	maxRows := max(6, min(22, bodyHeight-14))
	m.offlineFilterInput.Width = max(20, panelWidth-8)

	total := len(m.offlineTitles)
	visible := m.offlineVisibleTitles

	lines := []string{
		m.styles.PanelTitle.Render(fmt.Sprintf("Offline Library (%d cached)", total)),
	}

	if m.offlineFilterActive || strings.TrimSpace(m.offlineFilterInput.Value()) != "" {
		lines = append(lines, m.offlineFilterInput.View())
	} else {
		lines = append(lines, m.styles.Dim.Render("Press / to filter downloaded pages"))
	}
	lines = append(lines, "")

	if total == 0 {
		lines = append(lines, m.styles.Dim.Render("No offline pages yet. Press D for bundle download or A for full archive sync."))
	} else if len(visible) == 0 {
		query := strings.TrimSpace(m.offlineFilterInput.Value())
		if query == "" {
			query = "(empty filter)"
		}
		lines = append(lines, m.styles.Dim.Render("No cached pages match \""+truncate(query, max(12, panelWidth-24))+"\"."))
	} else {
		if m.offlineCursor < 0 {
			m.offlineCursor = 0
		}
		if m.offlineCursor >= len(visible) {
			m.offlineCursor = len(visible) - 1
		}

		start, end := clampWindow(m.offlineCursor, len(visible), maxRows)
		for i := start; i < end; i++ {
			prefix := "  "
			style := m.styles.Item
			if i == m.offlineCursor {
				prefix = "> "
				style = m.styles.ItemSelected
			}

			titleWidth := max(14, panelWidth-10)
			lines = append(lines, style.Render(prefix+truncate(visible[i], titleWidth)))
		}

		lines = append(lines, "", m.styles.Dim.Render(fmt.Sprintf("Position: %d/%d visible (%d total)", m.offlineCursor+1, len(visible), total)))
	}

	lines = append(lines, "", m.styles.Dim.Render("Enter open offline | / filter | Esc/Ctrl+D close"))
	panel := m.styles.Modal.Width(panelWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, panel)
}

func (m *Model) viewSearch(bodyHeight int) string {
	panelStyle := m.styles.PanelFocused.BorderForeground(lipgloss.Color("214"))
	innerWidth := max(20, m.width-6)
	innerHeight := max(6, bodyHeight-4)
	input := m.searchInput.View()
	example := m.homePrimarySuggestion()
	query := strings.TrimSpace(m.searchInput.Value())

	lines := make([]string, 0, innerHeight)
	lines = append(lines, m.styles.PanelTitle.Render("Search Arch Wiki"))
	if query == "" {
		lines = append(lines, m.styles.Dim.Render("Start typing to search"))
	} else {
		resultsLabel := ""
		if m.searchLoading {
			resultsLabel = "Searching..."
		} else {
			resultsLabel = fmt.Sprintf("%d results", len(m.searchResults))
		}
		lines = append(lines, m.styles.Dim.Render("Results for: \""+truncate(query, max(8, innerWidth-26))+"\"  ("+resultsLabel+")"))
	}
	lines = append(lines, "")
	lines = append(lines, m.styles.PanelTitle.Render("Search:"))

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(0, 1).
		Width(max(12, innerWidth-4)).
		Render(input)
	lines = appendBlockLines(lines, inputBox)
	if query == "" {
		lines = append(lines, m.styles.Dim.Render("Example: "+truncate(example, max(10, innerWidth-14))))
	}
	lines = append(lines, "")

	if query == "" {
		lines = append(lines, m.styles.Dim.Render("Recent:"))
		for _, title := range m.homeRecentTitles(3) {
			lines = append(lines, "  "+truncate(title, max(10, innerWidth-6)))
		}
		lines = append(lines, "")
		lines = append(lines, m.styles.Dim.Render("Popular:"))
		for _, title := range m.homePopularTitles(3) {
			lines = append(lines, "  "+truncate(title, max(10, innerWidth-6)))
		}
	} else if m.searchLoading && len(m.searchResults) == 0 {
		lines = append(lines, m.spinner.View()+" Searching...")
	} else if len(m.searchResults) == 0 {
		lines = append(lines, m.styles.Dim.Render("No results for \""+truncate(query, max(8, innerWidth-22))+"\""))
		lines = append(lines, m.styles.Dim.Render("Try broader terms or fewer words."))
		lines = append(lines, "")
		lines = append(lines, m.styles.Dim.Render("Popular:"))
		for _, title := range m.homePopularTitles(3) {
			lines = append(lines, "  "+truncate(title, max(10, innerWidth-6)))
		}
	} else {
		maxRows := innerHeight - len(lines) - 2
		if maxRows < 1 {
			maxRows = 1
		}
		if maxRows > searchMaxVisibleResults {
			maxRows = searchMaxVisibleResults
		}

		start, end := clampWindow(m.searchCursor, len(m.searchResults), maxRows)
		lines = append(lines, m.styles.Dim.Render(fmt.Sprintf("Results (%d)", len(m.searchResults))))
		for i := start; i < end; i++ {
			entry := m.searchResults[i]
			prefix := "  "
			style := m.styles.Item
			if i == m.searchCursor {
				prefix = "> "
				style = m.styles.ItemSelected
			}
			title := truncate(strings.TrimSpace(entry.Title), max(10, innerWidth-6))
			title = highlightSearchQuery(title, query)
			lines = append(lines, style.Render(prefix+title))
			if i == m.searchCursor {
				meta := normalizeSearchResultMeta(entry, query)
				if meta != "" {
					meta = truncate(meta, max(10, innerWidth-9))
					meta = highlightSearchQuery(meta, query)
					lines = append(lines, m.styles.Dim.Render("   "+meta))
				}
			}
		}

		if len(m.searchResults) > maxRows {
			lines = append(lines, m.styles.Dim.Render("... more results (↑↓ scroll)"))
		}

		if m.searchLoading {
			lines = append(lines, m.styles.Dim.Render(m.spinner.View()+" Searching for more..."))
		}
	}

	content := strings.Join(padLines(lines, innerHeight), "\n")
	panel := panelStyle.Width(innerWidth).Height(innerHeight).Render(content)

	return lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Top, panel)
}

func highlightSearchQuery(text, query string) string {
	text = strings.TrimSpace(text)
	query = strings.TrimSpace(query)
	if text == "" || query == "" {
		return text
	}

	highlighted := highlightSearchNeedle(text, query)
	if highlighted != text {
		return highlighted
	}

	for _, token := range strings.Fields(query) {
		if len(token) < 2 {
			continue
		}
		highlighted = highlightSearchNeedle(text, token)
		if highlighted != text {
			return highlighted
		}
	}

	return text
}

func highlightSearchNeedle(text, needle string) string {
	needle = strings.TrimSpace(needle)
	if text == "" || needle == "" {
		return text
	}

	lowerText := strings.ToLower(text)
	lowerNeedle := strings.ToLower(needle)
	if !strings.Contains(lowerText, lowerNeedle) {
		return text
	}

	highlight := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	var b strings.Builder
	pos := 0
	for pos < len(text) {
		idx := strings.Index(lowerText[pos:], lowerNeedle)
		if idx < 0 {
			b.WriteString(text[pos:])
			break
		}
		idx += pos
		b.WriteString(text[pos:idx])

		end := idx + len(needle)
		if end > len(text) {
			end = len(text)
		}
		b.WriteString(highlight.Render(text[idx:end]))
		pos = end
	}

	return b.String()
}

func normalizeSearchResultMeta(entry wiki.SearchResult, query string) string {
	snippet := strings.TrimSpace(entry.Snippet)
	query = strings.TrimSpace(query)
	title := strings.TrimSpace(entry.Title)

	if query != "" && strings.EqualFold(title, query) {
		return "Exact title match"
	}

	switch strings.ToLower(snippet) {
	case "indexed title match":
		return "Indexed title match"
	case "title prefix match":
		return "Title prefix match"
	}

	return snippet
}

func (m *Model) homePopularTitles(limit int) []string {
	if limit <= 0 {
		limit = 3
	}

	titles := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit*2)

	addTitle := func(title string) {
		title = strings.TrimSpace(title)
		if title == "" {
			return
		}
		key := strings.ToLower(title)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		titles = append(titles, title)
	}

	addTitle(m.homePrimarySuggestion())
	day := time.Now().UTC().YearDay() - 1
	if day < 0 {
		day = 0
	}
	for i := 1; i < len(homeDailySearchTopics) && len(titles) < limit; i++ {
		addTitle(homeDailySearchTopics[(day+i)%len(homeDailySearchTopics)])
	}

	if len(titles) == 0 {
		titles = append(titles, "pacman")
	}
	if len(titles) > limit {
		return titles[:limit]
	}
	return titles
}

func (m *Model) viewHelp(bodyHeight int) string {
	text := []string{
		"archwiki-tui keybindings",
		"",
		"Global",
		"  q / Ctrl+Q / Ctrl+C  Quit",
		"  /              Search",
		"  ?              Toggle help",
		"  Enter/Esc      Confirm yes/no in dialogs",
		"  y / n          Quick yes/no in dialogs",
		"",
		"Browse",
		"  Up/Down j/k    Move / scroll",
		"  PgUp/PgDn      Page scroll",
		"  Ctrl+U         Half page scroll",
		"  Ctrl+B/F       Half page scroll",
		"  Ctrl+P         Table of contents popup",
		"  Ctrl+Y         Navigation map popup",
		"  Ctrl+D         Offline library popup (offline pages)",
		"  TOC: Enter jump, → expand, ← collapse",
		"  Offline popup: / filter, ↑↓ select, Enter open cached page",
		"  Related/Links: l/→ expand group, ← collapse, Ctrl+U/PgDn jump",
		"  g / G          Top / bottom",
		"  Esc / b / h    Back",
		"  o              Open page in browser (or selected link in Related/Links)",
		"  c              Copy next code block (or selected link URL in Related/Links)",
		"  D              Download current page + links for offline",
		"  A              Sync full compressed ArchWiki archive",
		"  x              Clear local page cache, archive, and index",
		"  Mouse click     Open ArchWiki links in TUI",
		"",
		"Search",
		"  Type           Debounced live search",
		"  Up/Down        Move results",
		"  Ctrl+N/P       Move results",
		"  Enter          Open result",
		"  Esc            Clear query (or close if empty)",
		"",
		"Press Esc, Enter or ? to close help.",
	}

	panel := m.styles.Modal.Width(max(20, m.width-8)).Height(max(8, bodyHeight-2)).Render(strings.Join(text, "\n"))
	return lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, panel)
}

func (m *Model) viewConfirmDialog(bodyHeight int) string {
	if m.confirmDialog == nil {
		return m.viewBrowse(bodyHeight)
	}

	dlg := m.confirmDialog

	yesStyle := m.styles.Dim
	noStyle := m.styles.Dim
	if dlg.selectedYes {
		yesStyle = m.styles.ItemSelected
		noStyle = m.styles.Item
	} else {
		yesStyle = m.styles.Item
		noStyle = m.styles.ItemSelected
	}

	lines := []string{
		m.styles.PanelTitle.Render(dlg.title),
		"",
		dlg.message,
	}
	for _, detail := range dlg.details {
		if strings.TrimSpace(detail) == "" {
			continue
		}
		lines = append(lines, m.styles.Dim.Render(detail))
	}

	lines = append(lines, "")
	lines = append(lines, yesStyle.Render("[ "+dlg.yesLabel+" ]")+"  "+noStyle.Render("[ "+dlg.noLabel+" ]"))
	lines = append(lines, m.styles.Dim.Render("Use Left/Right or Tab to choose. Enter confirms, Esc cancels."))

	panelWidth := max(40, min(86, m.width-8))
	panel := m.styles.Modal.Width(panelWidth).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, panel)
}

func (m *Model) viewMainPanel(width, height int) string {
	panelStyle := m.styles.PanelFocused

	innerWidth := max(16, width-2)
	innerHeight := max(5, height-2)
	title := "Arch Wiki"
	if m.currentPage != nil {
		title = m.currentPage.Title
	}

	content := ""
	switch {
	case m.indexRefreshing && m.currentPage == nil:
		content = m.spinner.View() + " Building search cache..."
	case m.loading && m.currentPage == nil:
		content = m.spinner.View() + " Loading..."
	case m.currentPage == nil:
		content = m.viewHomeScreen(innerWidth)
	default:
		content = m.viewport.View()
	}

	lines := []string{m.styles.PanelTitle.Render(truncate(title, innerWidth))}
	if m.currentPage != nil {
		lines = append(lines, "")
		tabs := m.viewContentTabs(innerWidth)
		if tabs != "" {
			lines = append(lines, tabs, "")
		}
	}
	if m.currentPage != nil {
		if m.archiveSyncLoading || m.downloadLoading {
			lines = append(lines, m.spinner.View()+" Please wait, this may take time...")
		} else if m.loading {
			lines = append(lines, m.spinner.View()+" loading")
		}
	}
	lines = appendBlockLines(lines, content)

	rendered := strings.Join(padLines(lines, innerHeight), "\n")
	return panelStyle.Width(innerWidth).Height(innerHeight).Render(rendered)
}

func (m *Model) viewHomeScreen(width int) string {
	banner := `   Arch Wiki`

	suggestion := m.homePrimarySuggestion()
	recent := m.homeRecentTitles(3)

	lines := []string{
		m.styles.HeaderTitle.Render(banner),
		"",
		m.styles.Dim.Render("The definitive terminal browser for the Arch Wiki"),
		"",
		m.styles.ItemSelected.Render("/ Search Arch Wiki"),
		"",
		m.styles.Dim.Render("Suggested:"),
		m.styles.ItemSelected.Render("  > " + suggestion),
	}

	if len(recent) > 0 {
		lines = append(lines, "", m.styles.Dim.Render("Recent:"))
		for _, title := range recent {
			lines = append(lines, "  "+title)
		}
	}

	if m.bootstrapQuery != "" {
		hint := "Session hint: " + m.bootstrapQuery
		lines = append(lines, "", m.styles.Dim.Render(truncate(hint, max(16, width-2))))
	}

	for i, line := range lines {
		lines[i] = truncate(line, max(16, width-2))
	}

	return strings.Join(lines, "\n")
}

func (m *Model) homeHeaderStatus() string {
	connectivity := "online"
	if m.cfg.ForceOffline || m.offline {
		connectivity = "offline"
	}

	cacheState := "cached"
	if m.store == nil {
		cacheState = "volatile"
	}

	readiness := "ready"
	if m.indexRefreshing || m.loading {
		readiness = "loading"
	}

	return connectivity + " | " + cacheState + " | " + readiness
}

func (m *Model) homePrimarySuggestion() string {
	topic := strings.TrimSpace(m.homeTopSearch)
	if topic != "" {
		return topic
	}
	return topSearchOfDay(time.Now().UTC())
}

func (m *Model) homeRecentTitles(limit int) []string {
	if limit <= 0 {
		limit = 3
	}

	titles := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit*2)
	suggestion := strings.ToLower(strings.TrimSpace(m.homePrimarySuggestion()))

	addTitle := func(title string) {
		title = strings.TrimSpace(title)
		if title == "" {
			return
		}

		key := strings.ToLower(title)
		if key == suggestion {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}

		seen[key] = struct{}{}
		titles = append(titles, title)
	}

	if m.store != nil {
		for _, entry := range m.store.ListHistory(limit * 3) {
			addTitle(entry.Title)
			if len(titles) >= limit {
				break
			}
		}
	}

	if len(titles) < limit {
		day := time.Now().UTC().YearDay() - 1
		if day < 0 {
			day = 0
		}

		for i := 0; i < len(homeDailySearchTopics) && len(titles) < limit; i++ {
			addTitle(homeDailySearchTopics[(day+i)%len(homeDailySearchTopics)])
		}
	}

	if len(titles) == 0 {
		titles = append(titles, "pacman")
	}

	if len(titles) > limit {
		return titles[:limit]
	}
	return titles
}

func topSearchOfDay(now time.Time) string {
	if len(homeDailySearchTopics) == 0 {
		return "pacman"
	}

	day := now.YearDay() - 1
	if day < 0 {
		day = 0
	}

	idx := day % len(homeDailySearchTopics)
	return homeDailySearchTopics[idx]
}

func (m *Model) fetchTopSearchCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		titles, err := m.api.FetchPopularTitles(ctx, 40)
		if err != nil || len(titles) == 0 {
			return topSearchLoadedMsg{err: err}
		}

		day := time.Now().UTC().YearDay() - 1
		if day < 0 {
			day = 0
		}
		topic := strings.TrimSpace(titles[day%len(titles)])
		if topic == "" {
			return topSearchLoadedMsg{err: fmt.Errorf("popular title list is empty")}
		}

		return topSearchLoadedMsg{topic: topic}
	}
}

func (m *Model) resetTitleIndexFromDisk() {
	path := resolveIndexCachePath(m.store)
	idx, err := wiki.NewTitleIndex(path)
	if err != nil {
		m.titleIndex = nil
		m.indexRefreshing = false
		return
	}

	m.titleIndex = idx
	m.indexRefreshing = false
}

func (m *Model) clearCacheCmd() tea.Cmd {
	if m.store == nil {
		return func() tea.Msg {
			return cacheClearedMsg{err: fmt.Errorf("cache storage unavailable")}
		}
	}

	return func() tea.Msg {
		removedPages, indexRemoved, err := m.store.ClearCache()
		return cacheClearedMsg{removedPages: removedPages, indexRemoved: indexRemoved, err: err}
	}
}

func (m *Model) updateConfirmDialog(msg tea.KeyMsg) tea.Cmd {
	if m.confirmDialog == nil {
		return nil
	}

	key := strings.ToLower(msg.String())
	switch key {
	case "left", "h":
		m.confirmDialog.selectedYes = true
		return nil

	case "right", "l":
		m.confirmDialog.selectedYes = false
		return nil

	case "tab", "shift+tab":
		m.confirmDialog.selectedYes = !m.confirmDialog.selectedYes
		return nil

	case "y":
		m.confirmDialog.selectedYes = true
		return m.acceptConfirmDialog()

	case "n", "q", "esc":
		m.confirmDialog = nil
		return m.setTransientStatus("Canceled")

	case "enter", "ctrl+m", "ctrl+j":
		if !m.confirmDialog.selectedYes {
			m.confirmDialog = nil
			return m.setTransientStatus("Canceled")
		}
		return m.acceptConfirmDialog()
	}

	return nil
}

func (m *Model) updateTOCMode(msg tea.KeyMsg) tea.Cmd {
	if !m.showTOC {
		return nil
	}

	visible := m.visibleTOCIndices()
	if len(visible) > 0 {
		m.ensureTOCCursorVisible(visible)
		visible = m.visibleTOCIndices()
	}

	selectByDelta := func(delta int) {
		if len(visible) == 0 {
			return
		}
		pos := indexOfInt(visible, m.tocCursor)
		if pos < 0 {
			pos = 0
		}
		pos += delta
		if pos < 0 {
			pos = 0
		}
		if pos >= len(visible) {
			pos = len(visible) - 1
		}
		m.tocCursor = visible[pos]
	}

	key := msg.String()
	switch key {
	case "esc", "ctrl+p", "q":
		m.showTOC = false
		return nil

	case "up", "k":
		selectByDelta(-1)
		return nil

	case "down", "j":
		selectByDelta(1)
		return nil

	case "pgup":
		selectByDelta(-8)
		return nil

	case "pgdown":
		selectByDelta(8)
		return nil

	case "g", "home":
		if len(visible) > 0 {
			m.tocCursor = visible[0]
		}
		return nil

	case "G", "end":
		if len(visible) > 0 {
			m.tocCursor = visible[len(visible)-1]
		}
		return nil

	case "right":
		m.setTOCCursorExpanded(true)
		return nil

	case "left":
		m.collapseTOCCursorOrMoveParent()
		return nil

	case "enter":
		m.showTOC = false
		return m.jumpToTOCItem(m.tocCursor)
	}

	return nil
}

func (m *Model) setTOCCursorExpanded(expanded bool) {
	if !m.tocHasChildren(m.tocCursor) {
		return
	}

	if m.tocExpanded == nil {
		m.tocExpanded = make(map[int]bool)
	}

	m.tocExpanded[m.tocCursor] = expanded
	m.ensureTOCCursorVisible(m.visibleTOCIndices())
}

func (m *Model) collapseTOCCursorOrMoveParent() {
	if m.tocHasChildren(m.tocCursor) && m.tocExpanded != nil && m.tocExpanded[m.tocCursor] {
		m.tocExpanded[m.tocCursor] = false
		m.ensureTOCCursorVisible(m.visibleTOCIndices())
		return
	}

	if parent := m.tocParentIndex(m.tocCursor); parent >= 0 {
		m.tocCursor = parent
		m.ensureTOCCursorVisible(m.visibleTOCIndices())
	}
}

func (m *Model) toggleTOCPopup() tea.Cmd {
	if m.currentPage == nil {
		return m.setTransientStatus("Open a page first to view table of contents")
	}
	if m.activeTab != tabWiki {
		return m.setTransientStatus("Switch to Wiki tab to open table of contents")
	}

	if m.showTOC {
		m.showTOC = false
		return nil
	}
	m.showNav = false
	m.showOfflineLibrary = false

	if len(m.tocItems) == 0 {
		m.tocItems = buildTOCItems(m.wikiMarkdown)
		m.invalidateTOCOffsetCache()
	}
	if len(m.tocItems) == 0 {
		return m.setTransientStatus("No headings found in this article")
	}

	m.tocExpanded = make(map[int]bool, len(m.tocItems))
	m.tocCursor = m.nearestTOCIndexForViewport()
	for _, idx := range m.tocPathToRoot(m.tocCursor) {
		m.tocExpanded[idx] = true
	}
	m.ensureTOCCursorVisible(m.visibleTOCIndices())
	m.showTOC = true
	return nil
}

func (m *Model) updateNavMode(msg tea.KeyMsg) tea.Cmd {
	if !m.showNav {
		return nil
	}

	visible := m.visibleNavIndices()
	if len(visible) > 0 {
		m.ensureNavCursorVisible(visible)
		visible = m.visibleNavIndices()
	}

	selectByDelta := func(delta int) {
		if len(visible) == 0 {
			return
		}
		pos := indexOfInt(visible, m.navCursor)
		if pos < 0 {
			pos = 0
		}
		pos += delta
		if pos < 0 {
			pos = 0
		}
		if pos >= len(visible) {
			pos = len(visible) - 1
		}
		m.navCursor = visible[pos]
	}

	switch msg.String() {
	case "esc", "ctrl+y", "q":
		m.showNav = false
		return nil

	case "up", "k":
		selectByDelta(-1)
		return nil

	case "down", "j":
		selectByDelta(1)
		return nil

	case "pgup":
		selectByDelta(-8)
		return nil

	case "pgdown":
		selectByDelta(8)
		return nil

	case "g", "home":
		if len(visible) > 0 {
			m.navCursor = visible[0]
		}
		return nil

	case "G", "end":
		if len(visible) > 0 {
			m.navCursor = visible[len(visible)-1]
		}
		return nil

	case "right":
		m.setNavCursorExpanded(true)
		return nil

	case "left":
		m.collapseNavCursorOrMoveParent()
		return nil

	case "enter":
		m.showNav = false
		return m.jumpToNavNode(m.navCursor)
	}

	return nil
}

func (m *Model) setNavCursorExpanded(expanded bool) {
	if m.navCursor < 0 || m.navCursor >= len(m.navNodes) {
		return
	}
	if len(m.navNodes[m.navCursor].Children) == 0 {
		return
	}

	if m.navExpanded == nil {
		m.navExpanded = make(map[int]bool)
	}

	m.navExpanded[m.navCursor] = expanded
	m.ensureNavCursorVisible(m.visibleNavIndices())
}
