package main

import "testing"

func TestIsShellAssignment(t *testing.T) {
	cases := map[string]bool{
		"ANTHROPIC_API_KEY=sk-ant-fake": true,
		"FOO=bar":                       true,
		"_X=1":                          true,
		"A1=1":                          true,
		"1A=1":                          false, // can't start with a digit
		"=bar":                          false, // empty name
		"/usr/bin/capitulant-hook":      false, // no '='
		"foo=bar=baz":                   true,  // only the first '=' matters
		"has space=bar":                 false, // not a single field once Fields() splits it, but isShellAssignment itself just checks the name chars
	}
	for in, want := range cases {
		if got := isShellAssignment(in); got != want {
			t.Errorf("isShellAssignment(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestCapitulantSubcommand(t *testing.T) {
	cases := []struct {
		cmd     string
		wantSub string
		wantOK  bool
	}{
		{"/home/x/capitulant-hook stop", "stop", true},
		{"/home/x/capitulant-hook inject", "inject", true},
		{"ANTHROPIC_API_KEY='sk-ant-fake' /home/x/capitulant-hook stop", "stop", true},
		{"FOO=1 BAR=2 /home/x/capitulant-hook stop", "stop", true},
		{"/home/x/some-other-binary stop", "", false},
		{"/home/x/capitulant-hook", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		sub, ok := capitulantSubcommand(c.cmd)
		if sub != c.wantSub || ok != c.wantOK {
			t.Errorf("capitulantSubcommand(%q) = (%q, %v), want (%q, %v)", c.cmd, sub, ok, c.wantSub, c.wantOK)
		}
	}
}

func TestIsCapitulantCmd(t *testing.T) {
	yes := []string{
		"/home/x/capitulant-hook stop",
		"/home/x/capitulant-hook inject",
		"ANTHROPIC_API_KEY='sk-ant-fake' /home/x/capitulant-hook stop",
	}
	no := []string{
		"/home/x/capitulant-hook bench",
		"/home/x/capitulant-hook stop-worker /tmp/x.json",
		"/home/x/other-tool stop",
		"",
	}
	for _, c := range yes {
		if !isCapitulantCmd(c) {
			t.Errorf("isCapitulantCmd(%q) = false, want true", c)
		}
	}
	for _, c := range no {
		if isCapitulantCmd(c) {
			t.Errorf("isCapitulantCmd(%q) = true, want false", c)
		}
	}
}

// TestAddHookReplacesRotatedKey is the regression this whole file exists
// for: a re-run of install --write with a different (rotated)
// ANTHROPIC_API_KEY must replace the stop hook's command in place, not
// add a second Stop entry alongside the old key's -- two entries would
// mean classify.Run fires twice per matched turn, doubling cost and
// notifications, with the old key still live in settings.json.
func TestAddHookReplacesRotatedKey(t *testing.T) {
	hooks := map[string]any{}

	oldCmd := "ANTHROPIC_API_KEY='sk-ant-fake-old' /home/x/capitulant-hook stop"
	if !addHook(hooks, "Stop", oldCmd) {
		t.Fatal("first addHook should report a change")
	}

	newCmd := "ANTHROPIC_API_KEY='sk-ant-fake-new' /home/x/capitulant-hook stop"
	if !addHook(hooks, "Stop", newCmd) {
		t.Fatal("addHook with a rotated key should report a change (replace)")
	}

	stop, _ := hooks["Stop"].([]any)
	if len(stop) != 1 {
		t.Fatalf("want exactly one Stop entry after a key rotation, got %d: %+v", len(stop), stop)
	}
	entry, _ := stop[0].(map[string]any)
	inner, _ := entry["hooks"].([]any)
	if len(inner) != 1 {
		t.Fatalf("want exactly one hook in the Stop entry, got %d", len(inner))
	}
	h, _ := inner[0].(map[string]any)
	if got := h["command"]; got != newCmd {
		t.Errorf("command = %v, want the rotated command %q (old key should be gone, not duplicated)", got, newCmd)
	}

	// Re-adding the exact same command a second time is a true no-op.
	if addHook(hooks, "Stop", newCmd) {
		t.Error("addHook with an identical command should report no change")
	}
	stop, _ = hooks["Stop"].([]any)
	if len(stop) != 1 {
		t.Fatalf("want still exactly one Stop entry after a no-op addHook, got %d", len(stop))
	}
}

// TestAddHookLeavesUnrelatedHooksAlone confirms addHook's replace path
// only touches capitulant's own entries -- a hook some other tool wired
// into the same event must survive untouched.
func TestAddHookLeavesUnrelatedHooksAlone(t *testing.T) {
	hooks := map[string]any{
		"Stop": []any{
			map[string]any{
				"matcher": "",
				"hooks": []any{
					map[string]any{"type": "command", "command": "/home/x/some-other-tool stop"},
				},
			},
		},
	}

	if !addHook(hooks, "Stop", "/home/x/capitulant-hook stop") {
		t.Fatal("addHook should report a change")
	}

	stop, _ := hooks["Stop"].([]any)
	if len(stop) != 2 {
		t.Fatalf("want the unrelated hook preserved plus capitulant's own, got %d entries: %+v", len(stop), stop)
	}
}
