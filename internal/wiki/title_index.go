package wiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DefaultTitleIndexTTL controls how often the local title index should refresh.
const DefaultTitleIndexTTL = 7 * 24 * time.Hour

type indexedTitle struct {
	title      string
	normalized string
	compact    string
}

type titleIndexCache struct {
	UpdatedAt time.Time `json:"updated_at"`
	Titles    []string  `json:"titles"`
}

type scoredTitle struct {
	title string
	score int
}

// TitleIndex holds an on-disk cached index of Arch Wiki page titles for fast local search.
type TitleIndex struct {
	mu        sync.RWMutex
	cachePath string
	entries   []indexedTitle
	updatedAt time.Time
}

// NewTitleIndex creates and loads a title index from cachePath when available.
func NewTitleIndex(cachePath string) (*TitleIndex, error) {
	idx := &TitleIndex{
		cachePath: strings.TrimSpace(cachePath),
		entries:   []indexedTitle{},
	}
	if err := idx.load(); err != nil {
		return nil, err
	}
	return idx, nil
}

// Count returns total indexed titles.
func (idx *TitleIndex) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}

// Ready returns whether the index has searchable data.
func (idx *TitleIndex) Ready() bool {
	return idx.Count() > 0
}

// UpdatedAt returns when the index was last refreshed.
func (idx *TitleIndex) UpdatedAt() time.Time {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.updatedAt
}

// NeedsRefresh reports whether data should be refreshed based on ttl.
func (idx *TitleIndex) NeedsRefresh(ttl time.Duration) bool {
	if ttl <= 0 {
		ttl = DefaultTitleIndexTTL
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.entries) == 0 {
		return true
	}
	if idx.updatedAt.IsZero() {
		return true
	}
	return time.Since(idx.updatedAt) >= ttl
}

// Search performs fast local title matching with exact/prefix/contains/fuzzy ranking.
func (idx *TitleIndex) Search(query string, limit int) []SearchResult {
	queryNorm := normalizeSearchText(query)
	if queryNorm == "" {
		return nil
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	queryCompact := strings.ReplaceAll(queryNorm, " ", "")
	queryTokens := strings.Fields(queryNorm)

	idx.mu.RLock()
	entries := make([]indexedTitle, len(idx.entries))
	copy(entries, idx.entries)
	idx.mu.RUnlock()

	scored := make([]scoredTitle, 0, minInt(len(entries), 512))
	for _, entry := range entries {
		score := scoreIndexedTitle(entry, queryNorm, queryCompact, queryTokens)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredTitle{title: entry.title, score: score})
	}

	if len(scored) == 0 {
		return nil
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if len(scored[i].title) != len(scored[j].title) {
			return len(scored[i].title) < len(scored[j].title)
		}
		return strings.ToLower(scored[i].title) < strings.ToLower(scored[j].title)
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	results := make([]SearchResult, 0, len(scored))
	for _, hit := range scored {
		results = append(results, SearchResult{
			Title:   hit.title,
			Snippet: "indexed title match",
			URL:     ArticleURL(hit.title),
		})
	}

	return results
}

// Refresh downloads all wiki titles through the API and persists the index.
func (idx *TitleIndex) Refresh(ctx context.Context, client *Client) (int, error) {
	if client == nil {
		return idx.Count(), errors.New("wiki client cannot be nil")
	}

	titles, err := client.FetchAllTitles(ctx, 0)
	if err != nil {
		return idx.Count(), err
	}

	count, replaceErr := idx.ReplaceTitles(titles, time.Now().UTC())
	if replaceErr != nil {
		return count, replaceErr
	}
	return count, nil
}

// ReplaceTitles updates in-memory entries and writes cache to disk.
func (idx *TitleIndex) ReplaceTitles(titles []string, updatedAt time.Time) (int, error) {
	entries := buildIndexEntries(titles)
	if len(entries) == 0 {
		return 0, errors.New("title index is empty")
	}
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}

	idx.mu.Lock()
	idx.entries = entries
	idx.updatedAt = updatedAt
	idx.mu.Unlock()

	if err := idx.save(); err != nil {
		return len(entries), err
	}
	return len(entries), nil
}

func (idx *TitleIndex) load() error {
	if idx.cachePath == "" {
		return nil
	}

	raw, err := os.ReadFile(idx.cachePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read title index cache: %w", err)
	}

	var cache titleIndexCache
	if err := json.Unmarshal(raw, &cache); err != nil {
		return fmt.Errorf("decode title index cache: %w", err)
	}

	entries := buildIndexEntries(cache.Titles)
	idx.mu.Lock()
	idx.entries = entries
	idx.updatedAt = cache.UpdatedAt
	idx.mu.Unlock()

	return nil
}

func (idx *TitleIndex) save() error {
	if idx.cachePath == "" {
		return nil
	}

	idx.mu.RLock()
	cache := titleIndexCache{
		UpdatedAt: idx.updatedAt,
		Titles:    make([]string, 0, len(idx.entries)),
	}
	for _, entry := range idx.entries {
		cache.Titles = append(cache.Titles, entry.title)
	}
	idx.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(idx.cachePath), 0o755); err != nil {
		return fmt.Errorf("create title index cache dir: %w", err)
	}

	payload, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("encode title index cache: %w", err)
	}

	if err := os.WriteFile(idx.cachePath, payload, 0o644); err != nil {
		return fmt.Errorf("write title index cache: %w", err)
	}
	return nil
}

func buildIndexEntries(titles []string) []indexedTitle {
	seen := make(map[string]struct{}, len(titles))
	entries := make([]indexedTitle, 0, len(titles))

	for _, raw := range titles {
		title := strings.TrimSpace(raw)
		if title == "" {
			continue
		}

		key := strings.ToLower(title)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		normalized := normalizeSearchText(title)
		if normalized == "" {
			continue
		}

		entries = append(entries, indexedTitle{
			title:      title,
			normalized: normalized,
			compact:    strings.ReplaceAll(normalized, " ", ""),
		})
	}

	return entries
}

func normalizeSearchText(raw string) string {
	replacer := strings.NewReplacer(
		"_", " ",
		"-", " ",
		"/", " ",
		"\\", " ",
		".", " ",
		":", " ",
		",", " ",
		"(", " ",
		")", " ",
	)
	clean := strings.ToLower(strings.TrimSpace(raw))
	clean = replacer.Replace(clean)
	return strings.Join(strings.Fields(clean), " ")
}

func scoreIndexedTitle(entry indexedTitle, queryNorm, queryCompact string, queryTokens []string) int {
	score := 0
	norm := entry.normalized
	compact := entry.compact

	if norm == queryNorm {
		score += 12000
	}

	if strings.HasPrefix(norm, queryNorm) {
		score += 9000
		score -= minInt(1200, len(norm)-len(queryNorm))
	} else if at := strings.Index(norm, queryNorm); at >= 0 {
		score += 6800
		score -= minInt(900, at*20)
	}

	if len(queryTokens) > 0 {
		allTokens := true
		wordPrefixHits := 0
		for _, token := range queryTokens {
			if !strings.Contains(norm, token) {
				allTokens = false
				break
			}
			if hasWordPrefix(norm, token) {
				wordPrefixHits++
			}
		}
		if allTokens {
			score += 5200 + wordPrefixHits*220
		}
	}

	if queryCompact != "" {
		if compact == queryCompact {
			score += 8000
		}
		if strings.HasPrefix(compact, queryCompact) {
			score += 4200
		}
		if ok, gap := subsequenceGap(queryCompact, compact); ok {
			score += 3200
			score -= minInt(1600, gap*60)
		}
	}

	score -= len(norm) / 8
	return score
}

func hasWordPrefix(sentence, token string) bool {
	for _, field := range strings.Fields(sentence) {
		if strings.HasPrefix(field, token) {
			return true
		}
	}
	return false
}

func subsequenceGap(query, target string) (bool, int) {
	q := []rune(query)
	t := []rune(target)
	if len(q) == 0 || len(t) == 0 || len(q) > len(t) {
		return false, 0
	}

	qPos := 0
	first := -1
	last := -1
	for i, ch := range t {
		if qPos < len(q) && ch == q[qPos] {
			if first == -1 {
				first = i
			}
			last = i
			qPos++
			if qPos == len(q) {
				break
			}
		}
	}

	if qPos != len(q) || first == -1 || last == -1 {
		return false, 0
	}

	span := last - first + 1
	gap := span - len(q)
	if gap < 0 {
		gap = 0
	}
	return true, gap
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
