package app

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"archwiki-tui/internal/render"
	"archwiki-tui/internal/storage"
	"archwiki-tui/internal/wiki"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/browser"
)

func (m *Model) collapseNavCursorOrMoveParent() {
	if m.navCursor < 0 || m.navCursor >= len(m.navNodes) {
		return
	}

	if len(m.navNodes[m.navCursor].Children) > 0 && m.navExpanded != nil && m.navExpanded[m.navCursor] {
		m.navExpanded[m.navCursor] = false
		m.ensureNavCursorVisible(m.visibleNavIndices())
		return
	}

	if parent := m.navNodes[m.navCursor].Parent; parent >= 0 {
		m.navCursor = parent
		m.ensureNavCursorVisible(m.visibleNavIndices())
	}
}

func (m *Model) toggleNavPopup() tea.Cmd {
	if m.currentPage == nil {
		return m.setTransientStatus("Open a page first to view navigation map")
	}

	if m.showNav {
		m.showNav = false
		return nil
	}

	m.ensureNavigationPath()
	if len(m.navNodes) == 0 || len(m.navRoots) == 0 {
		return m.setTransientStatus("Navigation map is empty")
	}

	if m.navExpanded == nil {
		m.navExpanded = make(map[int]bool, len(m.navNodes))
	}
	for _, idx := range m.navPathToRoot(m.navCurrent) {
		m.navExpanded[idx] = true
	}

	m.navCursor = m.navCurrent
	m.ensureNavCursorVisible(m.visibleNavIndices())
	m.showTOC = false
	m.showOfflineLibrary = false
	m.showNav = true
	return nil
}

func (m *Model) toggleOfflineLibraryPopup() tea.Cmd {
	if m.store == nil {
		return m.setTransientStatus("Cache storage unavailable")
	}

	if m.showOfflineLibrary {
		m.showOfflineLibrary = false
		m.offlineFilterActive = false
		m.offlineFilterInput.Blur()
		return nil
	}

	m.showHelp = false
	m.showTOC = false
	m.showNav = false
	m.showOfflineLibrary = true
	m.offlineFilterActive = false
	m.offlineFilterInput.SetValue("")
	m.offlineFilterInput.Blur()
	m.offlineRefreshTitles()

	if len(m.offlineTitles) == 0 {
		return m.setTransientStatus("No offline pages found. Press D for bundle or A for full archive sync.")
	}

	return nil
}

func (m *Model) offlineRefreshTitles() {
	titles := m.offlineTitlesFromStore()
	m.offlineTitles = titles
	m.applyOfflineFilter()
}

func (m *Model) offlineTitlesFromStore() []string {
	if m.store == nil {
		return nil
	}

	cached := m.store.ListCachedTitles()
	archived := m.store.ListArchiveTitles()
	if len(cached) > 0 || len(archived) > 0 {
		seen := make(map[string]struct{}, len(cached)+len(archived))
		merged := make([]string, 0, len(cached)+len(archived))

		for _, title := range cached {
			title = strings.TrimSpace(title)
			if title == "" {
				continue
			}

			key := strings.ToLower(title)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, title)
		}

		for _, title := range archived {
			title = strings.TrimSpace(title)
			if title == "" {
				continue
			}

			key := strings.ToLower(title)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, title)
		}

		return merged
	}

	// Backward-compatible fallback for existing caches written before cached_titles existed.
	history := m.store.ListHistory(200)
	if len(history) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(history))
	out := make([]string, 0, len(history))
	for _, entry := range history {
		title := strings.TrimSpace(entry.Title)
		if title == "" {
			continue
		}

		key := strings.ToLower(title)
		if _, ok := seen[key]; ok {
			continue
		}
		if _, err := m.store.ReadPageCache(title); err != nil {
			continue
		}

		seen[key] = struct{}{}
		out = append(out, title)
	}

	return out
}

func (m *Model) applyOfflineFilter() {
	query := strings.ToLower(strings.TrimSpace(m.offlineFilterInput.Value()))
	if query == "" {
		m.offlineVisibleTitles = append([]string(nil), m.offlineTitles...)
	} else {
		filtered := make([]string, 0, len(m.offlineTitles))
		for _, title := range m.offlineTitles {
			if strings.Contains(strings.ToLower(title), query) {
				filtered = append(filtered, title)
			}
		}
		m.offlineVisibleTitles = filtered
	}

	if len(m.offlineVisibleTitles) == 0 {
		m.offlineCursor = 0
		return
	}
	if m.offlineCursor < 0 {
		m.offlineCursor = 0
	}
	if m.offlineCursor >= len(m.offlineVisibleTitles) {
		m.offlineCursor = len(m.offlineVisibleTitles) - 1
	}
}

func (m *Model) openSelectedOfflinePage() tea.Cmd {
	if len(m.offlineVisibleTitles) == 0 {
		return m.setTransientStatus("No downloaded pages found")
	}

	if m.offlineCursor < 0 {
		m.offlineCursor = 0
	}
	if m.offlineCursor >= len(m.offlineVisibleTitles) {
		m.offlineCursor = len(m.offlineVisibleTitles) - 1
	}

	return m.openCachedPage(m.offlineVisibleTitles[m.offlineCursor], true)
}

func (m *Model) openCachedPage(title string, pushCurrent bool) tea.Cmd {
	title = strings.TrimSpace(title)
	if title == "" {
		return m.setTransientStatus("Invalid offline page selection")
	}
	if m.store == nil {
		return m.setTransientStatus("Offline cache unavailable")
	}

	m.mode = modeBrowse
	m.showOfflineLibrary = false
	m.offlineFilterActive = false
	m.offlineFilterInput.Blur()
	m.showNav = false
	m.showTOC = false
	m.tocItems = nil
	m.tocCursor = 0
	m.tocExpanded = nil
	m.searchInput.Blur()
	m.status = "Opening offline: " + title
	m.statusSticky = true
	m.searchLoading = false
	m.activeSearchReq++
	if pushCurrent {
		m.pendingBackTitle = ""
	}

	if pushCurrent && m.currentPage != nil && !strings.EqualFold(m.currentPage.Title, title) {
		m.backStack = append(m.backStack, m.currentPage.Title)
		if len(m.backStack) > 100 {
			m.backStack = m.backStack[len(m.backStack)-100:]
		}
	}

	m.pageLoading = true
	m.syncLoadingState()
	m.activePageReq++
	requestID := m.activePageReq

	pageCmd := func() tea.Msg {
		cached, err := m.store.ReadPageCache(title)
		if err != nil {
			archived, archiveErr := m.store.ReadArchivePage(title)
			if archiveErr != nil {
				return pageLoadedMsg{requestID: requestID, err: fmt.Errorf("offline cache miss for %q", title)}
			}
			cached = archived
		}

		return pageLoadedMsg{
			requestID: requestID,
			fromCache: true,
			page: wiki.Page{
				Title:    title,
				URL:      wiki.ArticleURL(title),
				Markdown: cached,
			},
		}
	}

	return tea.Batch(pageCmd, m.spinner.Tick)
}

func (m *Model) updateOfflineLibraryMode(msg tea.KeyMsg) tea.Cmd {
	if !m.showOfflineLibrary {
		return nil
	}

	if m.offlineFilterActive {
		switch msg.String() {
		case "esc":
			if strings.TrimSpace(m.offlineFilterInput.Value()) != "" {
				m.offlineFilterInput.SetValue("")
				m.applyOfflineFilter()
				return nil
			}
			m.offlineFilterActive = false
			m.offlineFilterInput.Blur()
			return nil

		case "enter", "ctrl+m", "ctrl+j":
			m.offlineFilterActive = false
			m.offlineFilterInput.Blur()
			return nil
		}
	}

	switch msg.String() {
	case "esc", "ctrl+d", "q":
		m.showOfflineLibrary = false
		m.offlineFilterActive = false
		m.offlineFilterInput.Blur()
		return nil

	case "/":
		m.offlineFilterActive = true
		m.offlineFilterInput.Focus()
		m.offlineFilterInput.CursorEnd()
		return nil

	case "up", "k", "ctrl+p":
		if m.offlineCursor > 0 {
			m.offlineCursor--
		}
		return nil

	case "down", "j", "ctrl+n":
		if m.offlineCursor < len(m.offlineVisibleTitles)-1 {
			m.offlineCursor++
		}
		return nil

	case "pgup":
		m.offlineCursor = max(0, m.offlineCursor-8)
		return nil

	case "pgdown", "space":
		m.offlineCursor = min(len(m.offlineVisibleTitles)-1, m.offlineCursor+8)
		if m.offlineCursor < 0 {
			m.offlineCursor = 0
		}
		return nil

	case "g", "home":
		if len(m.offlineVisibleTitles) > 0 {
			m.offlineCursor = 0
		}
		return nil

	case "G", "end":
		if len(m.offlineVisibleTitles) > 0 {
			m.offlineCursor = len(m.offlineVisibleTitles) - 1
		}
		return nil

	case "enter", "ctrl+m", "ctrl+j":
		return m.openSelectedOfflinePage()
	}

	if m.offlineFilterActive {
		before := m.offlineFilterInput.Value()
		var cmd tea.Cmd
		m.offlineFilterInput, cmd = m.offlineFilterInput.Update(msg)
		if m.offlineFilterInput.Value() != before {
			m.applyOfflineFilter()
		}
		return cmd
	}

	return nil
}

func (m *Model) jumpToNavNode(index int) tea.Cmd {
	if index < 0 || index >= len(m.navNodes) {
		return m.setTransientStatus("Invalid navigation selection")
	}

	target := strings.TrimSpace(m.navNodes[index].Title)
	if target == "" {
		return m.setTransientStatus("Invalid navigation selection")
	}

	path := m.navPathToRoot(index)
	if len(path) == 0 {
		return m.setTransientStatus("Invalid navigation selection")
	}

	if m.currentPage != nil && strings.EqualFold(strings.TrimSpace(m.currentPage.Title), target) {
		newBack := make([]string, 0, max(0, len(path)-1))
		for i := 0; i < len(path)-1; i++ {
			newBack = append(newBack, m.navNodes[path[i]].Title)
		}
		m.backStack = newBack
		m.navCurrent = index
		return m.setTransientStatus("Already on " + target)
	}

	m.navJumpRestoreBackStack = append([]string(nil), m.backStack...)
	m.navJumpRestorePending = m.pendingBackTitle
	m.navJumpRestoreActive = true

	newBack := make([]string, 0, max(0, len(path)-1))
	for i := 0; i < len(path)-1; i++ {
		newBack = append(newBack, m.navNodes[path[i]].Title)
	}
	m.backStack = newBack
	m.pendingBackTitle = ""
	m.navCurrent = index

	return m.openPage(target, false)
}

func (m *Model) acceptConfirmDialog() tea.Cmd {
	if m.confirmDialog == nil {
		return nil
	}

	dlg := *m.confirmDialog
	titles := append([]string(nil), dlg.downloadTitles...)
	skipped := dlg.downloadSkipped
	m.confirmDialog = nil

	switch dlg.action {
	case confirmActionClearCache:
		return m.clearCacheCmd()

	case confirmActionDownloadOffline:
		if len(titles) == 0 {
			return m.setTransientStatus("No ArchWiki pages available for offline download")
		}

		m.downloadStartedAt = time.Now()
		m.downloadTotal = len(titles)
		m.downloadProcessed = 0
		m.downloadCached = 0
		m.downloadFailed = 0
		m.downloadSkipped = max(0, skipped)
		m.downloadLoading = true
		m.syncLoadingState()
		m.status = fmt.Sprintf("Downloading offline bundle (%d pages)...", len(titles))
		m.statusSticky = true
		return tea.Batch(m.downloadOfflinePageStepCmd(titles, len(titles), 0, 0, skipped), m.spinner.Tick)

	case confirmActionDownloadFullArchive:
		if m.cfg.ForceOffline {
			return m.setTransientStatus("Disable --offline to sync full archive")
		}
		if m.store == nil {
			return m.setTransientStatus("Cache storage unavailable")
		}

		m.archiveSyncLoading = true
		m.archiveSyncStartedAt = time.Now()
		m.archiveSyncTotal = 0
		m.archiveSyncProcessed = 0
		m.archiveSyncCached = 0
		m.archiveSyncFailed = 0
		m.syncLoadingState()
		m.status = "Preparing full archive sync..."
		m.statusSticky = true

		return tea.Batch(m.prepareArchiveSyncCmd(), m.spinner.Tick)
	}

	return nil
}

func (m *Model) offlineDownloadTitles(limit int) ([]string, int) {
	if m.currentPage == nil {
		return nil, 0
	}
	if limit <= 0 {
		limit = 1
	}

	seen := make(map[string]struct{}, limit)
	titles := make([]string, 0, limit)

	addTitle := func(title string) bool {
		title = strings.TrimSpace(title)
		if title == "" {
			return false
		}

		key := strings.ToLower(title)
		if _, ok := seen[key]; ok {
			return false
		}
		if len(titles) >= limit {
			return false
		}

		seen[key] = struct{}{}
		titles = append(titles, title)
		return true
	}

	_ = addTitle(m.currentPage.Title)

	skipped := 0
	for _, link := range m.allLinks {
		title, ok := render.ArchWikiTitleFromURL(link.URL)
		if !ok {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(title))
		if _, exists := seen[key]; exists {
			continue
		}

		if len(titles) >= limit {
			skipped++
			continue
		}
		_ = addTitle(title)
	}

	return titles, skipped
}

func (m *Model) downloadOfflinePageStepCmd(queue []string, requested, cached, failed, skipped int) tea.Cmd {
	remaining := append([]string(nil), queue...)

	return func() tea.Msg {
		if m.store == nil {
			return offlineDownloadedMsg{requested: requested, cached: cached, failed: failed, skipped: skipped, err: fmt.Errorf("cache storage unavailable")}
		}

		if len(remaining) == 0 {
			return offlineDownloadProgressMsg{
				requested: requested,
				cached:    cached,
				failed:    failed,
				skipped:   skipped,
				remaining: nil,
			}
		}

		batchSize := min(len(remaining), offlineDownloadBatchSize)
		batch := append([]string(nil), remaining[:batchSize]...)
		next := append([]string(nil), remaining[batchSize:]...)

		results := fetchPagesConcurrently(m.api, batch, 20*time.Second, offlineDownloadWorkers)
		cacheEntries := make([]storage.PageCacheEntry, 0, len(results))
		for _, result := range results {
			if result.err != nil {
				failed++
				continue
			}

			cacheEntries = append(cacheEntries, storage.PageCacheEntry{
				Title:    result.page.Title,
				Markdown: result.page.Markdown,
			})
		}

		written, writeFailed, err := m.store.WritePageCacheBatch(cacheEntries)
		if err != nil {
			return offlineDownloadedMsg{requested: requested, cached: cached, failed: failed, skipped: skipped, err: err}
		}

		cached += written
		failed += writeFailed

		return offlineDownloadProgressMsg{
			requested: requested,
			cached:    cached,
			failed:    failed,
			skipped:   skipped,
			remaining: next,
		}
	}
}

func (m *Model) prepareArchiveSyncCmd() tea.Cmd {
	return func() tea.Msg {
		if m.store == nil {
			return archiveSyncDoneMsg{err: fmt.Errorf("cache storage unavailable")}
		}
		if m.cfg.ForceOffline {
			return archiveSyncDoneMsg{err: fmt.Errorf("disable --offline to sync full archive")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		titles, err := m.api.FetchAllTitles(ctx, 0)
		cancel()
		if err != nil {
			return archiveSyncDoneMsg{err: err}
		}

		queue, cached, index, err := m.store.ArchiveSyncPlan(titles)
		if err != nil {
			return archiveSyncDoneMsg{err: err}
		}

		total := len(queue) + cached
		index.TotalTitles = total
		index.UpdatedAt = time.Now().UTC()
		if err := m.store.SaveArchiveIndex(index); err != nil {
			return archiveSyncDoneMsg{err: err}
		}

		return archiveSyncProgressMsg{
			phase:     "prepare",
			processed: cached,
			total:     total,
			cached:    cached,
			failed:    0,
			remaining: queue,
			index:     index,
		}
	}
}

func (m *Model) archiveSyncStepCmd(queue []string, total, cached, failed int, index storage.ArchiveIndex) tea.Cmd {
	remaining := append([]string(nil), queue...)

	return func() tea.Msg {
		if m.store == nil {
			return archiveSyncDoneMsg{total: total, cached: cached, failed: failed, err: fmt.Errorf("cache storage unavailable")}
		}

		if len(remaining) == 0 {
			return archiveSyncDoneMsg{total: total, cached: cached, failed: failed}
		}

		batchSize := min(len(remaining), archiveSyncBatchSize)
		batch := append([]string(nil), remaining[:batchSize]...)
		next := append([]string(nil), remaining[batchSize:]...)

		results := fetchPagesConcurrently(m.api, batch, 25*time.Second, archiveSyncWorkers)
		index.TotalTitles = total

		for _, result := range results {
			if result.err != nil {
				failed++
				continue
			}

			if err := m.store.WriteArchivePage(result.page.Title, result.page.URL, result.page.Markdown, &index); err != nil {
				failed++
				continue
			}

			cached++
		}

		processed := cached + failed
		shouldPersist := len(next) == 0
		if !shouldPersist && archiveSyncIndexFlushInterval > 0 {
			if processed > 0 && processed%archiveSyncIndexFlushInterval == 0 {
				shouldPersist = true
			}
		}

		if shouldPersist {
			index.TotalTitles = total
			index.UpdatedAt = time.Now().UTC()
			if err := m.store.SaveArchiveIndex(index); err != nil {
				return archiveSyncDoneMsg{total: total, cached: cached, failed: failed, err: err}
			}
		}

		return archiveSyncProgressMsg{
			phase:     "sync",
			processed: processed,
			total:     total,
			cached:    cached,
			failed:    failed,
			remaining: next,
			index:     index,
		}
	}
}

func (m *Model) archiveSyncDoneCmd(total, cached, failed int, err error) tea.Cmd {
	return func() tea.Msg {
		return archiveSyncDoneMsg{
			total:  total,
			cached: cached,
			failed: failed,
			err:    err,
		}
	}
}

func (m *Model) updateBrowseMode(msg tea.KeyMsg) tea.Cmd {
	if msg.String() == "ctrl+d" {
		return m.toggleOfflineLibraryPopup()
	}

	if cmd, handled := m.updateLinkTabMode(msg); handled {
		return cmd
	}

	switch msg.String() {
	case "ctrl+p":
		return m.toggleTOCPopup()

	case "ctrl+y":
		return m.toggleNavPopup()

	case "enter", "ctrl+m", "ctrl+j":
		if m.currentPage != nil {
			return nil
		}

		suggestion := strings.TrimSpace(m.homePrimarySuggestion())
		if suggestion == "" {
			return nil
		}
		return m.openPage(suggestion, true)

	case "/":
		m.mode = modeSearch
		m.searchInput.Focus()
		if len(m.searchResults) == 0 {
			m.searchInput.SetValue("")
		}
		m.searchInput.CursorEnd()
		return nil

	case "x":
		if m.store == nil {
			return m.setTransientStatus("Cache storage unavailable")
		}

		m.confirmDialog = &confirmDialogState{
			action:      confirmActionClearCache,
			title:       "Clear local cache?",
			message:     "Delete local page cache, full archive, and title index?",
			details:     []string{"This action cannot be undone."},
			yesLabel:    "Yes",
			noLabel:     "No",
			selectedYes: false,
		}
		return nil

	case "?":
		m.showHelp = true
		return nil

	case "tab":
		if m.currentPage == nil {
			return nil
		}
		m.cycleTab(1)
		return m.setTransientStatus("View: " + m.tabName(m.activeTab))

	case "shift+tab":
		if m.currentPage == nil {
			return nil
		}
		m.cycleTab(-1)
		return m.setTransientStatus("View: " + m.tabName(m.activeTab))

	case "1":
		if !m.activateTab(tabWiki) {
			return nil
		}
		return m.setTransientStatus("View: " + m.tabName(m.activeTab))

	case "2":
		if !m.activateTab(tabRelated) {
			return nil
		}
		return m.setTransientStatus("View: " + m.tabName(m.activeTab))

	case "3":
		if !m.activateTab(tabLinks) {
			return nil
		}
		return m.setTransientStatus("View: " + m.tabName(m.activeTab))

	case "esc", "h", "b":
		if len(m.backStack) == 0 {
			if msg.String() == "esc" {
				return nil
			}
			return m.setTransientStatus("Back stack is empty")
		}
		title := m.backStack[len(m.backStack)-1]
		m.backStack = m.backStack[:len(m.backStack)-1]
		m.pendingBackTitle = title
		return m.openPage(title, false)

	case "o":
		if m.currentPage == nil {
			return m.setTransientStatus("No page open")
		}
		url := m.currentPage.URL
		return func() tea.Msg {
			err := browser.OpenURL(url)
			return browserOpenedMsg{err: err}
		}

	case "c":
		if m.currentPage == nil {
			return m.setTransientStatus("No page open")
		}
		if len(m.currentPage.CodeBlocks) == 0 {
			return m.setTransientStatus("No code blocks found on this page")
		}
		idx := m.copyIndex % len(m.currentPage.CodeBlocks)
		code := m.currentPage.CodeBlocks[idx]
		m.copyIndex++
		total := len(m.currentPage.CodeBlocks)
		return func() tea.Msg {
			err := clipboard.WriteAll(code)
			return copiedCodeMsg{index: idx + 1, total: total, err: err}
		}

	case "d", "D":
		if m.archiveSyncLoading {
			return m.setTransientStatus("Full archive sync is running")
		}
		if m.currentPage == nil {
			return m.setTransientStatus("Open a page first to download offline bundle")
		}
		if m.cfg.ForceOffline {
			return m.setTransientStatus("Disable --offline to download fresh pages")
		}

		titles, skipped := m.offlineDownloadTitles(80)
		if len(titles) == 0 {
			return m.setTransientStatus("No ArchWiki pages available for offline download")
		}

		details := []string{fmt.Sprintf("Current page + internal links: %d pages", len(titles))}
		if skipped > 0 {
			details = append(details, fmt.Sprintf("%d additional links were skipped by bundle limit", skipped))
		}

		m.confirmDialog = &confirmDialogState{
			action:          confirmActionDownloadOffline,
			title:           "Download offline bundle?",
			message:         fmt.Sprintf("Download %d pages for offline reading now?", len(titles)),
			details:         details,
			yesLabel:        "Yes",
			noLabel:         "No",
			selectedYes:     true,
			downloadTitles:  titles,
			downloadSkipped: skipped,
		}
		return nil

	case "a", "A":
		if m.cfg.ForceOffline {
			return m.setTransientStatus("Disable --offline to sync full archive")
		}
		if m.store == nil {
			return m.setTransientStatus("Cache storage unavailable")
		}
		if m.downloadLoading {
			return m.setTransientStatus("Offline bundle download is running")
		}
		if m.pageLoading || m.searchLoading {
			return m.setTransientStatus("Wait for current request to finish before full sync")
		}
		if m.archiveSyncLoading {
			return m.setTransientStatus("Full archive sync is already running")
		}

		existing := len(m.store.ListArchiveTitles())
		details := []string{
			"Downloads all ArchWiki article pages and stores compressed markdown locally.",
			"Sync is incremental and resumes using a local archive index.",
			fmt.Sprintf("Already cached in full archive: %d pages", existing),
		}

		m.confirmDialog = &confirmDialogState{
			action:      confirmActionDownloadFullArchive,
			title:       "Sync full ArchWiki archive?",
			message:     "Fetch all ArchWiki pages for offline archive mode now?",
			details:     details,
			yesLabel:    "Yes",
			noLabel:     "No",
			selectedYes: true,
		}
		return nil

	case "up", "k":
		m.scrollContent(-1)
		return nil

	case "down", "j":
		m.scrollContent(1)
		return nil

	case "pgup", "ctrl+b":
		m.scrollContent(-(max(1, m.viewport.Height-2)))
		return nil

	case "pgdown", "ctrl+f", "space":
		m.scrollContent(max(1, m.viewport.Height-2))
		return nil

	case "ctrl+u":
		m.scrollContent(-(max(1, m.viewport.Height/2)))
		return nil

	case "g":
		m.viewport.YOffset = 0
		m.syncTOCCursorToViewport()
		return nil

	case "G":
		m.viewport.YOffset = m.maxViewportOffset()
		m.syncTOCCursorToViewport()
		return nil

	}
	return nil
}

func (m *Model) updateSearchMode(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		if strings.TrimSpace(m.searchInput.Value()) != "" || len(m.searchResults) > 0 {
			m.searchInput.SetValue("")
			m.searchCursor = 0
			m.searchResults = nil
			m.searchQuery = ""
			m.pendingSearchSeq++
			m.searchLoading = false
			m.syncLoadingState()
			m.activeSearchReq++
			return nil
		}

		m.mode = modeBrowse
		m.searchInput.Blur()
		m.pendingSearchSeq++
		m.searchLoading = false
		m.syncLoadingState()
		m.activeSearchReq++
		return nil

	case "up", "ctrl+p":
		if m.searchCursor > 0 {
			m.searchCursor--
		}
		return nil

	case "down", "ctrl+n":
		if m.searchCursor < len(m.searchResults)-1 {
			m.searchCursor++
		}
		return nil

	case "pgup":
		m.searchCursor = max(0, m.searchCursor-8)
		return nil

	case "pgdown":
		m.searchCursor = min(len(m.searchResults)-1, m.searchCursor+8)
		if m.searchCursor < 0 {
			m.searchCursor = 0
		}
		return nil

	case "enter", "ctrl+m", "ctrl+j":
		query := strings.TrimSpace(m.searchInput.Value())
		if query == "" {
			return nil
		}

		if len(m.searchResults) == 0 || !strings.EqualFold(query, m.searchQuery) {
			return m.startSearch(query, true)
		}

		if m.searchCursor < 0 || m.searchCursor >= len(m.searchResults) {
			m.searchCursor = 0
		}
		return m.openPage(m.searchResults[m.searchCursor].Title, true)
	}

	before := m.searchInput.Value()
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	after := strings.TrimSpace(m.searchInput.Value())

	if after == strings.TrimSpace(before) {
		return cmd
	}

	m.pendingSearchSeq++
	if after == "" {
		m.searchResults = nil
		m.searchCursor = 0
		m.searchLoading = false
		m.syncLoadingState()
		return cmd
	}

	return tea.Batch(cmd, debounceCmd(m.pendingSearchSeq, after))
}

func (m *Model) startSearch(query string, autoOpen bool) tea.Cmd {
	query = strings.TrimSpace(query)
	if query == "" {
		m.searchResults = nil
		m.searchCursor = 0
		m.searchLoading = false
		m.syncLoadingState()
		m.searchQuery = ""
		return nil
	}
	m.searchQuery = query

	indexed := m.localIndexedSearch(query, 40)
	if len(indexed) > 0 {
		m.searchResults = indexed
		m.searchCursor = 0
		m.status = fmt.Sprintf("%d indexed matches for \"%s\"", len(indexed), query)
		m.statusSticky = true
		if autoOpen {
			m.searchLoading = false
			m.syncLoadingState()
			return m.openPage(indexed[0].Title, true)
		}
	} else {
		m.searchResults = nil
		m.searchCursor = 0
	}

	if m.cfg.ForceOffline {
		m.searchLoading = false
		m.syncLoadingState()
		if len(m.searchResults) > 0 {
			return nil
		}
		return m.setTransientStatus("No local indexed results. Connect online to refresh title index.")
	}

	m.searchLoading = true
	m.syncLoadingState()
	m.activeSearchReq++
	requestID := m.activeSearchReq

	searchCmd := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		results, err := m.api.Search(ctx, query, 20)
		return searchResultMsg{
			requestID: requestID,
			query:     query,
			autoOpen:  autoOpen,
			results:   results,
			err:       err,
		}
	}

	return tea.Batch(searchCmd, m.spinner.Tick)
}

func (m *Model) refreshTitleIndexCmd() tea.Cmd {
	if m.titleIndex == nil {
		return nil
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		count, err := m.titleIndex.Refresh(ctx, m.api)
		return titleIndexRefreshedMsg{count: count, err: err}
	}
}

func (m *Model) localIndexedSearch(query string, limit int) []wiki.SearchResult {
	if m.titleIndex == nil || !m.titleIndex.Ready() {
		return nil
	}
	return m.titleIndex.Search(query, limit)
}

func (m *Model) openPage(title string, pushCurrent bool) tea.Cmd {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}

	// Leave search view immediately so users get instant feedback that page open started.
	m.mode = modeBrowse
	m.showNav = false
	m.showTOC = false
	m.tocItems = nil
	m.tocCursor = 0
	m.tocExpanded = nil
	m.searchInput.Blur()
	m.status = "Opening: " + title
	m.statusSticky = true
	m.searchLoading = false
	m.activeSearchReq++
	if pushCurrent {
		m.pendingBackTitle = ""
	}

	if pushCurrent && m.currentPage != nil && !strings.EqualFold(m.currentPage.Title, title) {
		m.backStack = append(m.backStack, m.currentPage.Title)
		if len(m.backStack) > 100 {
			m.backStack = m.backStack[len(m.backStack)-100:]
		}
	}

	m.pageLoading = true
	m.syncLoadingState()
	m.activePageReq++
	requestID := m.activePageReq

	pageCmd := func() tea.Msg {
		if m.cfg.ForceOffline {
			if m.store == nil {
				return pageLoadedMsg{requestID: requestID, err: fmt.Errorf("offline cache unavailable")}
			}
			cached, err := m.store.ReadPageCache(title)
			if err != nil {
				archived, archiveErr := m.store.ReadArchivePage(title)
				if archiveErr != nil {
					return pageLoadedMsg{requestID: requestID, err: fmt.Errorf("offline cache miss for %q", title)}
				}
				cached = archived
			}
			return pageLoadedMsg{
				requestID: requestID,
				fromCache: true,
				page: wiki.Page{
					Title:    title,
					URL:      wiki.ArticleURL(title),
					Markdown: cached,
				},
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		page, err := m.api.FetchPage(ctx, title)
		if err != nil && m.store != nil {
			cached, cacheErr := m.store.ReadPageCache(title)
			if cacheErr == nil {
				return pageLoadedMsg{
					requestID: requestID,
					fromCache: true,
					page: wiki.Page{
						Title:    title,
						URL:      wiki.ArticleURL(title),
						Markdown: cached,
					},
				}
			}

			archived, archiveErr := m.store.ReadArchivePage(title)
			if archiveErr == nil {
				return pageLoadedMsg{
					requestID: requestID,
					fromCache: true,
					page: wiki.Page{
						Title:    title,
						URL:      wiki.ArticleURL(title),
						Markdown: archived,
					},
				}
			}
		}
		if err != nil {
			return pageLoadedMsg{requestID: requestID, err: err}
		}

		return pageLoadedMsg{requestID: requestID, page: page}
	}

	return tea.Batch(pageCmd, m.spinner.Tick)
}

func (m *Model) setTransientStatus(text string) tea.Cmd {
	m.status = text
	m.statusSticky = false
	return tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func (m *Model) syncLoadingState() {
	m.loading = m.searchLoading || m.pageLoading || m.downloadLoading || m.archiveSyncLoading
}

func (m *Model) markdownForActiveTab() string {
	switch m.activeTab {
	case tabRelated:
		if strings.TrimSpace(m.relatedMarkdown) != "" {
			return m.relatedMarkdown
		}
	case tabLinks:
		if strings.TrimSpace(m.linksMarkdown) != "" {
			return m.linksMarkdown
		}
	default:
		if strings.TrimSpace(m.wikiMarkdown) != "" {
			return m.wikiMarkdown
		}
	}

	if strings.TrimSpace(m.wikiMarkdown) != "" {
		return m.wikiMarkdown
	}
	return m.currentMarkdown
}

func (m *Model) ensureTabLinkItemsBuilt() {
	if len(m.relatedTabItems) == 0 && len(m.relatedLinks) > 0 {
		m.relatedTabItems = buildTabLinkItems(m.relatedLinks)
	}
	if len(m.linksTabItems) == 0 && len(m.allLinks) > 0 {
		m.linksTabItems = buildTabLinkItems(m.allLinks)
	}
	m.clampTabLinkCursors()
}

func (m *Model) clampTabLinkCursors() {
	m.activeTabLinkCursor()
	active := m.activeTab

	m.activeTab = tabRelated
	m.activeTabLinkCursor()

	m.activeTab = tabLinks
	m.activeTabLinkCursor()

	m.activeTab = active
}

func (m *Model) activeTabLinkState() (*[]tabLinkItem, *int) {
	switch m.activeTab {
	case tabRelated:
		return &m.relatedTabItems, &m.relatedCursor
	case tabLinks:
		return &m.linksTabItems, &m.linksCursor
	default:
		return nil, nil
	}
}

func (m *Model) activeTabLinkExpandedState() *map[string]bool {
	switch m.activeTab {
	case tabRelated:
		return &m.relatedExpandedGroups
	case tabLinks:
		return &m.linksExpandedGroups
	default:
		return nil
	}
}

func groupTabLinkIndicesByGroup(items []tabLinkItem) (map[string][]int, []string) {
	grouped := make(map[string][]int, len(tabLinkGroupOrder))
	for i, item := range items {
		group := strings.TrimSpace(item.Group)
		if group == "" {
			group = linkGroupExternal
		}
		grouped[group] = append(grouped[group], i)
	}

	order := make([]string, 0, len(grouped))
	seen := make(map[string]bool, len(grouped))
	for _, group := range tabLinkGroupOrder {
		if len(grouped[group]) == 0 {
			continue
		}
		order = append(order, group)
		seen[group] = true
	}

	for _, item := range items {
		group := strings.TrimSpace(item.Group)
		if group == "" {
			group = linkGroupExternal
		}
		if seen[group] || len(grouped[group]) == 0 {
			continue
		}
		order = append(order, group)
		seen[group] = true
	}

	return grouped, order
}

func (m *Model) activeTabLinkGroupCounts(items []tabLinkItem) map[string]int {
	counts := make(map[string]int, len(tabLinkGroupOrder))
	for _, item := range items {
		group := strings.TrimSpace(item.Group)
		if group == "" {
			group = linkGroupExternal
		}
		counts[group]++
	}
	return counts
}

func (m *Model) isActiveTabLinkGroupExpanded(group string, total int) bool {
	if total <= tabLinkCollapsedVisible {
		return true
	}

	state := m.activeTabLinkExpandedState()
	if state == nil || *state == nil {
		return false
	}

	return (*state)[group]
}

func (m *Model) setActiveTabLinkGroupExpanded(group string, expanded bool) bool {
	items := m.activeTabLinkItems()
	if len(items) == 0 {
		return false
	}

	counts := m.activeTabLinkGroupCounts(items)
	if counts[group] <= tabLinkCollapsedVisible {
		return false
	}

	state := m.activeTabLinkExpandedState()
	if state == nil {
		return false
	}
	if *state == nil {
		*state = make(map[string]bool, len(tabLinkGroupOrder))
	}

	if (*state)[group] == expanded {
		return false
	}
	(*state)[group] = expanded
	return true
}

func (m *Model) toggleActiveTabLinkGroup(group string) bool {
	items := m.activeTabLinkItems()
	counts := m.activeTabLinkGroupCounts(items)
	if counts[group] <= tabLinkCollapsedVisible {
		return false
	}

	return m.setActiveTabLinkGroupExpanded(group, !m.isActiveTabLinkGroupExpanded(group, counts[group]))
}

func (m *Model) activeTabLinkVisibleIndices(items []tabLinkItem) []int {
	if len(items) == 0 {
		return nil
	}

	grouped, order := groupTabLinkIndicesByGroup(items)
	visible := make([]int, 0, len(items))
	for _, group := range order {
		indices := grouped[group]
		if len(indices) == 0 {
			continue
		}

		visibleCount := len(indices)
		if !m.isActiveTabLinkGroupExpanded(group, len(indices)) && len(indices) > tabLinkCollapsedVisible {
			visibleCount = tabLinkCollapsedVisible
		}

		visible = append(visible, indices[:visibleCount]...)
	}

	if len(visible) == 0 {
		return nil
	}

	return visible
}

func (m *Model) activeTabLinkItems() []tabLinkItem {
	items, _ := m.activeTabLinkState()
	if items == nil {
		return nil
	}
	return *items
}

func (m *Model) activeTabLinkCursor() int {
	items, cursor := m.activeTabLinkState()
	if items == nil || cursor == nil {
		return 0
	}

	if len(*items) == 0 {
		*cursor = 0
		return 0
	}
	visible := m.activeTabLinkVisibleIndices(*items)
	if len(visible) == 0 {
		*cursor = 0
		return 0
	}

	if *cursor < 0 {
		*cursor = 0
	}
	if *cursor >= len(*items) {
		*cursor = len(*items) - 1
	}

	if indexOfInt(visible, *cursor) >= 0 {
		return *cursor
	}

	for i := len(visible) - 1; i >= 0; i-- {
		if visible[i] < *cursor {
			*cursor = visible[i]
			return *cursor
		}
	}

	*cursor = visible[0]

	return *cursor
}

func (m *Model) setActiveTabLinkCursor(index int) {
	items, cursor := m.activeTabLinkState()
	if items == nil || cursor == nil {
		return
	}

	if len(*items) == 0 {
		*cursor = 0
		return
	}

	if index < 0 {
		index = 0
	}
	if index >= len(*items) {
		index = len(*items) - 1
	}
	*cursor = index

	visible := m.activeTabLinkVisibleIndices(*items)
	if len(visible) == 0 {
		*cursor = 0
		return
	}

	if indexOfInt(visible, *cursor) >= 0 {
		return
	}

	for _, v := range visible {
		if v >= *cursor {
			*cursor = v
			return
		}
	}

	*cursor = visible[len(visible)-1]
}

func (m *Model) moveActiveTabLinkCursor(delta int) {
	if delta == 0 {
		return
	}
	items := m.activeTabLinkItems()
	if len(items) == 0 {
		return
	}

	visible := m.activeTabLinkVisibleIndices(items)
	if len(visible) == 0 {
		return
	}

	current := m.activeTabLinkCursor()
	pos := indexOfInt(visible, current)
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

	m.setActiveTabLinkCursor(visible[pos])
	if m.activeTabLinkCursor() != current {
		m.renderPage()
	}
}

func (m *Model) selectedActiveTabLink() (tabLinkItem, bool) {
	items := m.activeTabLinkItems()
	if len(items) == 0 {
		return tabLinkItem{}, false
	}

	cursor := m.activeTabLinkCursor()
	if cursor < 0 || cursor >= len(items) {
		return tabLinkItem{}, false
	}

	return items[cursor], true
}

func (m *Model) selectedActiveTabLinkGroup() (string, bool) {
	item, ok := m.selectedActiveTabLink()
	if !ok {
		return "", false
	}

	group := strings.TrimSpace(item.Group)
	if group == "" {
		group = linkGroupExternal
	}
	return group, true
}

func (m *Model) resolveTabLinkURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, "#") {
		if m.currentPage != nil && strings.TrimSpace(m.currentPage.URL) != "" {
			return strings.TrimSpace(m.currentPage.URL) + raw
		}
		return "https://wiki.archlinux.org" + raw
	}

	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}

	if strings.HasPrefix(raw, "/") {
		return "https://wiki.archlinux.org" + raw
	}

	return raw
}

func (m *Model) openActiveTabLink(alwaysBrowser bool) tea.Cmd {
	item, ok := m.selectedActiveTabLink()
	if !ok {
		return m.setTransientStatus("No links in this tab")
	}

	if item.OpenInTUI && !alwaysBrowser && strings.TrimSpace(item.OpenTitle) != "" {
		return m.openPage(item.OpenTitle, true)
	}

	target := m.resolveTabLinkURL(item.URL)
	if strings.TrimSpace(target) == "" {
		return m.setTransientStatus("Selected link has no URL")
	}

	return func() tea.Msg {
		err := browser.OpenURL(target)
		return browserOpenedMsg{err: err}
	}
}

func (m *Model) copyActiveTabLink() tea.Cmd {
	item, ok := m.selectedActiveTabLink()
	if !ok {
		return m.setTransientStatus("No links in this tab")
	}

	target := m.resolveTabLinkURL(item.URL)
	if strings.TrimSpace(target) == "" {
		return m.setTransientStatus("Selected link has no URL")
	}

	if err := clipboard.WriteAll(target); err != nil {
		return m.setTransientStatus("Copy failed (install wl-copy or xclip): " + err.Error())
	}

	return m.setTransientStatus("Copied link URL")
}

func (m *Model) updateLinkTabMode(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m.currentPage == nil || (m.activeTab != tabRelated && m.activeTab != tabLinks) {
		return nil, false
	}

	items := m.activeTabLinkItems()
	if len(items) == 0 {
		switch msg.String() {
		case "enter", "ctrl+m", "ctrl+j", "o", "c", "l", "right", "left":
			return m.setTransientStatus("No links in this tab"), true
		}
	}
	visible := m.activeTabLinkVisibleIndices(items)

	switch msg.String() {
	case "up", "k":
		m.moveActiveTabLinkCursor(-1)
		return nil, true

	case "down", "j":
		m.moveActiveTabLinkCursor(1)
		return nil, true

	case "pgup", "ctrl+b":
		m.moveActiveTabLinkCursor(-max(1, m.viewport.Height-4))
		return nil, true

	case "pgdown", "ctrl+f", "space":
		m.moveActiveTabLinkCursor(max(1, m.viewport.Height-4))
		return nil, true

	case "ctrl+u":
		m.moveActiveTabLinkCursor(-max(1, m.viewport.Height/2))
		return nil, true

	case "g", "home":
		if len(visible) > 0 {
			m.setActiveTabLinkCursor(visible[0])
			m.renderPage()
		}
		return nil, true

	case "G", "end":
		if len(visible) > 0 {
			m.setActiveTabLinkCursor(visible[len(visible)-1])
			m.renderPage()
		}
		return nil, true

	case "l":
		group, ok := m.selectedActiveTabLinkGroup()
		if !ok {
			return nil, true
		}
		if !m.toggleActiveTabLinkGroup(group) {
			return m.setTransientStatus("Group is already fully visible"), true
		}
		m.renderPage()
		return nil, true

	case "right":
		group, ok := m.selectedActiveTabLinkGroup()
		if !ok {
			return nil, true
		}
		if !m.setActiveTabLinkGroupExpanded(group, true) {
			return m.setTransientStatus("Group is already expanded"), true
		}
		m.renderPage()
		return nil, true

	case "left":
		group, ok := m.selectedActiveTabLinkGroup()
		if !ok {
			return nil, true
		}
		if !m.setActiveTabLinkGroupExpanded(group, false) {
			return m.setTransientStatus("Group is already collapsed"), true
		}
		m.renderPage()
		return nil, true

	case "enter", "ctrl+m", "ctrl+j":
		return m.openActiveTabLink(false), true

	case "o":
		return m.openActiveTabLink(true), true

	case "c":
		return m.copyActiveTabLink(), true
	}

	return nil, false
}

func (m *Model) renderLinkGroupHeader(group string, count, width int, expanded, collapsible bool) string {
	if width < 20 {
		width = 20
	}

	indicator := "•"
	if collapsible {
		if expanded {
			indicator = "▼"
		} else {
			indicator = "▶"
		}
	}

	label := fmt.Sprintf("%s %s (%d)", indicator, group, count)
	header := "-- " + label + " "
	if fill := width - lipgloss.Width(header); fill > 0 {
		header += strings.Repeat("-", fill)
	}
	header = truncate(header, width)

	style := m.styles.Dim.Bold(true)
	switch group {
	case linkGroupArchWiki:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	case linkGroupManPages:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	case linkGroupAnchors:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Bold(true)
	case linkGroupExternal:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true)
	}

	return style.Render(header)
}

func (m *Model) renderLinkTabContent() string {
	items := m.activeTabLinkItems()
	if len(items) == 0 {
		return m.styles.Dim.Render("No links found in this section.")
	}

	visible := m.activeTabLinkVisibleIndices(items)
	if len(visible) == 0 {
		return m.styles.Dim.Render("No visible links in this section.")
	}

	cursor := m.activeTabLinkCursor()
	lineWidth := min(60, max(28, readableWrapWidth(m.viewport.Width)))
	titleWidth := min(40, max(16, lineWidth-4))

	grouped, groupOrder := groupTabLinkIndicesByGroup(items)
	cursorPos := indexOfInt(visible, cursor)
	if cursorPos < 0 {
		cursorPos = 0
	}

	lines := []string{
		m.styles.PanelTitle.Render(fmt.Sprintf("%s (%d)", m.tabName(m.activeTab), len(items))),
		"",
	}

	for groupIdx, group := range groupOrder {
		indices := grouped[group]
		if len(indices) == 0 {
			continue
		}

		expanded := m.isActiveTabLinkGroupExpanded(group, len(indices))
		collapsible := len(indices) > tabLinkCollapsedVisible
		visibleCount := len(indices)
		if collapsible && !expanded {
			visibleCount = tabLinkCollapsedVisible
		}

		lines = append(lines, m.renderLinkGroupHeader(group, len(indices), lineWidth, expanded, collapsible))

		for _, itemIndex := range indices[:visibleCount] {
			item := items[itemIndex]

			prefix := "  "
			style := m.styles.Item
			if itemIndex == cursor {
				prefix = "> "
				style = m.styles.ItemSelected
			}

			lines = append(lines, style.Render(prefix+truncate(item.Title, titleWidth)))
		}

		if collapsible && !expanded {
			hidden := len(indices) - visibleCount
			lines = append(lines, m.styles.Dim.Render("  ... more ("+strconv.Itoa(hidden)+") | l/→ expand"))
		}

		if groupIdx < len(groupOrder)-1 {
			lines = append(lines, "")
		}
	}

	selected := items[cursor]
	selectedURL := m.resolveTabLinkURL(selected.URL)
	if strings.TrimSpace(selectedURL) == "" {
		selectedURL = selected.URL
	}

	lines = append(lines,
		"",
		m.styles.Dim.Render(fmt.Sprintf("Position: %d/%d visible (%d total)", cursorPos+1, len(visible), len(items))),
		m.styles.Dim.Render("Selected:"),
		m.styles.Item.Render(truncate(selectedURL, max(20, lineWidth+14))),
	)
	return strings.Join(lines, "\n")
}

func (m *Model) activeTabLinkCursorRenderedLine(items []tabLinkItem, cursor int) int {
	if len(items) == 0 {
		return 0
	}

	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(items) {
		cursor = len(items) - 1
	}

	grouped, groupOrder := groupTabLinkIndicesByGroup(items)
	line := 2 // tab title + blank line
	for groupIdx, group := range groupOrder {
		indices := grouped[group]
		if len(indices) == 0 {
			continue
		}

		expanded := m.isActiveTabLinkGroupExpanded(group, len(indices))
		visibleCount := len(indices)
		if len(indices) > tabLinkCollapsedVisible && !expanded {
			visibleCount = tabLinkCollapsedVisible
		}

		line++ // group header
		for _, idx := range indices[:visibleCount] {
			if idx == cursor {
				return line
			}
			line++
		}

		if len(indices) > visibleCount {
			line++ // "... more" row
		}

		if groupIdx < len(groupOrder)-1 {
			line++ // group spacer
		}
	}

	return 0
}

func (m *Model) ensureActiveTabLinkCursorVisible() {
	items := m.activeTabLinkItems()
	if len(items) == 0 {
		m.viewport.YOffset = 0
		return
	}

	cursorLine := m.activeTabLinkCursorRenderedLine(items, m.activeTabLinkCursor())
	if cursorLine < m.viewport.YOffset {
		m.viewport.YOffset = cursorLine
		return
	}

	bottom := m.viewport.YOffset + m.viewport.Height - 1
	if cursorLine > bottom {
		m.viewport.YOffset = cursorLine - m.viewport.Height + 1
	}
}

func (m *Model) setArticleContent(rawMarkdown string) {
	doc := m.renderEngine.BuildDocument(rawMarkdown)
	m.wikiMarkdown = doc.WikiMarkdown
	m.relatedLinks = doc.RelatedLinks
	m.allLinks = doc.AllLinks
	m.relatedMarkdown = doc.RelatedMarkdown
	m.linksMarkdown = doc.LinksMarkdown
	m.relatedTabItems = buildTabLinkItems(m.relatedLinks)
	m.linksTabItems = buildTabLinkItems(m.allLinks)
	m.relatedCursor = 0
	m.linksCursor = 0
	m.relatedExpandedGroups = nil
	m.linksExpandedGroups = nil
	m.tocItems = nil
	m.tocCursor = 0
	m.tocExpanded = nil
	m.tocJumpOffset = -1
	m.renderedWikiContent = ""
	m.invalidateTOCOffsetCache()
}

func (m *Model) activateTab(tab contentTab) bool {
	if m.currentPage == nil {
		return false
	}
	if tab < tabWiki || tab > tabLinks {
		return false
	}

	m.ensureTabLinkItemsBuilt()

	m.activeTab = tab
	if tab != tabWiki {
		m.showTOC = false
	}
	m.viewport.YOffset = 0
	m.currentMarkdown = m.markdownForActiveTab()
	m.renderPage()
	return true
}

func (m *Model) cycleTab(step int) {
	if m.currentPage == nil {
		return
	}
	if step == 0 {
		return
	}

	next := int(m.activeTab) + step
	for next < int(tabWiki) {
		next += 3
	}
	next = next % 3
	_ = m.activateTab(contentTab(next))
}

func (m *Model) renderPage() {
	m.viewport.Height = m.browseViewportHeight(m.layoutBodyHeight())
	m.ensureTabLinkItemsBuilt()

	if m.activeTab == tabRelated || m.activeTab == tabLinks {
		rendered := m.renderLinkTabContent()
		m.viewport.SetContent(rendered)
		if strings.TrimSpace(rendered) == "" {
			m.contentLines = 0
			m.viewport.YOffset = 0
			return
		}
		m.contentLines = strings.Count(rendered, "\n") + 1
		m.ensureActiveTabLinkCursorVisible()
		m.clampViewport()
		return
	}

	markdown := m.markdownForActiveTab()
	m.currentMarkdown = markdown
	renderMarkdown := markdown
	var wikiLinks []wikiRenderLink
	if m.activeTab == tabWiki {
		renderMarkdown, wikiLinks = m.renderEngine.PrepareWikiMarkdownForDisplay(markdown)
	}

	if strings.TrimSpace(renderMarkdown) == "" {
		if m.activeTab == tabWiki {
			m.renderedWikiContent = ""
		}
		m.viewport.SetContent("")
		m.contentLines = 0
		return
	}

	wrapWidth := readableWrapWidth(m.viewport.Width)
	renderer, err := m.rendererForWidth(wrapWidth)
	if err != nil {
		if m.activeTab == tabWiki {
			m.renderedWikiContent = renderMarkdown
		}
		m.viewport.SetContent(renderMarkdown)
		m.contentLines = strings.Count(renderMarkdown, "\n") + 1
		return
	}

	rendered, err := renderer.Render(renderMarkdown)
	if err != nil {
		if m.activeTab == tabWiki {
			m.renderedWikiContent = renderMarkdown
		}
		m.viewport.SetContent(renderMarkdown)
		m.contentLines = strings.Count(renderMarkdown, "\n") + 1
		return
	}

	rendered = strings.TrimRight(rendered, "\n")
	if m.activeTab == tabWiki {
		rendered = m.renderEngine.ApplyCalloutStyles(rendered)
		rendered = m.renderEngine.ApplyWikiRenderLinks(rendered, wikiLinks)
		rendered = m.applyTOCJumpHighlight(rendered)
		m.renderedWikiContent = rendered
	}
	m.viewport.SetContent(rendered)
	m.contentLines = strings.Count(rendered, "\n") + 1
	m.clampViewport()
}

func (m *Model) applyTOCJumpHighlight(rendered string) string {
	if !m.tocJumpHighlight || m.activeTab != tabWiki {
		return rendered
	}
	if strings.TrimSpace(rendered) == "" {
		return rendered
	}

	lineIndex := m.tocJumpOffset
	if lineIndex < 0 {
		lineIndex = m.renderedOffsetForMarkdownLine(m.tocJumpLine)
	}
	lines := strings.Split(rendered, "\n")
	if lineIndex < 0 || lineIndex >= len(lines) {
		return rendered
	}

	target := lineIndex
	for target < len(lines) {
		if strings.TrimSpace(stripANSISequences(lines[target])) != "" {
			break
		}
		target++
	}
	if target >= len(lines) {
		return rendered
	}

	highlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("25")).Bold(true)
	lines[target] = highlightStyle.Render(stripANSISequences(lines[target]))
	return strings.Join(lines, "\n")
}

func (m *Model) rendererForWidth(wrapWidth int) (*glamour.TermRenderer, error) {
	if m.renderer != nil && m.rendererWrap == wrapWidth {
		return m.renderer, nil
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(render.ReadingGlamourStyle()),
		glamour.WithWordWrap(wrapWidth),
	)
	if err != nil {
		return nil, err
	}

	m.renderer = renderer
	m.rendererWrap = wrapWidth
	return renderer, nil
}

func (m *Model) resizeComponents() {
	bodyHeight := m.layoutBodyHeight()

	mainWidth := m.width
	if mainWidth <= 0 {
		mainWidth = 120
	}

	m.viewport.Width = max(20, mainWidth-10)
	m.viewport.Height = m.browseViewportHeight(bodyHeight)
	m.searchInput.Width = max(20, min(80, mainWidth-14))
	m.offlineFilterInput.Width = max(20, min(86, mainWidth-14))

	m.renderPage()
}

func (m *Model) layoutBodyHeight() int {
	if m.height <= 0 {
		return 24
	}

	headerHeight := 2
	statusHeight := 2
	if m.ready {
		headerHeight = lipgloss.Height(m.viewHeader())
		statusHeight = lipgloss.Height(m.viewStatusBar())
	}

	return max(6, m.height-headerHeight-statusHeight)
}

func (m *Model) browseViewportHeight(bodyHeight int) int {
	panelHeight := max(6, bodyHeight)
	innerHeight := max(5, panelHeight-2)

	chromeLines := 1 // panel title
	if m.currentPage != nil {
		chromeLines += 3 // spacing + tabs
		if m.loading {
			chromeLines++
		}
	}

	return max(3, innerHeight-chromeLines)
}

func (m *Model) scrollContent(delta int) {
	if m.currentPage == nil || delta == 0 {
		return
	}

	m.viewport.YOffset += delta
	m.clampViewport()
	m.syncTOCCursorToViewport()
}

func (m *Model) syncTOCCursorToViewport() {
	if m.currentPage == nil || m.activeTab != tabWiki || m.showTOC {
		return
	}

	if len(m.tocItems) == 0 {
		m.tocItems = buildTOCItems(m.wikiMarkdown)
		m.invalidateTOCOffsetCache()
	}
	if len(m.tocItems) == 0 {
		return
	}

	m.tocCursor = m.nearestTOCIndexForViewport()
}

func (m *Model) clearTOCJumpHighlight() {
	if !m.tocJumpHighlight {
		return
	}

	m.tocJumpHighlight = false
	m.tocJumpLine = 0
	m.tocJumpOffset = -1
	if m.currentPage != nil {
		m.renderPage()
	}
}

func (m *Model) clampViewport() {
	if m.viewport.YOffset < 0 {
		m.viewport.YOffset = 0
		return
	}
	maxOffset := m.maxViewportOffset()
	if m.viewport.YOffset > maxOffset {
		m.viewport.YOffset = maxOffset
	}
}

func (m *Model) maxViewportOffset() int {
	if m.contentLines <= m.viewport.Height {
		return 0
	}
	return max(0, m.contentLines-m.viewport.Height)
}

func (m *Model) scrollInfo() string {
	if m.contentLines == 0 {
		return ""
	}
	total := m.contentLines
	line := min(total, m.viewport.YOffset+1)
	if total <= 0 {
		return ""
	}
	percent := int(float64(min(total, m.viewport.YOffset+m.viewport.Height)) * 100 / float64(total))
	return fmt.Sprintf("%d%% line %d/%d", percent, line, total)
}

func debounceCmd(seq int, query string) tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(_ time.Time) tea.Msg {
		return debounceSearchMsg{seq: seq, query: query}
	})
}

func clampWindow(cursor, total, height int) (int, int) {
	if total <= 0 || height <= 0 {
		return 0, 0
	}
	if total <= height {
		return 0, total
	}

	start := cursor - (height / 2)
	if start < 0 {
		start = 0
	}
	if start+height > total {
		start = total - height
	}
	return start, start + height
}

func readableWrapWidth(viewportWidth int) int {
	if viewportWidth <= 0 {
		return 30
	}

	width := viewportWidth - 4
	if width < 30 {
		width = 30
	}
	if width > maxReadableArticleWidth {
		width = maxReadableArticleWidth
	}
	return width
}

func centeredLeftPadding(viewportWidth, contentWidth int) int {
	if viewportWidth <= contentWidth {
		return 0
	}
	leftPad := (viewportWidth - contentWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	return leftPad
}

func padRenderedBlock(block string, leftPad int) string {
	if leftPad <= 0 || block == "" {
		return block
	}

	pad := strings.Repeat(" ", leftPad)
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines[i] = pad + line
	}

	return strings.Join(lines, "\n")
}

func indexOfInt(items []int, target int) int {
	for i, item := range items {
		if item == target {
			return i
		}
	}
	return -1
}

func (m *Model) tocParentIndex(index int) int {
	if index <= 0 || index >= len(m.tocItems) {
		return -1
	}

	level := m.tocItems[index].Level
	for i := index - 1; i >= 0; i-- {
		if m.tocItems[i].Level < level {
			return i
		}
	}

	return -1
}

func (m *Model) tocPathToRoot(index int) []int {
	if index < 0 || index >= len(m.tocItems) {
		return nil
	}

	path := []int{index}
	parent := m.tocParentIndex(index)
	for parent >= 0 {
		path = append(path, parent)
		parent = m.tocParentIndex(parent)
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return path
}

func (m *Model) tocHasChildren(index int) bool {
	if index < 0 || index >= len(m.tocItems)-1 {
		return false
	}
	return m.tocItems[index+1].Level > m.tocItems[index].Level
}

func (m *Model) visibleTOCIndices() []int {
	if len(m.tocItems) == 0 {
		return nil
	}

	visible := make([]int, 0, len(m.tocItems))
	for i := range m.tocItems {
		parent := m.tocParentIndex(i)
		show := true
		for parent >= 0 {
			if m.tocExpanded == nil || !m.tocExpanded[parent] {
				show = false
				break
			}
			parent = m.tocParentIndex(parent)
		}

		if show {
			visible = append(visible, i)
		}
	}

	if len(visible) == 0 {
		visible = append(visible, 0)
	}
	return visible
}

func (m *Model) ensureTOCCursorVisible(visible []int) {
	if len(m.tocItems) == 0 {
		m.tocCursor = 0
		return
	}
	if len(visible) == 0 {
		m.tocCursor = 0
		return
	}

	if indexOfInt(visible, m.tocCursor) >= 0 {
		return
	}

	for parent := m.tocParentIndex(m.tocCursor); parent >= 0; parent = m.tocParentIndex(parent) {
		if indexOfInt(visible, parent) >= 0 {
			m.tocCursor = parent
			return
		}
	}

	m.tocCursor = visible[0]
}

func (m *Model) ensureNavigationPath() {
	if m.currentPage == nil {
		return
	}

	path := make([]string, 0, len(m.backStack)+1)
	for _, title := range m.backStack {
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		if len(path) > 0 && strings.EqualFold(path[len(path)-1], title) {
			continue
		}
		path = append(path, title)
	}

	currentTitle := strings.TrimSpace(m.currentPage.Title)
	if currentTitle == "" {
		return
	}
	if len(path) == 0 || !strings.EqualFold(path[len(path)-1], currentTitle) {
		path = append(path, currentTitle)
	}

	if len(path) == 0 {
		return
	}

	parent := -1
	current := -1
	for _, title := range path {
		current = m.navFindOrCreateChild(parent, title)
		parent = current
	}

	if current < 0 {
		return
	}

	m.navCurrent = current
	if m.navExpanded == nil {
		m.navExpanded = make(map[int]bool)
	}
	for _, idx := range m.navPathToRoot(current) {
		m.navExpanded[idx] = true
	}
}

func (m *Model) navFindOrCreateChild(parent int, title string) int {
	title = strings.TrimSpace(title)
	if title == "" {
		return -1
	}

	if existing := m.navFindExistingChild(parent, title); existing >= 0 {
		return existing
	}

	idx := len(m.navNodes)
	m.navNodes = append(m.navNodes, navNode{
		Title:  title,
		Parent: parent,
	})

	if parent >= 0 {
		m.navNodes[parent].Children = append(m.navNodes[parent].Children, idx)
	} else {
		m.navRoots = append(m.navRoots, idx)
	}

	return idx
}

func (m *Model) navFindExistingChild(parent int, title string) int {
	title = strings.TrimSpace(title)
	if title == "" {
		return -1
	}

	if parent >= 0 && parent < len(m.navNodes) {
		for _, child := range m.navNodes[parent].Children {
			if child < 0 || child >= len(m.navNodes) {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(m.navNodes[child].Title), title) {
				return child
			}
		}
		return -1
	}

	for _, root := range m.navRoots {
		if root < 0 || root >= len(m.navNodes) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(m.navNodes[root].Title), title) {
			return root
		}
	}

	return -1
}

func (m *Model) navPathToRoot(index int) []int {
	if index < 0 || index >= len(m.navNodes) {
		return nil
	}

	path := []int{index}
	for parent := m.navNodes[index].Parent; parent >= 0; {
		path = append(path, parent)
		if parent < 0 || parent >= len(m.navNodes) {
			break
		}
		parent = m.navNodes[parent].Parent
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	return path
}

func (m *Model) navDepth(index int) int {
	if index < 0 || index >= len(m.navNodes) {
		return 0
	}

	depth := 0
	for parent := m.navNodes[index].Parent; parent >= 0; {
		depth++
		if parent < 0 || parent >= len(m.navNodes) {
			break
		}
		parent = m.navNodes[parent].Parent
	}

	return depth
}

func (m *Model) visibleNavIndices() []int {
	if len(m.navNodes) == 0 || len(m.navRoots) == 0 {
		return nil
	}

	visible := make([]int, 0, len(m.navNodes))
	var walk func(int)
	walk = func(index int) {
		if index < 0 || index >= len(m.navNodes) {
			return
		}

		visible = append(visible, index)
		if m.navExpanded == nil || !m.navExpanded[index] {
			return
		}
		for _, child := range m.navNodes[index].Children {
			walk(child)
		}
	}

	for _, root := range m.navRoots {
		walk(root)
	}

	if len(visible) == 0 {
		if len(m.navRoots) > 0 {
			return []int{m.navRoots[0]}
		}
		return nil
	}

	return visible
}

func (m *Model) ensureNavCursorVisible(visible []int) {
	if len(visible) == 0 {
		m.navCursor = 0
		return
	}

	if indexOfInt(visible, m.navCursor) >= 0 {
		return
	}
	if indexOfInt(visible, m.navCurrent) >= 0 {
		m.navCursor = m.navCurrent
		return
	}

	m.navCursor = visible[0]
}

func (m *Model) nearestTOCIndexForViewport() int {
	if len(m.tocItems) == 0 {
		return 0
	}

	offsets := m.tocRenderedOffsets()
	if len(offsets) == 0 {
		return 0
	}

	top := m.viewport.YOffset
	bottom := top
	if m.viewport.Height > 0 {
		bottom = top + m.viewport.Height - 1
	}

	best := 0
	for i, offset := range offsets {
		if offset < top {
			best = i
			continue
		}
		if offset <= bottom {
			return i
		}
		break
	}

	return best
}

func (m *Model) tocRenderedOffsets() []int {
	if len(m.tocItems) == 0 {
		return nil
	}

	wrapWidth := readableWrapWidth(m.viewport.Width)
	if m.tocOffsetCache == nil || m.tocOffsetWrap != wrapWidth {
		m.tocOffsetCache = make(map[int]int, len(m.tocItems))
		m.tocOffsetWrap = wrapWidth
	}
	m.fillTOCOffsetCacheFromRendered()

	offsets := make([]int, len(m.tocItems))
	for i, item := range m.tocItems {
		if cached, ok := m.tocOffsetCache[item.MarkdownLine]; ok {
			offsets[i] = cached
			continue
		}

		offset := m.approximateRenderedOffsetForMarkdownLine(item.MarkdownLine)
		m.tocOffsetCache[item.MarkdownLine] = offset
		offsets[i] = offset
	}

	return offsets
}

func (m *Model) fillTOCOffsetCacheFromRendered() {
	if len(m.tocItems) == 0 || m.tocOffsetCache == nil {
		return
	}

	rendered := strings.TrimSpace(m.renderedWikiContent)
	if rendered == "" {
		return
	}

	lines := strings.Split(strings.ReplaceAll(rendered, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return
	}

	normLines := make([]string, len(lines))
	for i, line := range lines {
		normLines[i] = normalizeTOCMatchText(line)
	}

	lineCursor := 0
	for _, item := range m.tocItems {
		if _, ok := m.tocOffsetCache[item.MarkdownLine]; ok {
			continue
		}

		titleNorm := normalizeTOCMatchText(item.Title)
		if titleNorm == "" {
			continue
		}

		match := -1
		for i := lineCursor; i < len(normLines); i++ {
			if tocLineWindowMatches(normLines, i, titleNorm) {
				match = i
				break
			}
		}

		if match >= 0 {
			m.tocOffsetCache[item.MarkdownLine] = match
			lineCursor = match + 1
		}
	}
}

func (m *Model) approximateRenderedOffsetForMarkdownLine(markdownLine int) int {
	source := m.wikiMarkdown
	if strings.TrimSpace(source) == "" {
		source = m.currentMarkdown
	}

	markdownLines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	if len(markdownLines) <= 1 {
		return max(0, markdownLine-1)
	}

	if markdownLine <= 0 {
		return 0
	}
	if markdownLine >= len(markdownLines) {
		markdownLine = len(markdownLines) - 1
	}

	renderedLines := m.contentLines
	if renderedLines <= 0 && strings.TrimSpace(m.renderedWikiContent) != "" {
		renderedLines = strings.Count(m.renderedWikiContent, "\n") + 1
	}
	if renderedLines <= 1 {
		return max(0, markdownLine-1)
	}

	ratio := float64(markdownLine) / float64(len(markdownLines)-1)
	offset := int(ratio * float64(renderedLines-1))
	if offset < 0 {
		return 0
	}
	if offset >= renderedLines {
		return renderedLines - 1
	}
	return offset
}

func normalizeTOCMatchText(text string) string {
	text = strings.ToLower(strings.TrimSpace(stripANSISequences(text)))
	if text == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(text))
	lastSpace := true

	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastSpace = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '/':
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		}
	}

	return strings.TrimSpace(b.String())
}

func tocLineWindowMatches(lines []string, start int, title string) bool {
	if start < 0 || start >= len(lines) || title == "" {
		return false
	}

	joined := ""
	for i := start; i < len(lines) && i < start+3; i++ {
		part := strings.TrimSpace(lines[i])
		if part == "" {
			continue
		}

		if joined == "" {
			joined = part
		} else {
			joined += " " + part
		}

		if joined == title {
			return true
		}
	}

	return false
}

func (m *Model) invalidateTOCOffsetCache() {
	m.tocOffsetCache = nil
	m.tocOffsetWrap = 0
}

func (m *Model) jumpToTOCItem(index int) tea.Cmd {
	if index < 0 || index >= len(m.tocItems) {
		return m.setTransientStatus("Invalid table-of-contents selection")
	}

	target := m.tocItems[index]
	offset := m.renderedOffsetForMarkdownLine(target.MarkdownLine)
	m.tocJumpOffset = offset
	if m.tocOffsetCache != nil {
		m.tocOffsetCache[target.MarkdownLine] = offset
	}
	m.viewport.YOffset = offset
	m.clampViewport()
	m.tocCursor = index
	m.tocJumpHighlight = true
	m.tocJumpLine = target.MarkdownLine
	m.renderPage()

	return m.setTransientStatus("Jumped to " + cleanTOCTitle(target.Title))
}

func (m *Model) renderedOffsetForMarkdownLine(markdownLine int) int {
	source := m.wikiMarkdown
	if strings.TrimSpace(source) == "" {
		source = m.currentMarkdown
	}

	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	if len(lines) == 0 || markdownLine <= 0 {
		return 0
	}
	if markdownLine >= len(lines) {
		markdownLine = len(lines) - 1
	}

	prefix := strings.Join(lines[:markdownLine], "\n")
	if strings.TrimSpace(prefix) == "" {
		return 0
	}

	toRender := prefix
	if strings.TrimSpace(m.wikiMarkdown) != "" {
		if prepared, _ := m.renderEngine.PrepareWikiMarkdownForDisplay(prefix); strings.TrimSpace(prepared) != "" {
			toRender = prepared
		}
	}

	renderer, err := m.rendererForWidth(readableWrapWidth(m.viewport.Width))
	if err != nil {
		return max(0, markdownLine-1)
	}

	rendered, err := renderer.Render(toRender)
	if err != nil {
		return max(0, markdownLine-1)
	}

	rendered = strings.TrimRight(rendered, "\n")
	if strings.TrimSpace(m.wikiMarkdown) != "" {
		rendered = m.renderEngine.ApplyCalloutStyles(rendered)
	}

	if strings.TrimSpace(rendered) == "" {
		return 0
	}

	return strings.Count(rendered, "\n") + 1
}

func buildTOCItems(markdown string) []tocItem {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	if strings.TrimSpace(markdown) == "" {
		return nil
	}

	lines := strings.Split(markdown, "\n")
	items := make([]tocItem, 0, 24)

	for i, line := range lines {
		match := tocHeadingRE.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) < 3 {
			continue
		}

		level := len(match[1])
		if level == 1 {
			// ArchWiki markdown conversion can emit stray H1 lines from callouts.
			// Keep only a possible document title near the top.
			if len(items) > 0 || i > 20 {
				continue
			}
		}

		title := cleanTOCTitle(match[2])
		if title == "" {
			continue
		}

		items = append(items, tocItem{
			Title:        title,
			Level:        level,
			MarkdownLine: i,
		})
	}

	if len(items) == 0 {
		return nil
	}

	minLevel := items[0].Level
	for _, item := range items {
		if item.Level < minLevel {
			minLevel = item.Level
		}
	}
	if minLevel > 1 {
		for i := range items {
			items[i].Level = items[i].Level - minLevel + 1
		}
	}

	return items
}

func cleanTOCTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	title = tocMarkdownLinkRE.ReplaceAllString(title, "$1")
	title = strings.ReplaceAll(title, "`", "")
	title = strings.TrimSpace(strings.Join(strings.Fields(title), " "))
	return title
}

func buildTabLinkItems(links []articleLink) []tabLinkItem {
	if len(links) == 0 {
		return nil
	}

	buckets := map[string][]tabLinkItem{
		linkGroupArchWiki: nil,
		linkGroupManPages: nil,
		linkGroupAnchors:  nil,
		linkGroupExternal: nil,
	}

	seen := make(map[string]struct{}, len(links))
	for _, link := range links {
		rawURL := strings.TrimSpace(link.URL)
		if rawURL == "" {
			continue
		}

		normalizedURL := canonicalizeTabLinkURL(rawURL)
		key := normalizedTabLinkURLKey(normalizedURL)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		title := cleanTabLinkTitle(link.Text, rawURL)
		group, openTitle, openInTUI := classifyTabLink(rawURL, title)

		buckets[group] = append(buckets[group], tabLinkItem{
			Title:     title,
			URL:       normalizedURL,
			Group:     group,
			OpenTitle: openTitle,
			OpenInTUI: openInTUI,
		})
	}

	order := []string{linkGroupArchWiki, linkGroupManPages, linkGroupAnchors, linkGroupExternal}
	out := make([]tabLinkItem, 0, len(links))
	for _, group := range order {
		out = append(out, buckets[group]...)
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func classifyTabLink(rawURL, title string) (string, string, bool) {
	raw := strings.ToLower(strings.TrimSpace(rawURL))

	if strings.HasPrefix(raw, "#") {
		return linkGroupAnchors, "", false
	}

	if strings.Contains(raw, "man.archlinux.org") || manPageTitleRE.MatchString(strings.TrimSpace(title)) {
		return linkGroupManPages, "", false
	}

	if strings.Contains(raw, "#") {
		if _, ok := render.ArchWikiTitleFromURL(rawURL); ok {
			return linkGroupAnchors, "", false
		}
	}

	if pageTitle, ok := render.ArchWikiTitleFromURL(rawURL); ok {
		return linkGroupArchWiki, pageTitle, true
	}

	return linkGroupExternal, "", false
}

func canonicalizeTabLinkURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}

	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "?") {
		return "https://wiki.archlinux.org" + raw
	}

	return raw
}

func normalizedTabLinkURLKey(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err == nil && parsed.Host != "" {
		parsed.Scheme = strings.ToLower(parsed.Scheme)
		parsed.Host = strings.ToLower(parsed.Host)
		if parsed.Path != "/" {
			parsed.Path = strings.TrimRight(parsed.Path, "/")
		}
		return strings.ToLower(parsed.String())
	}

	return strings.ToLower(strings.TrimRight(raw, "/"))
}

func cleanTabLinkTitle(rawTitle, rawURL string) string {
	title := strings.TrimSpace(rawTitle)
	if strings.Contains(title, "://") {
		if host := hostFromURL(title); host != "" {
			title = host
		}
	}

	if title == "" {
		if wikiTitle, ok := render.ArchWikiTitleFromURL(rawURL); ok {
			title = wikiTitle
		}
	}

	title = strings.ReplaceAll(title, "_", " ")
	if idx := strings.Index(title, "#"); idx > 0 {
		base := strings.TrimSpace(title[:idx])
		anchor := strings.TrimSpace(strings.ReplaceAll(title[idx+1:], "_", " "))
		anchor = strings.TrimSpace(strings.Join(strings.Fields(anchor), " "))

		switch {
		case base != "" && anchor != "":
			title = base + " (" + anchor + ")"
		case base != "":
			title = base
		}
	}

	title = strings.TrimSpace(strings.Join(strings.Fields(title), " "))
	if title != "" {
		return title
	}

	if host := hostFromURL(rawURL); host != "" {
		return host
	}

	if strings.HasPrefix(strings.TrimSpace(rawURL), "#") {
		anchor := strings.TrimPrefix(strings.TrimSpace(rawURL), "#")
		anchor = strings.TrimSpace(strings.Join(strings.Fields(strings.ReplaceAll(anchor, "_", " ")), " "))
		if anchor == "" {
			return "Current section"
		}
		return "Section (" + anchor + ")"
	}

	return strings.TrimSpace(rawURL)
}

func hostFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

func appendBlockLines(dst []string, block string) []string {
	if block == "" {
		return append(dst, "")
	}
	block = strings.ReplaceAll(block, "\r\n", "\n")
	return append(dst, strings.Split(block, "\n")...)
}

func padLines(lines []string, height int) []string {
	if len(lines) >= height {
		return lines[:height]
	}
	out := make([]string, 0, height)
	out = append(out, lines...)
	for len(out) < height {
		out = append(out, "")
	}
	return out
}

func distributeRow(left, center, right string, width int) string {
	if width <= 0 {
		return left + center + right
	}

	leftW := lipgloss.Width(left)
	centerW := lipgloss.Width(center)
	rightW := lipgloss.Width(right)

	if leftW+centerW+rightW >= width {
		combined := left + " " + center + " " + right
		return truncate(combined, width)
	}

	remaining := width - leftW - centerW - rightW
	leftGap := remaining / 2
	rightGap := remaining - leftGap
	return left + strings.Repeat(" ", leftGap) + center + strings.Repeat(" ", rightGap) + right
}

func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 1 {
		return "…"
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)+"…") > maxWidth {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func (m *Model) handleMouseClick(msg tea.MouseMsg) tea.Cmd {
	if m.mode != modeBrowse || m.currentPage == nil {
		return nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return nil
	}

	target := m.linkURLAtScreenPosition(msg.X, msg.Y)
	if strings.TrimSpace(target) == "" {
		return nil
	}

	title, ok := render.ArchWikiTitleFromURL(target)
	if !ok {
		return m.setTransientStatus("External link ignored (ArchWiki links open in TUI)")
	}

	return m.openPage(title, true)
}

func (m *Model) linkURLAtScreenPosition(x, y int) string {
	if x < 0 || y < 0 {
		return ""
	}

	lines := strings.Split(m.View(), "\n")
	if y >= len(lines) {
		return ""
	}

	line := lines[y]
	if link := linkURLAtDisplayColumn(line, x); link != "" {
		return link
	}
	return plainURLAtDisplayColumn(line, x)
}

func linkURLAtDisplayColumn(rawLine string, targetCol int) string {
	if targetCol < 0 || rawLine == "" {
		return ""
	}

	activeURL := ""
	col := 0

	for i := 0; i < len(rawLine); {
		if rawLine[i] == '\x1b' {
			if strings.HasPrefix(rawLine[i:], "\x1b]8;;") {
				start := i + len("\x1b]8;;")
				endRel := strings.Index(rawLine[start:], "\x1b\\")
				if endRel == -1 {
					break
				}
				activeURL = rawLine[start : start+endRel]
				i = start + endRel + 2
				continue
			}

			next := skipEscapeSequence(rawLine, i)
			if next <= i {
				i++
			} else {
				i = next
			}
			continue
		}

		r, size := utf8.DecodeRuneInString(rawLine[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}

		w := lipgloss.Width(string(r))
		if w < 1 {
			w = 1
		}
		if targetCol >= col && targetCol < col+w {
			return trimURLPunctuation(activeURL)
		}

		col += w
		i += size
	}

	return ""
}

func plainURLAtDisplayColumn(rawLine string, targetCol int) string {
	if targetCol < 0 || rawLine == "" {
		return ""
	}

	plain := stripANSISequences(rawLine)
	if plain == "" {
		return ""
	}

	matches := plainURLRE.FindAllStringIndex(plain, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		startCol := lipgloss.Width(plain[:match[0]])
		endCol := lipgloss.Width(plain[:match[1]])
		if targetCol < startCol || targetCol >= endCol {
			continue
		}

		return trimURLPunctuation(plain[match[0]:match[1]])
	}

	return ""
}

func stripANSISequences(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			next := skipEscapeSequence(s, i)
			if next <= i {
				i++
			} else {
				i = next
			}
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		b.WriteRune(r)
		i += size
	}

	return b.String()
}

func skipEscapeSequence(s string, i int) int {
	if i < 0 || i >= len(s) || s[i] != '\x1b' {
		return i + 1
	}
	if i+1 >= len(s) {
		return len(s)
	}

	switch s[i+1] {
	case '[':
		j := i + 2
		for j < len(s) {
			b := s[j]
			if b >= 0x40 && b <= 0x7E {
				return j + 1
			}
			j++
		}
		return len(s)
	case ']':
		j := i + 2
		for j < len(s) {
			if s[j] == '\a' {
				return j + 1
			}
			if s[j] == '\x1b' && j+1 < len(s) && s[j+1] == '\\' {
				return j + 2
			}
			j++
		}
		return len(s)
	default:
		if i+2 <= len(s) {
			return i + 2
		}
		return len(s)
	}
}

func trimURLPunctuation(raw string) string {
	return strings.TrimRight(raw, ".,;:!?)\"]'")
}

func mergeSearchResults(primary, secondary []wiki.SearchResult, limit int) []wiki.SearchResult {
	if limit <= 0 {
		limit = 40
	}

	out := make([]wiki.SearchResult, 0, limit)
	seen := make(map[string]int, limit*2)

	appendSet := func(set []wiki.SearchResult) {
		for _, item := range set {
			title := strings.TrimSpace(item.Title)
			if title == "" {
				continue
			}
			key := strings.ToLower(title)

			if at, exists := seen[key]; exists {
				if out[at].Snippet == "" && item.Snippet != "" {
					out[at].Snippet = item.Snippet
				}
				if out[at].URL == "" && item.URL != "" {
					out[at].URL = item.URL
				}
				continue
			}

			out = append(out, item)
			seen[key] = len(out) - 1
			if len(out) >= limit {
				return
			}
		}
	}

	appendSet(primary)
	if len(out) < limit {
		appendSet(secondary)
	}

	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func resolveIndexCachePath(store *storage.Store) string {
	if store != nil {
		return store.IndexCachePath()
	}

	cacheBase, err := os.UserCacheDir()
	if err != nil {
		return ""
	}

	cacheRoot := filepath.Join(cacheBase, "archwiki-tui")
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return ""
	}

	return filepath.Join(cacheRoot, "title-index.json")
}

func fetchPagesConcurrently(api *wiki.Client, titles []string, timeout time.Duration, workers int) []pageFetchResult {
	if api == nil || len(titles) == 0 {
		return nil
	}
	if workers <= 0 {
		workers = 1
	}
	if workers > len(titles) {
		workers = len(titles)
	}

	results := make([]pageFetchResult, len(titles))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for i, raw := range titles {
		title := strings.TrimSpace(raw)
		if title == "" {
			results[i] = pageFetchResult{title: raw, err: fmt.Errorf("empty title")}
			continue
		}

		wg.Add(1)
		go func(idx int, pageTitle string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() {
				<-sem
			}()

			if pageFetchStaggerDelay > 0 {
				staggerSlot := idx % workers
				if staggerSlot > 0 {
					time.Sleep(time.Duration(staggerSlot) * pageFetchStaggerDelay)
				}
			}

			page, err := fetchPageWithRetry(api, pageTitle, timeout)

			results[idx] = pageFetchResult{
				title: pageTitle,
				page:  page,
				err:   err,
			}
		}(i, title)
	}

	wg.Wait()
	return results
}

func fetchPageWithRetry(api *wiki.Client, title string, timeout time.Duration) (wiki.Page, error) {
	if api == nil {
		return wiki.Page{}, fmt.Errorf("wiki client is nil")
	}

	var lastErr error
	for attempt := 1; attempt <= pageFetchMaxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		page, err := api.FetchPage(ctx, title)
		cancel()

		if err == nil {
			return page, nil
		}

		lastErr = err
		if attempt == pageFetchMaxAttempts || !shouldRetryFetchError(err) {
			break
		}

		delay := pageFetchRetryBaseDelay * time.Duration(1<<(attempt-1))
		if delay > pageFetchRetryMaxDelay {
			delay = pageFetchRetryMaxDelay
		}
		time.Sleep(delay)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("fetch failed")
	}
	return wiki.Page{}, lastErr
}

func shouldRetryFetchError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if message == "" {
		return false
	}

	retryTokens := []string{
		"timeout",
		"tempor",
		"connection reset",
		"broken pipe",
		"tls handshake timeout",
		"http2: server sent goaway",
		"429",
		"502",
		"503",
		"504",
		"rate limit",
	}

	for _, token := range retryTokens {
		if strings.Contains(message, token) {
			return true
		}
	}

	return false
}

func estimateETA(start time.Time, processed, total int) string {
	if start.IsZero() || processed <= 0 || total <= 0 {
		return ""
	}

	remaining := total - processed
	if remaining <= 0 {
		return "0s"
	}

	elapsed := time.Since(start)
	if elapsed <= 0 {
		return ""
	}

	rate := float64(processed) / elapsed.Seconds()
	if rate <= 0 {
		return ""
	}

	seconds := int(float64(remaining)/rate + 0.5)
	if seconds < 1 {
		seconds = 1
	}

	return formatDurationCompact(time.Duration(seconds) * time.Second)
}

func formatDurationCompact(d time.Duration) string {
	if d < 0 {
		d = 0
	}

	h := int(d / time.Hour)
	d -= time.Duration(h) * time.Hour
	m := int(d / time.Minute)
	d -= time.Duration(m) * time.Minute
	s := int(d / time.Second)

	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
