"""ForgeADE — AI development workspace on pytauri.

The whole Native SDK backend re-hosted as Python commands: fs, git, PTY
terminal, LSP, agent/LLM, MCP, external ACP agents, skills, usage, search,
syntax. The React frontend stays untouched apart from the zero-bridge shim
in frontend/src/lib/native.ts, which routes window.zero through Tauri IPC.
"""

from __future__ import annotations

import sys
from pathlib import Path

from anyio import create_task_group
from anyio.from_thread import start_blocking_portal
from pytauri import AppHandle

from . import bridge
from .bridge import commands, set_app_handle

# Import every command module so their @bridge.cmd registrations run.
from . import fs_cmds  # noqa: F401
from . import terminal  # noqa: F401
from . import llm  # noqa: F401
from . import agent  # noqa: F401
from . import mcp  # noqa: F401
from . import external  # noqa: F401
from . import lsp  # noqa: F401
from . import skills  # noqa: F401
from . import usage  # noqa: F401
from . import search  # noqa: F401
from . import syntax  # noqa: F401
from . import workspace  # noqa: F401
from . import misc_cmds  # noqa: F401

SRC_TAURI_DIR = Path(__file__).parent.absolute()

DEV = __import__("os").environ.get("FORGE_ADE_DEV") == "1"

task_group = None


def _single_instance_callback(app_handle: AppHandle, _args: list[str], _cwd: str) -> None:
    from pytauri import Manager

    main_window = Manager.get_webview_window(app_handle, "main")
    if main_window is not None:
        main_window.set_focus()


def main() -> int:
    """Run the ForgeADE Tauri app."""
    global task_group
    from pytauri_plugins import (
        clipboard_manager,
        dialog,
        fs,
        notification,
        opener,
        os,
        process,
        shell,
    )

    with (
        start_blocking_portal("asyncio") as portal,
        portal.wrap_async_context_manager(portal.call(create_task_group)) as task_group,
    ):
        import json as _json

        # context_factory takes the dynamic tauri config as a JSON STRING
        # (the pytauri-wheel contract — metdesk-v2 does the same).
        tauri_config = (
            _json.dumps({"build": {"frontendDist": "http://localhost:5173"}}) if DEV else None
        )

        from pytauri_wheel.lib import builder_factory, context_factory

        app = builder_factory().build(
            context=context_factory(SRC_TAURI_DIR, tauri_config=tauri_config),
            invoke_handler=commands.generate_handler(portal),
            plugins=(
                dialog.init(),
                notification.init(),
                clipboard_manager.init(),
                fs.init(),
                opener.init(),
                os.init(),
                process.init(),
                shell.init(),
            ),
        )
        set_app_handle(app)
        exit_code = app.run_return()
        return exit_code


if __name__ == "__main__":
    sys.exit(main())
