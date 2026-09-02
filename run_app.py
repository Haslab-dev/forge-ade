#!/usr/bin/env python
"""PyInstaller entry point for ForgeADE.

`python -m forge_ade` can't be expressed in a PyInstaller spec directly,
so this tiny launcher runs the same code path.
"""

import sys

from forge_ade import main

if __name__ == "__main__":
    sys.exit(main())
