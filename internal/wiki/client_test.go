package wiki

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestArticleURL(t *testing.T) {
	url := ArticleURL("Pacman Tips")
	want := "https://wiki.archlinux.org/title/Pacman_Tips"
	if url != want {
		t.Fatalf("ArticleURL mismatch: got %q want %q", url, want)
	}
}

func TestNormalizeSnippet(t *testing.T) {
	raw := "Install <span class=\"searchmatch\">pacman</span> quickly &amp; safely"
	got := normalizeSnippet(raw)
	want := "Install pacman quickly & safely"
	if got != want {
		t.Fatalf("normalizeSnippet mismatch: got %q want %q", got, want)
	}
}

func TestFetchPopularTitles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("list") != "querypage" || q.Get("qppage") != "Mostlinked" {
			t.Fatalf("unexpected query params: %v", q)
		}
		_, _ = w.Write([]byte(`{"query":{"querypage":{"results":[{"ns":0,"title":"Pacman"},{"ns":1,"title":"Talk:Pacman"},{"ns":0,"title":"Systemd"},{"ns":0,"title":"Pacman"}]}}}`))
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	client.baseURL = server.URL

	titles, err := client.FetchPopularTitles(context.Background(), 20)
	if err != nil {
		t.Fatalf("FetchPopularTitles error: %v", err)
	}

	if len(titles) != 2 {
		t.Fatalf("expected 2 deduped main-namespace titles, got %d (%v)", len(titles), titles)
	}
	if titles[0] != "Pacman" || titles[1] != "Systemd" {
		t.Fatalf("unexpected title ordering: %v", titles)
	}
}
