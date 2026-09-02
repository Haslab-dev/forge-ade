Looking at your current UI, I actually wouldn't make AI terminals a special type of panel. I would make **everything a Terminal Session**.

Think of ForgeADE as a **Terminal Runtime**.

The editor, explorer, and git are UI components.

Everything executable is a session.

---

# Core Architecture

```
Workspace

├── Explorer
├── Editor
├── Search
├── Git
└── Runtime
      │
      ├── Terminal Manager
      │
      ├── Session #1
      ├── Session #2
      ├── Session #3
      └── Session #N
```

Notice there is no "AI Manager".

Only **Session Manager**.

---

# Session Types

Every process is just a session.

```
Session

Type

Shell

AI

Docker

SSH

Python

Node

Git

Custom
```

Every session implements the same interface.

```go
type Session interface {
    ID() string
    Name() string

    Start() error
    Stop() error
    Kill() error

    Write([]byte)
    Resize(cols, rows int)

    PID() int

    Status() Status
}
```

Opencode and Kilo are simply implementations.

---

# Terminal Manager

```
Terminal Manager

├── Shell
│
├── Opencode
│
├── Claude
│
├── Kilo
│
├── Antigravity
│
├── Docker
│
├── Git
│
└── Python
```

Internally

```go
type TerminalManager struct {
    sessions map[string]*Session
}
```

---

# Session Object

```go
type Session struct {

    ID string

    Name string

    Type SessionType

    WorkspaceID string

    Folder string

    Provider string

    PTY *pty.PTY

    Process *exec.Cmd

    Status SessionStatus

    CreatedAt time.Time
}
```

Notice there is **no AI logic**.

---

# Agent Provider

Instead

```
Session

↓

Provider

↓

Opencode
```

or

```
Session

↓

Provider

↓

Kilo
```

---

## Example

```
New Session

Name:
Backend AI

Provider:
Opencode

Working Directory:
/backend

Command:

opencode
```

Another

```
New Session

Name:
Review

Provider:
Kilo

Working Directory:
/frontend

Command:

kilo
```

Another

```
New Session

Name:
Architecture

Provider:
antigravity-cli

Working Directory:
/docs

Command:

antigravity
```

They're identical.

---

# Provider Definition

```yaml
providers:

  opencode:

    executable: opencode

    args: []

  kilo:

    executable: kilo

    args: []

  antigravity:

    executable: antigravity-cli

    args: []
```

The runtime doesn't care what launches.

---

# Session Lifecycle

```
Create

↓

Spawn PTY

↓

Spawn Process

↓

Read stdout

↓

Publish Event

↓

React

↓

Render

↓

User Input

↓

stdin

↓

PTY

↓

Process
```

Exactly like a normal terminal.

---

# Multiple Agent Example

```
Workspace

Backend

    Shell

    Opencode

Frontend

    Shell

    Claude

Infrastructure

    Shell

    Antigravity

Documentation

    Shell

    Kilo
```

Each one is independent.

---

# Session Sidebar

Instead of

```
Terminal
```

I'd make

```
Runtime

▼ Backend

    🟢 Shell

    🤖 Opencode

▼ Frontend

    🟢 Shell

    🤖 Claude

▼ Infrastructure

    🟢 Shell

    🤖 Antigravity

▼ Documentation

    🤖 Kilo
```

Grouped by workspace folder.

---

# Runtime Monitor

Imagine another panel.

```
Runtime

🟢 Shell
PID 3012
CPU 0%

🟢 Opencode
PID 4021
CPU 8%

🟢 Claude
PID 4102
CPU 1%

🟢 Antigravity
PID 5100
CPU 15%
```

Exactly like Activity Monitor.

---

# Session Tabs

Bottom panel

```
┌────────────────────────────────────────────┐

Shell │ Opencode │ Claude │ Kilo │ + │

──────────────────────────────────────────────

Output

──────────────────────────────────────────────

```

Or

```
Backend

    Shell

    Opencode

Frontend

    Shell

    Claude

Docs

    Kilo
```

Both layouts can coexist.

---

# Session Templates

One feature I think will make ForgeADE stand out is **Session Templates**.

Instead of manually opening five terminals every morning:

```
New Session

↓

Opencode

↓

Claude

↓

Shell

↓

Docker

↓

Python
```

you save:

```yaml
sessionTemplates:

  backend:

    - shell

    - opencode

    - docker

  frontend:

    - shell

    - kilo

  docs:

    - antigravity
```

When the workspace opens:

```
Open Workspace

↓

Restore

↓

Spawn

Backend

    Shell

    Opencode

    Docker

Frontend

    Shell

    Kilo

Docs

    Antigravity
```

The developer is productive immediately, without recreating their environment. This is especially powerful when combined with your `.workspace` file, because the workspace defines not only which folders are open, but also which tools and AI agents should already be running. That makes ForgeADE feel less like an editor and more like a reproducible development environment.
