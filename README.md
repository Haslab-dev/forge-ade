# ForgeADE Native

> Pure Go Native Desktop IDE for ForgeADE powered by [MyGo Native UI](https://mygo.egoist.dev/)

## Overview

ForgeADE Native is a port of [ForgeADE](https://github.com/Haslab-dev/forge-ade) targeting a 100% pure Go native UI stack with `CGO_ENABLED=0`. It eliminates the Chromium/WebKit runtime, Node.js tooling, and IPC serialization layer in favor of direct GPU rendering and zero-copy in-memory state.

### Architecture Highlights

- **Pure Go & CGO-Free**: Runs with `CGO_ENABLED=0` across macOS (Metal), Windows (Direct3D 11), and Linux (OpenGL).
- **GPU-Accelerated Code Editor**: Powered by `github.com/egoist/mygo/ui/editor` with virtualized buffers, syntax highlighting, diff inspection, and multi-caret support.
- **Hardware-Accelerated Terminal**: Built on `github.com/egoist/mygo/plugins/terminal` using Ghostty's `libghostty-vt` terminal engine.
- **Native LLM Markdown Streaming**: Streaming agent turns and code blocks rendered directly via `github.com/egoist/mygo/ui/markdown`.
- **Integrated ForgeADE Engine**: Connects directly to ForgeADE's composition root (`github.com/hasdev/forge-ade/internal/bootstrap` and RakitKode).

## Getting Started

```bash
# Run in development mode
go run ./cmd/forge-native

# Build standalone binary
go build -o build/bin/forge-native ./cmd/forge-native
```
