package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

// tablerSVG wraps SVG path data in Tabler's 24x24 viewBox stroke template.
func tablerSVG(paths string) *ui.SVG {
	svgStr := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">%s</svg>`, paths)
	return ui.MustParseSVG([]byte(svgStr))
}

// Tabler Icons Definition Collection (https://tabler.io/icons)
var (
	TablerSparkles     = tablerSVG(`<path d="M16 18a2 2 0 0 1 2 2a2 2 0 0 1 2 -2a2 2 0 0 1 -2 -2a2 2 0 0 1 -2 2zm0 -12a2 2 0 0 1 2 2a2 2 0 0 1 2 -2a2 2 0 0 1 -2 -2a2 2 0 0 1 -2 2zm-7 1a6 6 0 0 1 6 6a6 6 0 0 1 -6 6a6 6 0 0 1 -6 -6a6 6 0 0 1 6 -6z"/>`)
	TablerCode         = tablerSVG(`<path d="M7 8l-4 4l4 4m10 -8l4 4l-4 4m-4 -10l-4 12"/>`)
	TablerPanelLeft    = tablerSVG(`<path d="M4 4m0 2a2 2 0 0 1 2 -2h12a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2z"/><path d="M9 4v16"/>`)
	TablerPanelRight   = tablerSVG(`<path d="M4 4m0 2a2 2 0 0 1 2 -2h12a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2z"/><path d="M15 4v16"/>`)
	TablerTerminal     = tablerSVG(`<path d="M8 9l3 3l-3 3"/><path d="M13 15l3 0"/><path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z"/>`)
	TablerFolder       = tablerSVG(`<path d="M5 4h4l3 3h7a2 2 0 0 1 2 2v8a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2v-11a2 2 0 0 1 2 -2"/>`)
	TablerFolderPlus   = tablerSVG(`<path d="M12 19h-7a2 2 0 0 1 -2 -2v-11a2 2 0 0 1 2 -2h4l3 3h7a2 2 0 0 1 2 2v3.5"/><path d="M16 19h6"/><path d="M19 16v6"/>`)
	TablerFiles        = tablerSVG(`<path d="M15 3v4a1 1 0 0 0 1 1h4"/><path d="M18 17h-7a2 2 0 0 1 -2 -2v-10a2 2 0 0 1 2 -2h4l5 5v7a2 2 0 0 1 -2 2z"/><path d="M16 20.334a1.998 1.998 0 0 1 -1 0.666h-7a2 2 0 0 1 -2 -2v-10a2 2 0 0 1 0.666 -1"/>`)
	TablerFile         = tablerSVG(`<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z"/>`)
	TablerFileCode     = tablerSVG(`<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z"/><path d="M10 13l-1 2l1 2"/><path d="M14 13l1 2l-1 2"/>`)
	TablerFileText     = tablerSVG(`<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z"/><path d="M9 9l1 0"/><path d="M9 13l6 0"/><path d="M9 17l6 0"/>`)
	TablerFilePlus     = tablerSVG(`<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z"/><path d="M12 11l0 6"/><path d="M9 14l6 0"/>`)
	TablerFileDiff     = tablerSVG(`<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z"/><path d="M9 11l6 0"/><path d="M12 8l0 6"/><path d="M9 17l6 0"/>`)
	TablerGitBranch    = tablerSVG(`<path d="M7 18m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M7 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M17 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M7 8l0 8"/><path d="M9 18h1a4 4 0 0 0 4 -4v-2a4 4 0 0 1 4 -4h1"/>`)
	TablerGitCommit    = tablerSVG(`<path d="M12 12m-3 0a3 3 0 1 0 6 0a3 3 0 1 0 -6 0"/><path d="M12 3l0 6"/><path d="M12 15l0 6"/>`)
	TablerGitFork      = tablerSVG(`<path d="M12 18m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M7 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M17 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M7 8v2a2 2 0 0 0 2 2h6a2 2 0 0 0 2 -2v-2"/><path d="M12 12l0 4"/>`)
	TablerGitCompare   = tablerSVG(`<path d="M6 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M18 18m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M11 6h5a2 2 0 0 1 2 2v8"/><path d="M14 9l-3 -3l3 -3"/><path d="M13 18h-5a2 2 0 0 1 -2 -2v-8"/><path d="M10 15l3 3l-3 3"/>`)
	TablerGitPullRequest = tablerSVG(`<path d="M6 18m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M6 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M18 18m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M6 8v8"/><path d="M11 6h4a2 2 0 0 1 2 2v8"/><path d="M14 9l-3 -3l3 -3"/>`)
	TablerDatabase     = tablerSVG(`<path d="M12 6m-8 0a8 3 0 1 0 16 0a8 3 0 1 0 -16 0"/><path d="M4 6v6a8 3 0 0 0 16 0v-6"/><path d="M4 12v6a8 3 0 0 0 16 0v-6"/>`)
	TablerSearch       = tablerSVG(`<path d="M10 10m-7 0a7 7 0 1 0 14 0a7 7 0 1 0 -14 0"/><path d="M21 21l-6 -6"/>`)
	TablerSettings     = tablerSVG(`<path d="M10.325 4.317c.426 -1.756 2.924 -1.756 3.35 0a1.724 1.724 0 0 0 2.573 1.066c1.543 -.94 3.31 .826 2.37 2.37a1.724 1.724 0 0 0 1.065 2.572c1.756 .426 1.756 2.924 0 3.35a1.724 1.724 0 0 0 -1.066 2.573c.94 1.543 -.826 3.31 -2.37 2.37a1.724 1.724 0 0 0 -2.572 1.065c-.426 1.756 -2.924 1.756 -3.35 0a1.724 1.724 0 0 0 -2.573 -1.066c-1.543 .94 -3.31 -.826 -2.37 -2.37a1.724 1.724 0 0 0 -1.065 -2.572c-1.756 -.426 -1.756 -2.924 0 -3.35a1.724 1.724 0 0 0 1.066 -2.573c-.94 -1.543 .826 -3.31 2.37 -2.37c1 .608 2.296 .07 2.572 -1.065z"/><path d="M9 12a3 3 0 1 0 6 0a3 3 0 0 0 -6 0"/>`)
	TablerPalette      = tablerSVG(`<path d="M12 21a9 9 0 0 1 0 -18c4.97 0 9 3.582 9 8c0 1.06 -.474 2.078 -1.318 2.828c-.844 .75 -1.989 1.172 -3.182 1.172h-2.5a2 2 0 0 0 -1 3.75a1.3 1.3 0 0 1 -1 2.25"/><path d="M8.5 10.5m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M12.5 7.5m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M16.5 10.5m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/>`)
	TablerPlus         = tablerSVG(`<path d="M12 5l0 14"/><path d="M5 12l14 0"/>`)
	TablerMinus        = tablerSVG(`<path d="M5 12l14 0"/>`)
	TablerX            = tablerSVG(`<path d="M18 6l-12 12"/><path d="M6 6l12 12"/>`)
	TablerCheck        = tablerSVG(`<path d="M5 12l5 5l10 -10"/>`)
	TablerCircleCheck  = tablerSVG(`<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"/><path d="M9 12l2 2l4 -4"/>`)
	TablerCircle       = tablerSVG(`<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"/>`)
	TablerCircleDot    = tablerSVG(`<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"/><path d="M12 12m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/>`)
	TablerLoader       = tablerSVG(`<path d="M12 3a9 9 0 1 0 9 9"/>`)
	TablerPin          = tablerSVG(`<path d="M15 4.5l-4 4l-4 1.5l-1.5 1.5l7 7l1.5 -1.5l1.5 -4l4 -4"/><path d="M9 15l-4.5 4.5"/><path d="M14.5 4l5.5 5.5"/>`)
	TablerPinFilled    = tablerSVG(`<path d="M15 4.5l-4 4l-4 1.5l-1.5 1.5l7 7l1.5 -1.5l1.5 -4l4 -4z" fill="currentColor"/><path d="M9 15l-4.5 4.5"/><path d="M14.5 4l5.5 5.5"/>`)
	TablerTrash        = tablerSVG(`<path d="M4 7l16 0"/><path d="M10 11l0 6"/><path d="M14 11l0 6"/><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12"/><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3"/>`)
	TablerPencil       = tablerSVG(`<path d="M4 20h4l10.5 -10.5a2.828 2.828 0 1 0 -4 -4l-10.5 10.5v4"/><path d="M13.5 6.5l4 4"/>`)
	TablerDots         = tablerSVG(`<path d="M5 12m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M12 12m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M19 12m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/>`)
	TablerDeviceFloppy = tablerSVG(`<path d="M6 4h10l4 4v10a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2v-12a2 2 0 0 1 2 -2"/><path d="M12 14m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M14 4l0 4l-6 0l0 -4"/>`)
	TablerColumns      = tablerSVG(`<path d="M4 4m0 2a2 2 0 0 1 2 -2h12a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2z"/><path d="M12 4l0 16"/>`)
	TablerSend         = tablerSVG(`<path d="M10 14l11 -11"/><path d="M21 3l-6.5 18a.55 .55 0 0 1 -1 0l-3.5 -7l-7 -3.5a.55 .55 0 0 1 0 -1l18 -6.5"/>`)
	TablerSquare       = tablerSVG(`<path d="M4 4m0 2a2 2 0 0 1 2 -2h12a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2z"/>`)
	TablerBrain        = tablerSVG(`<path d="M15.5 13a3.5 3.5 0 0 0 -3.5 3.5v1a3.5 3.5 0 0 0 7 0v-1.8"/><path d="M8.5 13a3.5 3.5 0 0 1 3.5 3.5v1a3.5 3.5 0 0 1 -7 0v-1.8"/><path d="M17.5 16a3.5 3.5 0 0 0 0 -7h-.5"/><path d="M19 9.3v-2.8a3.5 3.5 0 0 0 -7 0"/><path d="M6.5 16a3.5 3.5 0 0 1 0 -7h.5"/><path d="M5 9.3v-2.8a3.5 3.5 0 0 1 7 0"/>`)
	TablerCpu          = tablerSVG(`<path d="M5 5m0 1a1 1 0 0 1 1 -1h12a1 1 0 0 1 1 1v12a1 1 0 0 1 -1 1h-12a1 1 0 0 1 -1 -1z"/><path d="M9 9h6v6h-6z"/><path d="M3 10h2"/><path d="M3 14h2"/><path d="M10 3v2"/><path d="M14 3v2"/><path d="M21 10h-2"/><path d="M21 14h-2"/><path d="M14 21v-2"/><path d="M10 21v-2"/>`)
	TablerPlug         = tablerSVG(`<path d="M9.785 6l8.215 8.215l-2.054 2.054a5.81 5.81 0 1 1 -8.215 -8.215l2.054 -2.054z"/><path d="M4 20l3.5 -3.5"/><path d="M15 4l-3.5 3.5"/><path d="M20 9l-3.5 3.5"/>`)
	TablerRobot        = tablerSVG(`<path d="M7 7h10a2 2 0 0 1 2 2v8a2 2 0 0 1 -2 2h-10a2 2 0 0 1 -2 -2v-8a2 2 0 0 1 2 -2z"/><path d="M10 16h4"/><path d="M9 11v.01"/><path d="M15 11v.01"/><path d="M12 7v-3"/><path d="M8 4h8"/>`)
	TablerKeyboard     = tablerSVG(`<path d="M2 5m0 2a2 2 0 0 1 2 -2h16a2 2 0 0 1 2 2v10a2 2 0 0 1 -2 2h-16a2 2 0 0 1 -2 -2z"/><path d="M6 10l0 .01"/><path d="M10 10l0 .01"/><path d="M14 10l0 .01"/><path d="M18 10l0 .01"/><path d="M6 14l0 .01"/><path d="M18 14l0 .01"/><path d="M10 14l4 0"/>`)
	TablerRefresh      = tablerSVG(`<path d="M20 11a8.1 8.1 0 0 0 -15.5 -2m-.5 -5v5h5"/><path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 5v-5h-5"/>`)
	TablerHistory      = tablerSVG(`<path d="M12 8l0 4l2 2"/><path d="M3.05 11a9 9 0 1 1 .5 4m-.5 5v-5h5"/>`)
	TablerTable        = tablerSVG(`<path d="M3 5a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v14a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2v-14z"/><path d="M3 10h18"/><path d="M10 3v18"/>`)
	TablerEye          = tablerSVG(`<path d="M10 12a2 2 0 1 0 4 0a2 2 0 0 0 -4 0"/><path d="M21 12c-2.4 4 -5.4 6 -9 6c-3.6 0 -6.6 -2 -9 -6c2.4 -4 5.4 -6 9 -6c3.6 0 6.6 2 9 6"/>`)
	TablerPlayerPlay   = tablerSVG(`<path d="M7 4v16l13 -8z"/>`)
	TablerChecklist    = tablerSVG(`<path d="M9.615 20h-2.615a2 2 0 0 1 -2 -2v-12a2 2 0 0 1 2 -2h8a2 2 0 0 1 2 2v8"/><path d="M14 19l2 2l4 -4"/><path d="M9 8h4"/><path d="M9 12h2"/>`)
	TablerSun          = tablerSVG(`<path d="M12 12m-4 0a4 4 0 1 0 8 0a4 4 0 1 0 -8 0"/><path d="M3 12h1m8 -9v1m8 8h1m-9 8v1m-6.4 -15.4l.7 .7m12.1 -.7l-.7 .7m0 11.4l.7 .7m-12.1 -.7l-.7 .7"/>`)
	TablerMoon         = tablerSVG(`<path d="M12 3c.132 0 .263 0 .393 0a7.5 7.5 0 0 0 7.92 12.446a9 9 0 1 1 -8.313 -12.454z"/>`)
	TablerExternalLink = tablerSVG(`<path d="M12 6h-6a2 2 0 0 0 -2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2v-6"/><path d="M11 13l9 -9"/><path d="M15 4h5v5"/>`)
	TablerBrandGolang  = tablerSVG(`<path d="M16 10v4a2 2 0 0 1 -2 2h-4a2 2 0 0 1 -2 -2v-4a2 2 0 0 1 2 -2h4a2 2 0 0 1 2 2z"/><path d="M14 12h-4"/><path d="M8 8v8"/><path d="M16 8v8"/>`)

	// Additional Tabler Icons for 100% parity
	TablerBrandTypescript = tablerSVG(`<path d="M15 17.5c.32 .32 .754 .5 1.207 .5c.957 0 1.793 -.58 1.793 -1.5c0 -1.9 -3.5 -1.1 -3.5 -3.5c0 -1.05 .85 -2 2 -2c.6 0 1.15 .25 1.5 .65"/><path d="M9 11v7"/><path d="M6 11h6"/><path d="M3 5a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v14a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z"/>`)
	TablerBrandJavascript = tablerSVG(`<path d="M20 4a2 2 0 0 0 -2 -2h-12a2 2 0 0 0 -2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2z"/><path d="M10 10.5v5.5a1.5 1.5 0 0 1 -3 0"/><path d="M14 10.5c.5 -.5 1.5 -.5 2.5 0s1 1.5 1 2.5c0 2 -3.5 1.5 -3.5 3.5c0 1 .5 1.5 1.5 1.5s2 -.5 2.5 -1"/>`)
	TablerBrandPython     = tablerSVG(`<path d="M12 9h-7a2 2 0 0 0 -2 2v4a2 2 0 0 0 2 2h3v-2a2 2 0 0 1 2 -2h4a1 1 0 0 0 1 -1v-4a2 2 0 0 0 -2 -2h-1z"/><path d="M12 15h7a2 2 0 0 0 2 -2v-4a2 2 0 0 0 -2 -2h-3v2a2 2 0 0 1 -2 2h-4a1 1 0 0 0 -1 1v4a2 2 0 0 0 2 2h1z"/><circle cx="9" cy="6" r="1"/><circle cx="15" cy="18" r="1"/>`)
	TablerBrandHtml5      = tablerSVG(`<path d="M20 4l-2 14.5l-6 2l-6 -2l-2 -14.5z"/><path d="M15.5 8h-7l.5 4h6l-.5 3.5l-2.5 .75l-2.5 -.75l-.1 -.5"/>`)
	TablerBrandCss3       = tablerSVG(`<path d="M20 4l-2 14.5l-6 2l-6 -2l-2 -14.5z"/><path d="M8.5 8h7l-1 7l-2.5 .75l-2.5 -.75l-.2 -1.5h2l.1 .5l1 .3l1 -.3l.2 -2h-5z"/>`)
	TablerJson            = tablerSVG(`<path d="M20 16a3 3 0 0 0 -3 -3a3 3 0 0 0 3 -3"/><path d="M4 16a3 3 0 0 1 3 -3a3 3 0 0 1 -3 -3"/><path d="M11 8l2 0"/><path d="M12 8l0 8"/><path d="M13 16l-2 0"/>`)
	TablerMarkdown        = tablerSVG(`<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M7 15v-6l2 2l2 -2v6"/><path d="M14 13l2 2l2 -2"/><path d="M16 9v6"/>`)
	TablerClock           = tablerSVG(`<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"/><path d="M12 7v5l3 3"/>`)
	TablerChevronDown     = tablerSVG(`<path d="M6 9l6 6l6 -6"/>`)
	TablerChevronRight    = tablerSVG(`<path d="M9 6l6 6l-6 6"/>`)
	TablerChevronUp       = tablerSVG(`<path d="M6 15l6 -6l6 6"/>`)
	TablerChevronLeft     = tablerSVG(`<path d="M15 6l-6 6l6 6"/>`)
	TablerShieldAlert     = tablerSVG(`<path d="M12 3a12 12 0 0 0 8.5 3a12 12 0 0 1 -8.5 15a12 12 0 0 1 -8.5 -15a12 12 0 0 0 8.5 -3"/><path d="M12 8v4"/><path d="M12 16h.01"/>`)
	TablerShieldCheck     = tablerSVG(`<path d="M12 3a12 12 0 0 0 8.5 3a12 12 0 0 1 -8.5 15a12 12 0 0 1 -8.5 -15a12 12 0 0 0 8.5 -3"/><path d="M9 12l2 2l4 -4"/>`)
	TablerBolt            = tablerSVG(`<path d="M13 3l0 7l6 0l-8 11l0 -7l-6 0z"/>`)
	TablerFlame           = tablerSVG(`<path d="M12 12c2 -2.96 0 -7 -1 -8c0 3.038 -1.773 4.741 -3 6c-1.226 1.26 -2 3.24 -2 5a6 6 0 1 0 12 0c0 -1.532 -.77 -2.94 -1.714 -4c-1.786 3 -4.286 2.96 -4.286 1z"/>`)
	TablerAlertTriangle   = tablerSVG(`<path d="M12 9v4"/><path d="M12 16h.01"/><path d="M5 19h14a2 2 0 0 0 1.84 -2.75l-7.1 -12.25a2 2 0 0 0 -3.5 0l-7.1 12.25a2 2 0 0 0 1.75 2.75"/>`)
	TablerCopy            = tablerSVG(`<path d="M8 8m0 2a2 2 0 0 1 2 -2h8a2 2 0 0 1 2 2v8a2 2 0 0 1 -2 2h-8a2 2 0 0 1 -2 -2z"/><path d="M16 8v-2a2 2 0 0 0 -2 -2h-8a2 2 0 0 0 -2 2v8a2 2 0 0 0 2 2h2"/>`)
	TablerCopyCheck       = tablerSVG(`<path d="M9 5h-2a2 2 0 0 0 -2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2v-12a2 2 0 0 0 -2 -2h-2"/><rect x="9" y="3" width="6" height="4" rx="2"/><path d="M9 14l2 2l4 -4"/>`)
	TablerInfoCircle      = tablerSVG(`<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"/><path d="M12 8l.01 0"/><path d="M11 12l1 0l0 4l1 0"/>`)
	TablerHelpCircle      = tablerSVG(`<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"/><path d="M12 17l.01 0"/><path d="M12 13.5a1.5 1.5 0 0 1 1 -1.5a2.6 2.6 0 1 0 -3 -4"/>`)
	TablerKey             = tablerSVG(`<circle cx="8" cy="15" r="4"/><path d="M10.85 12.15l10.15 -10.15"/><path d="M18 5l2 2"/><path d="M15 8l2 2"/>`)
)

// TablerIcon renders a Tabler SVG icon element with explicit size.
func TablerIcon(c *ui.Context, icon *ui.SVG, size float32) ui.Element {
	return ui.Icon(c, icon).Width(size).Height(size)
}

// TablerButton creates a button styled with a Tabler icon.
func TablerButton(c *ui.Context, icon *ui.SVG, size float32) ui.Element {
	btn := ui.Button(c, "")
	btn.Children(func() {
		ui.Icon(c, icon).Width(size).Height(size)
	})
	return btn
}

// FileIconSVG returns the appropriate Tabler SVG icon for a given file name or path.
func FileIconSVG(path string) *ui.SVG {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return TablerBrandGolang
	case ".ts", ".tsx":
		return TablerBrandTypescript
	case ".js", ".jsx":
		return TablerBrandJavascript
	case ".py":
		return TablerBrandPython
	case ".json":
		return TablerJson
	case ".md":
		return TablerMarkdown
	case ".html", ".htm":
		return TablerBrandHtml5
	case ".css", ".scss", ".sass", ".less":
		return TablerBrandCss3
	case ".sql", ".db", ".sqlite", ".sqlite3", ".duckdb":
		return TablerDatabase
	case ".sh", ".bash", ".zsh":
		return TablerTerminal
	case ".yml", ".yaml", ".toml":
		return TablerSettings
	default:
		return TablerFile
	}
}
