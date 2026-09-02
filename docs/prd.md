# PRD — ForgeADE

### Native AI Development Workspace

**Version:** 1.0
**Status:** Draft
**Author:** Lutfi
**Target Platform:** macOS, Windows, Linux
**Technology Stack:** Go + Wails + React + CodeMirror + PTY

---

# Vision

Forge is a native, lightweight, AI-first development workspace built for modern software engineers.

Unlike traditional IDEs that treat AI as an extension, Forge treats AI agents, terminals, Git, and projects as first-class citizens.

The editor is only one part of the workspace.

Everything runs locally with minimal resource usage.

---

# Goals

## Primary

Create a desktop application that combines

* Code editing
* Multi-root workspaces
* Multiple terminal sessions
* AI agent runtime
* Git visualization
* Project management

inside one native application.

---

## Secondary

Provide a foundation for

* Autonomous AI development
* Team workspaces
* Plugin ecosystem
* Remote development
* Cloud synchronization

---

# Principles

## Native First

No Electron.

Backend written in Go.

Frontend rendered through Wails.

---

## Workspace First

Everything belongs to a workspace.

Folders are simply temporary workspaces.

---

## AI Native

AI is part of the architecture.

Not an extension.

---

## Lightweight

Fast startup.

Low RAM usage.

Minimal background processes.

---

## Offline First

Everything works locally.

Cloud features are optional.

---

# Core Features

---

# Workspace

Workspace represents an entire development environment.

A workspace may contain

* one folder
* multiple folders
* multiple Git repositories
* AI configuration
* terminal sessions
* layout
* project settings

---

## Open Folder

Open a single folder.

Internally converted into a temporary workspace.

Example

```
Open Folder

↓

~/Projects/backend

↓

Temporary Workspace
```

---

## Open Workspace

Open

```
company.workspace
```

Loads

* folders
* layout
* git
* terminals
* AI
* settings

---

## Save Workspace

Converts temporary workspace into persistent workspace.

```
Temporary

↓

Save Workspace

↓

company.workspace
```

---

## Workspace File

```yaml
version: 1

name: Company Platform

folders:
  - ./backend
  - ./frontend
  - ./mobile
  - ./infra

settings:
  theme: dark

git:
  autoFetch: true

agents:
  backend:
    provider: opencode

terminals:
  backend:
    shell: zsh
```

---

# Explorer

Supports

* Multiple folders
* File search
* Drag drop
* Rename
* Delete
* Copy
* Paste
* Symbolic links
* Hidden files

---

# Editor

Supports

* Multiple tabs
* Split editor
* Multi cursor
* Find Replace
* Code folding
* Minimap
* Sticky scroll
* Breadcrumb
* Diagnostics
* LSP integration
* Syntax highlighting
* Code actions

Editor engine

CodeMirror 6

---

# Terminal

Multiple concurrent PTY sessions.

Unlimited tabs.

Examples

```
Shell

Claude

Opencode

Codex

Docker

Python

Bun

Git
```

Each terminal owns

* process
* cwd
* environment
* history

Supports

* resize
* copy
* paste
* search
* hyperlinks
* ANSI colors
* persistent sessions

---

# AI Agents

Agents are independent runtime processes.

Supported providers

* Claude CLI
* Opencode
* Gemini CLI
* Codex CLI
* Aider
* Custom

Each agent owns

* terminal
* workspace
* history
* context
* MCP servers

Capabilities

* edit files
* execute commands
* search project
* review code
* generate code
* refactor
* explain
* create commits

---

# Agent Manager

Displays

* running agents
* current task
* CPU usage
* memory usage
* token usage
* active files
* command history
* execution logs

Multiple agents may operate simultaneously.

---

# File Search

Supports

* filename
* content
* regex
* case sensitive
* whole word

Indexing occurs in background.

---

# Global Search

Search across every folder in workspace.

---

# Git

Git is a first-class module.

Not an extension.

---

## Repository Manager

Workspace may contain multiple repositories.

Example

```
Backend

Frontend

Mobile

Infrastructure
```

Each repository maintains independent state.

---

## Source Control

Supports

* Stage
* Unstage
* Commit
* Amend
* Push
* Pull
* Fetch
* Stash
* Cherry Pick
* Revert
* Merge
* Rebase
* Reset
* Checkout

---

## Git Graph

Interactive commit graph.

Displays

* branches
* merges
* tags
* HEAD
* remotes

Selecting a commit displays

* changed files
* commit details
* inline diff

Supports

* checkout
* compare
* revert
* cherry-pick
* merge
* tag creation

---

## Branch Manager

Supports

* create
* rename
* delete
* checkout
* compare
* merge
* rebase

---

## Tags

Supports

* annotated
* lightweight
* push
* delete

---

## Commit History

Displays

* author
* avatar
* message
* changed files
* statistics

---

# Diff Viewer

Side-by-side comparison.

Supports

* inline diff
* syntax highlight
* word diff
* staging selected lines

---

# File Watcher

Real-time monitoring.

Automatically updates

* explorer
* editor
* search index
* git
* diagnostics

---

# Workspace Resources

Workspace may register external resources.

Examples

```
Folders

Repositories

Docker

SSH

Database

Redis

Kubernetes

Cloud

Notes

Environment
```

---

# Layout System

Dockable panels.

Examples

```
Explorer

Editor

Git

Terminal

AI

Problems

Search

Logs

Outline

Debug
```

Panels may be

* hidden
* moved
* resized
* detached

---

# Event Bus

Every subsystem communicates using events.

Examples

```
Git Commit

↓

Git Updated

↓

Explorer Refresh

↓

Status Refresh

↓

AI Refresh
```

No module communicates directly.

---

# Plugin System

Plugins may contribute

* commands
* panels
* editors
* AI providers
* git integrations
* themes
* snippets

Communication

JSON-RPC

---

# Settings

Levels

Global

Workspace

Folder

Supports

* JSON
* UI editor

---

# Theme

Supports

* Dark
* Light
* Custom

---

# Recent Projects

Displays

* workspaces
* folders

Includes

* pin
* favorite
* remove

---

# Session Restore

Restores

* open tabs
* split editors
* terminal sessions
* AI sessions
* layout

---

# Architecture

```
Frontend (React)

Explorer

Editor

Git

AI

Terminal

Panels

↓

IPC

↓

Go Backend

↓

Workspace Manager

↓

Git Manager

↓

PTY Manager

↓

Agent Manager

↓

File Watcher

↓

Event Bus

↓

Plugin Runtime
```

---

# Directory Structure

```
cmd/

internal/

    workspace/

    terminal/

    git/

    editor/

    ai/

    events/

    explorer/

    plugins/

    search/

frontend/

pkg/

runtime/

assets/
```

---

# Non-functional Requirements

Startup

< 2 seconds

Memory

< 150 MB idle

Terminal latency

< 20 ms

Workspace loading

< 1 second

Git graph rendering

< 100 ms

Search

Incremental indexing

Cross-platform

macOS

Windows

Linux

---

# Future Roadmap

## Phase 1

* Workspace
* Explorer
* Editor
* Terminal
* Git
* Search

## Phase 2

* AI runtime
* Agent manager
* MCP integration
* Multi-agent support

## Phase 3

* Plugin SDK
* Remote SSH
* Docker integration
* Kubernetes integration

## Phase 4

* Cloud sync
* Team collaboration
* Shared workspaces
* AI orchestration
* Workspace marketplace

---

# Product Vision

Forge is not intended to replace VS Code by copying its features.

Its purpose is to redefine the development environment around autonomous agents, native performance, and workspace-centric workflows.

The long-term vision is a desktop platform where code, terminals, infrastructure, Git, and AI agents operate as coordinated components inside a single native application.
