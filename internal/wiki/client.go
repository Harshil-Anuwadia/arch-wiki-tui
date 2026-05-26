package wiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"archwiki-tui/internal/render"
)

const (
	defaultBaseURL = "https://wiki.archlinux.org/api.php"
	userAgent      = "archwiki-tui/0.1 (+https://github.com/)"
)

var tagStripper = regexp.MustCompile(`<[^>]+>`)

// Client wraps all Arch Wiki API requests.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	renderEngine *render.Engine
}

// NewClient constructs a client with sensible defaults.
func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 12 * time.Second
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          128,
		MaxIdleConnsPerHost:   48,
		MaxConnsPerHost:       48,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Client{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		renderEngine: render.NewEngine(),
	}
}

// ArticleURL builds a canonical page URL for browser fallback.
func ArticleURL(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "https://wiki.archlinux.org"
	}

	normalized := strings.ReplaceAll(title, " ", "_")
	return "https://wiki.archlinux.org/title/" + url.PathEscape(normalized)
}

// Search fetches top article matches from Arch Wiki.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	if limit <= 0 || limit > 50 {
		limit = 20
	}

	prefixHits, prefixErr := c.prefixSearch(ctx, query, limit)
	titleHits, titleErr := c.searchMode(ctx, query, limit, "title")
	textHits, textErr := c.searchMode(ctx, query, limit, "text")

	merged := mergeResultSets(limit, prefixHits, titleHits, textHits)

	errs := make([]error, 0, 3)
	if prefixErr != nil {
		errs = append(errs, fmt.Errorf("prefix search: %w", prefixErr))
	}
	if titleErr != nil {
		errs = append(errs, fmt.Errorf("title search: %w", titleErr))
	}
	if textErr != nil {
		errs = append(errs, fmt.Errorf("text search: %w", textErr))
	}

	if len(merged) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return merged, nil
}

// FetchPopularTitles retrieves currently popular pages from ArchWiki query pages.
func (c *Client) FetchPopularTitles(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 || limit > 100 {
		limit = 40
	}

	params := url.Values{}
	params.Set("action", "query")
	params.Set("list", "querypage")
	params.Set("qppage", "Mostlinked")
	params.Set("qplimit", fmt.Sprintf("%d", limit))
	params.Set("qpnamespace", "0")
	params.Set("utf8", "1")
	params.Set("format", "json")
	params.Set("formatversion", "2")

	endpoint := c.baseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch popular titles failed: %s", resp.Status)
	}

	var payload struct {
		Query struct {
			QueryPage struct {
				Results []struct {
					NS    int    `json:"ns"`
					Title string `json:"title"`
				} `json:"results"`
			} `json:"querypage"`
		} `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode popular titles response: %w", err)
	}

	seen := make(map[string]struct{}, len(payload.Query.QueryPage.Results))
	titles := make([]string, 0, len(payload.Query.QueryPage.Results))
	for _, hit := range payload.Query.QueryPage.Results {
		if hit.NS != 0 {
			continue
		}

		title := strings.TrimSpace(hit.Title)
		if title == "" {
			continue
		}

		key := strings.ToLower(title)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		titles = append(titles, title)
	}

	if len(titles) == 0 {
		return nil, errors.New("popular title list is empty")
	}

	return titles, nil
}

func (c *Client) searchMode(ctx context.Context, query string, limit int, mode string) ([]SearchResult, error) {
	params := url.Values{}
	params.Set("action", "query")
	params.Set("list", "search")
	params.Set("srsearch", query)
	params.Set("srlimit", fmt.Sprintf("%d", limit))
	params.Set("srwhat", mode)
	params.Set("utf8", "1")
	params.Set("format", "json")
	params.Set("formatversion", "2")

	endpoint := c.baseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed: %s", resp.Status)
	}

	var payload struct {
		Query struct {
			Search []struct {
				Title   string `json:"title"`
				Snippet string `json:"snippet"`
			} `json:"search"`
		} `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	results := make([]SearchResult, 0, len(payload.Query.Search))
	for _, hit := range payload.Query.Search {
		title := strings.TrimSpace(hit.Title)
		if title == "" {
			continue
		}

		snippet := normalizeSnippet(hit.Snippet)
		results = append(results, SearchResult{
			Title:   title,
			Snippet: snippet,
			URL:     ArticleURL(title),
		})
	}

	return results, nil
}

func (c *Client) prefixSearch(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	params := url.Values{}
	params.Set("action", "query")
	params.Set("list", "prefixsearch")
	params.Set("pssearch", query)
	params.Set("pslimit", fmt.Sprintf("%d", limit))
	params.Set("utf8", "1")
	params.Set("format", "json")
	params.Set("formatversion", "2")

	endpoint := c.baseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prefix search failed: %s", resp.Status)
	}

	var payload struct {
		Query struct {
			PrefixSearch []struct {
				Title string `json:"title"`
			} `json:"prefixsearch"`
		} `json:"query"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode prefix search response: %w", err)
	}

	results := make([]SearchResult, 0, len(payload.Query.PrefixSearch))
	for _, hit := range payload.Query.PrefixSearch {
		title := strings.TrimSpace(hit.Title)
		if title == "" {
			continue
		}

		results = append(results, SearchResult{
			Title:   title,
			Snippet: "title prefix match",
			URL:     ArticleURL(title),
		})
	}

	return results, nil
}

func mergeResultSets(limit int, sets ...[]SearchResult) []SearchResult {
	if limit <= 0 {
		limit = 20
	}

	merged := make([]SearchResult, 0, limit)
	seen := make(map[string]int, limit*2)

	for _, set := range sets {
		for _, item := range set {
			title := strings.TrimSpace(item.Title)
			if title == "" {
				continue
			}
			key := strings.ToLower(title)

			if at, exists := seen[key]; exists {
				if merged[at].Snippet == "" && item.Snippet != "" {
					merged[at].Snippet = item.Snippet
				}
				continue
			}

			merged = append(merged, item)
			seen[key] = len(merged) - 1
			if len(merged) >= limit {
				return merged
			}
		}
	}

	return merged
}

// FetchAllTitles retrieves wiki article titles from namespace 0 and follows pagination.
func (c *Client) FetchAllTitles(ctx context.Context, limit int) ([]string, error) {
	titles := make([]string, 0, 25000)
	continueToken := ""

	for {
		batchLimit := 500
		if limit > 0 {
			remaining := limit - len(titles)
			if remaining <= 0 {
				break
			}
			if remaining < batchLimit {
				batchLimit = remaining
			}
		}

		params := url.Values{}
		params.Set("action", "query")
		params.Set("list", "allpages")
		params.Set("apnamespace", "0")
		params.Set("aplimit", fmt.Sprintf("%d", batchLimit))
		params.Set("format", "json")
		params.Set("formatversion", "2")
		params.Set("utf8", "1")
		if continueToken != "" {
			params.Set("apcontinue", continueToken)
		}

		endpoint := c.baseURL + "?" + params.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("fetch all titles failed: %s", resp.Status)
		}

		var payload struct {
			Continue struct {
				APContinue string `json:"apcontinue"`
			} `json:"continue"`
			Query struct {
				AllPages []struct {
					Title string `json:"title"`
				} `json:"allpages"`
			} `json:"query"`
		}

		decodeErr := json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode all titles response: %w", decodeErr)
		}

		for _, page := range payload.Query.AllPages {
			title := strings.TrimSpace(page.Title)
			if title == "" {
				continue
			}
			titles = append(titles, title)
			if limit > 0 && len(titles) >= limit {
				break
			}
		}

		if limit > 0 && len(titles) >= limit {
			break
		}

		continueToken = payload.Continue.APContinue
		if continueToken == "" {
			break
		}
	}

	if len(titles) == 0 {
		return nil, errors.New("title list is empty")
	}

	return titles, nil
}

// FetchPage downloads and converts one article into terminal-friendly markdown.
func (c *Client) FetchPage(ctx context.Context, title string) (Page, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Page{}, errors.New("title cannot be empty")
	}

	params := url.Values{}
	params.Set("action", "parse")
	params.Set("page", title)
	params.Set("prop", "text")
	params.Set("redirects", "true")
	params.Set("disabletoc", "true")
	params.Set("format", "json")
	params.Set("formatversion", "2")

	endpoint := c.baseURL + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Page{}, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Page{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Page{}, fmt.Errorf("fetch page failed: %s", resp.Status)
	}

	var payload struct {
		Error *struct {
			Code string `json:"code"`
			Info string `json:"info"`
		} `json:"error"`
		Parse struct {
			Title string `json:"title"`
			Text  string `json:"text"`
		} `json:"parse"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Page{}, fmt.Errorf("decode page response: %w", err)
	}

	if payload.Error != nil {
		return Page{}, fmt.Errorf("wiki error (%s): %s", payload.Error.Code, payload.Error.Info)
	}

	if strings.TrimSpace(payload.Parse.Text) == "" {
		return Page{}, errors.New("empty page content")
	}

	md, codeBlocks, err := c.renderEngine.BuildMarkdownFromHTML(payload.Parse.Text)
	if err != nil {
		return Page{}, err
	}

	resolvedTitle := strings.TrimSpace(payload.Parse.Title)
	if resolvedTitle == "" {
		resolvedTitle = title
	}

	return Page{
		Title:      resolvedTitle,
		URL:        ArticleURL(resolvedTitle),
		Markdown:   md,
		CodeBlocks: codeBlocks,
	}, nil
}

func normalizeSnippet(raw string) string {
	clean := tagStripper.ReplaceAllString(raw, "")
	clean = html.UnescapeString(clean)
	clean = strings.Join(strings.Fields(clean), " ")
	return strings.TrimSpace(clean)
}
