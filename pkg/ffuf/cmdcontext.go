package ffuf

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// noiseProcessNames lists process names that should be excluded from the
// reconstructed pipeline because they are spawned by the shell as
// supporting infrastructure rather than as actual pipeline members.
var noiseProcessNames = map[string]struct{}{
	"conhost.exe":        {},
	"openconsole.exe":    {},
	"WindowsTerminal.exe": {},
}

func isNoiseProcess(cmdline string) bool {
	cmd := strings.TrimSpace(cmdline)
	if cmd == "" {
		return true
	}
	// Take just the executable name portion (handles both quoted and
	// unquoted invocations, with or without forward / backward slashes).
	first := cmd
	if idx := strings.IndexAny(first, " \t"); idx > 0 {
		first = first[:idx]
	}
	first = strings.Trim(first, "\"'")
	exe := strings.ToLower(filepath.Base(first))
	if _, ok := noiseProcessNames[exe]; ok {
		return true
	}
	// Reject unknown special device paths like \??\... that conhost uses
	// when reported via gopsutil on Windows.
	if strings.HasPrefix(strings.TrimSpace(cmd), "\\??\\") {
		return true
	}
	return false
}

// CaptureFullCommand attempts to reconstruct the full shell pipeline that
// ffuf is part of (e.g. `cat words.txt | grep foo | ffuf -w -`). It does so
// by inspecting sibling processes (other children of ffuf's parent) and
// joining their command lines in start-time order with " | ".
//
// The result is best-effort: in interactive shell sessions the parent shell
// itself does not retain the pipeline string, so we approximate it from the
// list of currently-running sibling processes. If reconstruction fails for
// any reason (permission denied, parent gone, etc) the raw `os.Args` join
// is returned so the caller always gets *something*.
//
// Should be called early in main() so siblings spawned by the same shell
// pipeline are still alive.
func CaptureFullCommand() string {
	fallback := strings.Join(os.Args, " ")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	selfPid := int32(os.Getpid())
	self, err := process.NewProcessWithContext(ctx, selfPid)
	if err != nil {
		return fallback
	}

	parent, err := self.ParentWithContext(ctx)
	if err != nil || parent == nil {
		return fallback
	}

	children, err := parent.ChildrenWithContext(ctx)
	if err != nil || len(children) == 0 {
		return fallback
	}

	type sibling struct {
		pid     int32
		cmd     string
		started int64
	}
	siblings := make([]sibling, 0, len(children))
	for _, c := range children {
		if c == nil {
			continue
		}
		cmd, cerr := c.CmdlineWithContext(ctx)
		if cerr != nil || strings.TrimSpace(cmd) == "" {
			continue
		}
		if isNoiseProcess(cmd) {
			continue
		}
		ts, terr := c.CreateTimeWithContext(ctx)
		if terr != nil {
			ts = 0
		}
		siblings = append(siblings, sibling{pid: c.Pid, cmd: cmd, started: ts})
	}

	if len(siblings) == 0 {
		return fallback
	}
	// If the only remaining sibling is ffuf itself, no actual pipeline was
	// detected (we are running standalone in this shell). Return empty so
	// the caller can decide to omit the field rather than duplicating the
	// commandline.
	if len(siblings) == 1 && siblings[0].pid == selfPid {
		return ""
	}

	// Sort by start time so the pipeline reads left-to-right in the order
	// the shell actually spawned the processes.
	sort.SliceStable(siblings, func(i, j int) bool {
		if siblings[i].started == siblings[j].started {
			return siblings[i].pid < siblings[j].pid
		}
		return siblings[i].started < siblings[j].started
	})

	parts := make([]string, 0, len(siblings))
	for _, s := range siblings {
		parts = append(parts, strings.TrimSpace(s.cmd))
	}
	return strings.Join(parts, " | ")
}
