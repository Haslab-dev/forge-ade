package icons

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestGradientIcons(t *testing.T) {
	agyStr := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" fill="none"><defs><linearGradient id="agy-grad" x1="26" y1="4" x2="32" y2="64" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#e2574e" /><stop offset="0.18" stop-color="#d98a3f" /><stop offset="0.34" stop-color="#8bbf43" /><stop offset="0.62" stop-color="#5a8ef6" /><stop offset="1" stop-color="#4a7bf5" /></linearGradient></defs><path d="M11 55c3.5-22 10-38 21-46 11 8 17.5 24 21 46" stroke="url(#agy-grad)" stroke-width="11" stroke-linecap="round" fill="none"/></svg>`
	_ = ui.MustParseSVG([]byte(agyStr))

	forgeStr := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none"><defs><linearGradient id="forge-main" x1="100%" y1="0%" x2="0%" y2="100%"><stop offset="0%" stop-color="#38BDF8" /><stop offset="35%" stop-color="#2563EB" /><stop offset="70%" stop-color="#7C3AED" /><stop offset="100%" stop-color="#C026D3" /></linearGradient><linearGradient id="forge-mid-fold" x1="0%" y1="0%" x2="100%" y2="50%"><stop offset="0%" stop-color="#1E3A8A" /><stop offset="100%" stop-color="#2563EB" /></linearGradient><linearGradient id="forge-mid" x1="0%" y1="0%" x2="100%" y2="100%"><stop offset="0%" stop-color="#2563EB" /><stop offset="100%" stop-color="#60A5FA" /></linearGradient><linearGradient id="forge-tail" x1="100%" y1="0%" x2="0%" y2="100%"><stop offset="0%" stop-color="#7C3AED" /><stop offset="100%" stop-color="#E879F9" /></linearGradient></defs><g transform="translate(0.5, 0.5)"><path d="M 10.5 2.5 C 8.2 2.5 6.6 4.1 6.0 6.3 L 3.0 20.0 L 6.6 18.2 L 8.4 11.2 C 8.7 9.8 9.9 8.8 11.3 8.8 L 17.8 8.8 L 21.2 2.5 C 21.2 2.5 14.8 2.5 10.5 2.5 Z" fill="url(#forge-main)" /><path d="M 8.8 11.0 L 11.8 9.0 L 14.6 9.0 L 10.2 11.0 Z" fill="url(#forge-mid-fold)" /><path d="M 9.0 11.0 L 17.5 11.0 L 15.5 14.4 L 7.6 14.4 Z" fill="url(#forge-mid)" /><path d="M 3.0 20.0 L 6.6 18.2 L 7.6 14.4 L 5.2 14.4 Z" fill="url(#forge-tail)" /></g></svg>`
	_ = ui.MustParseSVG([]byte(forgeStr))

	ompStr := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64" fill="none"><defs><linearGradient id="omp-grad" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#ec4899" /><stop offset=".5" stop-color="#a855f7" /><stop offset="1" stop-color="#06b6d4" /></linearGradient></defs><path fill="url(#omp-grad)" d="M10 14h44v9H43v33h-9V23h-9v22h-9V23H10z" /></svg>`
	_ = ui.MustParseSVG([]byte(ompStr))
}
