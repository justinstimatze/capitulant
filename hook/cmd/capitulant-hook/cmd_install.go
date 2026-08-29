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
func hookWiring(exe string) []struct{ Event, Command string } {
	return []struct{ Event, Command string }{
		{"Stop", exe + " stop"},
		{"UserPromptSubmit", exe + " inject"},
	}
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

	if !*write {
		fmt.Printf("target: %s (project-local; this tool has no global install option)\n\n", path)
		fmt.Println("capitulant-hook install --write would merge these hooks:")
		for _, h := range hookWiring(exe) {
			fmt.Printf("  %-18s  %s\n", h.Event, h.Command)
		}
		fmt.Println("\nRequires ANTHROPIC_API_KEY in the environment to actually classify anything.")
		fmt.Println("Re-run with --write to apply (the existing file is backed up first).")
		return
	}

	changed, backup, err := mergeSettings(path, exe)
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

func mergeSettings(path, exe string) (added int, backup string, err error) {
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
	for _, h := range hookWiring(exe) {
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

func addHook(hooks map[string]any, event, cmd string) bool {
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
			if c, _ := hm["command"].(string); c == cmd {
				return false
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

func isCapitulantCmd(cmd string) bool {
	fields := strings.Fields(cmd)
	if len(fields) < 2 {
		return false
	}
	if filepath.Base(fields[0]) != "capitulant-hook" {
		return false
	}
	switch fields[1] {
	case "stop", "inject":
		return true
	}
	return false
}
