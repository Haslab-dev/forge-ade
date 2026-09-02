# -*- mode: python ; coding: utf-8 -*-
"""PyInstaller spec for ForgeADE (pytauri backend).

- `pytauri_wheel` carries the precompiled Tauri dylib as a PyO3 extension
  module — collect_all() pulls the binary extension in.
- `pytauri_plugins` is a namespace package with many submodules.
- `SRC_TAURI_DIR = Path(__file__).parent` in forge_ade resolves inside the
  bundle to `<_internal>/forge_ade`, so Tauri.toml / capabilities / icons are
  added as datas under that name. The frontend dist stays external
  (frontend/dist next to the bundle) in this bootstrap spec — add it as a
  resource when you want a fully self-contained bundle.
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

PKG = "forge_ade"

datas += [
    (f"python/src/{PKG}/Tauri.toml", PKG),
    (f"python/src/{PKG}/capabilities", f"{PKG}/capabilities"),
    (f"python/src/{PKG}/icons", f"{PKG}/icons"),
    (f"python/src/{PKG}/py.typed", PKG),
]

a = Analysis(
    ["run_app.py"],
    pathex=["python/src"],
    binaries=binaries,
    datas=datas,
    hiddenimports=hiddenimports + ["forge_ade"],
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
    name="forge-ade",
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
    name="forge-ade",
)

app = BUNDLE(
    coll,
    name="ForgeADE.app",
    icon="icons/forge_ade.icns" if os.path.exists("icons/forge_ade.icns") else None,
    bundle_identifier="dev.haslab.forge-ade",
)
