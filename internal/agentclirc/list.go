package agentclirc

import (
	"os/exec"
	"runtime"
)

func execLookPath(exe string) (string, error) {
	return exec.LookPath(exe)
}

// List reads every known agent CLI's configuration fresh from disk. Disk is
// always re-read (configs change while the app runs); only the builder set is
// static.
func List() []RuntimeConfig {
	home := homeDir()
	if home == "" {
		return nil
	}
	builders := []*builder{
		buildPi(home),
		buildOmp(home),
		buildOpencode(home),
		buildCodex(home),
		buildClaude(home),
		buildAntigravity(home),
		buildGemini(home),
	}
	out := make([]RuntimeConfig, 0, len(builders))
	for _, b := range builders {
		b.rc.Installed = installed(b.rc.Executable)
		b.rc.Platform = runtime.GOOS
		out = append(out, b.Runtime())
	}
	return out
}
