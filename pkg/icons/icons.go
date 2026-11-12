package icons

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
	TablerSparkles       = tablerSVG(`<path d="M16 18a2 2 0 0 1 2 2a2 2 0 0 1 2 -2a2 2 0 0 1 -2 -2a2 2 0 0 1 -2 2zm0 -12a2 2 0 0 1 2 2a2 2 0 0 1 2 -2a2 2 0 0 1 -2 -2a2 2 0 0 1 -2 2zm-7 1a6 6 0 0 1 6 6a6 6 0 0 1 -6 6a6 6 0 0 1 -6 -6a6 6 0 0 1 6 -6z"/>`)
	TablerCode           = tablerSVG(`<path d="M7 8l-4 4l4 4m10 -8l4 4l-4 4m-4 -10l-4 12"/>`)
	TablerPanelLeft      = tablerSVG(`<path d="M4 4m0 2a2 2 0 0 1 2 -2h12a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2z"/><path d="M9 4v16"/>`)
	TablerPanelRight     = tablerSVG(`<path d="M4 4m0 2a2 2 0 0 1 2 -2h12a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2z"/><path d="M15 4v16"/>`)
	TablerTerminal       = tablerSVG(`<path d="M8 9l3 3l-3 3"/><path d="M13 15l3 0"/><path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z"/>`)
	TablerFolder         = tablerSVG(`<path d="M5 4h4l3 3h7a2 2 0 0 1 2 2v8a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2v-11a2 2 0 0 1 2 -2"/>`)
	TablerFolderOpen     = tablerSVG(`<path d="M5 19l2.757 -7.351a1 1 0 0 1 .936 -.649h12.307a1 1 0 0 1 .986 1.164l-.996 5.211a2 2 0 0 1 -1.964 1.625h-14.026a2 2 0 0 1 -2 -2v-11a2 2 0 0 1 2 -2h4l3 3h7a2 2 0 0 1 2 2v1.5"/>`)
	TablerFolderPlus     = tablerSVG(`<path d="M12 19h-7a2 2 0 0 1 -2 -2v-11a2 2 0 0 1 2 -2h4l3 3h7a2 2 0 0 1 2 2v3.5"/><path d="M16 19h6"/><path d="M19 16v6"/>`)
	TablerFiles          = tablerSVG(`<path d="M15 3v4a1 1 0 0 0 1 1h4"/><path d="M18 17h-7a2 2 0 0 1 -2 -2v-10a2 2 0 0 1 2 -2h4l5 5v7a2 2 0 0 1 -2 2z"/><path d="M16 20.334a1.998 1.998 0 0 1 -1 0.666h-7a2 2 0 0 1 -2 -2v-10a2 2 0 0 1 0.666 -1"/>`)
	TablerFile           = tablerSVG(`<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z"/>`)
	TablerFileCode       = tablerSVG(`<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z"/><path d="M10 13l-1 2l1 2"/><path d="M14 13l1 2l-1 2"/>`)
	TablerFileText       = tablerSVG(`<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z"/><path d="M9 9l1 0"/><path d="M9 13l6 0"/><path d="M9 17l6 0"/>`)
	TablerFilePlus       = tablerSVG(`<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z"/><path d="M12 11l0 6"/><path d="M9 14l6 0"/>`)
	TablerFileDiff       = tablerSVG(`<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2z"/><path d="M9 11l6 0"/><path d="M12 8l0 6"/><path d="M9 17l6 0"/>`)
	TablerGitBranch      = tablerSVG(`<path d="M7 18m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M7 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M17 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M7 8l0 8"/><path d="M9 18h1a4 4 0 0 0 4 -4v-2a4 4 0 0 1 4 -4h1"/>`)
	TablerGitCommit      = tablerSVG(`<path d="M12 12m-3 0a3 3 0 1 0 6 0a3 3 0 1 0 -6 0"/><path d="M12 3l0 6"/><path d="M12 15l0 6"/>`)
	TablerGitFork        = tablerSVG(`<path d="M12 18m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M7 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M17 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M7 8v2a2 2 0 0 0 2 2h6a2 2 0 0 0 2 -2v-2"/><path d="M12 12l0 4"/>`)
	TablerGitCompare     = tablerSVG(`<path d="M6 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M18 18m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M11 6h5a2 2 0 0 1 2 2v8"/><path d="M14 9l-3 -3l3 -3"/><path d="M13 18h-5a2 2 0 0 1 -2 -2v-8"/><path d="M10 15l3 3l-3 3"/>`)
	TablerGitPullRequest = tablerSVG(`<path d="M6 18m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M6 6m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M18 18m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M6 8v8"/><path d="M11 6h4a2 2 0 0 1 2 2v8"/><path d="M14 9l-3 -3l3 -3"/>`)
	TablerDatabase       = tablerSVG(`<path d="M12 6m-8 0a8 3 0 1 0 16 0a8 3 0 1 0 -16 0"/><path d="M4 6v6a8 3 0 0 0 16 0v-6"/><path d="M4 12v6a8 3 0 0 0 16 0v-6"/>`)
	TablerSearch         = tablerSVG(`<path d="M10 10m-7 0a7 7 0 1 0 14 0a7 7 0 1 0 -14 0"/><path d="M21 21l-6 -6"/>`)
	TablerSettings       = tablerSVG(`<path d="M10.325 4.317c.426 -1.756 2.924 -1.756 3.35 0a1.724 1.724 0 0 0 2.573 1.066c1.543 -.94 3.31 .826 2.37 2.37a1.724 1.724 0 0 0 1.065 2.572c1.756 .426 1.756 2.924 0 3.35a1.724 1.724 0 0 0 -1.066 2.573c.94 1.543 -.826 3.31 -2.37 2.37a1.724 1.724 0 0 0 -2.572 1.065c-.426 1.756 -2.924 1.756 -3.35 0a1.724 1.724 0 0 0 -2.573 -1.066c-1.543 .94 -3.31 -.826 -2.37 -2.37a1.724 1.724 0 0 0 -1.065 -2.572c-1.756 -.426 -1.756 -2.924 0 -3.35a1.724 1.724 0 0 0 1.066 -2.573c-.94 -1.543 .826 -3.31 2.37 -2.37c1 .608 2.296 .07 2.572 -1.065z"/><path d="M9 12a3 3 0 1 0 6 0a3 3 0 0 0 -6 0"/>`)
	TablerPalette        = tablerSVG(`<path d="M12 21a9 9 0 0 1 0 -18c4.97 0 9 3.582 9 8c0 1.06 -.474 2.078 -1.318 2.828c-.844 .75 -1.989 1.172 -3.182 1.172h-2.5a2 2 0 0 0 -1 3.75a1.3 1.3 0 0 1 -1 2.25"/><path d="M8.5 10.5m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M12.5 7.5m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M16.5 10.5m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/>`)
	TablerPlus           = tablerSVG(`<path d="M12 5l0 14"/><path d="M5 12l14 0"/>`)
	TablerMinus          = tablerSVG(`<path d="M5 12l14 0"/>`)
	TablerX              = tablerSVG(`<path d="M18 6l-12 12"/><path d="M6 6l12 12"/>`)
	TablerCheck          = tablerSVG(`<path d="M5 12l5 5l10 -10"/>`)
	TablerCircleCheck    = tablerSVG(`<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"/><path d="M9 12l2 2l4 -4"/>`)
	TablerCircle         = tablerSVG(`<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"/>`)
	TablerCircleDot      = tablerSVG(`<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"/><path d="M12 12m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/>`)
	TablerLoader         = tablerSVG(`<path d="M12 3a9 9 0 1 0 9 9"/>`)
	TablerPin            = tablerSVG(`<path d="M15 4.5l-4 4l-4 1.5l-1.5 1.5l7 7l1.5 -1.5l1.5 -4l4 -4"/><path d="M9 15l-4.5 4.5"/><path d="M14.5 4l5.5 5.5"/>`)
	TablerPinFilled      = tablerSVG(`<path d="M15 4.5l-4 4l-4 1.5l-1.5 1.5l7 7l1.5 -1.5l1.5 -4l4 -4z" fill="currentColor"/><path d="M9 15l-4.5 4.5"/><path d="M14.5 4l5.5 5.5"/>`)
	TablerTrash          = tablerSVG(`<path d="M4 7l16 0"/><path d="M10 11l0 6"/><path d="M14 11l0 6"/><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12"/><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3"/>`)
	TablerPencil         = tablerSVG(`<path d="M4 20h4l10.5 -10.5a2.828 2.828 0 1 0 -4 -4l-10.5 10.5v4"/><path d="M13.5 6.5l4 4"/>`)
	TablerDots           = tablerSVG(`<path d="M5 12m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M12 12m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M19 12m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/>`)
	TablerDeviceFloppy   = tablerSVG(`<path d="M6 4h10l4 4v10a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2v-12a2 2 0 0 1 2 -2"/><path d="M12 14m-2 0a2 2 0 1 0 4 0a2 2 0 1 0 -4 0"/><path d="M14 4l0 4l-6 0l0 -4"/>`)
	TablerColumns        = tablerSVG(`<path d="M4 4m0 2a2 2 0 0 1 2 -2h12a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2z"/><path d="M12 4l0 16"/>`)
	TablerSend           = tablerSVG(`<path d="M10 14l11 -11"/><path d="M21 3l-6.5 18a.55 .55 0 0 1 -1 0l-3.5 -7l-7 -3.5a.55 .55 0 0 1 0 -1l18 -6.5"/>`)
	TablerSquare         = tablerSVG(`<path d="M4 4m0 2a2 2 0 0 1 2 -2h12a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2z"/>`)
	TablerBrain          = tablerSVG(`<path d="M15.5 13a3.5 3.5 0 0 0 -3.5 3.5v1a3.5 3.5 0 0 0 7 0v-1.8"/><path d="M8.5 13a3.5 3.5 0 0 1 3.5 3.5v1a3.5 3.5 0 0 1 -7 0v-1.8"/><path d="M17.5 16a3.5 3.5 0 0 0 0 -7h-.5"/><path d="M19 9.3v-2.8a3.5 3.5 0 0 0 -7 0"/><path d="M6.5 16a3.5 3.5 0 0 1 0 -7h.5"/><path d="M5 9.3v-2.8a3.5 3.5 0 0 1 7 0"/>`)
	TablerCpu            = tablerSVG(`<path d="M5 5m0 1a1 1 0 0 1 1 -1h12a1 1 0 0 1 1 1v12a1 1 0 0 1 -1 1h-12a1 1 0 0 1 -1 -1z"/><path d="M9 9h6v6h-6z"/><path d="M3 10h2"/><path d="M3 14h2"/><path d="M10 3v2"/><path d="M14 3v2"/><path d="M21 10h-2"/><path d="M21 14h-2"/><path d="M14 21v-2"/><path d="M10 21v-2"/>`)
	TablerPlug           = tablerSVG(`<path d="M9.785 6l8.215 8.215l-2.054 2.054a5.81 5.81 0 1 1 -8.215 -8.215l2.054 -2.054z"/><path d="M4 20l3.5 -3.5"/><path d="M15 4l-3.5 3.5"/><path d="M20 9l-3.5 3.5"/>`)
	TablerRobot          = tablerSVG(`<path d="M7 7h10a2 2 0 0 1 2 2v8a2 2 0 0 1 -2 2h-10a2 2 0 0 1 -2 -2v-8a2 2 0 0 1 2 -2z"/><path d="M10 16h4"/><path d="M9 11v.01"/><path d="M15 11v.01"/><path d="M12 7v-3"/><path d="M8 4h8"/>`)
	TablerKeyboard       = tablerSVG(`<path d="M2 5m0 2a2 2 0 0 1 2 -2h16a2 2 0 0 1 2 2v10a2 2 0 0 1 -2 2h-16a2 2 0 0 1 -2 -2z"/><path d="M6 10l0 .01"/><path d="M10 10l0 .01"/><path d="M14 10l0 .01"/><path d="M18 10l0 .01"/><path d="M6 14l0 .01"/><path d="M18 14l0 .01"/><path d="M10 14l4 0"/>`)
	TablerRefresh        = tablerSVG(`<path d="M20 11a8.1 8.1 0 0 0 -15.5 -2m-.5 -5v5h5"/><path d="M4 13a8.1 8.1 0 0 0 15.5 2m.5 5v-5h-5"/>`)
	TablerHistory        = tablerSVG(`<path d="M12 8l0 4l2 2"/><path d="M3.05 11a9 9 0 1 1 .5 4m-.5 5v-5h5"/>`)
	TablerTable          = tablerSVG(`<path d="M3 5a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v14a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2v-14z"/><path d="M3 10h18"/><path d="M10 3v18"/>`)
	TablerEye            = tablerSVG(`<path d="M10 12a2 2 0 1 0 4 0a2 2 0 0 0 -4 0"/><path d="M21 12c-2.4 4 -5.4 6 -9 6c-3.6 0 -6.6 -2 -9 -6c2.4 -4 5.4 -6 9 -6c3.6 0 6.6 2 9 6"/>`)
	TablerPlayerPlay     = tablerSVG(`<path d="M7 4v16l13 -8z"/>`)
	TablerChecklist      = tablerSVG(`<path d="M9.615 20h-2.615a2 2 0 0 1 -2 -2v-12a2 2 0 0 1 2 -2h8a2 2 0 0 1 2 2v8"/><path d="M14 19l2 2l4 -4"/><path d="M9 8h4"/><path d="M9 12h2"/>`)
	TablerSun            = tablerSVG(`<path d="M12 12m-4 0a4 4 0 1 0 8 0a4 4 0 1 0 -8 0"/><path d="M3 12h1m8 -9v1m8 8h1m-9 8v1m-6.4 -15.4l.7 .7m12.1 -.7l-.7 .7m0 11.4l.7 .7m-12.1 -.7l-.7 .7"/>`)
	TablerMoon           = tablerSVG(`<path d="M12 3c.132 0 .263 0 .393 0a7.5 7.5 0 0 0 7.92 12.446a9 9 0 1 1 -8.313 -12.454z"/>`)
	TablerExternalLink   = tablerSVG(`<path d="M12 6h-6a2 2 0 0 0 -2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2v-6"/><path d="M11 13l9 -9"/><path d="M15 4h5v5"/>`)
	TablerBrandGolang    = tablerSVG(`<path d="M16 10v4a2 2 0 0 1 -2 2h-4a2 2 0 0 1 -2 -2v-4a2 2 0 0 1 2 -2h4a2 2 0 0 1 2 2z"/><path d="M14 12h-4"/><path d="M8 8v8"/><path d="M16 8v8"/>`)
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
	TablerHand            = tablerSVG(`<path d="M8 13v-8.5a1.5 1.5 0 0 1 3 0v7.5"/><path d="M11 11.5v-2a1.5 1.5 0 0 1 3 0v2.5"/><path d="M14 10.5a1.5 1.5 0 0 1 3 0v1.5"/><path d="M17 11.5a1.5 1.5 0 0 1 3 0v4.5a6 6 0 0 1 -6 6h-2h.208a6 6 0 0 1 -5.012 -2.7l-.196 -.3c-.312 -.479 -1.407 -2.388 -3.286 -5.728a1.5 1.5 0 0 1 .536 -2.022a1.867 1.867 0 0 1 2.28 .28l1.47 1.47"/>`)
	TablerGripVertical    = tablerSVG(`<path d="M9 5m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M9 12m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M9 19m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M15 5m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M15 12m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M15 19m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/>`)
	TablerFilePen         = tablerSVG(`<path d="M14 3v4a1 1 0 0 0 1 1h4"/><path d="M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v7"/><path d="M18 18m-1 0a1 1 0 1 0 2 0a1 1 0 1 0 -2 0"/><path d="M19.5 15.5l-4.5 4.5h-2v-2l4.5 -4.5z"/>`)
	TablerCompass         = tablerSVG(`<path d="M8 16l2 -6l6 -2l-2 6l-6 2"/><path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"/>`)
	TablerBug             = tablerSVG(`<path d="M9 9v-1a3 3 0 0 1 6 0v1"/><path d="M8 9h8a6 6 0 0 1 1 3v3a5 5 0 0 1 -10 0v-3a6 6 0 0 1 1 -3"/><path d="M3 13l4 0"/><path d="M17 13l4 0"/><path d="M12 20l0 -6"/><path d="M4 19l3.35 -2"/><path d="M20 19l-3.35 -2"/><path d="M4 7l3.75 1.5"/><path d="M20 7l-3.75 1.5"/>`)
	TablerFlask           = tablerSVG(`<path d="M9 3l6 0"/><path d="M10 9l4 0"/><path d="M10 3v6l-4 11a.7 .7 0 0 0 .5 1h11a.7 .7 0 0 0 .5 -1l-4 -11v-6"/>`)
	TablerArrowLeft       = tablerSVG(`<path d="M5 12l14 0"/><path d="M5 12l6 6"/><path d="M5 12l6 -6"/>`)
	TablerArrowUp         = tablerSVG(`<path d="M12 5l0 14"/><path d="M12 5l-6 6"/><path d="M12 5l6 6"/>`)
	TablerEyeOff          = tablerSVG(`<path d="M10.585 10.587a2 2 0 0 0 2.829 2.828"/><path d="M16.681 16.673a8.717 8.717 0 0 1 -4.681 1.327c-3.6 0 -6.6 -2 -9 -6c1.272 -2.12 2.712 -3.678 4.32 -4.674m2.86 -1.146a9.055 9.055 0 0 1 1.82 -.18c3.6 0 6.6 2 9 6c-.666 1.11 -1.379 2.067 -2.138 2.87"/><path d="M3 3l18 18"/>`)
	TablerPackage         = tablerSVG(`<path d="M12 3l8 4.5l0 9l-8 4.5l-8 -4.5l0 -9l8 -4.5"/><path d="M12 12l8 -4.5"/><path d="M12 12l0 9"/><path d="M12 12l-8 -4.5"/>`)
	TablerWand            = tablerSVG(`<path d="M6 21l15 -15l-3 -3l-15 15l3 3"/><path d="M15 6l3 3"/><path d="M9 3a2 2 0 0 0 2 2a2 2 0 0 0 -2 2a2 2 0 0 0 -2 -2a2 2 0 0 0 2 -2"/><path d="M19 13a2 2 0 0 0 2 2a2 2 0 0 0 2 -2a2 2 0 0 0 -2 -2a2 2 0 0 0 -2 2"/>`)
	TablerShield          = tablerSVG(`<path d="M12 3a12 12 0 0 0 8.5 3a12 12 0 0 1 -8.5 15a12 12 0 0 1 -8.5 -15a12 12 0 0 0 8.5 -3"/>`)
	TablerFilter          = tablerSVG(`<path d="M4 4h16v2.172a2 2 0 0 1 -.586 1.414l-4.414 4.414v7l-6 2v-8.5l-4.48 -4.928a2 2 0 0 1 -.52 -1.364v-2.208z"/>`)
	TablerFileZip         = tablerSVG(`<path d="M6 20.735a2 2 0 0 1 -1 -1.735v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2h-1"/><path d="M11 17a2 2 0 0 1 2 2v2a1 1 0 0 1 -1 1h-2a1 1 0 0 1 -1 -1v-2a2 2 0 0 1 2 -2z"/><path d="M11 5l2 0"/><path d="M11 7l2 0"/><path d="M11 9l2 0"/><path d="M11 11l2 0"/><path d="M11 13l2 0"/><path d="M11 15l2 0"/>`)
	TablerServer          = tablerSVG(`<path d="M3 4m0 3a3 3 0 0 1 3 -3h12a3 3 0 0 1 3 3v2a3 3 0 0 1 -3 3h-12a3 3 0 0 1 -3 -3z"/><path d="M3 12m0 3a3 3 0 0 1 3 -3h12a3 3 0 0 1 3 3v2a3 3 0 0 1 -3 3h-12a3 3 0 0 1 -3 -3z"/><path d="M7 8l0 .01"/><path d="M7 16l0 .01"/>`)
	TablerHelp            = tablerSVG(`<path d="M12 12m-9 0a9 9 0 1 0 18 0a9 9 0 1 0 -18 0"/><path d="M12 17l0 .01"/><path d="M12 13.5a1.5 1.5 0 0 1 1 -1.5a2.6 2.6 0 1 0 -3 -2"/>`)
	TablerChartBar        = tablerSVG(`<path d="M3 12m0 1a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v6a1 1 0 0 1 -1 1h-4a1 1 0 0 1 -1 -1z"/><path d="M9 8m0 1a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v10a1 1 0 0 1 -1 1h-4a1 1 0 0 1 -1 -1z"/><path d="M15 4m0 1a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v14a1 1 0 0 1 -1 1h-4a1 1 0 0 1 -1 -1z"/><path d="M4 20l14 0"/>`)
	TablerLayoutKanban    = tablerSVG(`<path d="M4 4l0 16"/><path d="M7 6h4v11h-4z"/><path d="M15 6h4v6h-4z"/>`)
	TablerBook            = tablerSVG(`<path d="M3 19a9 9 0 0 1 9 0a9 9 0 0 1 9 0"/><path d="M3 6a9 9 0 0 1 9 0a9 9 0 0 1 9 0"/><path d="M3 6l0 13"/><path d="M12 6l0 13"/><path d="M21 6l0 13"/>`)
	TablerBot             = tablerSVG(`<path d="M7 7h10a2 2 0 0 1 2 2v8a2 2 0 0 1 -2 2h-10a2 2 0 0 1 -2 -2v-8a2 2 0 0 1 2 -2z"/><path d="M9 11v.01"/><path d="M15 11v.01"/><path d="M9.5 15a3.5 3.5 0 0 0 5 0"/><path d="M12 7v-4"/><path d="M10 3h4"/>`)
	TablerMessage         = tablerSVG(`<path d="M4 21v-13a3 3 0 0 1 3 -3h10a3 3 0 0 1 3 3v6a3 3 0 0 1 -3 3h-9l-4 4z"/>`)
	TablerArchive         = tablerSVG(`<path d="M3 4m0 2a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v0a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2z"/><path d="M5 8v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2v-10"/><path d="M10 12l4 0"/>`)
	ForgeSparkSVG         = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48" fill="currentColor"><path d="M24 3c1.6 10.6 6.4 16.7 20 21-13.6 4.3-18.4 10.4-20 21-1.6-10.6-6.4-16.7-20-21 13.6-4.3 18.4-10.4 20-21Z"/><circle cx="39.5" cy="8.5" r="3.2"/></svg>`))
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

// Provider brand SVGs
var (
	ProviderClaude   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 257" fill="#D97757"><path d="m50.23 170.32 50.36-28.26.843-2.46-.843-1.36h-2.46l-8.43-.518-28.77-.778-24.95-1.04-24.18-1.3-6.09-1.3L0 125.8l.583-3.76 5.12-3.43 7.32.648 16.2 1.1 24.3 1.69 17.63 1.04 26.12 2.72h4.15l.583-1.69-1.43-1.04-1.1-1.04-25.15-17.05-27.22-18.02-14.26-10.37-7.71-5.25-3.89-4.92-1.69-10.76 7-7.71 9.4.649 2.4.648 9.53 7.32 20.35 15.75L94.82 91.9l3.89 3.24 1.55-1.1.195-.777-1.75-2.92-14.45-26.12-15.43-26.57-6.87-11.02-1.81-6.61c-.648-2.72-1.1-4.99-1.1-7.78l7.97-10.82L71.42 0 82.05 1.43l4.47 3.89 6.61 15.1 10.69 23.79 16.59 32.34 4.86 9.59 2.59 8.88.973 2.72h1.69v-1.56l1.36-18.21 2.53-22.36 2.46-28.78.843-8.1 4.02-9.72 7.97-5.25 6.22 2.98 5.12 7.32-.713 4.73-3.05 19.77-5.96 30.98-3.89 20.74h2.27l2.59-2.59 10.5-13.93 17.63-22.04 7.78-8.75 9.07-9.66 5.83-4.6h11.02l8.1 12.05-3.63 12.44-11.34 14.39-9.4 12.18-13.48 18.15-8.43 14.52.778 1.17 2.01-.194 30.46-6.48 16.46-2.98 19.64-3.37 8.88 4.15.971 4.21-3.5 8.62-21 5.18-24.63 4.93-36.68 8.69-.454.32.519.65 16.53 1.55 7.07.389h17.3l32.21 2.4 8.43 5.57 5.05 6.8-.843 5.18-12.96 6.61-17.5-4.15-40.83-9.72-14-3.5h-1.94v1.17l11.67 11.41 21.39 19.31 26.77 24.89 1.36 6.16-3.43 4.86-3.63-.518-23.53-17.69-9.07-7.97-20.55-17.3h-1.36v1.81l4.73 6.93 25.02 37.59 1.3 11.54-1.81 3.76-6.48 2.27-7.13-1.3-14.65-20.54-15.1-23.14-12.19-20.74-1.49.84-7.19 77.45-3.37 3.95-7.78 2.98-6.48-4.92-3.44-7.97 3.44-15.75 4.15-20.54 3.37-16.33 3.05-20.29 1.81-6.74-.13-.454-1.49.19-15.29 21-23.27 31.43-18.41 19.7-4.41 1.75-7.65-3.95.713-7.06 4.28-6.29 25.47-32.41 15.36-20.09 9.92-11.6-.065-1.69h-.583L44.07 198.12l-12.05 1.55-5.18-4.86.65-7.97 2.46-2.59 20.35-14-.64.06Z"/></svg>`))
	ProviderOpenAI   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729zm-9.022 12.6081a4.4755 4.4755 0 0 1-2.8764-1.0408l.1419-.0804 4.7783-2.7582a.7948.7948 0 0 0 .3927-.6813v-6.7369l2.02 1.1683a.071.071 0 0 1 .038.052v5.5826a4.504 4.504 0 0 1-4.4945 4.4947zm-9.6607-4.1254a4.4708 4.4708 0 0 1-.5346-3.0137l.142.0852 4.783 2.7582a.7712.7712 0 0 0 .7806 0l5.8428-3.3685v2.3324a.0804.0804 0 0 1-.0332.0615L9.74 19.9502a4.4992 4.4992 0 0 1-6.1408-1.6464zM2.3408 7.8956a4.485 4.485 0 0 1 2.3655-1.9728V11.6a.7664.7664 0 0 0 .3879.6765l5.8144 3.3543-2.0201 1.1683a.0757.0757 0 0 1-.071 0l-4.8303-2.7865A4.504 4.504 0 0 1 2.3408 7.872zm16.5963 3.8558L13.1038 8.364 15.1195 7.2a.0757.0757 0 0 1 .071 0l4.8303 2.7913a4.4944 4.4944 0 0 1-.6765 8.1042v-5.6772a.79.79 0 0 0-.407-.6668zm2.0107-3.0231l-.142-.0852-4.7735-2.7818a.7759.7759 0 0 0-.7854 0L9.409 9.2297V6.8974a.0662.0662 0 0 1 .0284-.0615l4.8303-2.7866a4.4992 4.4992 0 0 1 6.6802 4.66zM8.3065 12.863l-2.02-1.1635a.0804.0804 0 0 1-.038-.0567V6.0748a4.4992 4.4992 0 0 1 7.3757-3.4537l-.142.0805L8.704 5.4598a.7948.7948 0 0 0-.3927.6813v6.7219zm1.0977-2.3654l2.602-1.4998 2.6069 1.4998v2.9994l-2.5974 1.4997-2.6067-1.4997Z"/></svg>`))
	ProviderGemini   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C12 7.52 7.52 12 2 12C7.52 12 12 16.48 12 22C12 16.48 16.48 12 22 12C16.48 12 12 7.52 12 2Z"/></svg>`))
	ProviderDeepSeek = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 3C6.48 3 2 7.48 2 13c0 3.32 1.62 6.26 4.12 8.08.38.28.91.06.91-.41v-2.07c0-.28.16-.54.41-.67 1.34-.69 2.29-2.01 2.5-3.56.06-.44.44-.77.89-.77h2.34c.45 0 .83.33.89.77.21 1.55 1.16 2.87 2.5 3.56.25.13.41.39.41.67v2.07c0 .47.53.69.91.41C19.38 19.26 21 16.32 21 13c0-5.52-4.48-10-10-10zm-3 8a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3zm6 0a1.5 1.5 0 1 1 0-3 1.5 1.5 0 0 1 0 3z"/></svg>`))
	ProviderOllama   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2a4 4 0 0 0-4 4v2H7a3 3 0 0 0-3 3v7a3 3 0 0 0 3 3h10a3 3 0 0 0 3-3v-7a3 3 0 0 0-3-3h-1V6a4 4 0 0 0-4-4zm-2 6V6a2 2 0 1 1 4 0v2h-4zm-1 5a1.5 1.5 0 1 1 0 3 1.5 1.5 0 0 1 0-3zm6 0a1.5 1.5 0 1 1 0 3 1.5 1.5 0 0 1 0-3z"/></svg>`))
	ProviderCursor   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 466.73 532.09" fill="currentColor"><path d="M457.43,125.94L244.42,2.96c-6.84-3.95-15.28-3.95-22.12,0L9.3,125.94c-5.75,3.32-9.3,9.46-9.3,16.11v247.99c0,6.65,3.55,12.79,9.3,16.11l213.01,122.98c6.84,3.95,15.28,3.95,22.12,0l213.01-122.98c5.75-3.32,9.3-9.46,9.3-16.11v-247.99c0-6.65-3.55-12.79-9.3-16.11h-.01ZM444.05,151.99l-205.63,356.16c-1.39,2.4-5.06,1.42-5.06-1.36v-233.21c0-4.66-2.49-8.97-6.53-11.31L24.87,145.67c-2.4-1.39-1.42-5.06,1.36-5.06h411.26c5.84,0,9.49,6.33,6.57,11.39h-.01Z"/></svg>`))
)

// ProviderSVG returns the appropriate brand SVG for a model or provider key.
func ProviderSVG(providerOrModel string) (*ui.SVG, ui.Color) {
	norm := strings.ToLower(providerOrModel)
	if strings.Contains(norm, "claude") || strings.Contains(norm, "anthropic") {
		return ProviderClaude, ui.Hex("#D97757")
	}
	if strings.Contains(norm, "openai") || strings.Contains(norm, "gpt") || strings.Contains(norm, "codex") || strings.Contains(norm, "o1") || strings.Contains(norm, "o3") {
		return ProviderOpenAI, ui.Hex("#10A37F")
	}
	if strings.Contains(norm, "gemini") || strings.Contains(norm, "google") || strings.Contains(norm, "antigravity") || strings.Contains(norm, "agy") {
		return ProviderGemini, ui.Hex("#4285F4")
	}
	if strings.Contains(norm, "deepseek") {
		return ProviderDeepSeek, ui.Hex("#4D6BFE")
	}
	if strings.Contains(norm, "ollama") {
		return ProviderOllama, ui.Hex("#A3A3A3")
	}
	if strings.Contains(norm, "cursor") {
		return ProviderCursor, ui.Hex("#6366F1")
	}
	return ForgeSparkSVG, ui.Hex("#6366F1")
}

// RenderProviderIcon renders the provider brand SVG element with brand coloring.
func RenderProviderIcon(c *ui.Context, providerOrModel string, size float32) ui.Element {
	svg, color := ProviderSVG(providerOrModel)
	return ui.Icon(c, svg).Width(size).Height(size).TextColor(color)
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
