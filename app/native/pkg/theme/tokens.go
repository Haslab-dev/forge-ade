package theme

import (
	"github.com/egoist/mygo/ui"
)

// This file is the ForgeADE design token system, ported 1:1 from
// frontend/src/lib/themePalettes.ts and frontend/src/index.css. Every color
// below is the original value, expressed in oklch exactly as the source app
// writes it, so native rendering matches the web app pixel for pixel
// (ui.Oklch keeps the wide-gamut value and derives the sRGB fallback as
// CSS does).

// ok is a shorthand for the source palette notation "oklch(l c h)".
func ok(l, c, h float32) ui.Color { return ui.Oklch(l, c, h) }

// ThemeColors carries the full set of ForgeADE theme color roles
// (THEME_COLOR_ROLES in themePalettes.ts) plus the semantic extras the
// original index.css defines on top of every palette.
type ThemeColors struct {
	// The 58 palette roles of themePalettes.ts.
	Canvas                      ui.Color
	Chrome                      ui.Color
	Toolbar                     ui.Color
	ToolbarForeground           ui.Color
	ToolbarBorder               ui.Color
	ToolbarControl              ui.Color
	ToolbarControlForeground    ui.Color
	ToolbarControlHover         ui.Color
	Surface                     ui.Color
	SurfaceRaised               ui.Color
	SurfaceOverlay              ui.Color
	Text                        ui.Color
	TextMuted                   ui.Color
	Border                      ui.Color
	Input                       ui.Color
	Focus                       ui.Color
	Accent                      ui.Color
	AccentForeground            ui.Color
	Secondary                   ui.Color
	SecondaryForeground         ui.Color
	Muted                       ui.Color
	MutedForeground             ui.Color
	Placeholder                 ui.Color
	SecondaryLabel              ui.Color
	IconMuted                   ui.Color
	Error                       ui.Color
	ErrorForeground             ui.Color
	ErrorSurface                ui.Color
	Warning                     ui.Color
	WarningForeground           ui.Color
	WarningSurface              ui.Color
	Update                      ui.Color
	UpdateForeground            ui.Color
	UpdateSurface               ui.Color
	AccentSurface               ui.Color
	AccentSurfaceForeground     ui.Color
	MessageSurface              ui.Color
	MessageForeground           ui.Color
	MessageAction               ui.Color
	MessageActionForeground     ui.Color
	MessageActionHover          ui.Color
	CodeBackground              ui.Color
	CodeForeground              ui.Color
	SearchMatchBackground       ui.Color
	SearchMatchForeground       ui.Color
	SearchMatchActiveBackground ui.Color
	SearchMatchActiveForeground ui.Color
	Sidebar                     ui.Color
	SidebarForeground           ui.Color
	SidebarMutedForeground      ui.Color
	SidebarControlSurface       ui.Color
	SidebarRowHover             ui.Color
	SidebarRowActive            ui.Color
	SidebarRowSelected          ui.Color
	SidebarBorder               ui.Color
	TerminalBackground          ui.Color
	TerminalForeground          ui.Color
	TerminalCursor              ui.Color
	TerminalSelection           ui.Color
	TerminalScrollbar           ui.Color
	TerminalScrollbarHover      ui.Color

	// Semantic extras from index.css :root (palette-independent in the
	// original, differing only by appearance).
	Success                ui.Color
	SuccessForeground      ui.Color
	Info                   ui.Color
	InfoForeground         ui.Color
	DiffAddition           ui.Color
	DiffDeletion           ui.Color
	DiffAdditionForeground ui.Color
	DiffDeletionForeground ui.Color
	FullAccess             ui.Color // --full-access-accent #f97316
	GraphColors            [10]ui.Color // --graph-c0..c9 commit-graph lanes
}

// forgePaletteLight returns the light appearance of one of the six built-in
// palettes; forgePaletteDark its dark variant. Blossom's dark variant is
// Forge Default's, exactly as in themePalettes.ts.

func forgeDefaultLight() ThemeColors {
	return ThemeColors{
		Canvas: ok(0.985, 0.002, 260), Chrome: ok(0.98, 0.002, 260),
		Toolbar: ok(0.98, 0.002, 260), ToolbarForeground: ok(0.21, 0.006, 260),
		ToolbarBorder: ok(0.91, 0.004, 260), ToolbarControl: ok(0.955, 0.003, 260),
		ToolbarControlForeground: ok(0.24, 0.006, 260), ToolbarControlHover: ok(0.93, 0.005, 260),
		Surface: ok(0.968, 0.003, 260), SurfaceRaised: ok(0.992, 0.001, 260), SurfaceOverlay: ok(1, 0, 0),
		Text: ok(0.18, 0.006, 260), TextMuted: ok(0.46, 0.01, 260),
		Border: ok(0.905, 0.004, 260), Input: ok(0.88, 0.005, 260),
		Focus: ok(0.55, 0.21, 258), Accent: ok(0.55, 0.21, 258), AccentForeground: ok(1, 0, 0),
		Secondary: ok(0.955, 0.003, 260), SecondaryForeground: ok(0.25, 0.006, 260),
		Muted: ok(0.945, 0.004, 260), MutedForeground: ok(0.48, 0.01, 260),
		Placeholder: ok(0.58, 0.008, 260), SecondaryLabel: ok(0.46, 0.01, 260), IconMuted: ok(0.52, 0.01, 260),
		Error: ok(0.60, 0.22, 25), ErrorForeground: ok(0.45, 0.19, 25), ErrorSurface: ok(0.96, 0.02, 25),
		Warning: ok(0.75, 0.16, 70), WarningForeground: ok(0.52, 0.14, 55), WarningSurface: ok(0.965, 0.02, 70),
		Update: ok(0.55, 0.21, 258), UpdateForeground: ok(0.45, 0.19, 258), UpdateSurface: ok(0.95, 0.02, 258),
		AccentSurface: ok(0.945, 0.015, 258), AccentSurfaceForeground: ok(0.28, 0.04, 258),
		MessageSurface: ok(0.948, 0.004, 260), MessageForeground: ok(0.18, 0.006, 260),
		MessageAction: ok(0.55, 0.21, 258), MessageActionForeground: ok(1, 0, 0), MessageActionHover: ok(0.49, 0.21, 258),
		CodeBackground: ok(0.962, 0.003, 260), CodeForeground: ok(0.22, 0.01, 260),
		SearchMatchBackground: ok(0.915, 0.11, 85), SearchMatchForeground: ok(0.216164, 0.028283, 77.423),
		SearchMatchActiveBackground: ok(0.774202, 0.151946, 74.331), SearchMatchActiveForeground: ok(0.216164, 0.028283, 77.423),
		Sidebar: ok(0.965, 0.003, 260), SidebarForeground: ok(0.20, 0.006, 260),
		SidebarMutedForeground: ok(0.50, 0.01, 260), SidebarControlSurface: ok(0.99, 0.001, 260),
		SidebarRowHover: ok(0.945, 0.004, 260), SidebarRowActive: ok(0.925, 0.006, 260),
		SidebarRowSelected: ok(0.93, 0.006, 260), SidebarBorder: ok(0.91, 0.004, 260),
		TerminalBackground: ok(0.985, 0.002, 260), TerminalForeground: ok(0.20, 0.006, 260),
		TerminalCursor: ok(0.55, 0.21, 258), TerminalSelection: ok(0.88, 0.04, 258),
		TerminalScrollbar: ok(0.86, 0.005, 260), TerminalScrollbarHover: ok(0.78, 0.008, 260),
	}
}

func forgeDefaultDark() ThemeColors {
	return ThemeColors{
		Canvas: ok(0.135, 0.003, 260), Chrome: ok(0.15, 0.004, 260),
		Toolbar: ok(0.15, 0.004, 260), ToolbarForeground: ok(0.96, 0.002, 260),
		ToolbarBorder: ok(0.24, 0.005, 260), ToolbarControl: ok(0.20, 0.005, 260),
		ToolbarControlForeground: ok(0.85, 0.005, 260), ToolbarControlHover: ok(0.25, 0.008, 260),
		Surface: ok(0.17, 0.005, 260), SurfaceRaised: ok(0.21, 0.006, 260), SurfaceOverlay: ok(0.25, 0.007, 260),
		Text: ok(0.97, 0.002, 260), TextMuted: ok(0.68, 0.01, 260),
		Border: ok(0.24, 0.005, 260), Input: ok(0.18, 0.005, 260),
		Focus: ok(0.65, 0.20, 250), Accent: ok(0.65, 0.20, 250), AccentForeground: ok(0.99, 0, 0),
		Secondary: ok(0.22, 0.005, 260), SecondaryForeground: ok(0.88, 0.005, 260),
		Muted: ok(0.20, 0.005, 260), MutedForeground: ok(0.62, 0.01, 260),
		Placeholder: ok(0.48, 0.01, 260), SecondaryLabel: ok(0.68, 0.01, 260), IconMuted: ok(0.62, 0.01, 260),
		Error: ok(0.60, 0.22, 25), ErrorForeground: ok(0.95, 0.02, 25), ErrorSurface: ok(0.20, 0.06, 25),
		Warning: ok(0.75, 0.16, 75), WarningForeground: ok(0.95, 0.05, 75), WarningSurface: ok(0.22, 0.05, 75),
		Update: ok(0.65, 0.20, 250), UpdateForeground: ok(0.95, 0.02, 250), UpdateSurface: ok(0.20, 0.05, 250),
		AccentSurface: ok(0.22, 0.05, 250), AccentSurfaceForeground: ok(0.95, 0.02, 250),
		MessageSurface: ok(0.17, 0.005, 260), MessageForeground: ok(0.97, 0.002, 260),
		MessageAction: ok(0.65, 0.20, 250), MessageActionForeground: ok(0.99, 0, 0), MessageActionHover: ok(0.58, 0.20, 250),
		CodeBackground: ok(0.12, 0.003, 260), CodeForeground: ok(0.92, 0.005, 260),
		SearchMatchBackground: ok(0.55, 0.1, 85), SearchMatchForeground: ok(0.970151, 0, 0),
		SearchMatchActiveBackground: ok(0.774202, 0.151946, 74.331), SearchMatchActiveForeground: ok(0.216164, 0.028283, 77.423),
		Sidebar: ok(0.145, 0.004, 260), SidebarForeground: ok(0.96, 0.002, 260),
		SidebarMutedForeground: ok(0.65, 0.01, 260), SidebarControlSurface: ok(0.18, 0.005, 260),
		SidebarRowHover: ok(0.19, 0.005, 260), SidebarRowActive: ok(0.21, 0.006, 260),
		SidebarRowSelected: ok(0.22, 0.006, 260), SidebarBorder: ok(0.22, 0.005, 260),
		TerminalBackground: ok(0.11, 0.002, 260), TerminalForeground: ok(0.96, 0.002, 260),
		TerminalCursor: ok(0.65, 0.20, 250), TerminalSelection: ok(0.28, 0.04, 250),
		TerminalScrollbar: ok(0.22, 0.005, 260), TerminalScrollbarHover: ok(0.32, 0.006, 260),
	}
}

func blossomLight() ThemeColors {
	return ThemeColors{
		Canvas: ok(0.982446, 0.010114, 325.653), Chrome: ok(0.982446, 0.010114, 325.653),
		Toolbar: ok(0.982446, 0.010114, 325.653), ToolbarForeground: ok(0.325698, 0.116116, 325.037),
		ToolbarBorder: ok(0.856784, 0.082879, 328.911), ToolbarControl: ok(0.939552, 0.024286, 321.664),
		ToolbarControlForeground: ok(0.325698, 0.116116, 325.037), ToolbarControlHover: ok(0.884525, 0.041658, 337.177),
		Surface: ok(0.971835, 0.012884, 321.894), SurfaceRaised: ok(0.988235, 0.005049, 325.615), SurfaceOverlay: ok(1, 0, 0),
		Text: ok(0.325698, 0.116116, 325.037), TextMuted: ok(0.494754, 0.190937, 354.544),
		Border: ok(0.923531, 0.021247, 328.096), Input: ok(0.851713, 0.055822, 336.6),
		Focus: ok(0.591646, 0.217985, 0.584), Accent: ok(0.591646, 0.217985, 0.584), AccentForeground: ok(1, 0, 0),
		Secondary: ok(0.869588, 0.06751, 334.899), SecondaryForeground: ok(0.444777, 0.134061, 324.799),
		Muted: ok(0.802407, 0.090963, 345.892), MutedForeground: ok(0.428932, 0.163929, 354.332),
		Placeholder: ok(0.549927, 0.090215, 323.149), SecondaryLabel: ok(0.494754, 0.190937, 354.544), IconMuted: ok(0.494754, 0.190937, 354.544),
		Error: ok(0.627117, 0.248974, 7.734), ErrorForeground: ok(0.458704, 0.169677, 3.815), ErrorSurface: ok(0.942787, 0.032076, 344.963),
		Warning: ok(0.76859, 0.164659, 70.08), WarningForeground: ok(0.54612, 0.143036, 48.949), WarningSurface: ok(0.962901, 0.015297, 48.56),
		Update: ok(0.591646, 0.217985, 0.584), UpdateForeground: ok(0.494754, 0.190937, 354.544), UpdateSurface: ok(0.930264, 0.036194, 341.45),
		AccentSurface: ok(0.939552, 0.024286, 321.664), AccentSurfaceForeground: ok(0.396296, 0.025134, 285.196),
		MessageSurface: ok(0.926746, 0.037898, 332.6), MessageForeground: ok(0.354591, 0.093575, 307.568),
		MessageAction: ok(0.591646, 0.217985, 0.584), MessageActionForeground: ok(1, 0, 0), MessageActionHover: ok(0.539042, 0.197866, 0.305),
		CodeBackground: ok(0.953855, 0.019695, 315.668), CodeForeground: ok(0.445128, 0.13005, 307.026),
		SearchMatchBackground: ok(0.915, 0.11, 85), SearchMatchForeground: ok(0.216164, 0.028283, 77.423),
		SearchMatchActiveBackground: ok(0.774202, 0.151946, 74.331), SearchMatchActiveForeground: ok(0.216164, 0.028283, 77.423),
		Sidebar: ok(0.928886, 0.031178, 322.592), SidebarForeground: ok(0.396296, 0.025134, 285.196),
		SidebarMutedForeground: ok(0.494754, 0.190937, 354.544), SidebarControlSurface: ok(0.978851, 0.001321, 106.424),
		SidebarRowHover: ok(0.978851, 0.001321, 106.424), SidebarRowActive: ok(0.978851, 0.001321, 106.424),
		SidebarRowSelected: ok(0.978851, 0.001321, 106.424), SidebarBorder: ok(0.938313, 0.002552, 48.717),
		TerminalBackground: ok(0.982446, 0.010114, 325.653), TerminalForeground: ok(0.325698, 0.116116, 325.037),
		TerminalCursor: ok(0.591646, 0.217985, 0.584), TerminalSelection: ok(0.869588, 0.06751, 334.899),
		TerminalScrollbar: ok(0.851713, 0.055822, 336.6), TerminalScrollbarHover: ok(0.802407, 0.090963, 345.892),
	}
}

func groveLight() ThemeColors {
	return ThemeColors{
		Canvas: ok(0.972369, 0.005497, 157.15), Chrome: ok(0.972369, 0.005497, 157.15),
		Toolbar: ok(0.972369, 0.005497, 157.15), ToolbarForeground: ok(0.222003, 0.03479, 328.979),
		ToolbarBorder: ok(0.909438, 0.021521, 164.612), ToolbarControl: ok(0.936464, 0.014601, 163.554),
		ToolbarControlForeground: ok(0.222003, 0.03479, 328.979), ToolbarControlHover: ok(0.909438, 0.021521, 164.612),
		Surface: ok(0.972369, 0.005497, 157.15), SurfaceRaised: ok(0.949276, 0.004496, 159.002), SurfaceOverlay: ok(0.932695, 0.003778, 160.944),
		Text: ok(0.222003, 0.03479, 328.979), TextMuted: ok(0.540472, 0.014944, 326.176),
		Border: ok(0.864831, 0.01312, 167.255), Input: ok(0.829746, 0.016084, 168.234),
		Focus: ok(0.523295, 0.112292, 158.089), Accent: ok(0.523295, 0.112292, 158.089), AccentForeground: ok(0.990339, 0.008411, 325.64),
		Secondary: ok(0.936464, 0.014601, 163.554), SecondaryForeground: ok(0.222003, 0.03479, 328.979),
		Muted: ok(0.945455, 0.012308, 162.879), MutedForeground: ok(0.527266, 0.012309, 320.683),
		Placeholder: ok(0.529681, 0.01551, 326.299), SecondaryLabel: ok(0.540472, 0.014944, 326.176), IconMuted: ok(0.540472, 0.014944, 326.176),
		Error: ok(0.637823, 0.237287, 25.436), ErrorForeground: ok(0.509494, 0.208583, 28.513), ErrorSurface: ok(0.936968, 0.014243, 26.295),
		Warning: ok(0.772406, 0.172798, 65.367), WarningForeground: ok(0.545036, 0.155019, 45.359), WarningSurface: ok(0.953175, 0.02009, 93.379),
		Update: ok(0.523295, 0.112292, 158.089), UpdateForeground: ok(0.388012, 0.080082, 158.768), UpdateSurface: ok(0.900411, 0.02384, 164.795),
		AccentSurface: ok(0.909438, 0.021521, 164.612), AccentSurfaceForeground: ok(0.222003, 0.03479, 328.979),
		MessageSurface: ok(0.891377, 0.026164, 164.929), MessageForeground: ok(0.222003, 0.03479, 328.979),
		MessageAction: ok(0.535028, 0.106403, 77.549), MessageActionForeground: ok(0.990339, 0.008411, 325.64), MessageActionHover: ok(0.488753, 0.096536, 77.829),
		CodeBackground: ok(0.955888, 0.004783, 158.391), CodeForeground: ok(0.222003, 0.03479, 328.979),
		SearchMatchBackground: ok(0.915, 0.11, 85), SearchMatchForeground: ok(0.216164, 0.028283, 77.423),
		SearchMatchActiveBackground: ok(0.774202, 0.151946, 74.331), SearchMatchActiveForeground: ok(0.216164, 0.028283, 77.423),
		Sidebar: ok(0.936464, 0.014601, 163.554), SidebarForeground: ok(0.222003, 0.03479, 328.979),
		SidebarMutedForeground: ok(0.515606, 0.011938, 318.897), SidebarControlSurface: ok(0.88585, 0.011734, 166.331),
		SidebarRowHover: ok(0.886676, 0.027374, 164.983), SidebarRowActive: ok(0.85335, 0.03597, 165.158),
		SidebarRowSelected: ok(0.836654, 0.040284, 165.149), SidebarBorder: ok(0.860274, 0.010287, 168.339),
		TerminalBackground: ok(0.972369, 0.005497, 157.15), TerminalForeground: ok(0.222003, 0.03479, 328.979),
		TerminalCursor: ok(0.523295, 0.112292, 158.089), TerminalSelection: ok(0.891377, 0.026164, 164.929),
		TerminalScrollbar: ok(0.824752, 0.001392, 294.641), TerminalScrollbarHover: ok(0.755495, 0.004415, 318.776),
	}
}

func groveDark() ThemeColors {
	return ThemeColors{
		Canvas: ok(0.260865, 0.02152, 162.75), Chrome: ok(0.260865, 0.02152, 162.75),
		Toolbar: ok(0.260865, 0.02152, 162.75), ToolbarForeground: ok(0.990339, 0.008411, 325.64),
		ToolbarBorder: ok(0.464636, 0.066083, 158.72), ToolbarControl: ok(0.380487, 0.048313, 159.608),
		ToolbarControlForeground: ok(0.990339, 0.008411, 325.64), ToolbarControlHover: ok(0.437021, 0.060312, 158.962),
		Surface: ok(0.260865, 0.02152, 162.75), SurfaceRaised: ok(0.363192, 0.016572, 165.32), SurfaceOverlay: ok(0.411828, 0.014378, 166.627),
		Text: ok(0.990339, 0.008411, 325.64), TextMuted: ok(0.666747, 0.004239, 187.292),
		Border: ok(0.457475, 0.044046, 160.971), Input: ok(0.519849, 0.049896, 160.863),
		Focus: ok(0.796228, 0.133058, 157.319), Accent: ok(0.796228, 0.133058, 157.319), AccentForeground: ok(0.222003, 0.03479, 328.979),
		Secondary: ok(0.380487, 0.048313, 159.608), SecondaryForeground: ok(0.990339, 0.008411, 325.64),
		Muted: ok(0.339728, 0.039456, 160.274), MutedForeground: ok(0.715427, 0.010896, 171.428),
		Placeholder: ok(0.739243, 0.002222, 223.225), SecondaryLabel: ok(0.666747, 0.004239, 187.292), IconMuted: ok(0.666747, 0.004239, 187.292),
		Error: ok(0.655108, 0.221148, 23.473), ErrorForeground: ok(0.704237, 0.187511, 22.228), ErrorSurface: ok(0.312773, 0.02923, 32.121),
		Warning: ok(0.772406, 0.172798, 65.367), WarningForeground: ok(0.829017, 0.171221, 81.038), WarningSurface: ok(0.345524, 0.046882, 99.736),
		Update: ok(0.796228, 0.133058, 157.319), UpdateForeground: ok(0.86276, 0.089288, 159.704), UpdateSurface: ok(0.448116, 0.062637, 158.86),
		AccentSurface: ok(0.437021, 0.060312, 158.962), AccentSurfaceForeground: ok(0.990339, 0.008411, 325.64),
		MessageSurface: ok(0.470111, 0.067221, 158.676), MessageForeground: ok(0.990339, 0.008411, 325.64),
		MessageAction: ok(0.791603, 0.129713, 83.299), MessageActionForeground: ok(0.222003, 0.03479, 328.979), MessageActionHover: ok(0.815227, 0.117902, 84.21),
		CodeBackground: ok(0.312979, 0.018942, 164.082), CodeForeground: ok(0.990339, 0.008411, 325.64),
		SearchMatchBackground: ok(0.55, 0.1, 85), SearchMatchForeground: ok(0.970151, 0, 0),
		SearchMatchActiveBackground: ok(0.774202, 0.151946, 74.331), SearchMatchActiveForeground: ok(0.216164, 0.028283, 77.423),
		Sidebar: ok(0.309925, 0.032827, 160.944), SidebarForeground: ok(0.990339, 0.008411, 325.64),
		SidebarMutedForeground: ok(0.711387, 0.007643, 175.89), SidebarControlSurface: ok(0.432727, 0.024549, 163.654),
		SidebarRowHover: ok(0.374959, 0.047124, 159.686), SidebarRowActive: ok(0.41688, 0.056069, 159.165),
		SidebarRowSelected: ok(0.437466, 0.060406, 158.958), SidebarBorder: ok(0.569253, 0.015933, 167.062),
		TerminalBackground: ok(0.260865, 0.02152, 162.75), TerminalForeground: ok(0.990339, 0.008411, 325.64),
		TerminalCursor: ok(0.796228, 0.133058, 157.319), TerminalSelection: ok(0.464636, 0.066083, 158.72),
		TerminalScrollbar: ok(0.594692, 0.006862, 176.022), TerminalScrollbarHover: ok(0.687968, 0.00354, 193.55),
	}
}

func oceanLight() ThemeColors {
	return ThemeColors{
		Canvas: ok(0.974199, 0.002856, 241.597), Chrome: ok(0.974199, 0.002856, 241.597),
		Toolbar: ok(0.974199, 0.002856, 241.597), ToolbarForeground: ok(0.222003, 0.03479, 328.979),
		ToolbarBorder: ok(0.91295, 0.018827, 241.836), ToolbarControl: ok(0.939254, 0.01193, 241.729),
		ToolbarControlForeground: ok(0.222003, 0.03479, 328.979), ToolbarControlHover: ok(0.91295, 0.018827, 241.836),
		Surface: ok(0.974199, 0.002856, 241.597), SurfaceRaised: ok(0.951058, 0.002962, 258.339), SurfaceOverlay: ok(0.934442, 0.003181, 269.1),
		Text: ok(0.222003, 0.03479, 328.979), TextMuted: ok(0.541555, 0.017468, 323.531),
		Border: ok(0.867646, 0.013482, 252.362), Input: ok(0.832939, 0.017389, 252.598),
		Focus: ok(0.536684, 0.120219, 247.01), Accent: ok(0.536684, 0.120219, 247.01), AccentForeground: ok(0.990339, 0.008411, 325.64),
		Secondary: ok(0.939254, 0.01193, 241.729), SecondaryForeground: ok(0.222003, 0.03479, 328.979),
		Muted: ok(0.948004, 0.009649, 241.695), MutedForeground: ok(0.528741, 0.01828, 313.823),
		Placeholder: ok(0.530733, 0.01795, 323.79), SecondaryLabel: ok(0.541555, 0.017468, 323.531), IconMuted: ok(0.541555, 0.017468, 323.531),
		Error: ok(0.637823, 0.237287, 25.436), ErrorForeground: ok(0.509494, 0.208583, 28.513), ErrorSurface: ok(0.938747, 0.016377, 7.186),
		Warning: ok(0.772406, 0.172798, 65.367), WarningForeground: ok(0.546927, 0.155556, 45.359), WarningSurface: ok(0.954846, 0.016009, 81.731),
		Update: ok(0.536684, 0.120219, 247.01), UpdateForeground: ok(0.397497, 0.084999, 246.523), UpdateSurface: ok(0.904165, 0.021144, 241.874),
		AccentSurface: ok(0.91295, 0.018827, 241.836), AccentSurfaceForeground: ok(0.222003, 0.03479, 328.979),
		MessageSurface: ok(0.895373, 0.023469, 241.913), MessageForeground: ok(0.222003, 0.03479, 328.979),
		MessageAction: ok(0.493961, 0.08175, 201.584), MessageActionForeground: ok(0.990339, 0.008411, 325.64), MessageActionHover: ok(0.45151, 0.074407, 201.516),
		CodeBackground: ok(0.957684, 0.002906, 253.68), CodeForeground: ok(0.222003, 0.03479, 328.979),
		SearchMatchBackground: ok(0.915, 0.11, 85), SearchMatchForeground: ok(0.216164, 0.028283, 77.423),
		SearchMatchActiveBackground: ok(0.774202, 0.151946, 74.331), SearchMatchActiveForeground: ok(0.216164, 0.028283, 77.423),
		Sidebar: ok(0.939254, 0.01193, 241.729), SidebarForeground: ok(0.222003, 0.03479, 328.979),
		SidebarMutedForeground: ok(0.517366, 0.018944, 311.433), SidebarControlSurface: ok(0.888479, 0.011475, 251.638),
		SidebarRowHover: ok(0.890798, 0.024681, 241.933), SidebarRowActive: ok(0.858363, 0.033325, 242.089),
		SidebarRowSelected: ok(0.842113, 0.037689, 242.174), SidebarBorder: ok(0.862823, 0.011384, 256.926),
		TerminalBackground: ok(0.974199, 0.002856, 241.597), TerminalForeground: ok(0.222003, 0.03479, 328.979),
		TerminalCursor: ok(0.536684, 0.120219, 247.01), TerminalSelection: ok(0.895373, 0.023469, 241.913),
		TerminalScrollbar: ok(0.826271, 0.006191, 305.456), TerminalScrollbarHover: ok(0.756866, 0.008685, 313.721),
	}
}

func oceanDark() ThemeColors {
	return ThemeColors{
		Canvas: ok(0.242641, 0.024125, 250.573), Chrome: ok(0.242641, 0.024125, 250.573),
		Toolbar: ok(0.242641, 0.024125, 250.573), ToolbarForeground: ok(0.990339, 0.008411, 325.64),
		ToolbarBorder: ok(0.439946, 0.0561, 243.479), ToolbarControl: ok(0.358725, 0.043145, 244.911),
		ToolbarControlForeground: ok(0.990339, 0.008411, 325.64), ToolbarControlHover: ok(0.413315, 0.051874, 243.855),
		Surface: ok(0.242641, 0.024125, 250.573), SurfaceRaised: ok(0.348439, 0.019942, 253.696), SurfaceOverlay: ok(0.398517, 0.018232, 255.72),
		Text: ok(0.990339, 0.008411, 325.64), TextMuted: ok(0.652227, 0.01149, 273.31),
		Border: ok(0.438653, 0.039496, 245.44), Input: ok(0.500905, 0.043574, 244.781),
		Focus: ok(0.758933, 0.105833, 241.548), Accent: ok(0.758933, 0.105833, 241.548), AccentForeground: ok(0.222003, 0.03479, 328.979),
		Secondary: ok(0.358725, 0.043145, 244.911), SecondaryForeground: ok(0.990339, 0.008411, 325.64),
		Muted: ok(0.319287, 0.036766, 246.065), MutedForeground: ok(0.691936, 0.016294, 261.588),
		Placeholder: ok(0.721641, 0.010192, 281.271), SecondaryLabel: ok(0.652227, 0.01149, 273.31), IconMuted: ok(0.652227, 0.01149, 273.31),
		Error: ok(0.655108, 0.221148, 23.473), ErrorForeground: ok(0.702184, 0.189226, 22.228), ErrorSurface: ok(0.298933, 0.036443, 350.094),
		Warning: ok(0.772406, 0.172798, 65.367), WarningForeground: ok(0.829017, 0.171221, 81.038), WarningSurface: ok(0.329449, 0.028712, 84.495),
		Update: ok(0.758933, 0.105833, 241.548), UpdateForeground: ok(0.840844, 0.069217, 240.151), UpdateSurface: ok(0.424017, 0.053575, 243.695),
		AccentSurface: ok(0.413315, 0.051874, 243.855), AccentSurfaceForeground: ok(0.990339, 0.008411, 325.64),
		MessageSurface: ok(0.445224, 0.056936, 243.413), MessageForeground: ok(0.990339, 0.008411, 325.64),
		MessageAction: ok(0.793363, 0.105022, 199.893), MessageActionForeground: ok(0.222003, 0.03479, 328.979), MessageActionHover: ok(0.815308, 0.096174, 199.862),
		CodeBackground: ok(0.29661, 0.021883, 251.968), CodeForeground: ok(0.990339, 0.008411, 325.64),
		SearchMatchBackground: ok(0.55, 0.1, 85), SearchMatchForeground: ok(0.970151, 0, 0),
		SearchMatchActiveBackground: ok(0.774202, 0.151946, 74.331), SearchMatchActiveForeground: ok(0.216164, 0.028283, 77.423),
		Sidebar: ok(0.290387, 0.032043, 247.274), SidebarForeground: ok(0.990339, 0.008411, 325.64),
		SidebarMutedForeground: ok(0.69099, 0.01395, 266.424), SidebarControlSurface: ok(0.417822, 0.02535, 250.162),
		SidebarRowHover: ok(0.353381, 0.042285, 245.043), SidebarRowActive: ok(0.393878, 0.048778, 244.179),
		SidebarRowSelected: ok(0.413744, 0.051943, 243.848), SidebarBorder: ok(0.55859, 0.019001, 256.223),
		TerminalBackground: ok(0.242641, 0.024125, 250.573), TerminalForeground: ok(0.990339, 0.008411, 325.64),
		TerminalCursor: ok(0.758933, 0.105833, 241.548), TerminalSelection: ok(0.439946, 0.0561, 243.479),
		TerminalScrollbar: ok(0.58613, 0.012959, 267.22), TerminalScrollbarHover: ok(0.681569, 0.010909, 276.465),
	}
}

func emberLight() ThemeColors {
	return ThemeColors{
		Canvas: ok(0.976527, 0.002685, 60.725), Chrome: ok(0.976527, 0.002685, 60.725),
		Toolbar: ok(0.976527, 0.002685, 60.725), ToolbarForeground: ok(0.222003, 0.03479, 328.979),
		ToolbarBorder: ok(0.916502, 0.01832, 49.597), ToolbarControl: ok(0.942267, 0.01151, 50.785),
		ToolbarControlForeground: ok(0.222003, 0.03479, 328.979), ToolbarControlHover: ok(0.916502, 0.01832, 49.597),
		Surface: ok(0.976527, 0.002685, 60.725), SurfaceRaised: ok(0.953321, 0.002701, 42.266), SurfaceOverlay: ok(0.936659, 0.002879, 29.96),
		Text: ok(0.222003, 0.03479, 328.979), TextMuted: ok(0.543023, 0.017316, 331.964),
		Border: ok(0.870631, 0.013204, 39.431), Input: ok(0.836213, 0.017153, 38.661),
		Focus: ok(0.552831, 0.129438, 44.656), Accent: ok(0.552831, 0.129438, 44.656), AccentForeground: ok(0.990339, 0.008411, 325.64),
		Secondary: ok(0.942267, 0.01151, 50.785), SecondaryForeground: ok(0.222003, 0.03479, 328.979),
		Muted: ok(0.950842, 0.009273, 51.528), MutedForeground: ok(0.530413, 0.018453, 341.181),
		Placeholder: ok(0.532339, 0.017796, 331.748), SecondaryLabel: ok(0.543023, 0.017316, 331.964), IconMuted: ok(0.543023, 0.017316, 331.964),
		Error: ok(0.637823, 0.237287, 25.436), ErrorForeground: ok(0.509494, 0.208583, 28.513), ErrorSurface: ok(0.941094, 0.019938, 19.375),
		Warning: ok(0.772406, 0.172798, 65.367), WarningForeground: ok(0.549154, 0.156188, 45.359), WarningSurface: ok(0.957148, 0.020843, 76.702),
		Update: ok(0.552831, 0.129438, 44.656), UpdateForeground: ok(0.408647, 0.091207, 45.037), UpdateSurface: ok(0.907902, 0.020621, 49.36),
		AccentSurface: ok(0.916502, 0.01832, 49.597), AccentSurfaceForeground: ok(0.222003, 0.03479, 328.979),
		MessageSurface: ok(0.899296, 0.022939, 49.163), MessageForeground: ok(0.222003, 0.03479, 328.979),
		MessageAction: ok(0.516323, 0.161628, 24.82), MessageActionForeground: ok(0.990339, 0.008411, 325.64), MessageActionHover: ok(0.471223, 0.145843, 24.688),
		CodeBackground: ok(0.959965, 0.002668, 47.512), CodeForeground: ok(0.222003, 0.03479, 328.979),
		SearchMatchBackground: ok(0.915, 0.11, 85), SearchMatchForeground: ok(0.216164, 0.028283, 77.423),
		SearchMatchActiveBackground: ok(0.774202, 0.151946, 74.331), SearchMatchActiveForeground: ok(0.216164, 0.028283, 77.423),
		Sidebar: ok(0.942267, 0.01151, 50.785), SidebarForeground: ok(0.222003, 0.03479, 328.979),
		SidebarMutedForeground: ok(0.519146, 0.019214, 343.427), SidebarControlSurface: ok(0.891332, 0.011179, 40.596),
		SidebarRowHover: ok(0.894819, 0.024151, 49.073), SidebarRowActive: ok(0.863104, 0.032855, 48.586),
		SidebarRowSelected: ok(0.84723, 0.037292, 48.403), SidebarBorder: ok(0.865593, 0.011154, 35.246),
		TerminalBackground: ok(0.976527, 0.002685, 60.725), TerminalForeground: ok(0.222003, 0.03479, 328.979),
		TerminalCursor: ok(0.552831, 0.129438, 44.656), TerminalSelection: ok(0.899296, 0.022939, 49.163),
		TerminalScrollbar: ok(0.828185, 0.005884, 349.533), TerminalScrollbarHover: ok(0.758584, 0.008423, 341.16),
	}
}

func emberDark() ThemeColors {
	return ThemeColors{
		Canvas: ok(0.245899, 0.019144, 42.044), Chrome: ok(0.245899, 0.019144, 42.044),
		Toolbar: ok(0.245899, 0.019144, 42.044), ToolbarForeground: ok(0.990339, 0.008411, 325.64),
		ToolbarBorder: ok(0.442681, 0.0608, 50.795), ToolbarControl: ok(0.361499, 0.044052, 49.515),
		ToolbarControlForeground: ok(0.990339, 0.008411, 325.64), ToolbarControlHover: ok(0.416048, 0.055354, 50.484),
		Surface: ok(0.245899, 0.019144, 42.044), SurfaceRaised: ok(0.351262, 0.01565, 37.592), SurfaceOverlay: ok(0.401111, 0.014308, 34.896),
		Text: ok(0.990339, 0.008411, 325.64), TextMuted: ok(0.654017, 0.009505, 13.287),
		Border: ok(0.44099, 0.040202, 48.807), Input: ok(0.503003, 0.045721, 49.44),
		Focus: ok(0.762174, 0.124117, 52.082), Accent: ok(0.762174, 0.124117, 52.082), AccentForeground: ok(0.222003, 0.03479, 328.979),
		Secondary: ok(0.361499, 0.044052, 49.515), SecondaryForeground: ok(0.990339, 0.008411, 325.64),
		Muted: ok(0.322144, 0.03574, 48.309), MutedForeground: ok(0.692479, 0.015227, 30.963),
		Placeholder: ok(0.723533, 0.008741, 4.515), SecondaryLabel: ok(0.654017, 0.009505, 13.287), IconMuted: ok(0.654017, 0.009505, 13.287),
		Error: ok(0.655108, 0.221148, 23.473), ErrorForeground: ok(0.702184, 0.189226, 22.228), ErrorSurface: ok(0.310955, 0.059624, 24.334),
		Warning: ok(0.772406, 0.172798, 65.367), WarningForeground: ok(0.829017, 0.171221, 81.038), WarningSurface: ok(0.339137, 0.055638, 66.911),
		Update: ok(0.762174, 0.124117, 52.082), UpdateForeground: ok(0.841456, 0.079585, 53.521), UpdateSurface: ok(0.426749, 0.057547, 50.618),
		AccentSurface: ok(0.416048, 0.055354, 50.484), AccentSurfaceForeground: ok(0.990339, 0.008411, 325.64),
		MessageSurface: ok(0.447961, 0.061874, 50.849), MessageForeground: ok(0.990339, 0.008411, 325.64),
		MessageAction: ok(0.747955, 0.135578, 29.432), MessageActionForeground: ok(0.222003, 0.03479, 328.979), MessageActionHover: ok(0.775116, 0.117953, 29.014),
		CodeBackground: ok(0.299662, 0.017229, 39.973), CodeForeground: ok(0.990339, 0.008411, 325.64),
		SearchMatchBackground: ok(0.55, 0.1, 85), SearchMatchForeground: ok(0.970151, 0, 0),
		SearchMatchActiveBackground: ok(0.774202, 0.151946, 74.331), SearchMatchActiveForeground: ok(0.216164, 0.028283, 77.423),
		Sidebar: ok(0.293349, 0.029554, 46.882), SidebarForeground: ok(0.990339, 0.008411, 325.64),
		SidebarMutedForeground: ok(0.691874, 0.012538, 24.638), SidebarControlSurface: ok(0.420227, 0.022893, 43.226),
		SidebarRowHover: ok(0.356163, 0.042933, 49.385), SidebarRowActive: ok(0.396617, 0.051353, 50.201),
		SidebarRowSelected: ok(0.416477, 0.055442, 50.489), SidebarBorder: ok(0.560372, 0.016998, 36.179),
		TerminalBackground: ok(0.245899, 0.019144, 42.044), TerminalForeground: ok(0.990339, 0.008411, 325.64),
		TerminalCursor: ok(0.762174, 0.124117, 52.082), TerminalSelection: ok(0.442681, 0.0608, 50.795),
		TerminalScrollbar: ok(0.587861, 0.010463, 20.444), TerminalScrollbarHover: ok(0.682876, 0.009156, 9.796),
	}
}

func irisLight() ThemeColors {
	return ThemeColors{
		Canvas: ok(0.976531, 0.003855, 303.226), Chrome: ok(0.976531, 0.003855, 303.226),
		Toolbar: ok(0.976531, 0.003855, 303.226), ToolbarForeground: ok(0.222003, 0.03479, 328.979),
		ToolbarBorder: ok(0.914882, 0.022965, 299.986), ToolbarControl: ok(0.941387, 0.014687, 300.474),
		ToolbarControlForeground: ok(0.222003, 0.03479, 328.979), ToolbarControlHover: ok(0.914882, 0.022965, 299.986),
		Surface: ok(0.976531, 0.003855, 303.226), SurfaceRaised: ok(0.953326, 0.004536, 307.676), SurfaceOverlay: ok(0.936665, 0.005041, 310.132),
		Text: ok(0.222003, 0.03479, 328.979), TextMuted: ok(0.543042, 0.018894, 325.652),
		Border: ok(0.869608, 0.018226, 303.859), Input: ok(0.834773, 0.023405, 303.676),
		Focus: ok(0.525348, 0.15373, 294.176), Accent: ok(0.525348, 0.15373, 294.176), AccentForeground: ok(0.990339, 0.008411, 325.64),
		Secondary: ok(0.941387, 0.014687, 300.474), SecondaryForeground: ok(0.222003, 0.03479, 328.979),
		Muted: ok(0.950194, 0.011956, 300.733), MutedForeground: ok(0.529955, 0.022319, 321.556),
		Placeholder: ok(0.532177, 0.019333, 325.784), SecondaryLabel: ok(0.543042, 0.018894, 325.652), IconMuted: ok(0.543042, 0.018894, 325.652),
		Error: ok(0.637823, 0.237287, 25.436), ErrorForeground: ok(0.509494, 0.208583, 28.513), ErrorSurface: ok(0.941043, 0.019582, 4.235),
		Warning: ok(0.772406, 0.172798, 65.367), WarningForeground: ok(0.549154, 0.156188, 45.359), WarningSurface: ok(0.957054, 0.016197, 69.932),
		Update: ok(0.525348, 0.15373, 294.176), UpdateForeground: ok(0.389926, 0.10825, 294.547), UpdateSurface: ok(0.90602, 0.025754, 299.867),
		AccentSurface: ok(0.914882, 0.022965, 299.986), AccentSurfaceForeground: ok(0.222003, 0.03479, 328.979),
		MessageSurface: ok(0.897143, 0.028558, 299.758), MessageForeground: ok(0.222003, 0.03479, 328.979),
		MessageAction: ok(0.516084, 0.185229, 340.776), MessageActionForeground: ok(0.990339, 0.008411, 325.64), MessageActionHover: ok(0.471003, 0.16748, 340.687),
		CodeBackground: ok(0.95997, 0.004338, 306.542), CodeForeground: ok(0.222003, 0.03479, 328.979),
		SearchMatchBackground: ok(0.915, 0.11, 85), SearchMatchForeground: ok(0.216164, 0.028283, 77.423),
		SearchMatchActiveBackground: ok(0.774202, 0.151946, 74.331), SearchMatchActiveForeground: ok(0.216164, 0.028283, 77.423),
		Sidebar: ok(0.941387, 0.014687, 300.474), SidebarForeground: ok(0.222003, 0.03479, 328.979),
		SidebarMutedForeground: ok(0.518417, 0.023683, 320.681), SidebarControlSurface: ok(0.890512, 0.0155, 303.803),
		SidebarRowHover: ok(0.892522, 0.030022, 299.704), SidebarRowActive: ok(0.85971, 0.040501, 299.36),
		SidebarRowSelected: ok(0.843236, 0.045818, 299.198), SidebarBorder: ok(0.864805, 0.015938, 305.371),
		TerminalBackground: ok(0.976531, 0.003855, 303.226), TerminalForeground: ok(0.222003, 0.03479, 328.979),
		TerminalCursor: ok(0.525348, 0.15373, 294.176), TerminalSelection: ok(0.897143, 0.028558, 299.758),
		TerminalScrollbar: ok(0.828195, 0.008526, 318.858), TerminalScrollbarHover: ok(0.758596, 0.010892, 321.538),
	}
}

func irisDark() ThemeColors {
	return ThemeColors{
		Canvas: ok(0.225975, 0.031062, 293.741), Chrome: ok(0.225975, 0.031062, 293.741),
		Toolbar: ok(0.225975, 0.031062, 293.741), ToolbarForeground: ok(0.990339, 0.008411, 325.64),
		ToolbarBorder: ok(0.395417, 0.085554, 294.182), ToolbarControl: ok(0.325405, 0.063614, 294.23),
		ToolbarControlForeground: ok(0.990339, 0.008411, 325.64), ToolbarControlHover: ok(0.372436, 0.07841, 294.204),
		Surface: ok(0.225975, 0.031062, 293.741), SurfaceRaised: ok(0.335291, 0.026008, 296.394), SurfaceOverlay: ok(0.386739, 0.024023, 297.509),
		Text: ok(0.990339, 0.008411, 325.64), TextMuted: ok(0.640465, 0.016197, 304.171),
		Border: ok(0.40874, 0.058536, 295.893), Input: ok(0.46756, 0.065775, 296.265),
		Focus: ok(0.671712, 0.169136, 293.929), Accent: ok(0.671712, 0.169136, 293.929), AccentForeground: ok(0.222003, 0.03479, 328.979),
		Secondary: ok(0.325405, 0.063614, 294.23), SecondaryForeground: ok(0.990339, 0.008411, 325.64),
		Muted: ok(0.291515, 0.05276, 294.209), MutedForeground: ok(0.663321, 0.025932, 301.862),
		Placeholder: ok(0.706249, 0.014508, 306.607), SecondaryLabel: ok(0.640465, 0.016197, 304.171), IconMuted: ok(0.640465, 0.016197, 304.171),
		Error: ok(0.655108, 0.221148, 23.473), ErrorForeground: ok(0.702184, 0.189226, 22.228), ErrorSurface: ok(0.291658, 0.054707, 352.238),
		Warning: ok(0.772406, 0.172798, 65.367), WarningForeground: ok(0.829017, 0.171221, 81.038), WarningSurface: ok(0.318952, 0.033845, 51.646),
		Update: ok(0.671712, 0.169136, 293.929), UpdateForeground: ok(0.785032, 0.108439, 296.344), UpdateSurface: ok(0.381668, 0.081286, 294.195),
		AccentSurface: ok(0.372436, 0.07841, 294.204), AccentSurfaceForeground: ok(0.990339, 0.008411, 325.64),
		MessageSurface: ok(0.399975, 0.086965, 294.177), MessageForeground: ok(0.990339, 0.008411, 325.64),
		MessageAction: ok(0.789904, 0.130063, 337.621), MessageActionForeground: ok(0.222003, 0.03479, 328.979), MessageActionHover: ok(0.813537, 0.114101, 337.23),
		CodeBackground: ok(0.281873, 0.028308, 295.193), CodeForeground: ok(0.990339, 0.008411, 325.64),
		SearchMatchBackground: ok(0.55, 0.1, 85), SearchMatchForeground: ok(0.970151, 0, 0),
		SearchMatchActiveBackground: ok(0.774202, 0.151946, 74.331), SearchMatchActiveForeground: ok(0.216164, 0.028283, 77.423),
		Sidebar: ok(0.266743, 0.044689, 294.138), SidebarForeground: ok(0.990339, 0.008411, 325.64),
		SidebarMutedForeground: ok(0.668773, 0.021522, 302.949), SidebarControlSurface: ok(0.399977, 0.035678, 297.031),
		SidebarRowHover: ok(0.320808, 0.062152, 294.23), SidebarRowActive: ok(0.355677, 0.073167, 294.217),
		SidebarRowSelected: ok(0.372806, 0.078525, 294.203), SidebarBorder: ok(0.545895, 0.027522, 299.871),
		TerminalBackground: ok(0.225975, 0.031062, 293.741), TerminalForeground: ok(0.990339, 0.008411, 325.64),
		TerminalCursor: ok(0.671712, 0.169136, 293.929), TerminalSelection: ok(0.395417, 0.085554, 294.182),
		TerminalScrollbar: ok(0.578663, 0.017888, 302.229), TerminalScrollbarHover: ok(0.676012, 0.015271, 305.433),
	}
}

// addSemanticExtras fills the palette-independent roles index.css defines
// once for both appearances and the ones that differ by appearance only.
func addSemanticExtras(tc *ThemeColors, dark bool) {
	if !dark {
		tc.Success = ok(0.696, 0.17, 162.48)                 // emerald-500
		tc.SuccessForeground = ok(0.627, 0.147, 163)         // emerald-600
		tc.Info = ok(0.623, 0.214, 259.815)                  // blue-500
		tc.InfoForeground = ok(0.546, 0.215, 262.881)        // blue-700
		tc.DiffAdditionForeground = ok(0.627, 0.147, 163)    // emerald-600
		tc.DiffDeletionForeground = ok(0.577, 0.245, 27.325) // red-600
	} else {
		tc.Success = ok(0.696, 0.17, 162.48)                 // emerald-500
		tc.SuccessForeground = ok(0.704, 0.191, 162)         // emerald-400
		tc.Info = ok(0.623, 0.214, 259.815)                  // blue-500
		tc.InfoForeground = ok(0.706, 0.165, 254.624)        // blue-400
		tc.DiffAdditionForeground = ok(0.704, 0.191, 162)    // emerald-400
		tc.DiffDeletionForeground = ok(0.704, 0.191, 22.216) // red-400
	}
	tc.DiffAddition = tc.Success
	tc.DiffDeletion = tc.Error
	tc.FullAccess = ui.Hex("#f97316")
	// Commit-graph lane colors of index.css (--graph-c0..c9).
	graphHexes := [10]string{
		"#a78bfa", "#22d3ee", "#34d399", "#fbbf24", "#fb7185",
		"#60a5fa", "#c084fc", "#f472b6", "#2dd4bf", "#facc15",
	}
	for i, hx := range graphHexes {
		tc.GraphColors[i] = ui.Hex(hx)
	}
}

// PaletteColors returns the full role set of one built-in palette in one
// appearance. This is ForgeADE's getThemeColorsForAppearance.
func PaletteColors(paletteID string, dark bool) ThemeColors {
	tc := func() ThemeColors {
		switch paletteID {
		case ThemePaletteBlossom:
			if dark {
				return forgeDefaultDark() // blossom's dark variant is forge-default's
			}
			return blossomLight()
		case ThemePaletteGrove:
			if dark {
				return groveDark()
			}
			return groveLight()
		case ThemePaletteOcean:
			if dark {
				return oceanDark()
			}
			return oceanLight()
		case ThemePaletteEmber:
			if dark {
				return emberDark()
			}
			return emberLight()
		case ThemePaletteIris:
			if dark {
				return irisDark()
			}
			return irisLight()
		default: // ThemePaletteDefault
			if dark {
				return forgeDefaultDark()
			}
			return forgeDefaultLight()
		}
	}()
	addSemanticExtras(&tc, dark)
	return tc
}

// MapTheme projects the ForgeADE roles onto MyGo's ui.Theme so every stock
// widget paints with the original app's palette.
func MapTheme(tc ThemeColors, dark bool) *ui.Theme {
	var t *ui.Theme
	if dark {
		t = ui.DarkTheme()
	} else {
		t = ui.LightTheme()
	}
	t.Background = tc.Canvas
	t.Surface = tc.Surface
	t.SurfaceHover = tc.ToolbarControlHover
	t.SurfacePressed = tc.SidebarRowActive
	t.Border = tc.Border
	t.Text = tc.Text
	t.TextMuted = tc.TextMuted
	t.Accent = tc.Accent
	t.AccentHover = tc.MessageActionHover
	t.AccentPressed = tc.MessageActionHover
	t.AccentText = tc.AccentForeground
	t.InactiveAccent = tc.Accent
	t.Danger = tc.Error
	t.Warning = tc.Warning
	t.Success = tc.Success
	// ::selection { color-mix(in oklab, var(--focus) 82%, white 18%) }
	t.Selection = tc.Focus.MixOklab(ui.RGB(255, 255, 255), 0.18)
	t.Focus = tc.Focus
	// Tooltips and toasts are the theme turned over.
	t.Inverse = tc.Text
	t.InverseText = tc.Canvas
	t.Scrollbar = tc.TerminalScrollbar
	t.Radius = 10 // --radius: 0.625rem
	t.Font = FontSans
	return t
}
