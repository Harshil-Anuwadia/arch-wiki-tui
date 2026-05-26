package wiki

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTitleIndexSearchFindsExpectedResults(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "title-index.json")
	idx, err := NewTitleIndex(cachePath)
	if err != nil {
		t.Fatalf("NewTitleIndex failed: %v", err)
	}

	_, err = idx.ReplaceTitles([]string{
		"Pacman",
		"Pacman/Tips and tricks",
		"Systemd",
		"Network configuration",
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("ReplaceTitles failed: %v", err)
	}

	results := idx.Search("pacm", 5)
	if len(results) == 0 {
		t.Fatalf("expected fuzzy results for pacm")
	}
	if results[0].Title != "Pacman" {
		t.Fatalf("expected first result Pacman, got %q", results[0].Title)
	}

	network := idx.Search("network", 5)
	if len(network) == 0 {
		t.Fatalf("expected results for network")
	}
}

func TestTitleIndexCacheReload(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "title-index.json")
	idx, err := NewTitleIndex(cachePath)
	if err != nil {
		t.Fatalf("NewTitleIndex failed: %v", err)
	}

	count, err := idx.ReplaceTitles([]string{"Pacman", "Systemd", "Mkinitcpio"}, time.Now().UTC())
	if err != nil {
		t.Fatalf("ReplaceTitles failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 indexed titles, got %d", count)
	}

	reloaded, err := NewTitleIndex(cachePath)
	if err != nil {
		t.Fatalf("NewTitleIndex reload failed: %v", err)
	}

	if !reloaded.Ready() {
		t.Fatalf("expected reloaded index to be ready")
	}
	if reloaded.Count() != 3 {
		t.Fatalf("expected 3 titles after reload, got %d", reloaded.Count())
	}
}

func TestTitleIndexNeedsRefresh(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "title-index.json")
	idx, err := NewTitleIndex(cachePath)
	if err != nil {
		t.Fatalf("NewTitleIndex failed: %v", err)
	}

	_, err = idx.ReplaceTitles([]string{"Pacman"}, time.Now().Add(-8*24*time.Hour))
	if err != nil {
		t.Fatalf("ReplaceTitles failed: %v", err)
	}

	if !idx.NeedsRefresh(7 * 24 * time.Hour) {
		t.Fatalf("expected index to need refresh")
	}
}
