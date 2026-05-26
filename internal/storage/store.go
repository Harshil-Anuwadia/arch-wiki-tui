package storage

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

const maxHistoryEntries = 150

const archiveIndexVersion = 1

// HistoryEntry tracks recently visited pages.
type HistoryEntry struct {
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	VisitedAt time.Time `json:"visited_at"`
}

// State is persisted to disk.
type State struct {
	History      []HistoryEntry `json:"history"`
	CachedTitles []string       `json:"cached_titles"`
}

// PageCacheEntry represents one markdown page write into the local page cache.
type PageCacheEntry struct {
	Title    string
	Markdown string
}

// ArchiveEntry tracks one compressed page in the offline full-archive store.
type ArchiveEntry struct {
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	File      string    `json:"file"`
	Bytes     int64     `json:"bytes"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ArchiveIndex is the persistent lookup table for compressed full-archive pages.
type ArchiveIndex struct {
	Version     int                     `json:"version"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
	TotalTitles int                     `json:"total_titles"`
	Entries     map[string]ArchiveEntry `json:"entries"`
}

// Store encapsulates state and local page cache.
type Store struct {
	mu              sync.Mutex
	statePath       string
	cacheRoot       string
	cacheDir        string
	archiveDir      string
	archivePagesDir string
	state           State
}

// NewStore initializes config and cache directories using XDG defaults.
func NewStore(appName string) (*Store, error) {
	if strings.TrimSpace(appName) == "" {
		return nil, errors.New("app name cannot be empty")
	}

	cfgBase, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve config dir: %w", err)
	}

	cacheBase, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("resolve cache dir: %w", err)
	}

	cfgDir := filepath.Join(cfgBase, appName)
	cacheRoot := filepath.Join(cacheBase, appName)
	cacheDir := filepath.Join(cacheRoot, "pages")
	archiveDir := filepath.Join(cacheRoot, "archive")
	archivePagesDir := filepath.Join(archiveDir, "pages")

	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create root cache dir: %w", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	if err := os.MkdirAll(archivePagesDir, 0o755); err != nil {
		return nil, fmt.Errorf("create archive cache dir: %w", err)
	}

	s := &Store{
		statePath:       filepath.Join(cfgDir, "state.json"),
		cacheRoot:       cacheRoot,
		cacheDir:        cacheDir,
		archiveDir:      archiveDir,
		archivePagesDir: archivePagesDir,
		state: State{
			History:      []HistoryEntry{},
			CachedTitles: []string{},
		},
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

// IndexCachePath returns the file path used for the searchable title index cache.
func (s *Store) IndexCachePath() string {
	return filepath.Join(s.cacheRoot, "title-index.json")
}

// ArchiveIndexPath returns the index path used for full compressed offline archive lookups.
func (s *Store) ArchiveIndexPath() string {
	return filepath.Join(s.archiveDir, "index.json")
}

// LoadArchiveIndex reads the archive index from disk, returning an empty initialized index on first run.
func (s *Store) LoadArchiveIndex() (ArchiveIndex, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadArchiveIndexLocked()
}

// SaveArchiveIndex persists the full-archive index atomically.
func (s *Store) SaveArchiveIndex(index ArchiveIndex) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveArchiveIndexLocked(index)
}

// ArchiveSyncPlan compares fetched titles against existing archive files and returns missing titles to download.
func (s *Store) ArchiveSyncPlan(titles []string) ([]string, int, ArchiveIndex, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadArchiveIndexLocked()
	if err != nil {
		return nil, 0, ArchiveIndex{}, err
	}

	ordered := make([]string, 0, len(titles))
	seen := make(map[string]struct{}, len(titles))
	for _, raw := range titles {
		title := strings.TrimSpace(raw)
		if title == "" {
			continue
		}

		key := archiveTitleKey(title)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		ordered = append(ordered, title)
	}

	if len(ordered) == 0 {
		return nil, 0, index, errors.New("archive title list is empty")
	}

	queue := make([]string, 0, len(ordered))
	cached := 0
	for _, title := range ordered {
		key := archiveTitleKey(title)
		entry, ok := index.Entries[key]
		if !ok || strings.TrimSpace(entry.File) == "" {
			queue = append(queue, title)
			continue
		}

		path := s.archiveEntryPathLocked(entry.File)
		info, statErr := os.Stat(path)
		if statErr != nil || info.IsDir() {
			delete(index.Entries, key)
			queue = append(queue, title)
			continue
		}

		if strings.TrimSpace(entry.Title) == "" {
			entry.Title = title
			index.Entries[key] = entry
		}
		cached++
	}

	for key := range index.Entries {
		if _, ok := seen[key]; !ok {
			delete(index.Entries, key)
		}
	}

	index.Version = archiveIndexVersion
	if index.CreatedAt.IsZero() {
		index.CreatedAt = time.Now().UTC()
	}
	index.TotalTitles = len(ordered)

	return queue, cached, index, nil
}

// WriteArchivePage compresses markdown with zstd and updates the provided archive index in-memory.
func (s *Store) WriteArchivePage(title, pageURL, markdown string, index *ArchiveIndex) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("archive title is empty")
	}
	if index == nil {
		return errors.New("archive index is nil")
	}

	key := archiveTitleKey(title)
	if key == "" {
		return errors.New("archive title key is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if index.Entries == nil {
		index.Entries = make(map[string]ArchiveEntry)
	}

	relFile := filepath.ToSlash(filepath.Join("pages", archiveFileName(title)))
	absFile := s.archiveEntryPathLocked(relFile)
	if err := os.MkdirAll(filepath.Dir(absFile), 0o755); err != nil {
		return fmt.Errorf("create archive page dir: %w", err)
	}

	tmpFile := absFile + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("create archive tmp page: %w", err)
	}

	encoder, err := zstd.NewWriter(f, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("create zstd encoder: %w", err)
	}

	_, writeErr := io.Copy(encoder, strings.NewReader(markdown))
	closeEncoderErr := encoder.Close()
	closeFileErr := f.Close()
	if writeErr != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("write compressed archive page: %w", writeErr)
	}
	if closeEncoderErr != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("finalize compressed archive page: %w", closeEncoderErr)
	}
	if closeFileErr != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("close archive tmp page: %w", closeFileErr)
	}

	if err := os.Rename(tmpFile, absFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("move compressed archive page into place: %w", err)
	}

	info, err := os.Stat(absFile)
	if err != nil {
		return fmt.Errorf("stat compressed archive page: %w", err)
	}

	index.Version = archiveIndexVersion
	if index.CreatedAt.IsZero() {
		index.CreatedAt = time.Now().UTC()
	}
	index.Entries[key] = ArchiveEntry{
		Title:     title,
		URL:       strings.TrimSpace(pageURL),
		File:      relFile,
		Bytes:     info.Size(),
		UpdatedAt: time.Now().UTC(),
	}

	return nil
}

// ReadArchivePage retrieves and decompresses a page from the full-archive cache.
func (s *Store) ReadArchivePage(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("archive title is empty")
	}

	key := archiveTitleKey(title)
	if key == "" {
		return "", errors.New("archive title key is empty")
	}

	s.mu.Lock()
	index, err := s.loadArchiveIndexLocked()
	if err != nil {
		s.mu.Unlock()
		return "", err
	}

	entry, ok := index.Entries[key]
	if !ok {
		s.mu.Unlock()
		return "", fmt.Errorf("archive page %q not found: %w", title, os.ErrNotExist)
	}

	path := s.archiveEntryPathLocked(entry.File)
	s.mu.Unlock()

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return "", fmt.Errorf("create zstd decoder: %w", err)
	}
	defer decoder.Close()

	decoded, err := decoder.DecodeAll(raw, nil)
	if err != nil {
		return "", fmt.Errorf("decode archive page %q: %w", title, err)
	}

	return string(decoded), nil
}

// ListArchiveTitles returns titles present in the full-archive index in case-insensitive sorted order.
func (s *Store) ListArchiveTitles() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadArchiveIndexLocked()
	if err != nil || len(index.Entries) == 0 {
		return nil
	}

	titles := make([]string, 0, len(index.Entries))
	seen := make(map[string]struct{}, len(index.Entries))
	for key, entry := range index.Entries {
		title := strings.TrimSpace(entry.Title)
		if title == "" {
			title = key
		}
		if title == "" {
			continue
		}

		norm := strings.ToLower(title)
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		titles = append(titles, title)
	}

	sort.Slice(titles, func(i, j int) bool {
		left := strings.ToLower(titles[i])
		right := strings.ToLower(titles[j])
		if left == right {
			return titles[i] < titles[j]
		}
		return left < right
	})

	return titles
}

// AddHistory inserts a page visit and keeps recent unique entries.
func (s *Store) AddHistory(title, url string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("history title is empty")
	}

	entry := HistoryEntry{
		Title:     title,
		URL:       strings.TrimSpace(url),
		VisitedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]HistoryEntry, 0, len(s.state.History)+1)
	filtered = append(filtered, entry)
	for _, h := range s.state.History {
		if strings.EqualFold(h.Title, title) {
			continue
		}
		filtered = append(filtered, h)
		if len(filtered) >= maxHistoryEntries {
			break
		}
	}

	s.state.History = filtered
	return s.saveLocked()
}

// ListHistory returns most recent entries first.
func (s *Store) ListHistory(limit int) []HistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 || limit > len(s.state.History) {
		limit = len(s.state.History)
	}

	out := make([]HistoryEntry, limit)
	copy(out, s.state.History[:limit])
	return out
}

// ListCachedTitles returns cached page titles, most recently cached first.
func (s *Store) ListCachedTitles() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.state.CachedTitles) == 0 {
		return nil
	}

	out := make([]string, len(s.state.CachedTitles))
	copy(out, s.state.CachedTitles)
	return out
}

// WritePageCache persists rendered markdown for offline fallback.
func (s *Store) WritePageCache(title, markdown string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("cache title is empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.cacheDir, cacheFileName(title))
	if err := os.WriteFile(filePath, []byte(markdown), 0o644); err != nil {
		return err
	}

	filtered := make([]string, 0, len(s.state.CachedTitles)+1)
	filtered = append(filtered, title)
	for _, existing := range s.state.CachedTitles {
		if strings.EqualFold(strings.TrimSpace(existing), title) {
			continue
		}
		filtered = append(filtered, existing)
	}
	s.state.CachedTitles = filtered

	return s.saveLocked()
}

// WritePageCacheBatch writes many cached pages and updates cached-title ordering with one state save.
func (s *Store) WritePageCacheBatch(entries []PageCacheEntry) (int, int, error) {
	if len(entries) == 0 {
		return 0, 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	successfulTitles := make([]string, 0, len(entries))
	written := 0
	failed := 0

	for _, entry := range entries {
		title := strings.TrimSpace(entry.Title)
		if title == "" {
			failed++
			continue
		}

		filePath := filepath.Join(s.cacheDir, cacheFileName(title))
		if err := os.WriteFile(filePath, []byte(entry.Markdown), 0o644); err != nil {
			failed++
			continue
		}

		successfulTitles = append(successfulTitles, title)
		written++
	}

	if written > 0 {
		seen := make(map[string]struct{}, len(successfulTitles)+len(s.state.CachedTitles))
		merged := make([]string, 0, len(successfulTitles)+len(s.state.CachedTitles))

		for _, title := range successfulTitles {
			key := strings.ToLower(strings.TrimSpace(title))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, title)
		}

		for _, title := range s.state.CachedTitles {
			key := strings.ToLower(strings.TrimSpace(title))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, title)
		}

		s.state.CachedTitles = merged
	}

	if err := s.saveLocked(); err != nil {
		return written, failed, err
	}

	return written, failed, nil
}

// ReadPageCache loads cached markdown by title.
func (s *Store) ReadPageCache(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("cache title is empty")
	}

	filePath := filepath.Join(s.cacheDir, cacheFileName(title))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ClearCache removes cached page markdown files and the cached title index.
func (s *Store) ClearCache() (int, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	removedPages := 0
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, false, fmt.Errorf("read cache dir: %w", err)
	}
	if err == nil {
		for _, entry := range entries {
			if entry.Type().IsRegular() {
				removedPages++
			}
		}
	}

	if err := os.RemoveAll(s.cacheDir); err != nil {
		return 0, false, fmt.Errorf("remove page cache dir: %w", err)
	}
	if err := os.MkdirAll(s.cacheDir, 0o755); err != nil {
		return 0, false, fmt.Errorf("recreate page cache dir: %w", err)
	}

	if err := os.RemoveAll(s.archiveDir); err != nil {
		return 0, false, fmt.Errorf("remove archive dir: %w", err)
	}
	if err := os.MkdirAll(s.archivePagesDir, 0o755); err != nil {
		return 0, false, fmt.Errorf("recreate archive dir: %w", err)
	}

	indexRemoved := false
	indexPath := s.IndexCachePath()
	if err := os.Remove(indexPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return removedPages, false, fmt.Errorf("remove title index cache: %w", err)
		}
	} else {
		indexRemoved = true
	}

	s.state.CachedTitles = []string{}
	if err := s.saveLocked(); err != nil {
		return removedPages, indexRemoved, err
	}

	return removedPages, indexRemoved, nil
}

func cacheFileName(title string) string {
	normalized := strings.ToLower(strings.TrimSpace(title))
	sum := sha1.Sum([]byte(normalized))
	return fmt.Sprintf("%x.md", sum[:8])
}

func archiveFileName(title string) string {
	normalized := archiveTitleKey(title)
	sum := sha1.Sum([]byte(normalized))
	return fmt.Sprintf("%x.md.zst", sum[:10])
}

func archiveTitleKey(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}

func newArchiveIndex() ArchiveIndex {
	return ArchiveIndex{
		Version: archiveIndexVersion,
		Entries: make(map[string]ArchiveEntry),
	}
}

func (s *Store) loadArchiveIndexLocked() (ArchiveIndex, error) {
	raw, err := os.ReadFile(s.ArchiveIndexPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newArchiveIndex(), nil
		}
		return ArchiveIndex{}, fmt.Errorf("read archive index: %w", err)
	}

	var index ArchiveIndex
	if err := json.Unmarshal(raw, &index); err != nil {
		return ArchiveIndex{}, fmt.Errorf("decode archive index: %w", err)
	}

	if index.Version == 0 {
		index.Version = archiveIndexVersion
	}
	if index.Entries == nil {
		index.Entries = make(map[string]ArchiveEntry)
	}

	return index, nil
}

func (s *Store) saveArchiveIndexLocked(index ArchiveIndex) error {
	if index.Version == 0 {
		index.Version = archiveIndexVersion
	}
	if index.CreatedAt.IsZero() {
		index.CreatedAt = time.Now().UTC()
	}
	if index.UpdatedAt.IsZero() {
		index.UpdatedAt = time.Now().UTC()
	}
	if index.Entries == nil {
		index.Entries = make(map[string]ArchiveEntry)
	}

	if err := os.MkdirAll(s.archiveDir, 0o755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}

	payload, err := json.Marshal(index)
	if err != nil {
		return fmt.Errorf("encode archive index: %w", err)
	}

	if err := os.WriteFile(s.ArchiveIndexPath(), payload, 0o644); err != nil {
		return fmt.Errorf("write archive index: %w", err)
	}

	return nil
}

func (s *Store) archiveEntryPathLocked(rel string) string {
	rel = filepath.FromSlash(strings.TrimSpace(rel))
	if rel == "" {
		return s.archiveDir
	}

	cleaned := filepath.Clean(rel)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return s.archiveDir
	}

	if filepath.IsAbs(cleaned) {
		return filepath.Join(s.archivePagesDir, archiveFileName(cleaned))
	}

	full := filepath.Join(s.archiveDir, cleaned)
	root := s.archiveDir + string(filepath.Separator)
	if !strings.HasPrefix(full, root) && full != s.archiveDir {
		return filepath.Join(s.archivePagesDir, archiveFileName(cleaned))
	}

	return full
}

func (s *Store) load() error {
	raw, err := os.ReadFile(s.statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return fmt.Errorf("decode state file: %w", err)
	}

	if state.History == nil {
		state.History = []HistoryEntry{}
	}
	if state.CachedTitles == nil {
		state.CachedTitles = []string{}
	}

	s.state = state
	return nil
}

func (s *Store) saveLocked() error {
	payload, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state file: %w", err)
	}
	if err := os.WriteFile(s.statePath, payload, 0o644); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}
	return nil
}
