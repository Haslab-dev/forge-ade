#!/usr/bin/env python
"""PyInstaller entry point for the tauri-app-wheel reference app.

`python -m tauri_app_wheel` can't be expressed in a PyInstaller spec directly,
so this tiny launcher runs the same code path.
"""

import sys

from tauri_app_wheel import main

if __name__ == "__main__":
    sys.exit(main())
