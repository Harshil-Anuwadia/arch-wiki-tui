package app

import (
	"regexp"
	"strings"
	"time"

	"archwiki-tui/internal/render"
	"archwiki-tui/internal/storage"
	"archwiki-tui/internal/ui"
	"archwiki-tui/internal/wiki"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type uiMode int

const (
	modeBrowse uiMode = iota
	modeSearch
)

type contentTab int

const (
	tabWiki contentTab = iota
	tabRelated
	tabLinks
)

type articleLink = render.ArticleLink
type wikiRenderLink = render.WikiRenderLink

var plainURLRE = regexp.MustCompile(`https?://[^\s<>()]+`)
var tocHeadingRE = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
var tocMarkdownLinkRE = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
var manPageTitleRE = regexp.MustCompile(`\([0-9][A-Za-z]*\)$`)

var homeDailySearchTopics = []string{
	"pacman",
	"systemd",
	"networkmanager",
	"pipewire",
	"nvidia",
	"grub",
	"mkinitcpio",
	"btrfs",
	"wayland",
	"bluetooth",
	"docker",
	"ssh",
	"firewalld",
	"zram",
	"virt-manager",
	"sway",
	"hyprland",
	"iwctl",
	"reflector",
	"ufw",
}

const (
	maxReadableArticleWidth       = 92
	searchMaxVisibleResults       = 10
	offlineDownloadBatchSize      = 8
	offlineDownloadWorkers        = 8
	archiveSyncBatchSize          = 24
	archiveSyncWorkers            = 12
	archiveSyncIndexFlushInterval = 240
	pageFetchMaxAttempts          = 4
	pageFetchRetryBaseDelay       = 200 * time.Millisecond
	pageFetchRetryMaxDelay        = 2 * time.Second
	pageFetchStaggerDelay         = 12 * time.Millisecond
)

type searchResultMsg struct {
	requestID int
	query     string
	autoOpen  bool
	results   []wiki.SearchResult
	err       error
}

type pageLoadedMsg struct {
	requestID int
	fromCache bool
	page      wiki.Page
	err       error
}

type debounceSearchMsg struct {
	seq   int
	query string
}

type bootstrapMsg struct {
	query string
}

type browserOpenedMsg struct {
	err error
}

type copiedCodeMsg struct {
	index int
	total int
	err   error
}

type clearStatusMsg struct{}

type titleIndexRefreshedMsg struct {
	count int
	err   error
}

type topSearchLoadedMsg struct {
	topic string
	err   error
}

type cacheClearedMsg struct {
	removedPages int
	indexRemoved bool
	err          error
}

type offlineDownloadedMsg struct {
	requested int
	cached    int
	failed    int
	skipped   int
	err       error
}

type offlineDownloadProgressMsg struct {
	requested int
	cached    int
	failed    int
	skipped   int
	remaining []string
}

type archiveSyncProgressMsg struct {
	phase     string
	processed int
	total     int
	cached    int
	failed    int
	remaining []string
	index     storage.ArchiveIndex
	err       error
}

type archiveSyncDoneMsg struct {
	total  int
	cached int
	failed int
	err    error
}

type pageFetchResult struct {
	title string
	page  wiki.Page
	err   error
}

type confirmAction int

const (
	confirmActionClearCache confirmAction = iota + 1
	confirmActionDownloadOffline
	confirmActionDownloadFullArchive
)

type confirmDialogState struct {
	action          confirmAction
	title           string
	message         string
	details         []string
	yesLabel        string
	noLabel         string
	selectedYes     bool
	downloadTitles  []string
	downloadSkipped int
}

type tocItem struct {
	Title        string
	Level        int
	MarkdownLine int
}

type tabLinkItem struct {
	Title     string
	URL       string
	Group     string
	OpenTitle string
	OpenInTUI bool
}

type navNode struct {
	Title    string
	Parent   int
	Children []int
}

const (
	linkGroupArchWiki       = "ArchWiki"
	linkGroupManPages       = "Man Pages"
	linkGroupAnchors        = "Anchors"
	linkGroupExternal       = "External"
	tabLinkCollapsedVisible = 10
)

var tabLinkGroupOrder = []string{linkGroupArchWiki, linkGroupManPages, linkGroupAnchors, linkGroupExternal}

// Model is the Bubble Tea application state.
type Model struct {
	cfg    Config
	styles ui.Styles

	api          *wiki.Client
	store        *storage.Store
	renderEngine *render.Engine

	titleIndex      *wiki.TitleIndex
	indexRefreshing bool

	width  int
	height int
	ready  bool

	mode uiMode

	spinner            spinner.Model
	searchInput        textinput.Model
	offlineFilterInput textinput.Model
	viewport           viewport.Model

	loading           bool
	searchLoading     bool
	pageLoading       bool
	downloadLoading   bool
	downloadStartedAt time.Time
	downloadTotal     int
	downloadProcessed int
	downloadCached    int
	downloadFailed    int
	downloadSkipped   int
	offline           bool
	status            string
	statusSticky      bool
	homeTopSearch     string

	searchResults []wiki.SearchResult
	searchCursor  int
	searchQuery   string

	pendingSearchSeq int
	activeSearchReq  int
	activePageReq    int

	bootstrapQuery string

	currentPage             *wiki.Page
	currentMarkdown         string
	wikiMarkdown            string
	relatedMarkdown         string
	linksMarkdown           string
	relatedLinks            []articleLink
	allLinks                []articleLink
	relatedTabItems         []tabLinkItem
	linksTabItems           []tabLinkItem
	relatedCursor           int
	linksCursor             int
	relatedExpandedGroups   map[string]bool
	linksExpandedGroups     map[string]bool
	activeTab               contentTab
	contentLines            int
	backStack               []string
	pendingBackTitle        string
	copyIndex               int
	showHelp                bool
	showOfflineLibrary      bool
	showNav                 bool
	showTOC                 bool
	archiveSyncLoading      bool
	archiveSyncStartedAt    time.Time
	archiveSyncTotal        int
	archiveSyncProcessed    int
	archiveSyncCached       int
	archiveSyncFailed       int
	offlineTitles           []string
	offlineVisibleTitles    []string
	offlineCursor           int
	offlineFilterActive     bool
	tocItems                []tocItem
	tocCursor               int
	tocExpanded             map[int]bool
	tocJumpHighlight        bool
	tocJumpLine             int
	tocJumpOffset           int
	tocOffsetCache          map[int]int
	tocOffsetWrap           int
	renderedWikiContent     string
	navNodes                []navNode
	navRoots                []int
	navCurrent              int
	navCursor               int
	navExpanded             map[int]bool
	navJumpRestoreActive    bool
	navJumpRestoreBackStack []string
	navJumpRestorePending   string
	confirmDialog           *confirmDialogState

	renderer     *glamour.TermRenderer
	rendererWrap int
}

// NewModel builds the root application model.
func NewModel(cfg Config) *Model {
	styles := ui.NewStyles()

	searchInput := textinput.New()
	searchInput.Prompt = "[/] "
	searchInput.Placeholder = "search Arch Wiki"
	searchInput.CharLimit = 120
	searchInput.Width = 50
	searchInput.PromptStyle = styles.InputPrompt
	searchInput.TextStyle = styles.InputText
	searchInput.PlaceholderStyle = styles.InputPlaceholder
	searchInput.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("214")).Bold(true)
	searchInput.Cursor.TextStyle = styles.InputText

	offlineFilterInput := textinput.New()
	offlineFilterInput.Prompt = "[/] "
	offlineFilterInput.Placeholder = "filter downloaded pages"
	offlineFilterInput.CharLimit = 100
	offlineFilterInput.Width = 50
	offlineFilterInput.PromptStyle = styles.InputPrompt
	offlineFilterInput.TextStyle = styles.InputText
	offlineFilterInput.PlaceholderStyle = styles.InputPlaceholder
	offlineFilterInput.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("214")).Bold(true)
	offlineFilterInput.Cursor.TextStyle = styles.InputText

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	store, storeErr := storage.NewStore("archwiki-tui")
	indexPath := resolveIndexCachePath(store)
	titleIndex, indexErr := wiki.NewTitleIndex(indexPath)

	m := &Model{
		cfg:                cfg,
		styles:             styles,
		api:                wiki.NewClient(12 * time.Second),
		store:              store,
		renderEngine:       render.NewEngine(),
		titleIndex:         titleIndex,
		mode:               modeBrowse,
		spinner:            spin,
		searchInput:        searchInput,
		offlineFilterInput: offlineFilterInput,
		viewport:           viewport.New(20, 10),
		offline:            cfg.ForceOffline,
		status:             "Press / to search Arch Wiki",
		statusSticky:       true,
		homeTopSearch:      topSearchOfDay(time.Now().UTC()),
		navCurrent:         -1,
		tocJumpOffset:      -1,
	}

	if storeErr != nil {
		m.status = "State persistence disabled (config dir unavailable)"
	}
	if indexErr != nil {
		m.status = "Search index unavailable, using API-only search"
	}

	if q := strings.TrimSpace(cfg.InitialQuery); q != "" {
		m.bootstrapQuery = q
	}
	if m.bootstrapQuery == "" {
		if q := strings.TrimSpace(cfg.ContextCommand); q != "" {
			m.bootstrapQuery = q
		}
	}

	return m
}

// Init starts spinner ticks and optional bootstrap search.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if !m.cfg.ForceOffline {
		cmds = append(cmds, m.fetchTopSearchCmd())
	}
	if m.titleIndex != nil && m.titleIndex.NeedsRefresh(wiki.DefaultTitleIndexTTL) {
		m.indexRefreshing = true
		if m.titleIndex.Count() == 0 {
			m.status = "Building search index... first run may take a moment"
			m.statusSticky = true
		}
		cmds = append(cmds, m.refreshTitleIndexCmd())
	}
	if m.bootstrapQuery != "" {
		cmds = append(cmds, func() tea.Msg {
			return bootstrapMsg{query: m.bootstrapQuery}
		})
	}
	return tea.Batch(cmds...)
}

// Update handles all state transitions.
