package ui

import "github.com/charmbracelet/lipgloss"

// Styles defines the visual language for the TUI.
type Styles struct {
	App              lipgloss.Style
	Header           lipgloss.Style
	HeaderTitle      lipgloss.Style
	HeaderMeta       lipgloss.Style
	PanelFocused     lipgloss.Style
	PanelTitle       lipgloss.Style
	Item             lipgloss.Style
	ItemSelected     lipgloss.Style
	CodeHint         lipgloss.Style
	Spinner          lipgloss.Style
	InputPrompt      lipgloss.Style
	InputText        lipgloss.Style
	InputPlaceholder lipgloss.Style
	Status           lipgloss.Style
	StatusLeft       lipgloss.Style
	StatusCenter     lipgloss.Style
	StatusRight      lipgloss.Style
	Modal            lipgloss.Style
	Dim              lipgloss.Style
}

// NewStyles returns theme-safe ANSI based styles.
func NewStyles() Styles {
	const (
		text   = "255"
		muted  = "246"
		border = "166"
		accent = "214"
		warn   = "220"
	)

	return Styles{
		App: lipgloss.NewStyle().
			Foreground(lipgloss.Color(text)),
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(border)).
			Padding(0, 1),
		HeaderTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accent)),
		HeaderMeta:  lipgloss.NewStyle().Foreground(lipgloss.Color(muted)),
		PanelFocused: lipgloss.NewStyle().
			Foreground(lipgloss.Color(text)).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(border)).
			Padding(0, 1),
		PanelTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accent)),
		Item:       lipgloss.NewStyle().Foreground(lipgloss.Color(text)),
		ItemSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(accent)).
			Bold(true),
		CodeHint: lipgloss.NewStyle().Foreground(lipgloss.Color(warn)).Bold(true),
		Spinner:  lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true),
		InputPrompt: lipgloss.NewStyle().
			Foreground(lipgloss.Color(accent)).
			Bold(true),
		InputText: lipgloss.NewStyle().Foreground(lipgloss.Color(text)),
		InputPlaceholder: lipgloss.NewStyle().
			Foreground(lipgloss.Color(muted)).
			Italic(true),
		Status: lipgloss.NewStyle().
			Foreground(lipgloss.Color(text)).
			BorderTop(true).
			BorderForeground(lipgloss.Color(border)).
			Padding(0, 1),
		StatusLeft:   lipgloss.NewStyle().Foreground(lipgloss.Color(text)),
		StatusCenter: lipgloss.NewStyle().Foreground(lipgloss.Color(accent)),
		StatusRight:  lipgloss.NewStyle().Foreground(lipgloss.Color(muted)),
		Modal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(border)).
			Padding(1, 2).
			Foreground(lipgloss.Color(text)),
		Dim: lipgloss.NewStyle().Foreground(lipgloss.Color(muted)),
	}
}
