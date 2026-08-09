package main

import (
	"fmt"
	"os"
	"time"

	"github.com/justinstimatze/capitulant/hook/internal/substrate"
)

// logf appends one line to a small local log. Every failure path is
// silent by design -- logging is the last thing that should be able to
// break a hook. This is a diagnostic trail (did the hook run, did the API
// call fail), not the findings substrate -- see internal/substrate for that.
func logf(name, format string, args ...any) {
	dir, err := substrate.Dir()
	if err != nil {
		return
	}
	f, err := os.OpenFile(dir+"/hook.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s [%s] %s\n", time.Now().UTC().Format(time.RFC3339), name, fmt.Sprintf(format, args...))
}
