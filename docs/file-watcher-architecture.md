# File Watcher Architecture

## Problem

macOS uses kqueue (not FSEvents) for fsnotify, which requires **one file descriptor per watched directory**. The default macOS ulimit (256) is too low for most projects.

## Solution

### Recursive Watching
`WatchDir` walks the entire directory tree and adds each directory to fsnotify:

```
Root/
├── src/       → fsnotify.Add("src/")
│   ├── lib/   → fsnotify.Add("src/lib/")
│   └── ui/    → fsnotify.Add("src/ui/")
├── public/    → fsnotify.Add("public/")
└── docs/      → fsnotify.Add("docs/")
```

### Skipped Directories
The following are **not** watched (saves FDs):
```
node_modules, .git, .svn, vendor, .next, .cache,
dist, build, coverage, __pycache__, .hg
```

### FD Limit
On startup, attempt to raise `RLIMIT_NOFILE` to 10240:
```go
var rLimit syscall.Rlimit
syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
rLimit.Cur = 10240
syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
```

If watching a directory fails (too many FDs), log a warning and skip it.

### Event Flow
```
fsnotify event
    ↓
handleEvent()
    ↓
events.Bus.Publish()
    ↓
app.go subscribers:
    ├── search: IndexFile / RemoveFile (incremental)
    └── runtime.EventsEmit("fs:changed", data)
        ↓
    frontend EventsOn("fs:changed")
        ↓
    Explorer: setRefreshKey → re-fetch file tree
```

### New Directory Handling
When a new directory is created at runtime:
```
event.Op & fsnotify.Create → os.Stat(event.Name)
    → if IsDir → fsnotify.Add(event.Name)
```

This ensures subdirectory changes are detected even for directories created after the initial watch was set up.
