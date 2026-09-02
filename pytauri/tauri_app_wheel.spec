# -*- mode: python ; coding: utf-8 -*-
"""PyInstaller spec for the pytauri-wheel reference app.

Builds an onedir macOS app bundle. Key points:

- `pytauri_wheel` carries the precompiled Tauri dylib as a PyO3 extension
  module — collect_all() pulls the binary extension in.
- `pytauri_plugins` is a namespace package with many submodules — collected
  explicitly.
- `SRC_TAURI_DIR = Path(__file__).parent` in tauri_app_wheel resolves inside
  the bundle to `<_internal>/tauri_app_wheel`, so Tauri.toml / capabilities /
  icons / frontend must be added as datas under that name.
"""

import os
from PyInstaller.utils.hooks import collect_all

datas = []
binaries = []
hiddenimports = []

for pkg in ("pytauri", "pytauri_plugins", "pytauri_wheel"):
    d, b, h = collect_all(pkg)
    datas += d
    binaries += b
    hiddenimports += h

PKG = "tauri_app_wheel"

datas += [
    (f"python/src/{PKG}/Tauri.toml", PKG),
    (f"python/src/{PKG}/capabilities", f"{PKG}/capabilities"),
    (f"python/src/{PKG}/icons", f"{PKG}/icons"),
    (f"python/src/{PKG}/frontend", f"{PKG}/frontend"),
    (f"python/src/{PKG}/py.typed", PKG),
]

a = Analysis(
    ["run_app.py"],
    pathex=[],
    binaries=binaries,
    datas=datas,
    hiddenimports=hiddenimports,
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=[],
    noarchive=False,
)
pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name="tauri-app-wheel",
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=False,
    console=False,
    disable_windowed_traceback=False,
    argv_emulation=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
)

coll = COLLECT(
    exe,
    a.binaries,
    a.datas,
    strip=False,
    upx=False,
    upx_exclude=[],
    name="tauri-app-wheel",
)

app = BUNDLE(
    coll,
    name="tauri-app-wheel.app",
    icon="icons/tauri_app_wheel.icns" if os.path.exists("icons/tauri_app_wheel.icns") else None,
    bundle_identifier="com.tauri-app-wheel.app",
)
