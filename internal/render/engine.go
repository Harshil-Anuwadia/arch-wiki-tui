package render

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	xhtml "golang.org/x/net/html"
)

var markdownLinkRE = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
var markdownAutolinkRE = regexp.MustCompile(`<https?://[^>\s]+>`)
var proseInlineURLRE = regexp.MustCompile(`(^|[ \t])https?://[^\s\])>]+?([,.;:!?]?)([ \t]|$)`)
var urlOnlyLineRE = regexp.MustCompile(`^\s*https?://\S+\s*$`)
var spaceBeforePunctuationRE = regexp.MustCompile(`\s+([,.;:!?])`)
var calloutURLRE = regexp.MustCompile(`https?://[^\s)]+`)
var ansiCSIRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
var ansiOSC8RE = regexp.MustCompile(`\x1b]8;;[^\x1b]*\x1b\\`)

var markdownAutolinkStripper = regexp.MustCompile(`<https?://[^>\s]+>`)
var proseURLStripper = regexp.MustCompile(`\s+https?://[^\s\])>]+`)
var proseURLOnlyLine = regexp.MustCompile(`^\s*https?://\S+\s*$`)

// ArticleLink is one extracted markdown link entry.
type ArticleLink struct {
	Text string
	URL  string
}

// LinkType describes semantic routing for rendered links.
type LinkType string

const (
	LinkTypeInternal  LinkType = "internal"
	LinkTypeSection   LinkType = "section"
	LinkTypeReference LinkType = "reference"
	LinkTypeExternal  LinkType = "external"
)

// LinkState tracks UI presentation state for rendered links.
type LinkState string

const (
	LinkStateNormal   LinkState = "normal"
	LinkStateFocused  LinkState = "focused"
	LinkStateVisited  LinkState = "visited"
	LinkStateInactive LinkState = "inactive"
)

// WikiRenderLink maps a temporary token to a clickable terminal hyperlink.
type WikiRenderLink struct {
	ID    int
	Token string
	Label string
	URL   string
	Type  LinkType
	State LinkState
}

// Document is a normalized article payload used by the TUI tabs.
type Document struct {
	WikiMarkdown    string
	RelatedMarkdown string
	LinksMarkdown   string
	RelatedLinks    []ArticleLink
	AllLinks        []ArticleLink
}

// Engine owns synchronous transformation from raw wiki HTML/markdown to readable docs.
type Engine struct{}

// NewEngine creates a new rendering engine.
func NewEngine() *Engine {
	return &Engine{}
}

// BuildMarkdownFromHTML parses wiki HTML with goquery and renders structured markdown-like content.
func (e *Engine) BuildMarkdownFromHTML(rawHTML string) (string, []string, error) {
	rawHTML = strings.TrimSpace(rawHTML)
	if rawHTML == "" {
		return "", nil, fmt.Errorf("raw html is empty")
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapHTMLFragment(rawHTML)))
	if err != nil {
		return "", nil, fmt.Errorf("parse html: %w", err)
	}

	root := selectContentRoot(doc)
	if root == nil || root.Length() == 0 {
		return "", nil, fmt.Errorf("html root not found")
	}

	sanitizeContentRoot(root)

	codeBlocks := extractCodeBlocksFromSelection(root)
	md := renderRootBlocks(root)
	md = normalizeMarkdown(md)
	md = simplifyMarkdownForReading(md)
	if md == "" {
		return "", nil, fmt.Errorf("converted markdown is empty")
	}

	return md, dedupeStrings(codeBlocks), nil
}

func wrapHTMLFragment(raw string) string {
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "<html") {
		return raw
	}
	return "<!doctype html><html><head><meta charset=\"utf-8\"></head><body>" + raw + "</body></html>"
}

func selectContentRoot(doc *goquery.Document) *goquery.Selection {
	selectors := []string{
		"#mw-content-text .mw-parser-output",
		".mw-parser-output",
		"#mw-content-text",
		"article",
		"main",
		"body",
	}

	for _, sel := range selectors {
		found := doc.Find(sel).First()
		if found.Length() > 0 {
			return found
		}
	}

	return doc.Selection
}

func sanitizeContentRoot(root *goquery.Selection) {
	root.Find("script,style,noscript,sup.reference,.mw-editsection,.noprint,#toc,.toc,.mw-jump,.mw-empty-elt,.navbox,.metadata,.printfooter,.catlinks").Remove()

	root.Find("a").Each(func(_ int, sel *goquery.Selection) {
		href, ok := sel.Attr("href")
		if !ok {
			return
		}
		href = normalizeWikiHref(href)
		if href == "" {
			sel.RemoveAttr("href")
			return
		}
		sel.SetAttr("href", href)
	})
}

func normalizeWikiHref(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "#") {
		return href
	}

	switch {
	case strings.HasPrefix(href, "https://"), strings.HasPrefix(href, "http://"):
		return href
	case strings.HasPrefix(href, "//"):
		return "https:" + href
	case strings.HasPrefix(href, "/title/"), strings.HasPrefix(href, "/index.php"):
		return "https://wiki.archlinux.org" + href
	case strings.HasPrefix(href, "index.php?"):
		return "https://wiki.archlinux.org/" + href
	case strings.HasPrefix(href, "/"):
		return "https://wiki.archlinux.org" + href
	default:
		return href
	}
}

func extractCodeBlocks(raw string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapHTMLFragment(raw)))
	if err != nil {
		return nil
	}
	return dedupeStrings(extractCodeBlocksFromSelection(doc.Selection))
}

func extractCodeBlocksFromSelection(root *goquery.Selection) []string {
	out := make([]string, 0)
	root.Find("pre").Each(func(_ int, sel *goquery.Selection) {
		text := strings.TrimSpace(sel.Text())
		if text != "" {
			out = append(out, text)
		}
	})
	return out
}

func dedupeStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func renderRootBlocks(root *goquery.Selection) string {
	blocks := make([]string, 0, 64)
	children := root.Children()

	if children.Length() == 0 {
		text := strings.TrimSpace(renderInlineSelection(root))
		if text != "" {
			blocks = append(blocks, text)
		}
		return strings.Join(blocks, "\n\n")
	}

	children.Each(func(_ int, child *goquery.Selection) {
		collectBlocks(child, 0, &blocks)
	})

	return strings.TrimSpace(strings.Join(blocks, "\n\n"))
}

func collectBlocks(sel *goquery.Selection, depth int, blocks *[]string) {
	if sel == nil || sel.Length() == 0 {
		return
	}

	tag := strings.ToLower(goquery.NodeName(sel))
	if tag == "" {
		return
	}

	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level, _ := strconv.Atoi(tag[1:])
		if level < 1 || level > 6 {
			level = 2
		}
		text := strings.TrimSpace(renderInlineSelection(sel))
		if text != "" {
			*blocks = append(*blocks, strings.Repeat("#", level)+" "+text)
		}

	case "p":
		text := strings.TrimSpace(renderInlineSelection(sel))
		if text != "" {
			*blocks = append(*blocks, text)
		}

	case "pre":
		code := strings.TrimSpace(sel.Text())
		if code != "" {
			*blocks = append(*blocks, "```text\n"+strings.TrimRight(code, "\n")+"\n```")
		}

	case "ul":
		if list := renderList(sel, false, depth); list != "" {
			*blocks = append(*blocks, list)
		}

	case "ol":
		if list := renderList(sel, true, depth); list != "" {
			*blocks = append(*blocks, list)
		}

	case "table":
		if isCalloutBox(sel) {
			if callout := renderCalloutBox(sel); callout != "" {
				*blocks = append(*blocks, callout)
			}
			return
		}
		if table := renderTable(sel); table != "" {
			*blocks = append(*blocks, table)
		}

	case "blockquote":
		text := strings.TrimSpace(renderInlineSelection(sel))
		if text != "" {
			lines := strings.Split(text, "\n")
			for i, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					lines[i] = "> " + line
				}
			}
			quote := strings.TrimSpace(strings.Join(lines, "\n"))
			if quote != "" {
				*blocks = append(*blocks, quote)
			}
		}

	case "dl":
		if dl := renderDefinitionList(sel); dl != "" {
			*blocks = append(*blocks, dl)
		}

	case "hr":
		*blocks = append(*blocks, "---")

	case "div", "section", "article", "main":
		if isCalloutBox(sel) {
			if callout := renderCalloutBox(sel); callout != "" {
				*blocks = append(*blocks, callout)
			}
			return
		}
		if sel.Children().Length() > 0 {
			sel.Children().Each(func(_ int, child *goquery.Selection) {
				collectBlocks(child, depth, blocks)
			})
			return
		}
		text := strings.TrimSpace(renderInlineSelection(sel))
		if text != "" {
			*blocks = append(*blocks, text)
		}

	default:
		if sel.Children().Length() > 0 {
			sel.Children().Each(func(_ int, child *goquery.Selection) {
				collectBlocks(child, depth, blocks)
			})
			return
		}
		text := strings.TrimSpace(renderInlineSelection(sel))
		if text != "" {
			*blocks = append(*blocks, text)
		}
	}
}

func renderList(sel *goquery.Selection, ordered bool, depth int) string {
	lines := make([]string, 0, 16)
	sel.ChildrenFiltered("li").Each(func(i int, li *goquery.Selection) {
		clone := li.Clone()
		clone.ChildrenFiltered("ul,ol").Remove()
		itemText := strings.TrimSpace(renderInlineSelection(clone))

		prefix := "- "
		if ordered {
			prefix = fmt.Sprintf("%d. ", i+1)
		} else if depth == 0 {
			prefix = "• "
		} else {
			prefix = "◦ "
		}
		indent := strings.Repeat("    ", depth)
		if itemText != "" {
			lines = append(lines, indent+prefix+itemText)
		}

		li.ChildrenFiltered("ul,ol").Each(func(_ int, nested *goquery.Selection) {
			nestedOrdered := strings.EqualFold(goquery.NodeName(nested), "ol")
			nestedRendered := renderList(nested, nestedOrdered, depth+1)
			if strings.TrimSpace(nestedRendered) != "" {
				lines = append(lines, nestedRendered)
			}
		})
	})

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderDefinitionList(sel *goquery.Selection) string {
	lines := make([]string, 0, 16)
	currentTerm := ""
	sel.Children().Each(func(_ int, child *goquery.Selection) {
		tag := strings.ToLower(goquery.NodeName(child))
		switch tag {
		case "dt":
			currentTerm = strings.TrimSpace(renderInlineSelection(child))
		case "dd":
			desc := strings.TrimSpace(renderInlineSelection(child))
			if currentTerm != "" && desc != "" {
				lines = append(lines, "- **"+currentTerm+"**: "+desc)
			} else if desc != "" {
				lines = append(lines, "- "+desc)
			}
		}
	})
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func isCalloutBox(sel *goquery.Selection) bool {
	if sel == nil || sel.Length() == 0 {
		return false
	}

	classAttr, _ := sel.Attr("class")
	classAttr = strings.ToLower(classAttr)
	hints := []string{
		"archwiki-template-box",
		"ambox",
		"warning",
		"caution",
		"important",
		"note",
		"tip",
		"notice",
		"messagebox",
	}

	for _, hint := range hints {
		if strings.Contains(classAttr, hint) {
			return true
		}
	}

	if sel.Find(".archwiki-template-box, .ambox").Length() > 0 {
		return true
	}

	return false
}

func renderCalloutBox(sel *goquery.Selection) string {
	text := strings.TrimSpace(renderInlineSelection(sel))
	if text == "" {
		return ""
	}
	text = normalizeCalloutText(text)
	if text == "" {
		return ""
	}

	kind := detectCalloutKind(sel, text)
	wrapped := wrapCalloutText(text, 60)
	if len(wrapped) == 0 {
		return ""
	}

	innerWidth := runeWidth(kind)
	for _, line := range wrapped {
		if runeWidth(line) > innerWidth {
			innerWidth = runeWidth(line)
		}
	}
	if innerWidth < 24 {
		innerWidth = 24
	}
	if innerWidth < runeWidth(kind)+2 {
		innerWidth = runeWidth(kind) + 2
	}

	topSuffixLen := innerWidth - runeWidth(kind) - 1
	if topSuffixLen < 1 {
		topSuffixLen = 1
	}
	top := "┌─ " + kind + " " + strings.Repeat("─", topSuffixLen) + "┐"

	boxLines := make([]string, 0, len(wrapped)+2)
	boxLines = append(boxLines, top)
	for _, line := range wrapped {
		boxLines = append(boxLines, "│ "+padRight(line, innerWidth)+" │")
	}
	boxLines = append(boxLines, "└"+strings.Repeat("─", innerWidth+2)+"┘")

	return "```text\n" + strings.Join(boxLines, "\n") + "\n```"
}

func detectCalloutKind(sel *goquery.Selection, text string) string {
	joined := strings.ToLower(text)
	if classAttr, ok := sel.Attr("class"); ok {
		joined = strings.ToLower(classAttr) + " " + joined
	}

	switch {
	case containsAny(joined, "warning", "caution", "danger", "alert"):
		return "WARNING"
	case containsAny(joined, "important", "critical"):
		return "IMPORTANT"
	case containsAny(joined, "tip", "hint"):
		return "TIP"
	case containsAny(joined, "note"):
		return "NOTE"
	default:
		return "INFO"
	}
}

func containsAny(s string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(s, part) {
			return true
		}
	}
	return false
}

func normalizeCalloutText(text string) string {
	text = strings.ReplaceAll(text, "\u00a0", " ")

	text = markdownLinkRE.ReplaceAllStringFunc(text, func(match string) string {
		parts := markdownLinkRE.FindStringSubmatch(match)
		if len(parts) < 2 {
			return ""
		}
		return strings.TrimSpace(parts[1])
	})

	text = markdownAutolinkRE.ReplaceAllStringFunc(text, func(match string) string {
		raw := strings.Trim(strings.TrimSpace(match), "<>")
		if host := hostLabel(raw); host != "" {
			return host
		}
		return ""
	})

	text = calloutURLRE.ReplaceAllStringFunc(text, func(raw string) string {
		if host := hostLabel(raw); host != "" {
			return host
		}
		return ""
	})

	text = collapseWhitespace(text)
	return strings.TrimSpace(text)
}

func wrapCalloutText(text string, width int) []string {
	if width < 24 {
		width = 24
	}

	words := strings.Fields(collapseWhitespace(text))
	if len(words) == 0 {
		return nil
	}

	tokens := make([]string, 0, len(words))
	for _, word := range words {
		tokens = append(tokens, splitWordByWidth(word, width)...)
	}
	if len(tokens) == 0 {
		return nil
	}

	lines := make([]string, 0, 6)
	current := tokens[0]
	for _, token := range tokens[1:] {
		if runeWidth(current)+1+runeWidth(token) <= width {
			current += " " + token
			continue
		}
		lines = append(lines, current)
		current = token
	}

	if strings.TrimSpace(current) != "" {
		lines = append(lines, current)
	}

	return lines
}

func splitWordByWidth(word string, width int) []string {
	if width < 1 {
		return nil
	}
	if runeWidth(word) <= width {
		return []string{word}
	}

	r := []rune(word)
	out := make([]string, 0, len(r)/width+1)
	for len(r) > width {
		out = append(out, string(r[:width]))
		r = r[width:]
	}
	if len(r) > 0 {
		out = append(out, string(r))
	}
	return out
}

func padRight(s string, width int) string {
	if runeWidth(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-runeWidth(s))
}

func runeWidth(s string) int {
	return len([]rune(s))
}

const (
	tableMaxColumns  = 8
	tableMaxWidth    = 84
	tableMaxColWidth = 22
	tableMinColWidth = 6
)

func renderTable(sel *goquery.Selection) string {
	rows := make([][]string, 0, 16)
	rowHasHeader := make([]bool, 0, 16)

	rowSel := sel.Find("> thead > tr, > tbody > tr, > tfoot > tr, > tr")
	if rowSel.Length() == 0 {
		rowSel = sel.Find("tr")
	}

	rowSel.Each(func(_ int, tr *goquery.Selection) {
		cells := tr.ChildrenFiltered("th,td")
		if cells.Length() == 0 {
			return
		}

		hasTH := false
		row := make([]string, 0, 8)
		cells.Each(func(_ int, cell *goquery.Selection) {
			if strings.EqualFold(goquery.NodeName(cell), "th") {
				hasTH = true
			}
			text := normalizeTableCellText(renderInlineSelection(cell))
			row = append(row, text)
		})

		if len(row) == 0 {
			return
		}
		if len(row) > tableMaxColumns {
			row = row[:tableMaxColumns]
		}

		rows = append(rows, row)
		rowHasHeader = append(rowHasHeader, hasTH)
	})

	if len(rows) == 0 {
		return ""
	}

	colCount := 0
	for _, row := range rows {
		if len(row) > colCount {
			colCount = len(row)
		}
	}
	if colCount == 0 {
		return ""
	}

	for i := range rows {
		if len(rows[i]) < colCount {
			pad := make([]string, colCount-len(rows[i]))
			rows[i] = append(rows[i], pad...)
		}
	}

	headerIndex := -1
	for i, hasHeader := range rowHasHeader {
		if hasHeader {
			headerIndex = i
			break
		}
	}

	header := make([]string, colCount)
	data := make([][]string, 0, len(rows))
	if headerIndex >= 0 {
		copy(header, rows[headerIndex])
		for i, row := range rows {
			if i == headerIndex {
				continue
			}
			data = append(data, row)
		}
	} else {
		for i := 0; i < colCount; i++ {
			header[i] = fmt.Sprintf("Column %d", i+1)
		}
		data = append(data, rows...)
	}

	colWidths := make([]int, colCount)
	for i := range colWidths {
		colWidths[i] = tableMinColWidth
		headerWidth := runeWidth(header[i])
		if headerWidth > colWidths[i] {
			colWidths[i] = headerWidth
		}
		if colWidths[i] > tableMaxColWidth {
			colWidths[i] = tableMaxColWidth
		}
	}

	for _, row := range data {
		for i := 0; i < colCount; i++ {
			cellWidth := runeWidth(row[i])
			if cellWidth > colWidths[i] {
				if cellWidth > tableMaxColWidth {
					colWidths[i] = tableMaxColWidth
				} else {
					colWidths[i] = cellWidth
				}
			}
		}
	}

	for tableRenderedWidth(colWidths) > tableMaxWidth {
		widest := widestReducibleColumn(colWidths, tableMinColWidth)
		if widest < 0 {
			break
		}
		colWidths[widest]--
	}

	separator := renderTableSeparator(colWidths)
	lines := make([]string, 0, len(data)*2+6)
	lines = append(lines, separator)
	lines = append(lines, renderTableRow(header, colWidths))
	lines = append(lines, separator)
	if len(data) == 0 {
		lines = append(lines, separator)
	} else {
		for _, row := range data {
			lines = append(lines, renderTableRow(row, colWidths))
			lines = append(lines, separator)
		}
	}

	return "```text\n" + strings.Join(lines, "\n") + "\n```"
}

func normalizeTableCellText(text string) string {
	text = strings.ReplaceAll(strings.TrimSpace(text), "\n", " ")
	text = markdownLinkRE.ReplaceAllString(text, "$1")
	text = markdownAutolinkRE.ReplaceAllStringFunc(text, func(match string) string {
		raw := strings.Trim(strings.TrimSpace(match), "<>")
		if host := hostLabel(raw); host != "" {
			return host
		}
		return raw
	})
	text = calloutURLRE.ReplaceAllStringFunc(text, func(raw string) string {
		if host := hostLabel(raw); host != "" {
			return host
		}
		return raw
	})
	text = collapseWhitespace(text)
	return text
}

func tableRenderedWidth(colWidths []int) int {
	if len(colWidths) == 0 {
		return 0
	}

	total := 1
	for _, width := range colWidths {
		total += width + 3
	}
	return total
}

func widestReducibleColumn(colWidths []int, minWidth int) int {
	idx := -1
	best := minWidth
	for i, width := range colWidths {
		if width <= minWidth {
			continue
		}
		if idx < 0 || width > best {
			idx = i
			best = width
		}
	}
	return idx
}

func renderTableSeparator(colWidths []int) string {
	var b strings.Builder
	b.WriteByte('+')
	for _, width := range colWidths {
		b.WriteString(strings.Repeat("-", width+2))
		b.WriteByte('+')
	}
	return b.String()
}

func renderTableRow(cols []string, colWidths []int) string {
	var b strings.Builder
	b.WriteByte('|')
	for i, width := range colWidths {
		cell := ""
		if i < len(cols) {
			cell = truncateTableCell(cols[i], width)
		}
		b.WriteByte(' ')
		b.WriteString(padRight(cell, width))
		b.WriteString(" |")
	}
	return b.String()
}

func truncateTableCell(s string, max int) string {
	s = collapseWhitespace(strings.TrimSpace(s))
	if max <= 0 {
		return ""
	}
	if runeWidth(s) <= max {
		return s
	}
	if max <= 3 {
		return strings.Repeat(".", max)
	}

	r := []rune(s)
	return string(r[:max-3]) + "..."
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	if max == 1 {
		return "..."
	}
	return string(r[:max-1]) + "..."
}

func renderInlineSelection(sel *goquery.Selection) string {
	if sel == nil || len(sel.Nodes) == 0 {
		return ""
	}

	var b strings.Builder
	for _, node := range sel.Nodes {
		b.WriteString(renderInlineNode(node))
	}

	text := strings.ReplaceAll(b.String(), "\u00a0", " ")
	text = normalizeInlineSpacing(text)
	text = spaceBeforePunctuationRE.ReplaceAllString(text, "$1")
	return strings.TrimSpace(text)
}

func renderInlineNode(node *xhtml.Node) string {
	if node == nil {
		return ""
	}

	switch node.Type {
	case xhtml.TextNode:
		return html.UnescapeString(node.Data)
	case xhtml.ElementNode:
		tag := strings.ToLower(node.Data)
		switch tag {
		case "br":
			return "\n"
		case "a":
			label := strings.TrimSpace(renderInlineChildren(node))
			href := ""
			for _, attr := range node.Attr {
				if strings.EqualFold(attr.Key, "href") {
					href = normalizeWikiHref(attr.Val)
					break
				}
			}
			if href == "" {
				return label
			}
			if label == "" {
				label = href
			}
			return "[" + collapseWhitespace(label) + "](" + href + ")"
		case "code":
			code := strings.TrimSpace(renderInlineChildren(node))
			if code == "" {
				return ""
			}
			return "`" + collapseWhitespace(code) + "`"
		case "sup", "sub":
			text := strings.TrimSpace(renderInlineChildren(node))
			if text == "" {
				return ""
			}
			return " " + text
		default:
			return renderInlineChildren(node)
		}
	default:
		return ""
	}
}

func renderInlineChildren(node *xhtml.Node) string {
	if node == nil {
		return ""
	}
	var b strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		b.WriteString(renderInlineNode(child))
	}
	return b.String()
}

func normalizeInlineSpacing(text string) string {
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.Join(strings.Fields(lines[i]), " ")
	}

	return strings.Join(lines, "\n")
}

// BuildDocument splits markdown into wiki/related/links tabs and applies heading numbering.
func (e *Engine) BuildDocument(rawMarkdown string) Document {
	main, related := splitRelatedArticles(rawMarkdown)

	main = strings.TrimSpace(main)
	if main == "" {
		main = strings.TrimSpace(rawMarkdown)
	}

	all := extractMarkdownLinks(main)

	relatedLinks := dedupeArticleLinks(filterSemanticRelatedLinks(related))
	allLinks := dedupeArticleLinks(all)

	return Document{
		WikiMarkdown:    addHierarchicalHeadingNumbers(main),
		RelatedLinks:    relatedLinks,
		AllLinks:        allLinks,
		RelatedMarkdown: addHierarchicalHeadingNumbers(linksViewMarkdown("Related Articles", "No related articles found.", relatedLinks)),
		LinksMarkdown:   addHierarchicalHeadingNumbers(linksViewMarkdown("Links in This Article", "No links found.", allLinks)),
	}
}

// PrepareWikiMarkdownForDisplay strips noisy inline URLs and creates terminal-hyperlink placeholders.
func (e *Engine) PrepareWikiMarkdownForDisplay(markdown string) (string, []WikiRenderLink) {
	if strings.TrimSpace(markdown) == "" {
		return markdown, nil
	}

	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	out := make([]string, 0, len(lines))
	links := make([]WikiRenderLink, 0)
	inCodeFence := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isFenceDelimiter(trimmed) {
			inCodeFence = !inCodeFence
			out = append(out, line)
			continue
		}

		if inCodeFence {
			out = append(out, line)
			continue
		}

		line = strings.ReplaceAll(line, "\u00a0", " ")

		line = markdownLinkRE.ReplaceAllStringFunc(line, func(match string) string {
			parts := markdownLinkRE.FindStringSubmatch(match)
			if len(parts) < 3 {
				return match
			}

			label := strings.TrimSpace(parts[1])
			url := strings.TrimSpace(parts[2])
			if label == "" {
				label = url
			}
			if label == "" || url == "" {
				return label
			}

			linkType := classifyLinkType(url)
			label = normalizeLinkLabel(label, url, linkType)
			id := len(links) + 1

			token := fmt.Sprintf("AWLINK%04dTOKEN", len(links)+1)
			links = append(links, WikiRenderLink{
				ID:    id,
				Token: token,
				Label: label,
				URL:   url,
				Type:  linkType,
				State: LinkStateNormal,
			})
			return token
		})

		line = markdownAutolinkRE.ReplaceAllString(line, "")
		line = proseInlineURLRE.ReplaceAllString(line, "$1$2$3")
		if urlOnlyLineRE.MatchString(strings.TrimSpace(line)) {
			line = ""
		}

		line = spaceBeforePunctuationRE.ReplaceAllString(line, "$1")
		line = strings.TrimRight(line, " \t")
		out = append(out, line)
	}

	return strings.Join(out, "\n"), links
}

// ApplyWikiRenderLinks swaps link tokens with OSC-8 terminal hyperlinks.
func (e *Engine) ApplyWikiRenderLinks(rendered string, links []WikiRenderLink) string {
	if len(links) == 0 || rendered == "" {
		return rendered
	}

	for _, link := range links {
		rendered = strings.ReplaceAll(rendered, link.Token, renderWikiLink(link))
	}

	return rendered
}

// ApplyCalloutStyles colorizes rendered callout boxes by semantic kind.
func (e *Engine) ApplyCalloutStyles(rendered string) string {
	if strings.TrimSpace(rendered) == "" {
		return rendered
	}

	lines := strings.Split(rendered, "\n")
	for i := 0; i < len(lines); i++ {
		visible := stripANSIFormatting(lines[i])
		color, ok := calloutColorForHeader(visible)
		if !ok {
			continue
		}

		lines[i] = styleText(visible, color, true, false)
		for j := i + 1; j < len(lines); j++ {
			row := stripANSIFormatting(lines[j])
			if strings.TrimSpace(row) == "" {
				lines[j] = row
				continue
			}

			lines[j] = styleText(row, color, false, false)
			if strings.Contains(row, "└") {
				i = j
				break
			}
		}
	}

	return strings.Join(lines, "\n")
}

func stripANSIFormatting(s string) string {
	if s == "" {
		return ""
	}
	return ansiCSIRE.ReplaceAllString(ansiOSC8RE.ReplaceAllString(s, ""), "")
}

func calloutColorForHeader(line string) (string, bool) {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "┌─ warning "):
		return "203", true
	case strings.Contains(lower, "┌─ tip "):
		return "78", true
	case strings.Contains(lower, "┌─ note "):
		return "117", true
	case strings.Contains(lower, "┌─ important "):
		return "220", true
	case strings.Contains(lower, "┌─ info "):
		return "117", true
	default:
		return "", false
	}
}

func renderWikiLink(link WikiRenderLink) string {
	label := strings.TrimSpace(link.Label)
	if label == "" {
		label = strings.TrimSpace(link.URL)
	}
	if label == "" {
		return ""
	}

	if link.ID > 0 {
		label = fmt.Sprintf("[%d] %s", link.ID, label)
	}

	color := "81"
	underline := true
	bold := true

	switch link.Type {
	case LinkTypeSection:
		color = "117"
	case LinkTypeReference:
		color = "245"
		underline = false
		bold = false
	case LinkTypeExternal:
		color = "180"
		bold = false
	}

	switch link.State {
	case LinkStateFocused:
		bold = true
		underline = true
		color = "229"
	case LinkStateVisited:
		color = "245"
	case LinkStateInactive:
		underline = false
		color = "241"
		return stylePlainText(label, color, false, false)
	}

	return makeStyledTerminalHyperlink(label, link.URL, color, bold, underline)
}

func stylePlainText(text, color string, bold, underline bool) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return styleText(text, color, bold, underline)
}

func styleText(text, color string, bold, underline bool) string {
	seq := strings.Builder{}
	if color != "" {
		seq.WriteString("\x1b[38;5;")
		seq.WriteString(color)
		seq.WriteString("m")
	}
	if bold {
		seq.WriteString("\x1b[1m")
	}
	if underline {
		seq.WriteString("\x1b[4m")
	}

	reset := strings.Builder{}
	if underline {
		reset.WriteString("\x1b[24m")
	}
	if bold {
		reset.WriteString("\x1b[22m")
	}
	if color != "" {
		reset.WriteString("\x1b[39m")
	}

	return seq.String() + text + reset.String()
}

func makeStyledTerminalHyperlink(label, target, color string, bold, underline bool) string {
	label = strings.TrimSpace(label)
	target = strings.TrimSpace(target)
	if label == "" {
		label = target
	}
	if label == "" {
		return ""
	}

	if target == "" {
		return styleText(label, color, bold, underline)
	}

	cleanTarget := strings.ReplaceAll(target, "\x1b", "")
	open := "\x1b]8;;" + cleanTarget + "\x1b\\"
	close := "\x1b]8;;\x1b\\"
	return styleText(open+label+close, color, bold, underline)
}

func classifyLinkType(rawURL string) LinkType {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return LinkTypeExternal
	}
	if strings.HasPrefix(rawURL, "#cite") || strings.Contains(rawURL, "#cite") {
		return LinkTypeReference
	}
	if strings.HasPrefix(rawURL, "#") {
		return LinkTypeSection
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return LinkTypeExternal
	}

	host := strings.ToLower(u.Hostname())
	if host == "wiki.archlinux.org" || host == "www.wiki.archlinux.org" {
		if strings.HasPrefix(strings.ToLower(u.Fragment), "cite") {
			return LinkTypeReference
		}
		if u.Fragment != "" && (u.Path == "" || u.Path == "/") {
			return LinkTypeSection
		}
		if u.Query().Get("title") != "" || strings.HasPrefix(u.Path, "/title/") {
			return LinkTypeInternal
		}
		if u.Fragment != "" {
			return LinkTypeSection
		}
		return LinkTypeInternal
	}

	if strings.HasPrefix(rawURL, "/title/") || strings.HasPrefix(rawURL, "/index.php") || strings.HasPrefix(rawURL, "index.php?") {
		return LinkTypeInternal
	}

	return LinkTypeExternal
}

func normalizeLinkLabel(label, target string, linkType LinkType) string {
	label = collapseWhitespace(label)
	if label != "" && !looksLikeURL(label) {
		return label
	}

	switch linkType {
	case LinkTypeInternal:
		if title, ok := ArchWikiTitleFromURL(target); ok {
			return title
		}
		return "page"
	case LinkTypeSection:
		if anchor := sectionLabel(target); anchor != "" {
			return anchor
		}
		return "section"
	default:
		if host := hostLabel(target); host != "" {
			return host
		}
		return "source"
	}
}

func looksLikeURL(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "www.")
}

func hostLabel(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	host = strings.TrimPrefix(host, "www.")
	return host
}

func sectionLabel(rawURL string) string {
	if strings.HasPrefix(rawURL, "#") {
		frag := strings.TrimPrefix(rawURL, "#")
		frag = strings.ReplaceAll(frag, "_", " ")
		frag = strings.TrimSpace(frag)
		if frag != "" {
			return frag
		}
		return "section"
	}

	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	frag := strings.TrimSpace(strings.ReplaceAll(u.Fragment, "_", " "))
	return frag
}

// ArchWikiTitleFromURL resolves an ArchWiki article title from absolute/relative wiki URLs.
func ArchWikiTitleFromURL(raw string) (string, bool) {
	raw = trimURLPunctuation(strings.TrimSpace(raw))
	if raw == "" {
		return "", false
	}

	if strings.HasPrefix(raw, "/") {
		raw = "https://wiki.archlinux.org" + raw
	}
	if strings.HasPrefix(raw, "index.php?") {
		raw = "https://wiki.archlinux.org/" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}

	host := strings.ToLower(u.Hostname())
	if host != "wiki.archlinux.org" && host != "www.wiki.archlinux.org" {
		return "", false
	}

	title := strings.TrimSpace(u.Query().Get("title"))
	if title == "" && strings.HasPrefix(u.Path, "/title/") {
		title = strings.TrimPrefix(u.Path, "/title/")
	}
	if title == "" {
		return "", false
	}

	decoded, err := url.PathUnescape(title)
	if err == nil {
		title = decoded
	}

	title = strings.TrimSpace(strings.ReplaceAll(title, "_", " "))
	if title == "" {
		return "", false
	}

	return title, true
}

// MakeTerminalHyperlink builds an OSC-8 hyperlink with consistent styling.
func MakeTerminalHyperlink(label, target string) string {
	return makeStyledTerminalHyperlink(label, target, "214", false, true)
}

func trimURLPunctuation(raw string) string {
	return strings.TrimRight(raw, ".,;:!?)\"]'")
}

func collapseWhitespace(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func normalizeMarkdown(md string) string {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))

	blankCount := 0
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if strings.TrimSpace(trimmed) == "" {
			blankCount++
			if blankCount > 1 {
				continue
			}
			out = append(out, "")
			continue
		}
		blankCount = 0
		out = append(out, trimmed)
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}

func simplifyMarkdownForReading(md string) string {
	if strings.TrimSpace(md) == "" {
		return ""
	}

	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	inCodeFence := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeFence = !inCodeFence
			out = append(out, line)
			continue
		}

		if inCodeFence {
			out = append(out, line)
			continue
		}

		line = markdownAutolinkStripper.ReplaceAllString(line, "")
		line = proseURLStripper.ReplaceAllString(line, "")
		if proseURLOnlyLine.MatchString(line) {
			line = ""
		}
		line = strings.TrimRight(line, " \t")

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

func splitRelatedArticles(markdown string) (string, []ArticleLink) {
	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	if strings.TrimSpace(normalized) == "" {
		return "", nil
	}

	lines := strings.Split(normalized, "\n")
	start := -1
	end := -1

	for i, line := range lines {
		if !isRelatedHeadingLine(line) {
			continue
		}

		start = i
		end = len(lines)
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			for j := i + 1; j < len(lines); j++ {
				if strings.HasPrefix(strings.TrimSpace(lines[j]), "#") {
					end = j
					break
				}
			}
		} else {
			sawLink := false
			for j := i + 1; j < len(lines); j++ {
				trimmed := strings.TrimSpace(lines[j])
				if trimmed == "" {
					continue
				}
				if strings.HasPrefix(trimmed, "#") {
					end = j
					break
				}
				if isListOrLinkLine(trimmed) {
					sawLink = true
					continue
				}
				if sawLink {
					end = j
				}
				break
			}
		}
		break
	}

	if start < 0 || end <= start {
		return normalized, nil
	}

	relatedSection := strings.Join(lines[start:end], "\n")
	relatedLinks := extractMarkdownLinks(relatedSection)

	remaining := make([]string, 0, len(lines)-(end-start))
	remaining = append(remaining, lines[:start]...)
	if end < len(lines) {
		remaining = append(remaining, lines[end:]...)
	}

	main := strings.TrimSpace(collapseBlankLines(strings.Join(remaining, "\n")))
	return main, relatedLinks
}

func isRelatedHeadingLine(line string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(line))

	headings := []string{"related articles", "related article", "see also", "see-also"}
	for _, heading := range headings {
		if trimmed == heading {
			return true
		}
	}

	for _, prefix := range []string{"# ", "## ", "### ", "#### ", "##### ", "###### "} {
		for _, heading := range headings {
			if strings.HasPrefix(trimmed, prefix+heading) {
				return true
			}
		}
	}

	return false
}

func isListOrLinkLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}
	if strings.HasPrefix(line, "[") || strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
		return true
	}
	return false
}

func extractMarkdownLinks(markdown string) []ArticleLink {
	matches := markdownLinkRE.FindAllStringSubmatch(markdown, -1)
	if len(matches) == 0 {
		return nil
	}

	links := make([]ArticleLink, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		text := strings.TrimSpace(match[1])
		url := strings.TrimSpace(match[2])
		if text == "" || url == "" {
			continue
		}

		links = append(links, ArticleLink{Text: text, URL: url})
	}

	return links
}

func dedupeArticleLinks(links []ArticleLink) []ArticleLink {
	if len(links) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(links))
	out := make([]ArticleLink, 0, len(links))
	for _, link := range links {
		key := strings.ToLower(strings.TrimSpace(link.Text) + "|" + strings.TrimSpace(link.URL))
		if key == "|" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, link)
	}
	return out
}

func filterSemanticRelatedLinks(links []ArticleLink) []ArticleLink {
	if len(links) == 0 {
		return nil
	}

	out := make([]ArticleLink, 0, len(links))
	for _, link := range links {
		if !isSemanticRelatedURL(link.URL) {
			continue
		}
		out = append(out, link)
	}

	return out
}

func isSemanticRelatedURL(raw string) bool {
	raw = trimURLPunctuation(strings.TrimSpace(raw))
	if raw == "" || strings.HasPrefix(raw, "#") {
		return false
	}

	normalized := raw
	if strings.HasPrefix(normalized, "/") {
		normalized = "https://wiki.archlinux.org" + normalized
	}
	if strings.HasPrefix(normalized, "index.php?") {
		normalized = "https://wiki.archlinux.org/" + normalized
	}

	if parsed, err := url.Parse(normalized); err == nil {
		if strings.TrimSpace(parsed.Fragment) != "" {
			return false
		}
	}

	_, ok := ArchWikiTitleFromURL(raw)
	return ok
}

func linksViewMarkdown(title, emptyMessage string, links []ArticleLink) string {
	lines := []string{"# " + title, ""}
	if len(links) == 0 {
		if strings.TrimSpace(emptyMessage) == "" {
			emptyMessage = "No entries found."
		}
		lines = append(lines, emptyMessage)
		return strings.Join(lines, "\n")
	}

	for _, link := range links {
		lines = append(lines, "- "+link.Text+" -> "+link.URL)
	}

	return strings.Join(lines, "\n")
}

func collapseBlankLines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

func addHierarchicalHeadingNumbers(markdown string) string {
	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	if strings.TrimSpace(normalized) == "" {
		return normalized
	}

	lines := strings.Split(normalized, "\n")
	out := make([]string, 0, len(lines))
	counters := [6]int{}
	inFence := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if isFenceDelimiter(trimmed) {
			inFence = !inFence
			out = append(out, line)
			continue
		}

		if inFence {
			out = append(out, line)
			continue
		}

		if level, title, ok := parseATXHeading(trimmed); ok {
			title = headingWithNumber(&counters, level, title)
			out = append(out, strings.Repeat("#", level)+" "+title)
			continue
		}

		if i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if setextLevel, ok := parseSetextLevel(next); ok && strings.TrimSpace(line) != "" {
				title := headingWithNumber(&counters, setextLevel, strings.TrimSpace(line))
				out = append(out, strings.Repeat("#", setextLevel)+" "+title)
				i++
				continue
			}
		}

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

func headingWithNumber(counters *[6]int, level int, title string) string {
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return title
	}

	if level > 3 {
		return title
	}

	(*counters)[level-1]++
	for i := level; i < len(*counters); i++ {
		(*counters)[i] = 0
	}

	if hasHeadingNumberPrefix(title) {
		return title
	}

	parts := make([]string, 0, level)
	for i := 0; i < level; i++ {
		if (*counters)[i] > 0 {
			parts = append(parts, fmt.Sprintf("%d", (*counters)[i]))
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "1")
	}

	return strings.Join(parts, ".") + " " + title
}

func hasHeadingNumberPrefix(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || s[0] < '0' || s[0] > '9' {
		return false
	}

	i := 0
	seenDigit := false
	for i < len(s) {
		if s[i] >= '0' && s[i] <= '9' {
			seenDigit = true
			i++
			continue
		}
		if s[i] == '.' {
			i++
			continue
		}
		break
	}

	return seenDigit && i < len(s) && s[i] == ' '
}

func parseATXHeading(line string) (level int, title string, ok bool) {
	if line == "" || line[0] != '#' {
		return 0, "", false
	}

	level = 0
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	if level == 0 {
		return 0, "", false
	}

	title = strings.TrimSpace(line[level:])
	title = strings.TrimSpace(strings.TrimRight(title, "#"))
	if title == "" {
		return 0, "", false
	}

	return level, title, true
}

func parseSetextLevel(line string) (int, bool) {
	if line == "" {
		return 0, false
	}
	if strings.Trim(line, "=") == "" {
		return 1, true
	}
	if strings.Trim(line, "-") == "" {
		return 2, true
	}
	return 0, false
}

func isFenceDelimiter(line string) bool {
	return strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~")
}
