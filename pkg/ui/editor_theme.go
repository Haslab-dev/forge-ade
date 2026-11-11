package ui

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"
)

// Editor themes ported 1:1 from ForgeADE's cmSetup.ts: the light
// HighlightStyle and @codemirror/theme-one-dark for the dark appearance,
// on top of the editor surface colors of its lightTheme/darkTheme objects.

// EditorCodeFontSize is the default code font size of ForgeADE
// (--font-size-code: 0.8125rem at a 16px root).
const EditorCodeFontSize = 13

// ForgeEditorTheme builds the code editor/diff theme for one appearance.
// diffScheme is "classic" (green/red) or "modern" (blue/orange), matching
// the [data-diff-color-scheme] variants in index.css.
func ForgeEditorTheme(tc ThemeColors, dark bool, diffScheme string) *editor.Theme {
	th := editor.DefaultTheme(MapTheme(tc, dark))

	// cmSetup.ts lightTheme/darkTheme base colors.
	if !dark {
		th.Background = tc.Canvas
		th.Text = ui.Hex("#111827")
		th.LineNumber = ui.Hex("#9ca3af")
		th.ActiveLineNumber = ui.Hex("#111827")
		th.ActiveLine = ui.RGBA(37, 99, 235, 0.06)
		th.Selection = ui.RGBA(191, 219, 254, 0.7)
		th.Cursor = ui.Hex("#2563eb")
	} else {
		th.Background = tc.Canvas
		th.Text = ui.Hex("#e2e8f0")
		th.LineNumber = ui.Hex("#555555")
		th.ActiveLineNumber = ui.Hex("#ffffff")
		th.ActiveLine = ui.RGBA(56, 189, 248, 0.07)
		th.Selection = ui.RGBA(38, 79, 120, 0.7)
		th.Cursor = ui.Hex("#38bdf8")
	}
	th.InactiveSelection = th.Selection.Alpha(0.55)
	// Search matches, as VS Code: yellow, chosen one stronger amber.
	if dark {
		th.Match = ui.RGBA(234, 179, 8, 0.4)
	} else {
		th.Match = ui.RGBA(234, 179, 8, 0.35)
	}
	th.CurrentMatch = ui.RGBA(245, 158, 11, 0.85)

	// Diff change colors: classic green/red or modern blue/orange
	// (--diff-color-scheme). Modified stays amber for both.
	if diffScheme == "modern" {
		if dark {
			th.Added = ui.Hex("#60a5fa")   // blue-400
			th.Deleted = ui.Hex("#fb923c") // orange-400
		} else {
			th.Added = ui.Hex("#2563eb")   // blue-600
			th.Deleted = ui.Hex("#ea580c") // orange-600
		}
	} else {
		th.Added = tc.DiffAddition
		th.Deleted = tc.DiffDeletion
	}
	th.Modified = tc.Warning

	// Token colors. Light: ForgeADE's lightHighlightStyle. Dark: One Dark
	// (the same mapping @codemirror/theme-one-dark applies to these kinds).
	set := func(k editor.TokenKind, hex string, styles ...func(*editor.TokenStyle)) {
		s := &th.Tokens[k]
		s.Color = ui.Hex(hex)
		for _, fn := range styles {
			fn(s)
		}
	}
	italic := func(s *editor.TokenStyle) { s.Italic = true }
	bold := func(s *editor.TokenStyle) { s.Bold = true }
	underline := func(s *editor.TokenStyle) { s.Italic = true } // links read as links; underline unsupported here
	_ = underline

	if dark {
		// One Dark Pro palette.
		set(editor.TokenText, "#abb2bf")
		set(editor.TokenKeyword, "#c678dd")
		set(editor.TokenType, "#e5c07b")
		set(editor.TokenFunction, "#61afef")
		set(editor.TokenVariable, "#e06c75")
		set(editor.TokenProperty, "#e06c75")
		set(editor.TokenConstant, "#d19a66")
		set(editor.TokenNumber, "#d19a66")
		set(editor.TokenString, "#98c379")
		set(editor.TokenEscape, "#56b6c2")
		set(editor.TokenRegexp, "#98c379")
		set(editor.TokenComment, "#5c6370", italic)
		set(editor.TokenOperator, "#56b6c2")
		set(editor.TokenPunctuation, "#abb2bf")
		set(editor.TokenTag, "#e06c75")
		set(editor.TokenAttribute, "#d19a66")
		set(editor.TokenMeta, "#abb2bf", italic)
		set(editor.TokenHeading, "#d19a66", bold)
		set(editor.TokenEmphasis, "#abb2bf", italic)
		set(editor.TokenStrong, "#abb2bf", bold)
		set(editor.TokenLink, "#61afef")
		set(editor.TokenCode, "#98c379")
		set(editor.TokenInserted, "#98c379")
		set(editor.TokenDeleted, "#e06c75")
		set(editor.TokenInvalid, "#ffffff")
	} else {
		// lightHighlightStyle.
		set(editor.TokenText, "#0f172a")
		set(editor.TokenKeyword, "#7c3aed")
		set(editor.TokenType, "#b45309")
		set(editor.TokenFunction, "#2563eb")
		set(editor.TokenVariable, "#0f172a")
		set(editor.TokenProperty, "#0f172a")
		set(editor.TokenConstant, "#0369a1")
		set(editor.TokenNumber, "#b45309")
		set(editor.TokenString, "#047857")
		set(editor.TokenEscape, "#0e7490")
		set(editor.TokenRegexp, "#0e7490")
		set(editor.TokenComment, "#6b7280")
		set(editor.TokenOperator, "#0e7490")
		set(editor.TokenPunctuation, "#0f172a")
		set(editor.TokenTag, "#0f172a")
		set(editor.TokenAttribute, "#b45309")
		set(editor.TokenMeta, "#6b7280")
		set(editor.TokenHeading, "#111827", bold)
		set(editor.TokenEmphasis, "#0f172a", italic)
		set(editor.TokenStrong, "#0f172a", bold)
		set(editor.TokenLink, "#2563eb")
		set(editor.TokenCode, "#047857")
		set(editor.TokenInserted, "#047857")
		set(editor.TokenDeleted, "#dc2626")
		set(editor.TokenInvalid, "#dc2626")
	}
	return th
}

// ForgeMarkdownTheme builds the chat Markdown theme, mirroring
// MarkdownRenderer.tsx: 15px body (--app-font-size-chat), Geist/Geist Mono,
// heading scale text-ui-2xl/xl/lg/base, info links, --code background code
// blocks.
func ForgeMarkdownTheme(tc ThemeColors, dark bool, codeColors *editor.Theme, codeFontSize float32, fontFamily, monoFamily string) *markdown.Theme {
	if codeFontSize <= 0 {
		codeFontSize = EditorCodeFontSize
	}
	th := &markdown.Theme{
		FontSize:            15, // --app-font-size-chat: 0.9375rem
		Font:                fontFamily,
		CodeFont:            monoFamily,
		LineHeight:          1.55,
		CodeLineHeight:      1.6, // leading-[1.6] on chat code blocks
		Text:                tc.Text,
		Muted:               tc.TextMuted,
		Link:                tc.Info,
		Strong:              tc.Text,
		Code:                tc.CodeForeground,
		CodeBackground:      tc.CodeBackground,
		CodeSize:            codeFontSize / 15, // in ems of body text
		BlockSpacing:        0.75,
		ItemSpacing:         0.25,
		ListIndent:          1.6,
		QuoteBar:            tc.Border,
		QuoteText:           tc.TextMuted,
		Rule:                tc.Border,
		CodeBlockBackground: tc.CodeBackground,
		CodeBlockBorder:     tc.Border,
		CodeBlockRadius:     8, // rounded-lg
		CodeColors:          codeColors,
		TableBorder:         tc.Border,
		TableHeader:         tc.ToolbarControlHover,
		Mark:                tc.SearchMatchBackground,
		Checkbox:            tc.Accent,
	}
	// MarkdownRenderer heading scale: h1 text-ui-2xl bold, h2 text-ui-xl
	// semibold, h3 text-ui-lg semibold, h4 text-ui-base semibold.
	th.Headings = [6]markdown.HeadingStyle{
		{Size: 26.0 / 15.0, Weight: 700},
		{Size: 22.0 / 15.0, Weight: 600},
		{Size: 17.0 / 15.0, Weight: 600},
		{Size: 14.0 / 15.0, Weight: 600},
		{Size: 14.0 / 15.0, Weight: 600},
		{Size: 14.0 / 15.0, Weight: 600},
	}
	return th
}
