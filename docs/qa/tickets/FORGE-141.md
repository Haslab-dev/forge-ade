# FORGE-141 — CLI agent sessions pollute recent projects with agent dot-dirs

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** workspace/recents
- **Labels:** `gui-verified`, `found-in-computer-use-round`
- **Evidence:** `~/.forge-ade/recent.json` contains `/Users/lutfiikbalmajid/.omp/agent`, `~/.pi/agent/skills`, `~/.gemini/antigravity-cli`, `~/.omp/agent/skills`; the editor RECENT PROJECTS list additionally shows fragment entries ("Users", "tmp", "agent").
- **Root cause (suspected):** creating a Terminal-Session (CLI agent) records the agent's working directory as a recent "workspace" (recents `addRecent` is fed the session cwd, not a user-chosen folder).
- **Fix sketch:** only register recents for user-initiated opens (dialog / explicit project switch), or filter out `~/.dotdir` paths; add a "forget" affordance.
