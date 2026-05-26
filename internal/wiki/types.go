package wiki

// SearchResult describes one Arch Wiki search hit.
type SearchResult struct {
	Title   string
	Snippet string
	URL     string
}

// Page is a fully prepared article payload for the TUI.
type Page struct {
	Title      string
	URL        string
	Markdown   string
	CodeBlocks []string
}
