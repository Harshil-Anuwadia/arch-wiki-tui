package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.resizeComponents()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.loading {
			return m, cmd
		}
		return m, nil

	case bootstrapMsg:
		m.mode = modeSearch
		m.searchInput.Focus()
		m.searchInput.SetValue(msg.query)
		m.searchInput.CursorEnd()
		return m, m.startSearch(msg.query, true)

	case titleIndexRefreshedMsg:
		m.indexRefreshing = false
		if msg.err != nil {
			if m.titleIndex != nil && m.titleIndex.Count() > 0 {
				return m, m.setTransientStatus("Index refresh failed, using cached titles")
			}
			return m, m.setTransientStatus("Index refresh failed: " + msg.err.Error())
		}
		if msg.count > 0 {
			return m, m.setTransientStatus(fmt.Sprintf("Search index ready (%d titles)", msg.count))
		}
		return m, nil

	case topSearchLoadedMsg:
		if topic := strings.TrimSpace(msg.topic); topic != "" {
			m.homeTopSearch = topic
		}
		return m, nil

	case cacheClearedMsg:
		if msg.err != nil {
			return m, m.setTransientStatus("Clear cache failed: " + msg.err.Error())
		}

		m.searchResults = nil
		m.searchCursor = 0
		m.searchQuery = ""
		m.resetTitleIndexFromDisk()

		status := fmt.Sprintf("Cache cleared: %d pages", msg.removedPages)
		if msg.indexRemoved {
			status += " + title index"
		}
		return m, m.setTransientStatus(status)

	case offlineDownloadProgressMsg:
		completed := msg.cached + msg.failed
		m.downloadTotal = max(0, msg.requested)
		m.downloadProcessed = max(0, completed)
		m.downloadCached = max(0, msg.cached)
		m.downloadFailed = max(0, msg.failed)
		m.downloadSkipped = max(0, msg.skipped)
		progress := fmt.Sprintf("Downloading offline bundle: %d/%d pages (%d cached", completed, msg.requested, msg.cached)
		if msg.failed > 0 {
			progress += fmt.Sprintf(", %d failed", msg.failed)
		}
		progress += ")"
		if msg.skipped > 0 {
			progress += fmt.Sprintf(" + %d skipped", msg.skipped)
		}
		m.status = progress
		m.statusSticky = true

		if len(msg.remaining) == 0 {
			m.downloadLoading = false
			m.downloadStartedAt = time.Time{}
			m.syncLoadingState()

			status := fmt.Sprintf("Offline cache updated: %d/%d pages", msg.cached, msg.requested)
			if msg.failed > 0 {
				status += fmt.Sprintf(", %d failed", msg.failed)
			}
			if msg.skipped > 0 {
				status += fmt.Sprintf(", %d skipped", msg.skipped)
			}
			return m, m.setTransientStatus(status)
		}

		return m, m.downloadOfflinePageStepCmd(msg.remaining, msg.requested, msg.cached, msg.failed, msg.skipped)

	case offlineDownloadedMsg:
		m.downloadLoading = false
		m.downloadStartedAt = time.Time{}
		m.downloadTotal = max(0, msg.requested)
		m.downloadProcessed = max(0, msg.cached+msg.failed)
		m.downloadCached = max(0, msg.cached)
		m.downloadFailed = max(0, msg.failed)
		m.downloadSkipped = max(0, msg.skipped)
		m.syncLoadingState()
		if msg.err != nil {
			return m, m.setTransientStatus("Offline download failed: " + msg.err.Error())
		}

		status := fmt.Sprintf("Offline cache updated: %d/%d pages", msg.cached, msg.requested)
		if msg.failed > 0 {
			status += fmt.Sprintf(", %d failed", msg.failed)
		}
		if msg.skipped > 0 {
			status += fmt.Sprintf(", %d skipped", msg.skipped)
		}
		return m, m.setTransientStatus(status)

	case archiveSyncProgressMsg:
		if msg.err != nil {
			m.archiveSyncLoading = false
			m.archiveSyncStartedAt = time.Time{}
			m.syncLoadingState()
			return m, m.setTransientStatus("Full archive sync failed: " + msg.err.Error())
		}

		m.archiveSyncTotal = max(0, msg.total)
		m.archiveSyncProcessed = max(0, msg.processed)
		m.archiveSyncCached = max(0, msg.cached)
		m.archiveSyncFailed = max(0, msg.failed)

		if msg.total > 0 {
			m.status = fmt.Sprintf("Syncing full archive: %d/%d pages (%d cached", msg.processed, msg.total, msg.cached)
			if msg.failed > 0 {
				m.status += fmt.Sprintf(", %d failed", msg.failed)
			}
			m.status += ")"
		} else {
			m.status = "Preparing full archive sync..."
		}
		m.statusSticky = true

		if len(msg.remaining) == 0 {
			return m, m.archiveSyncDoneCmd(msg.total, msg.cached, msg.failed, nil)
		}

		return m, m.archiveSyncStepCmd(msg.remaining, msg.total, msg.cached, msg.failed, msg.index)

	case archiveSyncDoneMsg:
		m.archiveSyncLoading = false
		m.archiveSyncStartedAt = time.Time{}
		m.syncLoadingState()

		m.archiveSyncTotal = max(0, msg.total)
		m.archiveSyncProcessed = max(0, msg.cached+msg.failed)
		m.archiveSyncCached = max(0, msg.cached)
		m.archiveSyncFailed = max(0, msg.failed)

		if msg.err != nil {
			return m, m.setTransientStatus("Full archive sync failed: " + msg.err.Error())
		}

		status := fmt.Sprintf("Full archive sync complete: %d/%d pages", msg.cached, msg.total)
		if msg.failed > 0 {
			status += fmt.Sprintf(", %d failed", msg.failed)
		}

		m.offlineRefreshTitles()

		return m, m.setTransientStatus(status)

	case debounceSearchMsg:
		if msg.seq != m.pendingSearchSeq {
			return m, nil
		}
		if m.mode != modeSearch {
			return m, nil
		}
		if strings.TrimSpace(msg.query) == "" {
			m.searchResults = nil
			m.searchCursor = 0
			m.searchLoading = false
			m.syncLoadingState()
			return m, nil
		}
		return m, m.startSearch(msg.query, false)

	case searchResultMsg:
		if msg.requestID != m.activeSearchReq {
			return m, nil
		}

		m.searchLoading = false
		m.syncLoadingState()
		if msg.err != nil {
			if m.pageLoading {
				return m, nil
			}
			if len(m.searchResults) > 0 {
				return m, m.setTransientStatus("API search failed, showing indexed results")
			}
			return m, m.setTransientStatus("Search failed: " + msg.err.Error())
		}

		merged := mergeSearchResults(m.searchResults, msg.results, 60)
		m.searchResults = merged
		m.searchCursor = 0
		if m.pageLoading {
			return m, nil
		}
		if len(m.searchResults) == 0 {
			return m, m.setTransientStatus("No results for \"" + msg.query + "\"")
		}

		if msg.autoOpen {
			return m, m.openPage(m.searchResults[0].Title, true)
		}

		m.status = fmt.Sprintf("%d results for \"%s\"", len(m.searchResults), msg.query)
		m.statusSticky = true
		return m, nil

	case pageLoadedMsg:
		if msg.requestID != m.activePageReq {
			return m, nil
		}

		m.pageLoading = false
		m.syncLoadingState()
		if msg.err != nil {
			if m.navJumpRestoreActive {
				m.backStack = append([]string(nil), m.navJumpRestoreBackStack...)
				m.pendingBackTitle = m.navJumpRestorePending
				m.navJumpRestoreBackStack = nil
				m.navJumpRestorePending = ""
				m.navJumpRestoreActive = false
			}
			if m.pendingBackTitle != "" {
				m.backStack = append(m.backStack, m.pendingBackTitle)
				m.pendingBackTitle = ""
			}
			return m, m.setTransientStatus("Open page failed: " + msg.err.Error())
		}
		m.pendingBackTitle = ""
		m.navJumpRestoreBackStack = nil
		m.navJumpRestorePending = ""
		m.navJumpRestoreActive = false

		if msg.fromCache {
			m.offline = true
		} else if !m.cfg.ForceOffline {
			m.offline = false
		}

		m.currentPage = &msg.page
		m.setArticleContent(msg.page.Markdown)
		m.activeTab = tabWiki
		m.currentMarkdown = m.markdownForActiveTab()
		m.copyIndex = 0
		m.showTOC = false
		m.tocItems = nil
		m.tocCursor = 0
		m.tocExpanded = nil
		m.tocJumpHighlight = false
		m.tocJumpLine = 0
		m.tocJumpOffset = -1
		m.invalidateTOCOffsetCache()
		m.showNav = false
		m.mode = modeBrowse
		m.searchInput.Blur()
		m.ensureNavigationPath()
		m.resizeComponents()

		if m.store != nil {
			_ = m.store.AddHistory(msg.page.Title, msg.page.URL)
			_ = m.store.WritePageCache(msg.page.Title, msg.page.Markdown)
		}

		if msg.fromCache {
			m.status = "Opened from local cache: " + msg.page.Title
		} else {
			m.status = "Opened: " + msg.page.Title
		}
		m.statusSticky = true
		return m, nil

	case browserOpenedMsg:
		if msg.err != nil {
			return m, m.setTransientStatus("Browser open failed: " + msg.err.Error())
		}
		return m, m.setTransientStatus("Opened page in browser")

	case copiedCodeMsg:
		if msg.err != nil {
			return m, m.setTransientStatus("Copy failed (install wl-copy or xclip): " + msg.err.Error())
		}
		return m, m.setTransientStatus(fmt.Sprintf("Copied code block %d/%d", msg.index, msg.total))

	case clearStatusMsg:
		if !m.statusSticky {
			m.status = "Press / to search Arch Wiki"
			m.statusSticky = true
		}
		return m, nil

	case tea.MouseMsg:
		m.clearTOCJumpHighlight()
		if m.mode == modeBrowse {
			if m.showOfflineLibrary {
				return m, nil
			}
			if m.showNav {
				return m, nil
			}
			if m.showTOC {
				return m, nil
			}
			if !m.showHelp {
				if cmd := m.handleMouseClick(msg); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}

			beforeOffset := m.viewport.YOffset
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			if m.viewport.YOffset != beforeOffset {
				m.syncTOCCursorToViewport()
			}
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "ctrl+q" {
			return m, tea.Quit
		}
		m.clearTOCJumpHighlight()

		if m.confirmDialog != nil {
			cmd := m.updateConfirmDialog(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		if m.showOfflineLibrary {
			cmd := m.updateOfflineLibraryMode(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		if m.showNav {
			cmd := m.updateNavMode(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		if m.showTOC {
			cmd := m.updateTOCMode(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		if msg.String() == "q" && m.mode != modeSearch {
			return m, tea.Quit
		}

		if m.showHelp {
			switch msg.String() {
			case "?", "esc", "enter":
				m.showHelp = false
			}
			return m, nil
		}

		if m.mode == modeSearch {
			cmd := m.updateSearchMode(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		cmd := m.updateBrowseMode(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	if m.mode == modeBrowse {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the terminal UI.
