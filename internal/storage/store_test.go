package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreHistory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	s, err := NewStore("archwiki-tui-test")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if err := s.AddHistory("Pacman", "https://wiki.archlinux.org/title/Pacman"); err != nil {
		t.Fatalf("AddHistory failed: %v", err)
	}
	if err := s.AddHistory("Systemd", "https://wiki.archlinux.org/title/Systemd"); err != nil {
		t.Fatalf("AddHistory failed: %v", err)
	}

	history := s.ListHistory(10)
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	if history[0].Title != "Systemd" {
		t.Fatalf("expected most recent history entry to be Systemd, got %q", history[0].Title)
	}
}

func TestPageCacheRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	s, err := NewStore("archwiki-tui-test")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	content := "# Pacman\n\nInstall packages with pacman -S"
	if err := s.WritePageCache("Pacman", content); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}

	cached, err := s.ReadPageCache("Pacman")
	if err != nil {
		t.Fatalf("ReadPageCache failed: %v", err)
	}
	if cached != content {
		t.Fatalf("cache content mismatch: got %q want %q", cached, content)
	}

	if got := cacheFileName("Pacman"); filepath.Ext(got) != ".md" {
		t.Fatalf("cache file should end with .md, got %q", got)
	}
}

func TestIndexCachePath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	s, err := NewStore("archwiki-tui-test")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	path := s.IndexCachePath()
	if filepath.Ext(path) != ".json" {
		t.Fatalf("expected index cache path to end with .json, got %q", path)
	}
}

func TestClearCache(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	s, err := NewStore("archwiki-tui-test")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if err := s.WritePageCache("Pacman", "# Pacman"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}
	if err := s.WritePageCache("Systemd", "# Systemd"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}

	indexPath := s.IndexCachePath()
	if err := os.WriteFile(indexPath, []byte(`{"updated_at":"2026-01-01T00:00:00Z","titles":["Pacman"]}`), 0o644); err != nil {
		t.Fatalf("write index cache failed: %v", err)
	}

	archiveIndex, err := s.LoadArchiveIndex()
	if err != nil {
		t.Fatalf("LoadArchiveIndex failed: %v", err)
	}
	if err := s.WriteArchivePage("Pacman", "https://wiki.archlinux.org/title/Pacman", "# Pacman\narchive", &archiveIndex); err != nil {
		t.Fatalf("WriteArchivePage failed: %v", err)
	}
	archiveIndex.TotalTitles = 1
	if err := s.SaveArchiveIndex(archiveIndex); err != nil {
		t.Fatalf("SaveArchiveIndex failed: %v", err)
	}

	removedPages, indexRemoved, err := s.ClearCache()
	if err != nil {
		t.Fatalf("ClearCache failed: %v", err)
	}
	if removedPages < 2 {
		t.Fatalf("expected at least 2 removed cached pages, got %d", removedPages)
	}
	if !indexRemoved {
		t.Fatalf("expected title index cache file to be removed")
	}

	if _, err := s.ReadPageCache("Pacman"); err == nil {
		t.Fatalf("expected pacman page cache to be removed")
	}
	if _, err := os.Stat(indexPath); !os.IsNotExist(err) {
		t.Fatalf("expected index cache file to be removed, stat err=%v", err)
	}

	if cached := s.ListCachedTitles(); len(cached) != 0 {
		t.Fatalf("expected cached title list to be reset after clear cache, got %v", cached)
	}

	if titles := s.ListArchiveTitles(); len(titles) != 0 {
		t.Fatalf("expected archive title list to be reset after clear cache, got %v", titles)
	}
	if _, err := s.ReadArchivePage("Pacman"); err == nil {
		t.Fatalf("expected archived pacman page to be removed")
	}
}

func TestListCachedTitlesTracksMostRecentUnique(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	s, err := NewStore("archwiki-tui-test")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if err := s.WritePageCache("Pacman", "# Pacman"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}
	if err := s.WritePageCache("Systemd", "# Systemd"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}
	if err := s.WritePageCache("pacman", "# Pacman updated"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}

	cached := s.ListCachedTitles()
	if len(cached) != 2 {
		t.Fatalf("expected 2 unique cached titles, got %d (%v)", len(cached), cached)
	}
	if cached[0] != "pacman" {
		t.Fatalf("expected most recent cached title first, got %q", cached[0])
	}
	if !strings.EqualFold(cached[1], "systemd") {
		t.Fatalf("expected second cached title to be systemd, got %q", cached[1])
	}
}

func TestArchivePageRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	s, err := NewStore("archwiki-tui-test")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	index, err := s.LoadArchiveIndex()
	if err != nil {
		t.Fatalf("LoadArchiveIndex failed: %v", err)
	}

	content := "# Pacman\n\nInstall packages with pacman -S"
	if err := s.WriteArchivePage("Pacman", "https://wiki.archlinux.org/title/Pacman", content, &index); err != nil {
		t.Fatalf("WriteArchivePage failed: %v", err)
	}
	index.TotalTitles = 1
	if err := s.SaveArchiveIndex(index); err != nil {
		t.Fatalf("SaveArchiveIndex failed: %v", err)
	}

	got, err := s.ReadArchivePage("Pacman")
	if err != nil {
		t.Fatalf("ReadArchivePage failed: %v", err)
	}
	if got != content {
		t.Fatalf("archive content mismatch: got %q want %q", got, content)
	}

	titles := s.ListArchiveTitles()
	if len(titles) != 1 || !strings.EqualFold(titles[0], "Pacman") {
		t.Fatalf("expected archive titles to include Pacman, got %v", titles)
	}
}

func TestArchiveSyncPlanSkipsExistingAndPrunesStaleEntries(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	s, err := NewStore("archwiki-tui-test")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	index, err := s.LoadArchiveIndex()
	if err != nil {
		t.Fatalf("LoadArchiveIndex failed: %v", err)
	}

	if err := s.WriteArchivePage("Pacman", "https://wiki.archlinux.org/title/Pacman", "# Pacman", &index); err != nil {
		t.Fatalf("WriteArchivePage failed: %v", err)
	}
	index.Entries[archiveTitleKey("Legacy")] = ArchiveEntry{
		Title: "Legacy",
		File:  "pages/legacy.md.zst",
	}
	index.TotalTitles = 2
	if err := s.SaveArchiveIndex(index); err != nil {
		t.Fatalf("SaveArchiveIndex failed: %v", err)
	}

	queue, cached, planned, err := s.ArchiveSyncPlan([]string{"Pacman", "Systemd"})
	if err != nil {
		t.Fatalf("ArchiveSyncPlan failed: %v", err)
	}

	if cached != 1 {
		t.Fatalf("expected one cached page from plan, got %d", cached)
	}
	if len(queue) != 1 || !strings.EqualFold(queue[0], "Systemd") {
		t.Fatalf("expected plan queue to include only Systemd, got %v", queue)
	}
	if _, ok := planned.Entries[archiveTitleKey("legacy")]; ok {
		t.Fatalf("expected stale Legacy entry to be pruned from plan")
	}
	if planned.TotalTitles != 2 {
		t.Fatalf("expected plan total titles to be 2, got %d", planned.TotalTitles)
	}
}

func TestWritePageCacheBatchWritesAndMaintainsRecency(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	s, err := NewStore("archwiki-tui-test")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	if err := s.WritePageCache("Old", "# Old"); err != nil {
		t.Fatalf("WritePageCache failed: %v", err)
	}

	written, failed, err := s.WritePageCacheBatch([]PageCacheEntry{
		{Title: "Pacman", Markdown: "# Pacman"},
		{Title: "Systemd", Markdown: "# Systemd"},
		{Title: "pacman", Markdown: "# Pacman latest"},
		{Title: "", Markdown: "# invalid"},
	})
	if err != nil {
		t.Fatalf("WritePageCacheBatch failed: %v", err)
	}
	if written != 3 {
		t.Fatalf("expected 3 successful batch writes, got %d", written)
	}
	if failed != 1 {
		t.Fatalf("expected 1 failed batch write, got %d", failed)
	}

	titles := s.ListCachedTitles()
	if len(titles) != 3 {
		t.Fatalf("expected 3 unique cached titles, got %d (%v)", len(titles), titles)
	}
	if titles[0] != "Pacman" {
		t.Fatalf("expected most recent unique title Pacman first, got %q", titles[0])
	}
	if !strings.EqualFold(titles[1], "Systemd") {
		t.Fatalf("expected second title Systemd, got %q", titles[1])
	}
	if !strings.EqualFold(titles[2], "Old") {
		t.Fatalf("expected prior title Old retained last, got %q", titles[2])
	}
}
