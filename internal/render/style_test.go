package render

import "testing"

func TestReadingStyleConfiguresTableSeparators(t *testing.T) {
	style := ReadingGlamourStyle()

	if style.Table.ColumnSeparator == nil || *style.Table.ColumnSeparator != "│" {
		t.Fatalf("expected table column separator to be set to │")
	}
	if style.Table.RowSeparator == nil || *style.Table.RowSeparator != "─" {
		t.Fatalf("expected table row separator to be set to ─")
	}
	if style.Table.CenterSeparator == nil || *style.Table.CenterSeparator != "┼" {
		t.Fatalf("expected table center separator to be set to ┼")
	}

	if style.H1.StylePrimitive.Prefix != "▛ " {
		t.Fatalf("expected H1 prefix to emphasize article anchor")
	}
	if style.H2.StylePrimitive.Prefix != "▍ " {
		t.Fatalf("expected H2 prefix to emphasize section heading")
	}
	if style.H3.StylePrimitive.Prefix != "▎ " {
		t.Fatalf("expected H3 prefix to emphasize subsection heading")
	}

	if style.H1.StylePrimitive.Bold == nil || !*style.H1.StylePrimitive.Bold {
		t.Fatalf("expected H1 to be bold")
	}
	if style.BlockQuote.IndentToken == nil || *style.BlockQuote.IndentToken != "┃ " {
		t.Fatalf("expected blockquote indent token to be strong callout marker")
	}
	if style.List.LevelIndent != 4 {
		t.Fatalf("expected list level indent to increase hierarchy readability")
	}
}
