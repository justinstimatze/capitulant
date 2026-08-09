package substrate

import (
	"os"
	"syscall"
)

// withLock runs fn while holding an exclusive advisory lock on the
// substrate's lock file, blocking until it's available. Guards against
// two concurrent Claude Code sessions in the same project racing a Stop
// hook's Append against an UserPromptSubmit hook's Acknowledge -- without
// this, a record appended mid-rewrite could be silently dropped.
//
// syscall.Flock is Linux-specific, matching this project's existing
// scope (internal/notify is notify-send-only for the same reason).
func withLock(fn func() error) error {
	p, err := path()
	if err != nil {
		return err
	}
	lockPath := p + ".lock"

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return fn()
}
