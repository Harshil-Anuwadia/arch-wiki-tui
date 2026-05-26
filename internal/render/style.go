package render

import (
	"github.com/charmbracelet/glamour/ansi"
	glamourStyles "github.com/charmbracelet/glamour/styles"
)

var docsReadingStyle = buildDocsReadingStyle()

// ReadingGlamourStyle returns the shared terminal markdown style used by the app.
func ReadingGlamourStyle() ansi.StyleConfig {
	return docsReadingStyle
}

func buildDocsReadingStyle() ansi.StyleConfig {
	style := glamourStyles.DarkStyleConfig

	style.Document.Margin = uintPtr(0)
	style.Document.StylePrimitive.BlockPrefix = ""
	style.Document.StylePrimitive.BlockSuffix = ""

	style.Heading.StylePrimitive.BlockPrefix = "\n"
	style.Heading.StylePrimitive.BlockSuffix = "\n\n"
	style.Heading.StylePrimitive.Color = strPtr("223")
	style.Heading.StylePrimitive.Bold = boolPtr(true)

	style.H1.StylePrimitive.Prefix = "▛ "
	style.H1.StylePrimitive.Suffix = " "
	style.H1.StylePrimitive.BlockPrefix = "\n"
	style.H1.StylePrimitive.BlockSuffix = "\n\n"
	style.H1.StylePrimitive.Color = strPtr("229")
	style.H1.StylePrimitive.Bold = boolPtr(true)
	style.H1.StylePrimitive.Underline = boolPtr(true)

	style.H2.StylePrimitive.Prefix = "▍ "
	style.H2.StylePrimitive.BlockPrefix = "\n"
	style.H2.StylePrimitive.BlockSuffix = "\n"
	style.H2.StylePrimitive.Color = strPtr("222")
	style.H2.StylePrimitive.Bold = boolPtr(true)

	style.H3.StylePrimitive.Prefix = "▎ "
	style.H3.StylePrimitive.BlockPrefix = "\n"
	style.H3.StylePrimitive.BlockSuffix = "\n"
	style.H3.StylePrimitive.Color = strPtr("216")
	style.H3.StylePrimitive.Bold = boolPtr(true)

	style.H4.StylePrimitive.Prefix = "• "
	style.H4.StylePrimitive.BlockPrefix = "\n"
	style.H4.StylePrimitive.BlockSuffix = "\n"
	style.H4.StylePrimitive.Color = strPtr("250")
	style.H4.StylePrimitive.Bold = boolPtr(true)

	style.H5.StylePrimitive.Prefix = "· "
	style.H5.StylePrimitive.BlockPrefix = "\n"
	style.H5.StylePrimitive.BlockSuffix = "\n"
	style.H5.StylePrimitive.Color = strPtr("248")
	style.H5.StylePrimitive.Bold = boolPtr(false)

	style.H6.StylePrimitive.Prefix = "· "
	style.H6.StylePrimitive.BlockPrefix = "\n"
	style.H6.StylePrimitive.BlockSuffix = "\n"
	style.H6.StylePrimitive.Color = strPtr("246")
	style.H6.StylePrimitive.Bold = boolPtr(false)

	style.HorizontalRule.Format = "\n────────────────────────────────────────\n"
	style.HorizontalRule.Color = strPtr("240")

	style.BlockQuote.Indent = uintPtr(0)
	style.BlockQuote.IndentToken = strPtr("┃ ")
	style.BlockQuote.StylePrimitive.BlockPrefix = "\n"
	style.BlockQuote.StylePrimitive.BlockSuffix = "\n"
	style.BlockQuote.StylePrimitive.Color = strPtr("216")
	style.BlockQuote.StylePrimitive.Bold = boolPtr(true)

	style.Code.StylePrimitive.Prefix = " "
	style.Code.StylePrimitive.Suffix = " "
	style.Code.StylePrimitive.Color = strPtr("222")

	style.CodeBlock.StyleBlock.Margin = uintPtr(0)
	style.CodeBlock.StyleBlock.Indent = uintPtr(0)
	style.CodeBlock.StyleBlock.StylePrimitive.BlockPrefix = "\n"
	style.CodeBlock.StyleBlock.StylePrimitive.BlockSuffix = "\n"

	style.List.StyleBlock.Margin = uintPtr(0)
	style.List.LevelIndent = 4
	style.Item.BlockPrefix = "• "
	style.Enumeration.BlockPrefix = ". "

	style.Table.StyleBlock.Margin = uintPtr(0)
	style.Table.StyleBlock.StylePrimitive.BlockPrefix = "\n"
	style.Table.StyleBlock.StylePrimitive.BlockSuffix = "\n"
	style.Table.ColumnSeparator = strPtr("│")
	style.Table.RowSeparator = strPtr("─")
	style.Table.CenterSeparator = strPtr("┼")

	style.LinkText.Color = strPtr("81")
	style.LinkText.Bold = boolPtr(true)
	style.Link.Color = strPtr("81")
	style.Link.Underline = boolPtr(true)

	return style
}

func boolPtr(v bool) *bool {
	return &v
}

func strPtr(v string) *string {
	return &v
}

func uintPtr(v uint) *uint {
	return &v
}
