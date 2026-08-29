package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// hookWiring is deliberately project-local only (./.claude/settings.json,
// no global option). This is an untested v0 running an unvalidated
// classifier that fires notify-send on every session -- wiring it into
// every project globally before any live calibration exists would be a
// much bigger blast radius than this prototype has earned.
//
// If apiKey is non-empty, it's baked directly into the Stop hook's own
// command string as a leading ANTHROPIC_API_KEY=... shell assignment --
// chosen over relying on the launching shell's environment (2026-08-29):
// nothing on this host auto-sources capitulant/.env, so an installed
// hook that only reads os.Getenv would silently log-and-skip forever
// unless the user remembered to source .env before every `claude`
// launch in this project. settings.json is already gitignored, so this
// isn't a new leak surface -- it moves the plaintext key from one
// gitignored file to another.
func hookWiring(exe, apiKey string) []struct{ Event, Command string } {
	stopCmd := exe + " stop"
	if apiKey != "" {
		stopCmd = fmt.Sprintf("ANTHROPIC_API_KEY=%s %s stop", shellSingleQuote(apiKey), exe)
	}
	return []struct{ Event, Command string }{
		{"Stop", stopCmd},
		{"UserPromptSubmit", exe + " inject"},
	}
}

// shellSingleQuote wraps s in single quotes for safe use in a shell
// command string, escaping any embedded single quote. Anthropic API keys
// don't contain shell metacharacters in practice, but the hook command
// this builds is written to a file and executed by a shell either way --
// quoting defensively costs nothing and isn't worth trusting on format
// alone.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// resolveAPIKeyForInstall finds the key to bake into the stop hook's
// command (see hookWiring). Checks the current environment first (already
// sourced, or manually exported), then falls back to reading ../.env
// directly -- capitulant/.env from cwd=hook/, the layout install's own
// documented steps assume ("cd hook" first). Returns "" with a non-nil
// err, not a fatal error, on failure -- callers decide whether a missing
// key should block install (it shouldn't; a key can be added later and
// the environment-variable path still works as a fallback).
func resolveAPIKeyForInstall() (string, error) {
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		return v, nil
	}
	data, err := os.ReadFile("../.env")
	if err != nil {
		return "", fmt.Errorf("not in environment and could not read ../.env: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		v, ok := strings.CutPrefix(line, "ANTHROPIC_API_KEY=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"'`)
		if v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("not in environment and no ANTHROPIC_API_KEY= line in ../.env")
}

func cmdInstall(args []string) {
	fl := flag.NewFlagSet("install", flag.ContinueOnError)
	fl.SetOutput(os.Stderr)
	write := fl.Bool("write", false, "apply the change to ./.claude/settings.json instead of printing it")
	if err := fl.Parse(args); err != nil {
		os.Exit(2)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "capitulant-hook install: cannot find own path: %s\n", err)
		os.Exit(1)
	}
	if abs, err := filepath.Abs(exe); err == nil {
		exe = abs
	}

	path, err := settingsPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "capitulant-hook install: %s\n", err)
		os.Exit(1)
	}

	apiKey, keyErr := resolveAPIKeyForInstall()

	if !*write {
		displayKey := ""
		if apiKey != "" {
			displayKey = "<redacted, resolved from ../.env>"
		}
		fmt.Printf("target: %s (project-local; this tool has no global install option)\n\n", path)
		fmt.Println("capitulant-hook install --write would merge these hooks:")
		for _, h := range hookWiring(exe, displayKey) {
			fmt.Printf("  %-18s  %s\n", h.Event, h.Command)
		}
		if apiKey != "" {
			fmt.Println("\nANTHROPIC_API_KEY resolved and will be baked into the stop hook's own command")
			fmt.Println("string on --write, so it works regardless of the launching shell's environment.")
		} else {
			fmt.Printf("\nANTHROPIC_API_KEY not resolved (%s) -- the stop\nhook's command will run without it baked in, and stop will log-and-skip until\nthe environment provides it.\n", keyErr)
		}
		fmt.Println("Re-run with --write to apply (the existing file is backed up first).")
		return
	}

	changed, backup, err := mergeSettings(path, exe, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "capitulant-hook install: %s\n", err)
		os.Exit(1)
	}
	if backup != "" {
		fmt.Printf("backup: %s\n", backup)
	}
	if changed == 0 {
		fmt.Printf("%s: already wired, nothing to do\n", path)
	} else {
		fmt.Printf("%s: %d hook(s) added\n", path, changed)
	}
	if apiKey == "" {
		fmt.Printf("\nWarning: ANTHROPIC_API_KEY not resolved (%s) -- stop will\nlog-and-skip until it's set in the environment Claude Code runs in.\n", keyErr)
	}
	fmt.Println("\nHooks load at session start -- restart Claude Code in this project to pick them up.")
}

func cmdUninstall(args []string) {
	fl := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fl.SetOutput(os.Stderr)
	if err := fl.Parse(args); err != nil {
		os.Exit(2)
	}

	path, err := settingsPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "capitulant-hook uninstall: %s\n", err)
		os.Exit(1)
	}

	removed, backup, err := unmergeSettings(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "capitulant-hook uninstall: %s\n", err)
		os.Exit(1)
	}
	if backup != "" {
		fmt.Printf("backup: %s\n", backup)
	}
	fmt.Printf("%s: %d hook(s) removed\n", path, removed)
}

func settingsPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".claude", "settings.json"), nil
}

func mergeSettings(path, exe, apiKey string) (added int, backup string, err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return 0, "", err
	}

	var mode os.FileMode = 0o600
	var settings map[string]any

	data, readErr := os.ReadFile(path)
	switch {
	case readErr == nil:
		if info, statErr := os.Stat(path); statErr == nil {
			mode = info.Mode().Perm()
		}
		backup = path + ".capitulant-backup-" + time.Now().Format("20060102-150405")
		if err := os.WriteFile(backup, data, mode); err != nil {
			return 0, "", fmt.Errorf("backup: %w", err)
		}
		if err := json.Unmarshal(data, &settings); err != nil {
			return 0, backup, fmt.Errorf("%s is not valid JSON: %w", path, err)
		}
	case errors.Is(readErr, os.ErrNotExist):
		settings = map[string]any{}
	default:
		return 0, "", readErr
	}
	if settings == nil {
		settings = map[string]any{}
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	for _, h := range hookWiring(exe, apiKey) {
		if addHook(hooks, h.Event, h.Command) {
			added++
		}
	}
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return 0, backup, err
	}
	return added, backup, os.WriteFile(path, append(out, '\n'), mode)
}

// addHook wires one event -> command. If an existing capitulant entry
// for the same subcommand (stop/inject) is already present, its command
// string is replaced in place rather than appended alongside -- without
// this, a rotated ANTHROPIC_API_KEY baked into the stop command (see
// hookWiring) would leave the OLD key's hook still firing after a
// reinstall, since the two command strings differ byte-for-byte and a
// plain equality check would treat them as unrelated. Returns whether
// anything changed (added or replaced).
func addHook(hooks map[string]any, event, cmd string) bool {
	newSub, _ := capitulantSubcommand(cmd)

	existing, _ := hooks[event].([]any)
	for _, entry := range existing {
		em, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		inner, _ := em["hooks"].([]any)
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			c, _ := hm["command"].(string)
			if c == cmd {
				return false
			}
			if sub, ok := capitulantSubcommand(c); ok && newSub != "" && sub == newSub {
				hm["command"] = cmd
				return true
			}
		}
	}
	hooks[event] = append(existing, map[string]any{
		"matcher": "",
		"hooks": []any{
			map[string]any{"type": "command", "command": cmd},
		},
	})
	return true
}

func unmergeSettings(path string) (removed int, backup string, err error) {
	data, readErr := os.ReadFile(path)
	if errors.Is(readErr, os.ErrNotExist) {
		return 0, "", nil
	}
	if readErr != nil {
		return 0, "", readErr
	}

	var mode os.FileMode = 0o600
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return 0, "", fmt.Errorf("%s is not valid JSON: %w", path, err)
	}

	backup = path + ".capitulant-backup-" + time.Now().Format("20060102-150405")
	if err := os.WriteFile(backup, data, mode); err != nil {
		return 0, "", fmt.Errorf("backup: %w", err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	for event, val := range hooks {
		list, ok := val.([]any)
		if !ok {
			continue
		}
		kept := make([]any, 0, len(list))
		for _, entry := range list {
			em, ok := entry.(map[string]any)
			if !ok {
				kept = append(kept, entry)
				continue
			}
			inner, _ := em["hooks"].([]any)
			keptInner := make([]any, 0, len(inner))
			for _, h := range inner {
				hm, ok := h.(map[string]any)
				if !ok {
					keptInner = append(keptInner, h)
					continue
				}
				if c, _ := hm["command"].(string); isCapitulantCmd(c) {
					removed++
					continue
				}
				keptInner = append(keptInner, h)
			}
			if len(keptInner) == 0 {
				continue
			}
			em["hooks"] = keptInner
			kept = append(kept, em)
		}
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return removed, backup, err
	}
	return removed, backup, os.WriteFile(path, append(out, '\n'), mode)
}

// capitulantSubcommand returns the capitulant-hook subcommand a hook
// command string invokes ("stop", "inject", ...), skipping any leading
// KEY=VALUE shell assignments -- the stop hook's own command can carry
// a leading ANTHROPIC_API_KEY=... (see hookWiring). ok is false if cmd
// doesn't invoke this project's own binary at all.
func capitulantSubcommand(cmd string) (subcmd string, ok bool) {
	fields := strings.Fields(cmd)
	i := 0
	for i < len(fields) && isShellAssignment(fields[i]) {
		i++
	}
	if len(fields) < i+2 || filepath.Base(fields[i]) != "capitulant-hook" {
		return "", false
	}
	return fields[i+1], true
}

// isShellAssignment reports whether field has the shape NAME=... with a
// valid shell identifier for NAME (letters, digits, underscore; not
// starting with a digit) -- the shape hookWiring's ANTHROPIC_API_KEY=...
// prefix takes, and the shape any similar future prefix would too.
func isShellAssignment(field string) bool {
	eq := strings.IndexByte(field, '=')
	if eq <= 0 {
		return false
	}
	for i := 0; i < eq; i++ {
		c := field[i]
		switch {
		case c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'):
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

func isCapitulantCmd(cmd string) bool {
	switch sub, ok := capitulantSubcommand(cmd); {
	case !ok:
		return false
	case sub == "stop" || sub == "inject":
		return true
	default:
		return false
	}
}
