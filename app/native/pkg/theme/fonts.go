package theme

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

// The Geist families ForgeADE's UI is designed on, embedded from the
// original app's fonts and registered under the names its CSS uses.
//
//go:embed assets/fonts/Geist-Variable.ttf
var geistFont []byte

//go:embed assets/fonts/GeistMono-Variable.ttf
var geistMonoFont []byte

// Font family names, matching ForgeADE's --font-sans and --font-mono stacks.
const (
	FontSans         = "Geist"
	FontMono         = "Geist Mono"
	FontFallback     = "" // the system's
	FontMonoFallback = "monospace"
)

func init() {
	_ = ui.RegisterFont(geistFont, FontSans)
	_ = ui.RegisterFont(geistMonoFont, FontMono)
}

// AppIconPNG is the ForgeADE app icon (512px PNG) used for the runtime Dock
// icon when the app is launched as a bare binary instead of the .app bundle.
//
//go:embed assets/appicon.png
var AppIconPNG []byte
