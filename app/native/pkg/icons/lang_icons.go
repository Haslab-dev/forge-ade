package icons

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Colored language & file-type brand SVGs (24x24 viewBox, vector-only, crisp at 12-18px).
// Explicit fill/stroke colors are preserved by MyGo's SVG renderer while fallback TablerFile
// continues to inherit currentColor from the caller's TextColor(tc.IconMuted).
var (
	// TypeScript: official #3178C6 rounded square + white TS letterforms
	LangTypeScript = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><rect x="2" y="2" width="20" height="20" rx="3.5" fill="#3178C6"/><path d="M5.5 11.2h6.2M8.6 11.2v7.3" stroke="#FFFFFF" stroke-width="2" stroke-linecap="round"/><path d="M18.2 12.6c-.5-.8-1.4-1.3-2.5-1.3-1.4 0-2.4.7-2.4 1.8 0 1.2 1 1.6 2.5 2 1.6.4 2.7.9 2.7 2.2 0 1.3-1.1 2.2-2.8 2.2-1.3 0-2.3-.5-2.9-1.4" stroke="#FFFFFF" stroke-width="1.9" stroke-linecap="round"/></svg>`))

	// React TSX: official TypeScript-React blue atom
	LangReactTS = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="2.1" fill="#38BDF8"/><path d="M12 7.2c5.2 0 9.4 2.15 9.4 4.8s-4.2 4.8-9.4 4.8S2.6 14.65 2.6 12 6.8 7.2 12 7.2z" stroke="#38BDF8" stroke-width="1.5"/><path d="M7.8 9.6c2.6-4.5 6.6-7.1 8.9-5.8s2 6-0.6 10.5-6.6 7.1-8.9 5.8-2-6 .6-10.5z" stroke="#38BDF8" stroke-width="1.5"/><path d="M16.2 9.6c-2.6-4.5-6.6-7.1-8.9-5.8s-2 6 .6 10.5 6.6 7.1 8.9 5.8 2-6-.6-10.5z" stroke="#38BDF8" stroke-width="1.5"/></svg>`))

	// React JSX: official React #61DAFB cyan atom
	LangReactJS = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="2.1" fill="#61DAFB"/><path d="M12 7.2c5.2 0 9.4 2.15 9.4 4.8s-4.2 4.8-9.4 4.8S2.6 14.65 2.6 12 6.8 7.2 12 7.2z" stroke="#61DAFB" stroke-width="1.5"/><path d="M7.8 9.6c2.6-4.5 6.6-7.1 8.9-5.8s2 6-0.6 10.5-6.6 7.1-8.9 5.8-2-6 .6-10.5z" stroke="#61DAFB" stroke-width="1.5"/><path d="M16.2 9.6c-2.6-4.5-6.6-7.1-8.9-5.8s-2 6 .6 10.5 6.6 7.1 8.9 5.8 2-6-.6-10.5z" stroke="#61DAFB" stroke-width="1.5"/></svg>`))

	// JavaScript: official #F7DF1E rounded square + dark JS letterforms
	LangJavaScript = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><rect x="2" y="2" width="20" height="20" rx="3.5" fill="#F7DF1E"/><path d="M10.8 11v5.6c0 1.4-.8 2.2-2.1 2.2-1 0-1.7-.4-2.1-1.1" stroke="#18181B" stroke-width="2" stroke-linecap="round"/><path d="M18.4 12.6c-.5-.8-1.3-1.3-2.4-1.3-1.4 0-2.3.7-2.3 1.8 0 1.2.9 1.6 2.4 2 1.5.4 2.5.9 2.5 2.1 0 1.3-1 2.1-2.6 2.1-1.2 0-2.2-.5-2.8-1.4" stroke="#18181B" stroke-width="1.9" stroke-linecap="round"/></svg>`))

	// Go: official #00ADD8 cyan Go emblem
	LangGo = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M2.5 9.5h3.5M1.8 12h3.2M3 14.5h2.8" stroke="#00ADD8" stroke-width="1.6" stroke-linecap="round"/><path d="M12.8 10.2c-.6-1.3-1.9-2.2-3.4-2.2-2.2 0-3.9 1.8-3.9 4s1.7 4 3.9 4c1.9 0 3.5-1.3 3.8-3.1h-3.4" stroke="#00ADD8" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round"/><circle cx="18.2" cy="12" r="3.8" stroke="#00ADD8" stroke-width="2.1"/></svg>`))

	// Python: two-tone interlocking snakes (#3776AB blue + #FFD43B yellow)
	LangPython = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M11.9 2.5c-3.8 0-3.6 1.6-3.6 1.6v2.1h3.7v.8H6.8S4 6.7 4 10.6s2.4 3.8 2.4 3.8h1.5v-1.9s-.1-2.4 2.4-2.4h3.9s2.2 0 2.2-2.1V4.6s.3-2.1-4.5-2.1z" fill="#3776AB"/><circle cx="9.8" cy="4.6" r="0.9" fill="#FFFFFF"/><path d="M12.1 21.5c3.8 0 3.6-1.6 3.6-1.6v-2.1H12V17h5.2s2.8.3 2.8-3.6-2.4-3.8-2.4-3.8h-1.5v1.9s.1 2.4-2.4 2.4H9.8s-2.2 0-2.2 2.1v3.4s-.3 2.1 4.5 2.1z" fill="#FFD43B"/><circle cx="14.2" cy="19.4" r="0.9" fill="#18181B"/></svg>`))

	// Rust: #DEA584 / #CE422B gear with R
	LangRust = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="7.5" stroke="#DEA584" stroke-width="2.2"/><path d="M12 2.5v2.2M12 19.3v2.2M2.5 12h2.2M19.3 12h2.2M5.3 5.3l1.6 1.6M17.1 17.1l1.6 1.6M18.7 5.3l-1.6 1.6M6.9 17.1l-1.6 1.6" stroke="#DEA584" stroke-width="2" stroke-linecap="round"/><path d="M9.5 15.5v-7h3.2c1.3 0 2.2.8 2.2 2s-.9 2-2.2 2H9.5M12.8 12.5l2.2 3" stroke="#CE422B" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`))

	// Swift: official #F05138 swift bird
	LangSwift = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><rect x="2" y="2" width="20" height="20" rx="4.5" fill="#F05138"/><path d="M17.4 15.6c.8-2.2.3-5.1-1.9-7.4-.6-.6-1.3-1.2-2-1.6 1.2 1.7 1.5 3.8.8 5.4-1.8-1.3-4.1-3-6.1-4.8 1.2 2 2.9 4 4.7 5.4-1.7-.4-3.7-1.4-5.4-2.8 1.3 2.5 3.7 4.6 6.7 5.3 1.3.3 2.6.2 3.7-.2.7.5 1.3 1 1.6 1.6.1-.4-.4-1.6-2.1-.9z" fill="#FFFFFF"/></svg>`))

	// Kotlin: tri-colored K prism (#7F52FF, #C711E1, #E44857)
	LangKotlin = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M3 3h18L12 12l9 9H3V3z" fill="#7F52FF"/><path d="M3 3h9.5L3 12.5V3z" fill="#0095D5"/><path d="M12.5 3H21L3 21v-8.5L12.5 3z" fill="#E44857"/></svg>`))

	// Dart: two-tone blue Dart crystal (#0175C2 + #13B9FD)
	LangDart = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M6.5 6.5L13 3l7.5 7.5V18h-7.5L6.5 11.5v-5z" fill="#0175C2"/><path d="M6.5 6.5h7L20.5 13.5V18L13 20.5 6.5 14V6.5z" fill="#13B9FD"/><path d="M3.5 11.5L6.5 6.5V14l6.5 6.5H7.5L3.5 16v-4.5z" fill="#2CB7F6"/></svg>`))

	// HTML5: official #E34F26 shield + white 5
	LangHTML = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M4 3l1.5 16.5L12 21.5l6.5-2L20 3H4z" fill="#E34F26"/><path d="M12 4.5v15.4l5.2-1.6L18.4 4.5H12z" fill="#EF652A"/><path d="M16.2 8.2H8.4l.4 3.5h6.9l-.5 4.7-3.2 1-3.2-1-.2-2" stroke="#FFFFFF" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>`))

	// CSS3: official #1572B6 shield + white 3
	LangCSS = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M4 3l1.5 16.5L12 21.5l6.5-2L20 3H4z" fill="#1572B6"/><path d="M12 4.5v15.4l5.2-1.6L18.4 4.5H12z" fill="#33A9DC"/><path d="M8.3 8.2h7.6l-.5 3.6H9.8M15.4 11.8l-.5 4.4-2.9 1-2.9-1-.2-1.7" stroke="#FFFFFF" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>`))

	// SCSS / Sass: #CC6699 pink S-curl
	LangSCSS = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="9.5" fill="#CC6699"/><path d="M15.5 8.5c-.6-.9-1.8-1.4-3.2-1.4-2 0-3.5 1.1-3.5 2.5 0 1.5 1.3 2.1 3.3 2.6 2.1.5 3.4 1.2 3.4 2.7 0 1.6-1.5 2.7-3.7 2.7-1.6 0-2.9-.6-3.6-1.6" stroke="#FFFFFF" stroke-width="2" stroke-linecap="round"/></svg>`))

	// Vue: #41B883 green outer V + #35495E dark inner V
	LangVue = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M2.5 4h4.2L12 13.4 17.3 4h4.2L12 20.5 2.5 4z" fill="#41B883"/><path d="M6.7 4h3.2L12 7.8 14.1 4h3.2L12 13.4 6.7 4z" fill="#35495E"/></svg>`))

	// Svelte: #FF3E00 S-flame
	LangSvelte = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M17.8 4.8c-1.8-2.4-5.2-3-7.6-1.3L6.4 6.1c-1.8 1.2-2.6 3.5-2 5.6.3 1 .9 1.8 1.7 2.4-.5.9-.7 2-.4 3.1.6 2.4 2.9 3.9 5.4 3.6 1-.1 1.9-.5 2.7-1.1l3.8-2.6c1.8-1.2 2.6-3.5 2-5.6-.3-1-.9-1.8-1.7-2.4.5-.9.7-2 .4-3.1-.1-.4-.3-.8-.5-1.2z" fill="#FF3E00"/><path d="M10.5 17.8c-1.1.2-2.2-.4-2.6-1.5-.2-.6-.1-1.3.2-1.8l3.5 2.3c-.3.5-.7.9-1.1 1zm3-11.6c1.1-.2 2.2.4 2.6 1.5.2.6.1 1.3-.2 1.8l-3.5-2.3c.3-.5.7-.9 1.1-1z" fill="#FFFFFF"/></svg>`))

	// JSON: #F59E0B amber curly braces
	LangJSON = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M8.5 4.5c-1.8 0-2.5 1-2.5 2.6v2.3c0 1.3-.8 2.2-2 2.6 1.2.4 2 1.3 2 2.6v2.3c0 1.6.7 2.6 2.5 2.6" stroke="#F59E0B" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round"/><path d="M15.5 4.5c1.8 0 2.5 1 2.5 2.6v2.3c0 1.3.8 2.2 2 2.6-1.2.4-2 1.3-2 2.6v2.3c0 1.6-.7 2.6-2.5 2.6" stroke="#F59E0B" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round"/><circle cx="10.2" cy="12" r="1.2" fill="#FBBF24"/><circle cx="13.8" cy="12" r="1.2" fill="#FBBF24"/></svg>`))

	// Markdown: #519ABA blue badge with M + down arrow
	LangMarkdown = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><rect x="2" y="4.5" width="20" height="15" rx="2.5" stroke="#519ABA" stroke-width="1.9"/><path d="M5.5 15.5v-7l2.8 3.2L11.1 8.5v7" stroke="#519ABA" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"/><path d="M16.5 8.5v6.5M14.2 12.8l2.3 2.5 2.3-2.5" stroke="#519ABA" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"/></svg>`))

	// YAML: #F43F5E rose-red structured key-value lines
	LangYAML = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M5 6l4 4.5L13 6M9 10.5V18" stroke="#F43F5E" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round"/><path d="M15 11h5M15 14.5h4M15 18h5" stroke="#FB7185" stroke-width="2" stroke-linecap="round"/></svg>`))

	// TOML / Config: #FB923C orange brackets
	LangTOML = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M7.5 4.5H5v15h2.5M16.5 4.5H19v15h-2.5" stroke="#9CA3AF" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M8.8 8.5h6.4M12 8.5v7.5" stroke="#FB923C" stroke-width="2.2" stroke-linecap="round"/></svg>`))

	// Shell / Bash / Makefile: #4EAA25 green terminal box
	LangShell = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><rect x="2.5" y="4" width="19" height="16" rx="2.8" stroke="#4EAA25" stroke-width="1.9"/><path d="M6.8 9.2l3.4 2.8-3.4 2.8" stroke="#4EAA25" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M12.5 14.8h4.5" stroke="#86EFAC" stroke-width="2" stroke-linecap="round"/></svg>`))

	// Docker: #2496ED whale + container blocks
	LangDocker = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><rect x="6" y="7" width="2.8" height="2.6" rx="0.4" fill="#2496ED"/><rect x="9.6" y="7" width="2.8" height="2.6" rx="0.4" fill="#2496ED"/><rect x="13.2" y="7" width="2.8" height="2.6" rx="0.4" fill="#2496ED"/><rect x="9.6" y="3.8" width="2.8" height="2.6" rx="0.4" fill="#38BDF8"/><rect x="6" y="10.2" width="2.8" height="2.6" rx="0.4" fill="#2496ED"/><rect x="9.6" y="10.2" width="2.8" height="2.6" rx="0.4" fill="#2496ED"/><rect x="13.2" y="10.2" width="2.8" height="2.6" rx="0.4" fill="#2496ED"/><path d="M2.5 13.8h15.2c1.2-.9 2.5-.9 3.5-.2-.3 1.4-1.2 2.4-2.5 2.7-1.1 2.7-4 4.2-7.8 4.2-5 0-8.4-2.6-8.4-6.7z" fill="#2496ED"/></svg>`))

	// SQL / Database: #38BDF8 / #336791 blue cylinder
	LangSQL = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M12 4.5c4.4 0 8 1.3 8 3s-3.6 3-8 3-8-1.3-8-3 3.6-3 8-3z" fill="#38BDF8" fill-opacity="0.22" stroke="#38BDF8" stroke-width="1.8"/><path d="M4 7.5v9c0 1.7 3.6 3 8 3s8-1.3 8-3v-9" stroke="#38BDF8" stroke-width="1.8"/><path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3" stroke="#38BDF8" stroke-width="1.8"/></svg>`))

	// Ruby: #CC342D faceted gem
	LangRuby = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M6.5 4.5h11L21 9.5 12 20 3 9.5l3.5-5z" fill="#CC342D"/><path d="M6.5 4.5L9 9.5h6l2.5-5M3 9.5h18M9 9.5L12 20l3-10.5" stroke="#FCA5A5" stroke-width="1.2" stroke-linejoin="round"/></svg>`))

	// PHP: #777BB4 indigo ellipse
	LangPHP = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M12 5.5c5.5 0 10 2.9 10 6.5s-4.5 6.5-10 6.5S2 15.6 2 12s4.5-6.5 10-6.5z" fill="#777BB4"/><path d="M7.5 14.5v-5h2.2c1 0 1.6.6 1.6 1.5s-.6 1.5-1.6 1.5H7.5M13.5 9.5v5M13.5 11.8h2.8M16.3 9.5v5" stroke="#FFFFFF" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>`))

	// Java: #ED8B00 orange coffee cup + steam
	LangJava = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M10 3.5c0 1.5 1.5 1.8 1.5 3.3M14 3.5c0 1.5 1.5 1.8 1.5 3.3" stroke="#F87171" stroke-width="1.7" stroke-linecap="round"/><path d="M5 10h11v5.5a4.5 4.5 0 0 1-4.5 4.5h-2A4.5 4.5 0 0 1 5 15.5V10z" stroke="#ED8B00" stroke-width="1.9" stroke-linejoin="round"/><path d="M16 11.5h2a2 2 0 0 1 0 4h-2" stroke="#ED8B00" stroke-width="1.9" stroke-linecap="round"/></svg>`))

	// C: #5C6BC0 blue hexagon + C
	LangC = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M12 2.5l8.2 4.75v9.5L12 21.5l-8.2-4.75v-9.5L12 2.5z" fill="#5C6BC0"/><path d="M15.2 9.4a4.2 4.2 0 1 0 0 5.2" stroke="#FFFFFF" stroke-width="2.2" stroke-linecap="round"/></svg>`))

	// C++: #00599C deep blue hexagon + C+
	LangCPP = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M12 2.5l8.2 4.75v9.5L12 21.5l-8.2-4.75v-9.5L12 2.5z" fill="#00599C"/><path d="M13.2 9.5a3.8 3.8 0 1 0 0 5" stroke="#FFFFFF" stroke-width="2" stroke-linecap="round"/><path d="M15.5 12h3M17 10.5v3" stroke="#FFFFFF" stroke-width="1.7" stroke-linecap="round"/></svg>`))

	// C#: #239120 green hexagon + C#
	LangCSharp = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M12 2.5l8.2 4.75v9.5L12 21.5l-8.2-4.75v-9.5L12 2.5z" fill="#239120"/><path d="M12.8 9.5a3.7 3.7 0 1 0 0 5" stroke="#FFFFFF" stroke-width="2" stroke-linecap="round"/><path d="M15 10.5l-.6 3M17.2 10.5l-.6 3M14.1 11.4h3.6M13.8 12.8h3.6" stroke="#FFFFFF" stroke-width="1.3" stroke-linecap="round"/></svg>`))

	// Zig: #F7A41D amber Z bolt
	LangZig = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M4 7h11l-7 10h12" stroke="#F7A41D" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/><path d="M4 7v6M20 11v6" stroke="#F7A41D" stroke-width="2.2" stroke-linecap="round"/></svg>`))

	// Lua: #3B82F6 blue planet + moon
	LangLua = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><circle cx="11.5" cy="12.5" r="7.5" fill="#3B82F6"/><circle cx="14.2" cy="9.8" r="2.1" fill="#FFFFFF"/><circle cx="18.8" cy="5.2" r="2" fill="#60A5FA"/></svg>`))

	// GraphQL: #E10098 pink hex + triangle
	LangGraphQL = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M12 3.5l7.4 4.25v8.5L12 20.5l-7.4-4.25v-8.5L12 3.5z" stroke="#E10098" stroke-width="1.6"/><path d="M12 3.5l7.4 12.75H4.6L12 3.5z" stroke="#E10098" stroke-width="1.5"/><circle cx="12" cy="3.5" r="1.5" fill="#E10098"/><circle cx="19.4" cy="7.75" r="1.5" fill="#E10098"/><circle cx="19.4" cy="16.25" r="1.5" fill="#E10098"/><circle cx="12" cy="20.5" r="1.5" fill="#E10098"/><circle cx="4.6" cy="16.25" r="1.5" fill="#E10098"/><circle cx="4.6" cy="7.75" r="1.5" fill="#E10098"/></svg>`))

	// Git: #F05032 git diamond/branch
	LangGit = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M12 2.5L21.5 12 12 21.5 2.5 12 12 2.5z" fill="#F05032"/><circle cx="12" cy="9.5" r="1.5" fill="#FFFFFF"/><circle cx="12" cy="15.5" r="1.5" fill="#FFFFFF"/><circle cx="15.5" cy="12" r="1.5" fill="#FFFFFF"/><path d="M9.5 7l2.5 2.5v6M12 9.5l3.5 2.5" stroke="#FFFFFF" stroke-width="1.6" stroke-linecap="round"/></svg>`))

	// Env / Lock: #EAB308 yellow key/lock
	LangEnv = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><rect x="5" y="11" width="14" height="9.5" rx="2.5" fill="#EAB308" fill-opacity="0.22" stroke="#EAB308" stroke-width="1.9"/><path d="M8.5 11V8a3.5 3.5 0 0 1 7 0v3" stroke="#EAB308" stroke-width="1.9" stroke-linecap="round"/><circle cx="12" cy="15.7" r="1.3" fill="#EAB308"/></svg>`))

	// Image: #A855F7 purple picture frame
	LangImage = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><rect x="3" y="4" width="18" height="16" rx="2.8" fill="#A855F7" fill-opacity="0.16" stroke="#A855F7" stroke-width="1.8"/><circle cx="15.5" cy="8.5" r="1.4" fill="#C084FC"/><path d="M3.8 16.5l4.8-4.8 3.2 3.2 2.8-2.8 5.6 4.4" stroke="#A855F7" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>`))

	// SVG: #FFB13B amber vector diamond
	LangSVG = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M12 3l8 9-8 9-8-9 8-9z" fill="#FFB13B" fill-opacity="0.2" stroke="#FFB13B" stroke-width="1.8" stroke-linejoin="round"/><circle cx="12" cy="12" r="2.2" fill="#FFB13B"/></svg>`))

	// PDF: #EF4444 red document
	LangPDF = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M14 3v4a1 1 0 0 0 1 1h4" stroke="#EF4444" stroke-width="1.8" stroke-linecap="round"/><path d="M17 21H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h7l5 5v11a2 2 0 0 1-2 2z" fill="#EF4444" fill-opacity="0.16" stroke="#EF4444" stroke-width="1.8"/><path d="M8.5 14.5h7M8.5 11.5h4" stroke="#F87171" stroke-width="1.8" stroke-linecap="round"/></svg>`))

	// XML / Plist: #F97316 orange code brackets
	LangXML = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><path d="M7.5 8L3.5 12l4 4M16.5 8l4 4-4 4M13.8 5.5l-3.6 13" stroke="#F97316" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`))
)

// FileIcon returns the colored language/file SVG and its brand accent color for
// a given file path. Unknown file types return TablerFile with fallbackColor.
func FileIcon(path string, fallbackColor ui.Color) (*ui.SVG, ui.Color) {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(base))

	// Exact filename / prefix matches first.
	switch {
	case base == "dockerfile" || strings.HasPrefix(base, "dockerfile.") ||
		base == "docker-compose.yml" || base == "docker-compose.yaml" ||
		base == "compose.yml" || base == "compose.yaml" || base == ".dockerignore":
		return LangDocker, ui.Hex("#2496ED")
	case base == "makefile" || base == "gnumakefile" || base == "justfile":
		return LangShell, ui.Hex("#4EAA25")
	case base == "go.mod" || base == "go.sum" || base == "go.work":
		return LangGo, ui.Hex("#00ADD8")
	case base == "cargo.toml" || base == "cargo.lock":
		return LangRust, ui.Hex("#DEA584")
	case base == ".gitignore" || base == ".gitattributes" || base == ".gitmodules":
		return LangGit, ui.Hex("#F05032")
	case base == ".env" || strings.HasPrefix(base, ".env.") ||
		strings.HasSuffix(base, ".lock") || base == "package-lock.json" ||
		base == "pnpm-lock.yaml":
		return LangEnv, ui.Hex("#EAB308")
	case base == "gemfile" || base == "rakefile":
		return LangRuby, ui.Hex("#CC342D")
	}

	switch ext {
	case ".tsx":
		return LangReactTS, ui.Hex("#38BDF8")
	case ".jsx":
		return LangReactJS, ui.Hex("#61DAFB")
	case ".ts", ".mts", ".cts":
		return LangTypeScript, ui.Hex("#3178C6")
	case ".js", ".mjs", ".cjs":
		return LangJavaScript, ui.Hex("#F7DF1E")
	case ".go":
		return LangGo, ui.Hex("#00ADD8")
	case ".py", ".pyw", ".pyi":
		return LangPython, ui.Hex("#3776AB")
	case ".rs":
		return LangRust, ui.Hex("#DEA584")
	case ".swift":
		return LangSwift, ui.Hex("#F05138")
	case ".kt", ".kts":
		return LangKotlin, ui.Hex("#7F52FF")
	case ".dart":
		return LangDart, ui.Hex("#13B9FD")
	case ".html", ".htm":
		return LangHTML, ui.Hex("#E34F26")
	case ".css":
		return LangCSS, ui.Hex("#1572B6")
	case ".scss", ".sass", ".less":
		return LangSCSS, ui.Hex("#CC6699")
	case ".vue":
		return LangVue, ui.Hex("#41B883")
	case ".svelte":
		return LangSvelte, ui.Hex("#FF3E00")
	case ".json", ".jsonc", ".json5":
		return LangJSON, ui.Hex("#F59E0B")
	case ".md", ".mdx", ".markdown":
		return LangMarkdown, ui.Hex("#519ABA")
	case ".yml", ".yaml":
		return LangYAML, ui.Hex("#F43F5E")
	case ".toml", ".ini", ".cfg", ".conf":
		return LangTOML, ui.Hex("#FB923C")
	case ".sh", ".bash", ".zsh", ".fish":
		return LangShell, ui.Hex("#4EAA25")
	case ".sql", ".db", ".sqlite", ".sqlite3", ".duckdb", ".prisma":
		return LangSQL, ui.Hex("#38BDF8")
	case ".rb":
		return LangRuby, ui.Hex("#CC342D")
	case ".php":
		return LangPHP, ui.Hex("#777BB4")
	case ".java", ".gradle":
		return LangJava, ui.Hex("#ED8B00")
	case ".c", ".h":
		return LangC, ui.Hex("#5C6BC0")
	case ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".mm":
		return LangCPP, ui.Hex("#00599C")
	case ".cs":
		return LangCSharp, ui.Hex("#239120")
	case ".zig", ".zon":
		return LangZig, ui.Hex("#F7A41D")
	case ".lua":
		return LangLua, ui.Hex("#3B82F6")
	case ".graphql", ".gql":
		return LangGraphQL, ui.Hex("#E10098")
	case ".svg":
		return LangSVG, ui.Hex("#FFB13B")
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".ico":
		return LangImage, ui.Hex("#A855F7")
	case ".pdf":
		return LangPDF, ui.Hex("#EF4444")
	case ".xml", ".plist", ".xhtml":
		return LangXML, ui.Hex("#F97316")
	default:
		return TablerFile, fallbackColor
	}
}

// RenderFileIcon renders the colored language/file SVG at the given size using
// ui.Image so explicit multi-color SVG fills/strokes are preserved (unlike
// ui.Icon which renders a monochrome mask). Generic TablerFile icons inherit
// fallbackColor via currentColor.
func RenderFileIcon(c *ui.Context, path string, size float32, fallbackColor ui.Color) ui.Element {
	svg, col := FileIcon(path, fallbackColor)
	return ui.Image(c, svg).Width(size).Height(size).Shrink(0).TextColor(col)
}
