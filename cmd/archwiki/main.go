package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"archwiki-tui/internal/app"
	"archwiki-tui/internal/storage"
	"archwiki-tui/internal/wiki"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var (
		showVersion  bool
		contextQuery string
		forceOffline bool
		clearCache   bool
		downloadPage string
	)

	flag.BoolVar(&showVersion, "version", false, "print version")
	flag.StringVar(&contextQuery, "context", "", "command context query to auto-open")
	flag.BoolVar(&forceOffline, "offline", false, "open only from local cache")
	flag.BoolVar(&clearCache, "clear-cache", false, "delete local page cache and title index, then exit")
	flag.StringVar(&downloadPage, "download", "", "download one article to local cache for offline reading, then exit")
	flag.Parse()

	if showVersion {
		fmt.Println(app.Version)
		return
	}

	if clearCache {
		if err := clearLocalCache(); err != nil {
			fmt.Fprintf(os.Stderr, "archwiki-tui clear-cache error: %v\n", err)
			os.Exit(1)
		}
	}

	if strings.TrimSpace(downloadPage) != "" {
		if err := downloadArticleForOffline(strings.TrimSpace(downloadPage)); err != nil {
			fmt.Fprintf(os.Stderr, "archwiki-tui download error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if clearCache {
		return
	}

	initialQuery := strings.TrimSpace(strings.Join(flag.Args(), " "))
	cfg := app.Config{
		InitialQuery:   initialQuery,
		ContextCommand: strings.TrimSpace(contextQuery),
		ForceOffline:   forceOffline,
	}

	program := tea.NewProgram(
		app.NewModel(cfg),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "archwiki-tui error: %v\n", err)
		os.Exit(1)
	}
}

func clearLocalCache() error {
	store, err := storage.NewStore("archwiki-tui")
	if err != nil {
		return err
	}

	removedPages, indexRemoved, err := store.ClearCache()
	if err != nil {
		return err
	}

	status := fmt.Sprintf("Cleared cache: %d pages", removedPages)
	if indexRemoved {
		status += " + title index"
	}
	fmt.Println(status)
	return nil
}

func downloadArticleForOffline(title string) error {
	store, err := storage.NewStore("archwiki-tui")
	if err != nil {
		return err
	}

	client := wiki.NewClient(20 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	page, err := client.FetchPage(ctx, title)
	if err != nil {
		return err
	}

	if err := store.WritePageCache(page.Title, page.Markdown); err != nil {
		return err
	}
	_ = store.AddHistory(page.Title, page.URL)

	fmt.Printf("Downloaded for offline reading: %s\n", page.Title)
	return nil
}
